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

func TestSetFluxInstanceDistribution(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	dist := fluxv1.Distribution{
		Version:  "2.x",
		Registry: "ghcr.io/fluxcd",
	}
	SetFluxInstanceDistribution(fi, dist)
	if fi.Spec.Distribution.Version != "2.x" {
		t.Errorf("distribution version not set")
	}
}

func TestSetFluxInstanceCommonMetadata(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	cm := &fluxv1.CommonMetadata{
		Labels: map[string]string{"app": "flux"},
	}
	SetFluxInstanceCommonMetadata(fi, cm)
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
	SetFluxInstanceCluster(fi, cluster)
	if fi.Spec.Cluster == nil || fi.Spec.Cluster.Domain != "cluster.local" {
		t.Errorf("cluster not set")
	}
}

func TestSetFluxInstanceSharding(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	shard := &fluxv1.Sharding{
		Key: "shard-key",
	}
	SetFluxInstanceSharding(fi, shard)
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
	SetFluxInstanceStorage(fi, storage)
	if fi.Spec.Storage == nil || fi.Spec.Storage.Class != "standard" {
		t.Errorf("storage not set")
	}
}

func TestSetFluxInstanceKustomize(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	kustomize := &fluxv1.Kustomize{}
	SetFluxInstanceKustomize(fi, kustomize)
	if fi.Spec.Kustomize == nil {
		t.Errorf("kustomize not set")
	}
}

func TestSetFluxInstanceMigrateResources(t *testing.T) {
	fi := CreateFluxInstance("flux", "ns", fluxv1.FluxInstanceSpec{})
	SetFluxInstanceMigrateResources(fi, true)
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
	SetFluxInstanceSync(fi, sync)
	if fi.Spec.Sync == nil || fi.Spec.Sync.URL != "https://github.com/org/repo" {
		t.Errorf("sync not set")
	}
}
