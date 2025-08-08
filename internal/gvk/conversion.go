package gvk

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ConversionPath represents a path from one version to another
type ConversionPath struct {
	From GVK
	To   GVK
	Converter Converter
}

// Converter defines the interface for converting between versions
type Converter interface {
	Convert(from interface{}) (interface{}, error)
}

// ConversionFunc is a function-based converter
type ConversionFunc func(from interface{}) (interface{}, error)

// Convert implements the Converter interface
func (f ConversionFunc) Convert(from interface{}) (interface{}, error) {
	return f(from)
}

// ConversionRegistry manages version conversion paths
type ConversionRegistry struct {
	conversions map[string]map[string]Converter // [fromGVK][toGVK] -> Converter
}

// NewConversionRegistry creates a new conversion registry
func NewConversionRegistry() *ConversionRegistry {
	return &ConversionRegistry{
		conversions: make(map[string]map[string]Converter),
	}
}

// Register registers a conversion path
func (r *ConversionRegistry) Register(from, to GVK, converter Converter) {
	fromKey := from.String()
	toKey := to.String()
	
	if r.conversions[fromKey] == nil {
		r.conversions[fromKey] = make(map[string]Converter)
	}
	r.conversions[fromKey][toKey] = converter
}

// RegisterFunc registers a conversion function
func (r *ConversionRegistry) RegisterFunc(from, to GVK, converter ConversionFunc) {
	r.Register(from, to, converter)
}

// Convert converts from one GVK to another
func (r *ConversionRegistry) Convert(from, to GVK, obj interface{}) (interface{}, error) {
	if from == to {
		return obj, nil // No conversion needed
	}

	fromKey := from.String()
	toKey := to.String()

	if fromConverters, exists := r.conversions[fromKey]; exists {
		if converter, exists := fromConverters[toKey]; exists {
			return converter.Convert(obj)
		}
	}

	return nil, fmt.Errorf("no conversion path from %s to %s", from, to)
}

// HasConversion checks if a conversion path exists
func (r *ConversionRegistry) HasConversion(from, to GVK) bool {
	if from == to {
		return true
	}

	fromKey := from.String()
	toKey := to.String()

	if fromConverters, exists := r.conversions[fromKey]; exists {
		_, exists := fromConverters[toKey]
		return exists
	}

	return false
}

// ListConversions returns all available conversion paths for a given GVK
func (r *ConversionRegistry) ListConversions(from GVK) []GVK {
	fromKey := from.String()
	var targets []GVK

	if fromConverters, exists := r.conversions[fromKey]; exists {
		for toKey := range fromConverters {
			// Parse the target GVK from the key
			// This is a simplified parser - in practice you'd store GVKs directly
			parts := strings.Split(toKey, ", Kind=")
			if len(parts) == 2 {
				apiVersion := parts[0]
				kind := parts[1]
				targets = append(targets, ParseAPIVersion(apiVersion, kind))
			}
		}
	}

	return targets
}

// VersionComparator compares version strings using semantic versioning
type VersionComparator struct{}

// Compare compares two version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func (vc *VersionComparator) Compare(v1, v2 string) int {
	// Handle special version formats
	v1Normalized := vc.normalizeVersion(v1)
	v2Normalized := vc.normalizeVersion(v2)

	v1Parts := vc.parseVersion(v1Normalized)
	v2Parts := vc.parseVersion(v2Normalized)

	// Compare major, minor, patch
	for i := 0; i < 3; i++ {
		if v1Parts[i] < v2Parts[i] {
			return -1
		}
		if v1Parts[i] > v2Parts[i] {
			return 1
		}
	}

	// Compare pre-release
	return vc.comparePrerelease(v1Parts[3], v2Parts[3])
}

// normalizeVersion normalizes version strings to standard format
func (vc *VersionComparator) normalizeVersion(v string) string {
	// Handle Kubernetes-style versions
	if strings.HasPrefix(v, "v") && len(v) > 1 {
		v = v[1:] // Remove 'v' prefix
	}

	// Handle alpha/beta versions
	v = strings.ReplaceAll(v, "alpha", "alpha.")
	v = strings.ReplaceAll(v, "beta", "beta.")

	return v
}

// parseVersion parses a version string into [major, minor, patch, prerelease]
func (vc *VersionComparator) parseVersion(v string) [4]int {
	parts := [4]int{0, 0, 0, 0} // major, minor, patch, prerelease

	// Split by pre-release
	mainAndPre := strings.SplitN(v, "alpha", 2)
	if len(mainAndPre) == 1 {
		mainAndPre = strings.SplitN(v, "beta", 2)
		if len(mainAndPre) == 2 {
			parts[3] = 2000 // beta is higher than alpha
		}
	} else {
		parts[3] = 1000 // alpha
	}

	// Parse pre-release number
	if len(mainAndPre) == 2 && mainAndPre[1] != "" {
		if preNum, err := strconv.Atoi(strings.TrimPrefix(mainAndPre[1], ".")); err == nil {
			parts[3] += preNum
		}
	} else if len(mainAndPre) == 1 {
		parts[3] = 9999 // stable version
	}

	// Parse main version
	mainParts := strings.Split(mainAndPre[0], ".")
	for i, part := range mainParts {
		if i >= 3 {
			break
		}
		if num, err := strconv.Atoi(part); err == nil {
			parts[i] = num
		}
	}

	return parts
}

// comparePrerelease compares pre-release values
func (vc *VersionComparator) comparePrerelease(p1, p2 int) int {
	if p1 < p2 {
		return -1
	}
	if p1 > p2 {
		return 1
	}
	return 0
}

// GetLatestVersion returns the latest version from a list of GVKs with the same group/kind
func (vc *VersionComparator) GetLatestVersion(gvks []GVK) (GVK, error) {
	if len(gvks) == 0 {
		return GVK{}, fmt.Errorf("no GVKs provided")
	}

	// Verify all have same group/kind
	first := gvks[0]
	for _, gvk := range gvks[1:] {
		if gvk.Group != first.Group || gvk.Kind != first.Kind {
			return GVK{}, fmt.Errorf("all GVKs must have same group and kind")
		}
	}

	// Sort by version
	sort.Slice(gvks, func(i, j int) bool {
		return vc.Compare(gvks[i].Version, gvks[j].Version) > 0
	})

	return gvks[0], nil
}