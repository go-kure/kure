package v1alpha1

import (
	"gopkg.in/yaml.v3"
	
	"github.com/go-kure/kure/internal/gvk"
)

// stackRegistry is the registry for stack types
var stackRegistry = gvk.NewRegistry[interface{}]()

func init() {
	// Register Cluster type
	stackRegistry.Register(gvk.GVK{
		Group:   GroupName,
		Version: Version,
		Kind:    "Cluster",
	}, func() interface{} {
		return &ClusterV1Alpha1{}
	})
	
	// Register Node type
	stackRegistry.Register(gvk.GVK{
		Group:   GroupName,
		Version: Version,
		Kind:    "Node",
	}, func() interface{} {
		return &NodeV1Alpha1{}
	})
	
	// Register Bundle type
	stackRegistry.Register(gvk.GVK{
		Group:   GroupName,
		Version: Version,
		Kind:    "Bundle",
	}, func() interface{} {
		return &BundleV1Alpha1{}
	})
}

// GetRegistry returns the stack registry for external use
func GetRegistry() *gvk.Registry[interface{}] {
	return stackRegistry
}

// CreateStackResource creates a new stack resource for the given apiVersion and kind
func CreateStackResource(apiVersion, kind string) (interface{}, error) {
	parsed := gvk.ParseAPIVersion(apiVersion, kind)
	return stackRegistry.Create(parsed)
}

// ParseStackYAML parses YAML data containing a stack resource
func ParseStackYAML(data []byte) (interface{}, error) {
	wrapper := gvk.NewTypedWrapper(stackRegistry)
	// Convert byte data to yaml.Node for unmarshaling
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	if err := wrapper.UnmarshalYAML(&node); err != nil {
		return nil, err
	}
	return wrapper.Spec, nil
}