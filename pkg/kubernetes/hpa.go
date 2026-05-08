package kubernetes

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateHorizontalPodAutoscaler creates a new HorizontalPodAutoscaler with the
// given name and namespace. The returned object has TypeMeta, labels, and
// annotations pre-populated so it can be serialized to YAML immediately.
func CreateHorizontalPodAutoscaler(name, namespace string) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: autoscalingv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
	}
}

// SetHPAScaleTargetRef sets the scale target reference for the HPA, identifying
// the resource (e.g. Deployment) that the autoscaler controls.
func SetHPAScaleTargetRef(hpa *autoscalingv2.HorizontalPodAutoscaler, apiVersion, kind, name string) {
	if hpa == nil {
		panic("SetHPAScaleTargetRef: hpa must not be nil")
	}
	hpa.Spec.ScaleTargetRef = autoscalingv2.CrossVersionObjectReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
	}
}

// SetHPAMinMaxReplicas sets the minimum and maximum replica counts for the HPA.
func SetHPAMinMaxReplicas(hpa *autoscalingv2.HorizontalPodAutoscaler, min, max int32) {
	if hpa == nil {
		panic("SetHPAMinMaxReplicas: hpa must not be nil")
	}
	hpa.Spec.MinReplicas = &min
	hpa.Spec.MaxReplicas = max
}

// AddHPACPUMetric adds a CPU utilization metric to the HPA. The
// targetUtilization is a percentage (e.g. 80 means 80%).
func AddHPACPUMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, targetUtilization int32) {
	if hpa == nil {
		panic("AddHPACPUMetric: hpa must not be nil")
	}
	hpa.Spec.Metrics = append(hpa.Spec.Metrics, autoscalingv2.MetricSpec{
		Type: autoscalingv2.ResourceMetricSourceType,
		Resource: &autoscalingv2.ResourceMetricSource{
			Name: corev1.ResourceCPU,
			Target: autoscalingv2.MetricTarget{
				Type:               autoscalingv2.UtilizationMetricType,
				AverageUtilization: &targetUtilization,
			},
		},
	})
}

// AddHPAMemoryMetric adds a memory utilization metric to the HPA. The
// targetUtilization is a percentage (e.g. 70 means 70%).
func AddHPAMemoryMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, targetUtilization int32) {
	if hpa == nil {
		panic("AddHPAMemoryMetric: hpa must not be nil")
	}
	hpa.Spec.Metrics = append(hpa.Spec.Metrics, autoscalingv2.MetricSpec{
		Type: autoscalingv2.ResourceMetricSourceType,
		Resource: &autoscalingv2.ResourceMetricSource{
			Name: corev1.ResourceMemory,
			Target: autoscalingv2.MetricTarget{
				Type:               autoscalingv2.UtilizationMetricType,
				AverageUtilization: &targetUtilization,
			},
		},
	})
}

// AddHPACustomMetric adds a caller-defined MetricSpec to the HPA. Use this for
// pod metrics, object metrics, or external metrics that are not covered by the
// built-in CPU and memory helpers.
func AddHPACustomMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, metric autoscalingv2.MetricSpec) {
	if hpa == nil {
		panic("AddHPACustomMetric: hpa must not be nil")
	}
	hpa.Spec.Metrics = append(hpa.Spec.Metrics, metric)
}

// SetHPABehavior sets the scaling behavior for the HPA, controlling scale-up
// and scale-down stabilization windows and policies.
func SetHPABehavior(hpa *autoscalingv2.HorizontalPodAutoscaler, behavior *autoscalingv2.HorizontalPodAutoscalerBehavior) {
	if hpa == nil {
		panic("SetHPABehavior: hpa must not be nil")
	}
	hpa.Spec.Behavior = behavior
}

// SetHPALabels replaces the labels on the HPA with the provided map.
func SetHPALabels(hpa *autoscalingv2.HorizontalPodAutoscaler, labels map[string]string) {
	if hpa == nil {
		panic("SetHPALabels: hpa must not be nil")
	}
	hpa.Labels = labels
}

// SetHPAAnnotations replaces the annotations on the HPA with the provided map.
func SetHPAAnnotations(hpa *autoscalingv2.HorizontalPodAutoscaler, annotations map[string]string) {
	if hpa == nil {
		panic("SetHPAAnnotations: hpa must not be nil")
	}
	hpa.Annotations = annotations
}
