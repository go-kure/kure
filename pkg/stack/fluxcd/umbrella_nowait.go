package fluxcd

import (
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		err = t.otherErr
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
	// cluster, or one a healthCheckExprs predicate decides, does not.
	for _, k := range t.kusts {
		key := crKey(k.Namespace, k.Name)
		if t.isUmbrella(key) {
			continue // R1: written without health checks, or refused above
		}
		checks, _ := checkedKustomizations(k)
		for _, c := range checks {
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
		checks, _ := checkedKustomizations(spec)
		for _, d := range append(slices.Clone(t.otherDeps[key]), checks...) {
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
	// here, over every Kustomization of the tree.
	for _, e := range rewritten {
		chain, err := t.orderPath("ready "+e.to, "apply "+e.from)
		if err != nil {
			return err
		}
		if chain != nil {
			return errors.Wrapf(ErrUmbrellaDependency, "Flux Kustomization %q depends on %q, written as a dependsOn on its terminal leaf %q, which cannot become Ready before %q applies (%s): the rewrite would close a cycle, and none of these Kustomizations would become Ready; remove the dependency",
				e.from, e.umbrella, e.to, e.from, strings.Join(chain, " -> "))
		}
	}
	return nil
}

// orderPath returns a chain from node from to node to, both included, in the
// graph checkReconcileOrder judges ("ready K" and "apply K" for each
// Kustomization K), here over every Kustomization of the tree, or nil when
// there is none. "ready K" needs "apply K", and what K's wait or its
// Kustomization health checks wait for (awaitedKustomizations), and, for one
// a healthCheckExprs predicate decides, that it exists: the apply of the
// member that writes it; "apply K" needs every Kustomization K depends on
// Ready, and the member whose apply writes K applied (soleWriter: none when
// another Kustomization's build writes K as well). A Kustomization
// that is no member is read from its own spec, as its author wrote it; its
// wait waits for what its build writes only when that build is the tree's
// (readsLocalBuild), and for nothing the tree holds otherwise.
func (t *kustomizationTree) orderPath(from, to string) ([]string, error) {
	writers := map[string]string{}
	writer := func(key string) string {
		w, ok := writers[key]
		if !ok {
			w = t.soleWriter(key)
			writers[key] = w
		}
		return w
	}
	next := func(n string) ([]string, error) {
		kind, key, _ := strings.Cut(n, " ")
		var out []string
		if kind == "apply" {
			for _, d := range t.deps(key) {
				out = append(out, "ready "+d)
			}
			if w := writer(key); w != "" {
				out = append(out, "apply "+w)
			}
			return out, nil
		}
		out = append(out, "apply "+key)
		k := t.byKey[key]
		if k == nil && t.otherObjs[key] != nil {
			spec, err := typedKustomization(t.otherObjs[key])
			if err != nil {
				return nil, err
			}
			k = spec
		}
		if k == nil {
			return out, nil
		}
		ready, exist := awaitedKustomizations(k, func() []string { return t.written(key) })
		for _, x := range ready {
			out = append(out, "ready "+x)
		}
		for _, x := range exist {
			if w := writer(x); w != "" {
				out = append(out, "apply "+w)
			}
		}
		return out, nil
	}
	seen := map[string]bool{}
	var walk func(n string) ([]string, error)
	walk = func(n string) ([]string, error) {
		if n == to {
			return []string{n}, nil
		}
		if seen[n] {
			return nil, nil
		}
		seen[n] = true
		succ, err := next(n)
		if err != nil {
			return nil, err
		}
		for _, m := range succ {
			rest, err := walk(m)
			if err != nil {
				return nil, err
			}
			if rest != nil {
				return append([]string{n}, rest...), nil
			}
		}
		return nil, nil
	}
	return walk(from)
}

// writer names the member whose apply writes the Flux Kustomization with key,
// as creator does, but over what a build holds (buildScope), so that one an
// AppFileSingle application's file holds counts: "" when the Flux bootstrap
// applies it or no member does. The innermost wins.
func (t *kustomizationTree) writer(key string) string {
	host := t.hostOf[key]
	if host == nil || slices.Contains(buildScope(t.root), host) {
		return ""
	}
	best, bestLen := "", -1
	for _, k := range t.kusts {
		other := crKey(k.Namespace, k.Name)
		if other == key {
			continue
		}
		if l := t.layoutAt[path.Clean(k.Spec.Path)]; l != nil && len(k.Spec.Path) > bestLen && slices.Contains(buildScope(l), host) {
			best, bestLen = other, len(k.Spec.Path)
		}
	}
	return best
}

// soleWriter is writer, but "" when a Kustomization that is no member, whose
// build is the tree's (readsLocalBuild), also writes the one with key and
// applies without waiting for anything (writtenByOther): the one with key
// then exists once that one applies, and no check on it waits for either.
func (t *kustomizationTree) soleWriter(key string) string {
	w := t.writer(key)
	if w == "" || t.writtenByOther(key) {
		return ""
	}
	return w
}

// soleCreator is creator, but "" where soleWriter is: for the placed check,
// which reads creator.
func (t *kustomizationTree) soleCreator(key string) string {
	c := t.creator(key)
	if c == "" || t.writtenByOther(key) {
		return ""
	}
	return c
}

// writtenByOther reports whether a Kustomization that is no member, whose
// build is the tree's (localOthers), writes the one with key and applies
// without waiting for anything (appliesFreely).
func (t *kustomizationTree) writtenByOther(key string) bool {
	host := t.hostOf[key]
	if host == nil {
		return false
	}
	for _, o := range t.localOthers() {
		if o == key {
			continue
		}
		if l := t.layoutAt[path.Clean(t.otherSpecs[o].Spec.Path)]; l != nil && slices.Contains(buildScope(l), host) && t.appliesFreely(o) {
			return true
		}
	}
	return false
}

// appliesFreely reports whether the Kustomization with key o applies without
// waiting for anything: o, and each Kustomization whose apply writes the one
// before it (writer), is read in typed form, sets no field that may change
// what it applies (plainSpec) and depends on nothing, and the first that
// writer names no writer of is one the Flux bootstrap applies. Otherwise o may
// never apply, or write, the one after it, or wait, directly or not,
// for what it would create (o is written by that Kustomization's own apply,
// or depends on one that waits for it): the check then keeps the member's
// creation edge, and may refuse a tree that would converge.
func (t *kustomizationTree) appliesFreely(o string) bool {
	seen := map[string]bool{}
	for x := o; !seen[x]; x = t.writer(x) {
		seen[x] = true
		k := t.byKey[x]
		if k == nil {
			k = t.otherSpecs[x]
		}
		if k == nil || !plainSpec(k) || len(t.deps(x)) > 0 {
			return false
		}
		if t.writer(x) == "" {
			host := t.hostOf[x]
			return host != nil && slices.Contains(t.rootScope(), host)
		}
	}
	return false
}

// plainSpec reports whether k sets no field but those that change neither
// whether it applies nor which objects of its directory it applies:
// interval, retryInterval, timeout, path, sourceRef, prune, deletionPolicy,
// force, wait, healthChecks, healthCheckExprs and dependsOn (which
// appliesFreely reads itself). Any other field (suspend, commonMetadata,
// spec.ignore, postBuild, decryption, serviceAccountName, images, a transform
// of its build, ...) may keep it from writing what its directory holds, so
// the check does not count it as another writer.
func plainSpec(k *kustv1.Kustomization) bool {
	s := k.Spec
	s.Interval, s.RetryInterval, s.Timeout = metav1.Duration{}, nil, nil
	s.Path, s.SourceRef, s.Prune, s.DeletionPolicy, s.Force, s.Wait = "", kustv1.CrossNamespaceSourceReference{}, false, "", false, false
	s.HealthChecks, s.HealthCheckExprs, s.DependsOn = nil, nil, nil
	return reflect.DeepEqual(s, kustv1.KustomizationSpec{})
}

// localOthers lists, once per tree, the Kustomizations that are no members
// and whose build is the tree's (readsLocalBuild).
func (t *kustomizationTree) localOthers() []string {
	if t.local == nil {
		t.local = []string{}
		for _, o := range t.otherKeys {
			if t.readsLocalBuild(o) {
				t.local = append(t.local, o)
			}
		}
	}
	return t.local
}

// rootScope is buildScope of the root, once per tree: what the Flux bootstrap
// applies.
func (t *kustomizationTree) rootScope() []*layout.ManifestLayout {
	if t.scope == nil {
		t.scope = buildScope(t.root)
	}
	return t.scope
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
