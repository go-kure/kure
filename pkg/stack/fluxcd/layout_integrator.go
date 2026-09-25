package fluxcd

import (
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// LayoutIntegrator implements the workflow.LayoutIntegrator interface for Flux.
// It handles integration of Flux resources with manifest layouts.
//
// Placement (FluxIntegratedPerLayout vs FluxSeparate) is configured via
// layout.LayoutRules.FluxPlacement on each call. CreateLayoutWithResources
// normalizes FluxUnset to FluxSeparate before invoking the SourceRef
// validation gate, WalkCluster, and IntegrateWithLayout, so all three
// observers agree on the effective placement.
type LayoutIntegrator struct {
	// ResourceGenerator generates the Flux resources
	Generator *ResourceGenerator
}

// NewLayoutIntegrator creates a FluxCD layout integrator.
func NewLayoutIntegrator(generator *ResourceGenerator) *LayoutIntegrator {
	return &LayoutIntegrator{
		Generator: generator,
	}
}

// IntegrateWithLayout adds Flux resources to a manifest layout that
// layout.WalkCluster built from c.
//
// Placement is driven by rules.FluxPlacement. FluxUnset is treated as
// FluxSeparate to match DefaultLayoutRules and the walker's normalization.
//
// Every placement first indexes the layout's origins (layout.IndexOrigins): a
// hand-built, partial or other-cluster tree is refused rather than matched by
// name, and every Kustomization's spec.path is the directory of the layout
// that renders its bundle. Integrating the same layout again adds nothing: a
// CR already present with the same name and spec.path in the layout that
// would host it is kept; one with the same name elsewhere or with another
// path is an error.
func (li *LayoutIntegrator) IntegrateWithLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	if ml == nil || c == nil || c.Node == nil {
		return nil
	}

	rules = normalizeRulesPlacement(rules)

	// The integration's placement is the tree's: the writers decide from
	// each layout's own FluxPlacement whether a child is listed as a
	// directory or through its CR, so a tree walked with another placement
	// would apply directories twice or leave CRs unapplied. A tree that
	// already holds Flux Kustomizations placed outside its applications (an
	// earlier integration's, or a caller's) was placed by them, so it is not
	// re-placed; and a refused call puts the tree back as it was.
	if !isFluxPlacement(rules.FluxPlacement) {
		return errors.NewValidationError("fluxPlacement", string(rules.FluxPlacement), "LayoutRules",
			[]string{string(layout.FluxIntegratedPerLayout), string(layout.FluxIntegratedPerBundle), string(layout.FluxSeparate)})
	}
	if ml.FluxPlacement != rules.FluxPlacement {
		if placed := placedKustomization(ml); placed != "" {
			return errors.Errorf("layout %q already holds Flux Kustomization %q, placed for %q: it cannot be integrated again as %q", ml.FullRepoPath(), placed, ml.FluxPlacement, rules.FluxPlacement)
		}
	}
	restore := saveLayouts(ml)
	setPlacement(ml, rules.FluxPlacement)

	var err error
	if rules.FluxPlacement == layout.FluxSeparate {
		err = li.addSeparateFluxToLayout(ml, c)
	} else {
		// Both inline placements put Flux CRs in the tree. They differ only
		// in granularity: PerLayout emits a CR for every layout node (incl.
		// augmenter-added child layouts); PerBundle stops at bundle
		// boundaries and lets kustomize include a bundle's application
		// directories.
		err = li.addIntegratedFluxToLayout(ml, c, rules.FluxPlacement == layout.FluxIntegratedPerLayout)
	}
	if err != nil {
		restore()
	}
	return err
}

// saveLayouts records what an integration changes on l and every layout
// below it — placement, application file mode, resources and children — and
// returns a function that puts it back, so a refused call leaves the tree as
// the caller gave it.
func saveLayouts(l *layout.ManifestLayout) func() {
	type state struct {
		placement layout.FluxPlacement
		fileMode  layout.ApplicationFileMode
		resources []client.Object
		children  []*layout.ManifestLayout
	}
	saved := map[*layout.ManifestLayout]state{}
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if l == nil {
			return
		}
		saved[l] = state{l.FluxPlacement, l.ApplicationFileMode, slices.Clone(l.Resources), slices.Clone(l.Children)}
		for _, child := range l.Children {
			walk(child)
		}
	}
	walk(l)
	return func() {
		for l, st := range saved {
			l.FluxPlacement, l.ApplicationFileMode = st.placement, st.fileMode
			l.Resources, l.Children = st.resources, st.children
		}
	}
}

