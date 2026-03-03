package io

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ---------- getConfigDataCount ----------

func TestGetConfigDataCount(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name: "config map with data",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("ConfigMap")
				obj.SetName("cm")
				obj.Object["data"] = map[string]interface{}{
					"k1": "v1",
					"k2": "v2",
				}
				return obj
			}(),
			expected: "2",
		},
		{
			name: "config map without data",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("ConfigMap")
				obj.SetName("cm")
				return obj
			}(),
			expected: "0",
		},
		{
			name: "config map with non-map data",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("ConfigMap")
				obj.SetName("cm")
				obj.Object["data"] = "not-a-map"
				return obj
			}(),
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getConfigDataCount(tt.obj)
			if got != tt.expected {
				t.Errorf("getConfigDataCount() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- getDeploymentReplicas ----------

func TestGetDeploymentReplicas(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name: "deployment with replicas",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.SetName("deploy")
				obj.Object["spec"] = map[string]interface{}{
					"replicas": float64(5),
				}
				return obj
			}(),
			expected: "5",
		},
		{
			name: "deployment without spec",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.SetName("deploy")
				return obj
			}(),
			expected: "0",
		},
		{
			name: "deployment with spec but no replicas",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.SetName("deploy")
				obj.Object["spec"] = map[string]interface{}{
					"selector": map[string]interface{}{},
				}
				return obj
			}(),
			expected: "1", // default
		},
		{
			name: "deployment with non-map spec",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.SetName("deploy")
				obj.Object["spec"] = "not-a-map"
				return obj
			}(),
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDeploymentReplicas(tt.obj)
			if got != tt.expected {
				t.Errorf("getDeploymentReplicas() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- getServiceType ----------

func TestGetServiceType(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name: "service with type",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["spec"] = map[string]interface{}{
					"type": "LoadBalancer",
				}
				return obj
			}(),
			expected: "LoadBalancer",
		},
		{
			name: "service without spec",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				return obj
			}(),
			expected: "ClusterIP", // default
		},
		{
			name: "service with spec but no type",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["spec"] = map[string]interface{}{
					"clusterIP": "10.0.0.1",
				}
				return obj
			}(),
			expected: "ClusterIP", // default
		},
		{
			name: "service with non-map spec",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["spec"] = "not-a-map"
				return obj
			}(),
			expected: "ClusterIP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getServiceType(tt.obj)
			if got != tt.expected {
				t.Errorf("getServiceType() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- getServiceClusterIP ----------

func TestGetServiceClusterIP(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name: "service with clusterIP",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["spec"] = map[string]interface{}{
					"clusterIP": "10.96.0.1",
				}
				return obj
			}(),
			expected: "10.96.0.1",
		},
		{
			name: "service without spec",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				return obj
			}(),
			expected: "<none>",
		},
		{
			name: "service with empty clusterIP",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["spec"] = map[string]interface{}{
					"clusterIP": "",
				}
				return obj
			}(),
			expected: "<none>",
		},
		{
			name: "service with non-map spec",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["spec"] = "not-a-map"
				return obj
			}(),
			expected: "<none>",
		},
		{
			name: "service with spec but no clusterIP",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["spec"] = map[string]interface{}{
					"type": "ClusterIP",
				}
				return obj
			}(),
			expected: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getServiceClusterIP(tt.obj)
			if got != tt.expected {
				t.Errorf("getServiceClusterIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- getDeploymentReadyStatus ----------

func TestGetDeploymentReadyStatus(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name: "deployment with ready replicas",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.SetName("deploy")
				obj.Object["status"] = map[string]interface{}{
					"replicas":      float64(3),
					"readyReplicas": float64(2),
				}
				return obj
			}(),
			expected: "2/3",
		},
		{
			name: "deployment without status",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.SetName("deploy")
				return obj
			}(),
			expected: "0/0",
		},
		{
			name: "deployment with non-map status",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.SetName("deploy")
				obj.Object["status"] = "not-a-map"
				return obj
			}(),
			expected: "0/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDeploymentReadyStatus(tt.obj)
			if got != tt.expected {
				t.Errorf("getDeploymentReadyStatus() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- getPodReadyStatus ----------

func TestGetPodReadyStatus(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name: "pod with container statuses",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["status"] = map[string]interface{}{
					"containerStatuses": []interface{}{
						map[string]interface{}{"ready": true},
						map[string]interface{}{"ready": false},
					},
				}
				return obj
			}(),
			expected: "1/2",
		},
		{
			name: "pod without status",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				return obj
			}(),
			expected: "0/0",
		},
		{
			name: "pod with non-map status",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["status"] = "not-a-map"
				return obj
			}(),
			expected: "0/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPodReadyStatus(tt.obj)
			if got != tt.expected {
				t.Errorf("getPodReadyStatus() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- getPodRestarts ----------

func TestGetPodRestarts(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name: "pod with restarts",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["status"] = map[string]interface{}{
					"containerStatuses": []interface{}{
						map[string]interface{}{"restartCount": float64(3)},
						map[string]interface{}{"restartCount": float64(2)},
					},
				}
				return obj
			}(),
			expected: "5",
		},
		{
			name: "pod without status",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				return obj
			}(),
			expected: "0",
		},
		{
			name: "pod with non-map status",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["status"] = "not-a-map"
				return obj
			}(),
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPodRestarts(tt.obj)
			if got != tt.expected {
				t.Errorf("getPodRestarts() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- getPodNode ----------

func TestGetPodNode(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name: "pod with node",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["spec"] = map[string]interface{}{
					"nodeName": "node-1",
				}
				return obj
			}(),
			expected: "node-1",
		},
		{
			name: "pod without spec",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				return obj
			}(),
			expected: "<none>",
		},
		{
			name: "pod with non-map spec",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["spec"] = "not-a-map"
				return obj
			}(),
			expected: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPodNode(tt.obj)
			if got != tt.expected {
				t.Errorf("getPodNode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- GetDetailedStatus ----------

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
			name: "object with status message",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["status"] = map[string]interface{}{
					"message": "OOMKilled",
				}
				return obj
			}(),
			expected: "OOMKilled",
		},
		{
			name: "object with status reason",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["status"] = map[string]interface{}{
					"reason": "Evicted",
				}
				return obj
			}(),
			expected: "Evicted",
		},
		{
			name: "object with status message and reason",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["status"] = map[string]interface{}{
					"message": "Container failed",
					"reason":  "CrashLoopBackOff",
				}
				return obj
			}(),
			expected: "Container failed, CrashLoopBackOff",
		},
		{
			name: "object without status",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				return obj
			}(),
			expected: "Unknown",
		},
		{
			name: "object with non-map status",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("pod")
				obj.Object["status"] = "not-a-map"
				return obj
			}(),
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDetailedStatus(tt.obj)
			if got != tt.expected {
				t.Errorf("GetDetailedStatus() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- getServiceExternalIP ----------

func TestGetServiceExternalIP(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name: "service with external IP from ingress",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["status"] = map[string]interface{}{
					"loadBalancer": map[string]interface{}{
						"ingress": []interface{}{
							map[string]interface{}{"ip": "1.2.3.4"},
						},
					},
				}
				return obj
			}(),
			expected: "1.2.3.4",
		},
		{
			name: "service with hostname from ingress",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["status"] = map[string]interface{}{
					"loadBalancer": map[string]interface{}{
						"ingress": []interface{}{
							map[string]interface{}{"hostname": "lb.example.com"},
						},
					},
				}
				return obj
			}(),
			expected: "lb.example.com",
		},
		{
			name: "service with spec externalIPs",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				obj.Object["spec"] = map[string]interface{}{
					"externalIPs": []interface{}{"5.6.7.8"},
				}
				return obj
			}(),
			expected: "5.6.7.8",
		},
		{
			name: "service without external IP",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Service")
				obj.SetName("svc")
				return obj
			}(),
			expected: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getServiceExternalIP(tt.obj)
			if got != tt.expected {
				t.Errorf("getServiceExternalIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------- DefaultColumns accessor edge cases ----------

func TestDefaultColumnsNamespaceAccessor(t *testing.T) {
	cols := DefaultColumns()

	// Find the NAMESPACE column
	var nsAccessor func(client.Object) string
	for _, col := range cols {
		if col.Header == "NAMESPACE" {
			nsAccessor = col.Accessor
			break
		}
	}
	if nsAccessor == nil {
		t.Fatal("NAMESPACE column not found")
	}

	// Test with empty namespace
	obj := &unstructured.Unstructured{}
	obj.SetKind("ConfigMap")
	obj.SetName("test")

	got := nsAccessor(obj)
	if got != "<none>" {
		t.Errorf("expected '<none>' for empty namespace, got %q", got)
	}

	// Test with namespace set
	obj.SetNamespace("default")
	got = nsAccessor(obj)
	if got != "default" {
		t.Errorf("expected 'default', got %q", got)
	}
}
