package fluxcd

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/envsubst"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	securefs "github.com/fluxcd/pkg/kustomize/filesys"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/internal/kustomizeschema"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// treeBuilds builds Flux Kustomizations of one layout tree as
// kustomize-controller v1.10.1 builds them (go-kure/kure#1094): Flux's
// generator writes the Kustomization's patches, components, name prefix and
// suffix and target namespace into the kustomization.yaml of its spec.path
// (internal/controller/kustomization_controller.go:799-803), and kustomize
// builds that directory with Flux's options (:850, fluxcd/pkg/kustomize
// v1.44.0 kustomize_generator.go:661-711). The tree is written once, on the
// first build, by WriteToDisk into a temporary directory that close removes;
// a spec.path is a directory of that tree, as the source of the tree's
// Kustomizations holds it. merged lists, once each, the names of the
// secretGenerator entries with behavior merge in the kustomization files its
// builds read (openAPIGuard).
type treeBuilds struct {
	root    *layout.ManifestLayout
	dir     string
	err     error
	written bool
	merged  []string
}

func newTreeBuilds(root *layout.ManifestLayout) *treeBuilds {
	return &treeBuilds{root: root}
}

// close removes the directory the tree was written into, if any.
func (b *treeBuilds) close() {
	if b.dir != "" {
		_ = os.RemoveAll(b.dir)
		b.dir = ""
	}
}

// tree writes the tree on first use and returns the directory it is in.
func (b *treeBuilds) tree() (string, error) {
	if b.written {
		return b.dir, b.err
	}
	b.written = true
	dir, err := os.MkdirTemp("", "kure-flux-build-")
	if err != nil {
		b.err = errors.Wrap(err, "make a directory to build the tree in")
		return "", b.err
	}
	b.dir = dir
	if err := b.root.WriteToDisk(dir); err != nil {
		b.err = errors.Wrap(err, "the tree cannot be written, so what its Flux Kustomizations apply cannot be built")
	}
	return b.dir, b.err
}

// fluxBuildMu holds one build at a time: kustomize-controller's builds take
// one lock too (kustomize_generator.go:647-648, :685-687), and a build here
// writes the kustomization.yaml of a directory another build may read.
var fluxBuildMu sync.Mutex

// build returns the objects k's build makes, in the order kustomize emits
// them, each as the unstructured object the build holds. It is
// kustomize-controller's build, kustomize with fluxcd/pkg/kustomize's Build
// options under one lock, but for SOPS decryption, which reads keys from the
// cluster, and for that Build's reset of kustomize's global OpenAPI schema
// around the build (kustomize_generator.go:706-707). The reset writes the
// schema that every other kustomize caller in the process reads without a
// lock, so two integrations, or one beside any other use of kustomize, would
// race. It matters only where the schema is not kustomize's built-in one:
// the build is not read, with an unreadBuild, when another use of kustomize
// in the process set the schema, or when the build reads a kustomization
// file that sets openapi (a component a layout's extra files write may),
// which openAPIGuard refuses before kustomize reads its fields. Like the
// controller it refuses a built object without an apiVersion, kind or name
// (kustomization_controller.go:855-859).
func (b *treeBuilds) build(k *kustv1.Kustomization) ([]client.Object, error) {
	tree, err := b.tree()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(tree, filepath.FromSlash(path.Clean(k.Spec.Path)))
	if rel, err := filepath.Rel(tree, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.Errorf("spec.path %q is outside the tree", k.Spec.Path)
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		return nil, errors.Wrap(err, "convert the Kustomization to unstructured")
	}
	kustomizeschema.Parse()
	fluxBuildMu.Lock()
	defer fluxBuildMu.Unlock()
	if v, ok := kustomizeschema.BuiltIn(); !ok {
		return nil, unreadBuild{errors.Errorf("kustomize's OpenAPI schema in this process is %q, not its built-in one: another use of kustomize set it, and kustomize-controller builds with the built-in schema, so kure cannot build it as Flux does", v)}
	}
	manifest, kfile, _, err := fluxkustomize.NewGenerator(tree, unstructured.Unstructured{Object: content}).GenerateManifest(dir)
	if err != nil {
		return nil, errors.Wrap(err, "generate the kustomization.yaml of its spec.path")
	}
	restore, err := replaceFile(kfile, manifest)
	if err != nil {
		return nil, err
	}
	defer restore()
	fsys, err := securefs.MakeFsOnDiskSecure(tree)
	if err != nil {
		return nil, errors.Wrap(err, "open the tree")
	}
	guard := &openAPIGuard{FileSystem: fsys}
	rm, err := kustomizeBuild(guard, dir)
	for _, n := range guard.merged {
		if !slices.Contains(b.merged, n) {
			b.merged = append(b.merged, n)
		}
	}
	if guard.sets != "" {
		rel, _ := filepath.Rel(tree, guard.sets)
		return nil, unreadBuild{errors.Errorf("its build reads %s, which sets openapi: kustomize would keep that schema for every later build in the process, and kure, which does not reset it around each build as kustomize-controller does, cannot build it as Flux does; drop the field", filepath.ToSlash(rel))}
	}
	if err != nil {
		return nil, errors.Wrap(err, "kustomize build failed")
	}
	objs := make([]client.Object, 0, rm.Size())
	for _, res := range rm.Resources() {
		if res.GetName() == "" || res.GetKind() == "" || res.GetApiVersion() == "" {
			return nil, errors.Errorf("failed to decode Kubernetes apiVersion, kind and name from: %v", res.String())
		}
		m, err := res.Map()
		if err != nil {
			return nil, errors.Wrap(err, "read a built object")
		}
		objs = append(objs, &unstructured.Unstructured{Object: m})
	}
	return objs, nil
}