// placedKustomization returns the name of a Flux Kustomization in the tree
// under ml that no application emitted — one an earlier integration or the
// caller placed — or "" when there is none. A Kustomization an application
// emits is that application's output (OriginBundleObjects), not a placement.
func placedKustomization(ml *layout.ManifestLayout) string {
	emitted := map[client.Object]bool{}
	var collect func(l *layout.ManifestLayout)
	collect = func(l *layout.ManifestLayout) {
		for _, b := range l.OriginBundles() {
			for _, o := range l.OriginBundleObjects(b) {
				emitted[o] = true
			}
		}
		for _, c := range l.Children {
			if c != nil {
				collect(c)
			}
		}
	}
	collect(ml)
	var find func(l *layout.ManifestLayout) string
	find = func(l *layout.ManifestLayout) string {
		for _, r := range l.Resources {
			if _, ok := fluxKustomizationPath(r); ok && !emitted[r] {
				return r.GetName()
			}
		}
		for _, c := range l.Children {
			if c == nil {
				continue
			}
			if name := find(c); name != "" {
				return name
			}
		}
		return ""
	}
	return find(ml)
}

// isFluxPlacement reports whether p is one of the three placements.
func isFluxPlacement(p layout.FluxPlacement) bool {
	return p == layout.FluxSeparate || p == layout.FluxIntegratedPerLayout || p == layout.FluxIntegratedPerBundle
}

// setPlacement sets placement on l and every layout below it, augmenter
// children included.
func setPlacement(l *layout.ManifestLayout, placement layout.FluxPlacement) {
	if l == nil {
		return
	}
	l.FluxPlacement = placement
	for _, child := range l.Children {
		setPlacement(child, placement)
	}
}

// CreateLayoutWithResources creates a new layout that includes Flux resources.
//
// rules.FluxPlacement is normalized once at the top of this method
// (FluxUnset -> FluxSeparate) and the normalized rules are passed to the
// SourceRef validation gate, WalkCluster, and IntegrateWithLayout. This
// guarantees a single placement authority per call.
func (li *LayoutIntegrator) CreateLayoutWithResources(c *stack.Cluster, rules layout.LayoutRules) (*layout.ManifestLayout, error) {
	if c == nil {
		return nil, nil
	}

	// Fail fast on umbrella / disjointness / multi-package violations before
	// we walk the tree.
	if err := stack.ValidateCluster(c); err != nil {
		return nil, err
	}

	rules = normalizeRulesPlacement(rules)

	// Both inline modes emit bundle/node Flux CRs that carry a spec.sourceRef,
	// so both require every reachable bundle to have a valid SourceRef.
	if rules.FluxPlacement == layout.FluxIntegratedPerLayout ||
		rules.FluxPlacement == layout.FluxIntegratedPerBundle {
		if err := validateSourceRefsForFluxIntegrated(c); err != nil {
			return nil, err
		}
	}

	// Generate the base manifest layout first
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		return nil, errors.ResourceValidationError("Cluster", c.Name, "layout",
			fmt.Sprintf("failed to create base layout: %v", err), err)
	}

	// Integrate Flux resources into the layout
	if err := li.IntegrateWithLayout(ml, c, rules); err != nil {
		return nil, errors.ResourceValidationError("Cluster", c.Name, "flux-integration",
			fmt.Sprintf("failed to integrate Flux resources: %v", err), err)
	}

	return ml, nil
}

// integratedPlacement is one pass of inline placement over a walked layout.
type integratedPlacement struct {
	gen       *ResourceGenerator
	ix        *layout.OriginIndex
	perLayout bool
	// names maps every Kustomization name this pass emitted to its
	// spec.path: Flux Kustomizations share one namespace, so a name is an
	// identity.
	names map[string]string
	// existing maps every Kustomization already in the tree (an earlier
	// integration's, or a caller's) to where it sits.
	existing map[string]existingCR
	// generated is the namespace/name of every Kustomization this pass
	// placed: the reconcile-order check covers these.
	generated map[string]bool
}

