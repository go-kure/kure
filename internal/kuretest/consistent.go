package kuretest

import (
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	securefs "github.com/fluxcd/pkg/kustomize/filesys"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/errors"
	kureio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// disagreeHeadline heads the findings of the consistency pass.
const disagreeHeadline = "%d object(s) that do not agree with the tree:"

// systemNamespaces are the namespaces every cluster creates itself.
var systemNamespaces = map[string]bool{
	metav1.NamespaceDefault:   true,
	metav1.NamespaceSystem:    true,
	metav1.NamespacePublic:    true,
	corev1.NamespaceNodeLease: true,
}

// Declared is something a tree relies on without creating it.
type Declared struct{ kind, namespace, name string }

// Namespace declares a namespace the cluster already has.
func Namespace(name string) Declared { return Declared{kind: "Namespace", name: name} }

// External declares an object outside the tree that a reference may name. A
// reference names a namespaced kind only, so an omitted namespace reads as
// default, as it does on an object.
func External(kind, namespace, name string) Declared {
	return Declared{kind: kind, namespace: effectiveNamespace(namespace), name: name}
}

// AssertConsistent encodes the objects with kure's default options and holds
// the YAML to AssertConsistentYAML.
func AssertConsistent(t testing.TB, objs []client.Object, declared ...Declared) {
	t.Helper()
	ptrs := make([]*client.Object, len(objs))
	for i := range objs {
		ptrs[i] = &objs[i]
	}
	data, err := kureio.EncodeObjectsToYAML(ptrs)
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}
	AssertConsistentYAML(t, data, declared...)
}

// AssertConsistentYAML checks that the objects in data agree with each other
// and with what is declared, and fails the test with every finding at once:
//
//   - every Flux Kustomization's spec.sourceRef and spec.dependsOn, and every
//     HelmRelease's spec.chartRef, spec.chart.spec.sourceRef and
//     spec.dependsOn, names an object in data or an External declaration;
//   - every object's metadata.namespace, and every Kustomization's
//     spec.targetNamespace, is a Namespace in data, a Namespace declaration
//     or one the cluster creates itself (default, kube-system, kube-public,
//     kube-node-lease).
//
// An object is known by kind, namespace and name: its group is not compared.
// An omitted namespace reads as default on an object, a declaration and a
// reference alike, except that a reference without one is in its
// referrer's; a cluster-scoped kind has none. kustomize's own configuration
// is not an object and is read past. Data with no object in it fails: there
// is nothing a pass could be about.
func AssertConsistentYAML(t testing.TB, data []byte, declared ...Declared) {
	t.Helper()
	decoded, err := decode(data)
	if err != nil {
		t.Fatalf("kuretest: %v", err)
	}
	var objs []*object
	for i, u := range decoded {
		if u.GroupVersionKind().Group != kustomizeGroup {
			objs = append(objs, &object{doc: describe(i, u), u: u})
		}
	}
	if len(objs) == 0 {
		t.Fatalf("kuretest: no objects in the YAML")
	}
	if err := agree(objs, declared); err != nil {
		t.Fatalf("kuretest: %v", err)
	}
	if findings := collect(objs, nil); len(findings) > 0 {
		t.Fatalf("%s", formatWith(disagreeHeadline, findings))
	}
}

