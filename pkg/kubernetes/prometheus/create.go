package prometheus

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateServiceMonitor returns a new ServiceMonitor with TypeMeta and
// ObjectMeta set. The selector and endpoints are left empty; use the setters
// to populate them.
func CreateServiceMonitor(name, namespace string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       monitoringv1.ServiceMonitorsKind,
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{},
		},
	}
}

// CreatePodMonitor returns a new PodMonitor with TypeMeta and ObjectMeta set.
// The selector and pod metrics endpoints are left empty; use the setters to
// populate them.
func CreatePodMonitor(name, namespace string) *monitoringv1.PodMonitor {
	return &monitoringv1.PodMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       monitoringv1.PodMonitorsKind,
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: monitoringv1.PodMonitorSpec{
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{},
		},
	}
}

// CreatePrometheusRule returns a new PrometheusRule with TypeMeta and
// ObjectMeta set. Groups are left empty; use AddPrometheusRuleGroup to
// populate them.
func CreatePrometheusRule(name, namespace string) *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			Kind:       monitoringv1.PrometheusRuleKind,
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{},
		},
	}
}

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
		SetServiceMonitorJobLabel(obj, cfg.JobLabel)
	}
	for _, label := range cfg.TargetLabels {
		AddServiceMonitorTargetLabel(obj, label)
	}
	if cfg.NamespaceSelector != nil {
		SetServiceMonitorNamespaceSelector(obj, *cfg.NamespaceSelector)
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
		SetPodMonitorJobLabel(obj, cfg.JobLabel)
	}
	for _, label := range cfg.PodTargetLabels {
		AddPodMonitorPodTargetLabel(obj, label)
	}
	if cfg.NamespaceSelector != nil {
		SetPodMonitorNamespaceSelector(obj, *cfg.NamespaceSelector)
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