// existingCR is a Kustomization found in the tree before this pass.
type existingCR struct {
	host *layout.ManifestLayout
	path string
}

// sourceScope is the SourceRef a layout's subtree sources its layout CRs
// from: that of the nearest layout (itself or an ancestor) rendering bundles.
type sourceScope struct {
	ref kustv1.CrossNamespaceSourceReference
}

// addIntegratedFluxToLayout places Flux Kustomizations alongside their target
// manifests in one walk over the layout tree.
//
// Each unit's CR (and Source) is generated with spec.path = the directory of
// the layout that renders its bundles, and hosted in the parent of that layout
// (the root hosts its own), under both placements: a unit's directory is
// applied by its own Kustomization only, so its CR cannot live inside it, and
// no parent lists it (the writers skip a child that renders bundles).
//
// PerLayout also gives every child layout that is not an umbrella child, not
// AppFileSingle and renders no bundle (application, augmenter and bundle-less
// node layouts) a CR in its parent, so the writer lists every child of a
// PerLayout layout as a CR file. A bundle-less node layout's CR is named
// <path with "/" replaced by "-">-node: with ClusterName "." node web's path is
// "web", which is also the name of its bundle's CR.
func (li *LayoutIntegrator) addIntegratedFluxToLayout(ml *layout.ManifestLayout, c *stack.Cluster, perLayout bool) error {
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		return err
	}
	p := &integratedPlacement{
		gen:       li.Generator,
		ix:        ix,
		perLayout: perLayout,
		names:     map[string]string{},
		generated: map[string]bool{},
	}
	existing, err := indexExistingKustomizations(ml, nil)
	if err != nil {
		return err
	}
	p.existing = existing
	if err := p.place(ml, sourceScope{}); err != nil {
		return err
	}
	return checkPlacedReconcileOrder(ml, p.generated)
}

// checkPlacedReconcileOrder runs checkReconcileOrder over the Kustomizations
// this integration placed (generated: their namespace/name keys), with the
// creation rule integrated placement adds: a CR exists only once the
// Kustomization whose directory references reach its file has applied, and a
// CR the root directory reaches is created by the Flux bootstrap. Under
// separate placement every CR sits in flux-system, which the root lists, so
// GenerateFromLayout's own check is the whole story.
func checkPlacedReconcileOrder(root *layout.ManifestLayout, generated map[string]bool) error {
	var kusts []*kustv1.Kustomization
	hostOf := map[string]*layout.ManifestLayout{}
	layoutAt := map[string]*layout.ManifestLayout{}
	var index func(l *layout.ManifestLayout)
	index = func(l *layout.ManifestLayout) {
		layoutAt[path.Clean(l.FullRepoPath())] = l
		for _, obj := range l.Resources {
			if k, ok := obj.(*kustv1.Kustomization); ok && generated[crKey(k.Namespace, k.Name)] {
				kusts = append(kusts, k)
				hostOf[crKey(k.Namespace, k.Name)] = l
			}
		}
		for _, child := range l.Children {
			if child != nil {
				index(child)
			}
		}
	}
	index(root)

	// reach is the set of layouts a kustomization build of l includes: l and
	// the child directories its kustomization.yaml lists, recursively. The
	// writers list a child unless it is an umbrella child or renders bundles
	// (a unit, applied by its own CR only), an AppFileSingle file, or its
	// parent is PerLayout (which lists the child's CR instead).
	var reach func(l *layout.ManifestLayout, into map[*layout.ManifestLayout]bool)
	reach = func(l *layout.ManifestLayout, into map[*layout.ManifestLayout]bool) {
		into[l] = true
		for _, child := range l.Children {
			if child == nil || child.UmbrellaChild || child.ApplicationFileMode == layout.AppFileSingle ||
				len(child.OriginBundles()) > 0 || l.FluxPlacement == layout.FluxIntegratedPerLayout {
				continue
			}
			reach(child, into)
		}
	}
	fromRoot := map[*layout.ManifestLayout]bool{}
	reach(root, fromRoot)
	reached := map[string]map[*layout.ManifestLayout]bool{}
	creator := func(key string) string {
		host := hostOf[key]
		if fromRoot[host] {
			return ""
		}
		best, bestLen := "", -1
		for _, k := range kusts {
			other := crKey(k.Namespace, k.Name)
			l := layoutAt[path.Clean(k.Spec.Path)]
			if other == key || l == nil {
				continue
			}
			if reached[other] == nil {
				reached[other] = map[*layout.ManifestLayout]bool{}
				reach(l, reached[other])
			}
			if reached[other][host] && len(k.Spec.Path) > bestLen {
				best, bestLen = other, len(k.Spec.Path)
			}
		}
		return best
	}
	// applied lists the generated CRs a Kustomization's apply writes: those
	// its directory's references reach. With wait it waits for all of them,
	// whoever else (the bootstrap) also creates them.
	applied := func(key string) []string {
		k := set(kusts)[key]
		l := layoutAt[path.Clean(k.Spec.Path)]
		if l == nil {
			return nil
		}
		if reached[key] == nil {
			reached[key] = map[*layout.ManifestLayout]bool{}
			reach(l, reached[key])
		}
		var out []string
		for _, other := range kusts {
			ok := crKey(other.Namespace, other.Name)
			if ok != key && reached[key][hostOf[ok]] {
				out = append(out, ok)
			}
		}
		return out
	}
	return checkReconcileOrder(kusts, creator, applied)
}

