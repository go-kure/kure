package prometheus

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreatePodMonitor(t *testing.T) {
	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "test"},
	}
	obj := CreatePodMonitor("test-pm", "monitoring", selector)
	if obj == nil {
		t.Fatal("expected non-nil PodMonitor")
	}
	if obj.Name != "test-pm" {
		t.Errorf("expected name test-pm, got %s", obj.Name)
	}
	if obj.Namespace != "monitoring" {
		t.Errorf("expected namespace monitoring, got %s", obj.Namespace)
	}
	if obj.Kind != monitoringv1.PodMonitorsKind {
		t.Errorf("expected kind %s, got %s", monitoringv1.PodMonitorsKind, obj.Kind)
	}
	if obj.Spec.PodMetricsEndpoints == nil {
		t.Error("expected non-nil PodMetricsEndpoints")
	}
}

func TestAddPodMonitorEndpoint(t *testing.T) {
	obj := CreatePodMonitor("test", "ns", metav1.LabelSelector{})
	port := "metrics"
	ep := monitoringv1.PodMetricsEndpoint{Port: &port}
	if err := AddPodMonitorEndpoint(obj, ep); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obj.Spec.PodMetricsEndpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(obj.Spec.PodMetricsEndpoints))
	}
	if obj.Spec.PodMetricsEndpoints[0].Port == nil || *obj.Spec.PodMetricsEndpoints[0].Port != "metrics" {
		t.Error("expected port metrics")
	}
}

func TestAddPodMonitorEndpointNil(t *testing.T) {
	if err := AddPodMonitorEndpoint(nil, monitoringv1.PodMetricsEndpoint{}); err == nil {
		t.Error("expected error for nil PodMonitor")
	}
}

func TestSetPodMonitorJobLabel(t *testing.T) {
	obj := CreatePodMonitor("test", "ns", metav1.LabelSelector{})
	if err := SetPodMonitorJobLabel(obj, "app"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.JobLabel != "app" {
		t.Errorf("expected jobLabel app, got %s", obj.Spec.JobLabel)
	}
}

func TestSetPodMonitorNamespaceSelector(t *testing.T) {
	obj := CreatePodMonitor("test", "ns", metav1.LabelSelector{})
	ns := monitoringv1.NamespaceSelector{Any: true}
	if err := SetPodMonitorNamespaceSelector(obj, ns); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !obj.Spec.NamespaceSelector.Any {
		t.Error("expected namespaceSelector.Any to be true")
	}
}

func TestSetPodMonitorSampleLimit(t *testing.T) {
	obj := CreatePodMonitor("test", "ns", metav1.LabelSelector{})
	if err := SetPodMonitorSampleLimit(obj, 10000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.SampleLimit == nil || *obj.Spec.SampleLimit != 10000 {
		t.Error("expected sampleLimit 10000")
	}
}
