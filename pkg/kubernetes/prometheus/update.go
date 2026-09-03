package prometheus

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// AddServiceMonitorEndpoint appends an endpoint to the ServiceMonitor.
func AddServiceMonitorEndpoint(obj *monitoringv1.ServiceMonitor, ep monitoringv1.Endpoint) {
	obj.Spec.Endpoints = append(obj.Spec.Endpoints, ep)
}

// SetServiceMonitorSampleLimit sets the per-scrape sample limit.
func SetServiceMonitorSampleLimit(obj *monitoringv1.ServiceMonitor, limit int64) {
	obj.Spec.SampleLimit = &limit
}

// AddServiceMonitorTargetLabel appends a target label to the ServiceMonitor.
func AddServiceMonitorTargetLabel(obj *monitoringv1.ServiceMonitor, label string) {
	obj.Spec.TargetLabels = append(obj.Spec.TargetLabels, label)
}

// AddPodMonitorEndpoint appends a pod metrics endpoint to the PodMonitor.
func AddPodMonitorEndpoint(obj *monitoringv1.PodMonitor, ep monitoringv1.PodMetricsEndpoint) {
	obj.Spec.PodMetricsEndpoints = append(obj.Spec.PodMetricsEndpoints, ep)
}

// SetPodMonitorSampleLimit sets the per-scrape sample limit.
func SetPodMonitorSampleLimit(obj *monitoringv1.PodMonitor, limit int64) {
	obj.Spec.SampleLimit = &limit
}

// AddPodMonitorPodTargetLabel appends a pod target label to the PodMonitor.
func AddPodMonitorPodTargetLabel(obj *monitoringv1.PodMonitor, label string) {
	obj.Spec.PodTargetLabels = append(obj.Spec.PodTargetLabels, label)
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
