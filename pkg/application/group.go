package application

import (
	"fmt"
)

// Group represents a unit of deployment, typically the resources that
// are reconciled by a single Flux Kustomization.
type Group struct {
	// Name identifies the application set.
	Name string
	// Applications holds the Kubernetes objects that belong to the application.
	Applications *[]*Application
	// Labels are common labels that should be applied to each resource.
	Labels map[string]string
}

// New constructs an Group with the given name, resources and labels.
// It returns an error if validation fails.
func New(name string, resources *[]*Application, labels map[string]string) (*Group, error) {
	a := &Group{Name: name, Applications: resources, Labels: labels}
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

// Validate performs basic sanity checks on the Group.
func (a *Group) Validate() error {
	if a == nil {
		return fmt.Errorf("nil Group")
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
