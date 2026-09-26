package fluxcd

import (
	"context"
	"fmt"
	"maps"
	"path"
	"reflect"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/resid"

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
// below it — placement, application file mode, resources, children and the
// Flux build marker — and
// returns a function that puts it back, so a refused call leaves the tree as
// the caller gave it.
func saveLayouts(l *layout.ManifestLayout) func() {
	type state struct {
		placement layout.FluxPlacement
		fileMode  layout.ApplicationFileMode
		resources []client.Object
		children  []*layout.ManifestLayout
		fluxBuild bool
	}
	saved := map[*layout.ManifestLayout]state{}
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if l == nil {
			return
		}
		saved[l] = state{l.FluxPlacement, l.ApplicationFileMode, slices.Clone(l.Resources), slices.Clone(l.Children), l.FluxBuild()}
		for _, child := range l.Children {
			walk(child)
		}
	}
	walk(l)
	return func() {
		for l, st := range saved {
			l.FluxPlacement, l.ApplicationFileMode = st.placement, st.fileMode
			l.Resources, l.Children = st.resources, st.children
			l.SetFluxBuild(st.fluxBuild)
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
	gen *ResourceGenerator
	ix  *layout.OriginIndex
	// root is the layout the Flux bootstrap applies: the root node's, which
	// its sync path ./<root> names, and not a ClusterName wrapper above it.
	// It hosts every Source this pass derives (go-kure/kure#876).
	root      *layout.ManifestLayout
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
	// sources maps every Flux Source identity (sourceKey) in the tree — there
	// before this pass or placed by it — to each copy and the layout holding
	// it: one identity is one object across the whole pass, not per host.
	sources map[string][]hostedObject
	// derived is the sourceKey of every Source this pass derived, and placed
	// every copy it added to a layout (not those it found already there):
	// hostSourcesOncePerBuild keeps each derived Source once per build.
	derived map[string]bool
	placed  map[client.Object]bool
}

// hostedObject is an object and the layout whose Resources hold it (directly
// or inside a List).
type hostedObject struct {
	host *layout.ManifestLayout
	obj  client.Object
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
// Each unit's CR is generated with spec.path = the directory of the layout
// that renders its bundles, and hosted in the parent of that layout (the root
// hosts its own), under both placements: a unit's directory is applied by its
// own Kustomization only, so its CR cannot live inside it, and no parent lists
// it (the writers skip a child that renders bundles). The unit's Source, if
// its SourceRef has a URL, is hosted in the root (go-kure/kure#876).
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
		root:      ix.NodeLayout(c.Node),
		perLayout: perLayout,
		names:     map[string]string{},
		generated: map[string]bool{},
		derived:   map[string]bool{},
		placed:    map[client.Object]bool{},
	}
	existing, err := indexExistingKustomizations(ml, nil)
	if err != nil {
		return err
	}
	p.existing = existing
	sources, err := indexExistingSources(ml, nil)
	if err != nil {
		return err
	}
	p.sources = sources
	if err := p.place(ml, sourceScope{}); err != nil {
		return err
	}
	if err := p.hostSourcesOncePerBuild(ml); err != nil {
		return err
	}
	if err := p.checkRootBuildKeepsHostedSources(ml); err != nil {
		return err
	}
	if err := checkPlacedReconcileOrder(ml, p.generated); err != nil {
		return err
	}
	markFluxBuilds(ml, p.generated)
	return nil
}

// markFluxBuilds marks, with SetFluxBuild, the directory each Kustomization
// this integration generated builds (generated: their namespace/name keys),
// and the root when it generated any: the Flux bootstrap applies the root. The
// writers check a KustomizationRecursive directory so marked against what
// Flux builds from it. A Kustomization a caller or an application placed marks
// nothing.
func markFluxBuilds(root *layout.ManifestLayout, generated map[string]bool) {
	layoutAt := map[string]*layout.ManifestLayout{}
	var paths []string
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		layoutAt[path.Clean(l.FullRepoPath())] = l
		for _, obj := range l.Resources {
			if k, ok := obj.(*kustv1.Kustomization); ok && generated[crKey(k.Namespace, k.Name)] {
				paths = append(paths, k.Spec.Path)
			}
		}
		for _, c := range l.Children {
			if c != nil {
				walk(c)
			}
		}
	}
	walk(root)
	if len(paths) == 0 {
		return
	}
	root.SetFluxBuild(true)
	for _, p := range paths {
		if l := layoutAt[path.Clean(p)]; l != nil {
			l.SetFluxBuild(true)
		}
	}
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

	reach := func(l *layout.ManifestLayout, into map[*layout.ManifestLayout]bool) {
		for _, b := range buildDirectories(l) {
			into[b] = true
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

// buildDirectories returns, in pre-order, the layouts whose directories a
// kustomize build of l includes: l and the child directories its
// kustomization.yaml lists, recursively. The writers list a child unless it is
// an umbrella child or renders bundles (a unit, applied by its own CR only), an
// AppFileSingle file, or its parent is PerLayout (which lists the child's CR
// instead).
func buildDirectories(l *layout.ManifestLayout) []*layout.ManifestLayout {
	out := []*layout.ManifestLayout{l}
	for _, child := range l.Children {
		if child == nil || child.UmbrellaChild || child.ApplicationFileMode == layout.AppFileSingle ||
			len(child.OriginBundles()) > 0 || l.FluxPlacement == layout.FluxIntegratedPerLayout {
			continue
		}
		out = append(out, buildDirectories(child)...)
	}
	return out
}

// hostSourcesOncePerBuild keeps each Source this pass derived once per
// kustomize build, which refuses one object twice: add hosts every derived
// Source in the root node's layout, and a build can include a copy the tree
// already held. The builds are top's, the whole tree's top layout (a
// ClusterName wrapper's kustomization.yaml can list the root node's
// directory), the root node's (the Flux bootstrap applies it) and the spec.path of every
// Kustomization this pass placed or kept, each with the directories
// buildDirectories lists from it and the AppFileSingle files written into
// them. Caller and application Kustomizations are not builds kure answers for.
//
// Per build and Source, a copy this pass did not place (an earlier
// integration's, a caller's or an application's) is the one kept; otherwise
// the first copy in depth-first layout order. Every other copy this
// pass placed in that build is removed: a sourceRef names the object, not the
// layout holding it. Two copies this pass did not place cannot be reduced to
// one, so they are an error. So is such a copy outside the root node's build
// in a build that also holds the root's copy (a ClusterName wrapper's): the
// root's copy cannot stay, and dropping it would leave the Source out of every
// build the bootstrap applies. A copy this pass did not place, in a build
// other than the root's, is otherwise kept beside the root's copy: it is the
// caller's.
func (p *integratedPlacement) hostSourcesOncePerBuild(top *layout.ManifestLayout) error {
	if len(p.derived) == 0 {
		return nil
	}
	layoutAt, crs := p.indexGenerated(top)
	builds := []*layout.ManifestLayout{top}
	seen := map[*layout.ManifestLayout]bool{top: true}
	if !seen[p.root] {
		seen[p.root] = true
		builds = append(builds, p.root)
	}
	for _, k := range crs {
		if b := layoutAt[path.Clean(k.Spec.Path)]; b != nil && !seen[b] {
			seen[b] = true
			builds = append(builds, b)
		}
	}

	rootScope := buildScope(p.root)

	for _, b := range builds {
		scope := buildScope(b)
		copies := map[string][]hostedObject{}
		for _, l := range scope {
			objs, err := resourceItems(l)
			if err != nil {
				return err
			}
			for _, obj := range objs {
				if key, ok := sourceKey(obj); ok && p.derived[key] {
					copies[key] = append(copies[key], hostedObject{host: l, obj: obj})
				}
			}
		}
		for _, key := range slices.Sorted(maps.Keys(copies)) {
			all := copies[key]
			var trees []hostedObject
			for _, c := range all {
				if !p.placed[c.obj] {
					trees = append(trees, c)
				}
			}
			if len(trees) > 1 {
				obj := trees[0].obj
				return errors.Errorf("layouts %q and %q both hold %s %q, and the kustomize build of %q includes both: kustomize refuses one object twice", trees[0].host.FullRepoPath(), trees[1].host.FullRepoPath(), obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), b.FullRepoPath())
			}
			keep := all[0].obj
			if len(trees) == 1 {
				keep = trees[0].obj
				if t := trees[0]; !slices.Contains(rootScope, t.host) && slices.ContainsFunc(all, func(c hostedObject) bool { return c.host == p.root }) {
					obj := t.obj
					return errors.Errorf("layout %q holds %s %q, and the kustomize build of %q includes it and the copy the integration hosts in %q, the root node's layout the Flux bootstrap applies: kustomize refuses one object twice, and without the root's copy no build the bootstrap applies holds the Source; move the copy into %q's build or remove it", t.host.FullRepoPath(), obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), b.FullRepoPath(), p.root.FullRepoPath(), p.root.FullRepoPath())
				}
			}
			for _, c := range all {
				if c.obj != keep {
					c.host.Resources = slices.DeleteFunc(c.host.Resources, func(o client.Object) bool { return o == c.obj })
				}
			}
		}
	}
	return nil
}

// indexGenerated maps every layout under top by its directory, and returns in
// depth-first layout order every Kustomization this pass placed or kept
// (generated).
func (p *integratedPlacement) indexGenerated(top *layout.ManifestLayout) (map[string]*layout.ManifestLayout, []*kustv1.Kustomization) {
	layoutAt := map[string]*layout.ManifestLayout{}
	var crs []*kustv1.Kustomization
	var index func(l *layout.ManifestLayout)
	index = func(l *layout.ManifestLayout) {
		layoutAt[path.Clean(l.FullRepoPath())] = l
		for _, obj := range l.Resources {
			if k, ok := obj.(*kustv1.Kustomization); ok && p.generated[crKey(k.Namespace, k.Name)] {
				crs = append(crs, k)
			}
		}
		for _, child := range l.Children {
			if child != nil {
				index(child)
			}
		}
	}
	index(top)
	return layoutAt, crs
}

// buildScope returns the layouts whose objects a kustomize build of b holds:
// the directories buildDirectories lists from b, and the AppFileSingle
// application files written into them.
func buildScope(b *layout.ManifestLayout) []*layout.ManifestLayout {
	var scope []*layout.ManifestLayout
	for _, l := range buildDirectories(b) {
		scope = append(scope, l)
		for _, child := range l.Children {
			if child != nil && child.ApplicationFileMode == layout.AppFileSingle {
				scope = append(scope, child)
			}
		}
	}
	return scope
}

// checkRootBuildKeepsHostedSources refuses a Kustomization this pass placed
// or kept whose build holds the root node's layout — the root bundle's, whose
// spec.path is the directory the Flux bootstrap applies — when one of its
// patches applies to, or its postBuild substitution changes, a Source this
// pass hosted there (go-kure/kure#908). The bootstrap applies that directory
// with neither, so the two would apply the Source differently and keep
// overwriting each other.
//
// A patch with a target applies to the Source when the target selects it
// (patchTargetMatcher); one without is a strategic-merge patch, which applies
// to the object whose identity its body names, compared as kustomize compares
// it (group, version, kind, name and effective namespace). A document that
// does not parse as one is left out: a JSON6902 patch needs a target, and
// kustomize refuses the build without one. The postBuild substitution runs as
// Flux's own, offline: the inline substitute vars, and — since the
// substituteFrom values are in the cluster — any ${...} expression when
// substituteFrom is set, which Flux replaces with the value or with nothing.
// A patch that selects the Source but leaves it unchanged is refused too.
//
// Only the Sources this pass placed are checked. When the root build already
// held a copy the pass did not place (the caller's, an application's or an
// earlier integration's), hostSourcesOncePerBuild kept that copy instead, and
// what the root bundle's patches do to it is its owner's.
func (p *integratedPlacement) checkRootBuildKeepsHostedSources(top *layout.ManifestLayout) error {
	var hosted []client.Object
	for _, obj := range p.root.Resources {
		if _, ok := sourceKey(obj); ok && p.placed[obj] {
			hosted = append(hosted, obj)
		}
	}
	if len(hosted) == 0 {
		return nil
	}
	layoutAt, crs := p.indexGenerated(top)
	rf := provider.NewDefaultDepProvider().GetResourceFactory()
	for _, k := range crs {
		b := layoutAt[path.Clean(k.Spec.Path)]
		if b == nil || !slices.Contains(buildScope(b), p.root) {
			continue
		}
		refuse := func(s client.Object, cause, remedy string) error {
			return errors.Errorf("Flux Kustomization %q (spec.path %q) builds %s %q, which the integration hosts in %q, the root node's layout the Flux bootstrap applies without patches or postBuild: its %s, so the two would apply the Source differently and keep overwriting each other; %s",
				k.Name, k.Spec.Path, s.GetObjectKind().GroupVersionKind().Kind, s.GetName(), p.root.FullRepoPath(), cause, remedy)
		}
		const movePatch = "narrow the patch target, or move the patch to a bundle below the root node"
		for i, patch := range k.Spec.Patches {
			if patch.Target != nil {
				t := &stack.PatchSelector{
					Group: patch.Target.Group, Version: patch.Target.Version, Kind: patch.Target.Kind,
					Name: patch.Target.Name, Namespace: patch.Target.Namespace,
					LabelSelector: patch.Target.LabelSelector, AnnotationSelector: patch.Target.AnnotationSelector,
				}
				selects, err := patchTargetMatcher(t)
				if err != nil {
					return errors.Wrapf(err, "Flux Kustomization %q: patch %d", k.Name, i)
				}
				for _, s := range hosted {
					if selects(s) {
						return refuse(s, fmt.Sprintf("patch %d (target %s) selects it", i, describeTarget(t)), movePatch)
					}
				}
				continue
			}
			docs, err := rf.SliceFromBytes([]byte(patch.Patch))
			if err != nil {
				continue
			}
			for _, doc := range docs {
				for _, s := range hosted {
					if doc.OrgId().Equals(resourceID(s)) {
						return refuse(s, fmt.Sprintf("patch %d (no target) names it", i), movePatch)
					}
				}
			}
		}
		if k.Spec.PostBuild == nil {
			continue
		}
		for _, s := range hosted {
			changed, err := postBuildChanges(rf, k, s)
			if err != nil {
				return errors.Wrapf(err, "Flux Kustomization %q (spec.path %q) builds %s %q, which the integration hosts in %q, the root node's layout the Flux bootstrap applies without postBuild, and its postBuild substitution fails on it",
					k.Name, k.Spec.Path, s.GetObjectKind().GroupVersionKind().Kind, s.GetName(), p.root.FullRepoPath())
			}
			if changed {
				return refuse(s, "postBuild substitution changes it", "drop the ${...} expression from the SourceRef, or move the postBuild to a bundle below the root node")
			}
		}
	}
	return nil
}

// resourceID is obj's identity as kustomize reads it from the object's
// apiVersion, kind, name and namespace.
func resourceID(obj client.Object) resid.ResId {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return resid.NewResIdWithNamespace(resid.NewGvk(gvk.Group, gvk.Version, gvk.Kind), obj.GetName(), obj.GetNamespace())
}

// postBuildChanges reports whether Flux's postBuild substitution with k's vars
// changes obj. It runs Flux's own substitution in dry-run mode, which reads no
// cluster: the inline substitute vars are applied, and with substituteFrom set
// it runs even without them (Always), since Flux loads those vars in the
// cluster; an expression whose var is unknown then becomes empty, as Flux's
// non-strict mode makes it. An object Flux's opt-out label or annotation
// excludes is unchanged.
func postBuildChanges(rf *resource.Factory, k *kustv1.Kustomization, obj client.Object) (bool, error) {
	content, err := comparableContent(obj)
	if err != nil {
		return false, err
	}
	res, err := rf.FromMap(content)
	if err != nil {
		return false, errors.Wrap(err, "read the Source")
	}
	before, err := res.Map()
	if err != nil {
		return false, errors.Wrap(err, "read the Source")
	}
	kust, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		return false, errors.Wrap(err, "convert the Kustomization to unstructured")
	}
	opts := []fluxkustomize.SubstituteOption{fluxkustomize.SubstituteWithDryRun(true)}
	if len(k.Spec.PostBuild.SubstituteFrom) > 0 {
		opts = append(opts, fluxkustomize.SubstituteWithAlways(true))
	}
	out, err := fluxkustomize.SubstituteVariables(context.Background(), nil, unstructured.Unstructured{Object: kust}, res, opts...)
	if err != nil {
		return false, err
	}
	if out == nil {
		return false, nil
	}
	after, err := out.Map()
	if err != nil {
		return false, errors.Wrap(err, "read the substituted Source")
	}
	return !reflect.DeepEqual(before, after), nil
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

// add appends objs to host.Resources, except a Source, which goes to the root
// (go-kure/kure#876). A Kustomization whose name this pass already emitted is
// an identity collision; one already present in host with the same spec.path
// is kept (a repeated integration adds nothing), with another spec.path it is
// an error. A Source is checked against every Source with its identity (kind,
// namespace, name, whatever the API version) anywhere in the tree, and every
// one this pass placed: a different one is an error, wherever it sits; an
// identical one in the root is kept once.
func (p *integratedPlacement) add(host *layout.ManifestLayout, objs []client.Object) error {
	for _, obj := range objs {
		to := host
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
		} else if key, ok := sourceKey(obj); ok {
			// One identity, one object: an identical Source (a repeated
			// integration, or two bundles sharing a SourceRef) is hosted
			// once, at the root, which the Flux bootstrap applies before any
			// Kustomization that uses it: one build, where a copy beside each
			// Kustomization would give each build its own. A different one
			// anywhere in the pass would silently repoint a Kustomization,
			// or leave two directories overwriting each other's Source.
			// hostSourcesOncePerBuild then drops the root's copy if the root
			// build already holds one.
			to = p.root
			p.derived[key] = true
			kept := false
			for _, s := range p.sources[key] {
				if !sameObject(s.obj, obj) {
					have, want := s.obj.GetObjectKind().GroupVersionKind(), obj.GetObjectKind().GroupVersionKind()
					if have.Version != want.Version {
						return errors.Errorf("layout %q already has %s %q at %s; this integration derives it at %s in layout %q: one Source identity must have one definition, at one API version", s.host.FullRepoPath(), want.Kind, obj.GetName(), have.GroupVersion(), want.GroupVersion(), to.FullRepoPath())
					}
					return errors.Errorf("layout %q already has %s %q (%s) with different content than the one this integration derives in layout %q: one Source identity must have one definition", s.host.FullRepoPath(), want.Kind, obj.GetName(), have.GroupVersion(), to.FullRepoPath())
				}
				kept = kept || s.host == to
			}
			if kept {
				continue
			}
			p.sources[key] = append(p.sources[key], hostedObject{host: to, obj: obj})
			p.placed[obj] = true
		}
		to.Resources = append(to.Resources, obj)
	}
	return nil
}

