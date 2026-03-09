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

	AddKustomizationResource(k, "deployment.yaml")
	AddKustomizationComponent(k, "../base")
	AddKustomizationCRD(k, "crd.yaml")
	img := types.Image{Name: "nginx", NewTag: "latest"}
	AddKustomizationImage(k, img)
	patch := types.Patch{Path: "patch.yaml"}
	AddKustomizationPatch(k, patch)
	SetKustomizationNamespace(k, "demo")

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
}
