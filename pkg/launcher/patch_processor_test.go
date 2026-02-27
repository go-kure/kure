package launcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/logger"
)

func TestPatchProcessor(t *testing.T) {
	log := logger.Noop()
	resolver := NewResolver(log)
	processor := NewPatchProcessor(log, resolver)
	ctx := context.Background()

	t.Run("ResolveDependencies", func(t *testing.T) {
		t.Run("simple enable", func(t *testing.T) {
			patches := []Patch{
				{
					Name: "patch1",
					Metadata: &PatchMetadata{
						Enabled: "true",
					},
				},
				{
					Name: "patch2",
					Metadata: &PatchMetadata{
						Enabled: "false",
					},
				},
				{
					Name: "patch3", // No metadata, enabled by default
				},
			}

			params := ParameterMap{}
			resolved, err := processor.ResolveDependencies(ctx, patches, params)

			require.NoError(t, err)
			assert.Len(t, resolved, 2) // patch1 and patch3

			names := []string{}
			for _, p := range resolved {
				names = append(names, p.Name)
			}
			assert.Contains(t, names, "patch1")
			assert.Contains(t, names, "patch3")
			assert.NotContains(t, names, "patch2")
		})

		t.Run("conditional enable", func(t *testing.T) {
			patches := []Patch{
				{
					Name: "feature-patch",
					Metadata: &PatchMetadata{
						Enabled: "${feature.enabled}",
					},
				},
				{
					Name: "env-patch",
					Metadata: &PatchMetadata{
						Enabled: "${env}",
					},
				},
			}

			params := ParameterMap{
				"feature": map[string]interface{}{
					"enabled": true,
				},
				"env": "prod",
			}

			resolved, err := processor.ResolveDependencies(ctx, patches, params)

			require.NoError(t, err)
			assert.Len(t, resolved, 2) // Both enabled
		})

		t.Run("dependency chain", func(t *testing.T) {
			patches := []Patch{
				{
					Name: "base",
				},
				{
					Name: "middle",
					Metadata: &PatchMetadata{
						Requires: []string{"base"},
					},
				},
				{
					Name: "top",
					Metadata: &PatchMetadata{
						Requires: []string{"middle"},
					},
				},
			}

			params := ParameterMap{}
			resolved, err := processor.ResolveDependencies(ctx, patches, params)

			require.NoError(t, err)
			assert.Len(t, resolved, 3)

			// Check order - base should come before middle, middle before top
			var baseIdx, middleIdx, topIdx int
			for i, p := range resolved {
				switch p.Name {
				case "base":
					baseIdx = i
				case "middle":
					middleIdx = i
				case "top":
					topIdx = i
				}
			}
			assert.Less(t, baseIdx, middleIdx)
			assert.Less(t, middleIdx, topIdx)
		})

		t.Run("missing dependency", func(t *testing.T) {
			patches := []Patch{
				{
					Name: "patch1",
					Metadata: &PatchMetadata{
						Requires: []string{"missing"},
					},
				},
			}

			params := ParameterMap{}
			_, err := processor.ResolveDependencies(ctx, patches, params)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "missing")
		})

		t.Run("conflict detection", func(t *testing.T) {
			patches := []Patch{
				{
					Name: "patch1",
				},
				{
					Name: "patch2",
					Metadata: &PatchMetadata{
						Conflicts: []string{"patch1"},
					},
				},
			}

			params := ParameterMap{}
			_, err := processor.ResolveDependencies(ctx, patches, params)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "conflict")
		})
	})

	t.Run("ApplyPatches", func(t *testing.T) {
		t.Run("simple patch", func(t *testing.T) {
			// Create a package definition with a deployment
			def := &PackageDefinition{
				Resources: []Resource{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Metadata: metav1.ObjectMeta{
							Name:      "test-app",
							Namespace: "default",
						},
						Raw: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "apps/v1",
								"kind":       "Deployment",
								"metadata": map[string]interface{}{
									"name":      "test-app",
									"namespace": "default",
								},
								"spec": map[string]interface{}{
									"replicas": float64(1),
								},
							},
						},
					},
				},
			}

			patches := []Patch{
				{
					Name:    "scale",
					Content: `spec.replicas: 3`,
				},
			}

			params := ParameterMap{}
			result, err := processor.ApplyPatches(ctx, def, patches, params)

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check that the original is unchanged
			assert.Equal(t, float64(1), def.Resources[0].Raw.Object["spec"].(map[string]interface{})["replicas"])

			// Check that the result has the patch applied
			spec := result.Resources[0].Raw.Object["spec"].(map[string]interface{})
			assert.Equal(t, 3, spec["replicas"])
		})

		t.Run("targeted patch", func(t *testing.T) {
			def := &PackageDefinition{
				Resources: []Resource{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Metadata: metav1.ObjectMeta{
							Name: "app1",
						},
						Raw: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "apps/v1",
								"kind":       "Deployment",
								"metadata": map[string]interface{}{
									"name": "app1",
								},
								"spec": map[string]interface{}{
									"replicas": float64(1),
								},
							},
						},
					},
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Metadata: metav1.ObjectMeta{
							Name: "app2",
						},
						Raw: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "apps/v1",
								"kind":       "Deployment",
								"metadata": map[string]interface{}{
									"name": "app2",
								},
								"spec": map[string]interface{}{
									"replicas": float64(1),
								},
							},
						},
					},
				},
			}

			// TOML format patch targeting specific deployment
			patches := []Patch{
				{
					Name: "scale-app1",
					Content: `[deployment.app1]
spec.replicas: 5`,
				},
			}

			params := ParameterMap{}
			result, err := processor.ApplyPatches(ctx, def, patches, params)

			require.NoError(t, err)

			// Check that only app1 was patched
			spec1 := result.Resources[0].Raw.Object["spec"].(map[string]interface{})
			spec2 := result.Resources[1].Raw.Object["spec"].(map[string]interface{})
			assert.Equal(t, 5, spec1["replicas"])
			assert.Equal(t, float64(1), spec2["replicas"]) // Unchanged
		})

		t.Run("patch with variables", func(t *testing.T) {
			def := &PackageDefinition{
				Resources: []Resource{
					{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Metadata: metav1.ObjectMeta{
							Name: "config",
						},
						Raw: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]interface{}{
									"name": "config",
								},
								"data": map[string]interface{}{},
							},
						},
					},
				},
			}

			patches := []Patch{
				{
					Name: "add-config",
					Content: `data.environment: ${values.env}
data.version: ${values.app.version}`,
				},
			}

			params := ParameterMap{
				"env": "production",
				"app": map[string]interface{}{
					"version": "1.2.3",
				},
			}

			result, err := processor.ApplyPatches(ctx, def, patches, params)

			require.NoError(t, err)

			data := result.Resources[0].Raw.Object["data"].(map[string]interface{})
			assert.Equal(t, "production", data["environment"])
			assert.Equal(t, "1.2.3", data["version"])
		})
	})

	t.Run("DebugPatchGraph", func(t *testing.T) {
		patches := []Patch{
			{
				Name: "base",
				Metadata: &PatchMetadata{
					Description: "Base configuration",
				},
			},
			{
				Name: "feature-a",
				Metadata: &PatchMetadata{
					Enabled:     "${features.a}",
					Description: "Feature A",
					Requires:    []string{"base"},
				},
			},
			{
				Name: "feature-b",
				Metadata: &PatchMetadata{
					Enabled:     "${features.b}",
					Description: "Feature B",
					Requires:    []string{"base"},
					Conflicts:   []string{"feature-a"},
				},
			},
		}

		graph := processor.DebugPatchGraph(patches)

		assert.Contains(t, graph, "Patch Dependency Graph")
		assert.Contains(t, graph, "base:")
		assert.Contains(t, graph, "Description: Base configuration")
		assert.Contains(t, graph, "feature-a:")
		assert.Contains(t, graph, "Condition: ${features.a}")
		assert.Contains(t, graph, "Requires:")
		assert.Contains(t, graph, "-> base")
		assert.Contains(t, graph, "feature-b:")
		assert.Contains(t, graph, "Conflicts:")
		assert.Contains(t, graph, "x feature-a")
	})

	t.Run("helper functions", func(t *testing.T) {
		p := &patchProcessor{logger: log}

		t.Run("evaluateExpression", func(t *testing.T) {
			params := ParameterMap{
				"enabled":  true,
				"disabled": false,
				"env":      "prod",
				"count":    5,
			}

			assert.True(t, p.evaluateExpression("${enabled}", params))
			assert.False(t, p.evaluateExpression("${disabled}", params))
			assert.True(t, p.evaluateExpression("${env}", params))      // Non-empty string
			assert.True(t, p.evaluateExpression("${count}", params))    // Non-zero number
			assert.False(t, p.evaluateExpression("${missing}", params)) // Missing variable
			assert.True(t, p.evaluateExpression("true", params))        // Literal
			assert.False(t, p.evaluateExpression("false", params))      // Literal
		})

		t.Run("toBool", func(t *testing.T) {
			assert.True(t, p.toBool(true))
			assert.False(t, p.toBool(false))
			assert.True(t, p.toBool("true"))
			assert.True(t, p.toBool("yes"))
			assert.True(t, p.toBool("1"))
			assert.True(t, p.toBool("enabled"))
			assert.False(t, p.toBool("false"))
			assert.False(t, p.toBool(""))
			assert.True(t, p.toBool(1))
			assert.False(t, p.toBool(0))
			assert.True(t, p.toBool(3.14))
			assert.False(t, p.toBool(0.0))
			assert.False(t, p.toBool(nil))
		})

		t.Run("matchesTarget", func(t *testing.T) {
			resource := &Resource{
				Kind: "Deployment",
				Metadata: metav1.ObjectMeta{
					Name: "test-app",
				},
			}

			// Empty target matches all
			assert.True(t, p.matchesTarget(resource, ""))

			// Kind only
			assert.True(t, p.matchesTarget(resource, "Deployment"))
			assert.True(t, p.matchesTarget(resource, "deployment")) // Case insensitive
			assert.False(t, p.matchesTarget(resource, "Service"))

			// Kind.Name format
			assert.True(t, p.matchesTarget(resource, "Deployment.test-app"))
			assert.True(t, p.matchesTarget(resource, "deployment.test-app"))
			assert.False(t, p.matchesTarget(resource, "Deployment.other-app"))

			// Kind/Name format
			assert.True(t, p.matchesTarget(resource, "Deployment/test-app"))
			assert.False(t, p.matchesTarget(resource, "Service/test-app"))
		})
	})

	t.Run("circular dependencies", func(t *testing.T) {
		patches := []Patch{
			{
				Name: "patch1",
				Metadata: &PatchMetadata{
					Requires: []string{"patch2"},
				},
			},
			{
				Name: "patch2",
				Metadata: &PatchMetadata{
					Requires: []string{"patch3"},
				},
			},
			{
				Name: "patch3",
				Metadata: &PatchMetadata{
					Requires: []string{"patch1"}, // Creates cycle
				},
			},
		}

		graph := processor.DebugPatchGraph(patches)
		assert.Contains(t, graph, "Issues Detected")
		assert.Contains(t, graph, "circular")
	})
}

