package fluxcd

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/go-kure/kure/pkg/errors"
	pubfluxcd "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// ResourceGenerator implements the workflow.ResourceGenerator interface for Flux.
// It focuses purely on generating Flux CRDs from stack components.
type ResourceGenerator struct {
	// DefaultInterval is the default reconciliation interval for generated resources
	DefaultInterval time.Duration
	// DefaultNamespace is the default namespace for generated Flux resources
	DefaultNamespace string
	// Prune is the garbage-collection input for Kustomizations generated from a
	// layout.ManifestLayout (FluxIntegratedPerLayout mode), which carries no
	// prune setting of its own. Kustomizations generated from a stack.Bundle
	// use that bundle's own Prune and ignore this field. nil emits
	// prune: false — see pruneValue.
	Prune *bool
}

// NewResourceGenerator creates a FluxCD resource generator seeded with the
// exported defaults ([DefaultInterval], [DefaultNamespace]).
// Assign the fields afterwards to override any of them; nothing else is
// injected into generated resources.
func NewResourceGenerator() *ResourceGenerator {
	return &ResourceGenerator{
		DefaultInterval:  DefaultInterval,
		DefaultNamespace: DefaultNamespace,
	}
}

// GenerateFromCluster creates Flux Kustomizations and Sources from a cluster
// definition. It runs stack.ValidateCluster first to fail fast on structural
// errors (umbrella cycles, disjointness violations, etc.), then walks the
// cluster with layout.DefaultLayoutRules and generates from that layout (see
// GenerateFromLayout).
//
// The spec.path values are therefore the directories WalkCluster writes under
// the default rules: the root node at <root>, its children at <root>/<child>.
// Callers that write the layout with other rules must generate from the
// layout they write instead — CreateLayoutWithResources, or GenerateFromLayout
// on their own WalkCluster result. The walk renders every application and
// runs every LayoutAugmenter, so their errors surface here.
func (g *ResourceGenerator) GenerateFromCluster(c *stack.Cluster) ([]client.Object, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}
	if err := stack.ValidateCluster(c); err != nil {
		return nil, err
	}
	ml, err := layout.WalkCluster(c, layout.DefaultLayoutRules())
	if err != nil {
		return nil, errors.ResourceValidationError("Cluster", c.Name, "layout",
			fmt.Sprintf("failed to walk the cluster with the default layout rules: %v", err), err)
	}
	return g.GenerateFromLayout(ml, c)
}

// GenerateFromLayout creates one Kustomization (and, when its SourceRef has a
// URL, a Source) for every reconciliation unit of the layout tree root — every
// directory that renders bundles — in layout pre-order. root must have been
// walked from c (layout.WalkCluster): layout.IndexOrigins refuses anything
// else. Each spec.path is that directory, relative to the writer's output
// root; the bundles a grouping axis merged into it share the Kustomization
// (see generateForUnit).
func (g *ResourceGenerator) GenerateFromLayout(root *layout.ManifestLayout, c *stack.Cluster) ([]client.Object, error) {
	if root == nil || c == nil || c.Node == nil {
		return nil, nil
	}
	ix, err := layout.IndexOrigins(root, c)
	if err != nil {
		return nil, err
	}
	var out []client.Object
	var kusts []*kustv1.Kustomization
	for _, l := range ix.Units() {
		objs, err := g.generateForUnit(l, ix)
		if err != nil {
			return nil, err
		}
		for _, o := range objs {
			if k, ok := o.(*kustv1.Kustomization); ok {
				kusts = append(kusts, k)
				out = append(out, o)
				continue
			}
			// Bundles in different units may share one URL-bearing
			// SourceRef: the Source is emitted once, and two different
			// definitions of one Source are refused.
			if same := findObject(out, o); same != nil {
				if sameObject(same, o) {
					continue
				}
				return nil, errors.Errorf("%s %q is defined twice with different content: bundles %q and %q name one Source differently",
					o.GetObjectKind().GroupVersionKind().Kind, o.GetName(), sourceOwner(out, same), l.OriginBundles()[0].Name)
			}
			out = append(out, o)
		}
	}
	if err := checkReconcileOrder(kusts, nil, nil); err != nil {
		return nil, err
	}
	return out, nil
}

