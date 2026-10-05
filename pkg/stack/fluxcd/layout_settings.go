package fluxcd

import (
	"fmt"
	"slices"
	"time"

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
// (ManifestLayout.Wait, Timeout, RetryInterval, Labels, Annotations) over what
// it inherits from the bundle that holds its application (layoutSettings on
// integratedPlacement). Interval and prune are not among them: they stay the
// generator's (createKustomizationForLayout).
type layoutSettings struct {
	wait          bool
	timeout       *metav1.Duration
	retryInterval *metav1.Duration
	labels        map[string]string
	annotations   map[string]string
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
// Kustomization, and refuses those Flux or the Kubernetes API would: a timeout
// or retry interval that is no duration, a label or annotation the API does
// not accept. The refusal names the layout by its directory and, for a value
// the layout inherits, the bundle it comes from.
//
// A scalar the layout sets replaces the bundle's; one it leaves unset is the
// bundle's. Labels and annotations are the bundle's with the layout's on top,
// key by key. Without a holding bundle (holdingBundle) the layout's own are
// all there is.
func (p *integratedPlacement) layoutSettings(l *layout.ManifestLayout) (layoutSettings, error) {
	holder := p.holdingBundle(l)
	inherit := stack.Bundle{}
	if holder != nil {
		inherit = *holder
	}

	wait := l.Wait
	if wait == nil {
		wait = inherit.Wait
	}
	s := layoutSettings{wait: waitValue(wait)}

	var err error
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

// layoutDuration resolves one duration setting of l's per-layout
// Kustomization: own, the layout's, or else inherited, the holding bundle's.
// Neither set leaves the field out. A value that does not parse is refused
// with the layout's directory, as parseBundleDuration refuses a bundle's.
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
			fmt.Sprintf("%s %q%s is not a valid duration: %v", name, value, from, err), err)
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
