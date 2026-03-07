package prometheus

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddServiceMonitorEndpoint_Public(t *testing.T) {
	obj := ServiceMonitor(&ServiceMonitorConfig{
		Name:      "test",
		Namespace: "ns",
		Selector:  metav1.LabelSelector{},
	})
	if err := AddServiceMonitorEndpoint(obj, monitoringv1.Endpoint{Port: "http"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obj.Spec.Endpoints) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(obj.Spec.Endpoints))
	}
}

func TestAddServiceMonitorEndpointNil_Public(t *testing.T) {
	if err := AddServiceMonitorEndpoint(nil, monitoringv1.Endpoint{}); err == nil {
		t.Error("expected error for nil ServiceMonitor")
	}
}

func TestSetServiceMonitorJobLabel_Public(t *testing.T) {
	obj := ServiceMonitor(&ServiceMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	if err := SetServiceMonitorJobLabel(obj, "job"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.JobLabel != "job" {
		t.Errorf("expected jobLabel job, got %s", obj.Spec.JobLabel)
	}
}

func TestSetServiceMonitorJobLabelNil_Public(t *testing.T) {
	if err := SetServiceMonitorJobLabel(nil, "job"); err == nil {
		t.Error("expected error for nil ServiceMonitor")
	}
}

func TestAddPodMonitorEndpoint_Public(t *testing.T) {
	obj := PodMonitor(&PodMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	port := "http"
	if err := AddPodMonitorEndpoint(obj, monitoringv1.PodMetricsEndpoint{Port: &port}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obj.Spec.PodMetricsEndpoints) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(obj.Spec.PodMetricsEndpoints))
	}
}

func TestAddPodMonitorEndpointNil_Public(t *testing.T) {
	if err := AddPodMonitorEndpoint(nil, monitoringv1.PodMetricsEndpoint{}); err == nil {
		t.Error("expected error for nil PodMonitor")
	}
}

func TestAddPrometheusRuleGroup_Public(t *testing.T) {
	obj := PrometheusRule(&PrometheusRuleConfig{
		Name: "test", Namespace: "ns",
	})
	if err := AddPrometheusRuleGroup(obj, monitoringv1.RuleGroup{Name: "grp"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obj.Spec.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(obj.Spec.Groups))
	}
}

func TestAddPrometheusRuleGroupNil_Public(t *testing.T) {
	if err := AddPrometheusRuleGroup(nil, monitoringv1.RuleGroup{}); err == nil {
		t.Error("expected error for nil PrometheusRule")
	}
}

func TestSetServiceMonitorNamespaceSelector_Public(t *testing.T) {
	if err := SetServiceMonitorNamespaceSelector(nil, monitoringv1.NamespaceSelector{}); err == nil {
		t.Error("expected error for nil ServiceMonitor")
	}
}

func TestSetServiceMonitorSampleLimit_Public(t *testing.T) {
	if err := SetServiceMonitorSampleLimit(nil, 100); err == nil {
		t.Error("expected error for nil ServiceMonitor")
	}
}

func TestSetPodMonitorJobLabel_Public(t *testing.T) {
	if err := SetPodMonitorJobLabel(nil, "job"); err == nil {
		t.Error("expected error for nil PodMonitor")
	}
}

func TestSetPodMonitorNamespaceSelector_Public(t *testing.T) {
	if err := SetPodMonitorNamespaceSelector(nil, monitoringv1.NamespaceSelector{}); err == nil {
		t.Error("expected error for nil PodMonitor")
	}
}

func TestSetPodMonitorSampleLimit_Public(t *testing.T) {
	if err := SetPodMonitorSampleLimit(nil, 100); err == nil {
		t.Error("expected error for nil PodMonitor")
	}
}
