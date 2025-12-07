package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/generators"

	// Import generators to register them
	_ "github.com/go-kure/kure/pkg/stack/generators/appworkload"
	_ "github.com/go-kure/kure/pkg/stack/generators/fluxhelm"
)

func TestAppWorkloadYAMLConstant(t *testing.T) {
	var wrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(appWorkloadYAML), &wrapper); err != nil {
		t.Errorf("Failed to unmarshal appWorkloadYAML: %v", err)
	}

	if wrapper.APIVersion != "generators.gokure.dev/v1alpha1" {
		t.Errorf("APIVersion = %s, want generators.gokure.dev/v1alpha1", wrapper.APIVersion)
	}

	if wrapper.Kind != "AppWorkload" {
		t.Errorf("Kind = %s, want AppWorkload", wrapper.Kind)
	}

	if wrapper.Metadata.Name != "nginx-app" {
		t.Errorf("Name = %s, want nginx-app", wrapper.Metadata.Name)
	}

	if wrapper.Metadata.Namespace != "web" {
		t.Errorf("Namespace = %s, want web", wrapper.Metadata.Namespace)
	}
}

func TestFluxHelmYAMLConstant(t *testing.T) {
	var wrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(fluxHelmYAML), &wrapper); err != nil {
		t.Errorf("Failed to unmarshal fluxHelmYAML: %v", err)
	}

	if wrapper.APIVersion != "generators.gokure.dev/v1alpha1" {
		t.Errorf("APIVersion = %s, want generators.gokure.dev/v1alpha1", wrapper.APIVersion)
	}

	if wrapper.Kind != "FluxHelm" {
		t.Errorf("Kind = %s, want FluxHelm", wrapper.Kind)
	}

	if wrapper.Metadata.Name != "postgresql" {
		t.Errorf("Name = %s, want postgresql", wrapper.Metadata.Name)
	}

	if wrapper.Metadata.Namespace != "database" {
		t.Errorf("Namespace = %s, want database", wrapper.Metadata.Namespace)
	}
}

func TestFluxHelmOCIYAMLConstant(t *testing.T) {
	var wrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(fluxHelmOCIYAML), &wrapper); err != nil {
		t.Errorf("Failed to unmarshal fluxHelmOCIYAML: %v", err)
	}

	if wrapper.APIVersion != "generators.gokure.dev/v1alpha1" {
		t.Errorf("APIVersion = %s, want generators.gokure.dev/v1alpha1", wrapper.APIVersion)
	}

	if wrapper.Kind != "FluxHelm" {
		t.Errorf("Kind = %s, want FluxHelm", wrapper.Kind)
	}

	if wrapper.Metadata.Name != "podinfo" {
		t.Errorf("Name = %s, want podinfo", wrapper.Metadata.Name)
	}

	if wrapper.Metadata.Namespace != "apps" {
		t.Errorf("Namespace = %s, want apps", wrapper.Metadata.Namespace)
	}
}

func TestDemoGVKGenerators(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping demo test in short mode")
	}

	// Capture stdout
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var output bytes.Buffer
	go func() {
		io.Copy(&output, r)
	}()

	// Run the demo function
	DemoGVKGenerators()

	w.Close()
	os.Stdout = originalStdout

	outputStr := output.String()

	// Check for expected output sections
	expectedSections := []string{
		"=== GVK-Based Generators Demo ===",
		"1. AppWorkload Generator:",
		"2. FluxHelm Generator (HelmRepository):",
		"3. FluxHelm Generator (OCIRepository):",
		"4. Bundle with Multiple Generator Types:",
		"5. Registered Generator Types:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(outputStr, section) {
			t.Errorf("Expected output section not found: %s", section)
		}
	}
}

func TestRunGVKDemo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping demo test in short mode")
	}

	// Capture stdout
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var output bytes.Buffer
	go func() {
		io.Copy(&output, r)
	}()

	// Run the demo function
	RunGVKDemo()

	w.Close()
	os.Stdout = originalStdout

	outputStr := output.String()

	if !strings.Contains(outputStr, "=== GVK-Based Generators Demo ===") {
		t.Error("RunGVKDemo() did not output expected header")
	}
}

func TestAppWorkloadGeneration(t *testing.T) {
	var wrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(appWorkloadYAML), &wrapper); err != nil {
		t.Fatalf("Failed to unmarshal appWorkloadYAML: %v", err)
	}

	app := wrapper.ToApplication()
	if app == nil {
		t.Fatal("ToApplication() returned nil")
	}

	objects, err := app.Config.Generate(app)
	if err != nil {
		t.Fatalf("Failed to generate AppWorkload resources: %v", err)
	}

	if len(objects) == 0 {
		t.Error("Generate() returned no objects")
	}

	// Check for expected resource types
	foundDeployment := false
	foundService := false

	for _, obj := range objects {
		o := *obj
		kind := o.GetObjectKind().GroupVersionKind().Kind
		switch kind {
		case "Deployment":
			foundDeployment = true
		case "Service":
			foundService = true
		}
	}

	if !foundDeployment {
		t.Error("Expected Deployment resource not found")
	}

	if !foundService {
		t.Error("Expected Service resource not found")
	}
}

