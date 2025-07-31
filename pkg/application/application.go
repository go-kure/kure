package application

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Application represents a deployable application with a configuration.
type Application struct {
	Name      string
	Namespace string
	Config    Config
}

// Config describes the behaviour of specific application types.
type Config interface {
	Generate(*Application) ([]*client.Object, error)
}

// NewApplication constructs an Application with the provided parameters.
func NewApplication(name, namespace string, cfg Config) *Application {
	return &Application{Name: name, Namespace: namespace, Config: cfg}
}

// SetName updates the application name.
func (a *Application) SetName(name string) { a.Name = name }

// SetNamespace updates the target namespace.
func (a *Application) SetNamespace(ns string) { a.Namespace = ns }

// SetConfig replaces the application configuration.
func (a *Application) SetConfig(cfg Config) { a.Config = cfg }

// Generate returns the resources for this application.
func (a *Application) Generate() ([]*client.Object, error) {
	if a.Config == nil {
		return nil, fmt.Errorf("application config is nil")
	}
	return a.Config.Generate(a)
}
