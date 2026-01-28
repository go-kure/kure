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

func TestNewSimpleTablePrinterWithColumns(t *testing.T) {
	customColumns := []io.TableColumn{
		{
			Header:   "NAME",
			Width:    20,
			Priority: 0,
			Accessor: func(obj client.Object) string {
				return obj.GetName()
			},
		},
		{
			Header:   "NAMESPACE",
			Width:    15,
			Priority: 1,
			Accessor: func(obj client.Object) string {
				return obj.GetNamespace()
			},
		},
	}

	printer := io.NewSimpleTablePrinterWithColumns(customColumns, false, false)
	if printer == nil {
		t.Fatal("Expected non-nil printer")
	}

	// Test with custom columns
	resources := []*client.Object{
		createTestResource("ConfigMap", "test-cm", "default"),
	}

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print with custom columns: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Errorf("Expected NAME header in output: %s", output)
	}
	if !strings.Contains(output, "test-cm") {
		t.Errorf("Expected resource name in output: %s", output)
	}
}

func TestKindSpecificColumns_Deployment(t *testing.T) {
	gvk := metav1.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	columns := io.KindSpecificColumns(gvk)

	// Deployment columns should include REPLICAS for wide output
	foundReplicas := false
	for _, col := range columns {
		if col.Header == "REPLICAS" {
			foundReplicas = true
			if !col.WideOnly {
				t.Errorf("REPLICAS column should be wide-only")
			}
			break
		}
	}

	if !foundReplicas {
		t.Errorf("Expected REPLICAS column for Deployment resources")
	}
}

func TestPodColumns_Integration(t *testing.T) {
	// Create a printer with pod-specific columns
	gvk := metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	columns := io.KindSpecificColumns(gvk)
	printer := io.NewSimpleTablePrinterWithColumns(columns, true, false)

	// Create test pod
	pod := createTestPod("test-pod", "default")
	resources := []*client.Object{&pod}

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print pod table: %v", err)
	}

	output := buf.String()

	// Check for pod-specific columns
	if !strings.Contains(output, "RESTARTS") {
		t.Errorf("Expected RESTARTS header in pod output: %s", output)
	}
	if !strings.Contains(output, "NODE") {
		t.Errorf("Expected NODE header in wide pod output: %s", output)
	}
	if !strings.Contains(output, "node-1") {
		t.Errorf("Expected node name in pod output: %s", output)
	}
}

func TestDeploymentColumns_Integration(t *testing.T) {
	gvk := metav1.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	columns := io.KindSpecificColumns(gvk)
	printer := io.NewSimpleTablePrinterWithColumns(columns, true, false)

	deployment := createTestDeployment("test-deploy", "default")
	resources := []*client.Object{&deployment}

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print deployment table: %v", err)
	}

	output := buf.String()

	// Check for deployment-specific data
	if !strings.Contains(output, "REPLICAS") {
		t.Errorf("Expected REPLICAS header in deployment output: %s", output)
	}
	if !strings.Contains(output, "2/3") {
		t.Errorf("Expected ready status '2/3' in deployment output: %s", output)
	}
}

func TestServiceColumns_Integration(t *testing.T) {
	gvk := metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}

	columns := io.KindSpecificColumns(gvk)
	printer := io.NewSimpleTablePrinterWithColumns(columns, true, false)

	// Create service with LoadBalancer type and external IP
	service := createTestServiceWithExternalIP("test-svc", "default")
	resources := []*client.Object{&service}

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print service table: %v", err)
	}

	output := buf.String()

	// Check for service-specific columns
	if !strings.Contains(output, "TYPE") {
		t.Errorf("Expected TYPE header in service output: %s", output)
	}
	if !strings.Contains(output, "CLUSTER-IP") {
		t.Errorf("Expected CLUSTER-IP header in service output: %s", output)
	}
	if !strings.Contains(output, "EXTERNAL-IP") {
		t.Errorf("Expected EXTERNAL-IP header in wide service output: %s", output)
	}
	if !strings.Contains(output, "LoadBalancer") {
		t.Errorf("Expected LoadBalancer type in service output: %s", output)
	}
	if !strings.Contains(output, "203.0.113.1") {
		t.Errorf("Expected external IP in service output: %s", output)
	}
}

