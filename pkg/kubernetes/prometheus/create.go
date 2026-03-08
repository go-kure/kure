package prometheus

import (
	intprom "github.com/go-kure/kure/internal/prometheus"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// ServiceMonitor converts the config to a Prometheus operator ServiceMonitor object.
func ServiceMonitor(cfg *ServiceMonitorConfig) *monitoringv1.ServiceMonitor {
	if cfg == nil {
		return nil
	}
	obj := intprom.CreateServiceMonitor(cfg.Name, cfg.Namespace, cfg.Selector)
	if cfg.Labels != nil {
		obj.Labels = cfg.Labels
	}
	for _, ep := range cfg.Endpoints {
		intprom.AddServiceMonitorEndpoint(obj, ep) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.JobLabel != "" {
		intprom.SetServiceMonitorJobLabel(obj, cfg.JobLabel) //nolint:errcheck,gosec // obj is freshly created
	}
	for _, label := range cfg.TargetLabels {
		intprom.AddServiceMonitorTargetLabel(obj, label) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.NamespaceSelector != nil {
		intprom.SetServiceMonitorNamespaceSelector(obj, *cfg.NamespaceSelector) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.SampleLimit != nil {
		intprom.SetServiceMonitorSampleLimit(obj, *cfg.SampleLimit) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}

// PodMonitor converts the config to a Prometheus operator PodMonitor object.
func PodMonitor(cfg *PodMonitorConfig) *monitoringv1.PodMonitor {
	if cfg == nil {
		return nil
	}
	obj := intprom.CreatePodMonitor(cfg.Name, cfg.Namespace, cfg.Selector)
	if cfg.Labels != nil {
		obj.Labels = cfg.Labels
	}
	for _, ep := range cfg.PodMetricsEndpoints {
		intprom.AddPodMonitorEndpoint(obj, ep) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.JobLabel != "" {
		intprom.SetPodMonitorJobLabel(obj, cfg.JobLabel) //nolint:errcheck,gosec // obj is freshly created
	}
	for _, label := range cfg.PodTargetLabels {
		intprom.AddPodMonitorPodTargetLabel(obj, label) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.NamespaceSelector != nil {
		intprom.SetPodMonitorNamespaceSelector(obj, *cfg.NamespaceSelector) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.SampleLimit != nil {
		intprom.SetPodMonitorSampleLimit(obj, *cfg.SampleLimit) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}

// PrometheusRule converts the config to a Prometheus operator PrometheusRule object.
func PrometheusRule(cfg *PrometheusRuleConfig) *monitoringv1.PrometheusRule {
	if cfg == nil {
		return nil
	}
	obj := intprom.CreatePrometheusRule(cfg.Name, cfg.Namespace)
	if cfg.Labels != nil {
		obj.Labels = cfg.Labels
	}
	for _, group := range cfg.Groups {
		intprom.AddPrometheusRuleGroup(obj, group) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}
