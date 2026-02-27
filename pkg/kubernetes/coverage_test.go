package kubernetes

import (
	stderrors "errors"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/errors"
)

// TestCoverageBoost focuses on improving statement coverage through comprehensive testing

func TestRegisterSchemes_Coverage(t *testing.T) {
	// Reset and test registration from scratch
	// Note: We can't actually reset sync.Once in production code,
	// but we can test that RegisterSchemes works correctly

	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("RegisterSchemes failed: %v", err)
	}

	// Verify global variables are properly initialized
	if Scheme == nil {
		t.Error("Scheme is nil after registration")
	}

	// Test that Codecs was properly initialized
	decoder := Codecs.UniversalDecoder()
	if decoder == nil {
		t.Error("Codecs decoder is nil after registration")
	}
}

func TestScheme_DeepObjectValidation(t *testing.T) {
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test creating objects from scheme
	tests := []struct {
		name string
		gvk  schema.GroupVersionKind
	}{
		{
			name: "Pod from GVK",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		},
		{
			name: "Service from GVK",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
		},
		{
			name: "Deployment from GVK",
			gvk:  schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := Scheme.New(tt.gvk)
			if err != nil {
				t.Fatalf("failed to create object from GVK %v: %v", tt.gvk, err)
			}
			if obj == nil {
				t.Error("created object is nil")
			}

			// Verify ObjectKinds works on the created object
			gvks, _, err := Scheme.ObjectKinds(obj)
			if err != nil {
				t.Errorf("ObjectKinds failed for created object: %v", err)
			}
			if len(gvks) == 0 {
				t.Error("ObjectKinds returned empty slice")
			}
		})
	}
}

func TestGetGroupVersionKind_EdgeCases(t *testing.T) {
	t.Run("multiple calls with same object", func(t *testing.T) {
		pod := &corev1.Pod{}

		// Call multiple times to test caching/consistency
		gvk1, err1 := GetGroupVersionKind(pod)
		if err1 != nil {
			t.Fatalf("first call failed: %v", err1)
		}

		gvk2, err2 := GetGroupVersionKind(pod)
		if err2 != nil {
			t.Fatalf("second call failed: %v", err2)
		}

		if gvk1 != gvk2 {
			t.Errorf("GVK inconsistent between calls: %v != %v", gvk1, gvk2)
		}
	})

	t.Run("object with type metadata set", func(t *testing.T) {
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
		}

		gvk, err := GetGroupVersionKind(pod)
		if err != nil {
			t.Fatalf("failed with TypeMeta set: %v", err)
		}

		if gvk.Kind != "Pod" {
			t.Errorf("expected Kind=Pod, got %s", gvk.Kind)
		}
		if gvk.Version != "v1" {
			t.Errorf("expected Version=v1, got %s", gvk.Version)
		}
	})

	t.Run("object without type metadata", func(t *testing.T) {
		pod := &corev1.Pod{}
		// TypeMeta not set

		gvk, err := GetGroupVersionKind(pod)
		if err != nil {
			t.Fatalf("failed without TypeMeta: %v", err)
		}

		// Should still work because it's registered in scheme
		if gvk.Kind != "Pod" {
			t.Errorf("expected Kind=Pod, got %s", gvk.Kind)
		}
	})
}

func TestGetGroupVersionKind_ErrorScenarios(t *testing.T) {
	t.Run("nil object", func(t *testing.T) {
		gvk, err := GetGroupVersionKind(nil)
		if !stderrors.Is(err, errors.ErrNilObject) {
			t.Errorf("expected ErrNilObject, got %v", err)
		}
		if gvk != (schema.GroupVersionKind{}) {
			t.Errorf("expected empty GVK, got %v", gvk)
		}
	})

	t.Run("unregistered type documented", func(t *testing.T) {
		// Note: Testing unregistered types requires implementing DeepCopyObject
		// which is complex. The behavior is tested via TestGetGroupVersionKind_UnknownObject
		// in utilities_test.go

		// GetGroupVersionKind should return an error for unregistered types
		t.Log("Unregistered type behavior tested in utilities_test.go")
	})
}

func TestIsGVKAllowed_Comprehensive(t *testing.T) {
	t.Run("nil allowed list", func(t *testing.T) {
		gvk := schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		}

		result := IsGVKAllowed(gvk, nil)
		if result {
			t.Error("expected false for nil allowed list")
		}
	})

	t.Run("empty group", func(t *testing.T) {
		gvk := schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}

		allowed := []schema.GroupVersionKind{
			{Group: "", Version: "v1", Kind: "Pod"},
		}

		result := IsGVKAllowed(gvk, allowed)
		if !result {
			t.Error("expected true for core group Pod")
		}
	})

	t.Run("case sensitivity", func(t *testing.T) {
		gvk := schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		}

		// Test that matching is case-sensitive
		allowed := []schema.GroupVersionKind{
			{Group: "apps", Version: "v1", Kind: "deployment"}, // lowercase kind
		}

		result := IsGVKAllowed(gvk, allowed)
		if result {
			t.Error("expected false - kind should be case-sensitive")
		}
	})

	t.Run("partial match not allowed", func(t *testing.T) {
		gvk := schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		}

		allowed := []schema.GroupVersionKind{
			{Group: "apps", Version: "v1", Kind: "StatefulSet"},
			{Group: "apps", Version: "v1beta1", Kind: "Deployment"},
		}

		result := IsGVKAllowed(gvk, allowed)
		if result {
			t.Error("expected false - no exact match")
		}
	})

	t.Run("multiple allowed entries", func(t *testing.T) {
		gvk := schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "DaemonSet",
		}

		allowed := []schema.GroupVersionKind{
			{Group: "apps", Version: "v1", Kind: "Deployment"},
			{Group: "apps", Version: "v1", Kind: "StatefulSet"},
			{Group: "apps", Version: "v1", Kind: "DaemonSet"},
			{Group: "apps", Version: "v1", Kind: "ReplicaSet"},
		}

		result := IsGVKAllowed(gvk, allowed)
		if !result {
			t.Error("expected true - DaemonSet is in the allowed list")
		}
	})
}

