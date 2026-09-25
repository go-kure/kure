package prometheus_test

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/kubernetes/prometheus"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

func ExampleCreateServiceMonitor() {
	obj := prometheus.CreateServiceMonitor("my-app", "monitoring")
	fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name)
	// Output: ServiceMonitor monitoring/my-app
}

func ExampleSetServiceMonitorSampleLimit() {
	sm := prometheus.CreateServiceMonitor("my-app", "monitoring")
	kubernetes.AddLabel(sm, "team", "platform")
	sm.Spec = monitoringv1.ServiceMonitorSpec{
		Selector:     metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
		Endpoints:    []monitoringv1.Endpoint{{Path: "/metrics", Port: "http", Interval: "30s"}},
		JobLabel:     "app",
		TargetLabels: []string{"app", "version"},
	}
	prometheus.SetServiceMonitorSampleLimit(sm, 10000)
	fmt.Println(sm.Labels["team"], sm.Spec.Endpoints[0].Port, *sm.Spec.SampleLimit)
	// Output: platform http 10000
}

func ExampleCreatePodMonitor() {
	pm := prometheus.CreatePodMonitor("my-app", "monitoring")
	pm.Spec = monitoringv1.PodMonitorSpec{
		Selector:            metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
		PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Path: "/metrics", Port: ptr.To("http"), Interval: "30s"}},
		NamespaceSelector:   monitoringv1.NamespaceSelector{Any: true},
	}
	fmt.Println(*pm.Spec.PodMetricsEndpoints[0].Port, pm.Spec.NamespaceSelector.Any)
	// Output: http true
}

func ExampleCreateRuleGroup() {
	rule := prometheus.CreatePrometheusRule("alerts", "monitoring")
	kubernetes.AddLabel(rule, "role", "alert-rules")

	group := prometheus.CreateRuleGroup("my-app.rules")
	prometheus.AddRuleGroupRule(&group, monitoringv1.Rule{
		Alert: "HighErrorRate",
		Expr:  intstr.FromString(`rate(http_requests_total{code=~"5.."}[5m]) > 0.1`),
		For:   ptr.To(monitoringv1.Duration("5m")),
	})
	prometheus.AddPrometheusRuleGroup(rule, group)
	fmt.Println(rule.Spec.Groups[0].Name, rule.Spec.Groups[0].Rules[0].Alert)
	// Output: my-app.rules HighErrorRate
}

func ExampleAddServiceMonitorEndpoint() {
	sm := prometheus.CreateServiceMonitor("my-app", "monitoring")
	pm := prometheus.CreatePodMonitor("my-app", "monitoring")
	rule := prometheus.CreatePrometheusRule("alerts", "monitoring")
	group := prometheus.CreateRuleGroup("my-app.rules")

	// ServiceMonitor modifiers
	prometheus.AddServiceMonitorEndpoint(sm, monitoringv1.Endpoint{Path: "/metrics", Port: "http"})
	sm.Spec.JobLabel = "app"
	prometheus.AddServiceMonitorTargetLabel(sm, "version")
	sm.Spec.NamespaceSelector = monitoringv1.NamespaceSelector{Any: true}
	prometheus.SetServiceMonitorSampleLimit(sm, 10000)

	// PodMonitor modifiers
	prometheus.AddPodMonitorEndpoint(pm, monitoringv1.PodMetricsEndpoint{Path: "/metrics", Port: ptr.To("http")})
	pm.Spec.JobLabel = "app"
	prometheus.AddPodMonitorPodTargetLabel(pm, "version")
	pm.Spec.NamespaceSelector = monitoringv1.NamespaceSelector{Any: true}
	prometheus.SetPodMonitorSampleLimit(pm, 10000)

	// PrometheusRule modifiers
	prometheus.AddPrometheusRuleGroup(rule, monitoringv1.RuleGroup{Name: "extra.rules"})
	prometheus.SetRuleGroupInterval(&group, monitoringv1.Duration("1m"))

	fmt.Println(sm.Spec.TargetLabels, pm.Spec.PodTargetLabels, rule.Spec.Groups[0].Name, *group.Interval)
	// Output: [version] [version] extra.rules 1m
}
