package application

import (
	"fmt"

	"k8s.io/api/core/v1"
	v2 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/k8s"
)

// AppWorkloadConfig describes a single deployable application.
type AppWorkloadConfig struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace,omitempty"`
	Image     string            `yaml:"image"`
	Ports     []int32           `yaml:"ports,omitempty"`
	Replicas  *int              `yaml:"replicas,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	Secrets   map[string]string `yaml:"secrets,omitempty"`
	Ingress   *IngressConfig    `yaml:"ingress,omitempty"`
	Resources map[string]string `yaml:"resources,omitempty"`
	Workload  WorkloadType      `yaml:"workload,omitempty"`
}

// WorkloadType enumerates the supported Kubernetes workload kinds.
type WorkloadType string

const (
	DeploymentWorkload  WorkloadType = "Deployment"
	StatefulSetWorkload WorkloadType = "StatefulSet"
	DaemonSetWorkload   WorkloadType = "DaemonSet"
)

// IngressConfig defines ingress settings for an application.
type IngressConfig struct {
	Host          string `yaml:"host"`
	Path          string `yaml:"path,omitempty"`
	TLS           bool   `yaml:"tls,omitempty"`
	Issuer        string `yaml:"issuer,omitempty"`
	UseACMEHTTP01 bool   `yaml:"useACMEHTTP01,omitempty"`
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
		sts := k8s.CreateStatefulSet(app.Name, app.Namespace)
		container := k8s.CreateContainer(app.Name, cfg.Image, nil, nil)
		for _, p := range cfg.Ports {
			_ = k8s.AddContainerPort(container, v1.ContainerPort{ContainerPort: p})
		}
		if err := k8s.AddStatefulSetContainer(sts, container); err != nil {
			return nil, err
		}
		if cfg.Replicas != nil {
			_ = k8s.SetStatefulSetReplicas(sts, int32(*cfg.Replicas))
		}
		objs = append(objs, sts)
	case DaemonSetWorkload:
		ds := k8s.CreateDaemonSet(app.Name, app.Namespace)
		container := k8s.CreateContainer(app.Name, cfg.Image, nil, nil)
		for _, p := range cfg.Ports {
			_ = k8s.AddContainerPort(container, v1.ContainerPort{ContainerPort: p})
		}
		if err := k8s.AddDaemonSetContainer(ds, container); err != nil {
			return nil, err
		}
		objs = append(objs, ds)
	default:
		dep := k8s.CreateDeployment(app.Name, app.Namespace)
		container := k8s.CreateContainer(app.Name, cfg.Image, nil, nil)
		for _, p := range cfg.Ports {
			_ = k8s.AddContainerPort(container, v1.ContainerPort{ContainerPort: p})
		}
		if err := k8s.AddDeploymentContainer(dep, container); err != nil {
			return nil, err
		}
		if cfg.Replicas != nil {
			_ = k8s.SetDeploymentReplicas(dep, int32(*cfg.Replicas))
		}
		objs = append(objs, dep)
	}

	// Service creation when ports are specified
	var svc *v1.Service
	if len(cfg.Ports) > 0 {
		svc = k8s.CreateService(app.Name, app.Namespace)
		_ = k8s.SetServiceSelector(svc, map[string]string{"app": app.Name})
		for _, p := range cfg.Ports {
			_ = k8s.AddServicePort(svc, v1.ServicePort{
				Name:       fmt.Sprintf("p-%d", p),
				Port:       p,
				TargetPort: intstr.FromInt32(p),
			})
		}
		objs = append(objs, svc)
	}

	if cfg.Ingress != nil && svc != nil {
		ing := k8s.CreateIngress(app.Name, app.Namespace, "")
		rule := k8s.CreateIngressRule(cfg.Ingress.Host)
		pt := v2.PathTypeImplementationSpecific
		path := cfg.Ingress.Path
		if path == "" {
			path = "/"
		}
		k8s.AddIngressRulePath(rule, k8s.CreateIngressPath(path, &pt, svc.Name, "p-"+fmt.Sprint(cfg.Ports[0])))
		k8s.AddIngressRule(ing, rule)
		if cfg.Ingress.TLS {
			tls := v2.IngressTLS{Hosts: []string{cfg.Ingress.Host}}
			k8s.AddIngressTLS(ing, tls)
		}
		objs = append(objs, ing)
	}

	return objs, nil
}
