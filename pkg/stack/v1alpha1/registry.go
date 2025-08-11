package v1alpha1

import (
	"fmt"
	"sync"

	"github.com/go-kure/kure/internal/gvk"
)

// StackConfigType represents the type of stack configuration
type StackConfigType string

const (
	// StackConfigTypeCluster represents a Cluster configuration
	StackConfigTypeCluster StackConfigType = "Cluster"

	// StackConfigTypeNode represents a Node configuration
	StackConfigTypeNode StackConfigType = "Node"

	// StackConfigTypeBundle represents a Bundle configuration
	StackConfigTypeBundle StackConfigType = "Bundle"
)

// StackConfig is a common interface for all stack configuration types
type StackConfig interface {
	gvk.VersionedType
	gvk.MetadataType
	Validate() error
}

// StackConfigFactory creates stack configurations from raw data
type StackConfigFactory func() StackConfig

// StackRegistry manages registration and creation of stack configurations
type StackRegistry struct {
	mu        sync.RWMutex
	factories map[gvk.GVK]StackConfigFactory
}

// globalRegistry is the singleton registry instance
var globalRegistry = &StackRegistry{
	factories: make(map[gvk.GVK]StackConfigFactory),
}

// init registers the default v1alpha1 stack types
func init() {
	// Register Cluster
	RegisterStackConfig(gvk.GVK{
		Group:   "stack.gokure.dev",
		Version: "v1alpha1",
		Kind:    "Cluster",
	}, func() StackConfig {
		return &ClusterConfig{}
	})

	// Register Node
	RegisterStackConfig(gvk.GVK{
		Group:   "stack.gokure.dev",
		Version: "v1alpha1",
		Kind:    "Node",
	}, func() StackConfig {
		return &NodeConfig{}
	})

	// Register Bundle
	RegisterStackConfig(gvk.GVK{
		Group:   "stack.gokure.dev",
		Version: "v1alpha1",
		Kind:    "Bundle",
	}, func() StackConfig {
		return &BundleConfig{}
	})
}

// RegisterStackConfig registers a stack configuration factory for a GVK
func RegisterStackConfig(gvk gvk.GVK, factory StackConfigFactory) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	globalRegistry.factories[gvk] = factory
}

// CreateStackConfig creates a new stack configuration for the given GVK
func CreateStackConfig(gvk gvk.GVK) (StackConfig, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	factory, exists := globalRegistry.factories[gvk]
	if !exists {
		return nil, fmt.Errorf("no factory registered for GVK: %s", gvk)
	}

	return factory(), nil
}

// GetRegisteredStackGVKs returns all registered stack configuration GVKs
func GetRegisteredStackGVKs() []gvk.GVK {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	gvks := make([]gvk.GVK, 0, len(globalRegistry.factories))
	for gvk := range globalRegistry.factories {
		gvks = append(gvks, gvk)
	}

	return gvks
}

// IsStackConfigRegistered checks if a GVK is registered for stack configurations
func IsStackConfigRegistered(g gvk.GVK) bool {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	_, exists := globalRegistry.factories[g]
	return exists
}

// StackWrapper provides a unified wrapper for stack configurations
type StackWrapper struct {
	gvk    gvk.GVK
	config StackConfig
}

// NewStackWrapper creates a new stack wrapper from a GVK and configuration
func NewStackWrapper(g gvk.GVK, config StackConfig) *StackWrapper {
	return &StackWrapper{
		gvk:    g,
		config: config,
	}
}

// CreateStackWrapper creates a new stack wrapper for the given GVK
func CreateStackWrapper(g gvk.GVK) (*StackWrapper, error) {
	config, err := CreateStackConfig(g)
	if err != nil {
		return nil, err
	}

	return &StackWrapper{
		gvk:    g,
		config: config,
	}, nil
}

// GetGVK returns the GVK of the wrapped configuration
func (w *StackWrapper) GetGVK() gvk.GVK {
	return w.gvk
}

// GetConfig returns the wrapped configuration
func (w *StackWrapper) GetConfig() StackConfig {
	return w.config
}

// GetType returns the stack configuration type
func (w *StackWrapper) GetType() StackConfigType {
	switch w.gvk.Kind {
	case "Cluster":
		return StackConfigTypeCluster
	case "Node":
		return StackConfigTypeNode
	case "Bundle":
		return StackConfigTypeBundle
	default:
		return StackConfigType(w.gvk.Kind)
	}
}

// Validate validates the wrapped configuration
func (w *StackWrapper) Validate() error {
	if w.config == nil {
		return fmt.Errorf("wrapped configuration is nil")
	}
	return w.config.Validate()
}

// AsCluster returns the configuration as a ClusterConfig if it is one
func (w *StackWrapper) AsCluster() (*ClusterConfig, bool) {
	cluster, ok := w.config.(*ClusterConfig)
	return cluster, ok
}

// AsNode returns the configuration as a NodeConfig if it is one
func (w *StackWrapper) AsNode() (*NodeConfig, bool) {
	node, ok := w.config.(*NodeConfig)
	return node, ok
}

// AsBundle returns the configuration as a BundleConfig if it is one
func (w *StackWrapper) AsBundle() (*BundleConfig, bool) {
	bundle, ok := w.config.(*BundleConfig)
	return bundle, ok
}
