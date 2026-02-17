package patch

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestParsePatchLine(t *testing.T) {
	op, err := ParsePatchLine("spec.template.spec.containers[+name=main]", map[string]interface{}{"image": "nginx"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if op.Op != "insertAfter" || op.Selector != "name=main" {
		t.Fatalf("unexpected op %+v", op)
	}
	if op.Path != "spec.template.spec.containers" {
		t.Fatalf("unexpected path %s", op.Path)
	}
}

func TestParsePatchLineIndexBased(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOp   string
		wantSel  string
		wantPath string
	}{
		{
			name:     "insert before index",
			input:    "spec.containers[-3]",
			wantOp:   "insertBefore",
			wantSel:  "3",
			wantPath: "spec.containers",
		},
		{
			name:     "insert after index",
			input:    "spec.containers[+2]",
			wantOp:   "insertAfter",
			wantSel:  "2",
			wantPath: "spec.containers",
		},
		{
			name:     "append to list",
			input:    "spec.containers[-]",
			wantOp:   "append",
			wantSel:  "",
			wantPath: "spec.containers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := ParsePatchLine(tt.input, map[string]interface{}{"name": "test"})
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if op.Op != tt.wantOp {
				t.Errorf("expected op %s, got %s", tt.wantOp, op.Op)
			}
			if op.Selector != tt.wantSel {
				t.Errorf("expected selector %s, got %s", tt.wantSel, op.Selector)
			}
			if op.Path != tt.wantPath {
				t.Errorf("expected path %s, got %s", tt.wantPath, op.Path)
			}
		})
	}
}

