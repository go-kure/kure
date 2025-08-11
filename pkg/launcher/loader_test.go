package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kure/kure/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageLoader(t *testing.T) {
	// Create a test logger
	log := logger.Noop()
	loader := NewPackageLoader(log)

	t.Run("LoadDefinition", func(t *testing.T) {
		t.Run("valid package", func(t *testing.T) {
			// Create test package structure
			tmpDir := t.TempDir()
			setupTestPackage(t, tmpDir)

			// Load the package
			ctx := context.Background()
			opts := DefaultOptions()
			def, err := loader.LoadDefinition(ctx, tmpDir, opts)

			require.NoError(t, err)
			require.NotNil(t, def)
			assert.Equal(t, "test-package", def.Metadata.Name)
			assert.Equal(t, "1.0.0", def.Metadata.Version)
			assert.NotEmpty(t, def.Parameters)
			assert.NotEmpty(t, def.Resources)
		})

		t.Run("missing package", func(t *testing.T) {
			ctx := context.Background()
			opts := DefaultOptions()
			def, err := loader.LoadDefinition(ctx, "/nonexistent/path", opts)

			assert.Error(t, err)
			assert.Nil(t, def)
		})

		t.Run("package with issues", func(t *testing.T) {
			// Create package with some invalid files
			tmpDir := t.TempDir()
			setupPackageWithIssues(t, tmpDir)

			ctx := context.Background()
			opts := DefaultOptions()
			def, _ := loader.LoadDefinition(ctx, tmpDir, opts)

			// Should return partial definition, possibly with warnings
			assert.NotNil(t, def)
			// The package should still load even with invalid parameters
			assert.Equal(t, "package-with-issues", def.Metadata.Name)
		})
	})

	t.Run("LoadResources", func(t *testing.T) {
		t.Run("valid resources", func(t *testing.T) {
			tmpDir := t.TempDir()
			setupResourceFiles(t, tmpDir)

			ctx := context.Background()
			opts := DefaultOptions()
			resources, err := loader.LoadResources(ctx, tmpDir, opts)

			require.NoError(t, err)
			assert.NotEmpty(t, resources)

			// Check first resource
			if len(resources) > 0 {
				res := resources[0]
				assert.Equal(t, "apps/v1", res.APIVersion)
				assert.Equal(t, "Deployment", res.Kind)
				assert.Equal(t, "test-app", res.GetName())
			}
		})

		t.Run("invalid YAML", func(t *testing.T) {
			tmpDir := t.TempDir()
			invalidYAML := filepath.Join(tmpDir, "invalid.yaml")
			err := os.WriteFile(invalidYAML, []byte("invalid: yaml: content:"), 0644)
			require.NoError(t, err)

			ctx := context.Background()
			opts := DefaultOptions()
			resources, err := loader.LoadResources(ctx, tmpDir, opts)

			// Should load as template data since template loading is enabled
			assert.NoError(t, err)
			assert.Len(t, resources, 1) // One resource loaded as template

			// Verify it has template data
			assert.NotEmpty(t, resources[0].TemplateData)
			assert.Contains(t, string(resources[0].TemplateData), "invalid: yaml: content:")
		})
	})

	t.Run("LoadPatches", func(t *testing.T) {
		t.Run("valid patches", func(t *testing.T) {
			tmpDir := t.TempDir()
			patchDir := filepath.Join(tmpDir, "patches")
			require.NoError(t, os.MkdirAll(patchDir, 0755))

			// Create patch file
			patchContent := `# kurel:enabled: ${feature.enabled}
# kurel:description: Test patch
# kurel:requires: base-patch

[deployment.test-app]
spec.replicas: 3`
			patchFile := filepath.Join(patchDir, "scale.kpatch")
			err := os.WriteFile(patchFile, []byte(patchContent), 0644)
			require.NoError(t, err)

			ctx := context.Background()
			opts := DefaultOptions()
			patches, err := loader.LoadPatches(ctx, tmpDir, opts)

			require.NoError(t, err)
			require.Len(t, patches, 1)

			patch := patches[0]
			assert.Equal(t, "scale", patch.Name)
			assert.NotNil(t, patch.Metadata)
			assert.Equal(t, "${feature.enabled}", patch.Metadata.Enabled)
			assert.Equal(t, "Test patch", patch.Metadata.Description)
			assert.Contains(t, patch.Metadata.Requires, "base-patch")
		})

		t.Run("no patches directory", func(t *testing.T) {
			tmpDir := t.TempDir()

			ctx := context.Background()
			opts := DefaultOptions()
			patches, err := loader.LoadPatches(ctx, tmpDir, opts)

			assert.NoError(t, err)
			assert.Empty(t, patches)
		})
	})

	t.Run("patch uniqueness", func(t *testing.T) {
		loader := &packageLoader{logger: log}

		patches := []Patch{
			{Name: "patch1"},
			{Name: "patch2"},
			{Name: "patch1"}, // Duplicate
		}

		err := loader.validatePatchUniqueness(patches)
		assert.Error(t, err)
	})
}

