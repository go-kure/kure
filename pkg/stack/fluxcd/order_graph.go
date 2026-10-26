package fluxcd

import (
	"path"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/internal/built"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// orderGraph is the graph checkReconcileOrder judges ("ready K" and "apply K"
// for each Kustomization K) over the Flux Kustomizations of a tree: the graph
// orderPath walks and checkTreeOrder judges. With others it follows every
// Kustomization of the tree, members and others alike, as UmbrellasDoNotWait
// reads them; without, a Kustomization that is no member waits for nothing
// here: what it waits for is its author's. Each node's successors are
// computed once.
type orderGraph struct {
	t       *kustomizationTree
	others  bool
	writers map[string]string
	succ    map[string][]string
}

func (t *kustomizationTree) newOrderGraph(others bool) *orderGraph {
	return &orderGraph{t: t, others: others, writers: map[string]string{}, succ: map[string][]string{}}
}

// next returns what node n waits for. "apply K" needs every Kustomization K
// depends on Ready, and the member whose apply writes K applied (writer).
// "ready K" needs "apply K"; what K's wait or its Kustomization health checks
// wait for (awaitedKustomizations); and, for each health check on an object
// that is no Flux Kustomization, the apply of the member whose build writes it
// (objectWriter): Flux fetches a checked object before it evaluates any
// healthCheckExprs entry, so an expression, whatever it says, still waits for
// the object to exist. Under wait Flux ignores health checks, and on another
// cluster (spec.kubeConfig) none names an object of the tree. Without others,
// a Kustomization that is no member has no successor.
func (g *orderGraph) next(n string) ([]string, error) {
	if out, ok := g.succ[n]; ok {
		return out, nil
	}
	t := g.t
	kind, key, _ := strings.Cut(n, " ")
	var out []string
	if !g.others && t.byKey[key] == nil {
		g.succ[n] = out
		return out, nil
	}
	if kind == "apply" {
		for _, d := range t.deps(key) {
			out = append(out, "ready "+d)
		}
		if w := g.writer(key); w != "" {
			out = append(out, "apply "+w)
		}
		g.succ[n] = out
		return out, nil
	}
	out = append(out, "apply "+key)
	k := t.byKey[key]
	obj := t.otherObjs[key]
	if base, ok := strings.CutSuffix(key, keptCopy); ok {
		obj = t.droppedObjs[base]
	} else if t.keptDropped(key) {
		obj = t.droppedObjs[key]
	}
	if k == nil && obj != nil {
		spec, err := typedKustomization(obj)
		if err != nil {
			return nil, err
		}
		k = spec
	}
	if k != nil {
		for _, x := range awaitedKustomizations(k, func() []string { return t.written(key) }) {
			out = append(out, "ready "+x)
		}
		if !k.Spec.Wait && k.Spec.KubeConfig == nil {
			for _, hc := range k.Spec.HealthChecks {
				if isFluxKustomizationCheck(hc.APIVersion, hc.Kind) {
					continue // read by awaitedKustomizations
				}
				w, err := t.objectWriter(hc.APIVersion, hc.Kind, hc.Namespace, hc.Name)
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
		w = g.t.writer(key)
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
// (builtItems), as written: its group, kind, namespace and name, and the
// layout that holds it.
type treeObject struct {
	group, kind, namespace, name string
	host                         *layout.ManifestLayout
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
			t.objects = append(t.objects, treeObject{group: apiGroup(apiVersion), kind: kind, namespace: obj.GetNamespace(), name: obj.GetName(), host: l})
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
// v1.9.5 internal/inventory/inventory.go:170-173). It is certain when every
// match sits in a directory exactly one member's build holds (hostWriters),
// the same member for all, and that member's build transforms nothing
// (transformsBuild). An object written with no namespace where the check
// names one, or one where the check names none, may be the checked object
// once applied; so may a configMapGenerator entry whose name the check's
// starts with, as kustomize appends a content hash. Either leaves the writer
// uncertain.
func (t *kustomizationTree) objectWriter(apiVersion, kind, namespace, name string) (string, error) {
	if err := t.objectIndex(); err != nil {
		return "", err
	}
	if apiVersion == "" {
		apiVersion = "apps/v1"
	}
	group := apiGroup(apiVersion)
	var hosts []*layout.ManifestLayout
	for _, o := range t.objects {
		if o.group != group || o.kind != kind || o.name != name {
			continue
		}
		if o.namespace != namespace {
			if o.namespace == "" || namespace == "" {
				return "", nil
			}
			continue
		}
		hosts = append(hosts, o.host)
	}
	if group == "" && kind == "ConfigMap" {
		for _, gen := range t.generators {
			if name == gen.name || strings.HasPrefix(name, gen.name+"-") {
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
	if writer == "" || transformsBuild(t.byKey[writer]) {
		return "", nil
	}
	return writer, nil
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
// members' edges only, each dependsOn as written and each health check whose
// object has a certain writer (objectWriter); with others, under
// UmbrellasDoNotWait, it follows the other Kustomizations of the tree as
// orderPath does. A cycle made only of Kustomizations the integration did not
// place is their author's, and is not refused here. The error names one
// cycle, as checkReconcileOrder does.
func (t *kustomizationTree) checkTreeOrder(others bool) error {
	g := t.newOrderGraph(others)
	var starts []string
	for _, k := range t.kusts {
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
				if g.t.byKey[key] != nil {
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
