package fluxcd

import (
	"reflect"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
)

func TestCreateKustomization(t *testing.T) {
	tests := []struct {
		name     string
		inName   string
		inNs     string
		inSpec   kustv1.KustomizationSpec
		wantName string
		wantNs   string
		wantSpec kustv1.KustomizationSpec
		wantKind string
		wantAPIV string
	}{
		{
			name:     "valid inputs create object correctly",
			inName:   "test-kustomization",
			inNs:     "default",
			inSpec:   kustv1.KustomizationSpec{Path: "./manifests", Prune: true},
			wantName: "test-kustomization",
			wantNs:   "default",
			wantSpec: kustv1.KustomizationSpec{Path: "./manifests", Prune: true},
			wantKind: "Kustomization",
			wantAPIV: kustv1.GroupVersion.String(),
		},
		{
			name:     "empty inputs produce valid object",
			inName:   "",
			inNs:     "",
			inSpec:   kustv1.KustomizationSpec{},
			wantName: "",
			wantNs:   "",
			wantSpec: kustv1.KustomizationSpec{},
			wantKind: "Kustomization",
			wantAPIV: kustv1.GroupVersion.String(),
		},
		{
			name:     "namespace only creates proper object",
			inName:   "",
			inNs:     "example-ns",
			inSpec:   kustv1.KustomizationSpec{},
			wantName: "",
			wantNs:   "example-ns",
			wantSpec: kustv1.KustomizationSpec{},
			wantKind: "Kustomization",
			wantAPIV: kustv1.GroupVersion.String(),
		},
		{
			name:     "non-empty spec creates valid object",
			inName:   "kustomization-with-spec",
			inNs:     "my-namespace",
			inSpec:   kustv1.KustomizationSpec{Prune: false, Images: []kustomize.Image{{Name: "nginx", NewTag: "latest"}}},
			wantName: "kustomization-with-spec",
			wantNs:   "my-namespace",
			wantSpec: kustv1.KustomizationSpec{Prune: false, Images: []kustomize.Image{{Name: "nginx", NewTag: "latest"}}},
			wantKind: "Kustomization",
			wantAPIV: kustv1.GroupVersion.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateKustomization(tt.inName, tt.inNs, tt.inSpec)

			if got.Name != tt.wantName {
				t.Errorf("expected Name %q, got %q", tt.wantName, got.Name)
			}
			if got.Namespace != tt.wantNs {
				t.Errorf("expected Namespace %q, got %q", tt.wantNs, got.Namespace)
			}
			if got.Kind != tt.wantKind {
				t.Errorf("expected Kind %q, got %q", tt.wantKind, got.Kind)
			}
			if got.APIVersion != tt.wantAPIV {
				t.Errorf("expected APIVersion %q, got %q", tt.wantAPIV, got.APIVersion)
			}
			if !reflect.DeepEqual(got.Spec, tt.wantSpec) {
				t.Errorf("expected Spec %+v, got %+v", tt.wantSpec, got.Spec)
			}
		})
	}
}
