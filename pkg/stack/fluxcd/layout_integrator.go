package fluxcd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"reflect"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/envsubst"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/go-kure/kure/internal/built"
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
// The rules are validated first, as given, by the check the walks use
// (layout.LayoutRules.Validate): this call reads the placement and the file
// naming of a tree it did not walk.
//
// Every placement first indexes the layout's origins (layout.IndexOrigins): a
// hand-built, partial or other-cluster tree is refused rather than matched by
// name, and every Kustomization's spec.path is the directory of the layout
// that renders its bundle. Integrating the same layout again adds nothing: a
// CR already present with the same name and spec.path in the layout that
// would host it is kept; one with the same name elsewhere or with another
// path is an error.
func (li *LayoutIntegrator) IntegrateWithLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	if err := rules.Validate(); err != nil {
		return err
	}
	if ml == nil || c == nil || c.Node == nil {
		return nil
	}

	rules = normalizeRulesPlacement(rules)

	// A Kustomization is hosted outside the directory it applies: in the
	// parent of that directory, or in the flux-system directory of the top
	// of the tree. A top that renders a bundle has no parent, and its
	// flux-system directory is inside it, so in every placement the
	// Kustomization that applies it would be part of the build it applies.
	// layout.WalkCluster returns no such tree: the root node's bundle has a
	// directory below the root node's (go-kure/kure#979).
	if bundles := ml.OriginBundles(); len(bundles) > 0 {
		return errors.Errorf("layout %q renders bundle %q and is the top of the tree: its Flux Kustomization would be part of the build it applies; integrate the whole tree layout.WalkCluster returns", ml.FullRepoPath(), bundles[0].Name)
	}

	// The integration's placement is the tree's: the writers decide from
	// each layout's own FluxPlacement whether a child is listed as a
	// directory or through its CR, so a tree walked with another placement
	// would apply directories twice or leave CRs unapplied. A tree that
	// already holds Flux Kustomizations placed outside its applications (an
	// earlier integration's, or a caller's) was placed by them, so it is not
	// re-placed; and a refused call puts the tree back as it was.
	if ml.FluxPlacement != rules.FluxPlacement {
		if placed := placedKustomization(ml); placed != "" {
			return errors.Errorf("layout %q already holds Flux Kustomization %q, placed for %q: it cannot be integrated again as %q", ml.FullRepoPath(), placed, ml.FluxPlacement, rules.FluxPlacement)
		}
	}
	restore := saveLayouts(ml)
	setPlacement(ml, rules.FluxPlacement)

	// An application's delivery intent becomes Flux's per-object annotations
	// on its objects, whatever the placement. They are set in place, which
	// saveLayouts does not cover (it keeps the same objects), so a refusal
	// takes them back through undoDelivery.
	deliverySources, undoDelivery, err := applyDeliveryIntents(ml)
	if err != nil {
		restore()
		return err
	}

	if rules.FluxPlacement == layout.FluxSeparate {
		err = li.addSeparateFluxToLayout(ml, c, rules.FileNaming)
	} else {
		// Both inline placements put Flux CRs in the tree. They differ only
		// in granularity: PerLayout emits a CR for every layout node (incl.
		// augmenter-added child layouts); PerBundle stops at bundle
		// boundaries and lets kustomize include a bundle's application
		// directories.
		err = li.addIntegratedFluxToLayout(ml, c, rules.FluxPlacement == layout.FluxIntegratedPerLayout, deliverySources)
	}
	if err != nil {
		undoDelivery()
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
//
// The rules are validated by the walk, not here (layout.LayoutRules.Validate),
// also for a nil cluster: invalid rules are an error whatever the cluster is.
func (li *LayoutIntegrator) CreateLayoutWithResources(c *stack.Cluster, rules layout.LayoutRules) (*layout.ManifestLayout, error) {
	if c == nil {
		// Nothing to build; the walk still refuses rules it would not walk.
		return layout.WalkCluster(nil, rules)
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
		if err := validateSourceRefsForFluxIntegrated(c, rules.FluxPlacement); err != nil {
			return nil, err
		}
	}

	// Generate the base manifest layout first
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		return nil, errors.ResourceValidationError("Cluster", c.Name, "layout",
			"failed to create base layout", err)
	}

	// Integrate Flux resources into the layout
	if err := li.IntegrateWithLayout(ml, c, rules); err != nil {
		return nil, errors.ResourceValidationError("Cluster", c.Name, "flux-integration",
			"failed to integrate Flux resources", err)
	}

	return ml, nil
}

// integratedPlacement is one pass of inline placement over a walked layout.
type integratedPlacement struct {
	gen *ResourceGenerator
	ix  *layout.OriginIndex
	// root is the root node's layout. It hosts every Source this pass derives
	// (go-kure/kure#876). The Flux bootstrap applies the top of the tree
	// (layout.TopDirectory): root itself, or a ClusterName wrapper above it.
	// A wrapper's build holds root's directory, unless the wrapper is
	// FluxIntegratedPerLayout and lists root's own Kustomization instead.
	root      *layout.ManifestLayout
	perLayout bool
	// nodes names the cluster's nodes and resolves a node to the layout
	// whose per-layout Kustomization is its own.
	nodes *nodeIndex
	// names maps every Kustomization name this pass emitted to its
	// spec.path and to what it was generated for: Flux Kustomizations share
	// one namespace, so a name is an identity.
	names map[string]claimant
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
	// derivable is the sourceKey of every Source this pass will derive
	// (derivableSources), known before the first is placed: layoutSource
	// passes one over for a Kustomization that would deliver it.
	derivable map[string]bool
	// delivery maps the sourceKey of every Source an application with a
	// delivery intent emits to the annotations that intent asks for
	// (applyDeliveryIntents): a Source this pass derives with that identity
	// carries them as well.
	delivery map[string]map[string]string
	// patchBuilds maps each bundle to its own Kustomization and to those of
	// the layouts of its applications, and patchOrder lists those bundles in
	// the order the pass met them: placeBundlePatches writes each bundle's
	// patches on them once every Kustomization is placed.
	patchBuilds map[*stack.Bundle]*bundlePatchBuilds
	patchOrder  []*stack.Bundle
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
	// dependsOn holds its spec.dependsOn entries
	// (fluxKustomizationDependsOn).
	dependsOn []string
}

// sourceScope is what a layout's subtree takes from the nearest layout
// (itself or an ancestor) rendering bundles: the SourceRef its layout CRs are
// sourced from, and the unit name their default names start with. Both are
// empty below no such layout. top is that layout, or the root of the tree
// below none: the layouts below it, down to the next one rendering bundles,
// are the ones a layout's DependsOn can name (layoutDependsOn).
//
// The root node's layout renders no bundle and still gives its subtree a
// SourceRef, that of its bundles' directory (unitSource); it changes neither
// unit nor top.
type sourceScope struct {
	ref  kustv1.CrossNamespaceSourceReference
	unit string
	top  *layout.ManifestLayout
}

// claimant is what a Kustomization name was claimed for: the spec.path of the
// Kustomization, and its owner as a message names it (bundle "<path>", node
// "<path>" or layout "<directory>").
type claimant struct {
	path  string
	owner string
}

