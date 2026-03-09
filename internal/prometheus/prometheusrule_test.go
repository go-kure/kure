package prometheus

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestCreatePrometheusRule(t *testing.T) {
	obj := CreatePrometheusRule("test-rule", "monitoring")
	if obj == nil {
		t.Fatal("expected non-nil PrometheusRule")
	}
	if obj.Name != "test-rule" {
		t.Errorf("expected name test-rule, got %s", obj.Name)
	}
	if obj.Namespace != "monitoring" {
		t.Errorf("expected namespace monitoring, got %s", obj.Namespace)
	}
	if obj.Kind != monitoringv1.PrometheusRuleKind {
		t.Errorf("expected kind %s, got %s", monitoringv1.PrometheusRuleKind, obj.Kind)
	}
	if obj.Spec.Groups == nil {
		t.Error("expected non-nil Groups")
	}
}

func TestAddPrometheusRuleGroup(t *testing.T) {
	obj := CreatePrometheusRule("test", "ns")
	group := CreateRuleGroup("test-group")
	AddPrometheusRuleGroup(obj, group)
	if len(obj.Spec.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(obj.Spec.Groups))
	}
	if obj.Spec.Groups[0].Name != "test-group" {
		t.Errorf("expected group name test-group, got %s", obj.Spec.Groups[0].Name)
	}
}

func TestCreateRuleGroup(t *testing.T) {
	group := CreateRuleGroup("my-group")
	if group.Name != "my-group" {
		t.Errorf("expected name my-group, got %s", group.Name)
	}
	if group.Rules == nil {
		t.Error("expected non-nil Rules")
	}
}

func TestAddRuleGroupRule(t *testing.T) {
	group := CreateRuleGroup("test")
	rule := monitoringv1.Rule{
		Alert: "HighErrorRate",
		Expr:  intstr.FromString("rate(http_errors_total[5m]) > 0.1"),
		Labels: map[string]string{
			"severity": "critical",
		},
		Annotations: map[string]string{
			"summary": "High error rate detected",
		},
	}
	AddRuleGroupRule(&group, rule)
	if len(group.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(group.Rules))
	}
	if group.Rules[0].Alert != "HighErrorRate" {
		t.Errorf("expected alert HighErrorRate, got %s", group.Rules[0].Alert)
	}
}

func TestSetRuleGroupInterval(t *testing.T) {
	group := CreateRuleGroup("test")
	interval := monitoringv1.Duration("30s")
	SetRuleGroupInterval(&group, interval)
	if group.Interval == nil || *group.Interval != "30s" {
		t.Error("expected interval 30s")
	}
}