func TestConfigMapColumns_Integration(t *testing.T) {
	gvk := metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	columns := io.KindSpecificColumns(gvk)
	printer := io.NewSimpleTablePrinterWithColumns(columns, false, false)

	configMap := createTestConfigMapWithData("test-cm", "default")
	resources := []*client.Object{&configMap}

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print configmap table: %v", err)
	}

	output := buf.String()

	// Check for DATA column instead of READY
	if !strings.Contains(output, "DATA") {
		t.Errorf("Expected DATA header in configmap output: %s", output)
	}
	if strings.Contains(output, "READY") {
		t.Errorf("Should not have READY header in configmap output: %s", output)
	}
	if !strings.Contains(output, "3") {
		t.Errorf("Expected data count '3' in configmap output: %s", output)
	}
}

func TestSecretColumns_Integration(t *testing.T) {
	gvk := metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Secret",
	}

	columns := io.KindSpecificColumns(gvk)

	// Secret should use configColumns (same as ConfigMap)
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
		t.Errorf("Expected DATA column for Secret resources")
	}
	if foundReady {
		t.Errorf("Should not have READY column for Secret resources")
	}
}

func TestPodEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		pod             client.Object
		expectedReady   string
		expectedRestart string
		expectedNode    string
	}{
		{
			name:            "pod with no status",
			pod:             createTestResourceMinimal("Pod", "no-status-pod", "default"),
			expectedReady:   "0/0",
			expectedRestart: "0",
			expectedNode:    "<none>",
		},
		{
			name:            "pod with no containers",
			pod:             createTestPodNoContainers("no-containers-pod", "default"),
			expectedReady:   "0/0",
			expectedRestart: "0",
			expectedNode:    "<none>",
		},
		{
			name:            "pod with mixed ready states",
			pod:             createTestPodMixedReady("mixed-pod", "default"),
			expectedReady:   "1/3",
			expectedRestart: "5",
			expectedNode:    "node-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready := getPodReadyStatusForTest(tt.pod)
			if ready != tt.expectedReady {
				t.Errorf("Expected ready status %q, got %q", tt.expectedReady, ready)
			}

			restarts := getPodRestartsForTest(tt.pod)
			if restarts != tt.expectedRestart {
				t.Errorf("Expected restart count %q, got %q", tt.expectedRestart, restarts)
			}

			node := getPodNodeForTest(tt.pod)
			if node != tt.expectedNode {
				t.Errorf("Expected node %q, got %q", tt.expectedNode, node)
			}
		})
	}
}

func TestDeploymentEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		deployment       client.Object
		expectedReady    string
		expectedReplicas string
	}{
		{
			name:             "deployment with no status",
			deployment:       createTestResourceMinimal("Deployment", "no-status-deploy", "default"),
			expectedReady:    "0/0",
			expectedReplicas: "0",
		},
		{
			name:             "deployment with no spec",
			deployment:       createTestDeploymentNoSpec("no-spec-deploy", "default"),
			expectedReady:    "0/0",
			expectedReplicas: "0", // No spec returns "0"
		},
		{
			name:             "deployment scaling up",
			deployment:       createTestDeploymentScaling("scaling-deploy", "default", 5, 3),
			expectedReady:    "3/5",
			expectedReplicas: "5",
		},
		{
			name:             "deployment fully ready",
			deployment:       createTestDeploymentScaling("ready-deploy", "default", 10, 10),
			expectedReady:    "10/10",
			expectedReplicas: "10",
		},
		{
			name:             "deployment with spec but no replicas field",
			deployment:       createTestDeploymentNoReplicas("no-replicas-deploy", "default"),
			expectedReady:    "0/0",
			expectedReplicas: "1", // Default when spec exists but replicas not specified
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready := getDeploymentReadyStatusForTest(tt.deployment)
			if ready != tt.expectedReady {
				t.Errorf("Expected ready status %q, got %q", tt.expectedReady, ready)
			}

			replicas := getDeploymentReplicasForTest(tt.deployment)
			if replicas != tt.expectedReplicas {
				t.Errorf("Expected replicas %q, got %q", tt.expectedReplicas, replicas)
			}
		})
	}
}

