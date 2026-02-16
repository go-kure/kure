package io

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type demo struct {
	Name string `yaml:"name"`
	Age  int    `yaml:"age"`
}

func TestBufferMarshalUnmarshal(t *testing.T) {
	b := &Buffer{}
	in := demo{Name: "test", Age: 5}
	if err := b.Marshal(in); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out demo
	if err := b.Unmarshal(&out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip mismatch: %#v != %#v", in, out)
	}
}

func TestEncodeObjectsToYAMLWithOptions(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName("test-deploy")
	obj.SetNamespace("default")
	obj.Object["spec"] = map[string]interface{}{
		"replicas": int64(3),
		"selector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"app": "test",
			},
		},
	}

	co := client.Object(obj)
	objects := []*client.Object{&co}

	opts := EncodeOptions{KubernetesFieldOrder: true}
	out, err := EncodeObjectsToYAMLWithOptions(objects, opts)
	if err != nil {
		t.Fatalf("EncodeObjectsToYAMLWithOptions: %v", err)
	}

	s := string(out)

	// Verify Kubernetes-conventional field order
	apiVersionIdx := strings.Index(s, "apiVersion:")
	kindIdx := strings.Index(s, "kind:")
	metadataIdx := strings.Index(s, "metadata:")
	specIdx := strings.Index(s, "spec:")

	if apiVersionIdx >= kindIdx {
		t.Errorf("apiVersion should come before kind:\n%s", s)
	}
	if kindIdx >= metadataIdx {
		t.Errorf("kind should come before metadata:\n%s", s)
	}
	if metadataIdx >= specIdx {
		t.Errorf("metadata should come before spec:\n%s", s)
	}

	// Verify the content is valid YAML with expected values
	if !strings.Contains(s, "apiVersion: apps/v1") {
		t.Errorf("expected apiVersion: apps/v1 in output:\n%s", s)
	}
	if !strings.Contains(s, "kind: Deployment") {
		t.Errorf("expected kind: Deployment in output:\n%s", s)
	}
}

func TestEncodeObjectsToYAML_BackwardCompatible(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-cm")
	obj.SetNamespace("default")
	obj.SetUID("abc-123")
	obj.SetResourceVersion("999")
	obj.SetGeneration(5)
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
	})
	obj.SetAnnotations(map[string]string{
		"kubectl.kubernetes.io/last-applied-configuration": `{"some":"config"}`,
	})
	obj.Object["data"] = map[string]interface{}{
		"key": "value",
	}

	co := client.Object(obj)
	objects := []*client.Object{&co}

	out, err := EncodeObjectsToYAML(objects)
	if err != nil {
		t.Fatalf("EncodeObjectsToYAML: %v", err)
	}

	s := string(out)

	// Verify the existing function still produces valid YAML
	if !strings.Contains(s, "apiVersion: v1") {
		t.Errorf("expected apiVersion: v1 in output:\n%s", s)
	}
	if !strings.Contains(s, "kind: ConfigMap") {
		t.Errorf("expected kind: ConfigMap in output:\n%s", s)
	}
	if !strings.Contains(s, "key: value") {
		t.Errorf("expected data key in output:\n%s", s)
	}

	// Should NOT have creationTimestamp (it's null and should be stripped)
	if strings.Contains(s, "creationTimestamp") {
		t.Errorf("expected creationTimestamp to be stripped:\n%s", s)
	}

	// Zero-value EncodeOptions means StripServerFieldsFull, so all server
	// fields should be stripped by default.
	for _, field := range []string{"managedFields", "resourceVersion", "uid", "generation", "last-applied-configuration"} {
		if strings.Contains(s, field) {
			t.Errorf("expected %s to be stripped by default:\n%s", field, s)
		}
	}
}

func TestSaveLoadFile(t *testing.T) {
	d := demo{Name: "file", Age: 8}
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.yaml")
	if err := SaveFile(path, d); err != nil {
		t.Fatalf("save: %v", err)
	}
	var out demo
	if err := LoadFile(path, &out); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(d, out) {
		t.Fatalf("file round trip mismatch: %#v != %#v", d, out)
	}
}