// generateForUnit creates the one Kustomization (and Source) for layout l, a
// directory that renders bundles. It is named after l's first bundle. Bundles
// merged into l must agree on every setting a Kustomization holds once; their
// health checks, labels, annotations and patches are combined, a patch only
// when its target selects none of the other bundles' objects
// (checkPatchScope); and
// spec.dependsOn is the units they depend on (OriginIndex.UnitDependencies)
// plus their NamedDependsOn.
func (g *ResourceGenerator) generateForUnit(l *layout.ManifestLayout, ix *layout.OriginIndex) ([]client.Object, error) {
	bundles := l.OriginBundles()
	if len(bundles) == 0 {
		return nil, nil
	}
	path := l.FullRepoPath()
	first := bundles[0]
	obj, err := g.kustomizationForBundle(first, path)
	if err != nil {
		return nil, err
	}
	unit := obj.(*kustv1.Kustomization)
	for _, b := range bundles[1:] {
		o, err := g.kustomizationForBundle(b, path)
		if err != nil {
			return nil, err
		}
		if err := g.mergeIntoUnit(unit, o.(*kustv1.Kustomization), first, b, path); err != nil {
			return nil, err
		}
	}
	if len(bundles) > 1 {
		for _, b := range bundles {
			for _, p := range b.Patches {
				if p.Target == nil {
					other := first
					if b == first {
						other = bundles[1]
					}
					return nil, unitConflict(other, b, path, "patches", "an untargeted patch would apply to every merged bundle's objects")
				}
				if err := checkPatchScope(l, b, p.Target, path); err != nil {
					return nil, err
				}
			}
		}
	}
	// A health check on a Flux Kustomization this pass generates names a
	// bundle; it follows the merge to that bundle's unit, and one on the
	// unit itself is dropped (it would wait for itself).
	var checks []metaapi.NamespacedObjectKindReference
	for _, hc := range unit.Spec.HealthChecks {
		if hc.Kind == "Kustomization" && strings.HasPrefix(hc.APIVersion, kustv1.GroupVersion.Group+"/") &&
			effectiveNS(hc.Namespace, g.DefaultNamespace) == effectiveNS(g.DefaultNamespace, "") {
			hc.Name = ix.UnitOfName(hc.Name)
			if hc.Name == unit.Name {
				continue
			}
		}
		if !slices.ContainsFunc(checks, func(x metaapi.NamespacedObjectKindReference) bool { return reflect.DeepEqual(x, hc) }) {
			checks = append(checks, hc)
		}
	}
	unit.Spec.HealthChecks = checks

	unit.Spec.DependsOn = nil
	named := map[string]bool{}
	for _, name := range append(ix.UnitDependencies(l), ix.UnitNamedDependencies(l)...) {
		if !named[name] {
			named[name] = true
			unit.Spec.DependsOn = append(unit.Spec.DependsOn, kustv1.DependencyReference{Name: name})
		}
	}
	resources := []client.Object{unit}
	if first.SourceRef != nil {
		source, err := g.createSource(first.SourceRef, first.Name)
		if err != nil {
			return nil, errors.ResourceValidationError("Bundle", first.Name, "source",
				fmt.Sprintf("failed to create source: %v", err), err)
		}
		if source != nil {
			resources = append(resources, source)
		}
	}
	return resources, nil
}

