package fluxcd

import (
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

// A spec.commonMetadata whose key or value holds "$" or "ENC[" may be applied
// otherwise than the tree shows it (a Kustomization read as the tree holds it
// is not substituted), so a copy under it is never read as excluded; a
// literal one is set over the copy's marks.
func TestObjectSource_ExcludesOnlyUnderLiteralCommonMetadata(t *testing.T) {
	copyIgnored := builtObject{group: "", kind: "ConfigMap", namespace: "default", name: "b-cm",
		annotations: map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}}
	for name, tc := range map[string]struct {
		labels, annotations map[string]string
		o                   builtObject
		want                bool
	}{
		"no commonMetadata":                 {o: copyIgnored, want: true},
		"a literal key over another":        {annotations: map[string]string{"team": "a"}, o: copyIgnored, want: true},
		"a literal Merge over Ignore":       {annotations: map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Merge"}, o: copyIgnored},
		"a key with a variable":             {annotations: map[string]string{"kustomize.toolkit.fluxcd.io/ssa${E}": "Merge"}, o: copyIgnored},
		"a label value with a variable":     {labels: map[string]string{"team": "${TEAM}"}, o: copyIgnored},
		"an encrypted value":                {annotations: map[string]string{"team": "ENC[x]"}, o: copyIgnored},
		"a literal Ignore over an object":   {annotations: map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}, o: builtObject{kind: "ConfigMap"}, want: true},
		"a copy written with a variable":    {o: builtObject{kind: "ConfigMap", annotations: copyIgnored.annotations, varies: true}},
		"literal Ignore over a varying one": {annotations: map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}, o: builtObject{kind: "ConfigMap", varies: true}},
	} {
		t.Run(name, func(t *testing.T) {
			k := &kustv1.Kustomization{}
			if tc.labels != nil || tc.annotations != nil {
				k.Spec.CommonMetadata = &kustv1.CommonMetadata{Labels: tc.labels, Annotations: tc.annotations}
			}
			if got := (objectSource{key: "flux-system/h", writer: true, spec: k}).excludes(tc.o); got != tc.want {
				t.Errorf("excludes = %v, want %v", got, tc.want)
			}
		})
	}
	// The bootstrap has no spec: only the copy's own marks count.
	if !(objectSource{}).excludes(copyIgnored) {
		t.Error("the bootstrap's copy marked Ignore is not excluded")
	}
}