func TestToClientObject_EdgeCases(t *testing.T) {
	t.Run("nil fields in object", func(t *testing.T) {
		pod := &corev1.Pod{
			// All fields at default values
		}

		clientObjPtr := ToClientObject(pod)
		if clientObjPtr == nil {
			t.Fatal("expected non-nil pointer")
		}

		clientObj := *clientObjPtr
		if clientObj == nil {
			t.Error("expected non-nil client object")
		}
	})

	t.Run("fully populated object", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test",
				},
				Annotations: map[string]string{
					"annotation": "value",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "nginx:latest",
					},
				},
			},
		}

		clientObjPtr := ToClientObject(pod)
		if clientObjPtr == nil {
			t.Fatal("expected non-nil pointer")
		}

		clientObj := *clientObjPtr
		if clientObj == nil {
			t.Error("expected non-nil client object")
		}

		// Verify data integrity
		if clientObj.GetName() != "test" {
			t.Errorf("expected name 'test', got %s", clientObj.GetName())
		}
		if clientObj.GetNamespace() != "default" {
			t.Errorf("expected namespace 'default', got %s", clientObj.GetNamespace())
		}
	})
}

func TestValidatePackageRef_EdgeCases(t *testing.T) {
	t.Run("exact GitRepository match", func(t *testing.T) {
		gvk := &schema.GroupVersionKind{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1beta1",
			Kind:    "GitRepository",
		}

		err := ValidatePackageRef(gvk)
		if err != nil {
			t.Errorf("GitRepository should be valid: %v", err)
		}
	})

	t.Run("exact OCIRepository match", func(t *testing.T) {
		gvk := &schema.GroupVersionKind{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1beta1",
			Kind:    "OCIRepository",
		}

		err := ValidatePackageRef(gvk)
		if err != nil {
			t.Errorf("OCIRepository should be valid: %v", err)
		}
	})

	t.Run("wrong group", func(t *testing.T) {
		gvk := &schema.GroupVersionKind{
			Group:   "helm.toolkit.fluxcd.io",
			Version: "v1beta1",
			Kind:    "GitRepository",
		}

		err := ValidatePackageRef(gvk)
		if !stderrors.Is(err, errors.ErrGVKNotAllowed) {
			t.Errorf("expected ErrGVKNotAllowed, got %v", err)
		}
	})

	t.Run("wrong version", func(t *testing.T) {
		gvk := &schema.GroupVersionKind{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "GitRepository",
		}

		err := ValidatePackageRef(gvk)
		if !stderrors.Is(err, errors.ErrGVKNotAllowed) {
			t.Errorf("expected ErrGVKNotAllowed, got %v", err)
		}
	})

	t.Run("empty fields", func(t *testing.T) {
		gvk := &schema.GroupVersionKind{}

		err := ValidatePackageRef(gvk)
		if !stderrors.Is(err, errors.ErrGVKNotAllowed) {
			t.Errorf("expected ErrGVKNotAllowed for empty GVK, got %v", err)
		}
	})
}

func TestConcurrentOperations(t *testing.T) {
	// Test that all functions are safe for concurrent use
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 4) // 4 operations per goroutine

	for i := 0; i < goroutines; i++ {
		// Test GetGroupVersionKind concurrency
		go func() {
			defer wg.Done()
			pod := &corev1.Pod{}
			_, _ = GetGroupVersionKind(pod)
		}()

		// Test IsGVKAllowed concurrency
		go func() {
			defer wg.Done()
			gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
			allowed := []schema.GroupVersionKind{gvk}
			_ = IsGVKAllowed(gvk, allowed)
		}()

		// Test ToClientObject concurrency
		go func() {
			defer wg.Done()
			pod := &corev1.Pod{}
			_ = ToClientObject(pod)
		}()

		// Test ValidatePackageRef concurrency
		go func() {
			defer wg.Done()
			gvk := &schema.GroupVersionKind{
				Group:   "source.toolkit.fluxcd.io",
				Version: "v1beta1",
				Kind:    "GitRepository",
			}
			_ = ValidatePackageRef(gvk)
		}()
	}

	wg.Wait()
}

func TestSchemeGlobals(t *testing.T) {
	// Ensure global variables are properly initialized
	if Scheme == nil {
		t.Fatal("global Scheme is nil")
	}

	// Test that Codecs is properly initialized
	decoder := Codecs.UniversalDecoder()
	if decoder == nil {
		t.Fatal("Codecs UniversalDecoder is nil")
	}

	encoder := Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	if encoder == nil {
		t.Fatal("Codecs LegacyCodec is nil")
	}
}
