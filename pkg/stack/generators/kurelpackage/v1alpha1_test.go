package kurelpackage

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"

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

		// Generate() requires real resource files on disk, so we don't
		// call it here — this test validates YAML parsing only.
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
			var kurelDoc map[string]any
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

	t.Run("GetAPIVersion and GetKind", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		if config.GetAPIVersion() != "generators.gokure.dev/v1alpha1" {
			t.Errorf("GetAPIVersion() = %q, want %q", config.GetAPIVersion(), "generators.gokure.dev/v1alpha1")
		}

		if config.GetKind() != "KurelPackage" {
			t.Errorf("GetKind() = %q, want %q", config.GetKind(), "KurelPackage")
		}
	})

	t.Run("Generate returns resources from package", func(t *testing.T) {
		tmpDir := t.TempDir()
		resDir := filepath.Join(tmpDir, "manifests")
		if err := os.MkdirAll(resDir, 0o755); err != nil {
			t.Fatal(err)
		}
		podYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  key: value
`
		if err := os.WriteFile(filepath.Join(resDir, "cm.yaml"), []byte(podYAML), 0o644); err != nil {
			t.Fatal(err)
		}

		config := &ConfigV1Alpha1{
			Package: PackageMetadata{
				Name:    "test-pkg",
				Version: "1.0.0",
			},
			Resources: []ResourceSource{
				{Source: resDir},
			},
		}
		app := &stack.Application{Config: config}

		objs, err := config.Generate(app)
		if err != nil {
			t.Fatalf("Generate() returned unexpected error: %v", err)
		}
		if len(objs) != 1 {
			t.Fatalf("Generate() returned %d objects, want 1", len(objs))
		}
	})

	t.Run("Validate with resources", func(t *testing.T) {
		config := &ConfigV1Alpha1{
			Package: PackageMetadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			Resources: []ResourceSource{
				{Source: ""},
			},
		}

		if err := config.Validate(); err == nil {
			t.Error("Should fail validation with empty resource source")
		}
	})

	t.Run("Validate with patches", func(t *testing.T) {
		config := &ConfigV1Alpha1{
			Package: PackageMetadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			Patches: []PatchDefinition{
				{
					Target: PatchTarget{Kind: "", Name: "test"},
					Patch:  "test",
				},
			},
		}

		if err := config.Validate(); err == nil {
			t.Error("Should fail validation with empty patch kind")
		}
	})

	t.Run("Validate with dependencies", func(t *testing.T) {
		config := &ConfigV1Alpha1{
			Package: PackageMetadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			Dependencies: []Dependency{
				{Name: "", Version: "1.0.0"},
			},
		}

		if err := config.Validate(); err == nil {
			t.Error("Should fail validation with empty dependency name")
		}
	})

	t.Run("Validate dependency version constraint", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		validConstraints := []string{
			"1.0.0",
			">=1.0.0",
			"^1.0.0",
			"~1.2.0",
			"<2.0.0",
		}

		for _, constraint := range validConstraints {
			if err := config.validateVersionConstraint(constraint); err != nil {
				t.Errorf("Version constraint %s should be valid: %v", constraint, err)
			}
		}

		invalidConstraints := []string{
			"invalid",
			"1.0",
			"v1.0.0",
		}

		for _, constraint := range invalidConstraints {
			if err := config.validateVersionConstraint(constraint); err == nil {
				t.Errorf("Version constraint %s should be invalid", constraint)
			}
		}
	})

	t.Run("Validate build config", func(t *testing.T) {
		config := &ConfigV1Alpha1{
			Package: PackageMetadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			Build: &BuildConfig{
				Format: "invalid",
			},
		}

		if err := config.Validate(); err == nil {
			t.Error("Should fail validation with invalid build format")
		}

		config.Build.Format = "oci"
		config.Build.Registry = ""
		if err := config.Validate(); err == nil {
			t.Error("Should fail validation with OCI format but no registry")
		}

		config.Build.Registry = "registry.example.com"
		config.Build.Repository = ""
		if err := config.Validate(); err == nil {
			t.Error("Should fail validation with OCI format but no repository")
		}

		config.Build.Repository = "my-repo"
		if err := config.Validate(); err != nil {
			t.Errorf("Valid OCI build config should pass: %v", err)
		}
	})

	t.Run("isKubernetesResource", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		validResource := []byte(`apiVersion: v1
kind: Pod
metadata:
  name: test`)

		if !config.isKubernetesResource(validResource) {
			t.Error("Should identify valid Kubernetes resource")
		}

		invalidResource := []byte(`name: not-k8s`)
		if config.isKubernetesResource(invalidResource) {
			t.Error("Should not identify non-Kubernetes content as resource")
		}

		invalidYAML := []byte(`{invalid: yaml:`)
		if config.isKubernetesResource(invalidYAML) {
			t.Error("Should not identify invalid YAML as resource")
		}
	})

	t.Run("shouldIncludeFile", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		// No includes/excludes - should include all
		resource := ResourceSource{}
		if !config.shouldIncludeFile("test.yaml", resource) {
			t.Error("Should include file when no patterns specified")
		}

		// With exclude pattern
		resource = ResourceSource{Excludes: []string{"*-test.yaml"}}
		if config.shouldIncludeFile("app-test.yaml", resource) {
			t.Error("Should exclude file matching exclude pattern")
		}
		if !config.shouldIncludeFile("app.yaml", resource) {
			t.Error("Should include file not matching exclude pattern")
		}

		// With include pattern
		resource = ResourceSource{Includes: []string{"*.yaml"}}
		if !config.shouldIncludeFile("test.yaml", resource) {
			t.Error("Should include file matching include pattern")
		}
		if config.shouldIncludeFile("test.txt", resource) {
			t.Error("Should not include file not matching include pattern")
		}
	})

	t.Run("validateJSONPatch", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		validPatch := `- op: replace
  path: /spec/replicas
  value: 3`
		if err := config.validateJSONPatch(validPatch); err != nil {
			t.Errorf("Valid JSON patch should pass: %v", err)
		}

		// Missing op
		invalidPatch := `- path: /spec/replicas
  value: 3`
		if err := config.validateJSONPatch(invalidPatch); err == nil {
			t.Error("Should fail for missing op")
		}

		// Missing path
		invalidPatch = `- op: replace
  value: 3`
		if err := config.validateJSONPatch(invalidPatch); err == nil {
			t.Error("Should fail for missing path")
		}

		// Invalid op
		invalidPatch = `- op: invalid
  path: /spec
  value: 3`
		if err := config.validateJSONPatch(invalidPatch); err == nil {
			t.Error("Should fail for invalid op")
		}

		// add without value
		invalidPatch = `- op: add
  path: /spec`
		if err := config.validateJSONPatch(invalidPatch); err == nil {
			t.Error("Should fail for add without value")
		}

		// move without from
		invalidPatch = `- op: move
  path: /spec/new`
		if err := config.validateJSONPatch(invalidPatch); err == nil {
			t.Error("Should fail for move without from")
		}

		// path not starting with /
		invalidPatch = `- op: replace
  path: spec/replicas
  value: 3`
		if err := config.validateJSONPatch(invalidPatch); err == nil {
			t.Error("Should fail for path not starting with /")
		}

		// remove is valid without value
		validRemove := `- op: remove
  path: /spec/field`
		if err := config.validateJSONPatch(validRemove); err != nil {
			t.Errorf("Valid remove patch should pass: %v", err)
		}
	})

	t.Run("validateStrategicMergePatch", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		validPatch := `spec:
  replicas: 3`
		if err := config.validateStrategicMergePatch(validPatch); err != nil {
			t.Errorf("Valid strategic merge patch should pass: %v", err)
		}

		emptyPatch := ``
		if err := config.validateStrategicMergePatch(emptyPatch); err == nil {
			t.Error("Should fail for empty patch")
		}

		invalidYAML := `{invalid: yaml:`
		if err := config.validateStrategicMergePatch(invalidYAML); err == nil {
			t.Error("Should fail for invalid YAML")
		}
	})

	t.Run("validateValuesConfig", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		// Empty values config
		emptyValues := ValuesConfig{}
		if err := config.validateValuesConfig(emptyValues); err == nil {
			t.Error("Should fail for empty values config")
		}

		// With inline values
		withInline := ValuesConfig{Values: map[string]any{"key": "value"}}
		if err := config.validateValuesConfig(withInline); err != nil {
			t.Errorf("Valid inline values should pass: %v", err)
		}
	})

	t.Run("validateExtension", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		// Empty name
		ext := Extension{Name: ""}
		if err := config.validateExtension(ext); err == nil {
			t.Error("Should fail for empty extension name")
		}

		// Invalid name format
		ext = Extension{Name: "Invalid_Name"}
		if err := config.validateExtension(ext); err == nil {
			t.Error("Should fail for invalid extension name")
		}

		// Valid extension
		ext = Extension{Name: "valid-ext"}
		if err := config.validateExtension(ext); err != nil {
			t.Errorf("Valid extension should pass: %v", err)
		}
	})

	t.Run("validatePackageName", func(t *testing.T) {
		config := &ConfigV1Alpha1{}

		validNames := []string{
			"valid-name",
			"valid123",
			"a",
			"abc-def-ghi",
		}

		for _, name := range validNames {
			if err := config.validatePackageName(name); err != nil {
				t.Errorf("Package name %q should be valid: %v", name, err)
			}
		}

		invalidNames := []string{
			"",
			"-invalid",
			"invalid-",
			"Invalid",
			"invalid_name",
			"invalid.name",
		}

		for _, name := range invalidNames {
			if err := config.validatePackageName(name); err == nil {
				t.Errorf("Package name %q should be invalid", name)
			}
		}
	})
}

func TestGenerateBasicResources(t *testing.T) {
	tmpDir := t.TempDir()
	resDir := filepath.Join(tmpDir, "resources")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatal(err)
	}

	podYAML := `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
spec:
  containers:
    - name: nginx
      image: nginx:latest
`
	if err := os.WriteFile(filepath.Join(resDir, "pod.yaml"), []byte(podYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &ConfigV1Alpha1{
		Package: PackageMetadata{
			Name:    "basic-pkg",
			Version: "1.0.0",
		},
		Resources: []ResourceSource{
			{Source: resDir},
		},
	}
	app := &stack.Application{Config: config}

	objs, err := config.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("Generate() returned %d objects, want 1", len(objs))
	}

	pod, ok := (*objs[0]).(*corev1.Pod)
	if !ok {
		t.Fatalf("expected *corev1.Pod, got %T", *objs[0])
	}
	if pod.Name != "test-pod" {
		t.Errorf("pod name = %q, want %q", pod.Name, "test-pod")
	}
}

func TestGenerateMultipleResources(t *testing.T) {
	tmpDir := t.TempDir()
	resDir := filepath.Join(tmpDir, "resources")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  key: value
`
	svcYAML := `apiVersion: v1
kind: Service
metadata:
  name: test-svc
  namespace: default
spec:
  ports:
    - port: 80
  selector:
    app: test
`
	if err := os.WriteFile(filepath.Join(resDir, "cm.yaml"), []byte(cmYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resDir, "svc.yaml"), []byte(svcYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &ConfigV1Alpha1{
		Package: PackageMetadata{
			Name:    "multi-pkg",
			Version: "1.0.0",
		},
		Resources: []ResourceSource{
			{Source: resDir},
		},
	}
	app := &stack.Application{Config: config}

	objs, err := config.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("Generate() returned %d objects, want 2", len(objs))
	}

	// Verify types — order is non-deterministic (map iteration), so collect by kind
	kinds := make(map[string]bool)
	for _, obj := range objs {
		kinds[(*obj).GetObjectKind().GroupVersionKind().Kind] = true
	}
	if !kinds["ConfigMap"] {
		t.Error("expected ConfigMap in results")
	}
	if !kinds["Service"] {
		t.Error("expected Service in results")
	}
}

func TestGenerateNoResources(t *testing.T) {
	config := &ConfigV1Alpha1{
		Package: PackageMetadata{
			Name:    "empty-pkg",
			Version: "1.0.0",
		},
	}
	app := &stack.Application{Config: config}

	objs, err := config.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if len(objs) != 0 {
		t.Fatalf("Generate() returned %d objects, want 0", len(objs))
	}
}

func TestGenerateExcludesNonResourceFiles(t *testing.T) {
	tmpDir := t.TempDir()
	resDir := filepath.Join(tmpDir, "resources")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  key: value
`
	if err := os.WriteFile(filepath.Join(resDir, "cm.yaml"), []byte(cmYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &ConfigV1Alpha1{
		Package: PackageMetadata{
			Name:    "with-patches",
			Version: "1.0.0",
		},
		Resources: []ResourceSource{
			{Source: resDir},
		},
		Patches: []PatchDefinition{
			{
				Target: PatchTarget{Kind: "ConfigMap", Name: "test-cm"},
				Patch: `- op: replace
  path: /data/key
  value: patched`,
				Type: "json",
			},
		},
	}
	app := &stack.Application{Config: config}

	files, err := config.GeneratePackageFiles(app)
	if err != nil {
		t.Fatalf("GeneratePackageFiles() error: %v", err)
	}

	// Verify patches/ and kurel.yaml exist in the file map
	hasPatch := false
	hasKurelYAML := false
	for path := range files {
		if filepath.Dir(path) == "patches" {
			hasPatch = true
		}
		if path == "kurel.yaml" {
			hasKurelYAML = true
		}
	}
	if !hasPatch {
		t.Fatal("expected patch files in GeneratePackageFiles output")
	}
	if !hasKurelYAML {
		t.Fatal("expected kurel.yaml in GeneratePackageFiles output")
	}

	// Generate() should only return the ConfigMap, not patches or kurel.yaml
	objs, err := config.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("Generate() returned %d objects, want 1", len(objs))
	}

	// Verify the returned object is the ConfigMap, not a patch
	obj := *objs[0]
	if obj.GetObjectKind().GroupVersionKind().Kind != "ConfigMap" {
		t.Errorf("expected ConfigMap, got %s", obj.GetObjectKind().GroupVersionKind().Kind)
	}
	if obj.GetName() != "test-cm" {
		t.Errorf("object name = %q, want %q", obj.GetName(), "test-cm")
	}
}

func TestGeneratePackageFiles_WithValues(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		Package: PackageMetadata{Name: "test-pkg", Version: "1.0.0"},
		Values: &ValuesConfig{
			Values: map[string]any{"replicas": 2, "image": "nginx:latest"},
		},
	}
	app := &stack.Application{Config: cfg}
	files, err := cfg.GeneratePackageFiles(app)
	if err != nil {
		t.Fatalf("GeneratePackageFiles with Values: %v", err)
	}
	if _, ok := files["values/values.yaml"]; !ok {
		t.Fatalf("expected values/values.yaml in output, got keys: %v", mapKeys(files))
	}
}

func TestGeneratePackageFiles_WithExtension(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		Package: PackageMetadata{Name: "test-pkg", Version: "1.0.0"},
		Extensions: []Extension{
			{Name: "extra"},
		},
	}
	app := &stack.Application{Config: cfg}
	files, err := cfg.GeneratePackageFiles(app)
	if err != nil {
		t.Fatalf("GeneratePackageFiles with extension: %v", err)
	}
	if _, ok := files["extensions/extra/extension.yaml"]; !ok {
		t.Fatalf("expected extension file in output, got keys: %v", mapKeys(files))
	}
}

// mapKeys returns sorted keys of a map for readable error messages.
func mapKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// validBaseKurelConfig returns a minimal valid ConfigV1Alpha1 for validation tests.
func validBaseKurelConfig() *ConfigV1Alpha1 {
	return &ConfigV1Alpha1{
		Package: PackageMetadata{Name: "my-package", Version: "1.0.0"},
	}
}

func TestValidate_PackageName(t *testing.T) {
	tests := []struct {
		name    string
		pkgName string
		wantErr bool
	}{
		{"empty name", "", true},
		{"uppercase letters", "MyPackage", true},
		{"starts with hyphen", "-mypackage", true},
		{"ends with hyphen", "mypackage-", true},
		{"valid lowercase", "my-package", false},
		{"valid alphanumeric", "mypkg123", false},
		{"single char", "a", false},
		{"too long (254 chars)", string(make([]byte, 254)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ConfigV1Alpha1{
				Package: PackageMetadata{Name: tt.pkgName, Version: "1.0.0"},
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_PackageVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"empty version", "", true},
		{"not semver", "v1.0", true},
		{"not semver with v prefix", "v1.0.0", true},
		{"valid semver", "1.0.0", false},
		{"valid semver with pre-release", "1.0.0-alpha.1", false},
		{"valid semver with build meta", "1.0.0+build123", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ConfigV1Alpha1{
				Package: PackageMetadata{Name: "my-package", Version: tt.version},
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_ResourceSource(t *testing.T) {
	tests := []struct {
		name    string
		source  ResourceSource
		wantErr bool
	}{
		{"empty Source field", ResourceSource{}, true},
		{"nonexistent path", ResourceSource{Source: "/tmp/nonexistent-kurel-path-99999"}, true},
		{"valid existing path", ResourceSource{Source: "/tmp"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBaseKurelConfig()
			cfg.Resources = []ResourceSource{tt.source}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_ResourceSource_Patterns(t *testing.T) {
	t.Run("valid include pattern", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Resources = []ResourceSource{{Source: "/tmp", Includes: []string{"*.yaml"}}}
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("valid exclude pattern", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Resources = []ResourceSource{{Source: "/tmp", Excludes: []string{"*.bak"}}}
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("invalid include pattern", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Resources = []ResourceSource{{Source: "/tmp", Includes: []string{"[invalid"}}}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for malformed include pattern")
		}
	})
	t.Run("invalid exclude pattern", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Resources = []ResourceSource{{Source: "/tmp", Excludes: []string{"[invalid"}}}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for malformed exclude pattern")
		}
	})
}

func TestValidate_ValuesConfig(t *testing.T) {
	t.Run("all empty fields is invalid", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Values = &ValuesConfig{}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for all-empty ValuesConfig")
		}
	})
	t.Run("inline values is valid", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Values = &ValuesConfig{Values: map[string]any{"k": "v"}}
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("defaults file exists is valid", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "values.yaml")
		os.WriteFile(f, []byte("replicas: 1"), 0o600) //nolint:errcheck
		cfg := validBaseKurelConfig()
		cfg.Values = &ValuesConfig{Defaults: f}
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("defaults file missing is invalid", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Values = &ValuesConfig{Defaults: "/tmp/nonexistent-defaults-12345.yaml"}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for nonexistent defaults file")
		}
	})
	t.Run("schema file exists is valid", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "schema.json")
		os.WriteFile(f, []byte(`{"type":"object"}`), 0o600) //nolint:errcheck
		cfg := validBaseKurelConfig()
		cfg.Values = &ValuesConfig{Schema: f}
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("schema file missing is invalid", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Values = &ValuesConfig{Schema: "/tmp/nonexistent-schema-12345.json"}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for nonexistent schema file")
		}
	})
}

func TestValidate_Dependency(t *testing.T) {
	tests := []struct {
		name    string
		dep     Dependency
		wantErr bool
	}{
		{"empty Name", Dependency{Version: "1.0.0"}, true},
		{"empty Version", Dependency{Name: "dep-a"}, true},
		{"invalid Name format", Dependency{Name: "DEP_A", Version: "1.0.0"}, true},
		{"invalid Version constraint", Dependency{Name: "dep-a", Version: "not-a-version"}, true},
		{"valid dependency exact", Dependency{Name: "dep-a", Version: "1.0.0"}, false},
		{"valid dependency caret constraint", Dependency{Name: "dep-a", Version: "^1.0.0"}, false},
		{"valid dependency gte constraint", Dependency{Name: "dep-a", Version: ">=1.2.3"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBaseKurelConfig()
			cfg.Dependencies = []Dependency{tt.dep}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Extension(t *testing.T) {
	tests := []struct {
		name    string
		ext     Extension
		wantErr bool
	}{
		{"empty Name", Extension{}, true},
		{"invalid Name format (uppercase)", Extension{Name: "MyExt"}, true},
		{"valid extension no when", Extension{Name: "my-ext"}, false},
		{"valid extension with valid CEL when", Extension{Name: "my-ext", When: ".Values.enabled == true"}, false},
		{"extension with empty CEL when (whitespace)", Extension{Name: "my-ext", When: "   "}, true},
		{"extension when without .Values reference", Extension{Name: "my-ext", When: "true == true"}, true},
		{"extension with invalid CEL syntax", Extension{Name: "my-ext", When: ".Values.enabled ==="}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBaseKurelConfig()
			cfg.Extensions = []Extension{tt.ext}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Extension_WithResources(t *testing.T) {
	t.Run("extension with valid resource", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Extensions = []Extension{
			{Name: "my-ext", Resources: []ResourceSource{{Source: "/tmp"}}},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("extension with invalid resource (nonexistent)", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Extensions = []Extension{
			{Name: "my-ext", Resources: []ResourceSource{{Source: "/tmp/nonexistent-ext-path-99999"}}},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for nonexistent extension resource path")
		}
	})
}

func TestValidate_PatchDefinition(t *testing.T) {
	validJSONPatch := `
- op: replace
  path: /spec/replicas
  value: 3
`
	validStrategicPatch := "spec:\n  replicas: 3\n"

	tests := []struct {
		name    string
		patch   PatchDefinition
		wantErr bool
	}{
		{
			"missing target Kind",
			PatchDefinition{Target: PatchTarget{Name: "my-deploy"}, Patch: validJSONPatch},
			true,
		},
		{
			"missing target Name",
			PatchDefinition{Target: PatchTarget{Kind: "Deployment"}, Patch: validJSONPatch},
			true,
		},
		{
			"missing patch content",
			PatchDefinition{Target: PatchTarget{Kind: "Deployment", Name: "my-deploy"}},
			true,
		},
		{
			"invalid patch type",
			PatchDefinition{Target: PatchTarget{Kind: "Deployment", Name: "my-deploy"}, Patch: validJSONPatch, Type: "unsupported"},
			true,
		},
		{
			"valid json patch (default type)",
			PatchDefinition{Target: PatchTarget{Kind: "Deployment", Name: "my-deploy"}, Patch: validJSONPatch},
			false,
		},
		{
			"valid json patch (explicit type)",
			PatchDefinition{Target: PatchTarget{Kind: "Deployment", Name: "my-deploy"}, Patch: validJSONPatch, Type: "json"},
			false,
		},
		{
			"valid strategic merge patch",
			PatchDefinition{Target: PatchTarget{Kind: "Deployment", Name: "my-deploy"}, Patch: validStrategicPatch, Type: "strategic"},
			false,
		},
		{
			"empty strategic merge patch",
			PatchDefinition{Target: PatchTarget{Kind: "Deployment", Name: "my-deploy"}, Patch: "{}", Type: "strategic"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBaseKurelConfig()
			cfg.Patches = []PatchDefinition{tt.patch}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_JSONPatch_Operations(t *testing.T) {
	validTarget := PatchTarget{Kind: "Deployment", Name: "my-deploy"}

	tests := []struct {
		name    string
		patch   string
		wantErr bool
	}{
		{
			"add op with value",
			"- op: add\n  path: /spec/replicas\n  value: 3\n",
			false,
		},
		{
			"remove op (no value needed)",
			"- op: remove\n  path: /spec/selector\n",
			false,
		},
		{
			"move op with from",
			"- op: move\n  path: /spec/newField\n  from: /spec/oldField\n",
			false,
		},
		{
			"copy op with from",
			"- op: copy\n  path: /spec/newField\n  from: /spec/oldField\n",
			false,
		},
		{
			"test op with value",
			"- op: test\n  path: /spec/replicas\n  value: 3\n",
			false,
		},
		{
			"invalid op",
			"- op: badop\n  path: /spec/replicas\n",
			true,
		},
		{
			"path without leading slash",
			"- op: remove\n  path: spec/selector\n",
			true,
		},
		{
			"missing op field",
			"- path: /spec/replicas\n  value: 3\n",
			true,
		},
		{
			"add op missing value",
			"- op: add\n  path: /spec/replicas\n",
			true,
		},
		{
			"replace op with value",
			"- op: replace\n  path: /spec/replicas\n  value: 5\n",
			false,
		},
		{
			"replace op missing value",
			"- op: replace\n  path: /spec/replicas\n",
			true,
		},
		{
			"move op missing from",
			"- op: move\n  path: /spec/newField\n",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBaseKurelConfig()
			cfg.Patches = []PatchDefinition{{Target: validTarget, Patch: tt.patch}}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_BuildConfig(t *testing.T) {
	tests := []struct {
		name    string
		build   BuildConfig
		wantErr bool
	}{
		{"empty build config", BuildConfig{}, false},
		{"format directory", BuildConfig{Format: "directory"}, false},
		{"format oci without registry", BuildConfig{Format: "oci"}, true},
		{"format oci without repository", BuildConfig{Format: "oci", Registry: "registry.example.com"}, true},
		{"format oci valid", BuildConfig{Format: "oci", Registry: "registry.example.com", Repository: "myrepo"}, false},
		{"invalid format", BuildConfig{Format: "tarball"}, true},
		{"valid outputDir with existing parent", BuildConfig{OutputDir: "/tmp/output"}, false},
		{"invalid outputDir with nonexistent parent", BuildConfig{OutputDir: "/tmp/nonexistent-parent-99999/output"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBaseKurelConfig()
			cfg.Build = &tt.build
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Extension_WithPatches(t *testing.T) {
	validJSONPatch := "- op: replace\n  path: /spec/replicas\n  value: 3\n"
	t.Run("extension with valid patch", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Extensions = []Extension{
			{
				Name: "my-ext",
				Patches: []PatchDefinition{
					{Target: PatchTarget{Kind: "Deployment", Name: "my-deploy"}, Patch: validJSONPatch},
				},
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("extension with invalid patch", func(t *testing.T) {
		cfg := validBaseKurelConfig()
		cfg.Extensions = []Extension{
			{
				Name: "my-ext",
				Patches: []PatchDefinition{
					{Target: PatchTarget{Kind: "Deployment", Name: "my-deploy"}, Patch: validJSONPatch, Type: "invalid"},
				},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid extension patch type")
		}
	})
}
