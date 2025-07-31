package kubernetes

import (
	"reflect"
	"testing"

	"sigs.k8s.io/kustomize/api/types"
)

func TestCreateKustomizationFile(t *testing.T) {
	k := CreateKustomizationFile()
	if k.Kind != types.KustomizationKind {
		t.Errorf("unexpected kind %q", k.Kind)
	}
	if k.APIVersion != types.KustomizationVersion {
		t.Errorf("unexpected apiVersion %q", k.APIVersion)
	}
	if len(k.Resources) != 0 || len(k.Components) != 0 || len(k.Crds) != 0 {
		t.Errorf("expected empty lists")
	}
}

func TestKustomizationFileFunctions(t *testing.T) {
	k := CreateKustomizationFile()

	if err := AddKustomizationResource(k, "deployment.yaml"); err != nil {
		t.Fatalf("AddKustomizationResource returned error: %v", err)
	}
	if err := AddKustomizationComponent(k, "../base"); err != nil {
		t.Fatalf("AddKustomizationComponent returned error: %v", err)
	}
	if err := AddKustomizationCRD(k, "crd.yaml"); err != nil {
		t.Fatalf("AddKustomizationCRD returned error: %v", err)
	}
	img := types.Image{Name: "nginx", NewTag: "latest"}
	if err := AddKustomizationImage(k, img); err != nil {
		t.Fatalf("AddKustomizationImage returned error: %v", err)
	}
	patch := types.Patch{Path: "patch.yaml"}
	if err := AddKustomizationPatch(k, patch); err != nil {
		t.Fatalf("AddKustomizationPatch returned error: %v", err)
	}
	if err := SetKustomizationNamespace(k, "demo"); err != nil {
		t.Fatalf("SetKustomizationNamespace returned error: %v", err)
	}

	if !reflect.DeepEqual(k.Resources, []string{"deployment.yaml"}) {
		t.Errorf("resource not added")
	}
	if !reflect.DeepEqual(k.Components, []string{"../base"}) {
		t.Errorf("component not added")
	}
	if !reflect.DeepEqual(k.Crds, []string{"crd.yaml"}) {
		t.Errorf("crd not added")
	}
	if len(k.Images) != 1 || k.Images[0] != img {
		t.Errorf("image not added")
	}
	if len(k.Patches) != 1 || !k.Patches[0].Equals(patch) {
		t.Errorf("patch not added")
	}
	if k.Namespace != "demo" {
		t.Errorf("namespace not set")
	}

	if err := AddKustomizationResource(nil, "x"); err == nil {
		t.Errorf("expected error when kustomization nil")
	}
	if err := AddKustomizationComponent(nil, "x"); err == nil {
		t.Errorf("expected error when kustomization nil")
	}
	if err := AddKustomizationCRD(nil, "x"); err == nil {
		t.Errorf("expected error when kustomization nil")
	}
	if err := AddKustomizationImage(nil, img); err == nil {
		t.Errorf("expected error when kustomization nil")
	}
	if err := AddKustomizationPatch(nil, patch); err == nil {
		t.Errorf("expected error when kustomization nil")
	}
	if err := SetKustomizationNamespace(nil, "x"); err == nil {
		t.Errorf("expected error when kustomization nil")
	}
}
