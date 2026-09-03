package prometheus

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// ServiceMonitor converts the config to a Prometheus operator ServiceMonitor object.
func ServiceMonitor(cfg *ServiceMonitorConfig) *monitoringv1.ServiceMonitor {
	if cfg == nil {
		return nil
	}
	obj := CreateServiceMonitor(cfg.Name, cfg.Namespace)
	obj.Spec.Selector = cfg.Selector
	if cfg.Labels != nil {
		obj.Labels = cfg.Labels
	}
	for _, ep := range cfg.Endpoints {
		AddServiceMonitorEndpoint(obj, ep)
	}
	if cfg.JobLabel != "" {
		obj.Spec.JobLabel = cfg.JobLabel
	}
	for _, label := range cfg.TargetLabels {
		AddServiceMonitorTargetLabel(obj, label)
	}
	if cfg.NamespaceSelector != nil {
		obj.Spec.NamespaceSelector = *cfg.NamespaceSelector
	}
	if cfg.SampleLimit != nil {
		SetServiceMonitorSampleLimit(obj, *cfg.SampleLimit)
	}
	return obj
}

// PodMonitor converts the config to a Prometheus operator PodMonitor object.
func PodMonitor(cfg *PodMonitorConfig) *monitoringv1.PodMonitor {
	if cfg == nil {
		return nil
	}
	obj := CreatePodMonitor(cfg.Name, cfg.Namespace)
	obj.Spec.Selector = cfg.Selector
	if cfg.Labels != nil {
		obj.Labels = cfg.Labels
	}
	for _, ep := range cfg.PodMetricsEndpoints {
		AddPodMonitorEndpoint(obj, ep)
	}
	if cfg.JobLabel != "" {
		obj.Spec.JobLabel = cfg.JobLabel
	}
	for _, label := range cfg.PodTargetLabels {
		AddPodMonitorPodTargetLabel(obj, label)
	}
	if cfg.NamespaceSelector != nil {
		obj.Spec.NamespaceSelector = *cfg.NamespaceSelector
	}
	if cfg.SampleLimit != nil {
		SetPodMonitorSampleLimit(obj, *cfg.SampleLimit)
	}
	return obj
}

// PrometheusRule converts the config to a Prometheus operator PrometheusRule object.
func PrometheusRule(cfg *PrometheusRuleConfig) *monitoringv1.PrometheusRule {
	if cfg == nil {
		return nil
	}
	obj := CreatePrometheusRule(cfg.Name, cfg.Namespace)
	if cfg.Labels != nil {
		obj.Labels = cfg.Labels
	}
	for _, group := range cfg.Groups {
		AddPrometheusRuleGroup(obj, group)
	}
	return obj
}
