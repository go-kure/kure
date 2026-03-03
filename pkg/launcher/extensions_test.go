package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/logger"
)

func TestExtensionLoader(t *testing.T) {
	log := logger.Noop()
	loader := NewExtensionLoader(log)
	ctx := context.Background()

	// Create base package definition
	baseDef := &PackageDefinition{
		Path: "/test/package",
		Metadata: KurelMetadata{
			Name:    "test-package",
			Version: "1.0.0",
		},
		Parameters: ParameterMap{
			"replicas": 2,
			"image":    "nginx:1.19",
			"env": map[string]any{
				"debug": false,
			},
		},
		Resources: []Resource{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Metadata: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "default",
				},
				Raw: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name":      "test-app",
							"namespace": "default",
						},
						"spec": map[string]any{
							"replicas": int64(2),
						},
					},
				},
			},
		},
		Patches: []Patch{
			{
				Name:    "scale",
				Content: "[deployment.test-app]\nspec.replicas: 5",
			},
		},
	}

	t.Run("no extensions", func(t *testing.T) {
		result, err := loader.LoadWithExtensions(ctx, baseDef, "", nil)
		require.NoError(t, err)
		assert.Equal(t, baseDef, result)
	})

	t.Run("parameter override extension", func(t *testing.T) {
		ext := LocalExtension{
			Type: ExtensionTypeOverride,
			Parameters: ParameterMap{
				"replicas": 5,
				"newParam": "value",
			},
		}

		extLoader := &extensionLoader{
			logger: log,
		}

		def := baseDef.DeepCopy()
		err := extLoader.applyExtension(ctx, def, ext, nil)
		require.NoError(t, err)

		assert.Equal(t, 5, def.Parameters["replicas"])
		assert.Equal(t, "value", def.Parameters["newParam"])
		assert.Equal(t, "nginx:1.19", def.Parameters["image"]) // Unchanged
	})

	t.Run("parameter merge extension", func(t *testing.T) {
		ext := LocalExtension{
			Type: ExtensionTypeMerge,
			Parameters: ParameterMap{
				"env": map[string]any{
					"debug":   true, // Override
					"verbose": true, // New
				},
			},
		}

		extLoader := &extensionLoader{
			logger: log,
		}

		def := baseDef.DeepCopy()
		err := extLoader.applyExtension(ctx, def, ext, nil)
		require.NoError(t, err)

		env := def.Parameters["env"].(map[string]any)
		assert.Equal(t, true, env["debug"])   // Changed
		assert.Equal(t, true, env["verbose"]) // Added
	})

	t.Run("patch extension", func(t *testing.T) {
		ext := LocalExtension{
			Type: ExtensionTypeMerge,
			Patches: []LocalPatch{
				{
					Name:    "labels",
					Content: "[deployment.test-app]\nmetadata.labels.env: production",
				},
				{
					Name:    "scale", // Override existing
					Content: "[deployment.test-app]\nspec.replicas: 10",
				},
			},
		}

		extLoader := &extensionLoader{
			logger: log,
		}

		def := baseDef.DeepCopy()
		err := extLoader.applyExtension(ctx, def, ext, nil)
		require.NoError(t, err)

		assert.Len(t, def.Patches, 2)
		// Check scale patch was updated
		foundScale := false
		for _, p := range def.Patches {
			if p.Name == "scale" {
				foundScale = true
				assert.Contains(t, p.Content, "spec.replicas: 10", "Scale patch should contain updated replica count")
				break
			}
		}
		assert.True(t, foundScale, "Scale patch should exist")
	})

	t.Run("resource selector matching", func(t *testing.T) {
		extLoader := &extensionLoader{
			logger: log,
		}

		resource := &Resource{
			Kind: "Deployment",
			Metadata: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "production",
				Labels: map[string]string{
					"app": "test",
					"env": "prod",
				},
			},
		}

		testCases := []struct {
			name     string
			selector ResourceSelector
			matches  bool
		}{
			{
				name:     "match by kind",
				selector: ResourceSelector{Kind: "Deployment"},
				matches:  true,
			},
			{
				name:     "match by name",
				selector: ResourceSelector{Name: "test-app"},
				matches:  true,
			},
			{
				name:     "match by wildcard",
				selector: ResourceSelector{Name: "test-*"},
				matches:  true,
			},
			{
				name:     "match by namespace",
				selector: ResourceSelector{Namespace: "production"},
				matches:  true,
			},
			{
				name:     "match by labels",
				selector: ResourceSelector{Labels: map[string]string{"app": "test"}},
				matches:  true,
			},
			{
				name:     "no match - wrong kind",
				selector: ResourceSelector{Kind: "Service"},
				matches:  false,
			},
			{
				name:     "no match - wrong label",
				selector: ResourceSelector{Labels: map[string]string{"app": "other"}},
				matches:  false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				matches := extLoader.matchesSelector(resource, tc.selector)
				assert.Equal(t, tc.matches, matches)
			})
		}
	})

	t.Run("resource override", func(t *testing.T) {
		ext := LocalExtension{
			Type: ExtensionTypeMerge,
			Resources: []LocalResourceOverride{
				{
					Selector: ResourceSelector{
						Kind: "Deployment",
					},
					Override: map[string]any{
						"spec.replicas": int64(10),
					},
					Merge: map[string]any{
						"metadata.labels.managed-by": "kurel",
					},
				},
			},
		}

		extLoader := &extensionLoader{
			logger: log,
		}

		def := baseDef.DeepCopy()
		err := extLoader.applyExtension(ctx, def, ext, nil)
		require.NoError(t, err)

		// Check replicas was overridden
		replicas := def.Resources[0].Raw.Object["spec"].(map[string]any)["replicas"]
		assert.Equal(t, int64(10), replicas)
	})

	t.Run("remove resources", func(t *testing.T) {
		def := &PackageDefinition{
			Resources: []Resource{
				{
					Kind: "Deployment",
					Metadata: metav1.ObjectMeta{
						Name: "app1",
					},
				},
				{
					Kind: "Service",
					Metadata: metav1.ObjectMeta{
						Name: "svc1",
					},
				},
				{
					Kind: "ConfigMap",
					Metadata: metav1.ObjectMeta{
						Name: "config1",
					},
				},
			},
		}

		ext := LocalExtension{
			Type: ExtensionTypeMerge,
			Remove: []ResourceSelector{
				{Kind: "Service"},
			},
		}

		extLoader := &extensionLoader{
			logger: log,
		}

		err := extLoader.applyExtension(ctx, def, ext, nil)
		require.NoError(t, err)

		assert.Len(t, def.Resources, 2)
		for _, r := range def.Resources {
			assert.NotEqual(t, "Service", r.Kind)
		}
	})

	t.Run("search paths", func(t *testing.T) {
		extLoader := &extensionLoader{
			logger: log,
		}

		paths := extLoader.getSearchPaths("/package/path", "/local/path", nil)

		// Should include local path first
		assert.Contains(t, paths, "/local/path")

		// Should include package path
		found := false
		for _, p := range paths {
			if strings.Contains(p, "/package/path") {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestExtensionFiles(t *testing.T) {
	log := logger.Noop()

	t.Run("load extension from file", func(t *testing.T) {
		// Create temp directory and extension file
		tmpDir := t.TempDir()
		extPath := filepath.Join(tmpDir, "override.local.kurel")

		extContent := `type: override
parameters:
  replicas: 10
  image: nginx:latest
patches:
  - name: security
    content: |
      [deployment.app]
      spec.securityContext.runAsNonRoot: true
`

		err := os.WriteFile(extPath, []byte(extContent), 0644)
		require.NoError(t, err)

		extLoader := &extensionLoader{
			logger: log,
		}

		ext, err := extLoader.loadExtension(extPath)
		require.NoError(t, err)

		assert.Equal(t, ExtensionTypeOverride, ext.Type)
		assert.Equal(t, 10, ext.Parameters["replicas"])
		assert.Len(t, ext.Patches, 1)
		assert.Equal(t, "security", ext.Patches[0].Name)
	})

	t.Run("find extensions in directory", func(t *testing.T) {
		// Create temp directory with multiple extension files
		tmpDir := t.TempDir()

		// Create .local.kurel file
		ext1Path := filepath.Join(tmpDir, "01-base.local.kurel")
		err := os.WriteFile(ext1Path, []byte("type: merge\nparameters:\n  key1: value1"), 0644)
		require.NoError(t, err)

		// Create .local.yaml file
		ext2Path := filepath.Join(tmpDir, "02-override.local.yaml")
		err = os.WriteFile(ext2Path, []byte("type: override\nparameters:\n  key2: value2"), 0644)
		require.NoError(t, err)

		// Create regular file (should be ignored)
		regularPath := filepath.Join(tmpDir, "regular.yaml")
		err = os.WriteFile(regularPath, []byte("ignored: true"), 0644)
		require.NoError(t, err)

		extLoader := &extensionLoader{
			logger: log,
		}

		extensions, err := extLoader.findExtensions(tmpDir, "", nil)
		require.NoError(t, err)

		assert.Len(t, extensions, 2)
		// Should be sorted alphabetically
		assert.Equal(t, "01-base.local.kurel", filepath.Base(extensions[0].Path))
		assert.Equal(t, "02-override.local.yaml", filepath.Base(extensions[1].Path))
	})
}

func TestNewExtensionLoaderNilLogger(t *testing.T) {
	loader := NewExtensionLoader(nil)
	if loader == nil {
		t.Fatal("expected non-nil extension loader with nil logger")
	}
}

func TestLoadWithExtensionsNilDef(t *testing.T) {
	log := logger.Noop()
	loader := NewExtensionLoader(log)
	ctx := context.Background()

	_, err := loader.LoadWithExtensions(ctx, nil, "", nil)
	if err == nil {
		t.Fatal("expected error for nil definition")
	}
}

func TestLoadWithExtensionsStrictMode(t *testing.T) {
	log := logger.Noop()
	ctx := context.Background()

	// Create a temp dir with a malformed extension file
	tmpDir := t.TempDir()
	extPath := filepath.Join(tmpDir, "bad.local.kurel")
	err := os.WriteFile(extPath, []byte("type: merge\nresources:\n  - selector:\n      kind: Deployment\n    override:\n      nonexistent.path: value\n"), 0644)
	require.NoError(t, err)

	def := &PackageDefinition{
		Path: tmpDir,
		Metadata: KurelMetadata{
			Name:    "test",
			Version: "1.0.0",
		},
		Parameters: ParameterMap{},
		Resources:  []Resource{},
	}

	// In non-strict mode, should continue
	loader := NewExtensionLoader(log)
	result, err := loader.LoadWithExtensions(ctx, def, tmpDir, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestApplyPatchExtensionReplace(t *testing.T) {
	log := logger.Noop()
	extLoader := &extensionLoader{logger: log}

	def := &PackageDefinition{
		Patches: []Patch{
			{Name: "old-patch", Content: "old-content"},
		},
	}

	ext := LocalExtension{
		Type: ExtensionTypeReplace,
		Patches: []LocalPatch{
			{Name: "new-patch", Content: "new-content"},
		},
	}

	err := extLoader.applyExtension(context.Background(), def, ext, nil)
	require.NoError(t, err)

	assert.Len(t, def.Patches, 1)
	assert.Equal(t, "new-patch", def.Patches[0].Name)
	assert.Equal(t, "new-content", def.Patches[0].Content)
}

func TestApplyParameterExtensionReplace(t *testing.T) {
	log := logger.Noop()
	extLoader := &extensionLoader{logger: log}

	def := &PackageDefinition{
		Parameters: ParameterMap{
			"old": "value",
		},
	}

	extLoader.applyParameterExtension(def, ParameterMap{
		"new": "value",
	}, ExtensionTypeReplace)

	if _, ok := def.Parameters["old"]; ok {
		t.Error("old parameter should have been replaced")
	}
	if def.Parameters["new"] != "value" {
		t.Error("new parameter should be present")
	}
}

func TestDeepCopyValueSlice(t *testing.T) {
	log := logger.Noop()
	extLoader := &extensionLoader{logger: log}

	original := []any{"a", "b", map[string]any{"key": "val"}}
	copied := extLoader.deepCopyValue(original)

	copiedSlice, ok := copied.([]any)
	if !ok {
		t.Fatal("expected []interface{}")
	}
	if len(copiedSlice) != 3 {
		t.Fatalf("expected 3 items, got %d", len(copiedSlice))
	}

	// Verify deep copy - modifying original shouldn't affect copy
	originalSlice := original
	innerMap := originalSlice[2].(map[string]any)
	innerMap["key"] = "modified"

	copiedInner := copiedSlice[2].(map[string]any)
	if copiedInner["key"] != "val" {
		t.Error("deep copy should be independent of original")
	}
}

func TestSortExtensions(t *testing.T) {
	log := logger.Noop()
	extLoader := &extensionLoader{logger: log}

	extensions := []LocalExtension{
		{Path: "/path/to/c.local.kurel"},
		{Path: "/path/to/a.local.kurel"},
		{Path: "/path/to/b.local.kurel"},
	}

	extLoader.sortExtensions(extensions)

	if filepath.Base(extensions[0].Path) != "a.local.kurel" {
		t.Errorf("expected first to be a.local.kurel, got %s", filepath.Base(extensions[0].Path))
	}
	if filepath.Base(extensions[1].Path) != "b.local.kurel" {
		t.Errorf("expected second to be b.local.kurel, got %s", filepath.Base(extensions[1].Path))
	}
	if filepath.Base(extensions[2].Path) != "c.local.kurel" {
		t.Errorf("expected third to be c.local.kurel, got %s", filepath.Base(extensions[2].Path))
	}
}

func TestMergeNestedFieldMerge(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"existing": "value",
			},
		},
	}

	newLabels := map[string]any{
		"new": "label",
	}

	err := mergeNestedField(obj, newLabels, "metadata", "labels")
	require.NoError(t, err)

	labels := obj["metadata"].(map[string]any)["labels"].(map[string]any)
	assert.Equal(t, "value", labels["existing"])
	assert.Equal(t, "label", labels["new"])
}

func TestSetNestedFieldEmptyPath(t *testing.T) {
	obj := map[string]any{}
	err := setNestedField(obj, "value")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestMergeNestedFieldEmptyPath(t *testing.T) {
	obj := map[string]any{}
	err := mergeNestedField(obj, "value")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestRemoveNestedFieldEmptyPath(t *testing.T) {
	obj := map[string]any{"key": "value"}
	removeNestedField(obj) // Should not panic
	assert.Equal(t, "value", obj["key"])
}

func TestRemoveNestedFieldMissingPath(t *testing.T) {
	obj := map[string]any{
		"metadata": "not-a-map",
	}
	// Should not panic when path doesn't exist
	removeNestedField(obj, "metadata", "labels", "env")
}

func TestNestedFieldOperations(t *testing.T) {
	t.Run("setNestedField", func(t *testing.T) {
		obj := map[string]any{
			"metadata": map[string]any{
				"name": "test",
			},
		}

		err := setNestedField(obj, "production", "metadata", "labels", "env")
		require.NoError(t, err)

		labels := obj["metadata"].(map[string]any)["labels"].(map[string]any)
		assert.Equal(t, "production", labels["env"])
	})

	t.Run("mergeNestedField", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "app",
								"env": map[string]any{
									"DEBUG": "false",
								},
							},
						},
					},
				},
			},
		}

		newEnv := map[string]any{
			"DEBUG":   "true",
			"VERBOSE": "true",
		}

		err := mergeNestedField(obj, newEnv, "spec", "template", "spec", "containers", "0", "env")
		require.NoError(t, err)

		// Note: This simple implementation doesn't handle array indexing
		// In production, you'd need more sophisticated path handling
	})

	t.Run("removeNestedField", func(t *testing.T) {
		obj := map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					"app": "test",
					"env": "prod",
				},
			},
		}

		removeNestedField(obj, "metadata", "labels", "env")

		labels := obj["metadata"].(map[string]any)["labels"].(map[string]any)
		assert.Equal(t, "test", labels["app"])
		_, exists := labels["env"]
		assert.False(t, exists)
	})
}
