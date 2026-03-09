package prometheus

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateServiceMonitor(t *testing.T) {
	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "test"},
	}
	obj := CreateServiceMonitor("test-sm", "monitoring", selector)
	if obj == nil {
		t.Fatal("expected non-nil ServiceMonitor")
	}
	if obj.Name != "test-sm" {
		t.Errorf("expected name test-sm, got %s", obj.Name)
	}
	if obj.Namespace != "monitoring" {
		t.Errorf("expected namespace monitoring, got %s", obj.Namespace)
	}
	if obj.Kind != monitoringv1.ServiceMonitorsKind {
		t.Errorf("expected kind %s, got %s", monitoringv1.ServiceMonitorsKind, obj.Kind)
	}
	if obj.Spec.Endpoints == nil {
		t.Error("expected non-nil Endpoints")
	}
}

func TestAddServiceMonitorEndpoint(t *testing.T) {
	obj := CreateServiceMonitor("test", "ns", metav1.LabelSelector{})
	ep := monitoringv1.Endpoint{Port: "metrics"}
	AddServiceMonitorEndpoint(obj, ep)
	if len(obj.Spec.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(obj.Spec.Endpoints))
	}
	if obj.Spec.Endpoints[0].Port != "metrics" {
		t.Errorf("expected port metrics, got %s", obj.Spec.Endpoints[0].Port)
	}
}

func TestSetServiceMonitorJobLabel(t *testing.T) {
	obj := CreateServiceMonitor("test", "ns", metav1.LabelSelector{})
	SetServiceMonitorJobLabel(obj, "app")
	if obj.Spec.JobLabel != "app" {
		t.Errorf("expected jobLabel app, got %s", obj.Spec.JobLabel)
	}
}

func TestSetServiceMonitorNamespaceSelector(t *testing.T) {
	obj := CreateServiceMonitor("test", "ns", metav1.LabelSelector{})
	ns := monitoringv1.NamespaceSelector{Any: true}
	SetServiceMonitorNamespaceSelector(obj, ns)
	if !obj.Spec.NamespaceSelector.Any {
		t.Error("expected namespaceSelector.Any to be true")
	}
}

func TestSetServiceMonitorSampleLimit(t *testing.T) {
	obj := CreateServiceMonitor("test", "ns", metav1.LabelSelector{})
	SetServiceMonitorSampleLimit(obj, 5000)
	if obj.Spec.SampleLimit == nil || *obj.Spec.SampleLimit != 5000 {
		t.Error("expected sampleLimit 5000")
	}
}

func TestAddServiceMonitorTargetLabel(t *testing.T) {
	obj := CreateServiceMonitor("test", "ns", metav1.LabelSelector{})
	AddServiceMonitorTargetLabel(obj, "version")
	if len(obj.Spec.TargetLabels) != 1 || obj.Spec.TargetLabels[0] != "version" {
		t.Error("expected targetLabel version")
	}
}
