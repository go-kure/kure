package kuretest

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/konfig"
	kusttypes "sigs.k8s.io/kustomize/api/types"
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
// AssertConsistentYAML, as one set of objects.
//
// root is the directory the Flux source serves, the one every spec.path is
// relative to: for layout.WriteManifest(base, cfg, ml) that is
// filepath.Join(base, cfg.ManifestsDir), for WriteToDisk(base) it is base.
//
// A file kustomize recognizes as a kustomization file is read as its
// configuration, not as objects, and a file one names as data — a
// configMapGenerator or secretGenerator files, envs or env entry, a
// patches path — is not decoded. Every other manifest decodes or fails the
// test. A tree whose files hold no object fails.
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
	if findings := collect(tr.objects, tr.kustomizations); len(findings) > 0 {
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
	errs          field.ErrorList
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
}

// readTree reads every kustomization file under root, then decodes every
// other .yaml and .yml file but the data those files name, in walk order.
func readTree(root string) (*tree, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var kfiles, manifests []string
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
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
	tr := &tree{root: abs}
	data := map[string]bool{}
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
		for _, e := range kf.dataEntries() {
			data[filepath.Join(filepath.Dir(path), e.file)] = true
		}
	}
	for _, path := range manifests {
		if data[path] {
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
