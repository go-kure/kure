package prometheus

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func strPtr(s string) *string { return &s }

func TestServiceMonitor(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		if ServiceMonitor(nil) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("basic config", func(t *testing.T) {
		cfg := &ServiceMonitorConfig{
			Name:      "my-app",
			Namespace: "monitoring",
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
			Endpoints: []monitoringv1.Endpoint{
				{Port: "metrics"},
				{Port: "admin"},
			},
			JobLabel:     "app",
			TargetLabels: []string{"version", "env"},
			Labels:       map[string]string{"team": "platform"},
		}
		obj := ServiceMonitor(cfg)
		if obj == nil {
			t.Fatal("expected non-nil ServiceMonitor")
		}
		if obj.Name != "my-app" {
			t.Errorf("expected name my-app, got %s", obj.Name)
		}
		if obj.Namespace != "monitoring" {
			t.Errorf("expected namespace monitoring, got %s", obj.Namespace)
		}
		if len(obj.Spec.Endpoints) != 2 {
			t.Errorf("expected 2 endpoints, got %d", len(obj.Spec.Endpoints))
		}
		if obj.Spec.JobLabel != "app" {
			t.Errorf("expected jobLabel app, got %s", obj.Spec.JobLabel)
		}
		if len(obj.Spec.TargetLabels) != 2 {
			t.Errorf("expected 2 target labels, got %d", len(obj.Spec.TargetLabels))
		}
		if obj.Labels["team"] != "platform" {
			t.Errorf("expected label team=platform")
		}
	})

	t.Run("with namespace selector and sample limit", func(t *testing.T) {
		limit := uint64(5000)
		cfg := &ServiceMonitorConfig{
			Name:      "test",
			Namespace: "ns",
			Selector:  metav1.LabelSelector{},
			NamespaceSelector: &monitoringv1.NamespaceSelector{
				MatchNames: []string{"prod", "staging"},
			},
			SampleLimit: &limit,
		}
		obj := ServiceMonitor(cfg)
		if len(obj.Spec.NamespaceSelector.MatchNames) != 2 {
			t.Errorf("expected 2 namespace matches")
		}
		if obj.Spec.SampleLimit == nil || *obj.Spec.SampleLimit != 5000 {
			t.Error("expected sampleLimit 5000")
		}
	})
}

func TestPodMonitor(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		if PodMonitor(nil) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("basic config", func(t *testing.T) {
		cfg := &PodMonitorConfig{
			Name:      "my-pods",
			Namespace: "monitoring",
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
				{Port: strPtr("metrics")},
			},
			JobLabel: "app",
			Labels:   map[string]string{"team": "platform"},
		}
		obj := PodMonitor(cfg)
		if obj == nil {
			t.Fatal("expected non-nil PodMonitor")
		}
		if obj.Name != "my-pods" {
			t.Errorf("expected name my-pods, got %s", obj.Name)
		}
		if len(obj.Spec.PodMetricsEndpoints) != 1 {
			t.Errorf("expected 1 endpoint, got %d", len(obj.Spec.PodMetricsEndpoints))
		}
		if obj.Spec.JobLabel != "app" {
			t.Errorf("expected jobLabel app, got %s", obj.Spec.JobLabel)
		}
		if obj.Labels["team"] != "platform" {
			t.Errorf("expected label team=platform")
		}
	})
}

func TestPrometheusRule(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		if PrometheusRule(nil) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("basic config", func(t *testing.T) {
		cfg := &PrometheusRuleConfig{
			Name:      "my-alerts",
			Namespace: "monitoring",
			Groups: []monitoringv1.RuleGroup{
				{
					Name: "http-alerts",
					Rules: []monitoringv1.Rule{
						{
							Alert: "HighErrorRate",
							Expr:  intstr.FromString("rate(http_errors_total[5m]) > 0.1"),
							Labels: map[string]string{
								"severity": "critical",
							},
						},
					},
				},
			},
			Labels: map[string]string{"team": "platform"},
		}
		obj := PrometheusRule(cfg)
		if obj == nil {
			t.Fatal("expected non-nil PrometheusRule")
		}
		if obj.Name != "my-alerts" {
			t.Errorf("expected name my-alerts, got %s", obj.Name)
		}
		if len(obj.Spec.Groups) != 1 {
			t.Errorf("expected 1 group, got %d", len(obj.Spec.Groups))
		}
		if len(obj.Spec.Groups[0].Rules) != 1 {
			t.Errorf("expected 1 rule, got %d", len(obj.Spec.Groups[0].Rules))
		}
		if obj.Labels["team"] != "platform" {
			t.Errorf("expected label team=platform")
		}
	})
}