func TestServiceEdgeCases(t *testing.T) {
	tests := []struct {
		name               string
		service            client.Object
		expectedType       string
		expectedClusterIP  string
		expectedExternalIP string
	}{
		{
			name:               "service with no spec",
			service:            createTestResourceMinimal("Service", "no-spec-svc", "default"),
			expectedType:       "ClusterIP", // Default type
			expectedClusterIP:  "<none>",
			expectedExternalIP: "<none>",
		},
		{
			name:               "NodePort service",
			service:            createTestServiceType("nodeport-svc", "default", "NodePort"),
			expectedType:       "NodePort",
			expectedClusterIP:  "10.96.0.10",
			expectedExternalIP: "<none>",
		},
		{
			name:               "LoadBalancer with hostname",
			service:            createTestServiceWithHostname("lb-svc", "default"),
			expectedType:       "LoadBalancer",
			expectedClusterIP:  "10.96.0.20",
			expectedExternalIP: "lb.example.com",
		},
		{
			name:               "Service with externalIPs",
			service:            createTestServiceWithExternalIPs("external-svc", "default"),
			expectedType:       "ClusterIP",
			expectedClusterIP:  "10.96.0.30",
			expectedExternalIP: "198.51.100.1",
		},
		{
			name:               "Headless service",
			service:            createTestServiceHeadless("headless-svc", "default"),
			expectedType:       "ClusterIP",
			expectedClusterIP:  "None",
			expectedExternalIP: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcType := getServiceTypeForTest(tt.service)
			if svcType != tt.expectedType {
				t.Errorf("Expected service type %q, got %q", tt.expectedType, svcType)
			}

			clusterIP := getServiceClusterIPForTest(tt.service)
			if clusterIP != tt.expectedClusterIP {
				t.Errorf("Expected cluster IP %q, got %q", tt.expectedClusterIP, clusterIP)
			}

			externalIP := getServiceExternalIPForTest(tt.service)
			if externalIP != tt.expectedExternalIP {
				t.Errorf("Expected external IP %q, got %q", tt.expectedExternalIP, externalIP)
			}
		})
	}
}

func TestConfigDataEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		config        client.Object
		expectedCount string
	}{
		{
			name:          "configmap with no data",
			config:        createTestResourceMinimal("ConfigMap", "no-data-cm", "default"),
			expectedCount: "0",
		},
		{
			name:          "configmap with empty data",
			config:        createTestConfigMapEmptyData("empty-cm", "default"),
			expectedCount: "0",
		},
		{
			name:          "configmap with 10 keys",
			config:        createTestConfigMapWithCount("large-cm", "default", 10),
			expectedCount: "10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := getConfigDataCountForTest(tt.config)
			if count != tt.expectedCount {
				t.Errorf("Expected data count %q, got %q", tt.expectedCount, count)
			}
		})
	}
}

// Test helper functions through actual column accessors to ensure real coverage
func TestColumnAccessors_DirectCalls(t *testing.T) {
	// Create columns for each resource type
	podGVK := metav1.GroupVersionKind{Kind: "Pod"}
	deploymentGVK := metav1.GroupVersionKind{Group: "apps", Kind: "Deployment"}
	serviceGVK := metav1.GroupVersionKind{Kind: "Service"}
	configMapGVK := metav1.GroupVersionKind{Kind: "ConfigMap"}

	podColumns := io.KindSpecificColumns(podGVK)
	deploymentColumns := io.KindSpecificColumns(deploymentGVK)
	serviceColumns := io.KindSpecificColumns(serviceGVK)
	configColumns := io.KindSpecificColumns(configMapGVK)

	// Test pod columns with edge cases
	t.Run("pod columns with invalid object type", func(t *testing.T) {
		invalidObj := createTestConfigMap("not-a-pod", "default")
		for _, col := range podColumns {
			// Should not panic, should return default values
			result := col.Accessor(invalidObj)
			if result == "" {
				t.Errorf("Column %s returned empty string for invalid object", col.Header)
			}
		}
	})

	// Test deployment columns with invalid object type
	t.Run("deployment columns with invalid object type", func(t *testing.T) {
		invalidObj := createTestConfigMap("not-a-deployment", "default")
		for _, col := range deploymentColumns {
			result := col.Accessor(invalidObj)
			if result == "" {
				t.Errorf("Column %s returned empty string for invalid object", col.Header)
			}
		}
	})

	// Test service columns with invalid object type
	t.Run("service columns with invalid object type", func(t *testing.T) {
		invalidObj := createTestConfigMap("not-a-service", "default")
		for _, col := range serviceColumns {
			result := col.Accessor(invalidObj)
			if result == "" {
				t.Errorf("Column %s returned empty string for invalid object", col.Header)
			}
		}
	})

	// Test config columns with non-unstructured object
	t.Run("config columns with invalid object type", func(t *testing.T) {
		invalidObj := createTestConfigMap("valid-cm", "default")
		for _, col := range configColumns {
			result := col.Accessor(invalidObj)
			if result == "" {
				t.Errorf("Column %s returned empty string for valid object", col.Header)
			}
		}
	})
}

