package gvk

import (
	"testing"
)

func TestConversionFuncConvert(t *testing.T) {
	converter := ConversionFunc(func(from interface{}) (interface{}, error) {
		// Simple passthrough converter
		return from, nil
	})

	result, err := converter.Convert("test")
	if err != nil {
		t.Errorf("ConversionFunc.Convert() error = %v", err)
	}
	if result != "test" {
		t.Errorf("ConversionFunc.Convert() = %v, want %v", result, "test")
	}
}

func TestNewConversionRegistry(t *testing.T) {
	registry := NewConversionRegistry()
	if registry == nil {
		t.Fatal("NewConversionRegistry() returned nil")
	}
	if registry.conversions == nil {
		t.Error("conversions map should be initialized")
	}
}

func TestConversionRegistryRegister(t *testing.T) {
	registry := NewConversionRegistry()

	fromGVK := GVK{Group: "test", Version: "v1", Kind: "Foo"}
	toGVK := GVK{Group: "test", Version: "v2", Kind: "Foo"}

	converter := ConversionFunc(func(from interface{}) (interface{}, error) {
		return from, nil
	})

	registry.Register(fromGVK, toGVK, converter)

	if !registry.HasConversion(fromGVK, toGVK) {
		t.Error("expected conversion to be registered")
	}
}

func TestConversionRegistryRegisterFunc(t *testing.T) {
	registry := NewConversionRegistry()

	fromGVK := GVK{Group: "test", Version: "v1", Kind: "Foo"}
	toGVK := GVK{Group: "test", Version: "v2", Kind: "Foo"}

	registry.RegisterFunc(fromGVK, toGVK, func(from interface{}) (interface{}, error) {
		return from, nil
	})

	if !registry.HasConversion(fromGVK, toGVK) {
		t.Error("expected conversion to be registered via RegisterFunc")
	}
}

func TestConversionRegistryConvert(t *testing.T) {
	registry := NewConversionRegistry()

	fromGVK := GVK{Group: "test", Version: "v1", Kind: "Foo"}
	toGVK := GVK{Group: "test", Version: "v2", Kind: "Foo"}

	registry.RegisterFunc(fromGVK, toGVK, func(from interface{}) (interface{}, error) {
		// Double the input (if string)
		if s, ok := from.(string); ok {
			return s + s, nil
		}
		return from, nil
	})

	t.Run("successful conversion", func(t *testing.T) {
		result, err := registry.Convert(fromGVK, toGVK, "test")
		if err != nil {
			t.Errorf("Convert() error = %v", err)
		}
		if result != "testtest" {
			t.Errorf("Convert() = %v, want %v", result, "testtest")
		}
	})

	t.Run("same GVK no-op", func(t *testing.T) {
		result, err := registry.Convert(fromGVK, fromGVK, "test")
		if err != nil {
			t.Errorf("Convert() error = %v", err)
		}
		if result != "test" {
			t.Errorf("Convert() = %v, want %v", result, "test")
		}
	})

	t.Run("no conversion path", func(t *testing.T) {
		unknownGVK := GVK{Group: "unknown", Version: "v1", Kind: "Unknown"}
		_, err := registry.Convert(fromGVK, unknownGVK, "test")
		if err == nil {
			t.Error("expected error for missing conversion path")
		}
	})
}

func TestConversionRegistryHasConversion(t *testing.T) {
	registry := NewConversionRegistry()

	fromGVK := GVK{Group: "test", Version: "v1", Kind: "Foo"}
	toGVK := GVK{Group: "test", Version: "v2", Kind: "Foo"}
	unknownGVK := GVK{Group: "unknown", Version: "v1", Kind: "Unknown"}

	registry.RegisterFunc(fromGVK, toGVK, func(from interface{}) (interface{}, error) {
		return from, nil
	})

	t.Run("same GVK", func(t *testing.T) {
		if !registry.HasConversion(fromGVK, fromGVK) {
			t.Error("HasConversion() should return true for same GVK")
		}
	})

	t.Run("registered conversion", func(t *testing.T) {
		if !registry.HasConversion(fromGVK, toGVK) {
			t.Error("HasConversion() should return true for registered conversion")
		}
	})

	t.Run("unregistered conversion", func(t *testing.T) {
		if registry.HasConversion(fromGVK, unknownGVK) {
			t.Error("HasConversion() should return false for unregistered conversion")
		}
	})

	t.Run("unknown source GVK", func(t *testing.T) {
		if registry.HasConversion(unknownGVK, toGVK) {
			t.Error("HasConversion() should return false for unknown source GVK")
		}
	})
}

func TestConversionRegistryListConversions(t *testing.T) {
	registry := NewConversionRegistry()

	fromGVK := GVK{Group: "test", Version: "v1", Kind: "Foo"}
	toGVKv2 := GVK{Group: "test", Version: "v2", Kind: "Foo"}
	toGVKv3 := GVK{Group: "test", Version: "v3", Kind: "Foo"}

	registry.RegisterFunc(fromGVK, toGVKv2, func(from interface{}) (interface{}, error) {
		return from, nil
	})
	registry.RegisterFunc(fromGVK, toGVKv3, func(from interface{}) (interface{}, error) {
		return from, nil
	})

	conversions := registry.ListConversions(fromGVK)
	if len(conversions) != 2 {
		t.Errorf("ListConversions() returned %d conversions, want 2", len(conversions))
	}
}

func TestVersionComparator(t *testing.T) {
	vc := &VersionComparator{}

	tests := []struct {
		v1     string
		v2     string
		expect int
	}{
		{"v1", "v2", -1},
		{"v2", "v1", 1},
		{"v1", "v1", 0},
		{"v1alpha1", "v1beta1", -1},
		{"v1beta1", "v1", -1},
		{"v1", "v1alpha1", 1},
		{"v2alpha1", "v1", 1},
	}

	for _, tt := range tests {
		result := vc.Compare(tt.v1, tt.v2)
		if result != tt.expect {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.v1, tt.v2, result, tt.expect)
		}
	}
}

func TestVersionComparatorGetLatestVersion(t *testing.T) {
	vc := &VersionComparator{}

	t.Run("empty list", func(t *testing.T) {
		_, err := vc.GetLatestVersion([]GVK{})
		if err == nil {
			t.Error("expected error for empty list")
		}
	})

	t.Run("single version", func(t *testing.T) {
		gvks := []GVK{{Group: "test", Version: "v1", Kind: "Foo"}}
		result, err := vc.GetLatestVersion(gvks)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Version != "v1" {
			t.Errorf("GetLatestVersion() = %q, want v1", result.Version)
		}
	})

	t.Run("multiple versions", func(t *testing.T) {
		gvks := []GVK{
			{Group: "test", Version: "v1", Kind: "Foo"},
			{Group: "test", Version: "v2", Kind: "Foo"},
			{Group: "test", Version: "v1alpha1", Kind: "Foo"},
			{Group: "test", Version: "v1beta1", Kind: "Foo"},
		}
		result, err := vc.GetLatestVersion(gvks)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Version != "v2" {
			t.Errorf("GetLatestVersion() = %q, want v2", result.Version)
		}
	})

	t.Run("mismatched groups", func(t *testing.T) {
		gvks := []GVK{
			{Group: "test1", Version: "v1", Kind: "Foo"},
			{Group: "test2", Version: "v1", Kind: "Foo"},
		}
		_, err := vc.GetLatestVersion(gvks)
		if err == nil {
			t.Error("expected error for mismatched groups")
		}
	})
}
