package application

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

// AppWorkloadConfig describes a single deployable application.
type AppWorkloadConfig struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace,omitempty"`
	Image     string            `yaml:"image"`
	Ports     []int             `yaml:"ports,omitempty"`
	Replicas  *int              `yaml:"replicas,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	Secrets   map[string]string `yaml:"secrets,omitempty"`
	Ingress   *IngressConfig    `yaml:"ingress,omitempty"`
	Resources map[string]string `yaml:"resources,omitempty"`
	Workload  WorkloadType      `yaml:"workload,omitempty"`
}
