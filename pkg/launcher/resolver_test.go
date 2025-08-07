package launcher

import (
	"context"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver(t *testing.T) {
	log := logger.Noop()
	resolver := NewResolver(log)
	ctx := context.Background()
	opts := DefaultOptions()

	t.Run("simple substitution", func(t *testing.T) {
		base := ParameterMap{
			"app": map[string]interface{}{
				"name":    "myapp",
				"version": "1.0.0",
			},
			"message": "Hello from ${app.name}",
		}

		result, err := resolver.Resolve(ctx, base, nil, opts)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Check resolved value
		assert.Equal(t, "Hello from myapp", result["message"].Value)
		assert.Equal(t, "package", result["message"].Location)
	})

	t.Run("nested substitution", func(t *testing.T) {
		base := ParameterMap{
			"env": "prod",
			"config": map[string]interface{}{
				"database": map[string]interface{}{
					"host": "db-${env}.example.com",
					"port": 5432,
				},
			},
		}

		result, err := resolver.Resolve(ctx, base, nil, opts)
		require.NoError(t, err)

		// Check nested resolution
		config := result["config"].Value.(map[string]interface{})
		database := config["database"].(map[string]interface{})
		assert.Equal(t, "db-prod.example.com", database["host"])
		assert.Equal(t, 5432, database["port"])
	})

	t.Run("multiple variables", func(t *testing.T) {
		base := ParameterMap{
			"first":  "Hello",
			"second": "World",
			"message": "${first} ${second}!",
		}

		result, err := resolver.Resolve(ctx, base, nil, opts)
		require.NoError(t, err)

		assert.Equal(t, "Hello World!", result["message"].Value)
	})

	t.Run("array access", func(t *testing.T) {
		base := ParameterMap{
			"items": []interface{}{"first", "second", "third"},
			"selected": "${items[1]}",
		}

		result, err := resolver.Resolve(ctx, base, nil, opts)
		require.NoError(t, err)

		assert.Equal(t, "second", result["selected"].Value)
	})

	t.Run("undefined variable", func(t *testing.T) {
		base := ParameterMap{
			"message": "Value: ${undefined.var}",
		}

		_, err := resolver.Resolve(ctx, base, nil, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "undefined")
	})

	t.Run("cyclic reference", func(t *testing.T) {
		base := ParameterMap{
			"a": "${b}",
			"b": "${c}",
			"c": "${a}", // Cycle: a -> b -> c -> a
		}

		_, err := resolver.Resolve(ctx, base, nil, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cyclic")
	})

	t.Run("max depth exceeded", func(t *testing.T) {
		// Create a deep chain of references
		base := ParameterMap{
			"v1": "${v2}",
			"v2": "${v3}",
			"v3": "${v4}",
			"v4": "${v5}",
			"v5": "${v6}",
			"v6": "${v7}",
			"v7": "${v8}",
			"v8": "${v9}",
			"v9": "${v10}",
			"v10": "${v11}",
			"v11": "${v12}",
			"v12": "final",
		}

		opts := &LauncherOptions{
			MaxDepth: 5, // Set low max depth
			Logger:   log,
		}

		result, err := resolver.Resolve(ctx, base, nil, opts)
		if err == nil {
			t.Logf("Expected error but got result: %v", result)
		}
		assert.Error(t, err, "Expected error for deep variable chain exceeding max depth")
		if err != nil {
			assert.Contains(t, err.Error(), "depth")
		}
	})

	t.Run("parameter merging", func(t *testing.T) {
		base := ParameterMap{
			"app": map[string]interface{}{
				"name":    "base-app",
				"version": "1.0.0",
			},
			"feature": true,
		}

		overrides := ParameterMap{
			"app": map[string]interface{}{
				"name": "override-app", // Override name
				// version stays from base
			},
			"new": "value", // Add new parameter
		}

		result, err := resolver.Resolve(ctx, base, overrides, opts)
		require.NoError(t, err)

		// Check merged values
		app := result["app"].Value.(map[string]interface{})
		assert.Equal(t, "override-app", app["name"])
		assert.Equal(t, "local", result["app"].Location)
		
		assert.Equal(t, true, result["feature"].Value)
		assert.Equal(t, "package", result["feature"].Location)
		
		assert.Equal(t, "value", result["new"].Value)
		assert.Equal(t, "local", result["new"].Location)
	})

	t.Run("boolean and numeric values", func(t *testing.T) {
		base := ParameterMap{
			"enabled": true,
			"count":   42,
			"price":   19.99,
			"message": "Enabled: ${enabled}, Count: ${count}, Price: ${price}",
		}

		result, err := resolver.Resolve(ctx, base, nil, opts)
		require.NoError(t, err)

		assert.Equal(t, "Enabled: true, Count: 42, Price: 19.99", result["message"].Value)
	})

	t.Run("direct variable reference", func(t *testing.T) {
		base := ParameterMap{
			"source": map[string]interface{}{
				"key": "value",
				"num": 123,
			},
			"ref": "${source}", // Direct reference to object
		}

		result, err := resolver.Resolve(ctx, base, nil, opts)
		require.NoError(t, err)

		// Should get the actual object, not a string
		ref := result["ref"].Value.(map[string]interface{})
		assert.Equal(t, "value", ref["key"])
		assert.Equal(t, 123, ref["num"])
	})

	t.Run("array resolution", func(t *testing.T) {
		base := ParameterMap{
			"prefix": "item",
			"items": []interface{}{
				"${prefix}-1",
				"${prefix}-2",
				"${prefix}-3",
			},
		}

		result, err := resolver.Resolve(ctx, base, nil, opts)
		require.NoError(t, err)

		items := result["items"].Value.([]interface{})
		assert.Equal(t, "item-1", items[0])
		assert.Equal(t, "item-2", items[1])
		assert.Equal(t, "item-3", items[2])
	})
}

func TestDebugVariableGraph(t *testing.T) {
	log := logger.Noop()
	resolver := NewResolver(log)

	t.Run("simple dependencies", func(t *testing.T) {
		params := ParameterMap{
			"a": "value",
			"b": "${a}",
			"c": "${b}",
		}

		graph := resolver.DebugVariableGraph(params)
		
		assert.Contains(t, graph, "Variable Dependency Graph")
		assert.Contains(t, graph, "b:")
		assert.Contains(t, graph, "-> a")
		assert.Contains(t, graph, "c:")
		assert.Contains(t, graph, "-> b")
	})

	t.Run("cycle detection", func(t *testing.T) {
		params := ParameterMap{
			"a": "${b}",
			"b": "${c}",
			"c": "${a}",
		}

		graph := resolver.DebugVariableGraph(params)
		
		assert.Contains(t, graph, "Cycles Detected")
		// The graph should show the cycle
		assert.True(t, 
			strings.Contains(graph, "a -> b -> c -> a") ||
			strings.Contains(graph, "b -> c -> a -> b") ||
			strings.Contains(graph, "c -> a -> b -> c"),
		)
	})

	t.Run("no dependencies", func(t *testing.T) {
		params := ParameterMap{
			"static1": "value1",
			"static2": 42,
			"static3": true,
		}

		graph := resolver.DebugVariableGraph(params)
		
		assert.Contains(t, graph, "(no dependencies)")
		assert.NotContains(t, graph, "Cycles Detected")
	})

	t.Run("complex dependencies", func(t *testing.T) {
		params := ParameterMap{
			"base": "value",
			"app": map[string]interface{}{
				"name": "${base}-app",
				"port": 8080,
			},
			"url": "http://${app.name}:${app.port}",
		}

		graph := resolver.DebugVariableGraph(params)
		
		assert.Contains(t, graph, "app.name:")
		assert.Contains(t, graph, "-> base")
		assert.Contains(t, graph, "url:")
	})
}

func TestResolverHelpers(t *testing.T) {
	resolver := &variableResolver{
		logger: logger.Noop(),
	}

	t.Run("lookupVariable", func(t *testing.T) {
		params := ParameterMap{
			"simple": "value",
			"nested": map[string]interface{}{
				"key": "nested-value",
				"deep": map[string]interface{}{
					"field": "deep-value",
				},
			},
			"array": []interface{}{"a", "b", "c"},
		}

		// Simple lookup
		assert.Equal(t, "value", resolver.lookupVariable("simple", params))
		
		// Nested lookup
		assert.Equal(t, "nested-value", resolver.lookupVariable("nested.key", params))
		assert.Equal(t, "deep-value", resolver.lookupVariable("nested.deep.field", params))
		
		// Array lookup
		assert.Equal(t, "b", resolver.lookupVariable("array[1]", params))
		
		// Non-existent
		assert.Nil(t, resolver.lookupVariable("missing", params))
		assert.Nil(t, resolver.lookupVariable("nested.missing", params))
	})

	t.Run("valueToString", func(t *testing.T) {
		assert.Equal(t, "hello", resolver.valueToString("hello"))
		assert.Equal(t, "true", resolver.valueToString(true))
		assert.Equal(t, "false", resolver.valueToString(false))
		assert.Equal(t, "42", resolver.valueToString(42))
		assert.Equal(t, "3.14", resolver.valueToString(3.14))
	})

	t.Run("deepCopyValue", func(t *testing.T) {
		original := map[string]interface{}{
			"key": "value",
			"nested": map[string]interface{}{
				"field": "nested",
			},
			"array": []interface{}{1, 2, 3},
		}

		copied := resolver.deepCopyValue(original).(map[string]interface{})
		
		// Verify it's a copy
		assert.Equal(t, original, copied)
		
		// Modify copy and ensure original is unchanged
		copied["key"] = "modified"
		assert.Equal(t, "value", original["key"])
		
		// Modify nested value
		copied["nested"].(map[string]interface{})["field"] = "modified"
		assert.Equal(t, "nested", original["nested"].(map[string]interface{})["field"])
	})
}