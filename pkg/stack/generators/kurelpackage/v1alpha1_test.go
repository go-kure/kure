package kurelpackage

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/stack"
)

func TestKurelPackageV1Alpha1(t *testing.T) {
	t.Run("Parse Basic KurelPackage", func(t *testing.T) {
		yamlContent := `
apiVersion: generators.gokure.dev/v1alpha1
kind: KurelPackage
metadata:
  name: my-app-package
  namespace: default
spec:
  package:
    name: my-app
    version: 1.0.0
    description: "My application package"
    authors:
      - "Developer Name"
    license: Apache-2.0
    homepage: https://example.com
    keywords:
      - kubernetes
      - app
  
  resources:
    - source: ./manifests
      includes: ["*.yaml"]
      excludes: ["*-test.yaml"]
      recurse: true
  
  patches:
    - target:
        kind: Deployment
        name: my-app
      patch: |
        - op: replace
          path: /spec/replicas
          value: 3
      type: json
  
  values:
    defaults: ./values.yaml
    schema: ./values.schema.json
  
  extensions:
    - name: monitoring
      when: .Values.monitoring.enabled
      resources:
        - source: ./monitoring
          includes: ["*.yaml"]
  
  dependencies:
    - name: base-config
      version: ">=1.0.0"
      repository: oci://registry.example.com/packages
  
  build:
    format: oci
    registry: registry.example.com
    repository: my-org/my-app
    tags:
      - latest
      - 1.0.0
`

		var wrapper stack.ApplicationWrapper
		if err := yaml.Unmarshal([]byte(yamlContent), &wrapper); err != nil {
			t.Fatalf("Failed to unmarshal KurelPackage: %v", err)
		}

		if wrapper.APIVersion != "generators.gokure.dev/v1alpha1" {
			t.Errorf("APIVersion = %v, want generators.gokure.dev/v1alpha1", wrapper.APIVersion)
		}

		if wrapper.Kind != "KurelPackage" {
			t.Errorf("Kind = %v, want KurelPackage", wrapper.Kind)
		}

		if wrapper.Metadata.Name != "my-app-package" {
			t.Errorf("Name = %v, want my-app-package", wrapper.Metadata.Name)
		}

		// Convert to application
		app := wrapper.ToApplication()
		if app == nil {
			t.Fatal("ToApplication returned nil")
		}

		// Try to generate (will fail with not implemented error for now)
		_, err := app.Config.Generate(app)
		if err == nil {
			t.Error("Expected not implemented error")
		}
	})

	t.Run("Minimal KurelPackage", func(t *testing.T) {
		yamlContent := `
apiVersion: generators.gokure.dev/v1alpha1
kind: KurelPackage
metadata:
  name: simple-package
spec:
  package:
    name: simple
    version: 0.1.0
  resources:
    - source: ./manifests
`

		var wrapper stack.ApplicationWrapper
		if err := yaml.Unmarshal([]byte(yamlContent), &wrapper); err != nil {
			t.Fatalf("Failed to unmarshal minimal KurelPackage: %v", err)
		}

		if wrapper.Kind != "KurelPackage" {
			t.Errorf("Kind = %v, want KurelPackage", wrapper.Kind)
		}

		app := wrapper.ToApplication()
		if app == nil {
			t.Fatal("ToApplication returned nil")
		}
	})

	t.Run("Complex Patches", func(t *testing.T) {
		yamlContent := `
apiVersion: generators.gokure.dev/v1alpha1
kind: KurelPackage
metadata:
  name: patch-example
spec:
  package:
    name: patch-demo
    version: 1.0.0
  
  patches:
    - target:
        apiVersion: apps/v1
        kind: Deployment
        name: app
        namespace: default
      patch: |
        - op: add
          path: /metadata/labels/environment
          value: production
        - op: replace
          path: /spec/replicas
          value: 5
      type: json
    
    - target:
        kind: Service
        name: app-service
        labels:
          app: myapp
      patch: |
        spec:
          type: LoadBalancer
          ports:
          - port: 443
            targetPort: 8443
            protocol: TCP
      type: strategic
`

		var wrapper stack.ApplicationWrapper
		if err := yaml.Unmarshal([]byte(yamlContent), &wrapper); err != nil {
			t.Fatalf("Failed to unmarshal KurelPackage with patches: %v", err)
		}

		if wrapper.Kind != "KurelPackage" {
			t.Errorf("Kind = %v, want KurelPackage", wrapper.Kind)
		}
	})

	t.Run("Multiple Extensions", func(t *testing.T) {
		yamlContent := `
apiVersion: generators.gokure.dev/v1alpha1
kind: KurelPackage
metadata:
  name: extensions-example
spec:
  package:
    name: feature-rich
    version: 2.0.0
  
  extensions:
    - name: monitoring
      when: .Values.monitoring.enabled
      resources:
        - source: ./monitoring
          includes: ["servicemonitor.yaml", "grafana-dashboard.yaml"]
    
    - name: ingress
      when: .Values.ingress.enabled
      resources:
        - source: ./ingress
      patches:
        - target:
            kind: Ingress
            name: main
          patch: |
            - op: replace
              path: /spec/rules/0/host
              value: "{{ .Values.ingress.host }}"
    
    - name: high-availability
      when: .Values.ha.enabled
      patches:
        - target:
            kind: Deployment
            name: app
          patch: |
            - op: replace
              path: /spec/replicas
              value: 3
`

		var wrapper stack.ApplicationWrapper
		if err := yaml.Unmarshal([]byte(yamlContent), &wrapper); err != nil {
			t.Fatalf("Failed to unmarshal KurelPackage with extensions: %v", err)
		}

		if wrapper.Kind != "KurelPackage" {
			t.Errorf("Kind = %v, want KurelPackage", wrapper.Kind)
		}
	})

	t.Run("Validation Tests", func(t *testing.T) {
		t.Run("Valid Configuration", func(t *testing.T) {
			config := &ConfigV1Alpha1{
				Package: PackageMetadata{
					Name:    "valid-package",
					Version: "1.0.0",
				},
			}

			if err := config.Validate(); err != nil {
				t.Errorf("Valid configuration should not fail validation: %v", err)
			}
		})

		t.Run("Missing Package Name", func(t *testing.T) {
			config := &ConfigV1Alpha1{
				Package: PackageMetadata{
					Version: "1.0.0",
				},
			}

			if err := config.Validate(); err == nil {
				t.Error("Should fail validation with missing package name")
			}
		})

		t.Run("Invalid Version Format", func(t *testing.T) {
			config := &ConfigV1Alpha1{
				Package: PackageMetadata{
					Name:    "test-package",
					Version: "invalid-version",
				},
			}

			if err := config.Validate(); err == nil {
				t.Error("Should fail validation with invalid version format")
			}
		})

		t.Run("Invalid Package Name", func(t *testing.T) {
			config := &ConfigV1Alpha1{
				Package: PackageMetadata{
					Name:    "Invalid_Name_With_Underscores",
					Version: "1.0.0",
				},
			}

			if err := config.Validate(); err == nil {
				t.Error("Should fail validation with invalid package name")
			}
		})
	})

	t.Run("GeneratePackageFiles", func(t *testing.T) {
		t.Run("Empty Configuration", func(t *testing.T) {
			config := &ConfigV1Alpha1{
				Package: PackageMetadata{
					Name:    "test-package",
					Version: "1.0.0",
				},
			}

			app := &stack.Application{
				Config: config,
			}

			files, err := config.GeneratePackageFiles(app)
			if err != nil {
				t.Fatalf("Failed to generate package files: %v", err)
			}

			// Should at least have kurel.yaml
			if _, exists := files["kurel.yaml"]; !exists {
				t.Error("Generated files should include kurel.yaml")
			}

			// Verify kurel.yaml content
			kurelContent := files["kurel.yaml"]
			var kurelDoc map[string]interface{}
			if err := yaml.Unmarshal(kurelContent, &kurelDoc); err != nil {
				t.Fatalf("kurel.yaml should be valid YAML: %v", err)
			}

			if kurelDoc["apiVersion"] != "kurel.gokure.dev/v1alpha1" {
				t.Error("kurel.yaml should have correct apiVersion")
			}

			if kurelDoc["kind"] != "Package" {
				t.Error("kurel.yaml should have kind Package")
			}
		})
	})

	t.Run("Patch Validation", func(t *testing.T) {
		t.Run("Valid JSON Patch", func(t *testing.T) {
			config := &ConfigV1Alpha1{}
			patch := PatchDefinition{
				Target: PatchTarget{
					Kind: "Deployment",
					Name: "test",
				},
				Patch: `- op: replace
  path: /spec/replicas
  value: 3`,
				Type: "json",
			}

			if err := config.validatePatchDefinition(patch); err != nil {
				t.Errorf("Valid JSON patch should not fail validation: %v", err)
			}
		})

		t.Run("Invalid JSON Patch", func(t *testing.T) {
			config := &ConfigV1Alpha1{}
			patch := PatchDefinition{
				Target: PatchTarget{
					Kind: "Deployment",
					Name: "test",
				},
				Patch: `- op: invalid-operation
  path: /spec/replicas
  value: 3`,
				Type: "json",
			}

			if err := config.validatePatchDefinition(patch); err == nil {
				t.Error("Invalid JSON patch should fail validation")
			}
		})

		t.Run("Valid Strategic Merge Patch", func(t *testing.T) {
			config := &ConfigV1Alpha1{}
			patch := PatchDefinition{
				Target: PatchTarget{
					Kind: "Deployment",
					Name: "test",
				},
				Patch: `spec:
  replicas: 3`,
				Type: "strategic",
			}

			if err := config.validatePatchDefinition(patch); err != nil {
				t.Errorf("Valid strategic merge patch should not fail validation: %v", err)
			}
		})
	})

	t.Run("Version Validation", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		validVersions := []string{
			"1.0.0",
			"1.0.0-alpha.1",
			"1.0.0+build.1",
			"1.0.0-alpha.1+build.1",
			"0.1.0",
			"10.20.30",
		}

		for _, version := range validVersions {
			if err := config.validateVersionFormat(version); err != nil {
				t.Errorf("Version %s should be valid: %v", version, err)
			}
		}

		invalidVersions := []string{
			"1.0",
			"1",
			"1.0.0.0",
			"v1.0.0",
			"1.0.0-",
			"1.0.0+",
			"invalid",
		}

		for _, version := range invalidVersions {
			if err := config.validateVersionFormat(version); err == nil {
				t.Errorf("Version %s should be invalid", version)
			}
		}
	})

	t.Run("CEL Expression Validation", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		validExpressions := []string{
			".Values.enabled",
			".Values.monitoring.enabled",
			".Values.replicas > 1",
			".Values.environment == 'production'",
		}

		for _, expr := range validExpressions {
			if err := config.validateCELExpression(expr); err != nil {
				t.Errorf("CEL expression %s should be valid: %v", expr, err)
			}
		}

		invalidExpressions := []string{
			"",
			"   ",
			"invalid$expression",
			"Values.enabled", // Missing dot
		}

		for _, expr := range invalidExpressions {
			if err := config.validateCELExpression(expr); err == nil {
				t.Errorf("CEL expression %s should be invalid", expr)
			}
		}
	})
}
