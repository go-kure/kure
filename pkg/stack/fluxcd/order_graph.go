package fluxcd

import (
	"maps"
	"path"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/internal/built"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// orderGraph is the graph checkReconcileOrder judges ("ready K" and "apply K"
// for each Kustomization K) over the Flux Kustomizations of a tree, each read
// as its writer's build makes it (builtView): the graph orderPath walks and
// checkTreeOrder judges. With others it follows every Kustomization of the
// tree, members and others alike, as UmbrellasDoNotWait reads them; without,
// a Kustomization that is no member waits for nothing here: what it waits for
// is its author's. Each node's successors are computed once.
type orderGraph struct {
	v       *builtView
	others  bool
	writers map[string]string
	succ    map[string][]string
}

func (v *builtView) newOrderGraph(others bool) *orderGraph {
	return &orderGraph{v: v, others: others, writers: map[string]string{}, succ: map[string][]string{}}
}

// next returns what node n waits for. "apply K" needs every Kustomization K
// depends on Ready, and the writer whose apply creates K applied (writer).
// "ready K" needs "apply K"; what K's wait or its Kustomization health checks
// wait for (awaitedKustomizations; the wait leaves out what K's apply skips,
// unskipped); and, for each health check on an object
// that is no Flux Kustomization, the apply of the member whose build writes it
// (objectWriter): Flux fetches a checked object before it evaluates any
// healthCheckExprs entry, so an expression, whatever it says, still waits for
// the object to exist. Under wait Flux ignores health checks, and on another
// cluster (spec.kubeConfig) none names an object of the tree. Without others,
// a Kustomization that is no member has no successor; with, one that cannot
// be read in typed form has none either (builtView.spec).
func (g *orderGraph) next(n string) ([]string, error) {
	if out, ok := g.succ[n]; ok {
		return out, nil
	}
	v := g.v
	kind, key, _ := strings.Cut(n, " ")
	var out []string
	if !g.others && !v.isMember(key) {
		g.succ[n] = out
		return out, nil
	}
	k, err := v.spec(key)
	if err != nil {
		return nil, err
	}
	if kind == "apply" {
		if k != nil {
			for _, d := range specDeps(k) {
				out = append(out, "ready "+d)
			}
		}
		if w := g.writer(key); w != "" {
			out = append(out, "apply "+w)
		}
		g.succ[n] = out
		return out, nil
	}
	out = append(out, "apply "+key)
	if k != nil {
		for _, x := range awaitedKustomizations(k, func() []string { return v.unskipped(key, v.written(key)) }) {
			out = append(out, "ready "+x)
		}
		if !k.Spec.Wait && k.Spec.KubeConfig == nil {
			for _, hc := range k.Spec.HealthChecks {
				// Flux drops a check on any Kustomization, whatever its
				// group, named as the checker itself (kustomize-controller
				// v1.10.1 internal/controller/kustomization_controller.go:1118-1126).
				if hc.Kind == kustv1.KustomizationKind && hc.Name == k.Name && hc.Namespace == k.Namespace {
					continue
				}
				if isFluxKustomizationCheck(hc.APIVersion, hc.Kind) {
					continue // read by awaitedKustomizations
				}
				w, err := v.t.objectWriter(hc.APIVersion, hc.Kind, hc.Namespace, hc.Name)
				if err != nil {
					return nil, err
				}
				if w != "" {
					out = append(out, "apply "+w)
				}
			}
		}
	}
	g.succ[n] = out
	return out, nil
}

func (g *orderGraph) writer(key string) string {
	w, ok := g.writers[key]
	if !ok {
		w = g.v.writer(key, false)
		g.writers[key] = w
	}
	return w
}

// isFluxKustomizationCheck reports whether a health check on apiVersion and
// kind names a Flux Kustomization, as checkedKustomizations reads one.
func isFluxKustomizationCheck(apiVersion, kind string) bool {
	return kind == "Kustomization" && strings.HasPrefix(apiVersion, kustv1.GroupVersion.Group+"/")
}

// treeObject is one object a kustomize build of a layout of the tree holds
// (builtItems), as written: its group, kind, namespace and name, whether any
// of those, or its apiVersion, may change at apply (unresolved), whether its
// labels or annotations let the apply leave it uncreated (skipsApply), and
// the layout that holds it.
type treeObject struct {
	group, kind, namespace, name string
	unresolved, skipped          bool
	host                         *layout.ManifestLayout
}

// skipsApply reports whether metadata, an object's labels or annotations,
// lets kustomize-controller leave the object uncreated:
// kustomize.toolkit.fluxcd.io/ssa IfNotPresent or Ignore, or
// kustomize.toolkit.fluxcd.io/reconcile disabled, matched on a label or an
// annotation and without regard to case (fluxcd/pkg/ssa v0.83.0
// utils/meta.go:61-71).
func skipsApply(metadata map[string]string) bool {
	ssa := metadata["kustomize.toolkit.fluxcd.io/ssa"]
	return strings.EqualFold(ssa, "IfNotPresent") || strings.EqualFold(ssa, "Ignore") ||
		strings.EqualFold(metadata["kustomize.toolkit.fluxcd.io/reconcile"], "disabled")
}

// excludedFromApply reports whether metadata, an object's labels or
// annotations, makes kustomize-controller's apply skip the object whatever
// the cluster holds: kustomize.toolkit.fluxcd.io/ssa Ignore or
// kustomize.toolkit.fluxcd.io/reconcile disabled, without regard to case
// (kustomize-controller v1.10.1
// internal/controller/kustomization_controller.go:916-928; fluxcd/pkg/ssa
// v0.83.0 utils/meta.go:61-71). Unlike skipsApply it leaves out ssa
// IfNotPresent, which skips only an object that already exists: on a fresh
// install the apply creates that one, and its wait waits for it.
func excludedFromApply(metadata map[string]string) bool {
	return strings.EqualFold(metadata["kustomize.toolkit.fluxcd.io/ssa"], "Ignore") ||
		strings.EqualFold(metadata["kustomize.toolkit.fluxcd.io/reconcile"], "disabled")
}

// setOver returns base with every key of over set over it, as Flux sets
// spec.commonMetadata over an object (fluxcd/pkg/ssa v0.83.0
// utils/meta.go:27-55).
func setOver(base, over map[string]string) map[string]string {
	out := maps.Clone(base)
	if out == nil {
		out = map[string]string{}
	}
	maps.Copy(out, over)
	return out
}

// unresolved reports whether s, a field of an object's identity as written,
// may become another value at apply: postBuild substitution rewrites "$"
// variables, and SOPS decryption "ENC[" values. Neither is valid in a name.
func unresolved(s string) bool {
	return strings.Contains(s, "$") || strings.Contains(s, "ENC[")
}

// kubernetesCreated reports whether Kubernetes itself creates the object of
// group, kind and name in every namespace: the ServiceAccount default and the
// ConfigMap kube-root-ca.crt. A tree may legitimately write either, to add to
// it, so neither has a certain writer.
func kubernetesCreated(group, kind, name string) bool {
	return group == "" && (kind == "ServiceAccount" && name == "default" || kind == "ConfigMap" && name == "kube-root-ca.crt")
}

// treeGenerator is one configMapGenerator entry of a layout of the tree.
type treeGenerator struct {
	name string
	host *layout.ManifestLayout
}

// objectIndex reads, once, every object the layouts of the tree hold and
// every configMapGenerator entry, in depth-first layout order.
func (t *kustomizationTree) objectIndex() error {
	if t.objectsRead {
		return t.objectsErr
	}
	t.objectsRead = true
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		objs, err := builtItems(l)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			kind, apiVersion, err := built.WrittenType(obj)
			if err != nil {
				return errors.Wrapf(err, "layout %q: read an object as written", l.FullRepoPath())
			}
			t.objects = append(t.objects, treeObject{
				group: apiGroup(apiVersion), kind: kind, namespace: obj.GetNamespace(), name: obj.GetName(),
				unresolved: unresolved(apiVersion) || unresolved(kind) || unresolved(obj.GetNamespace()) || unresolved(obj.GetName()),
				skipped:    skipsApply(obj.GetLabels()) || skipsApply(obj.GetAnnotations()), host: l,
			})
		}
		for _, gen := range l.ConfigMapGenerators {
			t.generators = append(t.generators, treeGenerator{name: gen.Name, host: l})
		}
		for _, c := range l.Children {
			if c != nil {
				if err := walk(c); err != nil {
					return err
				}
			}
		}
		return nil
	}
	t.objectsErr = walk(t.root)
	return t.objectsErr
}

