// Package api defines configuration structures used to generate
// Kubernetes manifests and Flux resources.
package api

import "github.com/go-kure/kure/pkg/fluxcd"

// WorkloadType enumerates the supported Kubernetes workload kinds.
type WorkloadType string

const (
	DeploymentWorkload  WorkloadType = "Deployment"
	StatefulSetWorkload WorkloadType = "StatefulSet"
	DaemonSetWorkload   WorkloadType = "DaemonSet"
)

// FileExportMode determines how resources are written to disk.
type FileExportMode string

const (
	// FilePerResource writes each resource to its own file.
	FilePerResource FileExportMode = "resource"
	// FilePerKind groups resources by kind into a single file.
	FilePerKind FileExportMode = "kind"
	// FilePerUnset indicates that no export mode is specified.
	FilePerUnset FileExportMode = ""
)

// IngressConfig defines ingress settings for an application.
type IngressConfig struct {
	Host          string `yaml:"host"`
	Path          string `yaml:"path,omitempty"`
	TLS           bool   `yaml:"tls,omitempty"`
	Issuer        string `yaml:"issuer,omitempty"`
	UseACMEHTTP01 bool   `yaml:"useACMEHTTP01,omitempty"`
}

// AppDeploymentConfig describes a single deployable application.
type AppDeploymentConfig struct {
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
	FilePer   FileExportMode    `yaml:"filePer,omitempty"`
}

// ClusterConfig is the root configuration for a cluster layout.
type ClusterConfig struct {
	Name      string                      `yaml:"name"`
	Interval  string                      `yaml:"interval"`
	SourceRef string                      `yaml:"sourceRef"`
	FilePer   FileExportMode              `yaml:"filePer,omitempty"`
	OCIRepo   *fluxcd.OCIRepositoryConfig `yaml:"ociRepo,omitempty"`
	AppGroups []AppGroup                  `yaml:"appGroups,omitempty"`
}

// AppGroup groups related applications under a single namespace.
type AppGroup struct {
	Name          string                `yaml:"name"`
	Namespace     string                `yaml:"namespace,omitempty"`
	Apps          []AppDeploymentConfig `yaml:"apps,omitempty"`
	FilePer       FileExportMode        `yaml:"filePer,omitempty"`
	FluxDependsOn []string              `yaml:"fluxDependsOn,omitempty"`
}
