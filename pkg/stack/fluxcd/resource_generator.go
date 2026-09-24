package fluxcd

import (
	"fmt"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// GenerateFromLayout creates a Kustomization (and, when its SourceRef has a
// URL, a Source) for every bundle the layout tree root renders, in layout
// pre-order. root must have been walked from c (layout.WalkCluster):
// layout.IndexOrigins refuses anything else. Each spec.path is the directory
// of the layout that renders the bundle (OriginIndex.KustomizationPath),
// relative to the writer's output root.
func (g *ResourceGenerator) GenerateFromLayout(root *layout.ManifestLayout, c *stack.Cluster) ([]client.Object, error) {
	if root == nil || c == nil || c.Node == nil {
		return nil, nil
	}
	ix, err := layout.IndexOrigins(root, c)
	if err != nil {
		return nil, err
	}
	var out []client.Object
	for _, b := range ix.Bundles() {
		path, err := ix.KustomizationPath(b)
		if err != nil {
			return nil, err
		}
		objs, err := g.GenerateForBundle(b, path)
		if err != nil {
			return nil, err
		}
		out = append(out, objs...)
	}
	return out, nil
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
