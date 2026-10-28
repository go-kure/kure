package fluxcd

import (
	"context"
	"encoding/json"
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
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"

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
// Kustomizations holds it.
type treeBuilds struct {
	root    *layout.ManifestLayout
	dir     string
	err     error
	written bool
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
// kustomize-controller's build but for two steps, which no tree the writers
// write needs: SOPS decryption, which reads keys from the cluster, and the
// reset of kustomize's global OpenAPI schema around the build, which a
// kustomization that sets openapi needs and the writers never write; that
// reset would race any other kustomize build of the process. Like the
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
	fluxBuildMu.Lock()
	defer fluxBuildMu.Unlock()
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
	rm, err := kustomizeLikeFlux(fsys, dir)
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

// kustomizeLikeFlux runs kustomize as fluxcd/pkg/kustomize Build does
// (kustomize_generator.go:682-711) after its schema reset: no load
// restriction, built-in plugins only, a kustomize panic recovered as an
// error.
func kustomizeLikeFlux(fsys filesys.FileSystem, dir string) (rm resmap.ResMap, err error) {
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

// substituted is one object of a build after k's postBuild substitution:
// the object, and whether its namespace or name holds a variable whose value
// only the cluster knows (unknownIdentity).
type substituted struct {
	obj             client.Object
	unknownIdentity bool
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
//     unknown (unknownIdentity).
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
	inline := k.Spec.PostBuild.Substitute
	fromCluster := len(k.Spec.PostBuild.SubstituteFrom) > 0
	always := k.GetSubstituteStrategy() == kustv1.SubstituteStrategyAlways
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
		if fromCluster {
			unknownIdentity, err = refuseUnknownVars(k, res, inline)
			if err != nil {
				return nil, err
			}
		}
		res, err = fluxkustomize.SubstituteVariables(context.Background(), nil, unstructured.Unstructured{Object: kust}, res,
			fluxkustomize.SubstituteWithDryRun(true), fluxkustomize.SubstituteWithAlways(always))
		if err != nil {
			return nil, errors.Wrapf(err, "post build failed for %s %q", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		}
		m, err := res.Map()
		if err != nil {
			return nil, errors.Wrap(err, "read a substituted object")
		}
		out = append(out, substituted{obj: &unstructured.Unstructured{Object: m}, unknownIdentity: unknownIdentity})
	}
	return out, nil
}

// refuseUnknownVars applies substituteBuild's rule for the variables res
// references and inline does not set, under k's substituteFrom: an error for
// one in a Flux Kustomization or in an apiVersion or kind, and whether one is
// in the namespace or the name.
func refuseUnknownVars(k *kustv1.Kustomization, res *resource.Resource, inline map[string]string) (bool, error) {
	refuse := func(where string, vars []string) error {
		return errors.Errorf("Flux Kustomization %q (spec.path %q) substitutes with spec.postBuild.substituteFrom, and %s %q of its build references %s in %s, which spec.postBuild.substitute does not set: its value is read from the cluster, so what the object waits for cannot be read from the tree; set %s in spec.postBuild.substitute",
			k.Name, k.Spec.Path, res.GetKind(), res.GetName(), quoteAll(vars), where, strings.Join(vars, ", "))
	}
	for _, f := range []struct{ where, value string }{{"its apiVersion", res.GetApiVersion()}, {"its kind", res.GetKind()}} {
		vars, err := unknownVars(f.value, inline)
		if err != nil {
			return false, err
		}
		if len(vars) > 0 {
			return false, refuse(f.where, vars)
		}
	}
	if isFluxKustomizationType(res.GetApiVersion(), res.GetKind()) {
		text, err := res.AsYAML()
		if err != nil {
			return false, errors.Wrap(err, "read the object")
		}
		vars, err := unknownVars(string(text), inline)
		if err != nil {
			return false, err
		}
		if len(vars) > 0 {
			return false, refuse("the Kustomization", vars)
		}
		return false, nil
	}
	for _, s := range []string{res.GetNamespace(), res.GetName()} {
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