// addIntegratedFluxToLayout places Flux Kustomizations alongside their target
// manifests in one walk over the layout tree.
//
// Each unit's CR is generated with spec.path = the directory of the layout
// that renders its bundles, and hosted in the parent of that layout, under
// both placements: a unit's directory is applied by its own Kustomization
// only, so its CR cannot live inside it, and no parent lists it (the writers
// skip a child that renders bundles). Every unit has a parent:
// IntegrateWithLayout refuses a tree whose top renders a bundle
// (go-kure/kure#979). The unit's Source, if its SourceRef has a URL, is
// hosted in the root (go-kure/kure#876).
//
// PerLayout also gives every child layout that is not an umbrella child, not
// AppFileSingle and renders no bundle (application, augmenter and bundle-less
// node layouts) a CR in its parent, so the writer lists every child of a
// PerLayout layout as a CR file. A bundle-less node layout's CR is named
// <path with "/" replaced by "-">-node: with ClusterName "." node web's path is
// "web", which is also the name of its bundle's CR. An application or
// augmenter layout's is named <unit>-<layout name>, after the Kustomization
// of the bundles it belongs to. Either default gives way to the layout's
// KustomizationName, which the walker takes from Node.KustomizationName
// (layoutCRName), and a node's CR depends on what the node's DependsOn and
// NamedDependsOn say (layoutDependsOn). Those three node fields are refused
// on a node that gets no CR of its own, and under PerBundle (checkFields);
// after them, the eleven settings of a layout's own CR on a layout that gets
// none (checkLayoutFields).
//
// delivery is what applyDeliveryIntents recorded for the Sources of
// applications with a delivery intent.
func (li *LayoutIntegrator) addIntegratedFluxToLayout(ml *layout.ManifestLayout, c *stack.Cluster, perLayout bool, delivery map[string]map[string]string) error {
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		return err
	}
	p := &integratedPlacement{
		gen:         li.Generator,
		ix:          ix,
		root:        ix.NodeLayout(c.Node),
		perLayout:   perLayout,
		nodes:       newNodeIndex(ix, c),
		names:       map[string]claimant{},
		generated:   map[string]bool{},
		derived:     map[string]bool{},
		placed:      map[client.Object]bool{},
		derivable:   derivableSources(ml, li.Generator.DefaultNamespace),
		delivery:    delivery,
		patchBuilds: map[*stack.Bundle]*bundlePatchBuilds{},
	}
	existing, err := indexExistingKustomizations(ml, nil)
	if err != nil {
		return err
	}
	p.existing = existing
	if err := p.nodes.checkFields(perLayout); err != nil {
		return err
	}
	noneHere := ""
	if !perLayout {
		noneHere = fmt.Sprintf("FluxPlacement %q gives a Kustomization to bundles alone", string(layout.FluxIntegratedPerBundle))
	}
	if err := checkLayoutFields(ml, noneHere); err != nil {
		return err
	}
	sources, err := indexExistingSources(ml, nil)
	if err != nil {
		return err
	}
	p.sources = sources
	if err := p.place(ml, sourceScope{top: ml}); err != nil {
		return err
	}
	if err := p.hostSourcesOncePerBuild(ml); err != nil {
		return err
	}
	if err := p.checkSourcesAreHostedBeforeUse(ml); err != nil {
		return err
	}
	// Every Kustomization and Source sits where it stays: a bundle's patches
	// go on the Kustomizations whose build they belong to, before the checks
	// that read a Kustomization's patches.
	if err := p.placeBundlePatches(); err != nil {
		return err
	}
	if err := p.keepEmbeddedPostBuild(ml); err != nil {
		return err
	}
	if err := p.checkRootBuildKeepsHostedSources(ml); err != nil {
		return err
	}
	if err := checkPlacedReconcileOrder(ml, p.generated); err != nil {
		return err
	}
	return markFluxBuilds(ml, p.generated)
}

// markFluxBuilds marks, with SetFluxBuild, the directory each Kustomization
// this integration generated builds (generated: their namespace/name keys),
// and the root when it generated any: the Flux bootstrap applies the root. The
// writers check a KustomizationRecursive directory so marked against what
// Flux builds from it. A Kustomization the integration kept in place of one of
// its own (add) has that key and marks its directory in whatever form it has:
// typed, unstructured or inside a List (go-kure/kure#977). Any other
// Kustomization a caller or an application placed marks nothing.
func markFluxBuilds(root *layout.ManifestLayout, generated map[string]bool) error {
	layoutAt := map[string]*layout.ManifestLayout{}
	var paths []string
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		layoutAt[path.Clean(l.FullRepoPath())] = l
		objs, err := resourceItems(l)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			if p, ok := fluxKustomizationPath(obj); ok && generated[crKey(obj.GetNamespace(), obj.GetName())] {
				paths = append(paths, p)
			}
		}
		for _, c := range l.Children {
			if c == nil {
				continue
			}
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}
	root.SetFluxBuild(true)
	for _, p := range paths {
		if l := layoutAt[path.Clean(p)]; l != nil {
			l.SetFluxBuild(true)
		}
	}
	return nil
}

