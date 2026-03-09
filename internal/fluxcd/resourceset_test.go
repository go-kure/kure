package fluxcd

import (
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestCreateResourceSet(t *testing.T) {
	rs := CreateResourceSet("rs", "ns", fluxv1.ResourceSetSpec{})
	if rs.Name != "rs" || rs.Namespace != "ns" {
		t.Fatalf("unexpected metadata")
	}
	if rs.TypeMeta.Kind != fluxv1.ResourceSetKind {
		t.Errorf("unexpected kind %s", rs.TypeMeta.Kind)
	}
}

func TestResourceSetHelpers(t *testing.T) {
	rs := CreateResourceSet("rs", "ns", fluxv1.ResourceSetSpec{})
	AddResourceSetInput(rs, fluxv1.ResourceSetInput{"k": &apiextensionsv1.JSON{Raw: []byte("1")}})
	AddResourceSetInputFrom(rs, fluxv1.InputProviderReference{Kind: fluxv1.ResourceSetInputProviderKind})
	AddResourceSetResource(rs, &apiextensionsv1.JSON{Raw: []byte("{}")})
	AddResourceSetDependency(rs, fluxv1.Dependency{Kind: "ConfigMap", Name: "cm"})
	SetResourceSetServiceAccountName(rs, "sa")
	SetResourceSetWait(rs, true)
	if len(rs.Spec.Inputs) != 1 {
		t.Errorf("input not added")
	}
	if len(rs.Spec.InputsFrom) != 1 {
		t.Errorf("input from not added")
	}
	if len(rs.Spec.Resources) != 1 {
		t.Errorf("resource not added")
	}
	if len(rs.Spec.DependsOn) != 1 {
		t.Errorf("dependency not added")
	}
	if rs.Spec.ServiceAccountName != "sa" {
		t.Errorf("sa not set")
	}
	if !rs.Spec.Wait {
		t.Errorf("wait not set")
	}
}
