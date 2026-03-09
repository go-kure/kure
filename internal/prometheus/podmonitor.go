package prometheus

import (
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
func AddPodMonitorEndpoint(obj *monitoringv1.PodMonitor, ep monitoringv1.PodMetricsEndpoint) {
	obj.Spec.PodMetricsEndpoints = append(obj.Spec.PodMetricsEndpoints, ep)
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

// AddPodMonitorPodTargetLabel appends a pod target label.
func AddPodMonitorPodTargetLabel(obj *monitoringv1.PodMonitor, label string) {
	obj.Spec.PodTargetLabels = append(obj.Spec.PodTargetLabels, label)
}