// mergeIntoUnit folds other, the Kustomization bundle b would have had on its
// own, into unit, the one for its directory.
func (g *ResourceGenerator) mergeIntoUnit(unit, other *kustv1.Kustomization, first, b *stack.Bundle, path string) error {
	same := []struct {
		field string
		a, b  any
	}{
		{"sourceRef", g.effectiveSourceRef(first.SourceRef), g.effectiveSourceRef(b.SourceRef)},
		{"interval", unit.Spec.Interval, other.Spec.Interval},
		{"timeout", unit.Spec.Timeout, other.Spec.Timeout},
		{"retryInterval", unit.Spec.RetryInterval, other.Spec.RetryInterval},
		{"prune", unit.Spec.Prune, other.Spec.Prune},
		{"wait", unit.Spec.Wait, other.Spec.Wait},
		{"force", unit.Spec.Force, other.Spec.Force},
		{"suspend", unit.Spec.Suspend, other.Spec.Suspend},
		{"postBuild", unit.Spec.PostBuild, other.Spec.PostBuild},
	}
	for _, s := range same {
		if !reflect.DeepEqual(s.a, s.b) {
			return unitConflict(first, b, path, s.field, "one Kustomization holds one value")
		}
	}
	for field, maps := range map[string][2]map[string]string{
		"labels":      {unit.Labels, other.Labels},
		"annotations": {unit.Annotations, other.Annotations},
	} {
		merged, err := unionStrings(maps[0], maps[1])
		if err != nil {
			return unitConflict(first, b, path, field, err.Error())
		}
		if field == "labels" {
			unit.Labels = merged
		} else {
			unit.Annotations = merged
		}
	}
	for _, hc := range other.Spec.HealthChecks {
		if !slices.ContainsFunc(unit.Spec.HealthChecks, func(x metaapi.NamespacedObjectKindReference) bool { return reflect.DeepEqual(x, hc) }) {
			unit.Spec.HealthChecks = append(unit.Spec.HealthChecks, hc)
		}
	}
	unit.Spec.Patches = append(unit.Spec.Patches, other.Spec.Patches...)
	return nil
}

// effectiveNS is namespace as Kubernetes reads it: an omitted one is fallback,
// or "default" when fallback is empty too.
func effectiveNS(namespace, fallback string) string {
	if namespace != "" {
		return namespace
	}
	if fallback != "" {
		return fallback
	}
	return "default"
}

// sourceOwner names the unit whose generated objects include obj.
func sourceOwner(out []client.Object, obj client.Object) string {
	owner := ""
	for _, o := range out {
		if k, ok := o.(*kustv1.Kustomization); ok {
			owner = k.Name
		}
		if o == obj {
			return owner
		}
	}
	return owner
}

// effectiveSourceRef is ref as the generated objects use it: an omitted
// namespace is the generator's DefaultNamespace (createSource, and the
// Kustomization's own namespace for its sourceRef).
func (g *ResourceGenerator) effectiveSourceRef(ref *stack.SourceRef) *stack.SourceRef {
	if ref == nil {
		return nil
	}
	out := *ref
	if out.Namespace == "" {
		out.Namespace = g.DefaultNamespace
	}
	if out.URL == "" {
		// Without a URL no Source is generated: only the reference's kind,
		// name and namespace reach an object.
		out.Tag, out.Branch = "", ""
	}
	return &out
}

// unionStrings merges two string maps, refusing one key with two values.
func unionStrings(a, b map[string]string) (map[string]string, error) {
	if len(a) == 0 && len(b) == 0 {
		return a, nil
	}
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if prev, ok := out[k]; ok && prev != v {
			return nil, errors.Errorf("key %q is %q and %q", k, prev, v)
		}
		out[k] = v
	}
	return out, nil
}

// unitConflict is the refusal for bundles merged into one directory whose
// Kustomization cannot express both.
func unitConflict(first, b *stack.Bundle, path, field, why string) error {
	return errors.ResourceValidationError("Bundle", b.Name, field,
		fmt.Sprintf("bundles %q and %q render one directory %q and so share one Kustomization, but their %s differ (%s); give them directories of their own (NodeGrouping or BundleGrouping GroupByName) or align them",
			first.Name, b.Name, path, field, why), nil)
}