func TestPatchIssueDetection(t *testing.T) {
	p := &patchProcessor{logger: logger.Noop()}

	t.Run("missing dependency", func(t *testing.T) {
		patchMap := map[string]*Patch{
			"patch1": {
				Name: "patch1",
				Metadata: &PatchMetadata{
					Requires: []string{"missing-patch"},
				},
			},
		}

		issues := p.findPatchIssues(patchMap)
		assert.Len(t, issues, 1)
		assert.Contains(t, issues[0], "non-existent")
	})

	t.Run("non-mutual conflict", func(t *testing.T) {
		patchMap := map[string]*Patch{
			"patch1": {
				Name: "patch1",
				Metadata: &PatchMetadata{
					Conflicts: []string{"patch2"},
				},
			},
			"patch2": {
				Name:     "patch2",
				Metadata: &PatchMetadata{
					// patch2 doesn't declare conflict with patch1
				},
			},
		}

		issues := p.findPatchIssues(patchMap)
		assert.Len(t, issues, 1)
		assert.Contains(t, issues[0], "not vice versa")
	})

	t.Run("valid configuration", func(t *testing.T) {
		patchMap := map[string]*Patch{
			"base": {
				Name: "base",
			},
			"feature": {
				Name: "feature",
				Metadata: &PatchMetadata{
					Requires: []string{"base"},
				},
			},
		}

		issues := p.findPatchIssues(patchMap)
		assert.Empty(t, issues)
	})
}

