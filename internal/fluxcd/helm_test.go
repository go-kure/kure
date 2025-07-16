package fluxcd

import (
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateHelmRelease(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		spec      helmv2.HelmReleaseSpec
		expected  *helmv2.HelmRelease
	}{
		{
			name:      "ValidInput",
			namespace: "default",
			spec: helmv2.HelmReleaseSpec{
				Chart: &helmv2.HelmChartTemplate{
					ObjectMeta: &helmv2.HelmChartTemplateObjectMeta{
						Labels:      nil,
						Annotations: nil,
					},
					Spec: helmv2.HelmChartTemplateSpec{
						Chart:   "example-chart",
						Version: "1.0.0",
					},
				},
			},
			expected: &helmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRelease",
					APIVersion: helmv2.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ValidInput",
					Namespace: "default",
				},
				Spec: helmv2.HelmReleaseSpec{
					Chart: &helmv2.HelmChartTemplate{
						Spec: helmv2.HelmChartTemplateSpec{
							Chart:   "example-chart",
							Version: "1.0.0",
						},
					},
				},
			},
		},
		{
			name:      "CustomNamespace",
			namespace: "custom-namespace",
			spec: helmv2.HelmReleaseSpec{
				Chart: &helmv2.HelmChartTemplate{
					Spec: helmv2.HelmChartTemplateSpec{
						Chart:   "custom-chart",
						Version: "2.1.0",
					},
				},
			},
			expected: &helmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRelease",
					APIVersion: helmv2.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "CustomNamespace",
					Namespace: "custom-namespace",
				},
				Spec: helmv2.HelmReleaseSpec{
					Chart: &helmv2.HelmChartTemplate{
						Spec: helmv2.HelmChartTemplateSpec{
							Chart:   "custom-chart",
							Version: "2.1.0",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateHelmRelease(tt.name, tt.namespace, tt.spec)
			if result.TypeMeta != tt.expected.TypeMeta {
				t.Errorf("TypeMeta mismatch: got %v, want %v", result.TypeMeta, tt.expected.TypeMeta)
			}
			if result.ObjectMeta.Name != tt.expected.ObjectMeta.Name {
				t.Errorf("Name mismatch: got %v, want %v", result.ObjectMeta.Name, tt.expected.ObjectMeta.Name)
			}
			if result.ObjectMeta.Namespace != tt.expected.ObjectMeta.Namespace {
				t.Errorf("Namespace mismatch: got %v, want %v", result.ObjectMeta.Namespace, tt.expected.ObjectMeta.Namespace)
			}
			if result.Spec.Chart.Spec.Chart != tt.expected.Spec.Chart.Spec.Chart ||
				result.Spec.Chart.Spec.Version != tt.expected.Spec.Chart.Spec.Version {
				t.Errorf("Spec mismatch: got %+v, want %+v", result.Spec, tt.expected.Spec)
			}
		})
	}
}
