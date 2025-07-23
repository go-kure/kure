package appset

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AppSet represents a unit of deployment, typically the resources that
// are reconciled by a single Flux Kustomization.
type AppSet struct {
	// Name identifies the application set.
	Name string
	// Namespace is the target namespace for the resources. It may be empty
	// for cluster scoped objects.
	Namespace string
	// Resources holds the Kubernetes objects that belong to the application.
	Resources []client.Object
	// Labels are common labels that should be applied to each resource.
	Labels map[string]string
}

// New constructs an AppSet with the given name, resources and labels.
// It returns an error if validation fails.
func New(name string, resources []client.Object, labels map[string]string) (*AppSet, error) {
	a := &AppSet{Name: name, Namespace: "", Resources: resources, Labels: labels}
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

// Validate performs basic sanity checks on the AppSet.
func (a *AppSet) Validate() error {
	if a == nil {
		return fmt.Errorf("nil AppSet")
	}
	if a.Name == "" {
		return fmt.Errorf("name is required")
	}
	for i, r := range a.Resources {
		if r == nil {
			return fmt.Errorf("resource %d is nil", i)
		}
	}
	return nil
}

// LabelRule defines a subset selection rule based on labels.
type LabelRule struct {
	// Name identifies the subset created from matching resources.
	Name string
	// Match specifies labels that must be present on a resource for it
	// to belong to the subset.
	Match map[string]string
}

func copyMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func labelsMatch(labels map[string]string, match map[string]string) bool {
	for k, v := range match {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// SplitByLabels divides the AppSet into new AppSets based on the provided
// label rules. Resources matching a rule are placed into the corresponding
// subset. Resources that do not match any rule remain in the original set.
func (a *AppSet) SplitByLabels(rules []LabelRule) ([]*AppSet, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}

	subsets := make([]*AppSet, len(rules))
	for i, r := range rules {
		subsets[i] = &AppSet{
			Name:      r.Name,
			Namespace: a.Namespace,
			Labels:    mergeLabels(a.Labels, r.Match),
		}
	}

	var remaining []client.Object
	for _, obj := range a.Resources {
		matched := false
		lbls := obj.GetLabels()
		for i, r := range rules {
			if labelsMatch(lbls, r.Match) {
				subsets[i].Resources = append(subsets[i].Resources, obj)
				matched = true
				break
			}
		}
		if !matched {
			remaining = append(remaining, obj)
		}
	}

	// Filter out empty subsets
	var result []*AppSet
	for _, s := range subsets {
		if len(s.Resources) > 0 {
			result = append(result, s)
		}
	}

	// Update current AppSet to hold remaining resources
	a.Resources = remaining
	if len(a.Resources) > 0 {
		result = append(result, a)
	}
	return result, nil
}

func mergeLabels(base, add map[string]string) map[string]string {
	out := copyMap(base)
	for k, v := range add {
		if out == nil {
			out = map[string]string{}
		}
		out[k] = v
	}
	return out
}
