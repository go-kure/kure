package kubernetes

import (
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

func TestCreateHorizontalPodAutoscaler(t *testing.T) {
	hpa := CreateHorizontalPodAutoscaler("my-hpa", "default")
	if hpa.Name != "my-hpa" || hpa.Namespace != "default" {
		t.Fatalf("metadata mismatch: %s/%s", hpa.Namespace, hpa.Name)
	}
	if hpa.Kind != "HorizontalPodAutoscaler" {
		t.Errorf("unexpected kind %q", hpa.Kind)
	}
	if hpa.Labels["app"] != "my-hpa" {
		t.Errorf("expected label app=my-hpa, got %v", hpa.Labels)
	}
}

func TestHPANilErrors(t *testing.T) {
	if err := SetHPAScaleTargetRef(nil, "apps/v1", "Deployment", "web"); err == nil {
		t.Error("expected error for nil HPA")
	}
	if err := SetHPAMinMaxReplicas(nil, 1, 10); err == nil {
		t.Error("expected error for nil HPA")
	}
	if err := AddHPACPUMetric(nil, 80); err == nil {
		t.Error("expected error for nil HPA")
	}
	if err := AddHPAMemoryMetric(nil, 80); err == nil {
		t.Error("expected error for nil HPA")
	}
	if err := AddHPACustomMetric(nil, autoscalingv2.MetricSpec{}); err == nil {
		t.Error("expected error for nil HPA")
	}
	if err := SetHPABehavior(nil, &autoscalingv2.HorizontalPodAutoscalerBehavior{}); err == nil {
		t.Error("expected error for nil HPA")
	}
	if err := SetHPALabels(nil, map[string]string{}); err == nil {
		t.Error("expected error for nil HPA")
	}
	if err := SetHPAAnnotations(nil, map[string]string{}); err == nil {
		t.Error("expected error for nil HPA")
	}
}

func TestHPAScaleTargetRef(t *testing.T) {
	hpa := CreateHorizontalPodAutoscaler("test", "default")
	if err := SetHPAScaleTargetRef(hpa, "apps/v1", "Deployment", "web"); err != nil {
		t.Fatalf("SetHPAScaleTargetRef: %v", err)
	}
	ref := hpa.Spec.ScaleTargetRef
	if ref.APIVersion != "apps/v1" || ref.Kind != "Deployment" || ref.Name != "web" {
		t.Errorf("scale target ref mismatch: %+v", ref)
	}
}

func TestHPAMinMaxReplicas(t *testing.T) {
	hpa := CreateHorizontalPodAutoscaler("test", "default")
	if err := SetHPAMinMaxReplicas(hpa, 2, 10); err != nil {
		t.Fatalf("SetHPAMinMaxReplicas: %v", err)
	}
	if hpa.Spec.MinReplicas == nil || *hpa.Spec.MinReplicas != 2 {
		t.Errorf("min replicas not set")
	}
	if hpa.Spec.MaxReplicas != 10 {
		t.Errorf("max replicas not set")
	}
}

func TestHPAMetrics(t *testing.T) {
	hpa := CreateHorizontalPodAutoscaler("test", "default")

	if err := AddHPACPUMetric(hpa, 80); err != nil {
		t.Fatalf("AddHPACPUMetric: %v", err)
	}
	if len(hpa.Spec.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(hpa.Spec.Metrics))
	}
	if hpa.Spec.Metrics[0].Resource.Target.AverageUtilization == nil || *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization != 80 {
		t.Errorf("CPU metric not set correctly")
	}

	if err := AddHPAMemoryMetric(hpa, 70); err != nil {
		t.Fatalf("AddHPAMemoryMetric: %v", err)
	}
	if len(hpa.Spec.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(hpa.Spec.Metrics))
	}

	custom := autoscalingv2.MetricSpec{
		Type: autoscalingv2.PodsMetricSourceType,
	}
	if err := AddHPACustomMetric(hpa, custom); err != nil {
		t.Fatalf("AddHPACustomMetric: %v", err)
	}
	if len(hpa.Spec.Metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(hpa.Spec.Metrics))
	}
}

func TestHPABehavior(t *testing.T) {
	hpa := CreateHorizontalPodAutoscaler("test", "default")
	behavior := &autoscalingv2.HorizontalPodAutoscalerBehavior{
		ScaleDown: &autoscalingv2.HPAScalingRules{
			StabilizationWindowSeconds: func(i int32) *int32 { return &i }(300),
		},
	}
	if err := SetHPABehavior(hpa, behavior); err != nil {
		t.Fatalf("SetHPABehavior: %v", err)
	}
	if hpa.Spec.Behavior == nil || hpa.Spec.Behavior.ScaleDown == nil {
		t.Errorf("behavior not set correctly")
	}
}

func TestHPALabelsAndAnnotations(t *testing.T) {
	hpa := CreateHorizontalPodAutoscaler("test", "default")

	labels := map[string]string{"env": "prod"}
	if err := SetHPALabels(hpa, labels); err != nil {
		t.Fatalf("SetHPALabels: %v", err)
	}
	if hpa.Labels["env"] != "prod" {
		t.Errorf("labels not set correctly")
	}

	annotations := map[string]string{"note": "test"}
	if err := SetHPAAnnotations(hpa, annotations); err != nil {
		t.Fatalf("SetHPAAnnotations: %v", err)
	}
	if hpa.Annotations["note"] != "test" {
		t.Errorf("annotations not set correctly")
	}
}
