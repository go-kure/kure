package stack_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/stack"
	_ "github.com/go-kure/kure/pkg/stack/generators/appworkload" // Register AppWorkload
	_ "github.com/go-kure/kure/pkg/stack/generators/fluxhelm"    // Register FluxHelm
)

func TestApplicationWrapper(t *testing.T) {
	t.Run("Unmarshal AppWorkload", func(t *testing.T) {
		yamlContent := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: test-app
  namespace: test-ns
spec:
  workload: Deployment
  replicas: 2
  containers:
    - name: app
      image: nginx:latest
`

		var wrapper stack.ApplicationWrapper
		err := yaml.Unmarshal([]byte(yamlContent), &wrapper)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if wrapper.APIVersion != "generators.gokure.dev/v1alpha1" {
			t.Errorf("Expected apiVersion generators.gokure.dev/v1alpha1, got %s", wrapper.APIVersion)
		}

		if wrapper.Kind != "AppWorkload" {
			t.Errorf("Expected kind AppWorkload, got %s", wrapper.Kind)
		}

		if wrapper.Metadata.Name != "test-app" {
			t.Errorf("Expected name test-app, got %s", wrapper.Metadata.Name)
		}

		if wrapper.Metadata.Namespace != "test-ns" {
			t.Errorf("Expected namespace test-ns, got %s", wrapper.Metadata.Namespace)
		}

		if wrapper.Spec == nil {
			t.Error("Expected spec to be populated")
		}
	})

	t.Run("Unmarshal FluxHelm", func(t *testing.T) {
		yamlContent := `
apiVersion: generators.gokure.dev/v1alpha1
kind: FluxHelm
metadata:
  name: postgres
  namespace: database
spec:
  chart:
    name: postgresql
    version: 12.0.0
  source:
    type: HelmRepository
    url: https://charts.bitnami.com/bitnami
`

		var wrapper stack.ApplicationWrapper
		err := yaml.Unmarshal([]byte(yamlContent), &wrapper)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if wrapper.Kind != "FluxHelm" {
			t.Errorf("Expected kind FluxHelm, got %s", wrapper.Kind)
		}

		if wrapper.Metadata.Name != "postgres" {
			t.Errorf("Expected name postgres, got %s", wrapper.Metadata.Name)
		}

		if wrapper.Spec == nil {
			t.Error("Expected spec to be populated")
		}
	})

	t.Run("Missing Required Fields", func(t *testing.T) {
		yamlContent := `
metadata:
  name: test-app
spec:
  replicas: 2
`

		var wrapper stack.ApplicationWrapper
		err := yaml.Unmarshal([]byte(yamlContent), &wrapper)
		if err == nil {
			t.Error("Expected error for missing apiVersion and kind")
		}
	})

	t.Run("Unknown Generator Type", func(t *testing.T) {
		yamlContent := `
apiVersion: generators.gokure.dev/v1alpha1
kind: UnknownGenerator
metadata:
  name: test
spec:
  foo: bar
`

		var wrapper stack.ApplicationWrapper
		err := yaml.Unmarshal([]byte(yamlContent), &wrapper)
		if err == nil {
			t.Error("Expected error for unknown generator type")
		}
	})

	t.Run("ToApplication", func(t *testing.T) {
		yamlContent := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: test-app
  namespace: test-ns
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: app
      image: nginx:latest
`

		var wrapper stack.ApplicationWrapper
		err := yaml.Unmarshal([]byte(yamlContent), &wrapper)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		app := wrapper.ToApplication()
		if app == nil {
			t.Fatal("ToApplication returned nil")
		}

		if app.Name != "test-app" {
			t.Errorf("Expected app name test-app, got %s", app.Name)
		}

		if app.Namespace != "test-ns" {
			t.Errorf("Expected app namespace test-ns, got %s", app.Namespace)
		}

		if app.Config == nil {
			t.Error("Expected app.Config to be populated")
		}
	})

	t.Run("Marshal", func(t *testing.T) {
		wrapper := stack.ApplicationWrapper{
			APIVersion: "generators.gokure.dev/v1alpha1",
			Kind:       "AppWorkload",
			Metadata: stack.ApplicationMetadata{
				Name:      "test-app",
				Namespace: "test-ns",
				Labels: map[string]string{
					"app": "test",
				},
			},
		}

		data, err := yaml.Marshal(&wrapper)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		// Unmarshal back to verify
		var result map[string]interface{}
		err = yaml.Unmarshal(data, &result)
		if err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if result["apiVersion"] != "generators.gokure.dev/v1alpha1" {
			t.Error("apiVersion not preserved in marshaling")
		}

		if result["kind"] != "AppWorkload" {
			t.Error("kind not preserved in marshaling")
		}

		metadata, ok := result["metadata"].(map[string]interface{})
		if !ok {
			t.Fatal("metadata not found or wrong type")
		}

		if metadata["name"] != "test-app" {
			t.Error("metadata.name not preserved in marshaling")
		}
	})
}

func TestApplicationWrappers(t *testing.T) {
	t.Run("Multiple Applications", func(t *testing.T) {
		yamlContent := `
- apiVersion: generators.gokure.dev/v1alpha1
  kind: AppWorkload
  metadata:
    name: app1
    namespace: ns1
  spec:
    workload: Deployment
    replicas: 1
    containers:
      - name: app
        image: nginx:latest
- apiVersion: generators.gokure.dev/v1alpha1
  kind: FluxHelm
  metadata:
    name: app2
    namespace: ns2
  spec:
    chart:
      name: postgresql
      version: 12.0.0
    source:
      type: HelmRepository
      url: https://charts.bitnami.com/bitnami
`

		var wrappers stack.ApplicationWrappers
		err := yaml.Unmarshal([]byte(yamlContent), &wrappers)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if len(wrappers) != 2 {
			t.Fatalf("Expected 2 applications, got %d", len(wrappers))
		}

		if wrappers[0].Kind != "AppWorkload" {
			t.Errorf("Expected first app kind AppWorkload, got %s", wrappers[0].Kind)
		}

		if wrappers[1].Kind != "FluxHelm" {
			t.Errorf("Expected second app kind FluxHelm, got %s", wrappers[1].Kind)
		}

		apps := wrappers.ToApplications()
		if len(apps) != 2 {
			t.Fatalf("Expected 2 applications from ToApplications, got %d", len(apps))
		}

		if apps[0].Name != "app1" {
			t.Errorf("Expected first app name app1, got %s", apps[0].Name)
		}

		if apps[1].Name != "app2" {
			t.Errorf("Expected second app name app2, got %s", apps[1].Name)
		}
	})
}