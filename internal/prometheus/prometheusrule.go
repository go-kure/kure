package prometheus

import (
	"errors"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreatePrometheusRule returns a new PrometheusRule with the provided name and
// namespace.
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

// AddPrometheusRuleGroup appends a rule group to the PrometheusRule.
func AddPrometheusRuleGroup(obj *monitoringv1.PrometheusRule, group monitoringv1.RuleGroup) error {
	if obj == nil {
		return errors.New("nil PrometheusRule")
	}
	obj.Spec.Groups = append(obj.Spec.Groups, group)
	return nil
}

// CreateRuleGroup returns a new RuleGroup with the provided name.
func CreateRuleGroup(name string) monitoringv1.RuleGroup {
	return monitoringv1.RuleGroup{
		Name:  name,
		Rules: []monitoringv1.Rule{},
	}
}

// AddRuleGroupRule appends a rule to the RuleGroup.
func AddRuleGroupRule(group *monitoringv1.RuleGroup, rule monitoringv1.Rule) error {
	if group == nil {
		return errors.New("nil RuleGroup")
	}
	group.Rules = append(group.Rules, rule)
	return nil
}

// SetRuleGroupInterval sets the evaluation interval for the rule group.
func SetRuleGroupInterval(group *monitoringv1.RuleGroup, interval monitoringv1.Duration) error {
	if group == nil {
		return errors.New("nil RuleGroup")
	}
	group.Interval = &interval
	return nil
}
