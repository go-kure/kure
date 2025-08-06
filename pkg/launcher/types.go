package launcher

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// KurelMetadata contains package metadata from the kurel: key in parameters.yaml
type KurelMetadata struct {
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	AppVersion  string   `yaml:"appVersion" json:"appVersion"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Home        string   `yaml:"home,omitempty" json:"home,omitempty"`
	Keywords    []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	Schemas     []string `yaml:"schemas,omitempty" json:"schemas,omitempty"` // CRD schema URLs
	Maintainers []struct {
		Name  string `yaml:"name" json:"name"`
		Email string `yaml:"email,omitempty" json:"email,omitempty"`
	} `yaml:"maintainers,omitempty" json:"maintainers,omitempty"`
}

// ParameterMap holds configuration parameters
type ParameterMap map[string]interface{}

// ParameterSource tracks where a parameter value came from for debugging
type ParameterSource struct {
	Value    interface{} `json:"value"`
	Location string      `json:"location"` // "package", "local", "default"
	File     string      `json:"file"`     // Which file it came from
	Line     int         `json:"line"`     // Line number if applicable
}

// ParameterMapWithSource maps parameter names to their values with source tracking
type ParameterMapWithSource map[string]ParameterSource

// Resource represents a Kubernetes resource with thread-safe access
type Resource struct {
	APIVersion string               `yaml:"apiVersion" json:"apiVersion"`
	Kind       string               `yaml:"kind" json:"kind"`
	Metadata   metav1.ObjectMeta    `yaml:"metadata" json:"metadata"`
	Raw        *unstructured.Unstructured // For patch system compatibility
	mu         sync.RWMutex         // Protect concurrent access
}

// GetName returns the resource name thread-safely
func (r *Resource) GetName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Metadata.Name
}

// GetNamespace returns the resource namespace thread-safely
func (r *Resource) GetNamespace() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Metadata.Namespace
}

// ToUnstructured converts the resource to unstructured format
func (r *Resource) ToUnstructured() (*unstructured.Unstructured, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.Raw == nil {
		return nil, nil
	}
	return r.Raw.DeepCopy(), nil
}

// DeepCopy creates an independent copy of the resource
func (r *Resource) DeepCopy() Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var rawCopy *unstructured.Unstructured
	if r.Raw != nil {
		rawCopy = r.Raw.DeepCopy()
	}
	
	return Resource{
		APIVersion: r.APIVersion,
		Kind:       r.Kind,
		Metadata:   *r.Metadata.DeepCopy(),
		Raw:        rawCopy,
	}
}

// Patch represents a patch file with its metadata
type Patch struct {
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	Content   string         `json:"-"` // TOML content
	Metadata  *PatchMetadata `json:"metadata,omitempty"`
}

// PatchMetadata contains patch configuration and dependencies
type PatchMetadata struct {
	Enabled     string   `yaml:"enabled,omitempty" json:"enabled,omitempty"`         // Variable expression
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Requires    []string `yaml:"requires,omitempty" json:"requires,omitempty"`       // Required patches
	Conflicts   []string `yaml:"conflicts,omitempty" json:"conflicts,omitempty"`     // Conflicting patches
}

// PackageDefinition represents an immutable kurel package
type PackageDefinition struct {
	Path       string        `json:"path"`
	Metadata   KurelMetadata `json:"metadata"`
	Parameters ParameterMap  `json:"parameters"`
	Resources  []Resource    `json:"resources"`
	Patches    []Patch       `json:"patches"`
	mu         sync.RWMutex  // Protect concurrent reads
}

// DeepCopy creates an independent copy of the package definition for safe mutation
func (pd *PackageDefinition) DeepCopy() *PackageDefinition {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	
	// Deep copy resources
	resources := make([]Resource, len(pd.Resources))
	for i, r := range pd.Resources {
		resources[i] = r.DeepCopy()
	}
	
	// Deep copy patches
	patches := make([]Patch, len(pd.Patches))
	for i, p := range pd.Patches {
		patches[i] = Patch{
			Name:     p.Name,
			Path:     p.Path,
			Content:  p.Content,
			Metadata: deepCopyPatchMetadata(p.Metadata),
		}
	}
	
	return &PackageDefinition{
		Path:       pd.Path,
		Metadata:   pd.Metadata, // struct copy
		Parameters: deepCopyParameterMap(pd.Parameters),
		Resources:  resources,
		Patches:    patches,
	}
}

// PackageInstance represents a package with user customization
type PackageInstance struct {
	Definition *PackageDefinition      `json:"definition"`
	UserValues ParameterMap            `json:"userValues"`
	Resolved   ParameterMapWithSource  `json:"resolved"` // Final values with source tracking
	LocalPath  string                  `json:"localPath,omitempty"`
}

// Helper functions for deep copying

func deepCopyParameterMap(m ParameterMap) ParameterMap {
	if m == nil {
		return nil
	}
	
	result := make(ParameterMap)
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}
	return result
}

func deepCopyValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = deepCopyValue(v)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = deepCopyValue(v)
		}
		return result
	default:
		// Primitive types are already immutable
		return val
	}
}

func deepCopyPatchMetadata(pm *PatchMetadata) *PatchMetadata {
	if pm == nil {
		return nil
	}
	
	requires := make([]string, len(pm.Requires))
	copy(requires, pm.Requires)
	
	conflicts := make([]string, len(pm.Conflicts))
	copy(conflicts, pm.Conflicts)
	
	return &PatchMetadata{
		Enabled:     pm.Enabled,
		Description: pm.Description,
		Requires:    requires,
		Conflicts:   conflicts,
	}
}