// unreadBuild is a build kure does not read, not one that fails: Flux may
// build and apply it, but kure cannot build it as Flux does.
type unreadBuild struct{ error }

// kustomizeBuild builds dir as fluxcd/pkg/kustomize v1.44.0's Build does
// after its schema reset (kustomize_generator.go:682-710): every load
// restriction lifted, plugins disabled, and a kustomize panic returned as an
// error.
func kustomizeBuild(fsys filesys.FileSystem, dir string) (rm resmap.ResMap, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("recovered from kustomize build panic: %v", r)
		}
	}()
	return krusty.MakeKustomizer(&krusty.Options{
		LoadRestrictions: kusttypes.LoadRestrictionsNone,
		PluginConfig:     kusttypes.DisabledPluginConfig(),
	}).Run(fsys, dir)
}

// openAPIGuard is the tree as a build reads it, but that it refuses a
// kustomization file setting openapi, and records it in sets, before
// kustomize reads its fields: kustomize sets the global OpenAPI schema from
// the field of every kustomization it loads (krusty/kustomizer.go:79-88,
// internal/target/kusttarget.go:511-520). kustomize reads a kustomization
// file it cannot load as one that is absent, so the build's own error does
// not name it; build reports sets instead. It also records in merged the
// names of the secretGenerator entries with behavior merge in each
// kustomization file the build reads, a component's included, and in no
// other: a file no build reads merges nothing. A file is a kustomization file
// by its name, so a generator's data file of that name that sets openapi is
// refused too, and its merge entries are recorded, though kustomize reads it
// only as data.
type openAPIGuard struct {
	filesys.FileSystem
	sets   string
	merged []string
}

func (g *openAPIGuard) ReadFile(name string) ([]byte, error) {
	b, err := g.FileSystem.ReadFile(name)
	if err != nil || !slices.Contains(konfig.RecognizedKustomizationFileNames(), filepath.Base(name)) {
		return b, err
	}
	var k kusttypes.Kustomization
	if yaml.Unmarshal(b, &k) == nil {
		if len(k.OpenAPI) > 0 {
			if g.sets == "" {
				g.sets = name
			}
			return nil, errors.Errorf("%s sets openapi", name)
		}
		for _, gen := range k.SecretGenerator {
			if gen.Behavior == kusttypes.BehaviorMerge.String() {
				g.merged = append(g.merged, gen.Name)
			}
		}
	}
	return b, nil
}

