package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestCreatePod(t *testing.T) {
	pod := CreatePod("test-pod", "default")

	if pod.Name != "test-pod" {
		t.Errorf("expected name %q got %q", "test-pod", pod.Name)
	}
	if pod.Namespace != "default" {
		t.Errorf("expected namespace %q got %q", "default", pod.Namespace)
	}
	if pod.Kind != "Pod" {
		t.Errorf("expected kind Pod got %q", pod.Kind)
	}
	if pod.APIVersion != "v1" {
		t.Errorf("expected apiVersion v1 got %q", pod.APIVersion)
	}
	if pod.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		t.Errorf("unexpected restart policy %v", pod.Spec.RestartPolicy)
	}
	if pod.Spec.TerminationGracePeriodSeconds == nil {
		t.Errorf("expected TerminationGracePeriodSeconds to be set")
	}
}