// checkPatchScope refuses a patch of bundle b whose target selects an object
// another bundle of unit l renders. Flux applies a Kustomization's patches to
// everything it builds, and the unit builds every merged bundle's objects, so
// such a patch would change objects its bundle does not own — something it
// did not do while b had a directory of its own. The target is matched the
// way kustomize matches it: group, version, kind, name and namespace as
// anchored regular expressions (an empty one matches anything), label and
// annotation selectors as Kubernetes selector expressions. Objects without a
// kind cannot be matched and are skipped.
func checkPatchScope(l *layout.ManifestLayout, b *stack.Bundle, t *stack.PatchSelector, path string) error {
	sel := kusttypes.Selector{
		ResId: resid.ResId{
			Gvk:       resid.Gvk{Group: t.Group, Version: t.Version, Kind: t.Kind},
			Name:      t.Name,
			Namespace: t.Namespace,
		},
		LabelSelector:      t.LabelSelector,
		AnnotationSelector: t.AnnotationSelector,
	}
	sr, err := kusttypes.NewSelectorRegex(&sel)
	if err != nil {
		return errors.Wrapf(err, "bundle %q: patch target %s", b.Name, sel.String())
	}
	labelSel, err := labels.Parse(t.LabelSelector)
	if err != nil {
		return errors.Wrapf(err, "bundle %q: patch target label selector", b.Name)
	}
	annotationSel, err := labels.Parse(t.AnnotationSelector)
	if err != nil {
		return errors.Wrapf(err, "bundle %q: patch target annotation selector", b.Name)
	}
	for _, other := range l.OriginBundles() {
		if other == b {
			continue
		}
		for _, o := range l.OriginBundleObjects(other) {
			gvk := o.GetObjectKind().GroupVersionKind()
			if gvk.Kind == "" ||
				!sr.MatchGvk(resid.Gvk{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind}) ||
				!sr.MatchName(o.GetName()) || !sr.MatchNamespace(o.GetNamespace()) ||
				!labelSel.Matches(labels.Set(o.GetLabels())) ||
				!annotationSel.Matches(labels.Set(o.GetAnnotations())) {
				continue
			}
			return errors.ResourceValidationError("Bundle", b.Name, "patches",
				fmt.Sprintf("bundles %q and %q render one directory %q and so share one Kustomization, which applies every patch to everything it builds: bundle %q's patch target %s also selects %s %q of bundle %q; narrow the target to %q's own objects, or give the bundles directories of their own (NodeGrouping or BundleGrouping GroupByName)",
					b.Name, other.Name, path, b.Name, sel.String(), gvk.Kind, objectName(o), other.Name, b.Name), nil)
		}
	}
	return nil
}

// objectName is obj's namespace/name, or its name when it has no namespace.
func objectName(obj client.Object) string {
	if obj.GetNamespace() == "" {
		return obj.GetName()
	}
	return obj.GetNamespace() + "/" + obj.GetName()
}

// GenerateForBundle creates the Flux resources for b itself: a Kustomization
// whose spec.path is path, verbatim, and a Source when b.SourceRef has a URL.
// Umbrella Children are not recursed. The generator computes no path: take
// it from the layout that renders b (layout.OriginIndex.KustomizationPath).
func (g *ResourceGenerator) GenerateForBundle(b *stack.Bundle, path string) ([]client.Object, error) {
	if b == nil {
		return nil, nil
	}

	kustomization, err := g.kustomizationForBundle(b, path)
	if err != nil {
		return nil, err
	}
	resources := []client.Object{kustomization}

	if b.SourceRef != nil {
		source, err := g.createSource(b.SourceRef, b.Name)
		if err != nil {
			return nil, errors.ResourceValidationError("Bundle", b.Name, "source",
				fmt.Sprintf("failed to create source: %v", err), err)
		}
		if source != nil {
			resources = append(resources, source)
		}
	}

	return resources, nil
}

