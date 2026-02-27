package kubernetes

import (
	stderrors "errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetGroupVersionKind_Success(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Object
		expectedGVK schema.GroupVersionKind
	}{
		{
			name: "Pod",
			obj:  &corev1.Pod{},
			expectedGVK: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
		},
		{
			name: "Service",
			obj:  &corev1.Service{},
			expectedGVK: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Service",
			},
		},
		{
			name: "Deployment",
			obj:  &appsv1.Deployment{},
			expectedGVK: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
		},
		{
			name: "ConfigMap",
			obj:  &corev1.ConfigMap{},
			expectedGVK: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "ConfigMap",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvk, err := GetGroupVersionKind(tt.obj)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if gvk != tt.expectedGVK {
				t.Errorf("expected GVK %v, got %v", tt.expectedGVK, gvk)
			}
		})
	}
}

func TestGetGroupVersionKind_NilObject(t *testing.T) {
	gvk, err := GetGroupVersionKind(nil)

	if !stderrors.Is(err, errors.ErrNilObject) {
		t.Errorf("expected ErrNilObject, got: %v", err)
	}

	expectedGVK := schema.GroupVersionKind{}
	if gvk != expectedGVK {
		t.Errorf("expected empty GVK for nil object, got: %v", gvk)
	}
}

// UnknownObject is a test type that's not registered in the scheme
type UnknownObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// Implement runtime.Object interface
func (u *UnknownObject) DeepCopyObject() runtime.Object {
	if u == nil {
		return nil
	}
	out := new(UnknownObject)
	u.DeepCopyInto(out)
	return out
}

func (u *UnknownObject) DeepCopyInto(out *UnknownObject) {
	*out = *u
	u.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

func TestGetGroupVersionKind_UnknownObject(t *testing.T) {
	unknownObj := &UnknownObject{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test/v1",
			Kind:       "Unknown",
		},
	}

	gvk, err := GetGroupVersionKind(unknownObj)

	// Should return an error because the type is not registered
	if err == nil {
		t.Error("expected error for unknown object type, got nil")
	}

	expectedGVK := schema.GroupVersionKind{}
	if gvk != expectedGVK {
		t.Errorf("expected empty GVK for unknown object, got: %v", gvk)
	}
}

func TestGetGroupVersionKind_ErrorPaths(t *testing.T) {
	t.Run("ObjectKinds returns empty slice", func(t *testing.T) {
		// This test verifies the error path when ObjectKinds returns an empty slice
		// While hard to trigger with registered types, we test with an object
		// that has incomplete type information

		// Create an object without proper scheme registration
		unknownObj := &UnknownObject{}

		gvk, err := GetGroupVersionKind(unknownObj)

		// We expect either ErrGVKNotFound or another scheme-related error
		if err == nil {
			t.Error("expected error for object without GVK, got nil")
		}

		expectedGVK := schema.GroupVersionKind{}
		if gvk != expectedGVK {
			t.Errorf("expected empty GVK on error, got: %v", gvk)
		}
	})
}

func TestIsGVKAllowed_Allowed(t *testing.T) {
	testGVK := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	allowedGVKs := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Pod"},
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "", Version: "v1", Kind: "Service"},
	}

	result := IsGVKAllowed(testGVK, allowedGVKs)

	if !result {
		t.Error("expected GVK to be allowed, got false")
	}
}

func TestIsGVKAllowed_NotAllowed(t *testing.T) {
	testGVK := schema.GroupVersionKind{
		Group:   "networking.k8s.io",
		Version: "v1",
		Kind:    "Ingress",
	}

	allowedGVKs := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Pod"},
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "", Version: "v1", Kind: "Service"},
	}

	result := IsGVKAllowed(testGVK, allowedGVKs)

	if result {
		t.Error("expected GVK to not be allowed, got true")
	}
}

func TestIsGVKAllowed_EmptyAllowedList(t *testing.T) {
	testGVK := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	allowedGVKs := []schema.GroupVersionKind{}

	result := IsGVKAllowed(testGVK, allowedGVKs)

	if result {
		t.Error("expected GVK to not be allowed with empty list, got true")
	}
}

func TestIsGVKAllowed_ExactMatch(t *testing.T) {
	testGVK := schema.GroupVersionKind{
		Group:   "batch",
		Version: "v1",
		Kind:    "Job",
	}

	allowedGVKs := []schema.GroupVersionKind{
		{Group: "batch", Version: "v1", Kind: "Job"},
	}

	result := IsGVKAllowed(testGVK, allowedGVKs)

	if !result {
		t.Error("expected exact match GVK to be allowed, got false")
	}
}

func TestIsGVKAllowed_VersionMismatch(t *testing.T) {
	testGVK := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	allowedGVKs := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1beta1", Kind: "Deployment"}, // Different version
	}

	result := IsGVKAllowed(testGVK, allowedGVKs)

	if result {
		t.Error("expected version mismatch to not be allowed, got true")
	}
}

