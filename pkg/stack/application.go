package stack

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
)

// Application represents a deployable application with a configuration.
type Application struct {
	Name      string
	Namespace string
	Config    ApplicationConfig
}

// ApplicationConfig describes the behaviour of specific application types.
type ApplicationConfig interface {
	Generate(*Application) ([]*client.Object, error)
}

// NewApplication constructs an Application with the provided parameters.
func NewApplication(name, namespace string, cfg ApplicationConfig) *Application {
	return &Application{Name: name, Namespace: namespace, Config: cfg}
}

// SetName updates the application name.
func (a *Application) SetName(name string) { a.Name = name }

// SetNamespace updates the target namespace.
func (a *Application) SetNamespace(ns string) { a.Namespace = ns }

// SetConfig replaces the application configuration.
func (a *Application) SetConfig(cfg ApplicationConfig) { a.Config = cfg }

// Generate returns the resources for this application.
func (a *Application) Generate() ([]*client.Object, error) {
	if a.Config == nil {
		return nil, errors.NewValidationError("application.config", "nil", "Required", []string{"non-nil application config"})
	}
	return a.Config.Generate(a)
}
