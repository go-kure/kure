package launcher

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kerrors "github.com/go-kure/kure/pkg/errors"
)

func TestNewLoadErrors(t *testing.T) {
	t.Run("with issues", func(t *testing.T) {
		partial := &PackageDefinition{
			Path: "/test/pkg",
			Metadata: KurelMetadata{
				Name: "test-pkg",
			},
		}
		issues := []error{
			errors.New("error 1"),
			errors.New("error 2"),
		}

		loadErr := NewLoadErrors(partial, issues)

		assert.NotNil(t, loadErr)
		assert.Equal(t, partial, loadErr.PartialDefinition)
		assert.Len(t, loadErr.Issues, 2)
		assert.Contains(t, loadErr.Error(), "2 issues")
		assert.Contains(t, loadErr.Error(), "error 1")
		assert.Contains(t, loadErr.Error(), "error 2")
	})

	t.Run("no issues", func(t *testing.T) {
		loadErr := NewLoadErrors(nil, []error{})

		assert.NotNil(t, loadErr)
		assert.Nil(t, loadErr.PartialDefinition)
		assert.Len(t, loadErr.Issues, 0)
		assert.Contains(t, loadErr.Error(), "loaded with issues")
	})
}

func TestLoadErrorsUnwrap(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	loadErr := NewLoadErrors(nil, []error{err1, err2})

	unwrapped := loadErr.Unwrap()
	require.Len(t, unwrapped, 2)
	assert.Equal(t, err1, unwrapped[0])
	assert.Equal(t, err2, unwrapped[1])
}

func TestLoadErrorsHasCriticalErrors(t *testing.T) {
	t.Run("no critical errors", func(t *testing.T) {
		configErr := kerrors.NewConfigError("test", "field", "value", "reason", nil)
		loadErr := NewLoadErrors(nil, []error{configErr})

		assert.False(t, loadErr.HasCriticalErrors())
	})

	t.Run("with critical error", func(t *testing.T) {
		sizeErr := NewSizeError("package", 1000, 100)
		loadErr := NewLoadErrors(nil, []error{sizeErr})

		assert.True(t, loadErr.HasCriticalErrors())
	})

	t.Run("empty issues", func(t *testing.T) {
		loadErr := NewLoadErrors(nil, []error{})
		assert.False(t, loadErr.HasCriticalErrors())
	})
}

func TestNewPatchError(t *testing.T) {
	t.Run("with resource name", func(t *testing.T) {
		patchErr := NewPatchError("patch1", "Deployment", "myapp", "/spec/replicas", "path not found")

		assert.NotNil(t, patchErr)
		assert.Equal(t, "patch1", patchErr.PatchName)
		assert.Equal(t, "Deployment", patchErr.ResourceKind)
		assert.Equal(t, "myapp", patchErr.ResourceName)
		assert.Equal(t, "/spec/replicas", patchErr.TargetPath)
		assert.Equal(t, "path not found", patchErr.Reason)
		assert.Contains(t, patchErr.Error(), "Deployment/myapp")
	})

	t.Run("without resource name", func(t *testing.T) {
		patchErr := NewPatchError("patch1", "Deployment", "", "/spec/replicas", "path not found")

		assert.NotNil(t, patchErr)
		assert.Equal(t, "", patchErr.ResourceName)
		assert.NotContains(t, patchErr.Error(), "/myapp")
	})
}

func TestNewDependencyError(t *testing.T) {
	tests := []struct {
		name          string
		depType       string
		source        string
		target        string
		chain         []string
		expectMessage string
	}{
		{
			name:          "circular dependency",
			depType:       "circular",
			source:        "a",
			target:        "b",
			chain:         []string{"a", "b", "c", "a"},
			expectMessage: "circular dependency",
		},
		{
			name:          "missing dependency",
			depType:       "missing",
			source:        "pkg-a",
			target:        "pkg-b",
			chain:         nil,
			expectMessage: "requires non-existent",
		},
		{
			name:          "conflict dependency",
			depType:       "conflict",
			source:        "pkg-a",
			target:        "pkg-b",
			chain:         nil,
			expectMessage: "conflicts with",
		},
		{
			name:          "unknown type",
			depType:       "unknown",
			source:        "a",
			target:        "b",
			chain:         nil,
			expectMessage: "dependency error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depErr := NewDependencyError(tt.depType, tt.source, tt.target, tt.chain)

			assert.NotNil(t, depErr)
			assert.Equal(t, tt.depType, depErr.DepType)
			assert.Equal(t, tt.source, depErr.Source)
			assert.Equal(t, tt.target, depErr.Target)
			assert.Contains(t, depErr.Error(), tt.expectMessage)
		})
	}
}