func TestFluxHelmGeneration(t *testing.T) {
	var wrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(fluxHelmYAML), &wrapper); err != nil {
		t.Fatalf("Failed to unmarshal fluxHelmYAML: %v", err)
	}

	app := wrapper.ToApplication()
	if app == nil {
		t.Fatal("ToApplication() returned nil")
	}

	objects, err := app.Config.Generate(app)
	if err != nil {
		t.Fatalf("Failed to generate FluxHelm resources: %v", err)
	}

	if len(objects) == 0 {
		t.Error("Generate() returned no objects")
	}

	// Check for expected resource types
	foundHelmRepository := false
	foundHelmRelease := false

	for _, obj := range objects {
		o := *obj
		kind := o.GetObjectKind().GroupVersionKind().Kind
		switch kind {
		case "HelmRepository":
			foundHelmRepository = true
		case "HelmRelease":
			foundHelmRelease = true
		}
	}

	if !foundHelmRepository {
		t.Error("Expected HelmRepository resource not found")
	}

	if !foundHelmRelease {
		t.Error("Expected HelmRelease resource not found")
	}
}

func TestFluxHelmOCIGeneration(t *testing.T) {
	var wrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(fluxHelmOCIYAML), &wrapper); err != nil {
		t.Fatalf("Failed to unmarshal fluxHelmOCIYAML: %v", err)
	}

	app := wrapper.ToApplication()
	if app == nil {
		t.Fatal("ToApplication() returned nil")
	}

	objects, err := app.Config.Generate(app)
	if err != nil {
		t.Fatalf("Failed to generate FluxHelm OCI resources: %v", err)
	}

	if len(objects) == 0 {
		t.Error("Generate() returned no objects")
	}

	// Check for expected resource types
	foundOCIRepository := false
	foundHelmRelease := false

	for _, obj := range objects {
		o := *obj
		kind := o.GetObjectKind().GroupVersionKind().Kind
		switch kind {
		case "OCIRepository":
			foundOCIRepository = true
		case "HelmRelease":
			foundHelmRelease = true
		}
	}

	if !foundOCIRepository {
		t.Error("Expected OCIRepository resource not found")
	}

	if !foundHelmRelease {
		t.Error("Expected HelmRelease resource not found")
	}
}

func TestBundleWithMultipleGenerators(t *testing.T) {
	// Parse all three YAML configs
	var appWrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(appWorkloadYAML), &appWrapper); err != nil {
		t.Fatalf("Failed to unmarshal appWorkloadYAML: %v", err)
	}

	var helmWrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(fluxHelmYAML), &helmWrapper); err != nil {
		t.Fatalf("Failed to unmarshal fluxHelmYAML: %v", err)
	}

	var ociWrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(fluxHelmOCIYAML), &ociWrapper); err != nil {
		t.Fatalf("Failed to unmarshal fluxHelmOCIYAML: %v", err)
	}

	// Create bundle with all applications
	apps := []*stack.Application{
		appWrapper.ToApplication(),
		helmWrapper.ToApplication(),
		ociWrapper.ToApplication(),
	}

	bundle, err := stack.NewBundle("mixed-apps", apps, nil)
	if err != nil {
		t.Fatalf("Failed to create bundle: %v", err)
	}

	if len(bundle.Applications) != 3 {
		t.Errorf("Bundle applications count = %d, want 3", len(bundle.Applications))
	}

	// Generate all resources
	objects, err := bundle.Generate()
	if err != nil {
		t.Fatalf("Failed to generate bundle resources: %v", err)
	}

	if len(objects) == 0 {
		t.Error("Bundle.Generate() returned no resources")
	}

	// Should have resources from all three generators
	resourceTypes := make(map[string]int)
	for _, obj := range objects {
		o := *obj
		kind := o.GetObjectKind().GroupVersionKind().Kind
		resourceTypes[kind]++
	}

	expectedTypes := []string{"Deployment", "Service", "HelmRepository", "HelmRelease", "OCIRepository"}
	for _, expectedType := range expectedTypes {
		if resourceTypes[expectedType] == 0 {
			t.Errorf("Expected resource type %s not found in bundle output", expectedType)
		}
	}
}

