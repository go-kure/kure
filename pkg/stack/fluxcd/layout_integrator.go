package fluxcd

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
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
// CR already present with the same name and spec.path is kept, one with the
// same name and another path is an error.
func (li *LayoutIntegrator) IntegrateWithLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	if ml == nil || c == nil || c.Node == nil {
		return nil
	}

	rules = normalizeRulesPlacement(rules)

	switch rules.FluxPlacement {
	case layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle:
		// Both place Flux CRs inline. They differ only in granularity:
		// PerLayout emits a CR for every layout node (incl. augmenter-added
		// child layouts); PerBundle stops at bundle boundaries and lets
		// kustomize include child directories.
		return li.addIntegratedFluxToLayout(ml, c, rules.FluxPlacement == layout.FluxIntegratedPerLayout)
	case layout.FluxSeparate:
		return li.addSeparateFluxToLayout(ml, c)
	default:
		return errors.NewValidationError("fluxPlacement", string(rules.FluxPlacement), "LayoutRules",
			[]string{string(layout.FluxIntegratedPerLayout), string(layout.FluxIntegratedPerBundle), string(layout.FluxSeparate)})
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
	// nodeOf maps a node bundle to its node (umbrella children have none).
	nodeOf map[*stack.Bundle]*stack.Node
	// names maps every Kustomization name this pass emitted to its
	// spec.path: Flux Kustomizations share one namespace, so a name is an
	// identity.
	names map[string]string
	// existing maps every Kustomization already in the tree (an earlier
	// integration's, or a caller's) to where it sits.
	existing map[string]existingCR
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
	// mixedAt is the layout whose bundles have different SourceRefs, which
	// therefore cannot source a layout CR.
	mixedAt string
}

// addIntegratedFluxToLayout places Flux Kustomizations alongside their target
// manifests in one walk over the layout tree.
//
// Each bundle's CR (and Source) is generated with spec.path = the directory of
// the layout that renders it. Host: under PerLayout the parent of that layout
// (the root hosts its own), the same rule as every other PerLayout child CR;
// under PerBundle the node's layout for a node bundle and the enclosing parent
// for an umbrella child.
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
		nodeOf:    map[*stack.Bundle]*stack.Node{},
		names:     map[string]string{},
		existing:  map[string]existingCR{},
	}
	if err := p.indexExisting(ml); err != nil {
		return err
	}
	var index func(n *stack.Node)
	index = func(n *stack.Node) {
		if n == nil {
			return
		}
		if n.Bundle != nil {
			p.nodeOf[n.Bundle] = n
		}
		for _, ch := range n.Children {
			index(ch)
		}
	}
	index(c.Node)
	return p.place(ml, sourceScope{})
}