// checkPlacedReconcileOrder runs checkReconcileOrder over the Kustomizations
// this integration placed or kept (generated: their namespace/name keys; a
// kept one in whatever form it has, generatedKustomizations), with the
// creation rule integrated placement adds: a CR exists only once the
// Kustomization whose directory references reach its file has applied, and a
// CR the root directory reaches is created by the Flux bootstrap. Under
// separate placement every CR sits in flux-system, which the root lists, so
// GenerateFromLayout's own check is the whole story.
func checkPlacedReconcileOrder(root *layout.ManifestLayout, generated map[string]bool) error {
	var kusts []*kustv1.Kustomization
	hostOf := map[string]*layout.ManifestLayout{}
	layoutAt := map[string]*layout.ManifestLayout{}
	var index func(l *layout.ManifestLayout) error
	index = func(l *layout.ManifestLayout) error {
		layoutAt[path.Clean(l.FullRepoPath())] = l
		own, err := generatedKustomizations(l, generated)
		if err != nil {
			return err
		}
		for _, k := range own {
			kusts = append(kusts, k)
			hostOf[crKey(k.Namespace, k.Name)] = l
		}
		for _, child := range l.Children {
			if child != nil {
				if err := index(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := index(root); err != nil {
		return err
	}

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
// already held. The builds are top's, the whole tree's top layout, which the
// Flux bootstrap applies (a ClusterName wrapper's kustomization.yaml can list
// the root node's directory), the root node's and the spec.path of every
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
// in a build that also holds the root's copy (a ClusterName wrapper's that
// lists the root node's directory): kustomize refuses one object twice in a
// build, and the pass removes neither, the caller's copy or the one in the
// layout that hosts every derived Source. A copy this pass did not place, in a
// build other than the root's, is otherwise kept beside the root's copy: it is
// the caller's.
func (p *integratedPlacement) hostSourcesOncePerBuild(top *layout.ManifestLayout) error {
	if len(p.derived) == 0 {
		return nil
	}
	layoutAt, crs, err := p.indexGenerated(top)
	if err != nil {
		return err
	}
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
					return errors.Errorf("layout %q holds %s %q, and the kustomize build of %q includes it and the copy the integration hosts in %q, the root node's layout: kustomize refuses one object twice in a build; move the copy into %q's build or remove it", t.host.FullRepoPath(), obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), b.FullRepoPath(), p.root.FullRepoPath(), p.root.FullRepoPath())
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
// (generated), in typed form whatever form it has (generatedKustomizations).
func (p *integratedPlacement) indexGenerated(top *layout.ManifestLayout) (map[string]*layout.ManifestLayout, []*kustv1.Kustomization, error) {
	layoutAt := map[string]*layout.ManifestLayout{}
	var crs []*kustv1.Kustomization
	var index func(l *layout.ManifestLayout) error
	index = func(l *layout.ManifestLayout) error {
		layoutAt[path.Clean(l.FullRepoPath())] = l
		own, err := generatedKustomizations(l, p.generated)
		if err != nil {
			return err
		}
		crs = append(crs, own...)
		for _, child := range l.Children {
			if child != nil {
				if err := index(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := index(top); err != nil {
		return nil, nil, err
	}
	return layoutAt, crs, nil
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
// or kept whose build holds the root node's layout, when the build the Flux
// bootstrap applies holds it too and one of the Kustomization's patches
// applies to, or its postBuild substitution changes, a Source this pass hosted
// there (go-kure/kure#908). The bootstrap applies that directory with neither,
// so the two would apply the Source differently and keep overwriting each
// other.
//
// The bootstrap applies top, the top of the tree (layout.TopDirectory). Its
// build holds the root node's layout when top is that layout or lists its
// directory. Below a ClusterName wrapper under FluxIntegratedPerLayout it does
// not: the wrapper lists the root node's layout Kustomization and not the
// directory, so that Kustomization is the only one to apply the Sources hosted
// there, and its patches and postBuild are its own (go-kure/kure#979: the
// bootstrap used to be pointed at the root node's directory whatever the rules
// were, and this check refused that Kustomization).
//
// No tree IntegrateWithLayout places reaches the refusal today. It sets one
// placement on the whole tree, and a Kustomization this pass places or keeps
// builds the root node's layout in two cases only. One is the layout
// Kustomization of that layout: placed under FluxIntegratedPerLayout only,
// where its host lists it and not the directory. The other is the
// Kustomization of a bundle that layout renders, on a tree changed by hand
// (the walker renders the root bundle one directory lower, go-kure/kure#979):
// no parent lists a directory that renders a bundle, and a top that renders one
// is refused. The check states the rule for a build that would break it.
//
// A patch with a target applies to the Source when the target selects it
// (patchTargetMatcher); one without is a strategic-merge patch, which applies
// to the object whose identity its body names, compared as kustomize compares
// it (group, version, kind, name and effective namespace). A document that
// does not parse as one is left out: a JSON6902 patch needs a target, and
// kustomize refuses the build without one. The postBuild substitution runs as
// Flux's own, offline, with the inline substitute vars (postBuildChanges);
// since the substituteFrom values are in the cluster, when substituteFrom is
// set any ${...} expression reading a var the inline vars do not set is
// refused whatever the offline result. A patch that selects the Source but
// leaves it unchanged is refused too.
//
// Only the Sources this pass placed are checked. When the root build already
// held a copy the pass did not place (the caller's, an application's or an
// earlier integration's), hostSourcesOncePerBuild kept that copy instead, and
// what a Kustomization's patches do to it is its owner's.
func (p *integratedPlacement) checkRootBuildKeepsHostedSources(top *layout.ManifestLayout) error {
	var hosted []client.Object
	for _, obj := range p.root.Resources {
		if _, ok := sourceKey(obj); ok && p.placed[obj] {
			hosted = append(hosted, obj)
		}
	}
	if len(hosted) == 0 || !slices.Contains(buildScope(top), p.root) {
		return nil
	}
	layoutAt, crs, err := p.indexGenerated(top)
	if err != nil {
		return err
	}
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
		// The remedies are worded for the Kustomization this check most
		// plainly meets: the root node's layout Kustomization, a layout
		// that renders no bundle to move a patch to.
		const movePatch = "narrow the patch target so that it leaves the Source out, or remove the patch from this Kustomization"
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
				id, ok := patchDocumentID(doc)
				if !ok {
					continue
				}
				for _, s := range hosted {
					if id.Equals(resourceID(s)) {
						return refuse(s, fmt.Sprintf("patch %d (no target) names it", i), movePatch)
					}
				}
			}
		}
		if k.Spec.PostBuild == nil {
			continue
		}
		for _, s := range hosted {
			cause, err := postBuildChanges(rf, k, s)
			if err != nil {
				return errors.Wrapf(err, "Flux Kustomization %q (spec.path %q) builds %s %q, which the integration hosts in %q, the root node's layout the Flux bootstrap applies without postBuild, and its postBuild substitution fails on it",
					k.Name, k.Spec.Path, s.GetObjectKind().GroupVersionKind().Kind, s.GetName(), p.root.FullRepoPath())
			}
			if cause != "" {
				return refuse(s, cause, "drop the ${...} expression from the SourceRef URL, or remove the postBuild from this Kustomization")
			}
		}
	}
	return nil
}

// patchDocumentID is the identity by which kustomize looks up the object an
// untargeted strategic-merge patch document is merged into: the first of the
// identities its previous-identity annotations
// (internal.config.kubernetes.io/previousKinds, previousNames and
// previousNamespaces) give it, or else the one it is written with. ok is false
// for a document kustomize cannot read them from, where they do not list
// equally many values: OrgId panics on it, and so does kustomize's own build
// of a Kustomization with that patch, so the document names no object of any
// build.
//
// The reader of those annotations is internal to kustomize, so its rule is
// not copied here, where the copy would have to follow kustomize's: the call
// is made and its panic recovered. The recover is around that one call and
// nothing else.
func patchDocumentID(doc *resource.Resource) (id resid.ResId, ok bool) {
	defer func() {
		if recover() != nil {
			id, ok = resid.ResId{}, false
		}
	}()
	return doc.OrgId(), true
}

// resourceID is obj's identity as kustomize reads it from the object's
// apiVersion, kind, name and namespace.
func resourceID(obj client.Object) resid.ResId {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return resid.NewResIdWithNamespace(resid.NewGvk(gvk.Group, gvk.Version, gvk.Kind), obj.GetName(), obj.GetNamespace())
}

// postBuildChanges reports how Flux's postBuild substitution with k's vars
// can change obj, or "" when it cannot. It runs Flux's own substitution in
// dry-run mode, which reads no cluster: the inline substitute vars are
// applied, and with substituteFrom set it runs even without them (Always).
// The substituteFrom values are in the cluster, so with substituteFrom set
// an expression that reads a var the inline vars do not set — which is the
// one substituteFrom can supply — can change obj whatever the offline result
// is; the inline vars override substituteFrom's, so an expression reading
// only those is decided by the offline result. An object Flux's opt-out
// label or annotation excludes is unchanged.
func postBuildChanges(rf *resource.Factory, k *kustv1.Kustomization, obj client.Object) (string, error) {
	content, err := comparableContent(obj)
	if err != nil {
		return "", err
	}
	res, err := rf.FromMap(content)
	if err != nil {
		return "", errors.Wrap(err, "read the Source")
	}
	before, err := res.Map()
	if err != nil {
		return "", errors.Wrap(err, "read the Source")
	}
	text, err := res.AsYAML()
	if err != nil {
		return "", errors.Wrap(err, "read the Source")
	}
	kust, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		return "", errors.Wrap(err, "convert the Kustomization to unstructured")
	}
	opts := []fluxkustomize.SubstituteOption{fluxkustomize.SubstituteWithDryRun(true)}
	if len(k.Spec.PostBuild.SubstituteFrom) > 0 {
		opts = append(opts, fluxkustomize.SubstituteWithAlways(true))
	}
	out, err := fluxkustomize.SubstituteVariables(context.Background(), nil, unstructured.Unstructured{Object: kust}, res, opts...)
	if err != nil {
		return "", err
	}
	if out == nil {
		return "", nil
	}
	if len(k.Spec.PostBuild.SubstituteFrom) > 0 {
		var fromCluster []string
		if _, err := envsubst.Eval(string(text), func(name string) (string, bool) {
			v, inline := k.Spec.PostBuild.Substitute[name]
			if !inline && !slices.Contains(fromCluster, name) {
				fromCluster = append(fromCluster, name)
			}
			return v, true
		}); err != nil {
			return "", errors.Wrap(err, "read the Source's ${...} expressions")
		}
		if len(fromCluster) > 0 {
			return fmt.Sprintf("postBuild substitution reads %s, which the inline vars do not set and substituteFrom can", strings.Join(fromCluster, ", ")), nil
		}
	}
	after, err := out.Map()
	if err != nil {
		return "", errors.Wrap(err, "read the substituted Source")
	}
	if !reflect.DeepEqual(before, after) {
		return "postBuild substitution changes it", nil
	}
	return "", nil
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
			return errors.Errorf("Flux Kustomizations can never all become Ready: %s waits for itself (%s); a dependsOn waits before applying, a health check before becoming Ready, a Kustomization with wait becomes Ready only once every Kustomization it applied is, and a CR exists only once the Kustomization that creates it has applied",
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
		scope = sourceScope{ref: sourceRefOf(bundles[0]), unit: p.ix.UnitName(bundles[0]), top: l}
	} else if ref, ok := unitSource(l); ok {
		// The root node's layout renders no bundle, so it starts no unit:
		// only the source changes.
		scope.ref = ref
	}

	// One Kustomization per directory that renders bundles: the bundles a
	// grouping axis merged into l share it (generateForUnit).
	if len(bundles) > 0 {
		objs, err := p.gen.generateForUnit(l, p.ix)
		if err != nil {
			return errors.ResourceValidationError("Bundle", bundles[0].Name, "flux-resources",
				"failed to generate Flux resources", err)
		}
		// The CR is hosted by the parent of the layout that renders the
		// bundles. Every unit has one: IntegrateWithLayout refuses a top
		// that renders a bundle (go-kure/kure#979).
		host := p.ix.Parent(l)
		if err := p.add(host, objs, bundleOwner(bundles[0]), bundleNamed(bundles[0])); err != nil {
			return err
		}
		if p.perLayout {
			// generateForUnit returns the unit's Kustomization first.
			var unit *kustv1.Kustomization
			if len(objs) > 0 {
				unit, _ = objs[0].(*kustv1.Kustomization)
			}
			if unit == nil {
				return errors.ResourceValidationError("Bundle", bundles[0].GetPath(), "flux-resources",
					fmt.Sprintf("the Flux resources generated for layout %q do not start with its Kustomization, so the bundle's patches cannot be placed", l.FullRepoPath()), nil)
			}
			p.recordUnitBuild(host, l, bundles, unit)
		}
	}

	if p.perLayout {
		for _, child := range l.Children {
			if !hasLayoutCR(child) {
				continue
			}
			name := layoutCRName(child, scope.unit)
			if err := p.checkLayoutCRName(child, name); err != nil {
				return err
			}
			owner := p.layoutOwner(child)
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
				if err := p.claim(name, e.path, owner); err != nil {
					return err
				}
				if err := p.checkKeptNodeCR(l, child, name, e, scope); err != nil {
					return err
				}
				p.generated[crKey(p.gen.DefaultNamespace, name)] = true
				p.recordLayoutBuild(l, child, name, nil)
				continue
			}
			ref, err := p.layoutSource(child, scope)
			if err != nil {
				return err
			}
			deps, err := p.layoutDependsOn(l, child, scope)
			if err != nil {
				return err
			}
			settings, err := p.layoutSettings(child)
			if err != nil {
				return err
			}
			cr := p.gen.createKustomizationForLayout(name, child, ref, deps, settings)
			if err := p.add(l, []client.Object{cr}, owner, p.layoutNamed(child)); err != nil {
				return err
			}
			p.recordLayoutBuild(l, child, name, cr)
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

// unitSource returns the SourceRef the root node's layout takes from the
// bundles a flat BundleGrouping merges into its node, and whether it names a
// source. The walker renders them one directory lower (the root node's layout
// renders no bundle, go-kure/kure#979); the root node's layout and the layout
// CRs below it that no other bundle encloses keep their source, as when the
// bundles were rendered in the layout itself. Under BundleGrouping by name a
// node's layout has no such directory; layoutSource gives it its node's
// bundle's SourceRef where it finds no other (nodeBundleSource).
func unitSource(l *layout.ManifestLayout) (kustv1.CrossNamespaceSourceReference, bool) {
	unit := l.OriginUnit()
	if unit == nil || len(unit.OriginBundles()) == 0 {
		return kustv1.CrossNamespaceSourceReference{}, false
	}
	ref := sourceRefOf(unit.OriginBundles()[0])
	return ref, ref.Kind != "" && ref.Name != ""
}

// layoutSource returns the SourceRef of child's layout CR: the scope's (the
// nearest bundle-rendering layout at or above the host), else the one of the
// bundles merged into child's node (unitSource), else the one SourceRef the
// URL-less bundles below child share, else the one of the bundle of child's
// own node or, for a node without one, of the nearest node above
// (nodeBundleSource).
//
// The last is what a node's layout takes where no layout renders its node's
// bundle or one above it: under BundleGrouping by name every bundle has a
// directory of its own, one level below its node's, so no bundle encloses a
// node's layout (go-kure/kure#979). It comes last, so it gives a source only
// to a layout that had none: one with no URL-less SourceRef below it, and one
// below which two of them differ.
//
// When child is the root node's layout (below a ClusterName wrapper, which
// hosts its CR) or, on a tree built by hand, a layout above it, its CR applies
// the directory that hosts every Source this pass derives (add), so it cannot
// take one of them: a Kustomization does not deliver its own Source
// (go-kure/kure#979). The Flux bootstrap applies the top of the written tree,
// the wrapper (bootstrapDir); where nothing there delivers that Source, the CR
// would wait for a Source that only its own apply creates. Such a reference is
// passed over, whichever of these it is, and when nothing else is left the
// refusal names it.
//
// A layout left without a source is refused. The refusal says which
// Kustomization it is and what would give it a source; where every SourceRef
// below has a URL it does not say that one is missing, and where the
// SourceRefs below differ it names the bundle that would decide.
func (p *integratedPlacement) layoutSource(child *layout.ManifestLayout, scope sourceScope) (kustv1.CrossNamespaceSourceReference, error) {
	delivers := holdsLayout(child, p.root)
	var own *kustv1.CrossNamespaceSourceReference
	delivered := func(ref kustv1.CrossNamespaceSourceReference) bool {
		if !delivers || !p.derivable[sourceRefKey(ref, p.gen.DefaultNamespace)] {
			return false
		}
		if own == nil {
			own = &ref
		}
		return true
	}
	if scope.ref.Kind != "" && scope.ref.Name != "" && !delivered(scope.ref) {
		return scope.ref, nil
	}
	if ref, ok := unitSource(child); ok && !delivered(ref) {
		return ref, nil
	}
	// Deduplicated by effective value: an omitted namespace is the
	// generator's DefaultNamespace, as it is for the Kustomization's own
	// sourceRef. The first reference is emitted as written.
	var refs []kustv1.CrossNamespaceSourceReference
	seen := map[kustv1.CrossNamespaceSourceReference]bool{}
	// urlBelow: a bundle below child has a SourceRef that was passed over
	// for its URL alone.
	urlBelow := false
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, b := range l.OriginBundles() {
			if b.SourceRef == nil {
				continue
			}
			ref := sourceRefOf(b)
			if delivered(ref) {
				continue
			}
			if b.SourceRef.URL != "" {
				urlBelow = true
				continue
			}
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
		if ref, ok := p.nodeBundleSource(child, delivered); ok {
			return ref, nil
		}
		name, owner := layoutCRName(child, scope.unit), p.layoutOwner(child)
		switch {
		case own != nil:
			return kustv1.CrossNamespaceSourceReference{}, errors.ResourceValidationError("ManifestLayout", child.Name, "sourceRef",
				fmt.Sprintf("%s; give a bundle a SourceRef without a URL, naming a Source that exists before the tree is applied, or use FluxIntegratedPerBundle, under which layout %q has no Kustomization of its own",
					p.sourceInsideDelivery(name, child.FullRepoPath(), *own), child.FullRepoPath()), nil)
		case urlBelow:
			// A SourceRef is set, below: what is missing is one this
			// Kustomization can take.
			return kustv1.CrossNamespaceSourceReference{}, errors.ResourceValidationError("ManifestLayout", child.Name, "sourceRef",
				fmt.Sprintf("Flux Kustomization %q of %s (spec.path %q) has no source: no bundle encloses it, no node at or above it has a bundle with a SourceRef, and every SourceRef on the bundles below it has a URL, which a Kustomization above those bundles does not take; give a bundle below it a SourceRef without a URL, give a node at or above it a bundle with a SourceRef, or use FluxIntegratedPerBundle, under which %s has no Kustomization of its own",
					name, owner, child.FullRepoPath(), owner), nil)
		}
		return kustv1.CrossNamespaceSourceReference{}, errors.ResourceValidationError("ManifestLayout", child.Name, "sourceRef",
			"FluxIntegratedPerLayout mode requires a SourceRef with Kind and Name; "+
				fmt.Sprintf("Flux Kustomization %q of %s (spec.path %q) has no source: no bundle at, below or above it has one", name, owner, child.FullRepoPath()), nil)
	default:
		// The bundles below do not agree; the node's own bundle, or the one
		// of the nearest node above, decides as it does where they have none.
		ref, ok := p.nodeBundleSource(child, delivered)
		if ok {
			return ref, nil
		}
		remedy := "give a node at or above it a bundle with a SourceRef, which its Kustomization then takes"
		if delivers {
			// One with a URL would be a Source this Kustomization delivers.
			remedy = "give a node at or above it a bundle with a SourceRef without a URL, which its Kustomization then takes"
		}
		if ref.Kind != "" && ref.Name != "" {
			// There is such a bundle, and its Source is one this
			// Kustomization would deliver.
			remedy = fmt.Sprintf("the bundle of the nearest node at or above it names %s %q, which the integration generates from a SourceRef with a URL and hosts inside what this Kustomization applies: give that bundle a SourceRef without a URL, naming a Source that exists before the tree is applied",
				ref.Kind, ref.Name)
		}
		return kustv1.CrossNamespaceSourceReference{}, errors.ResourceValidationError("ManifestLayout", child.Name, "sourceRef",
			fmt.Sprintf("layout %q has no enclosing bundle and the bundles below it have different SourceRefs, so its Flux Kustomization has no single source; %s",
				child.FullRepoPath(), remedy), nil)
	}
}

// nodeBundleSource returns the SourceRef of the bundle of the nearest node, at
// or above child, that has one, and whether child's layout CR can take it: it
// names a source and is not one child's own apply delivers (delivered, see
// layoutSource).
//
// A layout's own node is the first of its origin nodes: the ones a flat
// NodeGrouping or a FlattenSingleTier collapse rendered into it follow. Only
// that node's bundle is read, and only the nearest such bundle: with flat
// bundles it is the one that encloses child, which is where the scope's
// SourceRef comes from, and a bundle further up is not looked at there either.
func (p *integratedPlacement) nodeBundleSource(child *layout.ManifestLayout, delivered func(kustv1.CrossNamespaceSourceReference) bool) (kustv1.CrossNamespaceSourceReference, bool) {
	for l := child; l != nil; l = p.ix.Parent(l) {
		nodes := l.OriginNodes()
		if len(nodes) == 0 || nodes[0] == nil || nodes[0].Bundle == nil {
			continue
		}
		ref := sourceRefOf(nodes[0].Bundle)
		return ref, ref.Kind != "" && ref.Name != "" && !delivered(ref)
	}
	return kustv1.CrossNamespaceSourceReference{}, false
}

// sourceInsideDelivery words the refusal of a Kustomization that would take
// its source from ref, a Source this pass derives, while what it applies holds
// the root node's layout, which hosts that Source. The caller adds the remedy.
func (p *integratedPlacement) sourceInsideDelivery(name, specPath string, ref kustv1.CrossNamespaceSourceReference) string {
	return fmt.Sprintf("Flux Kustomization %q (spec.path %q) would take its source from %s %q, which the integration generates from a SourceRef with a URL and hosts in %q, the root node's layout: that is inside what the Kustomization applies, and a Kustomization must not deliver its own Source",
		name, specPath, ref.Kind, ref.Name, p.root.FullRepoPath())
}

// checkSourcesAreHostedBeforeUse refuses a Kustomization this pass placed or
// kept that takes its source from a Source the pass derived while its
// spec.path is the root node's layout or a layout above it. The pass hosts
// every derived Source in the root node's layout (add), which that
// Kustomization applies, itself or through the Kustomizations it creates, and
// a Kustomization does not deliver its own Source (go-kure/kure#979): the
// Flux bootstrap applies the top of the tree, and where that build does not
// hold the directory, the Kustomization would wait for a Source that only its
// own apply creates.
//
// On a walked tree the one such Kustomization is the layout Kustomization of
// the root node's layout below a ClusterName wrapper under
// FluxIntegratedPerLayout. layoutSource gives a generated one another source,
// or refuses it; this check is for the caller's own, kept in its place, and
// for a tree built by hand in which the root node's layout, or one above it,
// renders a bundle.
func (p *integratedPlacement) checkSourcesAreHostedBeforeUse(top *layout.ManifestLayout) error {
	if len(p.derived) == 0 {
		return nil
	}
	layoutAt, crs, err := p.indexGenerated(top)
	if err != nil {
		return err
	}
	for _, k := range crs {
		b := layoutAt[path.Clean(k.Spec.Path)]
		if b == nil || !holdsLayout(b, p.root) {
			continue
		}
		if p.derived[sourceRefKey(k.Spec.SourceRef, k.Namespace)] {
			return errors.Errorf("%s; name a Source that exists before the tree is applied", p.sourceInsideDelivery(k.Name, k.Spec.Path, k.Spec.SourceRef))
		}
	}
	return nil
}

// holdsLayout reports whether target is l or a layout below it.
func holdsLayout(l, target *layout.ManifestLayout) bool {
	if l == nil || target == nil {
		return false
	}
	if l == target {
		return true
	}
	for _, child := range l.Children {
		if holdsLayout(child, target) {
			return true
		}
	}
	return false
}

// derivableSources returns the sourceKey of every Source an integration of the
// tree under top derives: one per SourceRef with a URL and a kind createSource
// derives a Source for, among the bundles its layouts render (a URL with
// another kind is createSource's error, not a Source). An omitted namespace is
// defaultNS, as it is for the Source.
func derivableSources(top *layout.ManifestLayout, defaultNS string) map[string]bool {
	out := map[string]bool{}
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if l == nil {
			return
		}
		for _, b := range l.OriginBundles() {
			if b.SourceRef != nil && b.SourceRef.URL != "" && slices.Contains(derivedSourceKinds, b.SourceRef.Kind) {
				out[sourceRefKey(sourceRefOf(b), defaultNS)] = true
			}
		}
		for _, child := range l.Children {
			walk(child)
		}
	}
	walk(top)
	return out
}

// sourceRefKey is the sourceKey of the Source ref names. An omitted namespace
// is defaultNS: the generator's for a bundle's SourceRef, the Kustomization's
// own for its spec.sourceRef.
func sourceRefKey(ref kustv1.CrossNamespaceSourceReference, defaultNS string) string {
	ns := ref.Namespace
	if ns == "" {
		ns = defaultNS
	}
	return ref.Kind + " " + crKey(ns, ref.Name)
}

// add appends objs to host.Resources, except a Source, which goes to the root
// (go-kure/kure#876). A Kustomization whose name this pass already emitted is
// an identity collision; one already present in host with the same spec.path
// is kept (a repeated integration adds nothing), with another spec.path it is
// an error, which says whose the one in the tree is (heldBy) and what names
// the generated one apart (nameApart). A Source is checked against every
// Source with its identity (kind, namespace, name, whatever the API version)
// anywhere in the tree, and every one this pass placed: a different one is an
// error, wherever it sits; an identical one in the root is kept once.
//
// owner says what objs are generated for (see claimant), and by what names
// its Kustomization, for the refusal: bundleNamed, or layoutNamed's.
func (p *integratedPlacement) add(host *layout.ManifestLayout, objs []client.Object, owner string, by nameField) error {
	for _, obj := range objs {
		to := host
		if k, ok := obj.(*kustv1.Kustomization); ok {
			if err := p.claim(k.Name, k.Spec.Path, owner); err != nil {
				return err
			}
			key := crKey(k.Namespace, k.Name)
			p.generated[key] = true
			if e, ok := p.existing[key]; ok {
				if e.host == host && e.path == k.Spec.Path {
					continue
				}
				return errors.Errorf("layout %q already has Flux Kustomization %q with spec.path %q%s; this integration generates one of that name for %s, with spec.path %q in layout %q: %s",
					e.host.FullRepoPath(), k.Name, e.path, heldBy(p.ix, key, e.host), by.owner, k.Spec.Path, host.FullRepoPath(), nameApart(by.field, by.owner))
			}
		} else if key, ok := sourceKey(obj); ok {
			// One identity, one object: an identical Source (a repeated
			// integration, or two bundles sharing a SourceRef) is hosted
			// once, at the root, which is applied before any Kustomization
			// that uses it (by the Flux bootstrap, or below a
			// FluxIntegratedPerLayout wrapper by the root's own layout
			// Kustomization): one build, where a copy beside each
			// Kustomization would give each build its own. A different one
			// anywhere in the pass would silently repoint a Kustomization,
			// or leave two directories overwriting each other's Source.
			// hostSourcesOncePerBuild then drops the root's copy if the root
			// build already holds one.
			to = p.root
			p.derived[key] = true
			// A Source that an application with a delivery intent emits is
			// that application's object, and the derived one is the same
			// object: it carries the intent's annotations too, so that every
			// copy applied is the same. One that already sets another value
			// is left as it is, and is then a different definition below.
			if want := p.delivery[key]; len(want) > 0 {
				if _, _, conflict := conflicting(obj.GetAnnotations(), want); !conflict {
					if merged, changed := merge(obj.GetAnnotations(), want); changed {
						obj.SetAnnotations(merged)
					}
				}
			}
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
			out[key] = existingCR{host: l, path: path, dependsOn: fluxKustomizationDependsOn(r)}
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

// resourceItems returns l's resources as kustomize builds them, by the rule
// the layout package's pre-write check reads them by (built.Objects,
// go-kure/kure#977): a List is an envelope, and kustomize builds its items. A
// List is an object written with a kind that ends in "List", whatever kind its
// Go value reports, and that has items; a List among the items is opened as
// well, and one whose items are null holds nothing. Any other object is
// returned as itself, whatever fields it has: a kind that does not end in
// "List", a List kind without an items field, and one whose items are not a
// list.
//
// A typed List can hold an item as raw JSON (runtime.RawExtension), which the
// writers serialize as the object it encodes: it is returned as that object.
// An item need not carry object metadata (a client.Object) either: one that
// does not, and is no List, is returned as the object the writers serialize
// for it.
func resourceItems(l *layout.ManifestLayout) ([]client.Object, error) {
	var out []client.Object
	for _, r := range l.Resources {
		if r == nil {
			continue
		}
		objs, err := built.Objects(r)
		if err != nil {
			return nil, errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
		}
		for _, obj := range objs {
			if o, ok := obj.Object.(client.Object); ok {
				out = append(out, o)
				continue
			}
			raw, err := json.Marshal(obj.Object)
			if err != nil {
				return nil, errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
			}
			u := &unstructured.Unstructured{}
			if err := json.Unmarshal(raw, &u.Object); err != nil {
				return nil, errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
			}
			out = append(out, u)
		}
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

// fluxKustomizationDependsOn returns the entries of the spec.dependsOn of obj,
// a Flux Kustomization as fluxKustomizationPath reads one, in order. An entry
// in obj's own namespace (none given, or that one) is its name, as the
// integrator writes one; an entry in another namespace is "<namespace>/<name>",
// which no name the integrator asks for equals.
func fluxKustomizationDependsOn(obj client.Object) []string {
	var entries []string
	add := func(namespace, name string) {
		if namespace != "" && crKey(namespace, "") != crKey(obj.GetNamespace(), "") {
			name = namespace + "/" + name
		}
		entries = append(entries, name)
	}
	if k, ok := obj.(*kustv1.Kustomization); ok {
		for _, d := range k.Spec.DependsOn {
			add(d.Namespace, d.Name)
		}
		return entries
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	deps, _, _ := unstructured.NestedFieldNoCopy(u.Object, "spec", "dependsOn")
	list, _ := deps.([]any)
	for _, d := range list {
		if m, ok := d.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				namespace, _ := m["namespace"].(string)
				add(namespace, name)
			}
		}
	}
	return entries
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
// CR identity is its Bundle.UnitName (IndexOrigins refuses two with one).
// Objects already in the tree may sit in other namespaces; those are keyed by
// crKey in indexExistingKustomizations.
//
// owner says what the Kustomization was generated for (see claimant), so the
// refusal names both: a node whose Kustomization name is a bundle's, say.
func (p *integratedPlacement) claim(name, path, owner string) error {
	if prev, dup := p.names[name]; dup {
		return errors.Errorf("Flux Kustomization name %q is used twice, by %s (spec.path %q) and by %s (spec.path %q): Kustomization names must be unique; set KustomizationName on one of them",
			name, prev.owner, prev.path, owner, path)
	}
	p.names[name] = claimant{path: path, owner: owner}
	return nil
}

// hasLayoutCR reports whether child, a child layout, gets a Kustomization of
// its own under FluxIntegratedPerLayout: every child that is not an umbrella
// child, not AppFileSingle and renders no bundle (see place).
func hasLayoutCR(child *layout.ManifestLayout) bool {
	return child != nil && !child.UmbrellaChild && child.ApplicationFileMode != layout.AppFileSingle && len(child.OriginBundles()) == 0
}

// layoutCRName names the PerLayout CR of a child layout that renders no
// bundle. The layout's own KustomizationName wins: the walker copies a node's
// there, an augmenter sets it on a layout it creates, and a caller on a walked
// layout before integration. Without one, a node layout gets "<path with /
// replaced by ->-node" (see addIntegratedFluxToLayout), and any other layout
// "<unit>-<Name>", unit being the Kustomization name of the nearest layout at
// or above its parent that renders bundles: an application named like its
// bundle then does not take the bundle's Kustomization name; one over the
// limit is shortened (unitLayoutCRName). Below no such layout (unit is empty)
// it is the layout's Name.
//
// Every reference to the CR (the CR itself, and the dependsOn entries that
// name it) takes its name from here, so a shortened name is the same in all.
func layoutCRName(l *layout.ManifestLayout, unit string) string {
	switch {
	case l.KustomizationName != "":
		return l.KustomizationName
	case len(l.OriginNodes()) > 0:
		return strings.ReplaceAll(l.FullRepoPath(), "/", "-") + "-node"
	case unit != "":
		return unitLayoutCRName(unit, l.Name)
	}
	return l.Name
}

// layoutNameHashLength is the number of hexadecimal characters of the hash in
// a shortened "<unit>-<name>" default (unitLayoutCRName).
const layoutNameHashLength = 8

// layoutNameShortenMax is the longest layout name a shortened "<unit>-<name>"
// default keeps whole: the hash and the "-<name>" tail fill the 63 characters
// of a Flux Kustomization name. It is also the length of the prefix of a
// longer name that the default keeps instead.
const layoutNameShortenMax = stack.KustomizationNameMaxLength - layoutNameHashLength - 1

// unitLayoutCRName is the default name "<unit>-<name>" of a layout's CR,
// shortened where it is longer than a Flux Kustomization name may be
// (stack.KustomizationNameMaxLength, 63 characters), whatever the length of
// name. A default that fits is returned unchanged. One over the limit is
// shortened with the first 8 hexadecimal characters of the SHA-256 of the
// whole default, at most 63 characters:
//
//   - where name has at most layoutNameShortenMax (54) characters, it keeps
//     its "-<name>" tail and has the unit name replaced by a prefix of it and
//     the hash: "<unit prefix>-<hash>-<name>". The prefix takes the
//     63 - len(name) - 10 leading bytes of the unit name, without the hyphens
//     and dots that end them (so the result can be shorter than 63); where no
//     byte is left for it (a name of 53 or 54 characters), the result is
//     "<hash>-<name>";
//   - where name is longer, it leaves no room for the hash beside it, and the
//     result is "<name prefix>-<hash>": the first 54 bytes of name, without
//     the hyphens and dots that end them, and the hash; where nothing is left
//     of them, the result is the hash alone. The unit name and the
//     rest of name show only in the hash, and are not checked as part of the
//     result.
//
// The hash is of the whole default, so the result is the same on every run
// for the same unit and name. Two defaults that differ only in the part the
// shortening drops almost always differ in their hash, but 8 hexadecimal
// characters are 32 bits and two can collide; a name used twice, by a
// collision or otherwise, is refused by the duplicate-name check, which names
// both owners.
func unitLayoutCRName(unit, name string) string {
	composed := unit + "-" + name
	if len(composed) <= stack.KustomizationNameMaxLength {
		return composed
	}
	sum := sha256.Sum256([]byte(composed))
	hash := hex.EncodeToString(sum[:])[:layoutNameHashLength]
	if len(name) > layoutNameShortenMax {
		if prefix := strings.TrimRight(name[:layoutNameShortenMax], "-."); prefix != "" {
			return prefix + "-" + hash
		}
		return hash
	}
	// The default is over the limit, so keep is shorter than unit.
	if keep := stack.KustomizationNameMaxLength - len(name) - layoutNameHashLength - 2; keep > 0 {
		if prefix := strings.TrimRight(unit[:keep], "-."); prefix != "" {
			return prefix + "-" + hash + "-" + name
		}
	}
	return hash + "-" + name
}

// DefaultLayoutKustomizationName returns the name the layout integrator gives
// the per-layout Kustomization of an application or augmenter layout under
// FluxIntegratedPerLayout when the layout sets no KustomizationName: the
// default "<unit>-<layoutName>", shortened where it is longer than a Flux
// Kustomization name may be (see the package README, "Per-layout name rule").
// unit is the unit name: the name in effect (stack.Bundle.UnitName) of the
// Kustomization that applies the bundles the layout belongs to, which is the
// first bundle's where a grouping axis merged several into one directory, not
// necessarily that of the bundle holding the application. layoutName is the
// layout's Name.
//
// The result is the same on every run for the same inputs, so a caller that
// checks names before writing them (for collisions or length) gets the name
// kure writes. It is the name only: whether the integrator accepts it (its
// characters, a duplicate) is checked where the Kustomization is created.
func DefaultLayoutKustomizationName(unit, layoutName string) string {
	return unitLayoutCRName(unit, layoutName)
}

// checkLayoutCRName refuses a name for l's per-layout CR that Flux cannot
// reconcile (stack.ValidateKustomizationName), where the CR is created or
// kept. The error names what the caller sets to change the name, which
// depends on where layoutCRName took it from:
//
//   - a node's KustomizationName: the node and that field;
//   - a KustomizationName set on the layout itself (by an augmenter, or by a
//     caller on a walked layout): the layout and that field;
//   - the name derived for a node, "<path>-node": the node, and
//     Node.KustomizationName as the field to set;
//   - the default of an application or augmenter layout, "<unit name>-<layout
//     name>": the layout, and ManifestLayout.KustomizationName as the field
//     to set. That default is shortened where it is over the limit
//     (unitLayoutCRName), so it is refused here only for its characters.
//
// kure shortens no other name: a name set on a node or a layout, and the
// name derived for a node, are checked as they are.
func (p *integratedPlacement) checkLayoutCRName(l *layout.ManifestLayout, name string) error {
	err := stack.ValidateKustomizationName(name)
	if err == nil {
		return nil
	}
	nodes := l.OriginNodes()
	switch {
	case l.KustomizationName != "" && len(nodes) > 0 && nodes[0].KustomizationName == l.KustomizationName:
		return errors.ResourceValidationError("Node", p.nodes.path(nodes[0]), "kustomizationName", err.Error(), nil)
	case l.KustomizationName != "":
		return errors.ResourceValidationError("ManifestLayout", l.FullRepoPath(), "kustomizationName", err.Error(), nil)
	case len(nodes) > 0:
		return errors.ResourceValidationError("Node", p.nodes.path(nodes[0]), "kustomizationName",
			fmt.Sprintf("the node sets no name for its Flux Kustomization, and the name derived from its directory %q cannot be used: %v; set Node.KustomizationName", l.FullRepoPath(), err), nil)
	}
	return errors.ResourceValidationError("ManifestLayout", l.FullRepoPath(), "kustomizationName",
		fmt.Sprintf("the layout sets no name for its Flux Kustomization, and the default cannot be used: %v; set ManifestLayout.KustomizationName", err), nil)
}

// layoutOwner names a layout that gets a per-layout CR, for claim: the node
// it is the own layout of, or else its directory.
func (p *integratedPlacement) layoutOwner(l *layout.ManifestLayout) string {
	if nodes := l.OriginNodes(); len(nodes) > 0 {
		return fmt.Sprintf("node %q", p.nodes.path(nodes[0]))
	}
	return fmt.Sprintf("layout %q", l.FullRepoPath())
}

// bundleOwner names the bundle a unit's Kustomization is generated for, the
// first of the bundles its directory renders, for claim.
func bundleOwner(b *stack.Bundle) string {
	return fmt.Sprintf("bundle %q", b.GetPath())
}

// bundleNameField is the field that names a bundle's Kustomization.
const bundleNameField = "Bundle.KustomizationName"

// nameField is what names a generated Kustomization: field, set on owner.
type nameField struct{ field, owner string }

// bundleNamed is what names the Kustomization generated for b's unit.
func bundleNamed(b *stack.Bundle) nameField {
	return nameField{field: bundleNameField, owner: bundleOwner(b)}
}

// layoutNamed is what names the Kustomization of a layout with a per-layout
// CR, by the cases of checkLayoutCRName: the node and its field for a node's
// own layout whose name is the node's (the walker copies it to the layout) or
// derived from its directory; the layout and its own field where the layout
// sets a name the node does not (layoutCRName takes that one first), and for
// any other layout.
func (p *integratedPlacement) layoutNamed(l *layout.ManifestLayout) nameField {
	nodes := l.OriginNodes()
	if len(nodes) > 0 && (l.KustomizationName == "" || nodes[0].KustomizationName == l.KustomizationName) {
		return nameField{field: "Node.KustomizationName", owner: fmt.Sprintf("node %q", p.nodes.path(nodes[0]))}
	}
	return nameField{field: "ManifestLayout.KustomizationName", owner: fmt.Sprintf("layout %q", l.FullRepoPath())}
}

// heldBy says whose a Flux Kustomization already in the tree is, for the
// refusal of a generated one with its identity (key, a crKey), found in host:
// the application among whose objects the walk recorded one with that
// identity, and that application's bundle. Those objects are read as
// kustomize builds them, a List's items included (resourceItems), and only
// those host still holds, the very values the walk recorded: an object a
// caller put in an application's place since is not the application's
// (sameValue). It says nothing when no application's record holds one: an
// earlier integration's Kustomization, or one a caller added to a layout,
// which the tree does not tell apart. A record without an application names
// none, as in applyDeliveryIntents.
func heldBy(ix *layout.OriginIndex, key string, host *layout.ManifestLayout) string {
	for _, unit := range ix.Units() {
		for _, rec := range unit.OriginApplicationObjects() {
			if rec.Application == nil {
				continue
			}
			var held []client.Object
			for _, obj := range rec.Objects {
				if slices.ContainsFunc(host.Resources, func(r client.Object) bool { return sameValue(r, obj) }) {
					held = append(held, obj)
				}
			}
			// A List that cannot be read holds nothing to name here, and is
			// unreadable where it sits too: indexExistingKustomizations has
			// refused such a tree before any identity is compared.
			objs, _ := resourceItems(&layout.ManifestLayout{Resources: held})
			for _, obj := range objs {
				if _, ok := fluxKustomizationPath(obj); !ok || crKey(obj.GetNamespace(), obj.GetName()) != key {
					continue
				}
				for _, b := range unit.OriginBundles() {
					if slices.Contains(b.Applications, rec.Application) {
						return fmt.Sprintf(", an object of application %q of bundle %q", rec.Application.Name, b.GetPath())
					}
				}
			}
		}
	}
	return ""
}

// sameValue reports whether a and b are one object, not two of one content
// (sameObject): the same value, for the pointers objects are. An application
// may emit an object of any type, and == panics on a value that cannot be
// compared (a struct with a slice in it): such a value is the same as none,
// so nothing is said of it.
func sameValue(a, b client.Object) bool {
	return reflect.ValueOf(a).Comparable() && reflect.ValueOf(b).Comparable() && a == b
}

// nameApart is the way out of a generated Kustomization meeting one already
// in the tree: the two share namespace and name, so one of them gets another
// name. field is the field that names the generated one, on owner.
func nameApart(field, owner string) string {
	return fmt.Sprintf("the two share namespace and name, so one of them needs another name; set %s on %s to name the generated one, or give the one already there another name", field, owner)
}

// layoutDependsOn returns the spec.dependsOn names of child's per-layout CR,
// child being a child of host.
//
// A node layout's are the Kustomizations of the nodes its node depends on
// (Node.DependsOn), then its DependsOn as given: the walker copies the node's
// NamedDependsOn there, and those are Kustomization names.
//
// An application or augmenter layout's DependsOn lists layout names, and the
// CR of a layout is not named like the layout (layoutCRName). An entry that
// is the Name of a layout with a CR of its own in the same unit (below
// scope.top, down to the next layout rendering bundles) is written as that
// CR's name: a sibling first, then the one such layout elsewhere in the unit,
// the parent or a child, say. Two of them elsewhere are refused, since the
// entry does not say which. Any other entry is a Kustomization name and is
// written as given.
func (p *integratedPlacement) layoutDependsOn(host, child *layout.ManifestLayout, scope sourceScope) ([]string, error) {
	if nodes := child.OriginNodes(); len(nodes) > 0 {
		deps, err := p.nodes.dependencies(nodes[0])
		if err != nil {
			return nil, err
		}
		return append(deps, child.DependsOn...), nil
	}
	var deps []string
	for _, dep := range child.DependsOn {
		named := layoutsNamed(host.Children, dep, false)
		if len(named) == 0 {
			named = layoutsNamed(scope.top.Children, dep, true)
		}
		switch len(named) {
		case 0:
		case 1:
			dep = layoutCRName(named[0], scope.unit)
		default:
			return nil, errors.Errorf("layout %q lists %q in DependsOn, which is the name of %d layouts with a Flux Kustomization of their own (%q and %q among them): set KustomizationName on the one meant and list that name",
				child.FullRepoPath(), dep, len(named), named[0].FullRepoPath(), named[1].FullRepoPath())
		}
		deps = append(deps, dep)
	}
	return deps, nil
}

// checkKeptNodeCR refuses a node's Kustomization that is kept from the tree
// (kept, named name, in host) when the node sets DependsOn or NamedDependsOn
// and the kept spec.dependsOn lacks one of the Kustomizations they ask for:
// keeping it would drop that dependency without a word. Order does not
// matter, and what the kept one waits for besides is the caller's. A node
// that sets neither field is not held to this, whatever its layout's
// DependsOn says: such a tree was accepted before the fields existed. Nor is
// a layout that is not a node's: its DependsOn is the layout's, not the
// model's.
func (p *integratedPlacement) checkKeptNodeCR(host, child *layout.ManifestLayout, name string, kept existingCR, scope sourceScope) error {
	nodes := child.OriginNodes()
	if len(nodes) == 0 || (len(nodes[0].DependsOn) == 0 && len(nodes[0].NamedDependsOn) == 0) {
		return nil
	}
	deps, err := p.layoutDependsOn(host, child, scope)
	if err != nil {
		return err
	}
	var missing []string
	for _, dep := range deps {
		if !slices.Contains(kept.dependsOn, dep) {
			missing = append(missing, dep)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return errors.Errorf("layout %q already has Flux Kustomization %q for node %q with spec.dependsOn %q, which lacks %q that the node's DependsOn and NamedDependsOn ask for: the kept Kustomization would not wait for them; remove it, or add them to it",
		host.FullRepoPath(), name, p.nodes.path(nodes[0]), kept.dependsOn, missing)
}

// layoutsNamed returns the application and augmenter layouts among layouts
// that are named name and get a CR of their own; with deep, also those below
// them, not descending into a layout that renders bundles (another unit).
func layoutsNamed(layouts []*layout.ManifestLayout, name string, deep bool) []*layout.ManifestLayout {
	var found []*layout.ManifestLayout
	for _, l := range layouts {
		if l == nil {
			continue
		}
		if hasLayoutCR(l) && len(l.OriginNodes()) == 0 && l.Name == name {
			found = append(found, l)
		}
		if deep && len(l.OriginBundles()) == 0 {
			found = append(found, layoutsNamed(l.Children, name, true)...)
		}
	}
	return found
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
// error when it does not. A directory this call creates names its files by
// fileNaming, the rules'; left unset, by ml's own.
func (li *LayoutIntegrator) addSeparateFluxToLayout(ml *layout.ManifestLayout, c *stack.Cluster, fileNaming layout.FileNamingMode) error {
	fluxResources, ix, err := li.Generator.generateFromLayout(ml, c)
	if err != nil {
		return errors.ResourceValidationError("Cluster", c.Name, "flux-resources",
			"failed to generate Flux resources", err)
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
	// The directory must be free. The root node's bundle is rendered in a
	// directory named after it (go-kure/kure#979; by its DirName when it
	// sets one), and a child node in one named after the node: either of
	// them named like the Flux directory
	// would share it with the Flux resources, which the writers refuse only
	// when the tree is written. The directories are compared as the writers
	// compare them (SameDirectory): resolved under the output directory,
	// cleaned, and without regard to case.
	fluxPlace := &layout.ManifestLayout{Name: DefaultFluxDirName, Namespace: ml.FullRepoPath()}
	for _, child := range ml.Children {
		if !child.SameDirectory(fluxPlace) {
			continue
		}
		// A node's directory is named after the node, whatever it renders.
		switch {
		case len(child.OriginNodes()) > 0:
			return errors.Errorf("node %q is rendered to %q, the directory the Flux resources are written to under FluxSeparate: rename the node, or use an integrated placement",
				child.OriginNodes()[0].Name, child.FullRepoPath())
		case len(child.OriginBundles()) > 0:
			return errors.Errorf("bundle %q is rendered to %q, the directory the Flux resources are written to under FluxSeparate: give the bundle's directory another name (its DirName, or its Name without one), or use an integrated placement",
				child.OriginBundles()[0].Name, child.FullRepoPath())
		}
	}
	// The flux-system directory is applied beside the rest of the tree, so
	// a generated Kustomization's identity must not already be taken there
	// (an earlier flux-system child is compared as a whole below). The
	// refusal says whose the one in the tree is (heldBy) and what names the
	// generated one apart (nameApart): every Kustomization generated here is
	// a unit's, named after the first bundle its directory renders
	// (generateForUnit).
	existing, err := indexExistingKustomizations(ml, fluxDir)
	if err != nil {
		return err
	}
	owners := map[string]string{}
	for _, unit := range ix.Units() {
		first := unit.OriginBundles()[0]
		owners[first.UnitName()] = bundleOwner(first)
	}
	for _, obj := range fluxResources {
		path, ok := fluxKustomizationPath(obj)
		if !ok {
			continue
		}
		key := crKey(obj.GetNamespace(), obj.GetName())
		if e, dup := existing[key]; dup {
			owner := owners[obj.GetName()]
			return errors.Errorf("layout %q already has Flux Kustomization %q (spec.path %q)%s; the one generated for %s (spec.path %q) would register the same id in the kustomize build: %s",
				e.host.FullRepoPath(), obj.GetName(), e.path, heldBy(ix, key, e.host), owner, path, nameApart(bundleNameField, owner))
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
			return markFluxBuilds(ml, generated)
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
	//
	// FileNaming is the rules', so the directory's files are named like the
	// rest of the tree; rules that leave it unset take ml's, the parent's
	// (go-kure/kure#976).
	if fileNaming == layout.FileNamingUnset {
		fileNaming = ml.FileNaming
	}
	fluxLayout := &layout.ManifestLayout{
		Name:       DefaultFluxDirName,
		Namespace:  ml.FullRepoPath(),
		FilePer:    layout.DefaultLayoutRules().FilePer,
		FileNaming: fileNaming,
		Resources:  fluxResources,
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
	if err := markFluxBuilds(ml, generated); err != nil {
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
// The default is read from layout.DefaultLayoutRules rather than named here,
// because that function is where the layout package declares it and
// layout.WalkCluster resolves an unset placement from the same call. A
// constant in this package would be a second copy of a value this package
// does not own.
func normalizeRulesPlacement(rules layout.LayoutRules) layout.LayoutRules {
	if rules.FluxPlacement == layout.FluxUnset {
		rules.FluxPlacement = layout.DefaultLayoutRules().FluxPlacement
	}
	return rules
}

// generatedKustomizations returns, in typed form and in the order of l's
// resources, a List's in the List's place, the Flux Kustomizations of l that
// this integration placed or kept (generated: their namespace/name keys). It
// reads l as kustomize builds it (resourceItems), so one the integration kept
// in place of its own (add) counts in whatever form it has: typed,
// unstructured or inside a List. One that is not typed is converted through
// its JSON, and a conversion failure is an error naming the layout and the
// object, never a Kustomization the integrator's checks leave out
// (go-kure/kure#979).
func generatedKustomizations(l *layout.ManifestLayout, generated map[string]bool) ([]*kustv1.Kustomization, error) {
	objs, err := resourceItems(l)
	if err != nil {
		return nil, err
	}
	var out []*kustv1.Kustomization
	for _, obj := range objs {
		if _, ok := fluxKustomizationPath(obj); !ok {
			continue
		}
		key := crKey(obj.GetNamespace(), obj.GetName())
		if !generated[key] {
			continue
		}
		k, ok := obj.(*kustv1.Kustomization)
		if !ok {
			raw, err := json.Marshal(obj)
			if err == nil {
				k = &kustv1.Kustomization{}
				err = json.Unmarshal(raw, k)
			}
			if err != nil {
				return nil, errors.Wrapf(err, "layout %q: read Flux Kustomization %q in typed form", l.FullRepoPath(), key)
			}
		}
		out = append(out, k)
	}
	return out, nil
}
