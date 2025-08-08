package kurelpackage_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/stack"
	_ "github.com/go-kure/kure/pkg/stack/generators/kurelpackage" // Register generator
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
}