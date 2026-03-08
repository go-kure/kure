package prometheus

import (
	"github.com/go-kure/kure/pkg/errors"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreatePodMonitor returns a new PodMonitor with the provided name,
// namespace and label selector.
func CreatePodMonitor(name, namespace string, selector metav1.LabelSelector) *monitoringv1.PodMonitor {
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
			Selector:            selector,
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{},
		},
	}
}

// AddPodMonitorEndpoint appends a pod metrics endpoint to the PodMonitor.
func AddPodMonitorEndpoint(obj *monitoringv1.PodMonitor, ep monitoringv1.PodMetricsEndpoint) error {
	if obj == nil {
		return errors.ErrNilPodMonitor
	}
	obj.Spec.PodMetricsEndpoints = append(obj.Spec.PodMetricsEndpoints, ep)
	return nil
}

// SetPodMonitorJobLabel sets the jobLabel field.
func SetPodMonitorJobLabel(obj *monitoringv1.PodMonitor, label string) error {
	if obj == nil {
		return errors.ErrNilPodMonitor
	}
	obj.Spec.JobLabel = label
	return nil
}

// SetPodMonitorNamespaceSelector sets the namespace selector.
func SetPodMonitorNamespaceSelector(obj *monitoringv1.PodMonitor, ns monitoringv1.NamespaceSelector) error {
	if obj == nil {
		return errors.ErrNilPodMonitor
	}
	obj.Spec.NamespaceSelector = ns
	return nil
}

// SetPodMonitorSampleLimit sets the per-scrape sample limit.
func SetPodMonitorSampleLimit(obj *monitoringv1.PodMonitor, limit uint64) error {
	if obj == nil {
		return errors.ErrNilPodMonitor
	}
	obj.Spec.SampleLimit = &limit
	return nil
}

// AddPodMonitorPodTargetLabel appends a pod target label.
func AddPodMonitorPodTargetLabel(obj *monitoringv1.PodMonitor, label string) error {
	if obj == nil {
		return errors.ErrNilPodMonitor
	}
	obj.Spec.PodTargetLabels = append(obj.Spec.PodTargetLabels, label)
	return nil
}
