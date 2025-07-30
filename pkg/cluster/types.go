package cluster

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/application"
	"github.com/go-kure/kure/pkg/fluxcd"
)

// Config is the root configuration for a cluster layout.
type Config struct {
	Name              string                      `yaml:"name"`
	Interval          string                      `yaml:"interval"`
	SourceRef         string                      `yaml:"sourceRef"`
	OCIRepo           *fluxcd.OCIRepositoryConfig `yaml:"ociRepo,omitempty"`
	ApplicationGroups []application.Bundle        `yaml:"appGroups,omitempty"`
}

// Cluster describes a cluster configuration.
// A cluster configuration is a set of configurations that are packaged in one or more package units
type Cluster struct {
	Name string `yaml:"name"`
	Tree *Tree  `yaml:"tree,omitempty"`
}

// Tree represents a hierarchic structure holding all deployment bundles
// each tree has a list of children, which can be a deployment, or a subtree
// It could match a kubernetes cluster's full configuration, or it could be just
// a part of that, when parts are e.g. packaged in different OCI artifacts
// Tree's with a common PackageRef are packaged together
type Tree struct {
	// Name identifies the application set.
	Name string
	// Bundles list child bundles
	Children []*Child
	// PackageRef identifies in which package the tree of resources get bundled together
	// If undefined, the PackageRef of the parent is inherited
	PackageRef *schema.GroupVersionKind
}

// Child represents an item in the tree
type Child struct {
	// Tree identifies a subtree
	Tree *Tree
	// Bundle identifies a deployment
	Bundle *application.Bundle
}