// set indexes Kustomizations by namespace/name.
func set(kusts []*kustv1.Kustomization) map[string]*kustv1.Kustomization {
	out := make(map[string]*kustv1.Kustomization, len(kusts))
	for _, k := range kusts {
		out[crKey(k.Namespace, k.Name)] = k
	}
	return out
}

// checkReconcileOrder refuses a set of Flux Kustomizations kure generated that
// can never all become Ready. Each has two states, applied and Ready: applying
// waits for every dependsOn to be Ready; Ready waits for being applied and for
// every Kustomization it health-checks — unless wait is set, when Flux ignores
// health checks and waits for everything the Kustomization applied (applied,
// if set, lists the generated CRs among it). creator, if set, names the
// Kustomization whose apply creates a CR. Identities are namespace/name;
// references to objects outside the set (other namespaces, Kustomizations an
// application emits) are not modelled. A cycle is a deadlock on a fresh install: a merge can close one
// (a health check or dependency on a bundle merged into a unit that waits for
// it), and so can a dependency chain ending at a CR its first member creates.
func checkReconcileOrder(kusts []*kustv1.Kustomization, creator func(key string) string, applied func(key string) []string) error {
	set := set(kusts)
	edges := map[string][]string{}
	for _, k := range kusts {
		key := crKey(k.Namespace, k.Name)
		ready, apply := "ready "+key, "apply "+key
		edges[ready] = append(edges[ready], apply)
		for _, d := range k.Spec.DependsOn {
			ns := d.Namespace
			if ns == "" {
				ns = k.Namespace
			}
			if dep := crKey(ns, d.Name); set[dep] != nil {
				edges[apply] = append(edges[apply], "ready "+dep)
			}
		}
		if !k.Spec.Wait {
			for _, hc := range k.Spec.HealthChecks {
				if hc.Kind != "Kustomization" || !strings.HasPrefix(hc.APIVersion, kustv1.GroupVersion.Group+"/") {
					continue
				}
				ns := hc.Namespace
				if ns == "" {
					ns = k.Namespace
				}
				if checked := crKey(ns, hc.Name); set[checked] != nil {
					edges[ready] = append(edges[ready], "ready "+checked)
				}
			}
		}
		if k.Spec.Wait && applied != nil {
			for _, x := range applied(key) {
				edges[ready] = append(edges[ready], "ready "+x)
			}
		}
		if creator == nil {
			continue
		}
		if c := creator(key); c != "" {
			edges[apply] = append(edges[apply], "apply "+c)
		}
	}
	state := map[string]int{}
	var stack []string
	var visit func(n string) error
	visit = func(n string) error {
		switch state[n] {
		case 2:
			return nil
		case 1:
			i := slices.Index(stack, n)
			return errors.Errorf("Flux Kustomizations can never all become Ready: %s waits for itself (%s); a dependsOn waits before applying, a health check before becoming Ready, and a CR exists only once the Kustomization that creates it has applied",
				strings.TrimPrefix(strings.TrimPrefix(n, "ready "), "apply "), strings.Join(append(stack[i:], n), " -> "))
		}
		state[n] = 1
		stack = append(stack, n)
		for _, next := range edges[n] {
			if err := visit(next); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = 2
		return nil
	}
	for _, k := range kusts {
		key := crKey(k.Namespace, k.Name)
		for _, n := range []string{"ready " + key, "apply " + key} {
			if err := visit(n); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *integratedPlacement) place(l *layout.ManifestLayout, inherited sourceScope) error {
	scope := inherited
	bundles := l.OriginBundles()
	if len(bundles) > 0 {
		// The bundles l renders share one Kustomization, so one SourceRef:
		// generateForUnit refuses them otherwise.
		scope = sourceScope{ref: sourceRefOf(bundles[0])}
	}

	// One Kustomization per directory that renders bundles: the bundles a
	// grouping axis merged into l share it (generateForUnit).
	if len(bundles) > 0 {
		objs, err := p.gen.generateForUnit(l, p.ix)
		if err != nil {
			return errors.ResourceValidationError("Bundle", bundles[0].Name, "flux-resources",
				fmt.Sprintf("failed to generate Flux resources: %v", err), err)
		}
		if err := p.add(p.host(l), objs); err != nil {
			return err
		}
	}

	if p.perLayout {
		for _, child := range l.Children {
			if child == nil || child.UmbrellaChild || child.ApplicationFileMode == layout.AppFileSingle || len(child.OriginBundles()) > 0 {
				continue
			}
			name := layoutCRName(child)
			// The CR applies child's directory, so child must be written as
			// one: pinned here, a writer's Config-wide AppFileSingle cannot
			// turn it into a file in its parent (the layout's own mode wins).
			if child.ApplicationFileMode == layout.AppFileUnset {
				child.ApplicationFileMode = layout.AppFilePerResource
			}
			if e, ok := p.existing[crKey(p.gen.DefaultNamespace, name)]; ok && e.host == l && e.path == child.FullRepoPath() {
				// Placed by an earlier integration: kept as is, so its
				// source need not be resolved again. It is still one of
				// this integration's CRs for the reconcile-order check.
				if err := p.claim(name, e.path); err != nil {
					return err
				}
				p.generated[crKey(p.gen.DefaultNamespace, name)] = true
				continue
			}
			ref, err := p.layoutSource(child, scope)
			if err != nil {
				return err
			}
			cr := p.gen.createKustomizationForLayout(name, child, ref)
			if err := p.add(l, []client.Object{cr}); err != nil {
				return err
			}
		}
	}

	for _, child := range l.Children {
		if child == nil {
			continue
		}
		if err := p.place(child, scope); err != nil {
			return err
		}
	}
	return nil
}

// host returns the layout whose Resources receive the CR of bundle b, which
// layout l renders.
func (p *integratedPlacement) host(l *layout.ManifestLayout) *layout.ManifestLayout {
	if parent := p.ix.Parent(l); parent != nil {
		return parent
	}
	return l
}

// layoutSource returns the SourceRef of child's layout CR: the scope's (the
// nearest bundle-rendering layout at or above the host), else the one
// SourceRef the URL-less bundles below child share.
func (p *integratedPlacement) layoutSource(child *layout.ManifestLayout, scope sourceScope) (kustv1.CrossNamespaceSourceReference, error) {
	if scope.ref.Kind != "" && scope.ref.Name != "" {
		return scope.ref, nil
	}
	// Deduplicated by effective value: an omitted namespace is the
	// generator's DefaultNamespace, as it is for the Kustomization's own
	// sourceRef. The first reference is emitted as written.
	var refs []kustv1.CrossNamespaceSourceReference
	seen := map[kustv1.CrossNamespaceSourceReference]bool{}
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, b := range l.OriginBundles() {
			if b.SourceRef == nil || b.SourceRef.URL != "" {
				continue
			}
			ref := sourceRefOf(b)
			effective := ref
			if effective.Namespace == "" {
				effective.Namespace = p.gen.DefaultNamespace
			}
			if ref.Kind != "" && ref.Name != "" && !seen[effective] {
				seen[effective] = true
				refs = append(refs, ref)
			}
		}
		for _, c := range l.Children {
			if c != nil {
				walk(c)
			}
		}
	}
	walk(child)
	switch len(refs) {
	case 1:
		return refs[0], nil
	case 0:
		return kustv1.CrossNamespaceSourceReference{}, errors.ResourceValidationError("ManifestLayout", child.Name, "sourceRef",
			"FluxIntegratedPerLayout mode requires a SourceRef with Kind and Name; "+
				fmt.Sprintf("layout %q needs a Kustomization CR but no enclosing bundle and no bundle below it has one", child.FullRepoPath()), nil)
	default:
		return kustv1.CrossNamespaceSourceReference{}, errors.ResourceValidationError("ManifestLayout", child.Name, "sourceRef",
			fmt.Sprintf("layout %q has no enclosing bundle and the bundles below it have different SourceRefs, so its Flux Kustomization has no single source", child.FullRepoPath()), nil)
	}
}

// add appends objs to host.Resources. A Kustomization whose name this pass
// already emitted is an identity collision; one already present in host with
// the same spec.path is kept (a repeated integration adds nothing), with
// another spec.path it is an error. An identical Source already present is
// kept; a different one with its identity is an error.
func (p *integratedPlacement) add(host *layout.ManifestLayout, objs []client.Object) error {
	for _, obj := range objs {
		if k, ok := obj.(*kustv1.Kustomization); ok {
			if err := p.claim(k.Name, k.Spec.Path); err != nil {
				return err
			}
			p.generated[crKey(k.Namespace, k.Name)] = true
			if e, ok := p.existing[crKey(k.Namespace, k.Name)]; ok {
				if e.host == host && e.path == k.Spec.Path {
					continue
				}
				return errors.Errorf("layout %q already has Flux Kustomization %q with spec.path %q; this integration derives %q in layout %q", e.host.FullRepoPath(), k.Name, e.path, k.Spec.Path, host.FullRepoPath())
			}
		} else if same := findObject(host.Resources, obj); same != nil {
			// One identity, one object: an identical Source (a repeated
			// integration, or two bundles sharing a SourceRef) is kept once;
			// a different one would silently repoint a Kustomization.
			if sameObject(same, obj) {
				continue
			}
			return errors.Errorf("layout %q already has %s %q with different content: two SourceRefs name one Source differently", host.FullRepoPath(), obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		}
		host.Resources = append(host.Resources, obj)
	}
	return nil
}

// indexExistingKustomizations records every Flux Kustomization already in the
// tree under ml (an earlier integration's, a caller's, or one an application
// emits), typed or unstructured, keyed by namespace/name. skip, if set, is left
// out. One identity present twice is a collision before anything is added:
// kustomize would register the id twice.
func indexExistingKustomizations(ml, skip *layout.ManifestLayout) (map[string]existingCR, error) {
	out := map[string]existingCR{}
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		if l == nil || l == skip {
			return nil
		}
		for _, r := range l.Resources {
			path, ok := fluxKustomizationPath(r)
			if !ok {
				continue
			}
			key := crKey(r.GetNamespace(), r.GetName())
			if prev, dup := out[key]; dup {
				return errors.Errorf("Flux Kustomization name %q is present twice (in layout %q with spec.path %q and in layout %q with spec.path %q): Kustomization names must be unique", r.GetName(), prev.host.FullRepoPath(), prev.path, l.FullRepoPath(), path)
			}
			out[key] = existingCR{host: l, path: path}
		}
		for _, c := range l.Children {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	return out, walk(ml)
}

// fluxKustomizationPath reports whether obj is a Flux Kustomization — typed,
// or any object with its group and kind — and returns its spec.path.
func fluxKustomizationPath(obj client.Object) (string, bool) {
	if k, ok := obj.(*kustv1.Kustomization); ok {
		return k.Spec.Path, true
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Group != kustv1.GroupVersion.Group || gvk.Kind != "Kustomization" {
		return "", false
	}
	if u, ok := obj.(*unstructured.Unstructured); ok {
		path, _, _ := unstructured.NestedString(u.Object, "spec", "path")
		return path, true
	}
	return "", true
}

// crKey is a Kustomization's identity: Flux Kustomizations are namespaced,
// and an omitted namespace is "default", as Kubernetes and the writers'
// identity check read it.
func crKey(namespace, name string) string {
	if namespace == "" {
		namespace = "default"
	}
	return namespace + "/" + name
}

// claim records that this pass emits a Kustomization named name: a second
// claim of one name is a CR identity collision. The bare name is the key on
// purpose: every Kustomization this pass generates is placed in
// g.DefaultNamespace (kustomizationForBundle, createKustomizationForLayout),
// so name and namespace/name identify the same object here, and a bundle's
// name is its CR identity (IndexOrigins refuses two bundles with one name).
// Objects already in the tree may sit in other namespaces; those are keyed by
// crKey in indexExistingKustomizations.
func (p *integratedPlacement) claim(name, path string) error {
	if prev, dup := p.names[name]; dup {
		return errors.Errorf("Flux Kustomization name %q is used twice (spec.path %q and %q): Kustomization names must be unique", name, prev, path)
	}
	p.names[name] = path
	return nil
}

// layoutCRName names the PerLayout CR of a child layout that renders no
// bundle: a node layout gets "<path with / replaced by ->-node" (see
// addIntegratedFluxToLayout), any other layout its Name.
func layoutCRName(l *layout.ManifestLayout) string {
	if len(l.OriginNodes()) > 0 {
		return strings.ReplaceAll(l.FullRepoPath(), "/", "-") + "-node"
	}
	return l.Name
}

func sourceRefOf(b *stack.Bundle) kustv1.CrossNamespaceSourceReference {
	if b.SourceRef == nil {
		return kustv1.CrossNamespaceSourceReference{}
	}
	return kustv1.CrossNamespaceSourceReference{
		Kind:      b.SourceRef.Kind,
		Name:      b.SourceRef.Name,
		Namespace: b.SourceRef.Namespace,
	}
}

// findObject returns the object in resources with obj's kind, namespace and
// name, or nil.
func findObject(resources []client.Object, obj client.Object) client.Object {
	gvk := obj.GetObjectKind().GroupVersionKind()
	for _, r := range resources {
		if r.GetObjectKind().GroupVersionKind() == gvk && effectiveNamespace(r) == effectiveNamespace(obj) && r.GetName() == obj.GetName() {
			return r
		}
	}
	return nil
}

// sameObject reports whether a and b are the same object with the same
// content, reading an omitted namespace as "default".
func sameObject(a, b client.Object) bool {
	ca, okA := a.DeepCopyObject().(client.Object)
	cb, okB := b.DeepCopyObject().(client.Object)
	if !okA || !okB {
		return reflect.DeepEqual(a, b)
	}
	ca.SetNamespace(effectiveNamespace(ca))
	cb.SetNamespace(effectiveNamespace(cb))
	return reflect.DeepEqual(ca, cb)
}

// effectiveNamespace is obj's namespace as Kubernetes and the writers' identity
// check read it: an omitted one is "default".
func effectiveNamespace(obj client.Object) string {
	if ns := obj.GetNamespace(); ns != "" {
		return ns
	}
	return "default"
}

// addSeparateFluxToLayout creates a separate flux-system directory for Flux
// resources, generated from the layout itself (GenerateFromLayout) so every
// spec.path is a directory this layout writes. A flux-system child left by an
// earlier integration is kept when it holds the same resources and is an
// error when it does not.
func (li *LayoutIntegrator) addSeparateFluxToLayout(ml *layout.ManifestLayout, c *stack.Cluster) error {
	fluxResources, err := li.Generator.GenerateFromLayout(ml, c)
	if err != nil {
		return errors.ResourceValidationError("Cluster", c.Name, "flux-resources",
			fmt.Sprintf("failed to generate Flux resources: %v", err), err)
	}

	if len(fluxResources) == 0 {
		return nil
	}

	var fluxDir *layout.ManifestLayout
	for _, child := range ml.Children {
		if child != nil && child.Name == DefaultFluxDirName && child.OriginApplication() == nil &&
			len(child.OriginNodes()) == 0 && len(child.OriginBundles()) == 0 {
			fluxDir = child
		}
	}
	// The flux-system directory is applied beside the rest of the tree, so
	// a generated Kustomization's identity must not already be taken there
	// (an earlier flux-system child is compared as a whole below).
	existing, err := indexExistingKustomizations(ml, fluxDir)
	if err != nil {
		return err
	}
	for _, obj := range fluxResources {
		if _, ok := fluxKustomizationPath(obj); !ok {
			continue
		}
		if e, dup := existing[crKey(obj.GetNamespace(), obj.GetName())]; dup {
			return errors.Errorf("layout %q already has Flux Kustomization %q (spec.path %q); the generated one would register the same id in the kustomize build", e.host.FullRepoPath(), obj.GetName(), e.path)
		}
	}
	generated := map[string]bool{}
	for _, obj := range fluxResources {
		if k, ok := obj.(*kustv1.Kustomization); ok {
			generated[crKey(k.Namespace, k.Name)] = true
		}
	}
	if fluxDir != nil {
		// Kept only when it is exactly what this integration generates: the
		// same directory (the one ml's kustomization.yaml references), the
		// same resources and no layout beneath it (which the identity index
		// above leaves out with the child).
		if fluxDir.Namespace == ml.FullRepoPath() && len(fluxDir.Children) == 0 &&
			reflect.DeepEqual(fluxDir.Resources, fluxResources) {
			return checkPlacedReconcileOrder(ml, generated)
		}
		return errors.Errorf("layout %q already has a %s child with other Flux resources; integrate a freshly walked layout", ml.FullRepoPath(), DefaultFluxDirName)
	}

	// Create a separate Flux layout. The directory name is
	// [DefaultFluxDirName]; the file granularity is whatever
	// layout.DefaultLayoutRules declares rather than a second copy of it.
	//
	// Mode is left unset: the writer treats KustomizationUnset as
	// KustomizationExplicit, and this layout never gains children, so the
	// resource files are listed either way.
	//
	// Namespace is ml's own directory: ml's kustomization.yaml references
	// this layout as a child, so it must sit below ml. Joined onto
	// ml.Namespace instead, it landed beside ml when ml had a Name, and the
	// reference dangled (go-kure/kure#771).
	fluxLayout := &layout.ManifestLayout{
		Name:      DefaultFluxDirName,
		Namespace: ml.FullRepoPath(),
		FilePer:   layout.DefaultLayoutRules().FilePer,
		Resources: fluxResources,
	}

	ml.Children = append(ml.Children, fluxLayout)

	// GenerateFromLayout checked dependencies and health checks; the
	// placement adds what a wait on the root's Kustomization covers: the
	// root's kustomization.yaml lists flux-system, so it applies every CR.
	// A refused placement is taken back, so the tree is as the caller gave it.
	if err := checkPlacedReconcileOrder(ml, generated); err != nil {
		ml.Children = ml.Children[:len(ml.Children)-1]
		return err
	}
	return nil
}

// normalizeRulesPlacement returns a copy of rules with FluxPlacement filled in
// from layout.DefaultLayoutRules when it was FluxUnset. The integrator, the
// SourceRef validation gate, and the walker all read from this normalized value
// so they cannot disagree on what "unset" means.
//
// The default is read from layout.DefaultLayoutRules (pkg/stack/layout/types.go:154-163)
// rather than named here, because that function is where the layout package
// declares it and the walker resolves unset options from the same call
// (pkg/stack/layout/walker.go:42-43). A constant in this package would be a
// second copy of a value this package does not own.
func normalizeRulesPlacement(rules layout.LayoutRules) layout.LayoutRules {
	if rules.FluxPlacement == layout.FluxUnset {
		rules.FluxPlacement = layout.DefaultLayoutRules().FluxPlacement
	}
	return rules
}
