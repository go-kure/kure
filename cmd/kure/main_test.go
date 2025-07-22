package main

import (
	"github.com/go-kure/kure/pkg/appsets"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "file-*.yaml")
	if err != nil {
		t.Fatalf("temp create: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return f.Name()
}

func TestApplyPatch(t *testing.T) {
	base := `apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
  labels:
    app: demo
data:
  foo: bar
`
	patch := `- target: demo
  patch:
    data.foo: baz
    metadata.labels.env: prod
`
	basePath := writeTempFile(t, base)
	patchPath := writeTempFile(t, patch)

	objs, err := appsets.ApplyPatch(basePath, patchPath)
	if err != nil {
		t.Fatalf("appsets.ApplyPatch: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	labels, found, err := unstructured.NestedStringMap(objs[0].Object, "metadata", "labels")
	if err != nil || !found {
		t.Fatalf("labels missing")
	}
	if labels["env"] != "prod" {
		t.Fatalf("patch not applied: %+v", labels)
	}
}

func TestRunPatchMissingArgs(t *testing.T) {
	err := runPatch([]string{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

<<<<<<< HEAD
func TestRunClusterMissingConfig(t *testing.T) {
	if err := runCluster([]string{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunCluster(t *testing.T) {
	cfg := `name: demo
interval: 5m
sourceRef: flux-system
appGroups:
- name: apps
  namespace: default
  apps:
  - name: myapp
`
	cfgPath := writeTempFile(t, cfg)
	out := t.TempDir()
	manifests := filepath.Join(out, "manifests")
	flux := filepath.Join(out, "flux")

	err := runCluster([]string{"--config", cfgPath, "--manifests", manifests, "--flux", flux})
	if err != nil {
		t.Fatalf("runCluster: %v", err)
	}
	if _, err := os.Stat(filepath.Join(manifests, "clusters", "demo")); err != nil {
		t.Fatalf("manifests not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(flux, "clusters", "demo")); err != nil {
		t.Fatalf("flux not written: %v", err)

func TestRunCluster(t *testing.T) {
	cfg := `name: demo
interval: 5m
sourceRef: flux-system
appGroups:
  - name: apps
    namespace: default
    apps:
      - name: demo
        image: nginx
`
	cfgPath := writeTempFile(t, cfg)
	tmp := t.TempDir()
	manifests := filepath.Join(tmp, "manifests")
	flux := filepath.Join(tmp, "flux")

	if err := runCluster([]string{"--config", cfgPath, "--manifests", manifests, "--flux", flux}); err != nil {
		t.Fatalf("runCluster: %v", err)
	}
	exp := filepath.Join(flux, "clusters", "demo", "clusters", "demo", "kustomization-flux-system.yaml")
	if _, err := os.Stat(exp); err != nil {
		t.Fatalf("expected file not written: %v", err)
	}
}

func TestRunClusterMissingConfig(t *testing.T) {
	err := runCluster([]string{})
	if err == nil {
		t.Fatalf("expected error")
	}
}