func TestNewVariableError(t *testing.T) {
	t.Run("with expression", func(t *testing.T) {
		varErr := NewVariableError("myVar", "${values.foo}", "undefined reference")

		assert.NotNil(t, varErr)
		assert.Equal(t, "myVar", varErr.Variable)
		assert.Equal(t, "${values.foo}", varErr.Expression)
		assert.Equal(t, "undefined reference", varErr.Reason)
		assert.Contains(t, varErr.Error(), "myVar")
		assert.Contains(t, varErr.Error(), "${values.foo}")
	})

	t.Run("without expression", func(t *testing.T) {
		varErr := NewVariableError("myVar", "", "not defined")

		assert.NotNil(t, varErr)
		assert.Equal(t, "", varErr.Expression)
		assert.Contains(t, varErr.Error(), "myVar")
		assert.Contains(t, varErr.Error(), "not defined")
	})
}

func TestNewSchemaError(t *testing.T) {
	t.Run("with path", func(t *testing.T) {
		schemaErr := NewSchemaError("/spec/replicas", -1, "positive integer", "must be positive")

		assert.NotNil(t, schemaErr)
		assert.Equal(t, "/spec/replicas", schemaErr.Path)
		assert.Equal(t, -1, schemaErr.Value)
		assert.Equal(t, "positive integer", schemaErr.Expected)
		assert.Contains(t, schemaErr.Error(), "/spec/replicas")
		assert.Contains(t, schemaErr.Error(), "must be positive")
	})

	t.Run("without path", func(t *testing.T) {
		schemaErr := NewSchemaError("", "invalid", "string", "type mismatch")

		assert.NotNil(t, schemaErr)
		assert.Equal(t, "", schemaErr.Path)
		assert.Contains(t, schemaErr.Error(), "type mismatch")
	})
}

func TestNewSizeError(t *testing.T) {
	sizeErr := NewSizeError("package", 2048, 1024)

	assert.NotNil(t, sizeErr)
	assert.Equal(t, int64(2048), sizeErr.ActualSize)
	assert.Equal(t, int64(1024), sizeErr.MaxSize)
	assert.Equal(t, "package", sizeErr.SizeType)
	assert.Contains(t, sizeErr.Error(), "2048")
	assert.Contains(t, sizeErr.Error(), "1024")
	assert.Contains(t, sizeErr.Error(), "package")
}

func TestIsCriticalError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsCriticalError(nil))
	})

	t.Run("file error read", func(t *testing.T) {
		fileErr := kerrors.NewFileError("read", "/path", "not found", nil)
		assert.True(t, IsCriticalError(fileErr))
	})

	t.Run("file error load", func(t *testing.T) {
		fileErr := kerrors.NewFileError("load", "/path", "not found", nil)
		assert.True(t, IsCriticalError(fileErr))
	})

	t.Run("file error write", func(t *testing.T) {
		fileErr := kerrors.NewFileError("write", "/path", "failed", nil)
		// All file errors are considered critical by the IsType check for ErrorTypeFile
		assert.True(t, IsCriticalError(fileErr))
	})

	t.Run("parse error", func(t *testing.T) {
		parseErr := kerrors.NewParseError("test.yaml", "invalid", 1, 1, nil)
		assert.True(t, IsCriticalError(parseErr))
	})

	t.Run("size error", func(t *testing.T) {
		sizeErr := NewSizeError("package", 2048, 1024)
		assert.True(t, IsCriticalError(sizeErr))
	})

	t.Run("config error", func(t *testing.T) {
		configErr := kerrors.NewConfigError("test", "field", "value", "reason", nil)
		assert.False(t, IsCriticalError(configErr))
	})

	t.Run("standard error", func(t *testing.T) {
		standardErr := errors.New("standard error")
		assert.False(t, IsCriticalError(standardErr))
	})
}

func TestIsWarning(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsWarning(nil))
	})

	t.Run("config error is warning", func(t *testing.T) {
		configErr := kerrors.NewConfigError("test", "field", "value", "reason", nil)
		assert.True(t, IsWarning(configErr))
	})

	t.Run("parse error is not warning", func(t *testing.T) {
		parseErr := kerrors.NewParseError("test.yaml", "invalid", 1, 1, nil)
		// Parse errors are critical, so not warnings
		assert.False(t, IsWarning(parseErr))
	})

	t.Run("size error is not warning", func(t *testing.T) {
		sizeErr := NewSizeError("package", 2048, 1024)
		assert.False(t, IsWarning(sizeErr))
	})

	t.Run("standard error is warning", func(t *testing.T) {
		standardErr := errors.New("some warning")
		assert.True(t, IsWarning(standardErr))
	})
}

func TestAs(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		var target *SizeError
		assert.False(t, As(nil, &target))
	})

	t.Run("matching type", func(t *testing.T) {
		sizeErr := NewSizeError("package", 2048, 1024)
		var target *SizeError
		assert.True(t, As(sizeErr, &target))
		assert.Equal(t, int64(2048), target.ActualSize)
	})

	t.Run("non-matching type", func(t *testing.T) {
		varErr := NewVariableError("var", "expr", "reason")
		var target *SizeError
		assert.False(t, As(varErr, &target))
	})
}