func TestToClientObject_ValidObject(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	clientObjPtr := ToClientObject(pod)

	if clientObjPtr == nil {
		t.Fatal("expected non-nil client object pointer")
	}

	clientObj := *clientObjPtr
	if clientObj == nil {
		t.Error("expected non-nil client object")
	}

	// Test that it's still the same object
	podFromClient, ok := clientObj.(*corev1.Pod)
	if !ok {
		t.Error("expected client object to be a Pod")
	}

	if podFromClient.Name != "test-pod" {
		t.Errorf("expected pod name 'test-pod', got %s", podFromClient.Name)
	}
}

func TestToClientObject_DifferentTypes(t *testing.T) {
	objects := []client.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test-service"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "test-deployment"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"}},
	}

	for _, obj := range objects {
		t.Run(obj.GetObjectKind().GroupVersionKind().Kind, func(t *testing.T) {
			clientObjPtr := ToClientObject(obj)

			if clientObjPtr == nil {
				t.Fatal("expected non-nil client object pointer")
			}

			clientObj := *clientObjPtr
			if clientObj == nil {
				t.Error("expected non-nil client object")
			}

			// Verify the object retains its original name
			if clientObj.GetName() != obj.GetName() {
				t.Errorf("expected name %s, got %s", obj.GetName(), clientObj.GetName())
			}
		})
	}
}

func TestValidatePackageRef_GitRepository(t *testing.T) {
	validGitRepo := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1beta1",
		Kind:    "GitRepository",
	}

	err := ValidatePackageRef(validGitRepo)
	if err != nil {
		t.Errorf("expected GitRepository to be valid, got error: %v", err)
	}
}

func TestValidatePackageRef_OCIRepository(t *testing.T) {
	validOCIRepo := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1beta1",
		Kind:    "OCIRepository",
	}

	err := ValidatePackageRef(validOCIRepo)
	if err != nil {
		t.Errorf("expected OCIRepository to be valid, got error: %v", err)
	}
}

func TestValidatePackageRef_InvalidGVK(t *testing.T) {
	tests := []struct {
		name string
		gvk  *schema.GroupVersionKind
	}{
		{
			name: "wrong group",
			gvk: &schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
		},
		{
			name: "wrong version",
			gvk: &schema.GroupVersionKind{
				Group:   "source.toolkit.fluxcd.io",
				Version: "v1",
				Kind:    "GitRepository",
			},
		},
		{
			name: "wrong kind",
			gvk: &schema.GroupVersionKind{
				Group:   "source.toolkit.fluxcd.io",
				Version: "v1beta1",
				Kind:    "HelmRepository",
			},
		},
		{
			name: "completely different",
			gvk: &schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePackageRef(tt.gvk)
			if !stderrors.Is(err, errors.ErrGVKNotAllowed) {
				t.Errorf("expected ErrGVKNotAllowed, got: %v", err)
			}
		})
	}
}

func TestValidatePackageRef_NilGVK(t *testing.T) {
	// Test behavior with nil GVK (this will likely panic or have undefined behavior)
	// but we should test it to understand the function's behavior
	defer func() {
		if r := recover(); r != nil {
			// This is expected behavior - function dereferences the pointer
			t.Log("Function panicked with nil GVK as expected")
		}
	}()

	err := ValidatePackageRef(nil)
	if err == nil {
		t.Error("expected error with nil GVK")
	}
}

func TestValidatePackageRef_AllowedList(t *testing.T) {
	// Test that the allowed list contains exactly what we expect
	allowedGVKs := []schema.GroupVersionKind{
		{Group: "source.toolkit.fluxcd.io", Version: "v1beta1", Kind: "GitRepository"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1beta1", Kind: "OCIRepository"},
	}

	// Test each allowed GVK
	for _, gvk := range allowedGVKs {
		t.Run(gvk.Kind, func(t *testing.T) {
			err := ValidatePackageRef(&gvk)
			if err != nil {
				t.Errorf("expected %s to be allowed, got error: %v", gvk.Kind, err)
			}
		})
	}
}

func TestUtilities_Integration(t *testing.T) {
	// Test a complete workflow: create object -> get GVK -> validate allowed -> convert to client object
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	// Get GVK
	gvk, err := GetGroupVersionKind(pod)
	if err != nil {
		t.Fatalf("failed to get GVK: %v", err)
	}

	// Check if allowed in a custom list
	allowedGVKs := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Pod"},
		{Group: "apps", Version: "v1", Kind: "Deployment"},
	}

	if !IsGVKAllowed(gvk, allowedGVKs) {
		t.Error("expected Pod GVK to be allowed")
	}

	// Convert to client object
	clientObjPtr := ToClientObject(pod)
	if clientObjPtr == nil {
		t.Fatal("expected non-nil client object pointer")
	}

	clientObj := *clientObjPtr
	if clientObj.GetName() != "test-pod" {
		t.Errorf("expected client object name 'test-pod', got %s", clientObj.GetName())
	}
}
