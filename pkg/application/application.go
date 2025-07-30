package application

import (
	"fmt"

	intk8s "github.com/go-kure/kure/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Application represents a deployable application with a configuration.
type Application struct {
	Name      string
	Namespace string
	Config    ApplicationConfig
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

// Create invokes the underlying config to create new resources.
func (a *Application) Create() ([]client.Object, error) {
	if a.Config == nil {
		return nil, fmt.Errorf("application config is nil")
	}
	return a.Config.Create(a)
}

// Update allows the config to mutate existing resources.
func (a *Application) Update(objs []client.Object) error {
	if a.Config == nil {
		return fmt.Errorf("application config is nil")
	}
	return a.Config.Update(a, objs)
}

// Generate returns the resources for this application.
func (a *Application) Generate() ([]client.Object, error) {
	if a.Config == nil {
		return nil, fmt.Errorf("application config is nil")
	}
	return a.Config.Generate(a)
}

// ApplicationConfig describes the behaviour of specific application types.
type ApplicationConfig interface {
	Create(*Application) ([]client.Object, error)
	Update(*Application, []client.Object) error
	Generate(*Application) ([]client.Object, error)
}

// Create generates new resources for an AppWorkloadConfig.
func (cfg *AppWorkloadConfig) Create(app *Application) ([]client.Object, error) {
	return cfg.Generate(app)
}

// Update currently performs no operation for AppWorkloadConfig.
func (cfg *AppWorkloadConfig) Update(app *Application, objs []client.Object) error {
	return nil
}

// Generate builds Kubernetes resources for the application workload.
func (cfg *AppWorkloadConfig) Generate(app *Application) ([]client.Object, error) {
	var objs []client.Object

	// Determine workload type
	switch cfg.Workload {
	case StatefulSetWorkload:
		sts := intk8s.CreateStatefulSet(app.Name, app.Namespace)
		container := intk8s.CreateContainer(app.Name, cfg.Image, nil, nil)
		for _, p := range cfg.Ports {
			_ = intk8s.AddContainerPort(container, corev1.ContainerPort{ContainerPort: int32(p)})
		}
		if err := intk8s.AddStatefulSetContainer(sts, container); err != nil {
			return nil, err
		}
		if cfg.Replicas != nil {
			_ = intk8s.SetStatefulSetReplicas(sts, int32(*cfg.Replicas))
		}
		objs = append(objs, sts)
	case DaemonSetWorkload:
		ds := intk8s.CreateDaemonSet(app.Name, app.Namespace)
		container := intk8s.CreateContainer(app.Name, cfg.Image, nil, nil)
		for _, p := range cfg.Ports {
			_ = intk8s.AddContainerPort(container, corev1.ContainerPort{ContainerPort: int32(p)})
		}
		if err := intk8s.AddDaemonSetContainer(ds, container); err != nil {
			return nil, err
		}
		objs = append(objs, ds)
	default:
		dep := intk8s.CreateDeployment(app.Name, app.Namespace)
		container := intk8s.CreateContainer(app.Name, cfg.Image, nil, nil)
		for _, p := range cfg.Ports {
			_ = intk8s.AddContainerPort(container, corev1.ContainerPort{ContainerPort: int32(p)})
		}
		if err := intk8s.AddDeploymentContainer(dep, container); err != nil {
			return nil, err
		}
		if cfg.Replicas != nil {
			_ = intk8s.SetDeploymentReplicas(dep, int32(*cfg.Replicas))
		}
		objs = append(objs, dep)
	}

	// Service creation when ports are specified
	var svc *corev1.Service
	if len(cfg.Ports) > 0 {
		svc = intk8s.CreateService(app.Name, app.Namespace)
		_ = intk8s.SetServiceSelector(svc, map[string]string{"app": app.Name})
		for _, p := range cfg.Ports {
			_ = intk8s.AddServicePort(svc, corev1.ServicePort{
				Name:       fmt.Sprintf("p-%d", p),
				Port:       int32(p),
				TargetPort: intstr.FromInt(p),
			})
		}
		objs = append(objs, svc)
	}

	if cfg.Ingress != nil && svc != nil {
		ing := intk8s.CreateIngress(app.Name, app.Namespace, "")
		rule := intk8s.CreateIngressRule(cfg.Ingress.Host)
		pt := netv1.PathTypeImplementationSpecific
		path := cfg.Ingress.Path
		if path == "" {
			path = "/"
		}
		intk8s.AddIngressRulePath(rule, intk8s.CreateIngressPath(path, &pt, svc.Name, "p-"+fmt.Sprint(cfg.Ports[0])))
		intk8s.AddIngressRule(ing, rule)
		if cfg.Ingress.TLS {
			tls := netv1.IngressTLS{Hosts: []string{cfg.Ingress.Host}}
			intk8s.AddIngressTLS(ing, tls)
		}
		objs = append(objs, ing)
	}

	return objs, nil
}
