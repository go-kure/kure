package fluxcd

import (
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestCreateFluxInstance(t *testing.T) {
	spec := fluxv1.FluxInstanceSpec{
		Distribution: fluxv1.Distribution{Version: "2.x", Registry: "ghcr.io/fluxcd"},
	}
	fi := CreateFluxInstance("flux", "flux-system", spec)
	if fi.Name != "flux" || fi.Namespace != "flux-system" {
		t.Fatalf("unexpected metadata")
	}
	if fi.TypeMeta.Kind != fluxv1.FluxInstanceKind {
		t.Errorf("unexpected kind %s", fi.TypeMeta.Kind)
	}
	if fi.Spec.Distribution.Version != "2.x" {
		t.Errorf("distribution not set")
	}
}

func TestFluxInstanceHelpers(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{Distribution: fluxv1.Distribution{}})
	AddFluxInstanceComponent(fi, "source-controller")
	SetFluxInstanceWait(fi, true)
	if len(fi.Spec.Components) != 1 || fi.Spec.Components[0] != "source-controller" {
		t.Errorf("component not added")
	}
	if fi.Spec.Wait == nil || !*fi.Spec.Wait {
		t.Errorf("wait not set")
	}
}
