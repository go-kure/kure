package kubernetes

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/internal/validation"
)

// CreateHorizontalPodAutoscaler creates a new HPA with the given name and namespace.
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

// SetHPAScaleTargetRef sets the scale target reference for the HPA.
func SetHPAScaleTargetRef(hpa *autoscalingv2.HorizontalPodAutoscaler, apiVersion, kind, name string) error {
	validator := validation.NewValidator()
	if err := validator.ValidateHorizontalPodAutoscaler(hpa); err != nil {
		return err
	}
	hpa.Spec.ScaleTargetRef = autoscalingv2.CrossVersionObjectReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
	}
	return nil
}

// SetHPAMinMaxReplicas sets the minimum and maximum replica counts.
func SetHPAMinMaxReplicas(hpa *autoscalingv2.HorizontalPodAutoscaler, min, max int32) error {
	validator := validation.NewValidator()
	if err := validator.ValidateHorizontalPodAutoscaler(hpa); err != nil {
		return err
	}
	hpa.Spec.MinReplicas = &min
	hpa.Spec.MaxReplicas = max
	return nil
}

// AddHPACPUMetric adds a CPU utilization metric to the HPA.
func AddHPACPUMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, targetUtilization int32) error {
	validator := validation.NewValidator()
	if err := validator.ValidateHorizontalPodAutoscaler(hpa); err != nil {
		return err
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
	return nil
}

// AddHPAMemoryMetric adds a memory utilization metric to the HPA.
func AddHPAMemoryMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, targetUtilization int32) error {
	validator := validation.NewValidator()
	if err := validator.ValidateHorizontalPodAutoscaler(hpa); err != nil {
		return err
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
	return nil
}

// AddHPACustomMetric adds a custom metric to the HPA.
func AddHPACustomMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, metric autoscalingv2.MetricSpec) error {
	validator := validation.NewValidator()
	if err := validator.ValidateHorizontalPodAutoscaler(hpa); err != nil {
		return err
	}
	hpa.Spec.Metrics = append(hpa.Spec.Metrics, metric)
	return nil
}

// SetHPABehavior sets the scaling behavior for the HPA.
func SetHPABehavior(hpa *autoscalingv2.HorizontalPodAutoscaler, behavior *autoscalingv2.HorizontalPodAutoscalerBehavior) error {
	validator := validation.NewValidator()
	if err := validator.ValidateHorizontalPodAutoscaler(hpa); err != nil {
		return err
	}
	hpa.Spec.Behavior = behavior
	return nil
}

// SetHPALabels sets the labels on the HPA.
func SetHPALabels(hpa *autoscalingv2.HorizontalPodAutoscaler, labels map[string]string) error {
	validator := validation.NewValidator()
	if err := validator.ValidateHorizontalPodAutoscaler(hpa); err != nil {
		return err
	}
	hpa.Labels = labels
	return nil
}

// SetHPAAnnotations sets the annotations on the HPA.
func SetHPAAnnotations(hpa *autoscalingv2.HorizontalPodAutoscaler, annotations map[string]string) error {
	validator := validation.NewValidator()
	if err := validator.ValidateHorizontalPodAutoscaler(hpa); err != nil {
		return err
	}
	hpa.Annotations = annotations
	return nil
}
