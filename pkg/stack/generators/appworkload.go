package generators

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/kubernetes"
	"github.com/go-kure/kure/pkg/k8s"
	"github.com/go-kure/kure/pkg/stack"
)

// WorkloadType enumerates the supported Kubernetes workload kinds.
type WorkloadType string

const (
	DeploymentWorkload  WorkloadType = "Deployment"
	StatefulSetWorkload WorkloadType = "StatefulSet"
	DaemonSetWorkload   WorkloadType = "DaemonSet"
)

// ResourceRequirements wraps corev1.ResourceRequirements with custom YAML unmarshaling
type ResourceRequirements struct {
	Limits   map[string]string `json:"limits,omitempty" yaml:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty" yaml:"requests,omitempty"`
}

// ToKubernetesResources converts to standard Kubernetes ResourceRequirements
func (r *ResourceRequirements) ToKubernetesResources() (*corev1.ResourceRequirements, error) {
	if r == nil {
		return nil, nil
	}
	
	result := &corev1.ResourceRequirements{}
	
	if len(r.Limits) > 0 {
		result.Limits = make(corev1.ResourceList)
		for k, v := range r.Limits {
			qty, err := resource.ParseQuantity(v)
			if err != nil {
				return nil, fmt.Errorf("invalid resource limit %s=%s: %w", k, v, err)
			}
			result.Limits[corev1.ResourceName(k)] = qty
		}
	}
	
	if len(r.Requests) > 0 {
		result.Requests = make(corev1.ResourceList)
		for k, v := range r.Requests {
			qty, err := resource.ParseQuantity(v)
			if err != nil {
				return nil, fmt.Errorf("invalid resource request %s=%s: %w", k, v, err)
			}
			result.Requests[corev1.ResourceName(k)] = qty
		}
	}
	
	return result, nil
}

// VolumeClaimTemplate wraps PersistentVolumeClaim with custom resource parsing
type VolumeClaimTemplate struct {
	Metadata struct {
		Name string `json:"name" yaml:"name"`
	} `json:"metadata" yaml:"metadata"`
	Spec struct {
		AccessModes      []string          `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
		StorageClassName *string           `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
		Resources        *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	} `json:"spec" yaml:"spec"`
}

// ToKubernetesPVC converts to standard Kubernetes PersistentVolumeClaim
func (vct *VolumeClaimTemplate) ToKubernetesPVC() (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Name = vct.Metadata.Name
	
	// Convert access modes
	for _, mode := range vct.Spec.AccessModes {
		pvc.Spec.AccessModes = append(pvc.Spec.AccessModes, corev1.PersistentVolumeAccessMode(mode))
	}
	
	// Set storage class
	pvc.Spec.StorageClassName = vct.Spec.StorageClassName
	
	// Convert resources
	if vct.Spec.Resources != nil {
		k8sResources, err := vct.Spec.Resources.ToKubernetesResources()
		if err != nil {
			return nil, err
		}
		if k8sResources != nil {
			// Convert ResourceRequirements to VolumeResourceRequirements
			pvc.Spec.Resources = corev1.VolumeResourceRequirements{
				Limits:   k8sResources.Limits,
				Requests: k8sResources.Requests,
			}
		}
	}
	
	return pvc, nil
}

// AppWorkloadConfig describes a single deployable application.
type AppWorkloadConfig struct {
	Name      string            `json:"name" yaml:"name"`
	Namespace string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Workload  WorkloadType      `yaml:"workload,omitempty"`
	Replicas  int32             `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Labels    map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	Containers            []ContainerConfig       `json:"containers" yaml:"containers"`
	Volumes               []corev1.Volume         `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	VolumeClaimTemplates  []VolumeClaimTemplate   `json:"volumeClaimTemplates,omitempty" yaml:"volumeClaimTemplates,omitempty"`

	Services []ServiceConfig `json:"services,omitempty" yaml:"services,omitempty"`
	Ingress  *IngressConfig  `json:"ingress,omitempty" yaml:"ingress,omitempty"`
}

