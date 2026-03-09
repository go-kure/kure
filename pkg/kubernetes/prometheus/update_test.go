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
	AddServiceMonitorEndpoint(obj, monitoringv1.Endpoint{Port: "http"})
	if len(obj.Spec.Endpoints) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(obj.Spec.Endpoints))
	}
}

func TestSetServiceMonitorJobLabel_Public(t *testing.T) {
	obj := ServiceMonitor(&ServiceMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	SetServiceMonitorJobLabel(obj, "job")
	if obj.Spec.JobLabel != "job" {
		t.Errorf("expected jobLabel job, got %s", obj.Spec.JobLabel)
	}
}

func TestAddPodMonitorEndpoint_Public(t *testing.T) {
	obj := PodMonitor(&PodMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	port := "http"
	AddPodMonitorEndpoint(obj, monitoringv1.PodMetricsEndpoint{Port: &port})
	if len(obj.Spec.PodMetricsEndpoints) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(obj.Spec.PodMetricsEndpoints))
	}
}

func TestAddPrometheusRuleGroup_Public(t *testing.T) {
	obj := PrometheusRule(&PrometheusRuleConfig{
		Name: "test", Namespace: "ns",
	})
	AddPrometheusRuleGroup(obj, monitoringv1.RuleGroup{Name: "grp"})
	if len(obj.Spec.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(obj.Spec.Groups))
	}
}

func TestSetServiceMonitorNamespaceSelector_Public(t *testing.T) {
	obj := ServiceMonitor(&ServiceMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	SetServiceMonitorNamespaceSelector(obj, monitoringv1.NamespaceSelector{Any: true})
	if !obj.Spec.NamespaceSelector.Any {
		t.Error("namespace selector not set")
	}
}

func TestSetServiceMonitorSampleLimit_Public(t *testing.T) {
	obj := ServiceMonitor(&ServiceMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	SetServiceMonitorSampleLimit(obj, 100)
	if obj.Spec.SampleLimit == nil || *obj.Spec.SampleLimit != 100 {
		t.Error("sample limit not set")
	}
}

func TestSetPodMonitorJobLabel_Public(t *testing.T) {
	obj := PodMonitor(&PodMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	SetPodMonitorJobLabel(obj, "job")
	if obj.Spec.JobLabel != "job" {
		t.Error("job label not set")
	}
}

func TestSetPodMonitorNamespaceSelector_Public(t *testing.T) {
	obj := PodMonitor(&PodMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	SetPodMonitorNamespaceSelector(obj, monitoringv1.NamespaceSelector{Any: true})
	if !obj.Spec.NamespaceSelector.Any {
		t.Error("namespace selector not set")
	}
}

func TestSetPodMonitorSampleLimit_Public(t *testing.T) {
	obj := PodMonitor(&PodMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	SetPodMonitorSampleLimit(obj, 100)
	if obj.Spec.SampleLimit == nil || *obj.Spec.SampleLimit != 100 {
		t.Error("sample limit not set")
	}
}

func TestAddServiceMonitorTargetLabel_Public(t *testing.T) {
	obj := ServiceMonitor(&ServiceMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	AddServiceMonitorTargetLabel(obj, "version")
	if len(obj.Spec.TargetLabels) != 1 || obj.Spec.TargetLabels[0] != "version" {
		t.Error("expected targetLabel version")
	}
}

func TestAddPodMonitorPodTargetLabel_Public(t *testing.T) {
	obj := PodMonitor(&PodMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	AddPodMonitorPodTargetLabel(obj, "version")
	if len(obj.Spec.PodTargetLabels) != 1 || obj.Spec.PodTargetLabels[0] != "version" {
		t.Error("expected podTargetLabel version")
	}
}
