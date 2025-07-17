package api

type WorkloadType string

const (
	DeploymentWorkload  WorkloadType = "Deployment"
	StatefulSetWorkload WorkloadType = "StatefulSet"
	DaemonSetWorkload   WorkloadType = "DaemonSet"
)

type FileExportMode string

const (
	FilePerResource FileExportMode = "resource"
	FilePerKind     FileExportMode = "kind"
)

type OCIRepositoryConfig struct {
	Name      string
	Namespace string
	URL       string
	Ref       string
	Interval  string
}

type IngressConfig struct {
	Host          string
	Path          string
	TLS           bool
	Issuer        string
	UseACMEHTTP01 bool
}

type AppDeploymentConfig struct {
	Name      string
	Namespace string
	Image     string
	Ports     []int
	Replicas  *int
	Env       map[string]string
	Secrets   map[string]string
	Ingress   *IngressConfig
	Resources map[string]string
	Workload  WorkloadType
	FilePer   FileExportMode
}

type ClusterConfig struct {
	Name      string
	Interval  string
	SourceRef string
	FilePer   FileExportMode
	OCIRepo   *OCIRepositoryConfig
	AppGroups []AppGroup
}

type AppGroup struct {
	Name          string
	Namespace     string
	Apps          []AppDeploymentConfig
	FilePer       FileExportMode
	FluxDependsOn []string
}