func (p *integratedPlacement) place(l *layout.ManifestLayout, inherited sourceScope) error {
	scope := inherited
	bundles := l.OriginBundles()
	if len(bundles) > 0 {
		scope = sourceScope{ref: sourceRefOf(bundles[0])}
		for _, b := range bundles[1:] {
			if sourceRefOf(b) != scope.ref {
				scope.mixedAt = l.FullRepoPath()
			}
		}
	}

	for _, b := range bundles {
		path, err := p.ix.KustomizationPath(b)
		if err != nil {
			return err
		}
		objs, err := p.gen.GenerateForBundle(b, path)
		if err != nil {
			return errors.ResourceValidationError("Bundle", b.Name, "flux-resources",
				fmt.Sprintf("failed to generate Flux resources: %v", err), err)
		}
		if err := p.add(p.host(l, b), objs); err != nil {
			return err
		}
	}

	if p.perLayout {
		for _, child := range l.Children {
			if child == nil || child.UmbrellaChild || child.ApplicationFileMode == layout.AppFileSingle || len(child.OriginBundles()) > 0 {
				continue
			}
			name := layoutCRName(child)
			if e, ok := p.existing[name]; ok && e.host == l && e.path == child.FullRepoPath() {
				// Placed by an earlier integration: kept as is, so its
				// source need not be resolved again.
				if err := p.claim(name, e.path); err != nil {
					return err
				}
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
func (p *integratedPlacement) host(l *layout.ManifestLayout, b *stack.Bundle) *layout.ManifestLayout {
	if !p.perLayout {
		if n, ok := p.nodeOf[b]; ok {
			return p.ix.NodeLayout(n)
		}
	}
	if parent := p.ix.Parent(l); parent != nil {
		return parent
	}
	return l
}

// layoutSource returns the SourceRef of child's layout CR: the scope's (the
// nearest bundle-rendering layout at or above the host), else the one
// SourceRef the URL-less bundles below child share.
func (p *integratedPlacement) layoutSource(child *layout.ManifestLayout, scope sourceScope) (kustv1.CrossNamespaceSourceReference, error) {
	if scope.mixedAt != "" {
		return kustv1.CrossNamespaceSourceReference{}, errors.ResourceValidationError("ManifestLayout", child.Name, "sourceRef",
			fmt.Sprintf("layout %q renders bundles with different SourceRefs, so it cannot source the Flux Kustomization of its child layout %q", scope.mixedAt, child.FullRepoPath()), nil)
	}
	if scope.ref.Kind != "" && scope.ref.Name != "" {
		return scope.ref, nil
	}
	var refs []kustv1.CrossNamespaceSourceReference
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, b := range l.OriginBundles() {
			if b.SourceRef == nil || b.SourceRef.URL != "" {
				continue
			}
			ref := sourceRefOf(b)
			if ref.Kind != "" && ref.Name != "" && !slices.Contains(refs, ref) {
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
// another spec.path it is an error. A Source already present is kept.
func (p *integratedPlacement) add(host *layout.ManifestLayout, objs []client.Object) error {
	for _, obj := range objs {
		if k, ok := obj.(*kustv1.Kustomization); ok {
			if err := p.claim(k.Name, k.Spec.Path); err != nil {
				return err
			}
			if e, ok := p.existing[k.Name]; ok {
				if e.host == host && e.path == k.Spec.Path {
					continue
				}
				return errors.Errorf("layout %q already has Flux Kustomization %q with spec.path %q; this integration derives %q in layout %q", e.host.FullRepoPath(), k.Name, e.path, k.Spec.Path, host.FullRepoPath())
			}
		} else if hasObject(host.Resources, obj) {
			continue
		}
		host.Resources = append(host.Resources, obj)
	}
	return nil
}

// indexExisting records every Kustomization already in the tree. One name
// present twice is an identity collision before this pass adds anything.
func (p *integratedPlacement) indexExisting(l *layout.ManifestLayout) error {
	for _, r := range l.Resources {
		k, ok := r.(*kustv1.Kustomization)
		if !ok {
			continue
		}
		if prev, dup := p.existing[k.Name]; dup {
			return errors.Errorf("Flux Kustomization name %q is present twice (in layout %q with spec.path %q and in layout %q with spec.path %q): Kustomization names must be unique", k.Name, prev.host.FullRepoPath(), prev.path, l.FullRepoPath(), k.Spec.Path)
		}
		p.existing[k.Name] = existingCR{host: l, path: k.Spec.Path}
	}
	for _, c := range l.Children {
		if c == nil {
			continue
		}
		if err := p.indexExisting(c); err != nil {
			return err
		}
	}
	return nil
}

// claim records that this pass emits a Kustomization named name: a second
// claim of one name is a CR identity collision.
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

func hasObject(resources []client.Object, obj client.Object) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()
	for _, r := range resources {
		if r.GetObjectKind().GroupVersionKind() == gvk && r.GetNamespace() == obj.GetNamespace() && r.GetName() == obj.GetName() {
			return true
		}
	}
	return false
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

	for _, child := range ml.Children {
		if child == nil || child.Name != DefaultFluxDirName || child.OriginApplication() != nil ||
			len(child.OriginNodes()) > 0 || len(child.OriginBundles()) > 0 {
			continue
		}
		if reflect.DeepEqual(child.Resources, fluxResources) {
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