// indexExistingKustomizations records every Flux Kustomization already in the
// tree under ml (an earlier integration's, a caller's, or one an application
// emits), typed or unstructured, keyed by namespace/name. skip, if set, is left
// out. A Kustomization inside a List counts: kustomize builds a List's items.
// One identity present twice is a collision before anything is added:
// kustomize would register the id twice.
func indexExistingKustomizations(ml, skip *layout.ManifestLayout) (map[string]existingCR, error) {
	out := map[string]existingCR{}
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		if l == nil || l == skip {
			return nil
		}
		objs, err := resourceItems(l)
		if err != nil {
			return err
		}
		for _, r := range objs {
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

// indexExistingSources records every Flux Source already in the tree under ml
// (an earlier integration's, a caller's, or one an application emits), typed
// or unstructured, top-level or inside a List, keyed by sourceKey. skip, if
// set, is left out.
func indexExistingSources(ml, skip *layout.ManifestLayout) (map[string][]hostedObject, error) {
	out := map[string][]hostedObject{}
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		if l == nil || l == skip {
			return nil
		}
		objs, err := resourceItems(l)
		if err != nil {
			return err
		}
		for _, r := range objs {
			if key, ok := sourceKey(r); ok {
				out[key] = append(out[key], hostedObject{host: l, obj: r})
			}
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

// resourceItems returns l's resources with every List replaced by its items:
// a List is an envelope, and kustomize builds the items.
func resourceItems(l *layout.ManifestLayout) ([]client.Object, error) {
	var out []client.Object
	for _, r := range l.Resources {
		if r == nil {
			continue
		}
		if u, ok := r.(*unstructured.Unstructured); ok && u.IsList() {
			list, err := u.ToList()
			if err != nil {
				return nil, errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
			}
			for i := range list.Items {
				out = append(out, &list.Items[i])
			}
			continue
		}
		if meta.IsListType(r) {
			items, err := meta.ExtractList(r)
			if err != nil {
				return nil, errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
			}
			for _, item := range items {
				if obj, ok := item.(client.Object); ok {
					out = append(out, obj)
				}
			}
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// sourceKey returns obj's identity as a Flux Source — kind, then effective
// namespace/name (crKey) — and whether obj is one: any object in the Flux
// source API group, whatever its version.
func sourceKey(obj client.Object) (string, bool) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Group != sourcev1.GroupVersion.Group || gvk.Kind == "" {
		return "", false
	}
	return gvk.Kind + " " + crKey(obj.GetNamespace(), obj.GetName()), true
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
// content, typed or unstructured: both are compared in their unstructured
// form, with apiVersion and kind set, an omitted namespace read as "default",
// and the null and empty-object fields a typed object's conversion emits
// (metadata.creationTimestamp, status) left out.
func sameObject(a, b client.Object) bool {
	ua, errA := comparableContent(a)
	ub, errB := comparableContent(b)
	if errA != nil || errB != nil {
		return reflect.DeepEqual(a, b)
	}
	return reflect.DeepEqual(ua, ub)
}

// comparableContent is obj's unstructured content as sameObject compares it.
// It converts a copy: for an unstructured object the converter returns the
// object's own map, which the normalisation below must not touch.
func comparableContent(obj client.Object) (map[string]any, error) {
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj.DeepCopyObject())
	if err != nil {
		return nil, errors.Wrap(err, "convert to unstructured")
	}
	u := &unstructured.Unstructured{Object: content}
	u.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	u.SetNamespace(effectiveNamespace(obj))
	pruneEmpty(u.Object)
	return u.Object, nil
}

// pruneEmpty removes, depth first, every nil value and every map left empty
// from m: an omitted field and a null or empty one are the same content.
func pruneEmpty(m map[string]any) {
	for k, v := range m {
		if child, ok := v.(map[string]any); ok {
			pruneEmpty(child)
			if len(child) == 0 {
				delete(m, k)
			}
			continue
		}
		if v == nil {
			delete(m, k)
		}
	}
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
	// Likewise a generated Source's identity (kind, namespace, name, whatever
	// the API version) must not already be in the tree: flux-system is built
	// beside it, so even an identical copy would be a second resource with
	// the same id, and a different one a competing definition.
	sources, err := indexExistingSources(ml, fluxDir)
	if err != nil {
		return err
	}
	for _, obj := range fluxResources {
		key, ok := sourceKey(obj)
		if !ok {
			continue
		}
		if existing := sources[key]; len(existing) > 0 {
			s := existing[0]
			return errors.Errorf("layout %q already has %s %q (%s); the one generated in %s would define the same Source a second time in the kustomize build", s.host.FullRepoPath(), obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), s.obj.GetObjectKind().GroupVersionKind().GroupVersion(), DefaultFluxDirName)
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
			if err := checkPlacedReconcileOrder(ml, generated); err != nil {
				return err
			}
			markFluxBuilds(ml, generated)
			return nil
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
	markFluxBuilds(ml, generated)
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
