package v1alpha1

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
	
	"github.com/go-kure/kure/pkg/stack"
)

// StackDocument represents a parsed stack document with its type information
type StackDocument struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Resource   interface{} // The actual resource (ClusterV1Alpha1, NodeV1Alpha1, or BundleV1Alpha1)
}

// ParseStackDocuments parses multiple YAML documents containing stack resources
func ParseStackDocuments(reader io.Reader) ([]StackDocument, error) {
	var documents []StackDocument
	decoder := yaml.NewDecoder(reader)
	
	for {
		// Parse each document manually
		var raw struct {
			APIVersion string    `yaml:"apiVersion"`
			Kind       string    `yaml:"kind"`
			Name       string    `yaml:"name"`
			Spec       yaml.Node `yaml:"spec"`
		}
		
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode YAML document: %w", err)
		}
		
		// Create appropriate resource based on kind
		var resource interface{}
		var err error
		
		switch raw.Kind {
		case "Cluster":
			var spec ClusterSpec
			if err = raw.Spec.Decode(&spec); err != nil {
				return nil, fmt.Errorf("failed to decode Cluster spec: %w", err)
			}
			cluster := &ClusterV1Alpha1{Spec: spec}
			cluster.Name = raw.Name
			resource = cluster
			
		case "Node":
			var spec NodeSpec
			if err = raw.Spec.Decode(&spec); err != nil {
				return nil, fmt.Errorf("failed to decode Node spec: %w", err)
			}
			node := &NodeV1Alpha1{Spec: spec}
			node.Name = raw.Name
			resource = node
			
		case "Bundle":
			var spec BundleSpec
			if err = raw.Spec.Decode(&spec); err != nil {
				return nil, fmt.Errorf("failed to decode Bundle spec: %w", err)
			}
			bundle := &BundleV1Alpha1{Spec: spec}
			bundle.Name = raw.Name
			resource = bundle
			
		default:
			return nil, fmt.Errorf("unknown kind: %s", raw.Kind)
		}
		
		documents = append(documents, StackDocument{
			APIVersion: raw.APIVersion,
			Kind:       raw.Kind,
			Resource:   resource,
		})
	}
	
	return documents, nil
}

// ParseAndConvertCluster parses a YAML document and converts it to an unversioned Cluster
func ParseAndConvertCluster(data []byte) (*stack.Cluster, error) {
	// Parse the YAML manually to handle our specific structure
	var raw struct {
		APIVersion string       `yaml:"apiVersion"`
		Kind       string       `yaml:"kind"`
		Name       string       `yaml:"name"`
		Spec       ClusterSpec  `yaml:"spec"`
	}
	
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster: %w", err)
	}
	
	if raw.Kind != "Cluster" {
		return nil, fmt.Errorf("expected kind Cluster, got %s", raw.Kind)
	}
	
	cluster := &ClusterV1Alpha1{
		Spec: raw.Spec,
	}
	cluster.Name = raw.Name
	
	return cluster.ToUnversioned(), nil
}

// ParseAndConvertNode parses a YAML document and converts it to an unversioned Node
func ParseAndConvertNode(data []byte) (*stack.Node, error) {
	// Parse the YAML manually to handle our specific structure
	var raw struct {
		APIVersion string   `yaml:"apiVersion"`
		Kind       string   `yaml:"kind"`
		Name       string   `yaml:"name"`
		Spec       NodeSpec `yaml:"spec"`
	}
	
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node: %w", err)
	}
	
	if raw.Kind != "Node" {
		return nil, fmt.Errorf("expected kind Node, got %s", raw.Kind)
	}
	
	node := &NodeV1Alpha1{
		Spec: raw.Spec,
	}
	node.Name = raw.Name
	
	return node.ToUnversioned(), nil
}

// ParseAndConvertBundle parses a YAML document and converts it to an unversioned Bundle
func ParseAndConvertBundle(data []byte) (*stack.Bundle, error) {
	// Parse the YAML manually to handle our specific structure
	var raw struct {
		APIVersion string     `yaml:"apiVersion"`
		Kind       string     `yaml:"kind"`
		Name       string     `yaml:"name"`
		Spec       BundleSpec `yaml:"spec"`
	}
	
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bundle: %w", err)
	}
	
	if raw.Kind != "Bundle" {
		return nil, fmt.Errorf("expected kind Bundle, got %s", raw.Kind)
	}
	
	bundle := &BundleV1Alpha1{
		Spec: raw.Spec,
	}
	bundle.Name = raw.Name
	
	return bundle.ToUnversioned(), nil
}

// ConvertDocument converts a StackDocument to its unversioned equivalent
func ConvertDocument(doc StackDocument) (interface{}, error) {
	switch doc.Kind {
	case "Cluster":
		cluster, ok := doc.Resource.(*ClusterV1Alpha1)
		if !ok {
			return nil, fmt.Errorf("unexpected type for Cluster: %T", doc.Resource)
		}
		return cluster.ToUnversioned(), nil
		
	case "Node":
		node, ok := doc.Resource.(*NodeV1Alpha1)
		if !ok {
			return nil, fmt.Errorf("unexpected type for Node: %T", doc.Resource)
		}
		return node.ToUnversioned(), nil
		
	case "Bundle":
		bundle, ok := doc.Resource.(*BundleV1Alpha1)
		if !ok {
			return nil, fmt.Errorf("unexpected type for Bundle: %T", doc.Resource)
		}
		return bundle.ToUnversioned(), nil
		
	default:
		return nil, fmt.Errorf("unknown kind: %s", doc.Kind)
	}
}