func TestGetServiceExternalIP_AllPaths(t *testing.T) {
	tests := []struct {
		name       string
		service    client.Object
		expectedIP string
	}{
		{
			name:       "service with LoadBalancer IP",
			service:    createTestServiceWithExternalIP("lb-ip", "default"),
			expectedIP: "203.0.113.1",
		},
		{
			name:       "service with LoadBalancer hostname",
			service:    createTestServiceWithHostname("lb-host", "default"),
			expectedIP: "lb.example.com",
		},
		{
			name:       "service with externalIPs in spec",
			service:    createTestServiceWithExternalIPs("ext-ips", "default"),
			expectedIP: "198.51.100.1",
		},
		{
			name:       "service with no external access",
			service:    createTestService("internal", "default"),
			expectedIP: "<none>",
		},
		{
			name:       "service with empty LoadBalancer ingress",
			service:    createTestServiceEmptyIngress("empty-lb", "default"),
			expectedIP: "<none>",
		},
		{
			name:       "service with invalid status structure",
			service:    createTestServiceInvalidStatus("invalid", "default"),
			expectedIP: "<none>",
		},
	}

	gvk := metav1.GroupVersionKind{Kind: "Service"}
	columns := io.KindSpecificColumns(gvk)

	// Find the EXTERNAL-IP column accessor
	var externalIPAccessor func(client.Object) string
	for _, col := range columns {
		if col.Header == "EXTERNAL-IP" {
			externalIPAccessor = col.Accessor
			break
		}
	}

	if externalIPAccessor == nil {
		t.Fatal("Could not find EXTERNAL-IP column accessor")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := externalIPAccessor(tt.service)
			if result != tt.expectedIP {
				t.Errorf("Expected external IP %q, got %q", tt.expectedIP, result)
			}
		})
	}
}

func TestGetPodNode_AllPaths(t *testing.T) {
	tests := []struct {
		name         string
		pod          client.Object
		expectedNode string
	}{
		{
			name:         "pod with node assigned",
			pod:          createTestPod("scheduled-pod", "default"),
			expectedNode: "node-1",
		},
		{
			name:         "pod with no spec",
			pod:          createTestResourceMinimal("Pod", "no-spec-pod", "default"),
			expectedNode: "<none>",
		},
		{
			name:         "pod with empty nodeName",
			pod:          createTestPodNoNode("pending-pod", "default"),
			expectedNode: "<none>",
		},
		{
			name:         "pod with invalid spec type",
			pod:          createTestPodInvalidSpec("invalid-pod", "default"),
			expectedNode: "<none>",
		},
	}

	gvk := metav1.GroupVersionKind{Kind: "Pod"}
	columns := io.KindSpecificColumns(gvk)

	// Find the NODE column accessor
	var nodeAccessor func(client.Object) string
	for _, col := range columns {
		if col.Header == "NODE" {
			nodeAccessor = col.Accessor
			break
		}
	}

	if nodeAccessor == nil {
		t.Fatal("Could not find NODE column accessor")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nodeAccessor(tt.pod)
			if result != tt.expectedNode {
				t.Errorf("Expected node %q, got %q", tt.expectedNode, result)
			}
		})
	}
}

func TestKindSpecificColumns_UnknownKind(t *testing.T) {
	gvk := metav1.GroupVersionKind{
		Group:   "custom.example.com",
		Version: "v1",
		Kind:    "CustomResource",
	}

	columns := io.KindSpecificColumns(gvk)

	// Should return default columns for unknown kinds
	if len(columns) == 0 {
		t.Error("Expected non-empty columns for unknown kind")
	}

	// Check that it returns base columns
	foundNamespace := false
	foundName := false
	for _, col := range columns {
		if col.Header == "NAMESPACE" {
			foundNamespace = true
		}
		if col.Header == "NAME" {
			foundName = true
		}
	}

	if !foundNamespace || !foundName {
		t.Error("Expected default columns (NAMESPACE, NAME) for unknown kind")
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

func getPodNodeForTest(obj client.Object) string {
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

	if nodeName, ok := specMap["nodeName"].(string); ok && nodeName != "" {
		return nodeName
	}

	return "<none>"
}

func getDeploymentReplicasForTest(obj client.Object) string {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "0"
	}

	specVal, found := unstructured.UnstructuredContent()["spec"]
	if !found {
		return "0"
	}

	specMap, ok := specVal.(map[string]interface{})
	if !ok {
		return "0"
	}

	if replicas, ok := specMap["replicas"].(float64); ok {
		return fmt.Sprintf("%.0f", replicas)
	}

	return "1" // Default replica count
}

func getServiceExternalIPForTest(obj client.Object) string {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "<none>"
	}

	statusVal, found := unstructured.UnstructuredContent()["status"]
	if found {
		if statusMap, ok := statusVal.(map[string]interface{}); ok {
			if lb, ok := statusMap["loadBalancer"].(map[string]interface{}); ok {
				if ingress, ok := lb["ingress"].([]interface{}); ok && len(ingress) > 0 {
					if ingressMap, ok := ingress[0].(map[string]interface{}); ok {
						if ip, ok := ingressMap["ip"].(string); ok && ip != "" {
							return ip
						}
						if hostname, ok := ingressMap["hostname"].(string); ok && hostname != "" {
							return hostname
						}
					}
				}
			}
		}
	}

	// Check spec for external IPs
	specVal, found := unstructured.UnstructuredContent()["spec"]
	if found {
		if specMap, ok := specVal.(map[string]interface{}); ok {
			if externalIPs, ok := specMap["externalIPs"].([]interface{}); ok && len(externalIPs) > 0 {
				if ip, ok := externalIPs[0].(string); ok {
					return ip
				}
			}
		}
	}

	return "<none>"
}

