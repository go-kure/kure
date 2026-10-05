package kinds

import (
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/errors"
)

// skippedWrappers names the registered kinds that get no generated constructor,
// each with the reason.
//
// A wrapper is named after its kind alone, so two versions of one kind routed
// to one package would both be Create<Kind>. One of them keeps the name; the
// other is named here. It stays registered: its manifests still parse to their
// typed object and it keeps its row in the generated kind table. A caller that
// needs to build one constructs the upstream type directly.
//
// This is the only reason an entry may exist. [applySkips] refuses one that
// does not resolve such a clash, so the table cannot be used to drop a
// constructor that nothing else stands in for.
var skippedWrappers = map[schema.GroupVersionKind]string{
	{Group: "metallb.io", Version: "v1beta1", Kind: "BGPPeer"}: "deprecated upstream; CreateBGPPeer builds metallb.io/v1beta2, the stored version",
}

// applySkips marks, in place, the kinds the given table names.
//
// Every entry has to earn its place: it names a registered kind, it states a
// reason, and another registered kind in the same package — same Kind name, not
// itself skipped — takes the wrapper name. An entry failing any of these is an
// error rather than a silently missing constructor: the generator emits
// nothing for a skipped kind, so nothing downstream would go red.
func applySkips(all []Kind, skipped map[schema.GroupVersionKind]string) error {
	index := make(map[schema.GroupVersionKind]int, len(all))
	for i, k := range all {
		index[k.GVK] = i
	}
	// Sorted, so the entry an error names does not depend on map order.
	named := make([]schema.GroupVersionKind, 0, len(skipped))
	for gvk := range skipped {
		named = append(named, gvk)
	}
	sort.Slice(named, func(i, j int) bool { return named[i].String() < named[j].String() })

	for _, gvk := range named {
		i, ok := index[gvk]
		if !ok {
			return errors.Errorf("kinds: skippedWrappers names %s, which is not registered in the scheme; remove the entry", gvk)
		}
		if skipped[gvk] == "" {
			return errors.Errorf("kinds: skippedWrappers names %s and states no reason", gvk)
		}
		all[i].WrapperSkipped = true
	}
	for _, gvk := range named {
		k := all[index[gvk]]
		if !hasWrappedNamesake(all, k) {
			return errors.Errorf("kinds: skippedWrappers names %s, but no other registered kind in package %q takes the name Create%s; a kind is skipped only to resolve that clash",
				gvk, k.Package, k.GVK.Kind)
		}
	}
	return nil
}

// hasWrappedNamesake reports whether a kind other than k, in k's package and
// with k's Kind name, keeps its wrapper.
func hasWrappedNamesake(all []Kind, k Kind) bool {
	for _, other := range all {
		if other.GVK != k.GVK && other.Package == k.Package && other.GVK.Kind == k.GVK.Kind && !other.WrapperSkipped {
			return true
		}
	}
	return false
}

// Wrapped returns the kinds that get a generated constructor: all of them but
// the ones [Kind.WrapperSkipped] marks, in the order given. The slice it was
// handed is left as it is.
func Wrapped(all []Kind) []Kind {
	out := make([]Kind, 0, len(all))
	for _, k := range all {
		if !k.WrapperSkipped {
			out = append(out, k)
		}
	}
	return out
}
