package fluxcd

import (
	"fmt"
	"slices"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// layoutSettings are the settings in effect on a per-layout Kustomization,
// besides its name, path, source and dependencies: what the layout sets
// (ManifestLayout.Interval, Prune, Force, Suspend, Wait, Timeout,
// RetryInterval, Labels, Annotations) over what it inherits from the bundle
// that holds its application (layoutSettings on integratedPlacement). The
// postBuild substitution and the service account have no layout field: they
// are that bundle's. The bundle's patches are not among the settings: they
// are placed once every Kustomization is (placeBundlePatches).
type layoutSettings struct {
	interval           time.Duration
	prune              bool
	force              bool
	suspend            bool
	wait               bool
	timeout            *metav1.Duration
	retryInterval      *metav1.Duration
	labels             map[string]string
	annotations        map[string]string
	postBuild          *kustv1.PostBuild
	serviceAccountName string
}

// holdingBundle returns the bundle that holds the application l belongs to, or
// nil when l belongs to none. l belongs to an application when it is the
// application's own layout or a layout below it, at any depth: the walker
// gives an application's LayoutAugmenter that layout, and what the augmenter
// adds sits below it. The bundle is found by application membership among the
// bundles the directory above the application's renders, so where a grouping
// axis merged several bundles into that directory it is the one that lists
// the application, not the first of them.
//
// A node's own layout belongs to no application, and neither does a layout a
// caller added to the tree outside an application's: the walk up ends at the
// first layout that is a node's or renders bundles.
func (p *integratedPlacement) holdingBundle(l *layout.ManifestLayout) *stack.Bundle {
	for at := l; at != nil; at = p.ix.Parent(at) {
		if len(at.OriginNodes()) > 0 || len(at.OriginBundles()) > 0 {
			return nil
		}
		app := at.OriginApplication()
		if app == nil {
			continue
		}
		if host := p.ix.Parent(at); host != nil {
			for _, b := range host.OriginBundles() {
				if slices.Contains(b.Applications, app) {
					return b
				}
			}
		}
		return nil
	}
	return nil
}

// layoutSettings returns the settings in effect on l's per-layout
// Kustomization, and refuses those Flux or the Kubernetes API would: an
// interval, timeout or retry interval that is no duration, a label or
// annotation the API does not accept. The refusal names the layout by its
// directory and, for a value the layout inherits, the bundle it comes from.
//
// A scalar the layout sets replaces the bundle's; one it leaves unset is the
// bundle's. Labels and annotations are the bundle's with the layout's on top,
// key by key. Without a holding bundle (holdingBundle) the layout's own are
// all there is.
//
// Where neither sets one, interval and prune are the generator's
// (ResourceGenerator.DefaultInterval, ResourceGenerator.Prune), as they were
// for every per-layout Kustomization before the layout and the bundle were
// read (go-kure/kure#1021): a tree that sets neither renders as it did. The
// generator's interval is checked only where it is the one written
// (checkDefaultInterval), as for a bundle's own Kustomization.
//
// The postBuild substitution is the holding bundle's, whole: its inline
// variables and its substituteFrom references as they are. Flux reads a
// referenced ConfigMap or Secret from the Kustomization's namespace, and
// every Kustomization of the pass is in the generator's, so a reference the
// bundle's own Kustomization resolves is one this one resolves.
//
// The service account is the holding bundle's too, with no layout field: the
// layout's objects are the bundle's application's, applied under the account
// the bundle names. Flux looks it up in the Kustomization's namespace, the
// same for every Kustomization of the pass. The bundle's Validate has checked
// the name.
func (p *integratedPlacement) layoutSettings(l *layout.ManifestLayout) (layoutSettings, error) {
	holder := p.holdingBundle(l)
	inherit := stack.Bundle{}
	if holder != nil {
		inherit = *holder
	}

	s := layoutSettings{
		prune:              pruneValue(firstSet(l.Prune, inherit.Prune, p.gen.Prune)),
		force:              isTrue(firstSet(l.Force, inherit.Force)),
		suspend:            isTrue(firstSet(l.Suspend, inherit.Suspend)),
		wait:               waitValue(firstSet(l.Wait, inherit.Wait)),
		postBuild:          fluxPostBuild(inherit.PostBuild),
		serviceAccountName: inherit.ServiceAccountName,
	}

	var err error
	if l.Interval == "" && inherit.Interval == "" {
		if err = checkDefaultInterval("ResourceGenerator", p.gen.DefaultInterval); err != nil {
			return layoutSettings{}, err
		}
		s.interval = p.gen.DefaultInterval
	} else {
		interval, err := layoutDuration(l, holder, "interval", l.Interval, inherit.Interval)
		if err != nil {
			return layoutSettings{}, err
		}
		s.interval = interval.Duration
	}
	if s.timeout, err = layoutDuration(l, holder, "timeout", l.Timeout, inherit.Timeout); err != nil {
		return layoutSettings{}, err
	}
	if s.retryInterval, err = layoutDuration(l, holder, "retryInterval", l.RetryInterval, inherit.RetryInterval); err != nil {
		return layoutSettings{}, err
	}
	if s.labels, err = layoutMetadata(l, holder, "labels", l.Labels, inherit.Labels, func(m map[string]string) field.ErrorList {
		return metav1validation.ValidateLabels(m, field.NewPath("metadata", "labels"))
	}); err != nil {
		return layoutSettings{}, err
	}
	if s.annotations, err = layoutMetadata(l, holder, "annotations", l.Annotations, inherit.Annotations, func(m map[string]string) field.ErrorList {
		return apivalidation.ValidateAnnotations(m, field.NewPath("metadata", "annotations"))
	}); err != nil {
		return layoutSettings{}, err
	}
	return s, nil
}

// firstSet returns the first of values that is set, or nil when none is: a
// tri-state setting the layout sets, or else the one it inherits.
func firstSet(values ...*bool) *bool {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

// isTrue resolves a tri-state force or suspend input to the bool the upstream
// field holds. Both are optional with omitempty, so unset and false are the
// same emitted YAML: the key is absent.
func isTrue(v *bool) bool {
	return v != nil && *v
}

// layoutDuration resolves one duration setting of l's per-layout
// Kustomization: own, the layout's, or else inherited, the holding bundle's.
// Neither set leaves the field out. A value that does not parse, and one
// Flux does not take in a Kustomization (fluxDuration), is refused with the
// layout's directory and, when it is inherited, the bundle, as
// parseBundleDuration refuses a bundle's own.
func layoutDuration(l *layout.ManifestLayout, holder *stack.Bundle, name, own, inherited string) (*metav1.Duration, error) {
	value, from := own, ""
	if value == "" && inherited != "" {
		value, from = inherited, inheritedFrom(holder)
	}
	if value == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return nil, errors.ResourceValidationError("ManifestLayout", l.FullRepoPath(), name,
			fmt.Sprintf("%s %q%s is not a valid duration", name, value, from), err)
	}
	if written, ok := fluxDuration(d); !ok {
		return nil, errors.ResourceValidationError("ManifestLayout", l.FullRepoPath(), name,
			durationRefusal(name, value, from, written), nil)
	}
	return &metav1.Duration{Duration: d}, nil
}

// layoutMetadata resolves the labels or annotations (name) of l's per-layout
// Kustomization: inherited, the holding bundle's, with own, the layout's, on
// top. Each entry is checked on its own with validate, the apimachinery
// validator for that map, so that the refusal names the key and where it
// comes from; the merged map is then checked once more as a whole, for what
// only the whole map can break (the total size of the annotations).
func layoutMetadata(l *layout.ManifestLayout, holder *stack.Bundle, name string, own, inherited map[string]string, validate func(map[string]string) field.ErrorList) (map[string]string, error) {
	if len(own) == 0 && len(inherited) == 0 {
		return nil, nil
	}
	merged := make(map[string]string, len(own)+len(inherited))
	from := map[string]string{}
	for k, v := range inherited {
		merged[k] = v
		from[k] = inheritedFrom(holder)
	}
	for k, v := range own {
		merged[k] = v
		delete(from, k)
	}
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if errs := validate(map[string]string{k: merged[k]}); len(errs) > 0 {
			return nil, errors.ResourceValidationError("ManifestLayout", l.FullRepoPath(), name,
				fmt.Sprintf("%s entry %q%s is not one the Kubernetes API accepts on the layout's Flux Kustomization: %v", name, k, from[k], errs.ToAggregate()), nil)
		}
	}
	if errs := validate(merged); len(errs) > 0 {
		return nil, errors.ResourceValidationError("ManifestLayout", l.FullRepoPath(), name,
			fmt.Sprintf("the %s of the layout's Flux Kustomization are not what the Kubernetes API accepts: %v", name, errs.ToAggregate()), nil)
	}
	return merged, nil
}

// inheritedFrom words where an inherited setting comes from, for a refusal.
func inheritedFrom(holder *stack.Bundle) string {
	return fmt.Sprintf(" (inherited from bundle %q)", holder.GetPath())
}
