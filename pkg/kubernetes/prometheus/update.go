package prometheus

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetServiceMonitorSpec replaces the full spec on the ServiceMonitor.
func SetServiceMonitorSpec(obj *monitoringv1.ServiceMonitor, spec monitoringv1.ServiceMonitorSpec) {
	obj.Spec = spec
}

// AddServiceMonitorEndpoint appends an endpoint to the ServiceMonitor.
func AddServiceMonitorEndpoint(obj *monitoringv1.ServiceMonitor, ep monitoringv1.Endpoint) {
	obj.Spec.Endpoints = append(obj.Spec.Endpoints, ep)
}

// SetServiceMonitorSelector sets the label selector on the ServiceMonitor.
func SetServiceMonitorSelector(obj *monitoringv1.ServiceMonitor, selector metav1.LabelSelector) {
	obj.Spec.Selector = selector
}

// SetServiceMonitorJobLabel sets the jobLabel field.
func SetServiceMonitorJobLabel(obj *monitoringv1.ServiceMonitor, label string) {
	obj.Spec.JobLabel = label
}

// SetServiceMonitorNamespaceSelector sets the namespace selector.
func SetServiceMonitorNamespaceSelector(obj *monitoringv1.ServiceMonitor, ns monitoringv1.NamespaceSelector) {
	obj.Spec.NamespaceSelector = ns
}

// SetServiceMonitorSampleLimit sets the per-scrape sample limit.
func SetServiceMonitorSampleLimit(obj *monitoringv1.ServiceMonitor, limit uint64) {
	obj.Spec.SampleLimit = &limit
}

// AddServiceMonitorTargetLabel appends a target label to the ServiceMonitor.
func AddServiceMonitorTargetLabel(obj *monitoringv1.ServiceMonitor, label string) {
	obj.Spec.TargetLabels = append(obj.Spec.TargetLabels, label)
}

// SetPodMonitorSpec replaces the full spec on the PodMonitor.
func SetPodMonitorSpec(obj *monitoringv1.PodMonitor, spec monitoringv1.PodMonitorSpec) {
	obj.Spec = spec
}

// AddPodMonitorEndpoint appends a pod metrics endpoint to the PodMonitor.
func AddPodMonitorEndpoint(obj *monitoringv1.PodMonitor, ep monitoringv1.PodMetricsEndpoint) {
	obj.Spec.PodMetricsEndpoints = append(obj.Spec.PodMetricsEndpoints, ep)
}

// SetPodMonitorSelector sets the label selector on the PodMonitor.
func SetPodMonitorSelector(obj *monitoringv1.PodMonitor, selector metav1.LabelSelector) {
	obj.Spec.Selector = selector
}

// SetPodMonitorJobLabel sets the jobLabel field.
func SetPodMonitorJobLabel(obj *monitoringv1.PodMonitor, label string) {
	obj.Spec.JobLabel = label
}

// SetPodMonitorNamespaceSelector sets the namespace selector.
func SetPodMonitorNamespaceSelector(obj *monitoringv1.PodMonitor, ns monitoringv1.NamespaceSelector) {
	obj.Spec.NamespaceSelector = ns
}

// SetPodMonitorSampleLimit sets the per-scrape sample limit.
func SetPodMonitorSampleLimit(obj *monitoringv1.PodMonitor, limit uint64) {
	obj.Spec.SampleLimit = &limit
}

// AddPodMonitorPodTargetLabel appends a pod target label to the PodMonitor.
func AddPodMonitorPodTargetLabel(obj *monitoringv1.PodMonitor, label string) {
	obj.Spec.PodTargetLabels = append(obj.Spec.PodTargetLabels, label)
}

// SetPrometheusRuleSpec replaces the full spec on the PrometheusRule.
func SetPrometheusRuleSpec(obj *monitoringv1.PrometheusRule, spec monitoringv1.PrometheusRuleSpec) {
	obj.Spec = spec
}

// AddPrometheusRuleGroup appends a rule group to the PrometheusRule.
func AddPrometheusRuleGroup(obj *monitoringv1.PrometheusRule, group monitoringv1.RuleGroup) {
	obj.Spec.Groups = append(obj.Spec.Groups, group)
}

// CreateRuleGroup returns a new RuleGroup with the provided name.
func CreateRuleGroup(name string) monitoringv1.RuleGroup {
	return monitoringv1.RuleGroup{
		Name:  name,
		Rules: []monitoringv1.Rule{},
	}
}

// AddRuleGroupRule appends a rule to the RuleGroup.
func AddRuleGroupRule(group *monitoringv1.RuleGroup, rule monitoringv1.Rule) {
	group.Rules = append(group.Rules, rule)
}

// SetRuleGroupInterval sets the evaluation interval for the rule group.
func SetRuleGroupInterval(group *monitoringv1.RuleGroup, interval monitoringv1.Duration) {
	group.Interval = &interval
}
