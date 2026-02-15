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

// Validator is an optional interface that ApplicationConfig implementations
// can implement to validate their configuration before generation.
// If an ApplicationConfig also implements Validator, Application.Generate()
// calls Validate() automatically before calling Generate().
type Validator interface {
	Validate() error
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
// If the Config implements the Validator interface, Validate() is called
// before Generate(). A validation error stops generation immediately.
func (a *Application) Generate() ([]*client.Object, error) {
	if a.Config == nil {
		return nil, errors.NewValidationError("application.config", "nil", "Required", []string{"non-nil application config"})
	}

	if validator, ok := a.Config.(Validator); ok {
		if err := validator.Validate(); err != nil {
			return nil, errors.Wrapf(err,
				"validation failed for application %q in namespace %q",
				a.Name, a.Namespace)
		}
	}

	return a.Config.Generate(a)
}
