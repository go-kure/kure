package nilobj

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIs(t *testing.T) {
	var cm *corev1.ConfigMap
	var u *unstructured.Unstructured
	for name, tc := range map[string]struct {
		obj  client.Object
		want bool
	}{
		"nil interface":            {obj: nil, want: true},
		"typed nil pointer":        {obj: cm, want: true},
		"typed nil unstructured":   {obj: u, want: true},
		"object":                   {obj: &corev1.ConfigMap{}, want: false},
		"unstructured, no content": {obj: &unstructured.Unstructured{}, want: false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := Is(tc.obj); got != tc.want {
				t.Errorf("Is(%#v) = %v, want %v", tc.obj, got, tc.want)
			}
		})
	}
}
