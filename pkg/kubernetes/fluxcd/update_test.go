package fluxcd

import (
	"slices"
	"testing"
	"time"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFluxInstanceHelpers(t *testing.T) {
	instance := CreateFluxInstance("flux-instance", "flux-system")

	component := fluxv1.Component("source-controller")
	AddFluxInstanceComponent(instance, component)

	if !slices.Contains(instance.Spec.Components, component) {
		t.Errorf("expected component %q to be added", component)
	}

	SetFluxInstanceWait(instance, true)

	if instance.Spec.Wait == nil || !*instance.Spec.Wait {
		t.Error("expected Wait to be true")
	}
}

func TestFluxReportHelpers(t *testing.T) {
	report := CreateFluxReport("flux-report", "flux-system")

	componentStatus := fluxv1.FluxComponentStatus{
		Name:   "kustomize-controller",
		Status: "running",
	}
	AddFluxReportComponentStatus(report, componentStatus)
	if !slices.Contains(report.Spec.ComponentsStatus, componentStatus) {
		t.Errorf("expected component status %q to be added", componentStatus.Name)
	}
}

func TestResourceSetHelpers(t *testing.T) {
	resourceSet := CreateResourceSet("test-resourceset", "flux-system")

	input := fluxv1.ResourceSetInput{
		"test-input": &apiextensionsv1.JSON{Raw: []byte(`"value"`)},
	}
	AddResourceSetInput(resourceSet, input)
	if len(resourceSet.Spec.Inputs) != 1 {
		t.Fatalf("expected 1 input set, got %d", len(resourceSet.Spec.Inputs))
	}
	gotInput, ok := resourceSet.Spec.Inputs[0]["test-input"]
	if !ok || gotInput == nil {
		t.Fatal("expected input 'test-input' to be added")
	}
	if string(gotInput.Raw) != `"value"` {
		t.Errorf("expected input raw value %q, got %q", `"value"`, string(gotInput.Raw))
	}

	inputRef := fluxv1.InputProviderReference{
		Name: "input-provider",
	}
	AddResourceSetInputFrom(resourceSet, inputRef)
	if len(resourceSet.Spec.InputsFrom) != 1 {
		t.Fatalf("expected 1 input reference, got %d", len(resourceSet.Spec.InputsFrom))
	}
	if resourceSet.Spec.InputsFrom[0].Name != "input-provider" {
		t.Errorf("expected input reference name %q, got %q", "input-provider", resourceSet.Spec.InputsFrom[0].Name)
	}
}

func TestProviderHelpers(t *testing.T) {
	provider := CreateProvider("slack-provider", "flux-system")

	interval := metav1.Duration{Duration: 10 * time.Minute}
	SetProviderInterval(provider, interval)
	if provider.Spec.Interval.Duration != 10*time.Minute {
		t.Errorf("expected interval 10m, got %v", provider.Spec.Interval.Duration)
	}

	timeout := metav1.Duration{Duration: 30 * time.Second}
	SetProviderTimeout(provider, timeout)
	if provider.Spec.Timeout.Duration != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", provider.Spec.Timeout.Duration)
	}

	secretRef := &meta.LocalObjectReference{Name: "discord-secret"}
	SetProviderSecretRef(provider, secretRef)
	if provider.Spec.SecretRef == nil || provider.Spec.SecretRef.Name != "discord-secret" {
		t.Error("expected SecretRef.Name 'discord-secret'")
	}

	certSecretRef := &meta.LocalObjectReference{Name: "cert-secret"}
	SetProviderCertSecretRef(provider, certSecretRef)
	if provider.Spec.CertSecretRef == nil || provider.Spec.CertSecretRef.Name != "cert-secret" {
		t.Error("expected CertSecretRef.Name 'cert-secret'")
	}
}

func TestResourceSetInputProviderHelpers(t *testing.T) {
	provider := CreateResourceSetInputProvider("input-provider", "flux-system")

	secretRef := &meta.LocalObjectReference{Name: "registry-secret"}
	SetResourceSetInputProviderSecretRef(provider, secretRef)
	if provider.Spec.SecretRef != secretRef {
		t.Error("expected SecretRef to be set")
	}

	certSecretRef := &meta.LocalObjectReference{Name: "cert-secret"}
	SetResourceSetInputProviderCertSecretRef(provider, certSecretRef)
	if provider.Spec.CertSecretRef != certSecretRef {
		t.Error("expected CertSecretRef to be set")
	}

	schedule := fluxv1.Schedule{
		Cron: "0 */6 * * *",
	}
	AddResourceSetInputProviderSchedule(provider, schedule)
	if !slices.Contains(provider.Spec.Schedule, schedule) {
		t.Errorf("expected schedule %q to be added", schedule.Cron)
	}
}

func TestResourceSetAdvancedHelpers(t *testing.T) {
	resourceSet := CreateResourceSet("test-resourceset", "flux-system")

	resource := &apiextensionsv1.JSON{Raw: []byte(`{"apiVersion": "v1", "kind": "ConfigMap"}`)}
	AddResourceSetResource(resourceSet, resource)
	if len(resourceSet.Spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resourceSet.Spec.Resources))
	}
	if string(resourceSet.Spec.Resources[0].Raw) != string(resource.Raw) {
		t.Errorf("expected resource %s, got %s", string(resource.Raw), string(resourceSet.Spec.Resources[0].Raw))
	}

	dependency := fluxv1.Dependency{
		Name: "prerequisite-resource",
	}
	AddResourceSetDependency(resourceSet, dependency)
	if len(resourceSet.Spec.DependsOn) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(resourceSet.Spec.DependsOn))
	}
	if resourceSet.Spec.DependsOn[0].Name != dependency.Name {
		t.Errorf("expected dependency name %q, got %q", dependency.Name, resourceSet.Spec.DependsOn[0].Name)
	}

	commonMetadata := &fluxv1.CommonMetadata{
		Labels: map[string]string{
			"app": "test",
		},
	}
	SetResourceSetCommonMetadata(resourceSet, commonMetadata)
	if resourceSet.Spec.CommonMetadata == nil {
		t.Fatal("expected CommonMetadata to be set, got nil")
	}
	if resourceSet.Spec.CommonMetadata.Labels["app"] != "test" {
		t.Errorf("expected CommonMetadata label app=%q, got %q", "test", resourceSet.Spec.CommonMetadata.Labels["app"])
	}
}
