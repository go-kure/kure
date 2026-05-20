package helm

import (
	"sort"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HookGroup is a set of Helm manifests sharing the same hook phase and weight.
// Resources with no helm.sh/hook annotation have Phase="" and Weight=0 (main group).
type HookGroup struct {
	Phase     string
	Weight    int
	Resources []client.Object
}

// excludedHookPhases lists Helm hook phases that have no equivalent in a FluxCD
// GitOps lifecycle. Objects with these phases are dropped from SplitByHookWeight output.
var excludedHookPhases = map[string]bool{
	"pre-delete":    true,
	"post-delete":   true,
	"pre-rollback":  true,
	"post-rollback": true,
	"test":          true,
}

// SplitByHookWeight groups rendered Helm manifests by hook phase and weight for
// ordered FluxCD Kustomization generation. Groups are returned in execution order:
// pre-install, pre-upgrade, main (non-hook), post-install, post-upgrade, then any
// remaining unknown hook phases alphabetically.
//
// Objects whose helm.sh/hook phase has no FluxCD lifecycle equivalent
// (pre-delete, post-delete, pre-rollback, post-rollback, test) are excluded.
// Comma-separated hook annotations (e.g. "pre-install,post-install") are treated
// as a single opaque phase string and placed in the unknown group.
func SplitByHookWeight(objects []client.Object) []HookGroup {
	if len(objects) == 0 {
		return nil
	}
	type key struct {
		phase  string
		weight int
	}
	groupMap := map[key][]client.Object{}
	for _, obj := range objects {
		ann := obj.GetAnnotations()
		phase := ann["helm.sh/hook"]
		if excludedHookPhases[phase] {
			continue
		}
		weight := 0
		if w, err := strconv.Atoi(ann["helm.sh/hook-weight"]); err == nil {
			weight = w
		}
		k := key{phase, weight}
		groupMap[k] = append(groupMap[k], obj)
	}

	phaseOrder := func(phase string) int {
		switch phase {
		case "pre-install":
			return 0
		case "pre-upgrade":
			return 1
		case "":
			return 2
		case "post-install":
			return 3
		case "post-upgrade":
			return 4
		default:
			return 5
		}
	}

	keys := make([]key, 0, len(groupMap))
	for k := range groupMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, oj := phaseOrder(keys[i].phase), phaseOrder(keys[j].phase)
		if oi != oj {
			return oi < oj
		}
		if keys[i].phase != keys[j].phase {
			return keys[i].phase < keys[j].phase
		}
		return keys[i].weight < keys[j].weight
	})

	groups := make([]HookGroup, 0, len(keys))
	for _, k := range keys {
		groups = append(groups, HookGroup{Phase: k.phase, Weight: k.weight, Resources: groupMap[k]})
	}
	return groups
}
