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
	Volumes               []Volume                `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	VolumeClaimTemplates  []VolumeClaimTemplate   `json:"volumeClaimTemplates,omitempty" yaml:"volumeClaimTemplates,omitempty"`

	Services []ServiceConfig `json:"services,omitempty" yaml:"services,omitempty"`
	Ingress  *IngressConfig  `json:"ingress,omitempty" yaml:"ingress,omitempty"`
}

// Custom types for proper YAML parsing

type ContainerPort struct {
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	ContainerPort int32  `json:"containerPort" yaml:"containerPort"`
	Protocol      string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
}

func (cp ContainerPort) ToKubernetesPort() corev1.ContainerPort {
	protocol := corev1.ProtocolTCP
	if cp.Protocol != "" {
		protocol = corev1.Protocol(cp.Protocol)
	}
	return corev1.ContainerPort{
		Name:          cp.Name,
		ContainerPort: cp.ContainerPort,
		Protocol:      protocol,
	}
}

type VolumeMount struct {
	Name      string `json:"name" yaml:"name"`
	MountPath string `json:"mountPath" yaml:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SubPath   string `json:"subPath,omitempty" yaml:"subPath,omitempty"`
}

func (vm VolumeMount) ToKubernetesVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      vm.Name,
		MountPath: vm.MountPath,
		ReadOnly:  vm.ReadOnly,
		SubPath:   vm.SubPath,
	}
}

type EnvVarSource struct {
	SecretKeyRef    *SecretKeySelector    `json:"secretKeyRef,omitempty" yaml:"secretKeyRef,omitempty"`
	ConfigMapKeyRef *ConfigMapKeySelector `json:"configMapKeyRef,omitempty" yaml:"configMapKeyRef,omitempty"`
	FieldRef        *ObjectFieldSelector  `json:"fieldRef,omitempty" yaml:"fieldRef,omitempty"`
}

type SecretKeySelector struct {
	Name string `json:"name" yaml:"name"`
	Key  string `json:"key" yaml:"key"`
}

type ConfigMapKeySelector struct {
	Name string `json:"name" yaml:"name"`
	Key  string `json:"key" yaml:"key"`
}

type ObjectFieldSelector struct {
	FieldPath string `json:"fieldPath" yaml:"fieldPath"`
}

type EnvVar struct {
	Name      string        `json:"name" yaml:"name"`
	Value     string        `json:"value,omitempty" yaml:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}

func (env EnvVar) ToKubernetesEnvVar() corev1.EnvVar {
	k8sEnv := corev1.EnvVar{
		Name:  env.Name,
		Value: env.Value,
	}
	
	if env.ValueFrom != nil {
		k8sEnv.ValueFrom = &corev1.EnvVarSource{}
		if env.ValueFrom.SecretKeyRef != nil {
			k8sEnv.ValueFrom.SecretKeyRef = &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: env.ValueFrom.SecretKeyRef.Name,
				},
				Key: env.ValueFrom.SecretKeyRef.Key,
			}
		}
		if env.ValueFrom.ConfigMapKeyRef != nil {
			k8sEnv.ValueFrom.ConfigMapKeyRef = &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: env.ValueFrom.ConfigMapKeyRef.Name,
				},
				Key: env.ValueFrom.ConfigMapKeyRef.Key,
			}
		}
		if env.ValueFrom.FieldRef != nil {
			k8sEnv.ValueFrom.FieldRef = &corev1.ObjectFieldSelector{
				FieldPath: env.ValueFrom.FieldRef.FieldPath,
			}
		}
	}
	
	return k8sEnv
}

type HTTPGetAction struct {
	Path   string `json:"path,omitempty" yaml:"path,omitempty"`
	Port   int32  `json:"port" yaml:"port"`
	Scheme string `json:"scheme,omitempty" yaml:"scheme,omitempty"`
}

type ExecAction struct {
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
}

type Probe struct {
	HTTPGet                       *HTTPGetAction `json:"httpGet,omitempty" yaml:"httpGet,omitempty"`
	Exec                          *ExecAction    `json:"exec,omitempty" yaml:"exec,omitempty"`
	InitialDelaySeconds           int32          `json:"initialDelaySeconds,omitempty" yaml:"initialDelaySeconds,omitempty"`
	TimeoutSeconds                int32          `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
	PeriodSeconds                 int32          `json:"periodSeconds,omitempty" yaml:"periodSeconds,omitempty"`
	SuccessThreshold              int32          `json:"successThreshold,omitempty" yaml:"successThreshold,omitempty"`
	FailureThreshold              int32          `json:"failureThreshold,omitempty" yaml:"failureThreshold,omitempty"`
}

func (p Probe) ToKubernetesProbe() *corev1.Probe {
	if p.HTTPGet == nil && p.Exec == nil {
		return nil
	}
	
	k8sProbe := &corev1.Probe{
		InitialDelaySeconds: p.InitialDelaySeconds,
		TimeoutSeconds:      p.TimeoutSeconds,
		PeriodSeconds:       p.PeriodSeconds,
		SuccessThreshold:    p.SuccessThreshold,
		FailureThreshold:    p.FailureThreshold,
	}
	
	if p.HTTPGet != nil {
		k8sProbe.ProbeHandler.HTTPGet = &corev1.HTTPGetAction{
			Path: p.HTTPGet.Path,
			Port: intstr.FromInt32(p.HTTPGet.Port),
		}
		if p.HTTPGet.Scheme != "" {
			k8sProbe.ProbeHandler.HTTPGet.Scheme = corev1.URIScheme(p.HTTPGet.Scheme)
		}
	}
	
	if p.Exec != nil {
		k8sProbe.ProbeHandler.Exec = &corev1.ExecAction{
			Command: p.Exec.Command,
		}
	}
	
	return k8sProbe
}