// apiGroup is the group of apiVersion: "" for the core group ("v1").
func apiGroup(apiVersion string) string {
	group, _, ok := strings.Cut(apiVersion, "/")
	if !ok {
		return ""
	}
	return group
}

// objectWriter names the member whose apply writes the object a health check
// on apiVersion, kind, namespace and name names, only when that is certain:
// "" otherwise, and the check adds no edge. A check that waits for an object
// another apply may also create does not wait for this one, and an edge
// that may not exist would refuse a tree that converges. It matches by group,
// kind, namespace and name as written, the version ignored as Flux resolves
// it; an empty apiVersion is apps/v1, as Flux reads it (kustomize-controller
// v1.10.1 internal/inventory/inventory.go:166-169). It is certain when every
// match sits in a directory exactly one member's build holds (hostWriters),
// the same member for all, that member's build transforms nothing
// (transformsBuild) and its spec.commonMetadata does not let the apply skip
// what it writes (skipsApply), and no other apply may write the object under
// an identity its file does not show (transformedProducer). A check that
// names no namespace, or a match written with none, leaves the writer
// uncertain: a namespaced object may be applied in another namespace than its
// file says, and a cluster-scoped one may exist before any apply. So do a
// match whose labels or annotations let the apply skip it (skipsApply); an
// object whose apiVersion, kind, namespace or name may change at apply
// (unresolved), whatever it names, unless only the writer's build holds it,
// as another apply may give it the checked identity; a configMapGenerator entry whose name the check's
// starts with, as kustomize appends a content hash, or that may change at
// apply; and an object Kubernetes creates itself (kubernetesCreated). An
// object a controller creates, from a HelmRelease's chart, say, or a build
// that is no directory of the tree, is not modelled: the check assumes an
// object the tree writes is created only by the applies of the tree's
// directories.
func (t *kustomizationTree) objectWriter(apiVersion, kind, namespace, name string) (string, error) {
	if err := t.objectIndex(); err != nil {
		return "", err
	}
	if apiVersion == "" {
		apiVersion = "apps/v1"
	}
	group := apiGroup(apiVersion)
	if kubernetesCreated(group, kind, name) {
		return "", nil
	}
	var hosts []*layout.ManifestLayout
	for _, o := range t.objects {
		if o.unresolved || o.group != group || o.kind != kind || o.name != name {
			continue
		}
		if o.namespace == "" {
			return "", nil
		}
		if o.namespace == namespace {
			if o.skipped {
				return "", nil
			}
			hosts = append(hosts, o.host)
		}
	}
	if group == "" && kind == "ConfigMap" {
		for _, gen := range t.generators {
			if name == gen.name || strings.HasPrefix(name, gen.name+"-") || unresolved(gen.name) {
				return "", nil
			}
		}
	}
	writer := ""
	for _, h := range hosts {
		ws, ok := t.hostWriters(h)
		if !ok || len(ws) != 1 || (writer != "" && ws[0] != writer) {
			return "", nil
		}
		writer = ws[0]
	}
	if writer == "" {
		return "", nil
	}
	for _, o := range t.objects {
		if o.unresolved {
			if ws, ok := t.hostWriters(o.host); !ok || len(ws) != 1 || ws[0] != writer {
				return "", nil
			}
		}
	}
	w := t.byKey[writer]
	if transformsBuild(w) || w.Spec.CommonMetadata != nil &&
		(skipsApply(w.Spec.CommonMetadata.Labels) || skipsApply(w.Spec.CommonMetadata.Annotations)) ||
		t.transformedProducer(group, kind) {
		return "", nil
	}
	return writer, nil
}

