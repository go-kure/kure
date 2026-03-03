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

func TestResourceSetNilGuards(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"AddResourceSetInput", func() error { return AddResourceSetInput(nil, fluxv1.ResourceSetInput{}) }},
		{"AddResourceSetInputFrom", func() error { return AddResourceSetInputFrom(nil, fluxv1.InputProviderReference{}) }},
		{"AddResourceSetResource_NilRS", func() error {
			return AddResourceSetResource(nil, &apiextensionsv1.JSON{Raw: []byte("{}")})
		}},
		{"SetResourceSetResourcesTemplate", func() error { return SetResourceSetResourcesTemplate(nil, "") }},
		{"AddResourceSetDependency", func() error { return AddResourceSetDependency(nil, fluxv1.Dependency{}) }},
		{"SetResourceSetServiceAccountName", func() error { return SetResourceSetServiceAccountName(nil, "") }},
		{"SetResourceSetWait", func() error { return SetResourceSetWait(nil, true) }},
		{"SetResourceSetCommonMetadata", func() error { return SetResourceSetCommonMetadata(nil, nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Errorf("%s(nil) should return error", tt.name)
			}
		})
	}
}

func TestResourceSetHelpers(t *testing.T) {
	rs := CreateResourceSet("rs", "ns", fluxv1.ResourceSetSpec{})
	if err := AddResourceSetInput(rs, fluxv1.ResourceSetInput{"k": &apiextensionsv1.JSON{Raw: []byte("1")}}); err != nil {
		t.Fatalf("AddResourceSetInput returned error: %v", err)
	}
	if err := AddResourceSetInputFrom(rs, fluxv1.InputProviderReference{Kind: fluxv1.ResourceSetInputProviderKind}); err != nil {
		t.Fatalf("AddResourceSetInputFrom returned error: %v", err)
	}
	if err := AddResourceSetResource(rs, &apiextensionsv1.JSON{Raw: []byte("{}")}); err != nil {
		t.Fatalf("AddResourceSetResource returned error: %v", err)
	}
	if err := AddResourceSetDependency(rs, fluxv1.Dependency{Kind: "ConfigMap", Name: "cm"}); err != nil {
		t.Fatalf("AddResourceSetDependency returned error: %v", err)
	}
	if err := SetResourceSetServiceAccountName(rs, "sa"); err != nil {
		t.Fatalf("SetResourceSetServiceAccountName returned error: %v", err)
	}
	if err := SetResourceSetWait(rs, true); err != nil {
		t.Fatalf("SetResourceSetWait returned error: %v", err)
	}
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