type VolumeSource struct {
	EmptyDir  *EmptyDirVolumeSource  `json:"emptyDir,omitempty" yaml:"emptyDir,omitempty"`
	ConfigMap *ConfigMapVolumeSource `json:"configMap,omitempty" yaml:"configMap,omitempty"`
	Secret    *SecretVolumeSource    `json:"secret,omitempty" yaml:"secret,omitempty"`
	HostPath  *HostPathVolumeSource  `json:"hostPath,omitempty" yaml:"hostPath,omitempty"`
}

type EmptyDirVolumeSource struct {
	SizeLimit string `json:"sizeLimit,omitempty" yaml:"sizeLimit,omitempty"`
}

type ConfigMapVolumeSource struct {
	Name string `json:"name" yaml:"name"`
}

type SecretVolumeSource struct {
	SecretName string `json:"secretName" yaml:"secretName"`
}

type HostPathVolumeSource struct {
	Path string `json:"path" yaml:"path"`
}

type Volume struct {
	Name         string        `json:"name" yaml:"name"`
	VolumeSource *VolumeSource `json:",inline" yaml:",inline"`
}

func (v Volume) ToKubernetesVolume() corev1.Volume {
	k8sVol := corev1.Volume{
		Name: v.Name,
	}
	
	if v.VolumeSource != nil {
		if v.VolumeSource.EmptyDir != nil {
			k8sVol.VolumeSource.EmptyDir = &corev1.EmptyDirVolumeSource{}
			if v.VolumeSource.EmptyDir.SizeLimit != "" {
				qty, err := resource.ParseQuantity(v.VolumeSource.EmptyDir.SizeLimit)
				if err == nil {
					k8sVol.VolumeSource.EmptyDir.SizeLimit = &qty
				}
			}
		}
		if v.VolumeSource.ConfigMap != nil {
			k8sVol.VolumeSource.ConfigMap = &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: v.VolumeSource.ConfigMap.Name,
				},
			}
		}
		if v.VolumeSource.Secret != nil {
			k8sVol.VolumeSource.Secret = &corev1.SecretVolumeSource{
				SecretName: v.VolumeSource.Secret.SecretName,
			}
		}
		if v.VolumeSource.HostPath != nil {
			k8sVol.VolumeSource.HostPath = &corev1.HostPathVolumeSource{
				Path: v.VolumeSource.HostPath.Path,
			}
		}
	}
	
	return k8sVol
}

type ContainerConfig struct {
	Name         string                   `json:"name" yaml:"name"`
	Image        string                   `json:"image" yaml:"image"`
	Ports        []ContainerPort          `json:"ports,omitempty" yaml:"ports,omitempty"`
	Env          []EnvVar                 `json:"env,omitempty" yaml:"env,omitempty"`
	VolumeMounts []VolumeMount            `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`

	Resources *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`

	StartupProbe   *Probe `json:"startupProbe,omitempty" yaml:"startupProbe,omitempty"`
	LivenessProbe  *Probe `json:"livenessProbe,omitempty" yaml:"livenessProbe,omitempty"`
	ReadinessProbe *Probe `json:"readinessProbe,omitempty" yaml:"readinessProbe,omitempty"`
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
			k8sVol := v.ToKubernetesVolume()
			if err := kubernetes.AddStatefulSetVolume(sts, &k8sVol); err != nil {
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
			k8sVol := v.ToKubernetesVolume()
			if err := kubernetes.AddDaemonSetVolume(ds, &k8sVol); err != nil {
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
			k8sVol := v.ToKubernetesVolume()
			if err := kubernetes.AddDeploymentVolume(dep, &k8sVol); err != nil {
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
		k8sPort := p.ToKubernetesPort()
		_ = kubernetes.AddContainerPort(container, k8sPort)
		ports = append(ports, k8sPort)
	}
	
	// Add environment variables
	for _, env := range cfg.Env {
		k8sEnv := env.ToKubernetesEnvVar()
		_ = kubernetes.AddContainerEnv(container, k8sEnv)
	}
	
	// Add volume mounts
	for _, vm := range cfg.VolumeMounts {
		k8sMount := vm.ToKubernetesVolumeMount()
		_ = kubernetes.AddContainerVolumeMount(container, k8sMount)
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
		k8sProbe := cfg.LivenessProbe.ToKubernetesProbe()
		if k8sProbe != nil {
			_ = kubernetes.SetContainerLivenessProbe(container, *k8sProbe)
		}
	}
	if cfg.ReadinessProbe != nil {
		k8sProbe := cfg.ReadinessProbe.ToKubernetesProbe()
		if k8sProbe != nil {
			_ = kubernetes.SetContainerReadinessProbe(container, *k8sProbe)
		}
	}
	if cfg.StartupProbe != nil {
		k8sProbe := cfg.StartupProbe.ToKubernetesProbe()
		if k8sProbe != nil {
			_ = kubernetes.SetContainerStartupProbe(container, *k8sProbe)
		}
	}
	
	return container, ports, nil
}