func TestParsePatchPath(t *testing.T) {
	cases := []struct {
		in   string
		want []PathPart
	}{
		{
			in: "spec.template.spec.containers[0].image",
			want: []PathPart{
				{Field: "spec"},
				{Field: "template"},
				{Field: "spec"},
				{Field: "containers", MatchType: "index", MatchValue: "0"},
				{Field: "image"},
			},
		},
		{
			in: "spec.containers[name=main].image",
			want: []PathPart{
				{Field: "spec"},
				{Field: "containers", MatchType: "key", MatchValue: "name=main"},
				{Field: "image"},
			},
		},
	}

	for _, tc := range cases {
		got, err := ParsePatchPath(tc.in)
		if err != nil {
			t.Fatalf("ParsePatchPath error: %v", err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("segments len mismatch for %s: got %d want %d", tc.in, len(got), len(tc.want))
		}
		for i, p := range got {
			if p != tc.want[i] {
				t.Fatalf("segment %d mismatch for %s: got %+v want %+v", i, tc.in, p, tc.want[i])
			}
		}
	}
}

func TestDeleteOperation(t *testing.T) {
	yamlStr := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
  labels:
    app: demo
`
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		t.Fatalf("yaml decode: %v", err)
	}
	obj := &unstructured.Unstructured{Object: m}
	op, err := ParsePatchLine("metadata.labels.app[delete]", "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := applyPatchOp(obj.Object, op); err != nil {
		t.Fatalf("apply: %v", err)
	}
	labels, _, _ := unstructured.NestedStringMap(obj.Object, "metadata", "labels")
	if _, ok := labels["app"]; ok {
		t.Fatalf("label not deleted")
	}
}

func TestExplicitTargetAndSmart(t *testing.T) {
	resYaml := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
data:
  foo: bar
`
	var rm map[string]interface{}
	if err := yaml.Unmarshal([]byte(resYaml), &rm); err != nil {
		t.Fatalf("yaml decode: %v", err)
	}
	patchYaml := "- target: demo\n  patch:\n    data.foo: baz\n"
	patches, err := LoadPatchFile(strings.NewReader(patchYaml))
	if err != nil {
		t.Fatalf("load patch: %v", err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch")
	}
	set, err := LoadPatchableAppSet([]io.Reader{strings.NewReader(resYaml)}, strings.NewReader(patchYaml))
	if err != nil {
		t.Fatalf("LoadPatchableAppSet: %v", err)
	}
	resolved, err := set.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved) != 1 || resolved[0].Name != "demo" {
		t.Fatalf("unexpected resolve result")
	}
	if err := resolved[0].Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	val, _, _ := unstructured.NestedString(resolved[0].Base.Object, "data", "foo")
	if val != "baz" {
		t.Fatalf("patch not applied")
	}
}

func TestWriteToFile_StrategicPatches(t *testing.T) {
	baseYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: "1"
  template:
    spec:
      containers:
      - name: main
        image: nginx:1.24
`

	docSet, err := LoadResourcesWithStructure(strings.NewReader(baseYAML))
	if err != nil {
		t.Fatalf("LoadResourcesWithStructure: %v", err)
	}

	strategicPatch := StrategicPatch{
		Patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		},
	}

	pas := &PatchableAppSet{
		Resources:   docSet.GetResources(),
		DocumentSet: docSet,
		Patches: []struct {
			Target    string
			Patch     PatchOp
			Strategic *StrategicPatch
		}{
			{Target: "my-app", Strategic: &strategicPatch},
		},
	}

	tmpFile := t.TempDir() + "/output.yaml"
	if err := pas.WriteToFile(tmpFile); err != nil {
		t.Fatalf("WriteToFile: %v", err)
	}

	content, err := io.ReadAll(mustOpen(t, tmpFile))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if !strings.Contains(string(content), "replicas: 3") {
		t.Fatalf("expected strategic patch to update replicas to 3, got:\n%s", content)
	}
}

// TestWriteToFile_StrategicPatchDoesNotCrossKinds verifies that a strategic
// patch targeting a Deployment named "my-app" is NOT applied to a Service
// also named "my-app" in the same file.
func TestWriteToFile_StrategicPatchDoesNotCrossKinds(t *testing.T) {
	baseYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: "1"
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: default
spec:
  type: ClusterIP
  ports:
  - port: "80"
`

	docSet, err := LoadResourcesWithStructure(strings.NewReader(baseYAML))
	if err != nil {
		t.Fatalf("LoadResourcesWithStructure: %v", err)
	}

	// Strategic patch targets deployment.my-app only
	strategicPatch := StrategicPatch{
		Patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"replicas": int64(5),
			},
		},
	}

	pas := &PatchableAppSet{
		Resources:   docSet.GetResources(),
		DocumentSet: docSet,
		KindLookup:  nil,
		Patches: []struct {
			Target    string
			Patch     PatchOp
			Strategic *StrategicPatch
		}{
			{Target: "deployment.my-app", Strategic: &strategicPatch},
		},
	}

	tmpFile := t.TempDir() + "/output.yaml"
	if err := pas.WriteToFile(tmpFile); err != nil {
		t.Fatalf("WriteToFile: %v", err)
	}

	content, err := io.ReadAll(mustOpen(t, tmpFile))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	out := string(content)

	// The Deployment should have replicas: 5
	if !strings.Contains(out, "replicas: 5") {
		t.Errorf("expected Deployment replicas to be patched to 5, got:\n%s", out)
	}

	// The Service should NOT have replicas at all
	// Split on --- and check each document
	docs := strings.Split(out, "---")
	for _, doc := range docs {
		if strings.Contains(doc, "kind: Service") && strings.Contains(doc, "replicas") {
			t.Errorf("Service should not have replicas field, but got:\n%s", doc)
		}
	}
}

func mustOpen(t *testing.T, path string) io.Reader {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// TestNewPatchableAppSet_StrategicDoesNotCrossKinds verifies that a strategic
// patch targeting "deployment.my-app" via NewPatchableAppSet only patches the
// Deployment, not a Service with the same name.
func TestNewPatchableAppSet_StrategicDoesNotCrossKinds(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "my-app",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
			},
		},
	}
	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "my-app",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"type": "ClusterIP",
			},
		},
	}

	resources := []*unstructured.Unstructured{deployment, service}

	patches := []PatchSpec{
		{
			Target: "deployment.my-app",
			Strategic: &StrategicPatch{
				Patch: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": int64(5),
					},
				},
			},
		},
	}

	set, err := NewPatchableAppSet(resources, patches)
	if err != nil {
		t.Fatalf("NewPatchableAppSet: %v", err)
	}

	resolved, err := set.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved resource, got %d", len(resolved))
	}

	// The resolved resource must be the Deployment, not the Service
	if resolved[0].Base.GetKind() != "Deployment" {
		t.Errorf("expected patched resource to be Deployment, got %s", resolved[0].Base.GetKind())
	}

	if err := resolved[0].Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	replicas, found, err := unstructured.NestedFieldNoCopy(resolved[0].Base.Object, "spec", "replicas")
	if err != nil || !found {
		t.Fatal("replicas not found after patch")
	}

	switch v := replicas.(type) {
	case float64:
		if v != 5 {
			t.Errorf("expected replicas 5, got %v", v)
		}
	case int64:
		if v != 5 {
			t.Errorf("expected replicas 5, got %v", v)
		}
	default:
		t.Errorf("unexpected replicas type %T", replicas)
	}
}