// replaceFile writes content to name and returns what puts back the file as
// it was, or removes it when there was none.
func replaceFile(name string, content []byte) (func(), error) {
	before, err := os.ReadFile(name)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "read %s", name)
	}
	if err := os.WriteFile(name, content, 0o644); err != nil {
		return nil, errors.Wrapf(err, "write %s", name)
	}
	return func() {
		if existed {
			_ = os.WriteFile(name, before, 0o644) //nolint:gosec // G703: name is a file of build's own temporary tree, which build checks spec.path stays in
		} else {
			_ = os.Remove(name)
		}
	}, nil
}

// substituted is one object of a build after k's postBuild substitution:
// the object, whether its namespace or name holds a variable whose value
// only the cluster knows (unknownIdentity), and, under a substituteFrom, the
// labels and annotations that say whether the apply skips it, as written
// (writtenLabels, writtenAnnotations; skipAsWritten).
type substituted struct {
	obj                               client.Object
	unknownIdentity                   bool
	writtenLabels, writtenAnnotations writtenSkip
}

// substituteBuild runs k's postBuild substitution over objs, the objects its
// build makes, as kustomize-controller runs it on each
// (kustomization_controller.go:876-881; fluxcd/pkg/kustomize
// kustomize_varsub.go:114-173): spec.postBuild.substitute, the strategy
// Always only where spec.postBuild.substituteStrategy says so, not strict,
// and Flux's opt-out (kustomize.toolkit.fluxcd.io/substitute: disabled, as a
// label or an annotation, compared exactly) leaving an object as built. It
// runs offline, in dry-run mode, which reads no cluster: a variable
// spec.postBuild.substituteFrom may set is unknown. With a substituteFrom,
// every variable an object references and spec.postBuild.substitute does not
// set is unknown, a default (${var:=default}) included, since the cluster
// may set it:
//
//   - in a Flux Kustomization, or in any object's apiVersion or kind (the
//     object may then be a Flux Kustomization), it is refused, naming the
//     variable: what that Kustomization waits for cannot be read
//     (go-kure/kure#1094, Q2);
//   - in another object's namespace or name it leaves that object's identity
//     unknown (unknownIdentity);
//   - in any key at an object's top level, or in an inline value with a line
//     break YAML reads that the object references, it is refused: either may
//     change what the object is at apply (refuseMovedStructure); in any key
//     of its metadata, which may become its labels or annotations, no skip
//     is read from them;
//   - whether the apply skips another object, which kustomize-controller
//     decides on the substituted object (kustomization_controller.go:876-891,
//     :913-927), is read from its kustomize.toolkit.fluxcd.io/ssa and
//     kustomize.toolkit.fluxcd.io/reconcile labels and annotations as
//     written, as main reads them, never as substituted: a skip holds only
//     where the substitution leaves it as written, none beside a key
//     holding a $ (skipAsWritten), and none on an object referencing any
//     variable inline leaves unset, whose text may change around a skip at
//     apply, whatever the cluster's value (a substring of an inline quote
//     with a cluster-set length, say).
//
// Without a postBuild objs is returned as it is.
func substituteBuild(k *kustv1.Kustomization, objs []client.Object) ([]substituted, error) {
	out := make([]substituted, 0, len(objs))
	if k.Spec.PostBuild == nil {
		for _, obj := range objs {
			out = append(out, substituted{obj: obj})
		}
		return out, nil
	}
	kust, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		return nil, errors.Wrap(err, "convert the Kustomization to unstructured")
	}
	// Flux removes every newline from an inline value before it substitutes
	// (kustomize_varsub.go:155), so the checks read the values it uses.
	inline := make(map[string]string, len(k.Spec.PostBuild.Substitute))
	for name, value := range k.Spec.PostBuild.Substitute {
		inline[name] = strings.ReplaceAll(value, "\n", "")
	}
	fromCluster := len(k.Spec.PostBuild.SubstituteFrom) > 0
	always := k.GetSubstituteStrategy() == kustv1.SubstituteStrategyAlways
	substitute := func(res *resource.Resource) (*resource.Resource, error) {
		return fluxkustomize.SubstituteVariables(context.Background(), nil, unstructured.Unstructured{Object: kust}, res,
			fluxkustomize.SubstituteWithDryRun(true), fluxkustomize.SubstituteWithAlways(always))
	}
	kustomizeschema.Parse()
	rf := provider.NewDefaultDepProvider().GetResourceFactory()
	for _, obj := range objs {
		content, err := writtenContent(obj)
		if err != nil {
			return nil, err
		}
		res, err := rf.FromMap(content)
		if err != nil {
			return nil, errors.Wrapf(err, "read %s %q", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		}
		if res.GetLabels()[substituteOptOut] == fluxkustomize.DisabledValue || res.GetAnnotations()[substituteOptOut] == fluxkustomize.DisabledValue {
			out = append(out, substituted{obj: obj})
			continue
		}
		unknownIdentity := false
		var writtenLabels, writtenAnnotations writtenSkip
		var written *resource.Resource
		if fromCluster {
			if err := refuseUnknownType(k, res, inline); err != nil {
				return nil, err
			}
			// The substitution rewrites res in place
			// (kustomize_varsub.go:165).
			written = res.DeepCopy()
			metadataKeyVar, err := refuseMovedStructure(k, written, inline)
			if err != nil {
				return nil, err
			}
			// An object referencing no variable inline leaves unset applies
			// as substituted here, since inline values override the
			// cluster's (kustomize_varsub.go:145-156), but for an escape $$
			// the apply may turn into $ where this run substitutes nothing,
			// which moves no entry; any other object may change at apply
			// around a skip, so none is read from it.
			text, err := written.AsYAML()
			if err != nil {
				return nil, errors.Wrap(err, "read the object")
			}
			objectVars, err := unknownVars(string(text), inline)
			if err != nil {
				return nil, err
			}
			if metadataKeyVar || len(objectVars) > 0 {
				writtenLabels, writtenAnnotations = writtenSkip{read: true}, writtenSkip{read: true}
			} else {
				writtenLabels, writtenAnnotations = skipAsWritten(written.GetLabels()), skipAsWritten(written.GetAnnotations())
			}
		}
		res, err = substitute(res)
		if err != nil {
			return nil, errors.Wrapf(err, "Flux Kustomization %q (spec.path %q): post build failed for %s %q", k.Name, k.Spec.Path, obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		}
		if fromCluster {
			if unknownIdentity, err = refuseUnknownVars(k, written, res, inline); err != nil {
				return nil, err
			}
		}
		m, err := res.Map()
		if err != nil {
			return nil, errors.Wrap(err, "read a substituted object")
		}
		out = append(out, substituted{obj: &unstructured.Unstructured{Object: m}, unknownIdentity: unknownIdentity,
			writtenLabels: writtenLabels, writtenAnnotations: writtenAnnotations})
	}
	return out, nil
}

// skipKeys are the label and annotation keys whose value says whether the
// apply skips an object (skipsApply).
var skipKeys = []string{"kustomize.toolkit.fluxcd.io/ssa", "kustomize.toolkit.fluxcd.io/reconcile"}

// writtenSkip is, under a substituteFrom, what an object's labels or
// annotations say, as written, before substitution, about whether the apply
// skips it, as main reads them: read marks it so, and values holds each skip
// key with its value as written, none beside a key holding a $
// (skipAsWritten).
type writtenSkip struct {
	read   bool
	values map[string]string
}

// over returns metadata, the substituted labels or annotations, where w is
// read with each skip key kept only where the substitution leaves it as
// written: a skip is read as written, and the substitution may only take one
// away, never add or change one.
func (w writtenSkip) over(metadata map[string]string) map[string]string {
	if !w.read {
		return metadata
	}
	out := maps.Clone(metadata)
	for _, key := range skipKeys {
		if value, ok := w.values[key]; !ok || out[key] != value {
			delete(out, key)
		}
	}
	return out
}

// skipAsWritten returns, under a substituteFrom, how metadata, an object's
// labels or annotations as written, says whether the apply skips it, as main
// reads it: each skip key with its value as written, never as substituted,
// since a variable may take any value the cluster sets and an inline value
// may carry YAML that moves one the cluster sets into a skip
// (go-kure/kure#1094). A value an inline variable sets is not read either;
// that errs towards a certain writer, and so towards refusing. A key holding
// a $ may become a skip key at apply and meet a written one, so no skip is
// read beside one.
func skipAsWritten(metadata map[string]string) writtenSkip {
	w := writtenSkip{read: true}
	for key := range metadata {
		if strings.Contains(key, "$") {
			return w
		}
	}
	for _, key := range skipKeys {
		if value, ok := metadata[key]; ok {
			if w.values == nil {
				w.values = map[string]string{}
			}
			w.values[key] = value
		}
	}
	return w
}

// refuseUnknownType applies substituteBuild's rule for a variable written's
// apiVersion or kind references and inline does not set, under k's
// substituteFrom: an error naming it, since the object may then be a Flux
// Kustomization.
func refuseUnknownType(k *kustv1.Kustomization, written *resource.Resource, inline map[string]string) error {
	for _, f := range []struct{ where, value string }{{"its apiVersion", written.GetApiVersion()}, {"its kind", written.GetKind()}} {
		vars, err := unknownVars(f.value, inline)
		if err != nil {
			return err
		}
		if len(vars) > 0 {
			return refuseUnknown(k, written, f.where, vars)
		}
	}
	return nil
}

// yamlBreaks are the line breaks YAML reads (go.yaml.in/yaml/v2 v2.4.4
// yamlprivateh.go:102-107) but Flux keeps in a value, removing only "\n"
// (kustomize_varsub.go:155, :199, :210).
const yamlBreaks = "\r\u0085\u2028\u2029"

// refuseMovedStructure applies substituteBuild's rules for what may move an
// object's structure at apply, under k's substituteFrom, to written, an
// object as written: an error where it references a variable inline sets to
// a value holding a line break YAML reads, which may put any text, a value
// the cluster sets included, at any place in the object; an error where a key
// at its top level references a variable, which may become its apiVersion,
// kind or metadata at apply; and whether a key of its metadata references a
// variable, which may become its labels or annotations at apply, so that no
// skip is read from them. Its name and namespace are still read as
// substituted, which errs towards a certain writer, and so towards refusing.
func refuseMovedStructure(k *kustv1.Kustomization, written *resource.Resource, inline map[string]string) (bool, error) {
	text, err := written.AsYAML()
	if err != nil {
		return false, errors.Wrap(err, "read the object")
	}
	vars, err := referencedVars(string(text), inline)
	if err != nil {
		return false, err
	}
	var broken []string
	for _, name := range vars {
		if v, ok := inline[name]; ok && strings.ContainsAny(v, yamlBreaks) {
			broken = append(broken, name)
		}
	}
	if len(broken) > 0 {
		return false, errors.Errorf("Flux Kustomization %q (spec.path %q) substitutes with spec.postBuild.substituteFrom, and %s %q of its build references %s, which spec.postBuild.substitute sets to a value holding a line break Flux keeps (a carriage return, U+0085, U+2028 or U+2029): YAML reads it as a new line, which may move a value the cluster sets anywhere in the object, so what the object is cannot be read from the tree; remove the line break",
			k.Name, k.Spec.Path, written.GetKind(), written.GetName(), quoteAll(broken))
	}
	m, err := written.Map()
	if err != nil {
		return false, errors.Wrap(err, "read the object")
	}
	for _, key := range slices.Sorted(maps.Keys(m)) {
		vars, err := referencedVars(key, inline)
		if err != nil {
			return false, err
		}
		if len(vars) > 0 {
			return false, errors.Errorf("Flux Kustomization %q (spec.path %q) substitutes with spec.postBuild.substituteFrom, and %s %q of its build has a key at its top level, %q, that references %s: Flux substitutes keys too, so it may become the object's apiVersion, kind or metadata, and what the object is cannot be read from the tree; write the key without a variable",
				k.Name, k.Spec.Path, written.GetKind(), written.GetName(), key, quoteAll(vars))
		}
	}
	metadata, _ := m["metadata"].(map[string]any)
	for key := range metadata {
		vars, err := referencedVars(key, inline)
		if err != nil {
			return false, err
		}
		if len(vars) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// referencedVars lists, in order and once each, the variables text
// references, those in a default included, used or not: envsubst evaluates
// each default before it reads the variable (fluxcd/pkg/envsubst v1.9.0
// template.go:113-132). An escaped one ($${var}) is none
// (parse/scan.go:214-221).
func referencedVars(text string, inline map[string]string) ([]string, error) {
	var vars []string
	_, err := envsubst.Eval(text, func(name string) (string, bool) {
		if !slices.Contains(vars, name) {
			vars = append(vars, name)
		}
		return inline[name], true
	})
	if err != nil {
		return nil, errors.Wrap(err, "parse the variables")
	}
	return vars, nil
}

// refuseUnknownVars applies substituteBuild's rule for the variables
// written, an object as written, references and inline does not set, under
// k's substituteFrom, once refuseUnknownType has passed it: an error for one
// in a Flux Kustomization, by the type applied, the object substituted and
// parsed as Flux parses it (kustomize_varsub.go:236-247), so a type an inline
// value makes is read as Flux reads it, quoted or not; and whether one is in
// the namespace or the name.
func refuseUnknownVars(k *kustv1.Kustomization, written, applied *resource.Resource, inline map[string]string) (bool, error) {
	if isFluxKustomizationType(applied.GetApiVersion(), applied.GetKind()) {
		text, err := written.AsYAML()
		if err != nil {
			return false, errors.Wrap(err, "read the object")
		}
		vars, err := unknownVars(string(text), inline)
		if err != nil {
			return false, err
		}
		if len(vars) > 0 {
			return false, refuseUnknown(k, written, "the Kustomization", vars)
		}
		return false, nil
	}
	for _, s := range []string{written.GetNamespace(), written.GetName()} {
		vars, err := unknownVars(s, inline)
		if err != nil {
			return false, err
		}
		if len(vars) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// refuseUnknown is the error for vars, variables where in written, an object
// of k's build, references, which k's spec.postBuild.substitute does not set.
func refuseUnknown(k *kustv1.Kustomization, written *resource.Resource, where string, vars []string) error {
	return errors.Errorf("Flux Kustomization %q (spec.path %q) substitutes with spec.postBuild.substituteFrom, and %s %q of its build references %s in %s, which spec.postBuild.substitute does not set: its value is read from the cluster, so what the object waits for cannot be read from the tree; set %s in spec.postBuild.substitute",
		k.Name, k.Spec.Path, written.GetKind(), written.GetName(), quoteAll(vars), where, strings.Join(vars, ", "))
}

// unknownVars lists, in order and once each, the variables text references
// that inline does not set, as Flux's envsubst parses it (fluxcd/pkg/envsubst
// v1.9.0 template.go:50-80, :132-157): an escaped one ($${var}) is none.
func unknownVars(text string, inline map[string]string) ([]string, error) {
	var vars []string
	_, err := envsubst.Eval(text, func(name string) (string, bool) {
		v, ok := inline[name]
		if !ok && !slices.Contains(vars, name) {
			vars = append(vars, name)
		}
		return v, true
	})
	if err != nil {
		return nil, errors.Wrap(err, "parse the variables")
	}
	return vars, nil
}

// isFluxKustomizationType reports whether apiVersion and kind are a Flux
// Kustomization's, of any version.
func isFluxKustomizationType(apiVersion, kind string) bool {
	return kind == "Kustomization" && apiGroup(apiVersion) == kustv1.GroupVersion.Group
}

// writtenContent returns obj as the writers write it: an unstructured
// object's map, a typed one's JSON. Both are copies.
func writtenContent(obj client.Object) (map[string]any, error) {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return runtime.DeepCopyJSON(u.Object), nil
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.Wrapf(err, "marshal %s %q", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
	}
	var content map[string]any
	if err := json.Unmarshal(data, &content); err != nil {
		return nil, errors.Wrapf(err, "decode %s %q", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
	}
	return content, nil
}

// quoteAll quotes each of names and joins them.
func quoteAll(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = `"` + n + `"`
	}
	return strings.Join(q, ", ")
}