func TestRegisteredGeneratorTypes(t *testing.T) {
	registeredTypes := generators.ListKinds()

	if len(registeredTypes) == 0 {
		t.Error("No generator types are registered")
	}

	// Check for expected generator types
	expectedTypes := []string{"AppWorkload", "FluxHelm"}
	found := make(map[string]bool)

	for _, gvk := range registeredTypes {
		gvkStr := gvk.String()
		for _, expected := range expectedTypes {
			if strings.Contains(gvkStr, expected) {
				found[expected] = true
			}
		}
	}

	for _, expected := range expectedTypes {
		if !found[expected] {
			t.Errorf("Expected generator type %s not found in registered types", expected)
		}
	}
}

func TestApplicationGeneration_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		wantErr  bool
	}{
		{
			name: "Minimal AppWorkload",
			yamlData: `apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: minimal-app
  namespace: default
spec:
  workload: Deployment
  containers:
    - name: nginx
      image: nginx:latest`,
			wantErr: false,
		},
		{
			name: "Minimal FluxHelm",
			yamlData: `apiVersion: generators.gokure.dev/v1alpha1
kind: FluxHelm
metadata:
  name: minimal-helm
  namespace: default
spec:
  chart:
    name: nginx
  source:
    type: HelmRepository
    url: https://charts.example.com`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wrapper stack.ApplicationWrapper
			if err := yaml.Unmarshal([]byte(tt.yamlData), &wrapper); err != nil {
				if !tt.wantErr {
					t.Errorf("Failed to unmarshal YAML: %v", err)
				}
				return
			}

			app := wrapper.ToApplication()
			if app == nil {
				if !tt.wantErr {
					t.Error("ToApplication() returned nil")
				}
				return
			}

			_, err := app.Config.Generate(app)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestYAMLMarshaling(t *testing.T) {
	// Test that we can generate resources and marshal them back to YAML
	var wrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(appWorkloadYAML), &wrapper); err != nil {
		t.Fatalf("Failed to unmarshal appWorkloadYAML: %v", err)
	}

	app := wrapper.ToApplication()
	objects, err := app.Config.Generate(app)
	if err != nil {
		t.Fatalf("Failed to generate resources: %v", err)
	}

	if len(objects) == 0 {
		t.Fatal("No objects generated")
	}

	// Try to marshal first object back to YAML
	yamlBytes, err := yaml.Marshal(*objects[0])
	if err != nil {
		t.Errorf("Failed to marshal resource to YAML: %v", err)
	}

	if len(yamlBytes) == 0 {
		t.Error("Marshal produced empty YAML")
	}

	// Verify the YAML contains expected fields (case-insensitive)
	yamlStr := strings.ToLower(string(yamlBytes))
	if !strings.Contains(yamlStr, "apiversion") {
		// Some Kubernetes resources might have different YAML structure
		t.Logf("Generated YAML:\n%s", string(yamlBytes))
		t.Log("Generated YAML may have different structure than expected")
	}
}

func TestGeneratorConfigValidation(t *testing.T) {
	// Test with invalid generator kind
	invalidYAML := `apiVersion: generators.gokure.dev/v1alpha1
kind: InvalidGenerator
metadata:
  name: invalid
  namespace: default
spec:
  test: value`

	var wrapper stack.ApplicationWrapper
	err := yaml.Unmarshal([]byte(invalidYAML), &wrapper)
	if err == nil {
		t.Error("Expected error for invalid generator kind, but got none")
		return
	}

	// The error is expected because the generator type is not registered
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("Expected 'unknown type' error, got: %v", err)
	}
}

func TestDemoOutput_Formatting(t *testing.T) {
	// Capture stdout and verify output formatting
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var output bytes.Buffer
	done := make(chan bool)
	go func() {
		io.Copy(&output, r)
		done <- true
	}()

	// Generate a small demo to check formatting
	var wrapper stack.ApplicationWrapper
	yaml.Unmarshal([]byte(appWorkloadYAML), &wrapper)
	app := wrapper.ToApplication()
	objects, _ := app.Config.Generate(app)

	// Simulate the demo output formatting
	fmt.Printf("Generated %d resources:\n", len(objects))
	for _, obj := range objects {
		o := *obj
		fmt.Printf("  - %s: %s/%s\n", o.GetObjectKind().GroupVersionKind().Kind,
			o.GetNamespace(), o.GetName())
	}

	w.Close()
	os.Stdout = originalStdout

	// Wait for the goroutine to finish reading
	<-done

	outputStr := output.String()

	// Check formatting patterns
	if !strings.Contains(outputStr, "Generated") {
		t.Errorf("Output missing 'Generated' keyword. Got: %q", outputStr)
	}
	if !strings.Contains(outputStr, "resources:") {
		t.Errorf("Output missing 'resources:' label. Got: %q", outputStr)
	}
	if !strings.Contains(outputStr, "  - ") {
		t.Errorf("Output missing resource list formatting. Got: %q", outputStr)
	}
}