// TestWritePatchedFiles_PropagatesKindLookup verifies that WritePatchedFiles
// propagates KindLookup from the parent PatchableAppSet to the per-file sub-sets.
func TestWritePatchedFiles_PropagatesKindLookup(t *testing.T) {
	baseYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: "1"
  template:
    spec:
      containers:
      - name: main
        image: nginx:1.24
      - name: logger
        image: fluentd:latest
`
	docSet, err := LoadResourcesWithStructure(strings.NewReader(baseYAML))
	if err != nil {
		t.Fatalf("LoadResourcesWithStructure: %v", err)
	}

	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	// Build a strategic patch that merges containers by name (requires KindLookup)
	strategicPatch := StrategicPatch{
		Patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "main",
								"image": "nginx:1.25",
							},
						},
					},
				},
			},
		},
	}

	pas := &PatchableAppSet{
		Resources:   docSet.GetResources(),
		DocumentSet: docSet,
		KindLookup:  lookup,
		Patches: []struct {
			Target    string
			Patch     PatchOp
			Strategic *StrategicPatch
		}{
			{Target: "my-app", Strategic: &strategicPatch},
		},
	}

	// Write patched file using a patch file that exercises WritePatchedFiles path
	tmpDir := t.TempDir()
	patchContent := `- target: my-app
  type: strategic
  patch:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: nginx:1.25
`
	patchFile := tmpDir + "/patch.yaml"
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatalf("write patch file: %v", err)
	}

	baseFile := tmpDir + "/base.yaml"
	if err := os.WriteFile(baseFile, []byte(baseYAML), 0644); err != nil {
		t.Fatalf("write base file: %v", err)
	}

	outputDir := tmpDir + "/out"
	if err := pas.WritePatchedFiles(baseFile, []string{patchFile}, outputDir); err != nil {
		t.Fatalf("WritePatchedFiles: %v", err)
	}

	// Read the output and verify the logger container was preserved (SMP merge-by-name)
	outputFiles, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(outputFiles) == 0 {
		t.Fatal("expected at least one output file")
	}

	content, err := os.ReadFile(outputDir + "/" + outputFiles[0].Name())
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	out := string(content)

	// With KindLookup propagated, strategic merge should merge by name,
	// preserving the logger container. Without it, JSON merge would replace
	// the entire containers list, dropping logger.
	if !strings.Contains(out, "logger") {
		t.Errorf("expected logger container to be preserved (SMP merge-by-name), got:\n%s", out)
	}
	if !strings.Contains(out, "nginx:1.25") {
		t.Errorf("expected main container image to be updated to nginx:1.25, got:\n%s", out)
	}
}

// TestWritePatchedFiles_CrossKindSameName verifies that WritePatchedFiles
// correctly handles multiple resources sharing the same name across kinds
// (e.g. Deployment + Service both named "my-app"), applying strategic patches
// only to the targeted kind.
func TestWritePatchedFiles_CrossKindSameName(t *testing.T) {
	baseYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: "1"
  template:
    spec:
      containers:
      - name: main
        image: nginx:1.24
      - name: logger
        image: fluentd:latest
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: default
spec:
  type: ClusterIP
  ports:
  - name: http
    port: "80"
`
	docSet, err := LoadResourcesWithStructure(strings.NewReader(baseYAML))
	if err != nil {
		t.Fatalf("LoadResourcesWithStructure: %v", err)
	}

	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	pas := &PatchableAppSet{
		Resources:   docSet.GetResources(),
		DocumentSet: docSet,
		KindLookup:  lookup,
		Patches: make([]struct {
			Target    string
			Patch     PatchOp
			Strategic *StrategicPatch
		}, 0),
	}

	tmpDir := t.TempDir()

	// Create a strategic patch file targeting only the Deployment
	patchContent := `- target: deployment.my-app
  type: strategic
  patch:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: nginx:1.25
`
	patchFile := tmpDir + "/patch.yaml"
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatalf("write patch file: %v", err)
	}

	baseFile := tmpDir + "/base.yaml"
	if err := os.WriteFile(baseFile, []byte(baseYAML), 0644); err != nil {
		t.Fatalf("write base file: %v", err)
	}

	outputDir := tmpDir + "/out"
	if err := pas.WritePatchedFiles(baseFile, []string{patchFile}, outputDir); err != nil {
		t.Fatalf("WritePatchedFiles: %v", err)
	}

	// Read the output
	outputFiles, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(outputFiles) == 0 {
		t.Fatal("expected at least one output file")
	}

	content, err := os.ReadFile(outputDir + "/" + outputFiles[0].Name())
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	out := string(content)

	// The Deployment should have its image updated
	if !strings.Contains(out, "nginx:1.25") {
		t.Errorf("expected main container image to be updated to nginx:1.25, got:\n%s", out)
	}

	// The logger container should be preserved (SMP merge-by-name)
	if !strings.Contains(out, "logger") {
		t.Errorf("expected logger container to be preserved, got:\n%s", out)
	}

	// The Service should NOT have any container fields
	docs := strings.Split(out, "---")
	for _, doc := range docs {
		if strings.Contains(doc, "kind: Service") {
			if strings.Contains(doc, "containers") {
				t.Errorf("Service should not have containers field, but got:\n%s", doc)
			}
			if strings.Contains(doc, "replicas") {
				t.Errorf("Service should not have replicas field, but got:\n%s", doc)
			}
		}
	}
}

