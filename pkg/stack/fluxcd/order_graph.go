package fluxcd

import (
	"maps"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// orderGraph is the graph checkReconcileOrder judges ("ready K" and "apply K"
// for each Kustomization K) over the Flux Kustomizations of a tree, each read
// as its writer's build makes it (builtView): the graph orderPath walks and
// checkTreeOrder judges. It follows every Kustomization of the tree, members
// and the others alike, each read from its writer's result, with or without
// UmbrellasDoNotWait. Each node's successors are computed once.
type orderGraph struct {
	v       *builtView
	writers map[string]string
	succ    map[string][]string
}

func (v *builtView) newOrderGraph() *orderGraph {
	return &orderGraph{v: v, writers: map[string]string{}, succ: map[string][]string{}}
}

// next returns what node n waits for. "apply K" needs every Kustomization K
// depends on Ready, or, for an entry that waits only for its dependency D to
// exist (specDeps), "exist D"; and the writer whose apply creates K applied
// (writer). "exist D" needs the writer whose apply creates D applied, and
// nothing when no Kustomization of the tree creates it: D keeps its own node
// so that a cycle through D's existence is judged as one through D.
// "ready K" needs "apply K"; what K's wait or its Kustomization health checks
// wait for (awaitedKustomizations; the wait leaves out what K's apply skips,
// unskipped); and, for each health check on an object that is no Flux
// Kustomization, the apply of the writer whose result holds it, when that
// writer is certain (objectWriter): Flux fetches a checked object before it evaluates any
// healthCheckExprs entry, so an expression, whatever it says, still waits for
// the object to exist. Under wait Flux ignores health checks, and on another
// cluster (spec.kubeConfig) none names an object of the tree. A dependsOn
// waits for the dependency's Ready also with a readyExpr: with the
// AdditiveCELDependencyCheck feature gate off, its default, Flux evaluates
// only the expression (kustomize-controller v1.10.1
// internal/features/features.go:97-99,
// internal/controller/kustomization_controller.go:614-626), but kure does not
// evaluate CEL and an expression may itself require Ready, so this read never
// admits a deadlock, and refuses a tree whose expression asks for less. With
// the gate on Flux checks Ready after the expression (:630-655), so the one
// exception, a readyExpr of exactly "true" as an existence wait, applies only
// where the caller states the gate is off
// (ResourceGenerator.AdditiveCELDependencyCheckOff); Flux requires the
// dependency to exist in either case (:597-611).
// A Kustomization that is no member and cannot be read in typed form, even
// from its writer's substituted result, has no successor: its writer's apply
// fails, a stall the checks do not judge (builtView.spec).
func (g *orderGraph) next(n string) ([]string, error) {
	if out, ok := g.succ[n]; ok {
		return out, nil
	}
	v := g.v
	kind, key, _ := strings.Cut(n, " ")
	var out []string
	if kind == "exist" {
		if w := g.writer(key); w != "" {
			out = append(out, "apply "+w)
		}
		g.succ[n] = out
		return out, nil
	}
	k, err := v.spec(key)
	if err != nil {
		return nil, err
	}
	if k == nil && !v.isMember(key) {
		g.succ[n] = out
		return out, nil
	}
	if kind == "apply" {
		if k != nil {
			for _, d := range specDeps(k, v.existenceOnly) {
				out = append(out, d.node())
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
				w, err := v.objectWriter(hc.APIVersion, hc.Kind, hc.Namespace, hc.Name)
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

// nodeKey is the key of the Kustomization a node of the reconcile-order graph
// names ("ready K", "apply K" or "exist K").
func nodeKey(n string) string {
	_, key, _ := strings.Cut(n, " ")
	return key
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

// apiGroup is the group of apiVersion: "" for the core group ("v1").
func apiGroup(apiVersion string) string {
	group, _, ok := strings.Cut(apiVersion, "/")
	if !ok {
		return ""
	}
	return group
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
// Ready because of a cycle through a judged Kustomization (judged): a strongly
// connected component of the order graph (orderGraph) that holds a cycle and
// a judged Kustomization's node. The graph is entered at those nodes and
// follows the other Kustomizations of the tree as orderPath does: each
// dependsOn as built and each health check whose object has a certain writer
// (objectWriter). A cycle made only of Kustomizations the integration did not
// place, under the identities the tree holds them by, is their author's, and
// is not refused here. The error names one cycle, as checkReconcileOrder does.
func (v *builtView) checkTreeOrder() error {
	g := v.newOrderGraph()
	var starts []string
	for _, k := range v.t.kusts {
		key := crKey(k.Namespace, k.Name)
		starts = append(starts, "ready "+key, "apply "+key)
	}
	for _, w := range v.writers {
		for _, key := range v.heldKeys(w) {
			if !v.isMember(key) && v.judged(key) && !slices.Contains(starts, "ready "+key) {
				starts = append(starts, "ready "+key, "apply "+key)
			}
		}
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
				nodeKey(n), strings.Join(append(stack[i:], n), " -> "))
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
// and a judged Kustomization's node.
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
				if g.v.judged(key) {
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
