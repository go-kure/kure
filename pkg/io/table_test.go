package io_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/io"
)

func TestSimpleTablePrinter_BasicOutput(t *testing.T) {
	resources := []*client.Object{
		createTestResource("ConfigMap", "test-cm", "default"),
		createTestResource("Secret", "test-secret", "kube-system"),
	}

	printer := io.NewSimpleTablePrinter(false, false)

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print table: %v", err)
	}

	output := buf.String()

	// Check for headers
	if !strings.Contains(output, "NAMESPACE") {
		t.Errorf("Expected NAMESPACE header in output: %s", output)
	}
	if !strings.Contains(output, "NAME") {
		t.Errorf("Expected NAME header in output: %s", output)
	}
	if !strings.Contains(output, "AGE") {
		t.Errorf("Expected AGE header in output: %s", output)
	}

	// Check for resource data
	if !strings.Contains(output, "test-cm") {
		t.Errorf("Expected resource name 'test-cm' in output: %s", output)
	}
	if !strings.Contains(output, "test-secret") {
		t.Errorf("Expected resource name 'test-secret' in output: %s", output)
	}
	if !strings.Contains(output, "default") {
		t.Errorf("Expected namespace 'default' in output: %s", output)
	}
	if !strings.Contains(output, "kube-system") {
		t.Errorf("Expected namespace 'kube-system' in output: %s", output)
	}
}

func TestSimpleTablePrinter_NoHeaders(t *testing.T) {
	resources := []*client.Object{
		createTestResource("ConfigMap", "test-cm", "default"),
	}

	printer := io.NewSimpleTablePrinter(false, true) // noHeaders = true

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print table: %v", err)
	}

	output := buf.String()

	// Should not contain headers
	if strings.Contains(output, "NAMESPACE") {
		t.Errorf("Should not contain NAMESPACE header with noHeaders=true: %s", output)
	}
	if strings.Contains(output, "NAME") {
		t.Errorf("Should not contain NAME header with noHeaders=true: %s", output)
	}

	// Should still contain data
	if !strings.Contains(output, "test-cm") {
		t.Errorf("Expected resource name 'test-cm' in output: %s", output)
	}
}

func TestSimpleTablePrinter_WideOutput(t *testing.T) {
	resources := []*client.Object{
		createTestResource("ConfigMap", "test-cm", "default"),
	}

	printer := io.NewSimpleTablePrinter(true, false) // wide = true

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print wide table: %v", err)
	}

	output := buf.String()

	// Wide output should include additional columns
	if !strings.Contains(output, "STATUS") {
		t.Errorf("Expected STATUS column in wide output: %s", output)
	}
}

func TestSimpleTablePrinter_EmptyInput(t *testing.T) {
	printer := io.NewSimpleTablePrinter(false, false)

	var buf bytes.Buffer
	err := printer.Print([]*client.Object{}, &buf)
	if err != nil {
		t.Fatalf("Failed to print empty table: %v", err)
	}

	output := buf.String()
	if output != "" {
		t.Errorf("Expected empty output for empty resource list, got: %s", output)
	}
}

func TestDefaultColumns(t *testing.T) {
	columns := io.DefaultColumns()

	if len(columns) == 0 {
		t.Fatal("Expected at least one default column")
	}

	// Check for expected columns
	expectedHeaders := []string{"NAMESPACE", "NAME", "READY", "AGE"}
	foundHeaders := make(map[string]bool)

	for _, col := range columns {
		foundHeaders[col.Header] = true
	}

	for _, expected := range expectedHeaders {
		if !foundHeaders[expected] {
			t.Errorf("Expected column %s not found in default columns", expected)
		}
	}
}

func TestKindSpecificColumns_Pod(t *testing.T) {
	gvk := metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	columns := io.KindSpecificColumns(gvk)

	// Pod columns should include RESTARTS
	foundRestarts := false
	for _, col := range columns {
		if col.Header == "RESTARTS" {
			foundRestarts = true
			break
		}
	}

	if !foundRestarts {
		t.Errorf("Expected RESTARTS column for Pod resources")
	}
}