// kustomizationForBundle creates a Flux Kustomization resource from a bundle,
// with spec.path set to path. An empty Interval takes g.DefaultInterval and an empty Timeout or
// RetryInterval leaves the field unset; a non-empty value that does not parse
// is an error, never a silent fallback. Bundle.Validate reports the same
// error earlier; checking here as well covers callers that generate without
// validating first.
func (g *ResourceGenerator) kustomizationForBundle(b *stack.Bundle, path string) (client.Object, error) {
	interval := g.DefaultInterval
	if b.Interval != "" {
		d, err := parseBundleDuration(b, "interval", b.Interval)
		if err != nil {
			return nil, err
		}
		interval = d
	}

	// Prune is a declared tri-state input, passed through untouched. An unset
	// Prune emits prune: false — see pruneValue for why nil cannot mean "leave
	// the key out".

	kust := &kustv1.Kustomization{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kustv1.GroupVersion.String(),
			Kind:       "Kustomization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        b.Name,
			Namespace:   g.DefaultNamespace,
			Labels:      b.Labels,
			Annotations: b.Annotations,
		},
		Spec: kustv1.KustomizationSpec{
			Interval: metav1.Duration{Duration: interval},
			Path:     path,
			Prune:    pruneValue(b.Prune),
			Wait:     waitValue(b.Wait),
		},
	}

	// Set timeout if specified
	if b.Timeout != "" {
		d, err := parseBundleDuration(b, "timeout", b.Timeout)
		if err != nil {
			return nil, err
		}
		kust.Spec.Timeout = &metav1.Duration{Duration: d}
	}

	// Set retry interval if specified
	if b.RetryInterval != "" {
		d, err := parseBundleDuration(b, "retryInterval", b.RetryInterval)
		if err != nil {
			return nil, err
		}
		kust.Spec.RetryInterval = &metav1.Duration{Duration: d}
	}

	// Set force if specified
	if b.Force != nil {
		kust.Spec.Force = *b.Force
	}

	// Set suspend if specified
	if b.Suspend != nil {
		kust.Spec.Suspend = *b.Suspend
	}

	// Set source reference
	if b.SourceRef != nil {
		kust.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
			Kind: b.SourceRef.Kind,
			Name: b.SourceRef.Name,
		}
		if b.SourceRef.Namespace != "" {
			kust.Spec.SourceRef.Namespace = b.SourceRef.Namespace
		}
	}

	// Umbrella bundles: prepend an auto HealthCheck for each child
	// Kustomization, so the umbrella is Ready only when every child is.
	// User-supplied HealthChecks are appended AFTER the auto entries.
	//
	// Wait is no longer forced to true here. It was, and that made these
	// HealthChecks dead weight: upstream documents that "when enabled, the
	// HealthChecks are ignored" (kustomize-controller
	// api/v1/kustomization_types.go:175-177). Leaving Wait to the caller's
	// tri-state input is what makes the entries below take effect.
	if len(b.Children) > 0 {
		b.InitializeUmbrella()
		for _, child := range b.Children {
			if child == nil {
				continue
			}
			kust.Spec.HealthChecks = append(kust.Spec.HealthChecks, metaapi.NamespacedObjectKindReference{
				APIVersion: kustv1.GroupVersion.String(),
				Kind:       "Kustomization",
				Name:       child.Name,
				Namespace:  g.DefaultNamespace,
			})
		}
	}

	// Append user-specified health checks. For umbrella bundles, these come
	// AFTER the auto entries emitted above.
	for _, hc := range b.HealthChecks {
		kust.Spec.HealthChecks = append(kust.Spec.HealthChecks, metaapi.NamespacedObjectKindReference{
			APIVersion: hc.APIVersion,
			Kind:       hc.Kind,
			Name:       hc.Name,
			Namespace:  hc.Namespace,
		})
	}

	// Apply patches
	for _, p := range b.Patches {
		patch := kustomize.Patch{Patch: p.Patch}
		if p.Target != nil {
			patch.Target = &kustomize.Selector{
				Group:              p.Target.Group,
				Version:            p.Target.Version,
				Kind:               p.Target.Kind,
				Name:               p.Target.Name,
				Namespace:          p.Target.Namespace,
				LabelSelector:      p.Target.LabelSelector,
				AnnotationSelector: p.Target.AnnotationSelector,
			}
		}
		kust.Spec.Patches = append(kust.Spec.Patches, patch)
	}

	// Apply postBuild variable substitution
	if b.PostBuild != nil {
		pb := &kustv1.PostBuild{}
		if len(b.PostBuild.Substitute) > 0 {
			pb.Substitute = b.PostBuild.Substitute
		}
		for _, ref := range b.PostBuild.SubstituteFrom {
			pb.SubstituteFrom = append(pb.SubstituteFrom, kustv1.SubstituteReference{
				Kind:     ref.Kind,
				Name:     ref.Name,
				Optional: ref.Optional,
			})
		}
		kust.Spec.PostBuild = pb
	}

	// Add dependencies
	for _, dep := range b.DependsOn {
		kust.Spec.DependsOn = append(kust.Spec.DependsOn, kustv1.DependencyReference{
			Name: dep.Name,
		})
	}
	for _, name := range b.NamedDependsOn {
		kust.Spec.DependsOn = append(kust.Spec.DependsOn, kustv1.DependencyReference{
			Name: name,
		})
	}

	return kust, nil
}