// transformedProducer reports whether another apply may write an object of
// group and kind under an identity its file does not show. That is so when a
// Kustomization of the tree on this cluster:
//   - cannot be read in typed form;
//   - builds a directory of the tree and has spec.components, which may add
//     any object;
//   - has spec.patches and its build holds any object or configMapGenerator
//     entry, local config included: a patch may change a kind, or keep what
//     the build would drop as local config (patchesBuild);
//   - otherwise transforms (transformsBuild: a name prefix or suffix, or a
//     target namespace) and its build holds an object of group and kind, or,
//     for a ConfigMap, a configMapGenerator entry.
func (t *kustomizationTree) transformedProducer(group, kind string) bool {
	ks := slices.Clone(t.kusts)
	for _, keys := range t.others {
		for _, key := range keys {
			k := t.otherSpecs[key]
			if k == nil {
				return true
			}
			if k.Spec.KubeConfig == nil {
				ks = append(ks, k)
			}
		}
	}
	for _, k := range ks {
		l := t.layoutAt[path.Clean(k.Spec.Path)]
		if l == nil || !transformsBuild(k) {
			continue
		}
		if len(k.Spec.Components) > 0 {
			return true
		}
		scope := buildScope(l)
		if len(k.Spec.Patches) > 0 {
			for _, s := range scope {
				if len(s.Resources) > 0 || len(s.ConfigMapGenerators) > 0 {
					return true
				}
			}
			continue
		}
		for _, o := range t.objects {
			if o.group == group && o.kind == kind && slices.Contains(scope, o.host) {
				return true
			}
		}
		if group == "" && kind == "ConfigMap" {
			for _, gen := range t.generators {
				if slices.Contains(scope, gen.host) {
					return true
				}
			}
		}
	}
	return false
}

