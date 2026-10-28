package fluxcd

import (
	"fmt"
	"path"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// waitInputs says what in a generated Kustomization's inputs asks it to wait:
// where an explicit Wait true comes from, and where health checks do. Empty
// strings ask for nothing. Under UmbrellasDoNotWait R1 refuses either on an
// umbrella (refuseUmbrellaWaitInputs). leafConflict is the refusal of a unit
// whose merged bundles resolve to different waits (unitWaitConflict), raised
// only when the unit turns out to be a leaf.
type waitInputs struct {
	wait         string
	checks       string
	leafConflict error
}

// bundleWaitInputs is what the bundles of one Kustomization (a unit's, or the
// one GenerateForBundle writes) ask: the first that sets Wait true, and the
// first that sets HealthChecks.
func bundleWaitInputs(bundles ...*stack.Bundle) waitInputs {
	var in waitInputs
	for _, b := range bundles {
		if b == nil {
			continue
		}
		if in.wait == "" && b.Wait != nil && *b.Wait {
			in.wait = fmt.Sprintf("bundle %q sets Wait true", b.GetPath())
		}
		if in.checks == "" && len(b.HealthChecks) > 0 {
			in.checks = fmt.Sprintf("bundle %q sets HealthChecks", b.GetPath())
		}
	}
	return in
}

// layoutWaitInputs is what a per-layout Kustomization's inputs ask: Wait true
// set on the layout, or else on the bundle that holds its application, which
// the layout inherits (layoutSettings). Such a Kustomization has no health
// checks.
func (p *integratedPlacement) layoutWaitInputs(l *layout.ManifestLayout) waitInputs {
	switch holder := p.holdingBundle(l); {
	case l.Wait != nil:
		if *l.Wait {
			return waitInputs{wait: fmt.Sprintf("layout %q sets Wait true", l.FullRepoPath())}
		}
	case holder != nil && holder.Wait != nil && *holder.Wait:
		return waitInputs{wait: fmt.Sprintf("bundle %q, which holds the application of layout %q, sets Wait true and the layout inherits it", holder.GetPath(), l.FullRepoPath())}
	}
	return waitInputs{}
}

// refuseUmbrellaWaitInputs refuses, under R1, an umbrella (what names it)
// whose inputs ask it to wait: the setting exists to prevent that wait, so it
// is not dropped without a word. Unset, under the setting, is what a caller
// leaves on an umbrella.
func refuseUmbrellaWaitInputs(what string, in waitInputs) error {
	const why = "under UmbrellasDoNotWait a Kustomization that applies Flux Kustomizations does not wait, so that it holds no kustomize-controller worker while its children need one; readiness flows through dependsOn onto its terminal leaves"
	if in.wait != "" {
		return errors.Wrapf(ErrUmbrellaWait, "%s applies Flux Kustomizations, and %s: %s; leave Wait unset there (each leaf then waits by default)", what, in.wait, why)
	}
	if in.checks != "" {
		return errors.Wrapf(ErrUmbrellaWait, "%s applies Flux Kustomizations, and %s: %s; move the health checks to a bundle without Children whose own Kustomization applies the objects", what, in.checks, why)
	}
	return nil
}

// umbrellasDoNotWait applies R1 and R3 of UmbrellasDoNotWait to the
// Kustomizations this pass placed, once every one sits where it stays and
// before the reconcile-order check, which then judges the rewritten specs.
// R2 was applied at generation (ResourceGenerator.waitFor).
//
// Umbrellas and terminal leaves are read from the tree as it is
// (kustomizationTree), over the Kustomizations this pass placed or kept: an
// application's Flux Kustomization is a leaf. One kept from an earlier
// integration is not rewritten. It is refused when it breaks R1 or R3: it
// was placed without the setting.
//
// All terminal sets are computed before any dependsOn is rewritten, from the
// dependencies as generated, so the order of the rewrite does not matter.
func (p *integratedPlacement) umbrellasDoNotWait(root *layout.ManifestLayout) error {
	if !p.gen.UmbrellasDoNotWait {
		return nil
	}
	t, err := indexKustomizationTree(root, generatedMembers(p.generated))
	if err == nil {
		t.readExistence(p.gen.AdditiveCELDependencyCheckOff)
		err = t.otherErr
	}
	if err == nil {
		err = t.keptDroppedErr()
	}
	if err != nil {
		return err
	}
	kept := func(sentinel error, k *kustv1.Kustomization, broken string) error {
		return errors.Wrapf(sentinel, "Flux Kustomization %q (spec.path %q) is kept from the tree, and %s: it was placed without UmbrellasDoNotWait; integrate a tree layout.WalkCluster returns", k.Name, k.Spec.Path, broken)
	}

	terminals := map[string][]string{}
	for _, k := range t.kusts {
		key := crKey(k.Namespace, k.Name)
		created := p.created[key]
		if !t.isUmbrella(key) {
			if created != nil && p.waitInputs[key].leafConflict != nil {
				return p.waitInputs[key].leafConflict
			}
			continue
		}
		if created == nil {
			switch {
			case k.Spec.Wait:
				return kept(ErrUmbrellaWait, k, "it applies Flux Kustomizations and sets wait")
			case len(k.Spec.HealthChecks) > 0:
				return kept(ErrUmbrellaWait, k, "it applies Flux Kustomizations and sets healthChecks")
			}
		} else {
			if err := refuseUmbrellaWaitInputs(fmt.Sprintf("Flux Kustomization %q (spec.path %q)", k.Name, k.Spec.Path), p.waitInputs[key]); err != nil {
				return err
			}
		}
		terms, err := t.terminals(key)
		if err != nil {
			return err
		}
		terminals[key] = terms
	}
	for _, c := range p.keptNodeDeps {
		if err := c.check(terminals); err != nil {
			return err
		}
	}
	// A health check is not rewritten: one naming an umbrella would be
	// satisfied once the umbrella applied, before its leaves are Ready. Nor is
	// a Flux Kustomization this pass did not place (an application's, or the
	// caller's), its dependsOn included. Only a check that waits for the
	// umbrella's readiness counts (checkedKustomizations): one on another
	// cluster does not; one a healthCheckExprs predicate decides does.
	for _, k := range t.kusts {
		key := crKey(k.Namespace, k.Name)
		if t.isUmbrella(key) {
			continue // R1: written without health checks, or refused above
		}
		for _, c := range checkedKustomizations(k) {
			if terms, umbrella := terminals[c]; umbrella {
				return errors.Wrapf(ErrUmbrellaDependency, "Flux Kustomization %q (spec.path %q) health-checks %q, which applies Flux Kustomizations: under UmbrellasDoNotWait %q does not wait, so it becomes Ready before what it applies, and a health check is not rewritten; health-check its terminal leaves %q instead", k.Name, k.Spec.Path, c, c, terms)
			}
		}
	}
	for _, key := range t.otherKeys {
		spec, err := typedKustomization(t.otherObjs[key])
		if err != nil {
			return err
		}
		for _, d := range append(t.deps(key), checkedKustomizations(spec)...) {
			if terms, umbrella := terminals[d]; umbrella {
				return errors.Wrapf(ErrUmbrellaDependency, "Flux Kustomization %q (spec.path %q), which this integration did not place, depends on or health-checks %q, which applies Flux Kustomizations: under UmbrellasDoNotWait %q does not wait, so it becomes Ready before what it applies, and the integration does not rewrite a Kustomization it did not place; name its terminal leaves %q instead", key, spec.Spec.Path, d, d, terms)
			}
		}
	}

	// rewritten lists every dependsOn edge R3 writes onto a terminal leaf, so
	// that the graph after the rewrite can be checked for a cycle it closes.
	type edge struct{ from, umbrella, to string }
	var rewritten []edge
	for _, k := range t.kusts {
		key := crKey(k.Namespace, k.Name)
		created := p.created[key]
		var deps []kustv1.DependencyReference
		seen := map[string]bool{}
		for _, d := range k.Spec.DependsOn {
			ns := d.Namespace
			if ns == "" {
				ns = k.Namespace
			}
			target := crKey(ns, d.Name)
			terms, umbrella := terminals[target]
			// An entry that waits only for the umbrella to exist
			// (specDeps) waits for nothing its wait would cover: kept.
			if !umbrella || (t.existenceOnly && d.ReadyExpr == "true") {
				if !seen[target] {
					seen[target] = true
					deps = append(deps, d)
				}
				continue
			}
			if created == nil {
				return kept(ErrUmbrellaDependency, k, fmt.Sprintf("its dependsOn names %q, which applies Flux Kustomizations", d.Name))
			}
			if t.within(target, key) {
				return errors.Wrapf(ErrUmbrellaDependency, "Flux Kustomization %q (spec.path %q) depends on %q, which applies it: written as a dependsOn on %q's terminal leaves, it would depend on itself", k.Name, k.Spec.Path, d.Name, d.Name)
			}
			for _, term := range terms {
				// Checked also when an earlier entry already names the
				// leaf: that entry was the caller's, judged by no check.
				rewritten = append(rewritten, edge{from: key, umbrella: target, to: term})
				if seen[term] {
					continue
				}
				seen[term] = true
				deps = append(deps, dependencyOn(k.Namespace, term))
			}
		}
		if created == nil {
			continue
		}
		created.Spec.DependsOn = deps
		if t.isUmbrella(key) {
			created.Spec.Wait = false
			created.Spec.HealthChecks = nil
		}
	}

	// A terminal leaf can become Ready only after the dependent applies: it
	// depends on the dependent, or it is created through it, directly or
	// through Kustomizations an application emits. The edge onto the umbrella
	// closed no cycle the reconcile-order check sees, but the edge onto the
	// leaf closes one. That check reads members only, so the cycle is refused
	// here, over every Kustomization of the tree, each read as its writer's
	// build makes it from the rewritten tree (builtView). A build that writes
	// a copy of the dependent under another name (a name prefix, say) copies
	// the rewritten dependsOn too, and the cycle may close through the copy
	// alone: the leaf then never becomes Ready, and the dependent, waiting for
	// it, never applies.
	if len(rewritten) == 0 {
		return nil
	}
	v, err := newBuiltView(t, p.gen.AdditiveCELDependencyCheckOff)
	if err != nil {
		return p.laterPatchRefusal(root, err)
	}
	for _, e := range rewritten {
		chain, err := v.orderPath("ready "+e.to, "apply "+e.from)
		if err != nil {
			return err
		}
		if chain != nil {
			return errors.Wrapf(ErrUmbrellaDependency, "Flux Kustomization %q depends on %q, written as a dependsOn on its terminal leaf %q, which cannot become Ready before %q applies (%s): the rewrite would close a cycle, and none of these Kustomizations would become Ready; remove the dependency",
				e.from, e.umbrella, e.to, e.from, strings.Join(chain, " -> "))
		}
		chain, err = v.cycleThrough("ready " + e.to)
		if err != nil {
			return err
		}
		if chain != nil {
			return errors.Wrapf(ErrUmbrellaDependency, "Flux Kustomization %q depends on %q, written as a dependsOn on its terminal leaf %q, which waits for itself (%s): %q would never become Ready, so %q would never apply; remove the dependency",
				e.from, e.umbrella, e.to, strings.Join(chain, " -> "), e.to, e.from)
		}
	}
	return nil
}

// orderPath returns a chain from node from to node to, both included, in the
// graph checkReconcileOrder judges ("ready K" and "apply K" for each
// Kustomization K), here over every Kustomization of the tree, or nil when
// there is none (orderGraph). "ready K" needs "apply K", what K's wait or its
// Kustomization health checks wait for (awaitedKustomizations), and the apply
// of the Kustomization certain to write each other object K health-checks,
// when it is the only one (objectWriters; a check with several writers adds
// no edge here); "apply K" needs every Kustomization K depends on Ready, and the member whose apply
// writes K applied (writer), also when another Kustomization's build writes K
// as well: that one is no alternate creator. Every Kustomization is read as
// its writer's build makes it (builtView); one that is no member waits for
// what its build writes only when that build is the tree's, and for nothing
// the tree holds otherwise.
func (v *builtView) orderPath(from, to string) ([]string, error) {
	return v.newOrderGraph().path(from, to, map[string]bool{})
}

// cycleThrough returns a chain from node n back to itself, both ends n, in
// the graph orderPath walks, or nil when n is on no cycle.
func (v *builtView) cycleThrough(n string) ([]string, error) {
	g := v.newOrderGraph()
	succ, err := g.next(n)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, m := range succ {
		rest, err := g.path(m, n, seen)
		if err != nil {
			return nil, err
		}
		if rest != nil {
			return append([]string{n}, rest...), nil
		}
	}
	return nil, nil
}

// path returns a chain from node from to node to, both included, through no
// node of seen, which it marks; nil when there is none.
func (g *orderGraph) path(from, to string, seen map[string]bool) ([]string, error) {
	if from == to {
		return []string{from}, nil
	}
	if seen[from] {
		return nil, nil
	}
	seen[from] = true
	succ, err := g.next(from)
	if err != nil {
		return nil, err
	}
	for _, m := range succ {
		rest, err := g.path(m, to, seen)
		if err != nil {
			return nil, err
		}
		if rest != nil {
			return append([]string{from}, rest...), nil
		}
	}
	return nil, nil
}

// writer names the member whose apply writes the Flux Kustomization with key,
// as creator does, but over what a build holds (buildScope), so that one an
// AppFileSingle application's file holds counts: "" when the Flux bootstrap
// applies it or no member does. The innermost wins.
func (t *kustomizationTree) writer(key string) string {
	return t.hostWriter(t.hostOf[key], key)
}

// hostWriter names the innermost member whose apply writes what layout host
// holds, other than self (the Kustomization asked about; "" for none): "" when
// the Flux bootstrap applies host or no member does.
func (t *kustomizationTree) hostWriter(host *layout.ManifestLayout, self string) string {
	if host == nil || slices.Contains(buildScope(t.root), host) {
		return ""
	}
	best, bestLen := "", -1
	for _, k := range t.kusts {
		other := crKey(k.Namespace, k.Name)
		if other == self {
			continue
		}
		if l := t.layoutAt[path.Clean(k.Spec.Path)]; l != nil && len(k.Spec.Path) > bestLen && slices.Contains(buildScope(l), host) {
			best, bestLen = other, len(k.Spec.Path)
		}
	}
	return best
}

// keptNodeDependencies is a node's Kustomization kept from the tree whose
// spec.dependsOn lacks what the node's DependsOn and NamedDependsOn ask for
// (checkKeptNodeCR). Under UmbrellasDoNotWait the check waits for the
// terminal sets: a dependsOn on an umbrella was written as one on its
// terminal leaves, so an earlier integration with the setting kept those.
type keptNodeDependencies struct {
	namespace string
	have      []string
	missing   []string
	refusal   error
}

// check refuses the kept Kustomization unless each missing dependency is an
// umbrella whose every terminal leaf the kept spec.dependsOn names. A missing
// dependency that is no umbrella is refused as without the setting; one on an
// umbrella wraps ErrUmbrellaDependency (R3).
func (c keptNodeDependencies) check(terminals map[string][]string) error {
	have := map[string]bool{}
	for _, h := range c.have {
		have[dependencyKey(c.namespace, h)] = true
	}
	for _, m := range c.missing {
		terms, umbrella := terminals[dependencyKey(c.namespace, m)]
		if !umbrella {
			return c.refusal
		}
		if slices.ContainsFunc(terms, func(term string) bool { return !have[term] }) {
			return errors.Wrapf(ErrUmbrellaDependency, "%s; %q applies Flux Kustomizations, so under UmbrellasDoNotWait a dependsOn on its terminal leaves %q also satisfies it", c.refusal.Error(), m, terms)
		}
	}
	return nil
}

// dependencyOn is a dependsOn entry, on a Kustomization in namespace, for
// the Kustomization with key (crKey): its name, with its namespace only when
// that is another one.
func dependencyOn(namespace, key string) kustv1.DependencyReference {
	ns, name, _ := strings.Cut(key, "/")
	if crKey(ns, name) == crKey(namespace, name) {
		return kustv1.DependencyReference{Name: name}
	}
	return kustv1.DependencyReference{Name: name, Namespace: ns}
}

// isUmbrellaBundle reports whether b has a child: GenerateForBundle's umbrella.
func isUmbrellaBundle(b *stack.Bundle) bool {
	return slices.ContainsFunc(b.Children, func(c *stack.Bundle) bool { return c != nil })
}