// Test helpers

func setupTestPackage(t *testing.T, dir string) {
	// Create kurel.yaml
	kurelYAML := `name: test-package
version: 1.0.0
appVersion: 2.0.0
description: Test package`
	err := os.WriteFile(filepath.Join(dir, "kurel.yaml"), []byte(kurelYAML), 0644)
	require.NoError(t, err)

	// Create parameters.yaml
	paramsYAML := `app:
  name: test-app
  replicas: 2
feature:
  enabled: true`
	err = os.WriteFile(filepath.Join(dir, "parameters.yaml"), []byte(paramsYAML), 0644)
	require.NoError(t, err)

	// Create resources directory with a deployment
	resourceDir := filepath.Join(dir, "resources")
	require.NoError(t, os.MkdirAll(resourceDir, 0755))

	deploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: app
        image: nginx:latest`
	err = os.WriteFile(filepath.Join(resourceDir, "deployment.yaml"), []byte(deploymentYAML), 0644)
	require.NoError(t, err)
}

func setupPackageWithIssues(t *testing.T, dir string) {
	// Create valid kurel.yaml
	kurelYAML := `name: package-with-issues
version: 1.0.0`
	err := os.WriteFile(filepath.Join(dir, "kurel.yaml"), []byte(kurelYAML), 0644)
	require.NoError(t, err)

	// Create invalid parameters.yaml (invalid YAML)
	err = os.WriteFile(filepath.Join(dir, "parameters.yaml"), []byte("invalid: yaml: content:"), 0644)
	require.NoError(t, err)

	// Create resources directory
	resourceDir := filepath.Join(dir, "resources")
	require.NoError(t, os.MkdirAll(resourceDir, 0755))

	// Add one valid resource
	validYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value`
	err = os.WriteFile(filepath.Join(resourceDir, "configmap.yaml"), []byte(validYAML), 0644)
	require.NoError(t, err)
}

func setupResourceFiles(t *testing.T, dir string) {
	// Create resources directory
	resourceDir := filepath.Join(dir, "resources")
	require.NoError(t, os.MkdirAll(resourceDir, 0755))

	// Create a deployment
	deploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: app
        image: nginx:latest`
	err := os.WriteFile(filepath.Join(resourceDir, "deployment.yaml"), []byte(deploymentYAML), 0644)
	require.NoError(t, err)

	// Create a service
	serviceYAML := `apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: default
spec:
  selector:
    app: test
  ports:
  - port: 80
    targetPort: 8080`
	err = os.WriteFile(filepath.Join(resourceDir, "service.yaml"), []byte(serviceYAML), 0644)
	require.NoError(t, err)
}

func TestHelperFunctions(t *testing.T) {
	t.Run("isYAMLFile", func(t *testing.T) {
		assert.True(t, isYAMLFile("test.yaml"))
		assert.True(t, isYAMLFile("test.yml"))
		assert.True(t, isYAMLFile("TEST.YAML"))
		assert.False(t, isYAMLFile("test.json"))
		assert.False(t, isYAMLFile("test.txt"))
	})

	t.Run("isPatchFile", func(t *testing.T) {
		assert.True(t, isPatchFile("test.kpatch"))
		assert.True(t, isPatchFile("test.patch"))
		assert.True(t, isPatchFile("patches/test.yaml"))
		assert.False(t, isPatchFile("resources/test.yaml"))
		assert.False(t, isPatchFile("test.json"))
	})
}
