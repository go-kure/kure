package prometheus

import (
	intprom "github.com/go-kure/kure/internal/prometheus"
	"github.com/go-kure/kure/pkg/errors"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// Nil resource errors for Prometheus operator resources.
var (
	ErrNilServiceMonitor = errors.ResourceValidationError("ServiceMonitor", "", "servicemonitor", "service monitor cannot be nil", nil)
	ErrNilPodMonitor     = errors.ResourceValidationError("PodMonitor", "", "podmonitor", "pod monitor cannot be nil", nil)
	ErrNilPrometheusRule = errors.ResourceValidationError("PrometheusRule", "", "prometheusrule", "prometheus rule cannot be nil", nil)
)

// AddServiceMonitorEndpoint appends an endpoint to the ServiceMonitor.
func AddServiceMonitorEndpoint(obj *monitoringv1.ServiceMonitor, ep monitoringv1.Endpoint) error {
	if obj == nil {
		return ErrNilServiceMonitor
	}
	return intprom.AddServiceMonitorEndpoint(obj, ep)
}

// SetServiceMonitorJobLabel sets the jobLabel field.
func SetServiceMonitorJobLabel(obj *monitoringv1.ServiceMonitor, label string) error {
	if obj == nil {
		return ErrNilServiceMonitor
	}
	return intprom.SetServiceMonitorJobLabel(obj, label)
}

// SetServiceMonitorNamespaceSelector sets the namespace selector.
func SetServiceMonitorNamespaceSelector(obj *monitoringv1.ServiceMonitor, ns monitoringv1.NamespaceSelector) error {
	if obj == nil {
		return ErrNilServiceMonitor
	}
	return intprom.SetServiceMonitorNamespaceSelector(obj, ns)
}

// SetServiceMonitorSampleLimit sets the per-scrape sample limit.
func SetServiceMonitorSampleLimit(obj *monitoringv1.ServiceMonitor, limit uint64) error {
	if obj == nil {
		return ErrNilServiceMonitor
	}
	return intprom.SetServiceMonitorSampleLimit(obj, limit)
}

// AddPodMonitorEndpoint appends a pod metrics endpoint to the PodMonitor.
func AddPodMonitorEndpoint(obj *monitoringv1.PodMonitor, ep monitoringv1.PodMetricsEndpoint) error {
	if obj == nil {
		return ErrNilPodMonitor
	}
	return intprom.AddPodMonitorEndpoint(obj, ep)
}

// SetPodMonitorJobLabel sets the jobLabel field.
func SetPodMonitorJobLabel(obj *monitoringv1.PodMonitor, label string) error {
	if obj == nil {
		return ErrNilPodMonitor
	}
	return intprom.SetPodMonitorJobLabel(obj, label)
}

// SetPodMonitorNamespaceSelector sets the namespace selector.
func SetPodMonitorNamespaceSelector(obj *monitoringv1.PodMonitor, ns monitoringv1.NamespaceSelector) error {
	if obj == nil {
		return ErrNilPodMonitor
	}
	return intprom.SetPodMonitorNamespaceSelector(obj, ns)
}

// SetPodMonitorSampleLimit sets the per-scrape sample limit.
func SetPodMonitorSampleLimit(obj *monitoringv1.PodMonitor, limit uint64) error {
	if obj == nil {
		return ErrNilPodMonitor
	}
	return intprom.SetPodMonitorSampleLimit(obj, limit)
}

// AddPrometheusRuleGroup appends a rule group to the PrometheusRule.
func AddPrometheusRuleGroup(obj *monitoringv1.PrometheusRule, group monitoringv1.RuleGroup) error {
	if obj == nil {
		return ErrNilPrometheusRule
	}
	return intprom.AddPrometheusRuleGroup(obj, group)
}