// hostWriters lists the Flux Kustomizations of the tree whose build holds
// layout host: every member, and every other on this cluster, whatever its
// Source, whose spec.path names a directory whose build holds it. ok is false
// when that is not known: the Flux bootstrap applies host, or a Kustomization
// of the tree cannot be read in typed form, so its spec.path is unknown.
func (t *kustomizationTree) hostWriters(host *layout.ManifestLayout) ([]string, bool) {
	if host == nil || slices.Contains(buildScope(t.root), host) {
		return nil, false
	}
	holds := func(p string) bool {
		l := t.layoutAt[path.Clean(p)]
		return l != nil && slices.Contains(buildScope(l), host)
	}
	var out []string
	for _, k := range t.kusts {
		if holds(k.Spec.Path) {
			out = append(out, crKey(k.Namespace, k.Name))
		}
	}
	for _, keys := range t.others {
		for _, key := range keys {
			k := t.otherSpecs[key]
			if k == nil {
				return nil, false
			}
			if k.Spec.KubeConfig == nil && holds(k.Spec.Path) {
				out = append(out, key)
			}
		}
	}
	return out, true
}

// transformsBuild reports whether k's build may change what its directory
// holds, so that an object of it may be applied under another identity, or
// not at all: spec.patches, spec.components (which may carry patches),
// spec.namePrefix, spec.nameSuffix or spec.targetNamespace.
func transformsBuild(k *kustv1.Kustomization) bool {
	return k == nil || len(k.Spec.Patches) > 0 || len(k.Spec.Components) > 0 ||
		k.Spec.NamePrefix != "" || k.Spec.NameSuffix != "" || k.Spec.TargetNamespace != ""
}

// checkTreeOrder refuses a tree whose Flux Kustomizations can never all become
// Ready because of a cycle through a member: a strongly connected component of
// the order graph (orderGraph) that holds a cycle and a member's node. The
// graph is entered at the members' nodes. Without others it judges the
// members' edges only, each dependsOn as built and each health check whose
// object has a certain writer (objectWriter); with others, under
// UmbrellasDoNotWait, it follows the other Kustomizations of the tree as
// orderPath does. A cycle made only of Kustomizations the integration did not
// place is their author's, and is not refused here. The error names one
// cycle, as checkReconcileOrder does.
func (v *builtView) checkTreeOrder(others bool) error {
	g := v.newOrderGraph(others)
	var starts []string
	for _, k := range v.t.kusts {
		key := crKey(k.Namespace, k.Name)
		starts = append(starts, "ready "+key, "apply "+key)
	}
	comp, err := g.components(starts)
	if err != nil {
		return err
	}
	state := map[string]int{}
	var stack []string
	var visit func(n string) error
	visit = func(n string) error {
		switch state[n] {
		case 2:
			return nil
		case 1:
			if !comp[n].refused {
				return nil
			}
			i := slices.Index(stack, n)
			return errors.Errorf("Flux Kustomizations can never all become Ready: %s waits for itself (%s); a dependsOn waits before applying, a health check before becoming Ready, a Kustomization with wait becomes Ready only once every Kustomization it applied is, and a CR exists only once the Kustomization that creates it has applied",
				strings.TrimPrefix(strings.TrimPrefix(n, "ready "), "apply "), strings.Join(append(stack[i:], n), " -> "))
		}
		state[n] = 1
		stack = append(stack, n)
		succ, err := g.next(n)
		if err != nil {
			return err
		}
		for _, m := range succ {
			if err := visit(m); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = 2
		return nil
	}
	for _, n := range starts {
		if err := visit(n); err != nil {
			return err
		}
	}
	return nil
}

// component is a strongly connected component of the order graph: refused
// when it holds a cycle (more than one node, or a node waiting for itself)
// and a member's node.
type component struct {
	refused bool
}

// components returns the strongly connected component of every node reachable
// from starts (Tarjan's algorithm).
func (g *orderGraph) components(starts []string) (map[string]*component, error) {
	index := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	comp := map[string]*component{}
	var strongConnect func(n string) error
	strongConnect = func(n string) error {
		index[n] = len(index)
		low[n] = index[n]
		stack = append(stack, n)
		onStack[n] = true
		succ, err := g.next(n)
		if err != nil {
			return err
		}
		self := false
		for _, m := range succ {
			if m == n {
				self = true
			}
			if _, seen := index[m]; !seen {
				if err := strongConnect(m); err != nil {
					return err
				}
				low[n] = min(low[n], low[m])
			} else if onStack[m] {
				low[n] = min(low[n], index[m])
			}
		}
		if low[n] != index[n] {
			return nil
		}
		c := &component{}
		var nodes []string
		for {
			m := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[m] = false
			comp[m] = c
			nodes = append(nodes, m)
			if m == n {
				break
			}
		}
		if len(nodes) > 1 || self {
			for _, m := range nodes {
				_, key, _ := strings.Cut(m, " ")
				if g.v.isMember(key) {
					c.refused = true
					break
				}
			}
		}
		return nil
	}
	for _, n := range starts {
		if _, seen := index[n]; !seen {
			if err := strongConnect(n); err != nil {
				return nil, err
			}
		}
	}
	return comp, nil
}
