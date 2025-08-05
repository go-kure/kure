package fluxcd

import (
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	v1 "github.com/fluxcd/notification-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
)

// OCIRepositoryConfig describes an OCIRepository resource used by Flux.
type OCIRepositoryConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	URL       string `yaml:"url"`
	Ref       string `yaml:"ref"`
	Interval  string `yaml:"interval"`
}

// GitRepositoryConfig contains the minimal settings for a GitRepository.
type GitRepositoryConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	URL       string `yaml:"url"`
	Interval  string `yaml:"interval"`
	Ref       string `yaml:"ref,omitempty"`
}

// HelmRepositoryConfig contains the configuration for a HelmRepository.
type HelmRepositoryConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	URL       string `yaml:"url"`
}

// BucketConfig contains the configuration for a Bucket source.
type BucketConfig struct {
	Name       string `yaml:"name"`
	Namespace  string `yaml:"namespace"`
	BucketName string `yaml:"bucketName"`
	Endpoint   string `yaml:"endpoint"`
	Interval   string `yaml:"interval"`
	Provider   string `yaml:"provider,omitempty"`
}

// HelmChartConfig contains the configuration for a HelmChart.
type HelmChartConfig struct {
	Name      string                                 `yaml:"name"`
	Namespace string                                 `yaml:"namespace"`
	Chart     string                                 `yaml:"chart"`
	Version   string                                 `yaml:"version,omitempty"`
	SourceRef sourcev1.LocalHelmChartSourceReference `yaml:"sourceRef"`
	Interval  string                                 `yaml:"interval"`
}

// KustomizationConfig contains the configuration for a Kustomization.
type KustomizationConfig struct {
	Name      string                               `yaml:"name"`
	Namespace string                               `yaml:"namespace"`
	Path      string                               `yaml:"path,omitempty"`
	Interval  string                               `yaml:"interval"`
	Prune     bool                                 `yaml:"prune"`
	SourceRef kustv1.CrossNamespaceSourceReference `yaml:"sourceRef"`
}

// HelmReleaseConfig contains the configuration for a HelmRelease.
type HelmReleaseConfig struct {
	Name        string                               `yaml:"name"`
	Namespace   string                               `yaml:"namespace"`
	Chart       string                               `yaml:"chart"`
	Version     string                               `yaml:"version,omitempty"`
	SourceRef   helmv2.CrossNamespaceObjectReference `yaml:"sourceRef"`
	Interval    string                               `yaml:"interval"`
	ReleaseName string                               `yaml:"releaseName,omitempty"`
}

// ProviderConfig contains the configuration for a notification Provider.
type ProviderConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	Type      string `yaml:"type"`
	Address   string `yaml:"address,omitempty"`
	Channel   string `yaml:"channel,omitempty"`
}

// AlertConfig contains the configuration for an Alert.
type AlertConfig struct {
	Name          string                             `yaml:"name"`
	Namespace     string                             `yaml:"namespace"`
	ProviderRef   string                             `yaml:"providerRef"`
	EventSources  []v1.CrossNamespaceObjectReference `yaml:"eventSources"`
	EventSeverity string                             `yaml:"eventSeverity,omitempty"`
}

// ReceiverConfig contains the configuration for a Receiver.
type ReceiverConfig struct {
	Name       string                             `yaml:"name"`
	Namespace  string                             `yaml:"namespace"`
	Type       string                             `yaml:"type"`
	SecretName string                             `yaml:"secretName"`
	Resources  []v1.CrossNamespaceObjectReference `yaml:"resources"`
	Events     []string                           `yaml:"events,omitempty"`
}

// ImageUpdateAutomationConfig contains the configuration for ImageUpdateAutomation.
type ImageUpdateAutomationConfig struct {
	Name      string                                `yaml:"name"`
	Namespace string                                `yaml:"namespace"`
	Interval  string                                `yaml:"interval"`
	SourceRef imagev1.CrossNamespaceSourceReference `yaml:"sourceRef"`
}

// ResourceSetConfig contains the configuration for a ResourceSet.
type ResourceSetConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// ResourceSetInputProviderConfig contains the configuration for a ResourceSetInputProvider.
type ResourceSetInputProviderConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	Type      string `yaml:"type"`
	URL       string `yaml:"url,omitempty"`
}

// FluxInstanceConfig contains the configuration for a FluxInstance.
type FluxInstanceConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	Version   string `yaml:"version"`
	Registry  string `yaml:"registry"`
}

// FluxReportConfig contains the configuration for a FluxReport.
type FluxReportConfig struct {
	Name        string `yaml:"name"`
	Namespace   string `yaml:"namespace"`
	Entitlement string `yaml:"entitlement"`
	Status      string `yaml:"status"`
}

// ReceiverSecretRefConfig is not a resource but included for completeness.
type ReceiverSecretRefConfig struct {
	Name      string                    `yaml:"name"`
	Namespace string                    `yaml:"namespace"`
	Ref       meta.LocalObjectReference `yaml:"ref"`
}
