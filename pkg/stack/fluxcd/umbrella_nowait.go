package fluxcd

import (
	"fmt"
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
			if !umbrella {
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
				if seen[term] {
					continue
				}
				seen[term] = true
				deps = append(deps, dependencyOn(k.Namespace, term))
				rewritten = append(rewritten, edge{from: key, umbrella: target, to: term})
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

	// A terminal leaf can itself depend, directly or through Kustomizations
	// an application emits, on the dependent: the edge onto the umbrella was
	// no cycle, since the umbrella holds no wait, but the edge onto the leaf
	// is one. The reconcile-order check that follows reads members only, so
	// the cycle is refused here, over every Kustomization of the tree.
	for _, e := range rewritten {
		if chain := t.dependencyPath(e.to, e.from); chain != nil {
			return errors.Wrapf(ErrUmbrellaDependency, "Flux Kustomization %q depends on %q, written as a dependsOn on its terminal leaf %q, which depends on %q in turn (%s): the rewrite would close a dependsOn cycle, and no Kustomization of it would become Ready; remove one of these dependencies",
				e.from, e.umbrella, e.to, e.from, strings.Join(chain, " -> "))
		}
	}
	return nil
}

// dependencyPath returns a chain of spec.dependsOn edges from the
// Kustomization with key from to the one with key to, both included, over
// every Kustomization of the tree (deps), or nil when there is none.
func (t *kustomizationTree) dependencyPath(from, to string) []string {
	seen := map[string]bool{}
	var walk func(k string) []string
	walk = func(k string) []string {
		if k == to {
			return []string{k}
		}
		if seen[k] {
			return nil
		}
		seen[k] = true
		for _, d := range t.deps(k) {
			if rest := walk(d); rest != nil {
				return append([]string{k}, rest...)
			}
		}
		return nil
	}
	return walk(from)
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
// umbrella whose every terminal leaf the kept spec.dependsOn names.
func (c keptNodeDependencies) check(terminals map[string][]string) error {
	have := map[string]bool{}
	for _, h := range c.have {
		have[dependencyKey(c.namespace, h)] = true
	}
	for _, m := range c.missing {
		terms, umbrella := terminals[dependencyKey(c.namespace, m)]
		if !umbrella || slices.ContainsFunc(terms, func(term string) bool { return !have[term] }) {
			return c.refusal
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
