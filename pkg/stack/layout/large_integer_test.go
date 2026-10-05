package layout_test

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestWriters_KeepLargeIntegers: every writer writes an integer above 2^53 as
// the integer it is, for a typed and an unstructured object.
func TestWriters_KeepLargeIntegers(t *testing.T) {
	// 2^53 + 1, the first integer a float64 cannot hold.
	const large = int64(9007199254740993)
	const want = ": 9007199254740993\n"

	grace := large
	typed := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "typed", Namespace: "default"},
	}
	typed.Spec.Template.Spec.TerminationGracePeriodSeconds = &grace

	loose := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "loose", "namespace": "default"},
		"spec":       map[string]any{"serial": large},
	}}

	for _, writer := range allWriters {
		t.Run(writer, func(t *testing.T) {
			ml := &layout.ManifestLayout{
				Name:      "p",
				Namespace: ".",
				Resources: []client.Object{typed.DeepCopy(), loose.DeepCopy()},
			}
			files := writtenFiles(t, writer, layout.DefaultLayoutConfig(), ml)
			for _, name := range []string{"p/default-deployment-typed.yaml", "p/default-widget-loose.yaml"} {
				content, ok := files[name]
				if !ok {
					t.Fatalf("%s is not written; files: %v", name, files)
				}
				if !strings.Contains(content, want) {
					t.Errorf("%s does not hold %s:\n%s", name, want, content)
				}
			}
		})
	}
}
