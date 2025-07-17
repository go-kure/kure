package fluxcd

import (
	"reflect"
	"testing"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestKustomizationSettersAndAdders(t *testing.T) {
	k := CreateKustomization("demo", "default", kustv1.KustomizationSpec{})
	iv := metav1.Duration{Duration: time.Minute}
	rv := metav1.Duration{Duration: time.Second}
	SetKustomizationInterval(k, iv)
	SetKustomizationRetryInterval(k, rv)
	SetKustomizationPath(k, "./manifests")
	SetKustomizationPrune(k, true)
	SetKustomizationDeletionPolicy(k, "Delete")
	AddKustomizationComponent(k, "comp")
	AddKustomizationDependsOn(k, meta.NamespacedObjectReference{Name: "other"})
	AddKustomizationHealthCheck(k, meta.NamespacedObjectKindReference{Kind: "Deployment", Name: "app"})
	AddKustomizationImage(k, kustomize.Image{Name: "nginx", NewTag: "1"})
	AddKustomizationPatch(k, kustomize.Patch{Patch: "data"})
	if k.Spec.Interval != iv {
		t.Errorf("interval not set")
	}
	if k.Spec.RetryInterval == nil || *k.Spec.RetryInterval != rv {
		t.Errorf("retry interval not set")
	}
	if k.Spec.Path != "./manifests" || !k.Spec.Prune || k.Spec.DeletionPolicy != "Delete" {
		t.Errorf("basic fields not set")
	}
	if len(k.Spec.Components) != 1 || k.Spec.Components[0] != "comp" {
		t.Errorf("component not added")
	}
	if len(k.Spec.DependsOn) != 1 || k.Spec.DependsOn[0].Name != "other" {
		t.Errorf("dependsOn not added")
	}
	if len(k.Spec.HealthChecks) != 1 || k.Spec.HealthChecks[0].Name != "app" {
		t.Errorf("health check not added")
	}
	if len(k.Spec.Images) != 1 || k.Spec.Images[0].Name != "nginx" {
		t.Errorf("image not added")
	}
	if len(k.Spec.Patches) != 1 || k.Spec.Patches[0].Patch != "data" {
		t.Errorf("patch not added")
	}
}

func TestPostBuildAndCommonMetadataHelpers(t *testing.T) {
	pb := CreatePostBuild()
	AddPostBuildSubstitute(pb, "VAR", "value")
	ref := CreateSubstituteReference("ConfigMap", "vars", false)
	AddPostBuildSubstituteFrom(pb, ref)
	if pb.Substitute["VAR"] != "value" {
		t.Errorf("substitute not added")
	}
	if len(pb.SubstituteFrom) != 1 || pb.SubstituteFrom[0].Name != "vars" {
		t.Errorf("substituteFrom not added")
	}

	cm := CreateCommonMetadata()
	AddCommonMetadataLabel(cm, "app", "demo")
	AddCommonMetadataAnnotation(cm, "owner", "team")
	if cm.Labels["app"] != "demo" || cm.Annotations["owner"] != "team" {
		t.Errorf("common metadata not updated")
	}

	dec := CreateDecryption("sops", &meta.LocalObjectReference{Name: "sec"})
	if dec.Provider != "sops" || dec.SecretRef.Name != "sec" {
		t.Errorf("decryption not created")
	}
}
