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

func TestFluxReportNilGuards(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"SetFluxReportDistribution", func() error { return SetFluxReportDistribution(nil, fluxv1.FluxDistributionStatus{}) }},
		{"SetFluxReportCluster", func() error { return SetFluxReportCluster(nil, nil) }},
		{"SetFluxReportOperator", func() error { return SetFluxReportOperator(nil, nil) }},
		{"AddFluxReportComponentStatus", func() error { return AddFluxReportComponentStatus(nil, fluxv1.FluxComponentStatus{}) }},
		{"AddFluxReportReconcilerStatus", func() error {
			return AddFluxReportReconcilerStatus(nil, fluxv1.FluxReconcilerStatus{})
		}},
		{"SetFluxReportSyncStatus", func() error { return SetFluxReportSyncStatus(nil, nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Errorf("%s(nil) should return error", tt.name)
			}
		})
	}
}

func TestFluxReportHelpers(t *testing.T) {
	fr := CreateFluxReport("flux", "ns", fluxv1.FluxReportSpec{})
	if err := AddFluxReportComponentStatus(fr, fluxv1.FluxComponentStatus{Name: "source-controller"}); err != nil {
		t.Fatalf("AddFluxReportComponentStatus returned error: %v", err)
	}
	if err := AddFluxReportReconcilerStatus(fr, fluxv1.FluxReconcilerStatus{Kind: "Kustomization"}); err != nil {
		t.Fatalf("AddFluxReportReconcilerStatus returned error: %v", err)
	}
	if len(fr.Spec.ComponentsStatus) != 1 {
		t.Errorf("component status not added")
	}
	if len(fr.Spec.ReconcilersStatus) != 1 {
		t.Errorf("reconciler status not added")
	}
}