func TestNewPatchableAppSet(t *testing.T) {
	resYaml := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo2
data:
  foo: bar
`
	var rm map[string]interface{}
	if err := yaml.Unmarshal([]byte(resYaml), &rm); err != nil {
		t.Fatalf("yaml decode: %v", err)
	}
	patchYaml := "- target: demo2\n  patch:\n    data.foo: baz\n"
	patches, err := LoadPatchFile(strings.NewReader(patchYaml))
	if err != nil {
		t.Fatalf("load patch: %v", err)
	}
	base := &unstructured.Unstructured{Object: rm}
	set, err := NewPatchableAppSet([]*unstructured.Unstructured{base}, patches)
	if err != nil {
		t.Fatalf("NewPatchableAppSet: %v", err)
	}
	resolved, err := set.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved) != 1 || resolved[0].Name != "demo2" {
		t.Fatalf("unexpected resolve result")
	}
	if err := resolved[0].Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	val, _, _ := unstructured.NestedString(resolved[0].Base.Object, "data", "foo")
	if val != "baz" {
		t.Fatalf("patch not applied")
	}
}

// TestNewPatchableAppSet_FieldPatchKindQualifiedWithNameCollision verifies that
// a kind-qualified field-level target like "deployment.my-app" resolves and
// applies correctly even when another resource shares the same short name.
func TestNewPatchableAppSet_FieldPatchKindQualifiedWithNameCollision(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "my-app",
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
			},
		},
	}
	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "my-app",
			},
			"spec": map[string]interface{}{
				"type": "ClusterIP",
			},
		},
	}

	resources := []*unstructured.Unstructured{deployment, service}

	patchYaml := "- target: deployment.my-app\n  patch:\n    spec.replicas: 3\n"
	patches, err := LoadPatchFile(strings.NewReader(patchYaml))
	if err != nil {
		t.Fatalf("load patch: %v", err)
	}

	set, err := NewPatchableAppSet(resources, patches)
	if err != nil {
		t.Fatalf("NewPatchableAppSet: %v", err)
	}

	resolved, err := set.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved resource, got %d", len(resolved))
	}
	if resolved[0].Base.GetKind() != "Deployment" {
		t.Errorf("expected Deployment, got %s", resolved[0].Base.GetKind())
	}

	if err := resolved[0].Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	replicas, found, err := unstructured.NestedFieldNoCopy(resolved[0].Base.Object, "spec", "replicas")
	if err != nil || !found {
		t.Fatal("replicas not found after patch")
	}
	if fmt.Sprintf("%v", replicas) != "3" {
		t.Errorf("expected replicas 3, got %v", replicas)
	}
}