// newServerFieldObj creates an Unstructured object with all server-set fields populated.
func newServerFieldObj() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-cm")
	obj.SetNamespace("default")
	obj.SetUID("abc-123")
	obj.SetResourceVersion("999")
	obj.SetGeneration(5)
	obj.SetSelfLink("/api/v1/namespaces/default/configmaps/test-cm")
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
	})
	obj.SetAnnotations(map[string]string{
		"kubectl.kubernetes.io/last-applied-configuration": `{"some":"config"}`,
		"app.kubernetes.io/name":                           "myapp",
	})
	obj.Object["data"] = map[string]interface{}{
		"key": "value",
	}
	return obj
}

func TestCleanResourceMap_FullStripping(t *testing.T) {
	obj := newServerFieldObj()
	co := client.Object(obj)
	objects := []*client.Object{&co}

	out, err := EncodeObjectsToYAMLWithOptions(objects, EncodeOptions{
		ServerFieldStripping: StripServerFieldsFull,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s := string(out)

	// All server-set fields should be stripped
	for _, field := range []string{
		"managedFields", "resourceVersion", "uid", "generation",
		"selfLink", "last-applied-configuration",
	} {
		if strings.Contains(s, field) {
			t.Errorf("expected %s to be stripped:\n%s", field, s)
		}
	}

	// User annotation must survive
	if !strings.Contains(s, "app.kubernetes.io/name") {
		t.Errorf("expected user annotation to be preserved:\n%s", s)
	}

	// Null creationTimestamp should be stripped
	if strings.Contains(s, "creationTimestamp") {
		t.Errorf("expected creationTimestamp to be stripped:\n%s", s)
	}
}

func TestCleanResourceMap_NestedMetadata(t *testing.T) {
	// Deployment with pod template metadata containing server fields
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName("test-deploy")
	obj.SetNamespace("default")
	obj.SetUID("deploy-uid")
	obj.SetResourceVersion("42")
	obj.Object["spec"] = map[string]interface{}{
		"replicas": int64(1),
		"template": map[string]interface{}{
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"uid":               "pod-uid",
				"resourceVersion":   "11",
				"labels": map[string]interface{}{
					"app": "test",
				},
			},
		},
	}

	co := client.Object(obj)
	objects := []*client.Object{&co}

	out, err := EncodeObjectsToYAMLWithOptions(objects, EncodeOptions{})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s := string(out)

	// Server fields at both levels should be stripped
	if strings.Contains(s, "uid") {
		t.Errorf("expected uid to be stripped from all levels:\n%s", s)
	}
	if strings.Contains(s, "resourceVersion") {
		t.Errorf("expected resourceVersion to be stripped from all levels:\n%s", s)
	}
	if strings.Contains(s, "creationTimestamp") {
		t.Errorf("expected creationTimestamp to be stripped:\n%s", s)
	}

	// User labels must survive in nested template
	if !strings.Contains(s, "app: test") {
		t.Errorf("expected nested labels to be preserved:\n%s", s)
	}
}

func TestCleanResourceMap_CronJobNested(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("batch/v1")
	obj.SetKind("CronJob")
	obj.SetName("test-cj")
	obj.SetNamespace("default")
	obj.Object["spec"] = map[string]interface{}{
		"schedule": "*/5 * * * *",
		"jobTemplate": map[string]interface{}{
			"metadata": map[string]interface{}{
				"uid":               "job-uid",
				"creationTimestamp": nil,
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"uid":               "pod-uid",
						"resourceVersion":   "33",
						"creationTimestamp": nil,
						"labels": map[string]interface{}{
							"job": "cron",
						},
					},
				},
			},
		},
	}

	co := client.Object(obj)
	objects := []*client.Object{&co}

	out, err := EncodeObjectsToYAMLWithOptions(objects, EncodeOptions{})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s := string(out)

	if strings.Contains(s, "uid") {
		t.Errorf("expected uid stripped from CronJob nested metadata:\n%s", s)
	}
	if strings.Contains(s, "resourceVersion") {
		t.Errorf("expected resourceVersion stripped from CronJob nested metadata:\n%s", s)
	}
	if strings.Contains(s, "creationTimestamp") {
		t.Errorf("expected creationTimestamp stripped from CronJob nested metadata:\n%s", s)
	}
	if !strings.Contains(s, "job: cron") {
		t.Errorf("expected nested labels preserved in CronJob:\n%s", s)
	}
}

