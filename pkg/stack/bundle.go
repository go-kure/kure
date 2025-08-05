package stack

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
)

// Bundle represents a unit of deployment, typically the resources that
// are reconciled by a single Flux Kustomization.
type Bundle struct {
	// Name identifies the application set.
	Name string
	// ParentPath is the hierarchical path to the parent bundle (e.g., "cluster/infrastructure")
	// Empty for root bundles. This avoids circular references while maintaining hierarchy.
	ParentPath string
	// DependsOn lists other bundles this bundle depends on
	DependsOn []*Bundle
	// Interval controls how often Flux reconciles the bundle.
	Interval string
	// SourceRef specifies the source for the bundle.
	SourceRef *SourceRef
	// Applications holds the Kubernetes objects that belong to the application.
	Applications []*Application
	// Labels are common labels that should be applied to each resource.
	Labels map[string]string
	
	// Internal fields for runtime hierarchy navigation (not serialized)
	parent   *Bundle            `yaml:"-"` // Runtime parent reference for efficient traversal
	pathMap  map[string]*Bundle `yaml:"-"` // Runtime path lookup map (shared across tree)
}

// SourceRef defines a reference to a Flux source.
type SourceRef struct {
	Kind      string
	Name      string
	Namespace string
}

// NewBundle constructs a Bundle with the given name, resources and labels.
// It returns an error if validation fails.
func NewBundle(name string, resources []*Application, labels map[string]string) (*Bundle, error) {
	a := &Bundle{Name: name, Applications: resources, Labels: labels}
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

// Validate performs basic sanity checks on the Bundle.
func (a *Bundle) Validate() error {
	if a == nil {
		return errors.ErrNilBundle
	}
	if a.Name == "" {
		return errors.NewValidationError("name", "", "Bundle", nil)
	}
	for i, r := range a.Applications {
		if r == nil {
			return errors.NewResourceValidationError("Bundle", a.Name, "applications", fmt.Sprintf("application at index %d is nil", i), nil)
		}
	}
	return nil
}

func (a *Bundle) Generate() ([]*client.Object, error) {
	var resources []*client.Object
	for _, app := range a.Applications {
		addresources, err := app.Generate()
		if err != nil {
			return nil, err
		}
		resources = append(resources, addresources...)
	}
	return resources, nil
}

// GetParent returns the runtime parent reference (may be nil).
func (b *Bundle) GetParent() *Bundle {
	return b.parent
}

// GetParentPath returns the hierarchical path to the parent bundle.
func (b *Bundle) GetParentPath() string {
	return b.ParentPath
}

// SetParent sets the parent bundle and updates the ParentPath accordingly.
// This method maintains both the serializable path and runtime reference.
func (b *Bundle) SetParent(parent *Bundle) {
	b.parent = parent
	if parent == nil {
		b.ParentPath = ""
	} else {
		b.ParentPath = parent.GetPath()
	}
}

// GetPath returns the full hierarchical path of this bundle.
func (b *Bundle) GetPath() string {
	if b.ParentPath == "" {
		return b.Name
	}
	return b.ParentPath + "/" + b.Name
}

// InitializePathMap builds the runtime path lookup map for efficient hierarchy navigation.
// This should be called on the root bundle after the tree structure is complete.
func (b *Bundle) InitializePathMap(allBundles []*Bundle) {
	pathMap := make(map[string]*Bundle)
	
	// Build path map for all bundles
	for _, bundle := range allBundles {
		if bundle.Name != "" {
			pathMap[bundle.GetPath()] = bundle
		}
	}
	
	// Set path map and parent references on all bundles
	for _, bundle := range allBundles {
		bundle.pathMap = pathMap
		if bundle.ParentPath != "" {
			if parent, exists := pathMap[bundle.ParentPath]; exists {
				bundle.parent = parent
			}
		}
	}
}