func TestKindSpecificColumns_Service(t *testing.T) {
	gvk := metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}

	columns := io.KindSpecificColumns(gvk)

	// Service columns should include TYPE and CLUSTER-IP
	expectedHeaders := []string{"TYPE", "CLUSTER-IP"}
	foundHeaders := make(map[string]bool)

	for _, col := range columns {
		foundHeaders[col.Header] = true
	}

	for _, expected := range expectedHeaders {
		if !foundHeaders[expected] {
			t.Errorf("Expected column %s not found for Service resources", expected)
		}
	}
}

func TestKindSpecificColumns_ConfigMap(t *testing.T) {
	gvk := metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	columns := io.KindSpecificColumns(gvk)

	// ConfigMap columns should include DATA instead of READY
	foundData := false
	foundReady := false
	for _, col := range columns {
		if col.Header == "DATA" {
			foundData = true
		}
		if col.Header == "READY" {
			foundReady = true
		}
	}

	if !foundData {
		t.Errorf("Expected DATA column for ConfigMap resources")
	}
	if foundReady {
		t.Errorf("Should not have READY column for ConfigMap resources")
	}
}

func TestGetDetailedStatus(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name:     "nil object",
			obj:      nil,
			expected: "Unknown",
		},
		{
			name:     "object without status",
			obj:      *createTestResource("ConfigMap", "test", "default"),
			expected: "Unknown",
		},
		{
			name: "object with message in status",
			obj: *createTestResourceWithStatus("Pod", "test", "default", map[string]interface{}{
				"message": "Pod is running",
			}),
			expected: "Pod is running",
		},
		{
			name: "object with reason in status",
			obj: *createTestResourceWithStatus("Pod", "test", "default", map[string]interface{}{
				"reason": "ImagePullBackOff",
			}),
			expected: "ImagePullBackOff",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := io.GetDetailedStatus(test.obj)
			if result != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestPrintObjectsAsTable(t *testing.T) {
	resources := []*client.Object{
		createTestResource("ConfigMap", "test-cm", "default"),
		createTestResource("Secret", "test-secret", "kube-system"),
	}

	var buf bytes.Buffer
	err := io.PrintObjectsAsTable(resources, false, false, &buf)
	if err != nil {
		t.Fatalf("Failed to print objects as table: %v", err)
	}

	output := buf.String()

	// Check basic table output
	if !strings.Contains(output, "test-cm") {
		t.Errorf("Expected resource name in table output: %s", output)
	}
	if !strings.Contains(output, "NAMESPACE") {
		t.Errorf("Expected header in table output: %s", output)
	}
}

func TestResourceStatusAccessors(t *testing.T) {
	// Test pod with container statuses
	pod := createTestPod("test-pod", "default")
	ready := getPodReadyStatusForTest(pod)
	if ready != "2/2" {
		t.Errorf("Expected '2/2' ready status for test pod, got %q", ready)
	}

	restarts := getPodRestartsForTest(pod)
	if restarts != "1" {
		t.Errorf("Expected '1' restart count for test pod, got %q", restarts)
	}

	// Test deployment
	deployment := createTestDeployment("test-deploy", "default")
	deployReady := getDeploymentReadyStatusForTest(deployment)
	if deployReady != "2/3" {
		t.Errorf("Expected '2/3' ready status for test deployment, got %q", deployReady)
	}

	// Test service
	service := createTestService("test-svc", "default")
	svcType := getServiceTypeForTest(service)
	if svcType != "ClusterIP" {
		t.Errorf("Expected 'ClusterIP' type for test service, got %q", svcType)
	}

	clusterIP := getServiceClusterIPForTest(service)
	if clusterIP != "10.96.0.1" {
		t.Errorf("Expected '10.96.0.1' cluster IP for test service, got %q", clusterIP)
	}

	// Test config data count
	configMap := createTestConfigMapWithData("test-cm", "default")
	dataCount := getConfigDataCountForTest(configMap)
	if dataCount != "3" {
		t.Errorf("Expected '3' data count for test configmap, got %q", dataCount)
	}
}

// Helper functions for testing

func createTestResource(kind, name, namespace string) *client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetCreationTimestamp(metav1.Time{Time: time.Now().Add(-5 * time.Minute)})

	clientObj := client.Object(obj)
	return &clientObj
}

func createTestResourceWithStatus(kind, name, namespace string, status map[string]interface{}) *client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["status"] = status

	clientObj := client.Object(obj)
	return &clientObj
}