func TestCleanResourceMap_BasicLevel(t *testing.T) {
	obj := newServerFieldObj()
	co := client.Object(obj)
	objects := []*client.Object{&co}

	out, err := EncodeObjectsToYAMLWithOptions(objects, EncodeOptions{
		ServerFieldStripping: StripServerFieldsBasic,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s := string(out)

	// Basic level should only strip null creationTimestamp and empty status.
	// Server-set fields should be preserved.
	if strings.Contains(s, "creationTimestamp") {
		t.Errorf("expected creationTimestamp stripped at Basic level:\n%s", s)
	}
	for _, field := range []string{"managedFields", "resourceVersion", "uid", "generation"} {
		if !strings.Contains(s, field) {
			t.Errorf("expected %s to be preserved at Basic level:\n%s", field, s)
		}
	}
}

func TestCleanResourceMap_NoneLevel(t *testing.T) {
	obj := newServerFieldObj()
	// Set a non-null creationTimestamp so it appears in output
	obj.Object["metadata"].(map[string]interface{})["creationTimestamp"] = "2024-01-01T00:00:00Z"
	obj.Object["status"] = map[string]interface{}{}

	co := client.Object(obj)
	objects := []*client.Object{&co}

	out, err := EncodeObjectsToYAMLWithOptions(objects, EncodeOptions{
		ServerFieldStripping: StripServerFieldsNone,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s := string(out)

	// Nothing should be stripped
	for _, field := range []string{
		"managedFields", "resourceVersion", "uid", "generation",
		"creationTimestamp", "status",
	} {
		if !strings.Contains(s, field) {
			t.Errorf("expected %s to be preserved at None level:\n%s", field, s)
		}
	}
}

func TestCleanResourceMap_AnnotationPreservation(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	obj.SetName("test-svc")
	obj.SetNamespace("default")
	obj.SetAnnotations(map[string]string{
		"kubectl.kubernetes.io/last-applied-configuration": `{"some":"config"}`,
		"prometheus.io/scrape":                             "true",
		"prometheus.io/port":                               "9090",
	})

	co := client.Object(obj)
	objects := []*client.Object{&co}

	out, err := EncodeObjectsToYAMLWithOptions(objects, EncodeOptions{})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s := string(out)

	// kubectl annotation stripped
	if strings.Contains(s, "last-applied-configuration") {
		t.Errorf("expected kubectl annotation to be stripped:\n%s", s)
	}

	// User annotations preserved
	if !strings.Contains(s, "prometheus.io/scrape") {
		t.Errorf("expected prometheus.io/scrape to be preserved:\n%s", s)
	}
	if !strings.Contains(s, "prometheus.io/port") {
		t.Errorf("expected prometheus.io/port to be preserved:\n%s", s)
	}

	// Annotations map should still exist (not deleted because it has remaining entries)
	if !strings.Contains(s, "annotations") {
		t.Errorf("expected annotations key to remain:\n%s", s)
	}
}

func TestCleanResourceMap_EmptyAnnotationsRemoved(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-cm")
	obj.SetNamespace("default")
	// Only the kubectl annotation — should be removed along with the annotations map
	obj.SetAnnotations(map[string]string{
		"kubectl.kubernetes.io/last-applied-configuration": `{"some":"config"}`,
	})

	co := client.Object(obj)
	objects := []*client.Object{&co}

	out, err := EncodeObjectsToYAMLWithOptions(objects, EncodeOptions{})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s := string(out)

	if strings.Contains(s, "annotations") {
		t.Errorf("expected empty annotations map to be removed:\n%s", s)
	}
}