func TestOrderByDependencies(t *testing.T) {
	p := &patchProcessor{logger: logger.Noop()}

	t.Run("linear dependencies", func(t *testing.T) {
		enabled := map[string]bool{
			"a": true,
			"b": true,
			"c": true,
		}

		patchMap := map[string]*Patch{
			"a": {Name: "a"},
			"b": {
				Name: "b",
				Metadata: &PatchMetadata{
					Requires: []string{"a"},
				},
			},
			"c": {
				Name: "c",
				Metadata: &PatchMetadata{
					Requires: []string{"b"},
				},
			},
		}

		order := p.orderByDependencies(enabled, patchMap)
		assert.Equal(t, []string{"a", "b", "c"}, order)
	})

	t.Run("parallel dependencies", func(t *testing.T) {
		enabled := map[string]bool{
			"base":  true,
			"feat1": true,
			"feat2": true,
		}

		patchMap := map[string]*Patch{
			"base": {Name: "base"},
			"feat1": {
				Name: "feat1",
				Metadata: &PatchMetadata{
					Requires: []string{"base"},
				},
			},
			"feat2": {
				Name: "feat2",
				Metadata: &PatchMetadata{
					Requires: []string{"base"},
				},
			},
		}

		order := p.orderByDependencies(enabled, patchMap)

		// base must come first
		assert.Equal(t, "base", order[0])
		// feat1 and feat2 can be in any order after base
		assert.Contains(t, order[1:], "feat1")
		assert.Contains(t, order[1:], "feat2")
	})

	t.Run("no dependencies", func(t *testing.T) {
		enabled := map[string]bool{
			"patch1": true,
			"patch2": true,
			"patch3": true,
		}

		patchMap := map[string]*Patch{
			"patch1": {Name: "patch1"},
			"patch2": {Name: "patch2"},
			"patch3": {Name: "patch3"},
		}

		order := p.orderByDependencies(enabled, patchMap)
		assert.Len(t, order, 3)

		// Should contain all patches (order doesn't matter)
		assert.Contains(t, order, "patch1")
		assert.Contains(t, order, "patch2")
		assert.Contains(t, order, "patch3")
	})
}

func TestCreateVariableContext(t *testing.T) {
	p := &patchProcessor{logger: logger.Noop()}

	params := ParameterMapWithSource{
		"app": ParameterSource{
			Value: map[string]interface{}{
				"name": "test-app",
				"port": 8080,
			},
		},
		"enabled": ParameterSource{
			Value: true,
		},
		"items": ParameterSource{
			Value: []interface{}{"a", "b", "c"},
		},
	}

	varCtx := p.createVariableContext(params)

	// Check that values are converted correctly
	// The new implementation stores values directly, not as strings
	assert.Equal(t, "test-app", varCtx.Values["app.name"])
	assert.Equal(t, 8080, varCtx.Values["app.port"])
	assert.Equal(t, true, varCtx.Values["enabled"])
	assert.Equal(t, "a", varCtx.Values["items[0]"])
	assert.Equal(t, "b", varCtx.Values["items[1]"])
	assert.Equal(t, "c", varCtx.Values["items[2]"])
}
