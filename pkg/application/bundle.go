package application

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Bundle represents a unit of deployment, typically the resources that
// are reconciled by a single Flux Kustomization.
type Bundle struct {
	// Name identifies the application set.
	Name string
	// Parent identifies the parent bundle
	Parent *Bundle
	// DependsOn lists other bundles this bundle depends on
	DependsOn []*Bundle
	// Applications holds the Kubernetes objects that belong to the application.
	Applications *[]*Application
	// Labels are common labels that should be applied to each resource.
	Labels map[string]string
}

// NewBundle constructs a Bundle with the given name, resources and labels.
// It returns an error if validation fails.
func NewBundle(name string, resources *[]*Application, labels map[string]string) (*Bundle, error) {
	a := &Bundle{Name: name, Applications: resources, Labels: labels}
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

// Validate performs basic sanity checks on the Bundle.
func (a *Bundle) Validate() error {
	if a == nil {
		return fmt.Errorf("nil Bundle")
	}
	if a.Name == "" {
		return fmt.Errorf("name is required")
	}
	for i, r := range *a.Applications {
		if r == nil {
			return fmt.Errorf("resource %d is nil", i)
		}
	}
	return nil
}

func (a *Bundle) Generate() ([]*client.Object, error) {
	var resources []*client.Object
	for _, app := range *a.Applications {
		addresources, err := app.Generate()
		if err != nil {
			return nil, err
		}
		resources = append(resources, addresources...)
	}
	return resources, nil
}