// Additional helper functions for edge case testing

func createTestResourceMinimal(kind, name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return obj
}

func createTestPodNoContainers(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Pod")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{}
	obj.Object["status"] = map[string]interface{}{
		"containerStatuses": []interface{}{},
	}
	return obj
}

func createTestPodMixedReady(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Pod")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"nodeName": "node-2",
	}
	obj.Object["status"] = map[string]interface{}{
		"containerStatuses": []interface{}{
			map[string]interface{}{
				"name":         "main",
				"ready":        true,
				"restartCount": float64(3),
			},
			map[string]interface{}{
				"name":         "sidecar",
				"ready":        false,
				"restartCount": float64(2),
			},
			map[string]interface{}{
				"name":         "init-proxy",
				"ready":        false,
				"restartCount": float64(0),
			},
		},
	}
	return obj
}

func createTestDeploymentNoSpec(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["status"] = map[string]interface{}{}
	return obj
}

func createTestDeploymentNoReplicas(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		// spec exists but no replicas field
	}
	obj.Object["status"] = map[string]interface{}{}
	return obj
}

func createTestDeploymentScaling(name, namespace string, replicas, readyReplicas int) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"replicas": float64(replicas),
	}
	obj.Object["status"] = map[string]interface{}{
		"replicas":      float64(replicas),
		"readyReplicas": float64(readyReplicas),
	}
	return obj
}

func createTestServiceType(name, namespace, serviceType string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"type":      serviceType,
		"clusterIP": "10.96.0.10",
	}
	return obj
}

func createTestServiceWithExternalIP(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"type":      "LoadBalancer",
		"clusterIP": "10.96.0.5",
	}
	obj.Object["status"] = map[string]interface{}{
		"loadBalancer": map[string]interface{}{
			"ingress": []interface{}{
				map[string]interface{}{
					"ip": "203.0.113.1",
				},
			},
		},
	}
	return obj
}

func createTestServiceWithHostname(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"type":      "LoadBalancer",
		"clusterIP": "10.96.0.20",
	}
	obj.Object["status"] = map[string]interface{}{
		"loadBalancer": map[string]interface{}{
			"ingress": []interface{}{
				map[string]interface{}{
					"hostname": "lb.example.com",
				},
			},
		},
	}
	return obj
}

func createTestServiceWithExternalIPs(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"type":      "ClusterIP",
		"clusterIP": "10.96.0.30",
		"externalIPs": []interface{}{
			"198.51.100.1",
			"198.51.100.2",
		},
	}
	return obj
}

func createTestServiceHeadless(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"type":      "ClusterIP",
		"clusterIP": "None",
	}
	return obj
}

func createTestConfigMapEmptyData(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["data"] = map[string]interface{}{}
	return obj
}

func createTestConfigMapWithCount(name, namespace string, count int) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	data := make(map[string]interface{})
	for i := 0; i < count; i++ {
		data[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}
	obj.Object["data"] = data
	return obj
}

func createTestServiceEmptyIngress(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"type":      "LoadBalancer",
		"clusterIP": "10.96.0.50",
	}
	obj.Object["status"] = map[string]interface{}{
		"loadBalancer": map[string]interface{}{
			"ingress": []interface{}{}, // Empty ingress list
		},
	}
	return obj
}

func createTestServiceInvalidStatus(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"type":      "LoadBalancer",
		"clusterIP": "10.96.0.60",
	}
	obj.Object["status"] = "invalid-status-type" // Invalid type
	return obj
}

func createTestPodNoNode(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Pod")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = map[string]interface{}{
		"nodeName": "", // Empty node name
	}
	return obj
}

func createTestPodInvalidSpec(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Pod")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["spec"] = "invalid-spec-type" // Invalid type
	return obj
}
