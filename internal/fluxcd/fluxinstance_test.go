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
	if err := AddFluxInstanceComponent(fi, "source-controller"); err != nil {
		t.Fatalf("AddFluxInstanceComponent returned error: %v", err)
	}
	if err := SetFluxInstanceWait(fi, true); err != nil {
		t.Fatalf("SetFluxInstanceWait returned error: %v", err)
	}
	if len(fi.Spec.Components) != 1 || fi.Spec.Components[0] != "source-controller" {
		t.Errorf("component not added")
	}
	if fi.Spec.Wait == nil || !*fi.Spec.Wait {
		t.Errorf("wait not set")
	}
}

func TestSetFluxInstanceDistribution(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	dist := fluxv1.Distribution{
		Version:  "2.x",
		Registry: "ghcr.io/fluxcd",
	}
	if err := SetFluxInstanceDistribution(fi, dist); err != nil {
		t.Fatalf("SetFluxInstanceDistribution returned error: %v", err)
	}
	if fi.Spec.Distribution.Version != "2.x" {
		t.Errorf("distribution version not set")
	}
}

func TestSetFluxInstanceCommonMetadata(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	cm := &fluxv1.CommonMetadata{
		Labels: map[string]string{"app": "flux"},
	}
	if err := SetFluxInstanceCommonMetadata(fi, cm); err != nil {
		t.Fatalf("SetFluxInstanceCommonMetadata returned error: %v", err)
	}
	if fi.Spec.CommonMetadata == nil || fi.Spec.CommonMetadata.Labels["app"] != "flux" {
		t.Errorf("common metadata not set")
	}
}

func TestSetFluxInstanceCluster(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	cluster := &fluxv1.Cluster{
		Type:   "kubernetes",
		Domain: "cluster.local",
	}
	if err := SetFluxInstanceCluster(fi, cluster); err != nil {
		t.Fatalf("SetFluxInstanceCluster returned error: %v", err)
	}
	if fi.Spec.Cluster == nil || fi.Spec.Cluster.Domain != "cluster.local" {
		t.Errorf("cluster not set")
	}
}

func TestSetFluxInstanceSharding(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	shard := &fluxv1.Sharding{
		Key: "shard-key",
	}
	if err := SetFluxInstanceSharding(fi, shard); err != nil {
		t.Fatalf("SetFluxInstanceSharding returned error: %v", err)
	}
	if fi.Spec.Sharding == nil || fi.Spec.Sharding.Key != "shard-key" {
		t.Errorf("sharding not set")
	}
}

func TestSetFluxInstanceStorage(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	storage := &fluxv1.Storage{
		Class: "standard",
		Size:  "10Gi",
	}
	if err := SetFluxInstanceStorage(fi, storage); err != nil {
		t.Fatalf("SetFluxInstanceStorage returned error: %v", err)
	}
	if fi.Spec.Storage == nil || fi.Spec.Storage.Class != "standard" {
		t.Errorf("storage not set")
	}
}

func TestSetFluxInstanceKustomize(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	kustomize := &fluxv1.Kustomize{}
	if err := SetFluxInstanceKustomize(fi, kustomize); err != nil {
		t.Fatalf("SetFluxInstanceKustomize returned error: %v", err)
	}
	if fi.Spec.Kustomize == nil {
		t.Errorf("kustomize not set")
	}
}

func TestSetFluxInstanceMigrateResources(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	if err := SetFluxInstanceMigrateResources(fi, true); err != nil {
		t.Fatalf("SetFluxInstanceMigrateResources returned error: %v", err)
	}
	if fi.Spec.MigrateResources == nil || !*fi.Spec.MigrateResources {
		t.Errorf("migrate resources not set")
	}
}

func TestSetFluxInstanceSync(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	sync := &fluxv1.Sync{
		Kind: "GitRepository",
		URL:  "https://github.com/org/repo",
		Path: "./clusters",
	}
	if err := SetFluxInstanceSync(fi, sync); err != nil {
		t.Fatalf("SetFluxInstanceSync returned error: %v", err)
	}
	if fi.Spec.Sync == nil || fi.Spec.Sync.URL != "https://github.com/org/repo" {
		t.Errorf("sync not set")
	}
}