// parseBundleDuration parses one of a bundle's duration fields, returning a
// validation error that names the field and the rejected value.
func parseBundleDuration(b *stack.Bundle, field, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, errors.ResourceValidationError("Bundle", b.Name, field,
			fmt.Sprintf("%s %q is not a valid duration: %v", field, value, err), err)
	}
	return d, nil
}

// createKustomizationForLayout creates a Flux Kustomization CR named name for a
// ManifestLayout child in FluxIntegratedPerLayout mode. spec.path is
// ml.FullRepoPath(); spec.dependsOn is populated from ml.DependsOn.
func (g *ResourceGenerator) createKustomizationForLayout(
	name string,
	ml *layout.ManifestLayout,
	sourceRef kustv1.CrossNamespaceSourceReference,
) client.Object {
	kust := &kustv1.Kustomization{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kustv1.GroupVersion.String(),
			Kind:       "Kustomization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.DefaultNamespace,
		},
		Spec: kustv1.KustomizationSpec{
			Interval:  metav1.Duration{Duration: g.DefaultInterval},
			Path:      ml.FullRepoPath(),
			Prune:     pruneValue(g.Prune),
			SourceRef: sourceRef,
		},
	}
	for _, dep := range ml.DependsOn {
		kust.Spec.DependsOn = append(kust.Spec.DependsOn,
			kustv1.DependencyReference{Name: dep})
	}
	return kust
}

// createSource creates a Flux source resource based on the source reference.
// When the SourceRef has a URL, the corresponding source CRD is created.
// When URL is empty, only a reference is used (the source already exists in the cluster).
func (g *ResourceGenerator) createSource(ref *stack.SourceRef, name string) (client.Object, error) {
	if ref.URL == "" {
		return nil, nil
	}

	namespace := ref.Namespace
	if namespace == "" {
		namespace = g.DefaultNamespace
	}

	switch ref.Kind {
	case "GitRepository":
		gr := pubfluxcd.CreateGitRepository(ref.Name, namespace)
		gr.Spec.URL = ref.URL
		gr.Spec.Interval = metav1.Duration{Duration: g.DefaultInterval}
		if ref.Branch != "" {
			pubfluxcd.SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Branch: ref.Branch})
		} else if ref.Tag != "" {
			pubfluxcd.SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Tag: ref.Tag})
		}
		return gr, nil
	case "OCIRepository":
		or := pubfluxcd.CreateOCIRepository(ref.Name, namespace)
		or.Spec.URL = ref.URL
		or.Spec.Interval = metav1.Duration{Duration: g.DefaultInterval}
		if ref.Tag != "" {
			pubfluxcd.SetOCIRepositoryReference(or, &sourcev1.OCIRepositoryRef{Tag: ref.Tag})
		}
		return or, nil
	default:
		return nil, errors.NewValidationError("kind", ref.Kind, "SourceRef",
			[]string{"GitRepository", "OCIRepository"})
	}
}

// GetName returns the name of this resource generator.
func (g *ResourceGenerator) GetName() string {
	return "FluxCD Resource Generator"
}

// GetVersion returns the version of this resource generator.
func (g *ResourceGenerator) GetVersion() string {
	return "v1.0.0"
}
