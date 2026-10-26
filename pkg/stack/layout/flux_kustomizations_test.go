package layout_test

import (
	"reflect"
	"testing"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// SetFluxKustomizations keeps a copy of what it is given and
// FluxKustomizations returns a copy of what it keeps, so neither slice
// changes the record afterwards; a layout nothing recorded on returns nil.
func TestFluxKustomizations_RecordIsCopied(t *testing.T) {
	ml := &layout.ManifestLayout{Name: "apps"}
	if got := ml.FluxKustomizations(); got != nil {
		t.Fatalf("FluxKustomizations of a new layout = %v, want nil", got)
	}
	rec := layout.FluxKustomizationRecord{
		Namespace: "flux-system", Name: "apps", Path: "clusters/demo/apps",
		SourceKind: "GitRepository", SourceNamespace: "flux-system", SourceName: "flux-system",
	}
	given := []layout.FluxKustomizationRecord{rec}
	ml.SetFluxKustomizations(given)
	given[0].Name = "changed"
	got := ml.FluxKustomizations()
	if want := []layout.FluxKustomizationRecord{rec}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FluxKustomizations = %v, want %v", got, want)
	}
	got[0].KubeConfig = true
	if again := ml.FluxKustomizations(); !reflect.DeepEqual(again, []layout.FluxKustomizationRecord{rec}) {
		t.Errorf("FluxKustomizations after editing the returned slice = %v, want %v", again, []layout.FluxKustomizationRecord{rec})
	}
	ml.SetFluxKustomizations(nil)
	if got := ml.FluxKustomizations(); got != nil {
		t.Errorf("FluxKustomizations after recording nil = %v, want nil", got)
	}
}
