package fluxcd

import (
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestCreateFluxReport(t *testing.T) {
	spec := fluxv1.FluxReportSpec{
		Distribution: fluxv1.FluxDistributionStatus{Entitlement: "oss", Status: "Running"},
	}
	fr := CreateFluxReport("flux", "flux-system", spec)
	if fr.Name != "flux" || fr.Namespace != "flux-system" {
		t.Fatalf("unexpected metadata")
	}
	if fr.TypeMeta.Kind != fluxv1.FluxReportKind {
		t.Errorf("unexpected kind %s", fr.TypeMeta.Kind)
	}
	if fr.Spec.Distribution.Entitlement != "oss" {
		t.Errorf("distribution not set")
	}
}

func TestFluxReportHelpers(t *testing.T) {
	fr := CreateFluxReport("flux", "ns", fluxv1.FluxReportSpec{})
	AddFluxReportComponentStatus(fr, fluxv1.FluxComponentStatus{Name: "source-controller"})
	AddFluxReportReconcilerStatus(fr, fluxv1.FluxReconcilerStatus{Kind: "Kustomization"})
	if len(fr.Spec.ComponentsStatus) != 1 {
		t.Errorf("component status not added")
	}
	if len(fr.Spec.ReconcilersStatus) != 1 {
		t.Errorf("reconciler status not added")
	}
}
