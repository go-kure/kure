package prometheus

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/go-kure/kure/internal/kuretest"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// The fixtures under testdata were written by the config-struct layer this
// package used to carry (ServiceMonitor(&ServiceMonitorConfig{...}),
// PodMonitor, PrometheusRule) before it was retired. Each test below builds
// the same object on the generated constructor plus the upstream struct and
// must reproduce that output byte for byte.

func TestGolden_ServiceMonitor(t *testing.T) {
	obj := CreateServiceMonitor("my-app", "monitoring")
	kubernetes.SetLabels(obj, map[string]string{"team": "platform"})
	obj.Spec = monitoringv1.ServiceMonitorSpec{
		Selector:          metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
		Endpoints:         []monitoringv1.Endpoint{{Port: "metrics", Path: "/metrics", Interval: "30s"}, {Port: "admin"}},
		JobLabel:          "app",
		TargetLabels:      []string{"version", "env"},
		NamespaceSelector: monitoringv1.NamespaceSelector{MatchNames: []string{"prod", "staging"}},
	}
	SetServiceMonitorSampleLimit(obj, 5000)
	kuretest.Golden(t, "servicemonitor.yaml", obj)
}

func TestGolden_PodMonitor(t *testing.T) {
	obj := CreatePodMonitor("my-pods", "monitoring")
	kubernetes.SetLabels(obj, map[string]string{"team": "platform"})
	obj.Spec = monitoringv1.PodMonitorSpec{
		Selector:            metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
		PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: ptr.To("metrics"), Path: "/metrics"}},
		JobLabel:            "app",
		PodTargetLabels:     []string{"version"},
		NamespaceSelector:   monitoringv1.NamespaceSelector{Any: true},
	}
	SetPodMonitorSampleLimit(obj, 1000)
	kuretest.Golden(t, "podmonitor.yaml", obj)
}

func TestGolden_PrometheusRule(t *testing.T) {
	forDur := monitoringv1.Duration("5m")
	obj := CreatePrometheusRule("my-alerts", "monitoring")
	kubernetes.SetLabels(obj, map[string]string{"role": "alert-rules"})
	AddPrometheusRuleGroup(obj, monitoringv1.RuleGroup{
		Name: "http-alerts",
		Rules: []monitoringv1.Rule{{
			Alert:  "HighErrorRate",
			Expr:   intstr.FromString("rate(http_errors_total[5m]) > 0.1"),
			For:    &forDur,
			Labels: map[string]string{"severity": "critical"},
		}},
	})
	kuretest.Golden(t, "prometheusrule.yaml", obj)
}
