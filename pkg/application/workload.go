package application

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/k8s"
)

// WorkloadType enumerates the supported Kubernetes workload kinds.
type WorkloadType string

const (
	DeploymentWorkload  WorkloadType = "Deployment"
	StatefulSetWorkload WorkloadType = "StatefulSet"
	DaemonSetWorkload   WorkloadType = "DaemonSet"
)

// AppWorkloadConfig describes a single deployable application.
type AppWorkloadConfig struct {
	Name      string            `json:"name" yaml:"name"`
	Namespace string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Workload  WorkloadType      `yaml:"workload,omitempty"`
	Replicas  int32             `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Labels    map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	Containers []ContainerConfig `json:"containers" yaml:"containers"`
	Volumes    map[string]string `json:"volumes,omitempty" yaml:"volumes,omitempty"` // volumeName -> hostPath

	Services []ServiceConfig `json:"services,omitempty" yaml:"services,omitempty"`
	Ingress  *IngressConfig  `json:"ingress,omitempty" yaml:"ingress,omitempty"`
}

type ContainerConfig struct {
	Name         string                 `json:"name" yaml:"name"`
	Image        string                 `json:"image" yaml:"image"`
	Ports        []corev1.ContainerPort `json:"ports,omitempty" yaml:"ports,omitempty"`
	Env          map[string]string      `json:"env,omitempty" yaml:"env,omitempty"`
	VolumeMounts map[string]string      `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"` // mountPath -> volumeName

	Resources *corev1.ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`

	StartupProbe   *corev1.Probe `json:"startupProbe,omitempty" yaml:"startupProbe,omitempty"`
	LivenessProbe  *corev1.Probe `json:"livenessProbe,omitempty" yaml:"livenessProbe,omitempty"`
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty" yaml:"readinessProbe,omitempty"`
}

type ServiceConfig struct {
	Name       string             `json:"name" yaml:"name"`
	Type       corev1.ServiceType `json:"type,omitempty" yaml:"type,omitempty"`
	Port       int32              `json:"port" yaml:"port"`             // service port
	TargetPort int32              `json:"targetPort" yaml:"targetPort"` // container port
	Protocol   corev1.Protocol    `json:"protocol,omitempty" yaml:"protocol,omitempty"`

	// Optional explicit selector override (else falls back to Deployment labels)
	Selector map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`
}

type IngressConfig struct {
	Host            string `json:"host" yaml:"host"`
	Path            string `json:"path" yaml:"path"`
	ServiceName     string `json:"serviceName" yaml:"serviceName"`
	ServicePortName string `json:"servicePortName" yaml:"servicePortName"`
}

// Generate builds Kubernetes resources for the application workload.
func (cfg *AppWorkloadConfig) Generate(app *Application) ([]client.Object, error) {
	var objs []client.Object
	var allports []corev1.ContainerPort

	var containers []*corev1.Container
	for _, c := range cfg.Containers {
		container, ports, err := c.Generate()
		if err != nil {
			return nil, err
		}
		containers = append(containers, container)
		allports = append(allports, ports...)
	}
	// Determine workload type
	switch cfg.Workload {
	case StatefulSetWorkload:
		sts := k8s.CreateStatefulSet(app.Name, app.Namespace)
		for _, c := range containers {
			if err := k8s.AddStatefulSetContainer(sts, c); err != nil {
				return nil, err
			}
		}
		_ = k8s.SetStatefulSetReplicas(sts, cfg.Replicas)
		objs = append(objs, sts)
	case DaemonSetWorkload:
		ds := k8s.CreateDaemonSet(app.Name, app.Namespace)
		for _, c := range containers {
			if err := k8s.AddDaemonSetContainer(ds, c); err != nil {
				return nil, err
			}
		}
		objs = append(objs, ds)
	case DeploymentWorkload:
		dep := k8s.CreateDeployment(app.Name, app.Namespace)
		for _, c := range containers {
			if err := k8s.AddDeploymentContainer(dep, c); err != nil {
				return nil, err
			}
		}
		_ = k8s.SetDeploymentReplicas(dep, cfg.Replicas)
		objs = append(objs, dep)
	default:
		return nil, fmt.Errorf("unsupported workload type %s", cfg.Workload)
	}

	// Service creation when ports are specified
	var svc *corev1.Service
	if len(allports) > 0 {
		svc = k8s.CreateService(app.Name, app.Namespace)
		_ = k8s.SetServiceSelector(svc, map[string]string{"app": app.Name})
		for _, p := range allports {
			_ = k8s.AddServicePort(svc, corev1.ServicePort{
				Name:       p.Name,
				Port:       p.HostPort,
				TargetPort: intstr.FromInt32(p.HostPort),
			})
		}
		objs = append(objs, svc)
	}

	if cfg.Ingress != nil && svc != nil {
		ing := k8s.CreateIngress(app.Name, app.Namespace, "")
		rule := k8s.CreateIngressRule(cfg.Ingress.Host)
		pt := netv1.PathTypeImplementationSpecific
		path := cfg.Ingress.Path
		if path == "" {
			path = "/"
		}
		port := cfg.Ingress.ServicePortName
		k8s.AddIngressRulePath(rule, k8s.CreateIngressPath(path, &pt, svc.Name, port))
		k8s.AddIngressRule(ing, rule)
		k8s.AddIngressTLS(ing, netv1.IngressTLS{Hosts: []string{cfg.Ingress.Host}, SecretName: fmt.Sprintf("%s-tls", app.Name)})
		objs = append(objs, ing)
		objs = append(objs, ing)
	}

	return objs, nil
}

func (cfg ContainerConfig) Generate() (*corev1.Container, []corev1.ContainerPort, error) {
	container := k8s.CreateContainer(cfg.Name, cfg.Image, nil, nil)
	var ports []corev1.ContainerPort
	for _, p := range cfg.Ports {
		_ = k8s.AddContainerPort(container, p)
		ports = append(ports, p)
	}
	for k, v := range cfg.VolumeMounts {
		volume := corev1.VolumeMount{
			Name:      k,
			MountPath: v,
		}
		_ = k8s.AddContainerVolumeMount(container, volume)
	}
	return container, ports, nil
}