type ContainerConfig struct {
	Name         string                   `json:"name" yaml:"name"`
	Image        string                   `json:"image" yaml:"image"`
	Ports        []corev1.ContainerPort   `json:"ports,omitempty" yaml:"ports,omitempty"`
	Env          []corev1.EnvVar          `json:"env,omitempty" yaml:"env,omitempty"`
	VolumeMounts []corev1.VolumeMount     `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`

	Resources *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`

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
func (cfg *AppWorkloadConfig) Generate(app *stack.Application) ([]*client.Object, error) {
	var objs []*client.Object
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
		sts := kubernetes.CreateStatefulSet(app.Name, app.Namespace)
		for _, c := range containers {
			if err := kubernetes.AddStatefulSetContainer(sts, c); err != nil {
				return nil, err
			}
		}
		for _, v := range cfg.Volumes {
			if err := kubernetes.AddStatefulSetVolume(sts, &v); err != nil {
				return nil, err
			}
		}
		for _, vct := range cfg.VolumeClaimTemplates {
			k8sPVC, err := vct.ToKubernetesPVC()
			if err != nil {
				return nil, err
			}
			if err := kubernetes.AddStatefulSetVolumeClaimTemplate(sts, *k8sPVC); err != nil {
				return nil, err
			}
		}
		_ = kubernetes.SetStatefulSetReplicas(sts, cfg.Replicas)
		objs = append(objs, k8s.ToClientObject(sts))
	case DaemonSetWorkload:
		ds := kubernetes.CreateDaemonSet(app.Name, app.Namespace)
		for _, c := range containers {
			if err := kubernetes.AddDaemonSetContainer(ds, c); err != nil {
				return nil, err
			}
		}
		for _, v := range cfg.Volumes {
			if err := kubernetes.AddDaemonSetVolume(ds, &v); err != nil {
				return nil, err
			}
		}
		objs = append(objs, k8s.ToClientObject(ds))
	case DeploymentWorkload:
		dep := kubernetes.CreateDeployment(app.Name, app.Namespace)
		for _, c := range containers {
			if err := kubernetes.AddDeploymentContainer(dep, c); err != nil {
				return nil, err
			}
		}
		for _, v := range cfg.Volumes {
			if err := kubernetes.AddDeploymentVolume(dep, &v); err != nil {
				return nil, err
			}
		}
		_ = kubernetes.SetDeploymentReplicas(dep, cfg.Replicas)
		objs = append(objs, k8s.ToClientObject(dep))
	default:
		return nil, fmt.Errorf("unsupported workload type %s", cfg.Workload)
	}

	// Service creation when ports are specified
	var svc *corev1.Service
	if len(allports) > 0 {
		svc = kubernetes.CreateService(app.Name, app.Namespace)
		_ = kubernetes.SetServiceSelector(svc, map[string]string{"app": app.Name})
		for _, p := range allports {
			_ = kubernetes.AddServicePort(svc, corev1.ServicePort{
				Name:       p.Name,
				Port:       p.ContainerPort,
				TargetPort: intstr.FromInt32(p.ContainerPort),
			})
		}
		objs = append(objs, k8s.ToClientObject(svc))
	}

	if cfg.Ingress != nil && svc != nil {
		ing := kubernetes.CreateIngress(app.Name, app.Namespace, "")
		rule := kubernetes.CreateIngressRule(cfg.Ingress.Host)
		pt := netv1.PathTypeImplementationSpecific
		path := cfg.Ingress.Path
		if path == "" {
			path = "/"
		}
		port := cfg.Ingress.ServicePortName
		kubernetes.AddIngressRulePath(rule, kubernetes.CreateIngressPath(path, &pt, svc.Name, port))
		kubernetes.AddIngressRule(ing, rule)
		kubernetes.AddIngressTLS(ing, netv1.IngressTLS{Hosts: []string{cfg.Ingress.Host}, SecretName: fmt.Sprintf("%s-tls", app.Name)})
		objs = append(objs, k8s.ToClientObject(ing))
	}

	return objs, nil
}

func (cfg ContainerConfig) Generate() (*corev1.Container, []corev1.ContainerPort, error) {
	container := kubernetes.CreateContainer(cfg.Name, cfg.Image, nil, nil)
	var ports []corev1.ContainerPort
	
	// Add ports
	for _, p := range cfg.Ports {
		_ = kubernetes.AddContainerPort(container, p)
		ports = append(ports, p)
	}
	
	// Add environment variables
	for _, env := range cfg.Env {
		_ = kubernetes.AddContainerEnv(container, env)
	}
	
	// Add volume mounts
	for _, vm := range cfg.VolumeMounts {
		_ = kubernetes.AddContainerVolumeMount(container, vm)
	}
	
	// Set resources if provided
	if cfg.Resources != nil {
		k8sResources, err := cfg.Resources.ToKubernetesResources()
		if err != nil {
			return nil, nil, err
		}
		if k8sResources != nil {
			_ = kubernetes.SetContainerResources(container, *k8sResources)
		}
	}
	
	// Set probes if provided
	if cfg.LivenessProbe != nil {
		_ = kubernetes.SetContainerLivenessProbe(container, *cfg.LivenessProbe)
	}
	if cfg.ReadinessProbe != nil {
		_ = kubernetes.SetContainerReadinessProbe(container, *cfg.ReadinessProbe)
	}
	if cfg.StartupProbe != nil {
		_ = kubernetes.SetContainerStartupProbe(container, *cfg.StartupProbe)
	}
	
	return container, ports, nil
}
