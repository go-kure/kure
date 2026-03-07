package prometheus

import (
	"errors"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateServiceMonitor returns a new ServiceMonitor with the provided name,
// namespace and label selector.
func CreateServiceMonitor(name, namespace string, selector metav1.LabelSelector) *monitoringv1.ServiceMonitor {
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
			Selector:  selector,
			Endpoints: []monitoringv1.Endpoint{},
		},
	}
}

// AddServiceMonitorEndpoint appends an endpoint to the ServiceMonitor.
func AddServiceMonitorEndpoint(obj *monitoringv1.ServiceMonitor, ep monitoringv1.Endpoint) error {
	if obj == nil {
		return errors.New("nil ServiceMonitor")
	}
	obj.Spec.Endpoints = append(obj.Spec.Endpoints, ep)
	return nil
}

// SetServiceMonitorJobLabel sets the jobLabel field.
func SetServiceMonitorJobLabel(obj *monitoringv1.ServiceMonitor, label string) error {
	if obj == nil {
		return errors.New("nil ServiceMonitor")
	}
	obj.Spec.JobLabel = label
	return nil
}

// SetServiceMonitorNamespaceSelector sets the namespace selector.
func SetServiceMonitorNamespaceSelector(obj *monitoringv1.ServiceMonitor, ns monitoringv1.NamespaceSelector) error {
	if obj == nil {
		return errors.New("nil ServiceMonitor")
	}
	obj.Spec.NamespaceSelector = ns
	return nil
}

// SetServiceMonitorSampleLimit sets the per-scrape sample limit.
func SetServiceMonitorSampleLimit(obj *monitoringv1.ServiceMonitor, limit uint64) error {
	if obj == nil {
		return errors.New("nil ServiceMonitor")
	}
	obj.Spec.SampleLimit = &limit
	return nil
}

// AddServiceMonitorTargetLabel appends a target label.
func AddServiceMonitorTargetLabel(obj *monitoringv1.ServiceMonitor, label string) error {
	if obj == nil {
		return errors.New("nil ServiceMonitor")
	}
	obj.Spec.TargetLabels = append(obj.Spec.TargetLabels, label)
	return nil
}