// AssertConsistentDir holds every .yaml and .yml file under root to
// AssertConsistentYAML, as one set of objects, then holds the tree to the
// paths it names and builds what each Flux Kustomization in it applies:
//
//   - every Flux Kustomization's spec.path is a directory in the tree;
//   - every resources and components entry of a kustomization file is a
//     file or a directory holding a kustomization file (a components entry
//     a directory only), anywhere in the tree; every patches path and
//     configMapGenerator or secretGenerator files, envs or env entry is a
//     file;
//   - every Flux Kustomization with such a path builds as
//     kustomize-controller builds it: Flux generates or amends the
//     directory's kustomization file with the Kustomization's own fields,
//     then kustomize builds it, plugins off, loading anywhere in the tree.
//     The builds run in walk order on one copy of the tree, which each
//     leaves as it found it, so the tree itself is never written.
//
// Nothing is built when a path could take a build past the tree, or a
// write past the copy: an entry that is not a local path — kustomize would
// fetch it; a kustomization field whose loads the pass does not resolve
// (bases, crds, openapi, configurations, generators, transformers,
// validators, the helm fields, patchesStrategicMerge, patchesJson6902,
// replacements); a symbolic link anywhere under root — the copy keeps it
// as a link, and Flux's generator and kustomize write through one; or a
// file named <kustomization file>.original, which Flux's generator
// overwrites with its own backup and then consumes. Each of those is a
// finding. A link is still read as what its name says, so the other checks
// run.
//
// Three rules are stricter than a Flux build, because the pass holds a
// tree to the layout kure writes; a hand-written tree that breaks one gets
// a finding where Flux would build. A file entry stays in or below its
// kustomization file's directory — kustomize's default load restriction,
// which Flux lifts to the source root. An entry holding : or @ is read as
// one kustomize would fetch — the scp-style and file:// forms it clones
// before any load restriction applies — unless it names a file in the
// tree, which kustomize reads before it parses an entry as a repository;
// a directory so named, or nothing, it looks for only after. And spec.path
// is a relative path that stays in the tree, where Flux joins an absolute
// or escaping one under the source root. kure writes relative paths and
// files that exist, so none of its trees breaks one.
//
// root is the directory the Flux source serves, the one every spec.path is
// relative to: for layout.WriteManifest(base, cfg, ml) that is
// filepath.Join(base, cfg.ManifestsDir), for WriteToDisk(base) it is base.
//
// A file kustomize recognizes as a kustomization file is read as its
// configuration, not as objects, and a file one names as data — a
// configMapGenerator or secretGenerator files, envs or env entry, a
// patches path — is not decoded, unless a kustomization file also names it
// as a resources entry: kustomize applies it as a manifest then, so it is
// read as one. Every other manifest decodes or fails the test. A tree whose
// files hold no object fails.
func AssertConsistentDir(t testing.TB, root string, declared ...Declared) {
	t.Helper()
	tr, err := readTree(root)
	if err != nil {
		t.Fatalf("kuretest: %v", err)
	}
	if len(tr.objects) == 0 {
		t.Fatalf("kuretest: no objects under %s", root)
	}
	if err := agree(tr.objects, declared); err != nil {
		t.Fatalf("kuretest: %v", err)
	}
	if err := tr.checkPaths(); err != nil {
		t.Fatalf("kuretest: %v", err)
	}
	if findings := tr.findings(); len(findings) > 0 {
		t.Fatalf("%s", formatWith(disagreeHeadline, findings))
	}
}

// object is one decoded object, the Flux kinds the pass reads converted, and
// the errors found on it.
type object struct {
	doc           string
	u             *unstructured.Unstructured
	kustomization *kustv1.Kustomization
	release       *helmv2.HelmRelease
	// dir is a Flux Kustomization's spec.path, cleaned, once it names a
	// directory in the tree: "." for the root, empty until then.
	dir  string
	errs field.ErrorList
}

// kustomizationFile is one file kustomize recognizes, read as its
// configuration.
type kustomizationFile struct {
	path string
	k    kusttypes.Kustomization
	errs field.ErrorList
}

// tree is a written directory read for the pass.
type tree struct {
	root           string
	objects        []*object
	kustomizations []*kustomizationFile
	// errs are the findings on the tree itself: an entry a build would
	// write through or consume, by its path under root.
	errs field.ErrorList
}

