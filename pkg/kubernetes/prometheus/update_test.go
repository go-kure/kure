package prometheus

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func TestSetServiceMonitorSampleLimit_Public(t *testing.T) {
	obj := ServiceMonitor(&ServiceMonitorConfig{
		Name: "test", Namespace: "ns", Selector: metav1.LabelSelector{},
	})
	SetServiceMonitorSampleLimit(obj, 100)
	if obj.Spec.SampleLimit == nil || *obj.Spec.SampleLimit != 100 {
		t.Error("sample limit not set")
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

func TestCreateRuleGroup(t *testing.T) {
	g := CreateRuleGroup("test-group")
	if g.Name != "test-group" {
		t.Errorf("expected name test-group, got %s", g.Name)
	}
	if g.Rules == nil {
		t.Error("expected initialized Rules slice")
	}
}

func TestAddRuleGroupRule(t *testing.T) {
	g := CreateRuleGroup("test-group")
	rule := monitoringv1.Rule{Alert: "HighErrorRate", Expr: intstr.FromString("error_rate > 0.1")}
	AddRuleGroupRule(&g, rule)
	if len(g.Rules) != 1 || g.Rules[0].Alert != "HighErrorRate" {
		t.Errorf("rule not appended: %+v", g.Rules)
	}
}

func TestSetRuleGroupInterval(t *testing.T) {
	g := CreateRuleGroup("test-group")
	interval := monitoringv1.Duration("1m")
	SetRuleGroupInterval(&g, interval)
	if g.Interval == nil || *g.Interval != interval {
		t.Errorf("interval not set: %v", g.Interval)
	}
}
