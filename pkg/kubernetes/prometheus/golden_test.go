package prometheus

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
)

var update = flag.Bool("update", false, "update golden files")

func goldenTest(t *testing.T, filename string, obj client.Object) {
	t.Helper()
	objects := []*client.Object{&obj}
	got, err := kureio.EncodeObjectsToYAMLWithOptions(objects, kureio.EncodeOptions{
		KubernetesFieldOrder: true,
	})
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}
	golden := filepath.Join("testdata", filename)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("updating golden file: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n\ngot:\n%s\nwant:\n%s", golden, got, want)
	}
}

func TestGolden_ServiceMonitor(t *testing.T) {
	limit := int64(5000)
	obj := ServiceMonitor(&ServiceMonitorConfig{
		Name:              "my-app",
		Namespace:         "monitoring",
		Selector:          metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
		Endpoints:         []monitoringv1.Endpoint{{Port: "metrics", Path: "/metrics", Interval: "30s"}, {Port: "admin"}},
		JobLabel:          "app",
		TargetLabels:      []string{"version", "env"},
		NamespaceSelector: &monitoringv1.NamespaceSelector{MatchNames: []string{"prod", "staging"}},
		SampleLimit:       &limit,
		Labels:            map[string]string{"team": "platform"},
	})
	goldenTest(t, "servicemonitor.yaml", obj)
}

func TestGolden_PodMonitor(t *testing.T) {
	limit := int64(1000)
	port := "metrics"
	obj := PodMonitor(&PodMonitorConfig{
		Name:                "my-pods",
		Namespace:           "monitoring",
		Selector:            metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
		PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: &port, Path: "/metrics"}},
		JobLabel:            "app",
		PodTargetLabels:     []string{"version"},
		NamespaceSelector:   &monitoringv1.NamespaceSelector{Any: true},
		SampleLimit:         &limit,
		Labels:              map[string]string{"team": "platform"},
	})
	goldenTest(t, "podmonitor.yaml", obj)
}

func TestGolden_PrometheusRule(t *testing.T) {
	forDur := monitoringv1.Duration("5m")
	obj := PrometheusRule(&PrometheusRuleConfig{
		Name:      "my-alerts",
		Namespace: "monitoring",
		Groups: []monitoringv1.RuleGroup{{
			Name: "http-alerts",
			Rules: []monitoringv1.Rule{{
				Alert:  "HighErrorRate",
				Expr:   intstr.FromString("rate(http_errors_total[5m]) > 0.1"),
				For:    &forDur,
				Labels: map[string]string{"severity": "critical"},
			}},
		}},
		Labels: map[string]string{"role": "alert-rules"},
	})
	goldenTest(t, "prometheusrule.yaml", obj)
}