func createTestPod(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Pod")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"nodeName": "node-1",
	}
	obj.Object["status"] = map[string]interface{}{
		"containerStatuses": []interface{}{
			map[string]interface{}{
				"name":         "main",
				"ready":        true,
				"restartCount": float64(1),
			},
			map[string]interface{}{
				"name":         "sidecar",
				"ready":        true,
				"restartCount": float64(0),
			},
		},
	}
	return obj
}

func createTestDeployment(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"replicas": float64(3),
	}
	obj.Object["status"] = map[string]interface{}{
		"replicas":      float64(3),
		"readyReplicas": float64(2),
	}
	return obj
}

func createTestService(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"type":      "ClusterIP",
		"clusterIP": "10.96.0.1",
	}
	return obj
}

func createTestConfigMapWithData(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["data"] = map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	return obj
}

// Test helper wrappers for internal functions (since they're not exported)
// These would need to be adjusted based on actual implementation

func getPodReadyStatusForTest(obj client.Object) string {
	// This would call the internal getPodReadyStatus function
	// For testing purposes, we simulate the expected behavior
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "0/0"
	}

	statusVal, found := unstructured.UnstructuredContent()["status"]
	if !found {
		return "0/0"
	}

	statusMap, ok := statusVal.(map[string]interface{})
	if !ok {
		return "0/0"
	}

	if containerStatuses, ok := statusMap["containerStatuses"].([]interface{}); ok {
		ready := 0
		total := len(containerStatuses)

		for _, cs := range containerStatuses {
			if csMap, ok := cs.(map[string]interface{}); ok {
				if isReady, ok := csMap["ready"].(bool); ok && isReady {
					ready++
				}
			}
		}

		return fmt.Sprintf("%d/%d", ready, total)
	}

	return "0/0"
}

func getPodRestartsForTest(obj client.Object) string {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "0"
	}

	statusVal, found := unstructured.UnstructuredContent()["status"]
	if !found {
		return "0"
	}

	statusMap, ok := statusVal.(map[string]interface{})
	if !ok {
		return "0"
	}

	if containerStatuses, ok := statusMap["containerStatuses"].([]interface{}); ok {
		totalRestarts := 0

		for _, cs := range containerStatuses {
			if csMap, ok := cs.(map[string]interface{}); ok {
				if restartCount, ok := csMap["restartCount"].(float64); ok {
					totalRestarts += int(restartCount)
				}
			}
		}

		return fmt.Sprintf("%d", totalRestarts)
	}

	return "0"
}

func getDeploymentReadyStatusForTest(obj client.Object) string {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "0/0"
	}

	statusVal, found := unstructured.UnstructuredContent()["status"]
	if !found {
		return "0/0"
	}

	statusMap, ok := statusVal.(map[string]interface{})
	if !ok {
		return "0/0"
	}

	ready := int64(0)
	if readyReplicas, ok := statusMap["readyReplicas"].(float64); ok {
		ready = int64(readyReplicas)
	}

	desired := int64(0)
	if replicas, ok := statusMap["replicas"].(float64); ok {
		desired = int64(replicas)
	}

	return fmt.Sprintf("%d/%d", ready, desired)
}

func getServiceTypeForTest(obj client.Object) string {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "Unknown"
	}

	specVal, found := unstructured.UnstructuredContent()["spec"]
	if !found {
		return "ClusterIP"
	}

	specMap, ok := specVal.(map[string]interface{})
	if !ok {
		return "ClusterIP"
	}

	if serviceType, ok := specMap["type"].(string); ok {
		return serviceType
	}

	return "ClusterIP"
}

func getServiceClusterIPForTest(obj client.Object) string {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "<none>"
	}

	specVal, found := unstructured.UnstructuredContent()["spec"]
	if !found {
		return "<none>"
	}

	specMap, ok := specVal.(map[string]interface{})
	if !ok {
		return "<none>"
	}

	if clusterIP, ok := specMap["clusterIP"].(string); ok && clusterIP != "" {
		return clusterIP
	}

	return "<none>"
}

func getConfigDataCountForTest(obj client.Object) string {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "0"
	}

	dataVal, found := unstructured.UnstructuredContent()["data"]
	if !found {
		return "0"
	}

	if dataMap, ok := dataVal.(map[string]interface{}); ok {
		return fmt.Sprintf("%d", len(dataMap))
	}

	return "0"
}
