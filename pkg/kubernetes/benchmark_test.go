package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Benchmark tests also contribute to coverage

func BenchmarkRegisterSchemes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = RegisterSchemes()
	}
}

func BenchmarkGetGroupVersionKind(b *testing.B) {
	err := RegisterSchemes()
	if err != nil {
		b.Fatalf("failed to register schemes: %v", err)
	}

	pod := &corev1.Pod{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetGroupVersionKind(pod)
	}
}

func BenchmarkIsGVKAllowed(b *testing.B) {
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	allowed := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Pod"},
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "", Version: "v1", Kind: "Service"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsGVKAllowed(gvk, allowed)
	}
}

func BenchmarkToClientObject(b *testing.B) {
	pod := &corev1.Pod{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ToClientObject(pod)
	}
}

func BenchmarkValidatePackageRef(b *testing.B) {
	gvk := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1beta1",
		Kind:    "GitRepository",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidatePackageRef(gvk)
	}
}
