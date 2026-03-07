package prometheus

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceMonitorConfig contains the configuration for a Prometheus ServiceMonitor.
type ServiceMonitorConfig struct {
	Name              string                          `yaml:"name"`
	Namespace         string                          `yaml:"namespace"`
	Selector          metav1.LabelSelector            `yaml:"selector"`
	Endpoints         []monitoringv1.Endpoint         `yaml:"endpoints"`
	JobLabel          string                          `yaml:"jobLabel,omitempty"`
	TargetLabels      []string                        `yaml:"targetLabels,omitempty"`
	NamespaceSelector *monitoringv1.NamespaceSelector `yaml:"namespaceSelector,omitempty"`
	SampleLimit       *uint64                         `yaml:"sampleLimit,omitempty"`
	Labels            map[string]string               `yaml:"labels,omitempty"`
}

// PodMonitorConfig contains the configuration for a Prometheus PodMonitor.
type PodMonitorConfig struct {
	Name                string                            `yaml:"name"`
	Namespace           string                            `yaml:"namespace"`
	Selector            metav1.LabelSelector              `yaml:"selector"`
	PodMetricsEndpoints []monitoringv1.PodMetricsEndpoint `yaml:"podMetricsEndpoints"`
	JobLabel            string                            `yaml:"jobLabel,omitempty"`
	PodTargetLabels     []string                          `yaml:"podTargetLabels,omitempty"`
	NamespaceSelector   *monitoringv1.NamespaceSelector   `yaml:"namespaceSelector,omitempty"`
	SampleLimit         *uint64                           `yaml:"sampleLimit,omitempty"`
	Labels              map[string]string                 `yaml:"labels,omitempty"`
}

// PrometheusRuleConfig contains the configuration for a PrometheusRule.
type PrometheusRuleConfig struct {
	Name      string                   `yaml:"name"`
	Namespace string                   `yaml:"namespace"`
	Groups    []monitoringv1.RuleGroup `yaml:"groups"`
	Labels    map[string]string        `yaml:"labels,omitempty"`
}
