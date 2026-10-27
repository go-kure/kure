package fluxcd_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestKustomizeBuildsAsControllerPin holds kure's kustomize to the version
// kustomize-controller v1.10.1 pins (api/kyaml v0.21.1, by the replace
// directives in go.mod; go-kure/kure#1147). It builds one tree the two
// versions build differently: labels with includeTemplates on a StatefulSet.
// v0.21.1, which the controller applies with, labels the StatefulSet's
// volumeClaimTemplates too; v0.21.2, which minimum-version selection picks
// without the replace directives, labels them only with
// includeVolumeClaimTemplates. So the test fails if the replace directives are
// dropped or move to a version the controller does not pin. A coordinated Flux
// upgrade that moves the controller's pin changes this test with it.
func TestKustomizeBuildsAsControllerPin(t *testing.T) {
	tree := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(tree, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("kustomization.yaml", `resources:
- sts.yaml
labels:
- pairs:
    tier: db
  includeTemplates: true
`)
	write("sts.yaml", `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: db
spec:
  serviceName: db
  template:
    spec:
      containers:
      - name: db
        image: db
  volumeClaimTemplates:
  - metadata:
      name: data
`)

	rm, err := buildLikeFlux(t, tree, tree)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rm.AsYaml()
	if err != nil {
		t.Fatal(err)
	}
	want := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    tier: db
  name: db
spec:
  serviceName: db
  template:
    metadata:
      labels:
        tier: db
    spec:
      containers:
      - image: db
        name: db
  volumeClaimTemplates:
  - metadata:
      labels:
        tier: db
      name: data
`
	if string(got) != want {
		t.Errorf("build differs from kustomize-controller's kustomize (v0.21.1), which labels volumeClaimTemplates under includeTemplates:\n--- got\n%s\n--- want\n%s", got, want)
	}
}