// readTree reads every kustomization file under root, then decodes every
// other .yaml and .yml file, in walk order, but one those files name as
// data and none names as a resource. A symbolic link, and a file named as
// Flux's generator names its backup, are findings on the tree (refuse).
func readTree(root string) (*tree, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	tr := &tree{root: abs}
	var kfiles, manifests []string
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := tr.refuse(path, d); err != nil {
			return err
		}
		switch {
		case d.IsDir():
		case slices.Contains(konfig.RecognizedKustomizationFileNames(), d.Name()):
			kfiles = append(kfiles, path)
		case filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml":
			manifests = append(manifests, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	data, resources := map[string]bool{}, map[string]bool{}
	for _, path := range kfiles {
		raw, err := os.ReadFile(path) //nolint:gosec // a kustomization file under the directory named
		if err != nil {
			return nil, err
		}
		kf := &kustomizationFile{path: path}
		if err := yaml.Unmarshal(raw, &kf.k); err != nil {
			return nil, errors.Wrap(err, path)
		}
		tr.kustomizations = append(tr.kustomizations, kf)
		dir := filepath.Dir(path)
		for _, e := range kf.dataEntries() {
			data[filepath.Join(dir, e.file)] = true
		}
		for _, r := range kf.k.Resources {
			resources[filepath.Join(dir, r)] = true
		}
	}
	for _, path := range manifests {
		// a file named as data is not decoded — unless a kustomization file
		// also names it as a resource, which kustomize applies as a manifest
		if data[path] && !resources[path] {
			continue
		}
		raw, err := os.ReadFile(path) //nolint:gosec // a manifest under the directory named
		if err != nil {
			return nil, err
		}
		decoded, err := decode(raw)
		if err != nil {
			return nil, errors.Wrap(err, path)
		}
		for i, u := range decoded {
			if u.GroupVersionKind().Group != kustomizeGroup {
				tr.objects = append(tr.objects, &object{doc: path + ": " + describe(i, u), u: u})
			}
		}
	}
	return tr, nil
}

// original reports whether name is one Flux's generator keeps its backup
// under: a recognized kustomization file name with .original appended.
func original(name string) bool {
	base, ok := strings.CutSuffix(name, ".original")
	return ok && slices.Contains(konfig.RecognizedKustomizationFileNames(), base)
}

// refuse records a finding on an entry of the tree that no build may run
// over: a symbolic link — the copy the builds run on keeps it as a link,
// and Flux's generator and kustomize write through one — or an entry named
// as Flux's generator names its backup, which it overwrites and then
// consumes. The entry is still read as what its name says, as kustomize
// reads it, so every other check runs.
func (tr *tree) refuse(path string, d fs.DirEntry) error {
	link := d.Type()&fs.ModeSymlink != 0
	if !link && !original(d.Name()) {
		return nil
	}
	rel, err := filepath.Rel(tr.root, path)
	if err != nil {
		return err
	}
	at := field.NewPath(rel)
	if !link {
		tr.errs = append(tr.errs, field.Forbidden(at, anOriginal))
		return nil
	}
	target, err := os.Readlink(path)
	if err != nil {
		return err
	}
	tr.errs = append(tr.errs, field.Invalid(at, target, aLink))
	return nil
}

// entry is one path a kustomization file names, where it names it, and the
// file part of it.
type entry struct {
	at   *field.Path
	file string
}

// dataEntries returns the files the kustomization reads as data rather than
// as objects: generator sources and patch files. A files entry is
// [key=]path; the path is what names the file.
func (kf *kustomizationFile) dataEntries() []entry {
	var out []entry
	sources := func(at *field.Path, s kusttypes.KvPairSources) {
		for i, f := range s.FileSources {
			if _, p, ok := strings.Cut(f, "="); ok {
				f = p
			}
			out = append(out, entry{at.Child("files").Index(i), f})
		}
		for i, e := range s.EnvSources {
			out = append(out, entry{at.Child("envs").Index(i), e})
		}
		// the singular form kustomize folds into envs when it loads the file
		if s.EnvSource != "" {
			out = append(out, entry{at.Child("env"), s.EnvSource})
		}
	}
	for i, g := range kf.k.ConfigMapGenerator {
		sources(field.NewPath("configMapGenerator").Index(i), g.KvPairSources)
	}
	for i, g := range kf.k.SecretGenerator {
		sources(field.NewPath("secretGenerator").Index(i), g.KvPairSources)
	}
	for i, p := range kf.k.Patches {
		if p.Path != "" {
			out = append(out, entry{field.NewPath("patches").Index(i).Child("path"), p.Path})
		}
	}
	return out
}

// unresolved names the fields of the kustomization that load something the
// pass does not resolve, in the order the type declares them.
func (kf *kustomizationFile) unresolved() []string {
	k := kf.k
	var out []string
	for _, f := range []struct {
		name string
		set  bool
	}{
		// a deprecated field is read because kustomize still loads it
		{"openapi", len(k.OpenAPI) > 0},
		{"patchesStrategicMerge", len(k.PatchesStrategicMerge) > 0}, //nolint:staticcheck // deprecated, still loaded
		{"patchesJson6902", len(k.PatchesJson6902) > 0},             //nolint:staticcheck // deprecated, still loaded
		{"replacements", len(k.Replacements) > 0},
		{"crds", len(k.Crds) > 0},
		{"bases", len(k.Bases) > 0}, //nolint:staticcheck // deprecated, still loaded
		{"helmGlobals", k.HelmGlobals != nil},
		{"helmCharts", len(k.HelmCharts) > 0},
		{"helmChartInflationGenerator", len(k.HelmChartInflationGenerator) > 0},
		{"configurations", len(k.Configurations) > 0},
		{"generators", len(k.Generators) > 0},
		{"transformers", len(k.Transformers) > 0},
		{"validators", len(k.Validators) > 0},
	} {
		if f.set {
			out = append(out, f.name)
		}
	}
	return out
}

// notLocal is the finding on an entry kustomize would fetch.
const notLocal = "not a local path, so nothing is built: kustomize would fetch it"

// aLink is the finding on a symbolic link under root.
const aLink = "a symbolic link, which a build would write through, so nothing is built"

// anOriginal is the finding on a file named as Flux's generator names the
// backup it keeps.
const anOriginal = "Flux's generator keeps its backup under this name and consumes it, so nothing is built"

// isLocal reports whether kustomize reads entry from the tree rather than
// fetching it: Flux's own test, narrowed by the scp-style user@host:path and
// the file:// base that pass it and that kustomize clones all the same. The
// narrowing is a directory's — a Flux Kustomization's components, and a
// kustomization file's entry that names a directory or nothing — where a
// file so named is read before any clone (readsFile).
func isLocal(entry string) bool {
	return fluxkustomize.IsLocalRelativePath(entry) && !strings.ContainsAny(entry, ":@")
}

// readsFile reports whether entry — a kustomization file's entry that
// isLocal refuses, joined to that file's directory as target — is read from
// the tree after all: kustomize tries an entry as a file before it parses
// it as a repository to clone, so a file in the tree is read whatever its
// name. A directory, or nothing, under that name is parsed first, as
// isLocal has it, and a form Flux's own test refuses is not looked for.
func (tr *tree) readsFile(entry, target string) (bool, error) {
	if !fluxkustomize.IsLocalRelativePath(entry) {
		return false, nil
	}
	inTree, err := within(tr.root, target)
	if err != nil || !inTree {
		return false, err
	}
	info, err := os.Stat(target)
	switch {
	case stderrors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

// checkPaths resolves every path the tree names, then builds what each Flux
// Kustomization applies — unless a path could take a build past the tree,
// or the tree holds an entry a build would write through (refuse).
func (tr *tree) checkPaths() error {
	contained := len(tr.errs) == 0
	for _, o := range tr.objects {
		if o.kustomization == nil {
			continue
		}
		if err := tr.checkSpecPath(o); err != nil {
			return err
		}
		for i, c := range o.kustomization.Spec.Components {
			if !isLocal(c) {
				o.errs = append(o.errs, field.Invalid(field.NewPath("spec", "components").Index(i), c, notLocal))
				contained = false
			}
		}
	}
	for _, kf := range tr.kustomizations {
		ok, err := tr.checkEntries(kf)
		if err != nil {
			return err
		}
		contained = contained && ok
	}
	if !contained {
		return nil
	}
	return tr.build()
}

// checkSpecPath records the directory a Flux Kustomization's spec.path names
// in the tree, or a finding when it names none. An empty path and ./ are the
// root.
func (tr *tree) checkSpecPath(o *object) error {
	at := field.NewPath("spec", "path")
	path := o.kustomization.Spec.Path
	dir := filepath.Clean(path)
	if !filepath.IsLocal(dir) {
		o.errs = append(o.errs, field.Invalid(at, path, "not a path inside the tree"))
		return nil
	}
	info, err := os.Stat(filepath.Join(tr.root, dir))
	switch {
	case stderrors.Is(err, fs.ErrNotExist):
		o.errs = append(o.errs, field.NotFound(at, path))
	case err != nil:
		return err
	case !info.IsDir():
		o.errs = append(o.errs, field.Invalid(at, path, "not a directory"))
	default:
		o.dir = dir
	}
	return nil
}

// names is what an entry of a kustomization file may name.
type names int

const (
	fileOrDirectory names = iota // a resources entry
	directory                    // a components entry
	file                         // a generator source or a patch
)

// checkEntries resolves every entry of a kustomization file, and reports
// whether every one is a local path and every field one the pass resolves,
// so that a build reads nothing past the tree.
func (tr *tree) checkEntries(kf *kustomizationFile) (bool, error) {
	contained := true
	check := func(at *field.Path, e string, n names) error {
		local, err := tr.checkEntry(kf, at, e, n)
		contained = contained && local
		return err
	}
	for i, r := range kf.k.Resources {
		if err := check(field.NewPath("resources").Index(i), r, fileOrDirectory); err != nil {
			return false, err
		}
	}
	for i, c := range kf.k.Components {
		if err := check(field.NewPath("components").Index(i), c, directory); err != nil {
			return false, err
		}
	}
	for _, e := range kf.dataEntries() {
		if err := check(e.at, e.file, file); err != nil {
			return false, err
		}
	}
	for _, name := range kf.unresolved() {
		kf.errs = append(kf.errs, field.Forbidden(field.NewPath(name), "the pass does not resolve what this field loads, so nothing is built"))
		contained = false
	}
	return contained, nil
}

// checkEntry records a finding unless the entry e, at its place in kf, is a
// local path in the tree to what n allows, and reports whether it is a
// local path. A file stays in or below kf's directory; a directory holds a
// kustomization file.
func (tr *tree) checkEntry(kf *kustomizationFile, at *field.Path, e string, n names) (bool, error) {
	base := filepath.Dir(kf.path)
	target := filepath.Join(base, e)
	if !isLocal(e) {
		reads, err := tr.readsFile(e, target)
		if err != nil {
			return false, err
		}
		if !reads {
			kf.errs = append(kf.errs, field.Invalid(at, e, notLocal))
			return false, nil
		}
	}
	inTree, err := within(tr.root, target)
	if err != nil {
		return false, err
	}
	if !inTree {
		kf.errs = append(kf.errs, field.Invalid(at, e, "outside the tree"))
		return true, nil
	}
	info, err := os.Stat(target)
	if stderrors.Is(err, fs.ErrNotExist) {
		kf.errs = append(kf.errs, field.NotFound(at, e))
		return true, nil
	}
	if err != nil {
		return false, err
	}
	problem := ""
	switch {
	case info.IsDir() && n == file:
		problem = "a directory, where a file is named"
	case info.IsDir():
		held, err := holdsKustomization(target)
		if err != nil {
			return false, err
		}
		if !held {
			problem = "a directory without a kustomization file"
		}
	case n == directory:
		problem = "a file, where a component is a directory"
	default:
		inDir, err := within(base, target)
		if err != nil {
			return false, err
		}
		if !inDir {
			problem = "a file outside the kustomization file's directory"
		}
	}
	if problem != "" {
		kf.errs = append(kf.errs, field.Invalid(at, e, problem))
	}
	return true, nil
}

// within reports whether target is base or below it.
func within(base, target string) (bool, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false, err
	}
	return filepath.IsLocal(rel), nil
}

// holdsKustomization reports whether dir holds a file kustomize recognizes
// as a kustomization file.
func holdsKustomization(dir string) (bool, error) {
	for _, name := range konfig.RecognizedKustomizationFileNames() {
		info, err := os.Stat(filepath.Join(dir, name))
		switch {
		case stderrors.Is(err, fs.ErrNotExist):
		case err != nil:
			return false, err
		case !info.IsDir():
			return true, nil
		}
	}
	return false, nil
}

// build applies every Flux Kustomization whose spec.path is a directory in
// the tree, in walk order, to one copy of the tree, as kustomize-controller
// does, and records a finding on each that Flux cannot generate a
// kustomization file for or that does not build. Each build leaves the copy
// as it found it, for the next: Flux's generator keeps the kustomization
// file it amends as .original, and CleanDirectory puts it back or removes
// the one it generated.
func (tr *tree) build() (err error) {
	work, err := os.MkdirTemp("", "kuretest-consistent-*")
	if err != nil {
		return err
	}
	defer func() {
		if rmErr := os.RemoveAll(work); err == nil {
			err = rmErr
		}
	}()
	if err = os.CopyFS(work, os.DirFS(tr.root)); err != nil {
		return err
	}
	fsys, err := securefs.MakeFsOnDiskSecure(work)
	if err != nil {
		return err
	}
	// the copy's path, in what Flux and kustomize say, is the tree's
	inTree := func(e error) string { return strings.ReplaceAll(e.Error(), work, tr.root) }
	at := field.NewPath("spec", "path")
	for _, o := range tr.objects {
		if o.dir == "" {
			continue
		}
		dir := filepath.Join(work, o.dir)
		action, genErr := fluxkustomize.NewGenerator(work, *o.u).WriteFile(dir, fluxkustomize.WithSaveOriginalKustomization())
		if genErr != nil {
			// nothing was written, or WriteFile removed it itself
			o.errs = append(o.errs, field.Invalid(at, o.kustomization.Spec.Path, "Flux cannot generate its kustomization file: "+inTree(genErr)))
			continue
		}
		buildErr := buildLikeFlux(fsys, dir)
		if err = fluxkustomize.CleanDirectory(dir, action); err != nil {
			return err
		}
		if buildErr != nil {
			o.errs = append(o.errs, field.Invalid(at, o.kustomization.Spec.Path, "the build fails: "+inTree(buildErr)))
		}
	}
	return nil
}

// buildLikeFlux is kustomize's build with the options of Flux's own — no
// load restriction, plugins off — and its recovery, since kustomize panics
// on some accidental data.
func buildLikeFlux(fsys filesys.FileSystem, dir string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("recovered from kustomize build panic: %v", r)
		}
	}()
	_, err = krusty.MakeKustomizer(&krusty.Options{
		LoadRestrictions: kusttypes.LoadRestrictionsNone,
		PluginConfig:     kusttypes.DisabledPluginConfig(),
	}).Run(fsys, dir)
	return err
}

// agree runs the reference and namespace checks over objs, which are also
// the tree the references resolve in.
func agree(objs []*object, declared []Declared) error {
	index := map[string]bool{}
	for _, d := range declared {
		index[key(d.kind, d.namespace, d.name)] = true
	}
	for _, o := range objs {
		ns := ""
		if namespaced(o.u) {
			ns = effectiveNamespace(o.u.GetNamespace())
		}
		index[key(o.u.GetKind(), ns, o.u.GetName())] = true
	}
	for _, o := range objs {
		if err := o.convert(); err != nil {
			return err
		}
		o.checkReferences(index)
		o.checkNamespaces(index)
	}
	return nil
}

// convert reads a Flux Kustomization or HelmRelease into its pinned type, so
// the fields the pass reads are named by the API, not by strings.
func (o *object) convert() error {
	gvk := o.u.GroupVersionKind()
	var into any
	switch {
	case gvk.Group == kustv1.GroupVersion.Group && gvk.Kind == kustv1.KustomizationKind:
		o.kustomization = &kustv1.Kustomization{}
		into = o.kustomization
	case gvk.Group == helmv2.GroupVersion.Group && gvk.Kind == helmv2.HelmReleaseKind:
		o.release = &helmv2.HelmRelease{}
		into = o.release
	default:
		return nil
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.u.Object, into); err != nil {
		return errors.Wrap(err, o.doc)
	}
	return nil
}

// checkReferences resolves every reference the object makes.
func (o *object) checkReferences(index map[string]bool) {
	referrer := effectiveNamespace(o.u.GetNamespace())
	spec := field.NewPath("spec")
	if k := o.kustomization; k != nil {
		o.resolve(index, spec.Child("sourceRef"), k.Spec.SourceRef.Kind, k.Spec.SourceRef.Namespace, k.Spec.SourceRef.Name, referrer)
		for i, d := range k.Spec.DependsOn {
			o.resolve(index, spec.Child("dependsOn").Index(i), kustv1.KustomizationKind, d.Namespace, d.Name, referrer)
		}
	}
	if r := o.release; r != nil {
		if ref := r.Spec.ChartRef; ref != nil {
			o.resolve(index, spec.Child("chartRef"), ref.Kind, ref.Namespace, ref.Name, referrer)
		}
		if c := r.Spec.Chart; c != nil {
			ref := c.Spec.SourceRef
			o.resolve(index, spec.Child("chart", "spec", "sourceRef"), ref.Kind, ref.Namespace, ref.Name, referrer)
		}
		for i, d := range r.Spec.DependsOn {
			o.resolve(index, spec.Child("dependsOn").Index(i), helmv2.HelmReleaseKind, d.Namespace, d.Name, referrer)
		}
	}
}

// resolve records a finding unless the reference names an indexed object. A
// reference without a namespace is in its referrer's.
func (o *object) resolve(index map[string]bool, at *field.Path, kind, namespace, name, referrer string) {
	if name == "" {
		o.errs = append(o.errs, field.Required(at.Child("name"), "the reference names nothing"))
		return
	}
	if namespace == "" {
		namespace = referrer
	}
	if k := key(kind, namespace, name); !index[k] {
		o.errs = append(o.errs, field.NotFound(at, k))
	}
}

// checkNamespaces holds the object's namespace, and a Kustomization's
// targetNamespace, to the namespaces that exist: kustomize-controller
// creates neither.
func (o *object) checkNamespaces(index map[string]bool) {
	exists := func(ns string) bool { return systemNamespaces[ns] || index[key("Namespace", "", ns)] }
	if ns := o.u.GetNamespace(); ns != "" && namespaced(o.u) && !exists(ns) {
		o.errs = append(o.errs, field.NotFound(field.NewPath("metadata", "namespace"), ns))
	}
	if k := o.kustomization; k != nil && k.Spec.TargetNamespace != "" && !exists(k.Spec.TargetNamespace) {
		o.errs = append(o.errs, field.NotFound(field.NewPath("spec", "targetNamespace"), k.Spec.TargetNamespace))
	}
}

// findings returns collect's findings on the tree's objects and
// kustomization files, then one on the tree itself when it has errors.
func (tr *tree) findings() []finding {
	out := collect(tr.objects, tr.kustomizations)
	if len(tr.errs) > 0 {
		out = append(out, finding{doc: tr.root, errs: tr.errs})
	}
	return out
}

// collect returns one finding per object, then per kustomization file, that
// has errors, in the order they were read.
func collect(objs []*object, kfiles []*kustomizationFile) []finding {
	var out []finding
	for _, o := range objs {
		if len(o.errs) > 0 {
			out = append(out, finding{doc: o.doc, errs: o.errs})
		}
	}
	for _, kf := range kfiles {
		if len(kf.errs) > 0 {
			out = append(out, finding{doc: kf.path, errs: kf.errs})
		}
	}
	return out
}

// key is how the pass knows an object: kind, namespace, name.
func key(kind, namespace, name string) string { return kind + "/" + namespace + "/" + name }

// effectiveNamespace reads an omitted namespace as default, as kustomize and
// the API server do.
func effectiveNamespace(ns string) string {
	if ns == "" {
		return metav1.NamespaceDefault
	}
	return ns
}

// namespaced reports whether the object's kind is namespaced, as the kinds
// registry says. A kind kure does not register reads as namespaced, as
// kustomize reads it; AssertValid names such a kind.
func namespaced(u *unstructured.Unstructured) bool {
	if info, ok := kubernetes.KindForAnyVersion(u.GetAPIVersion(), u.GetKind()); ok {
		return info.Namespaced
	}
	return true
}
