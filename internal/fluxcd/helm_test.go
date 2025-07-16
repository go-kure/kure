package fluxcd

import (
	"testing"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

func TestHelmReleaseHelpers(t *testing.T) {
	hr := CreateHelmRelease("demo", "ns", helmv2.HelmReleaseSpec{})
	AddHelmReleaseLabel(hr, "app", "demo")
	AddHelmReleaseAnnotation(hr, "team", "dev")
	SetHelmReleaseReleaseName(hr, "demo")
	SetHelmReleaseTargetNamespace(hr, "target")
	SetHelmReleaseStorageNamespace(hr, "storage")
	SetHelmReleaseInterval(hr, metav1.Duration{Duration: time.Minute})
	SetHelmReleaseTimeout(hr, metav1.Duration{Duration: time.Minute})
	SetHelmReleaseMaxHistory(hr, 2)
	SetHelmReleaseServiceAccountName(hr, "sa")
	SetHelmReleasePersistentClient(hr, true)
	SetHelmReleaseSuspend(hr, true)
	SetHelmReleaseKubeConfig(hr, &meta.KubeConfigReference{SecretRef: meta.SecretKeyReference{Name: "k"}})
	AddHelmReleaseDependsOn(hr, meta.NamespacedObjectReference{Name: "dep"})
	SetHelmReleaseValues(hr, &apiextensionsv1.JSON{Raw: []byte("{}")})
	AddHelmReleaseValuesFrom(hr, helmv2.ValuesReference{Kind: "ConfigMap", Name: "vals"})
	AddHelmReleasePostRenderer(hr, helmv2.PostRenderer{})

	if hr.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}
	if hr.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}
	if hr.Spec.ReleaseName != "demo" {
		t.Errorf("release name not set")
	}
	if hr.Spec.TargetNamespace != "target" {
		t.Errorf("target namespace not set")
	}
	if hr.Spec.StorageNamespace != "storage" {
		t.Errorf("storage namespace not set")
	}
	if hr.Spec.Interval.Duration != time.Minute {
		t.Errorf("interval not set")
	}
	if hr.Spec.Timeout == nil || hr.Spec.Timeout.Duration != time.Minute {
		t.Errorf("timeout not set")
	}
	if hr.Spec.MaxHistory == nil || *hr.Spec.MaxHistory != 2 {
		t.Errorf("maxHistory not set")
	}
	if hr.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account not set")
	}
	if hr.Spec.PersistentClient == nil || !*hr.Spec.PersistentClient {
		t.Errorf("persistent client not set")
	}
	if !hr.Spec.Suspend {
		t.Errorf("suspend not set")
	}
	if hr.Spec.KubeConfig == nil || hr.Spec.KubeConfig.SecretRef.Name != "k" {
		t.Errorf("kubeconfig not set")
	}
	if len(hr.Spec.DependsOn) != 1 || hr.Spec.DependsOn[0].Name != "dep" {
		t.Errorf("dependsOn not added")
	}
	if hr.Spec.Values == nil {
		t.Errorf("values not set")
	}
	if len(hr.Spec.ValuesFrom) != 1 || hr.Spec.ValuesFrom[0].Name != "vals" {
		t.Errorf("valuesFrom not added")
	}
	if len(hr.Spec.PostRenderers) != 1 {
		t.Errorf("postRenderer not added")
	}
}
