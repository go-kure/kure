package fluxcd

import (
	"fmt"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// layoutKustomizationFields lists the settings of its own Flux Kustomization
// layout l sets, in the order ManifestLayout declares them. An empty list or
// map sets nothing.
func layoutKustomizationFields(l *layout.ManifestLayout) []string {
	var set []string
	for _, f := range []struct {
		name string
		set  bool
	}{
		{"DependsOn", len(l.DependsOn) > 0},
		{"KustomizationName", l.KustomizationName != ""},
		{"Wait", l.Wait != nil},
		{"Timeout", l.Timeout != ""},
		{"RetryInterval", l.RetryInterval != ""},
		{"Labels", len(l.Labels) > 0},
		{"Annotations", len(l.Annotations) > 0},
		{"Interval", l.Interval != ""},
		{"Prune", l.Prune != nil},
		{"Force", l.Force != nil},
		{"Suspend", l.Suspend != nil},
	} {
		if f.set {
			set = append(set, f.name)
		}
	}
	return set
}

// checkLayoutFields refuses a layout of the tree under root that sets one of
// the settings of its own Flux Kustomization (layoutKustomizationFields) but
// gets none: the settings would be dropped (go-kure/kure#1032). The first such
// layout in pre-order is named, with every setting it sets.
//
// noneHere is empty where this generation places per-layout Kustomizations,
// which only LayoutIntegrator under FluxIntegratedPerLayout does. A layout
// then gets one exactly where place gives it one: a child layout for which
// hasLayoutCR holds. Anywhere else noneHere says why no layout gets one.
//
// It runs after nodeIndex.checkFields. The walker copies a node's
// KustomizationName and NamedDependsOn onto the node's own layout, and a node
// that sets them where it gets no Kustomization is refused there, by its
// path: a layout that reaches this check carries a node's value only where
// the node's Kustomization is the layout's own, so a value is never refused
// twice.
func checkLayoutFields(root *layout.ManifestLayout, noneHere string) error {
	var walk func(l *layout.ManifestLayout, top bool) error
	walk = func(l *layout.ManifestLayout, top bool) error {
		if set := layoutKustomizationFields(l); len(set) > 0 {
			if why := noLayoutKustomization(l, top, noneHere); why != "" {
				return errors.Errorf("layout %q sets %s, but has no Flux Kustomization of its own: %s. Only LayoutIntegrator under FluxPlacement %q (FluxIntegratedPerLayout) carries these fields, on the Kustomization it gives a child layout that renders no bundle and is neither an umbrella child nor AppFileSingle; unset them on this layout",
					l.FullRepoPath(), strings.Join(set, ", "), why, string(layout.FluxIntegratedPerLayout))
			}
		}
		for _, child := range l.Children {
			if child == nil {
				continue
			}
			if err := walk(child, false); err != nil {
				return err
			}
		}
		return nil
	}
	if root == nil {
		return nil
	}
	return walk(root, true)
}

// noLayoutKustomization says why layout l gets no Flux Kustomization of its
// own, or "" when it gets one. top says whether l is the root of the tree;
// noneHere is checkLayoutFields'. The cases after hasLayoutCR are the terms
// of hasLayoutCR, so they cannot disagree with place.
func noLayoutKustomization(l *layout.ManifestLayout, top bool, noneHere string) string {
	switch {
	case noneHere != "":
		return noneHere
	case top:
		return "it is the root of the tree, which the Flux bootstrap applies"
	case hasLayoutCR(l):
		return ""
	case len(l.OriginBundles()) > 0:
		return fmt.Sprintf("it renders bundle %q, and that bundle's Kustomization applies it", l.OriginBundles()[0].GetPath())
	case l.UmbrellaChild:
		return "it is an umbrella child, which its bundle's Kustomization applies"
	default:
		return "it is AppFileSingle, written as a file into its parent's directory"
	}
}
