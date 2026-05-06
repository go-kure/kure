package fluxcd

import (
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
)

// OCIRepositoryConfig describes an OCIRepository resource used by Flux.
// Ref and Digest are mutually exclusive: when Digest is non-empty it is used
// as spec.reference.digest and Ref is ignored.
type OCIRepositoryConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	URL       string `yaml:"url"`
	Ref       string `yaml:"ref,omitempty"`
	// Digest is a content-addressable reference (e.g. "sha256:abc…"). When
	// set, Ref is ignored and spec.reference.digest is used instead.
	Digest   string `yaml:"digest,omitempty"`
	Interval string `yaml:"interval"`
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
	Type      string `yaml:"type,omitempty"`
	Interval  string `yaml:"interval,omitempty"`
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
	// TargetNamespace overrides the namespace for all reconciled resources.
	TargetNamespace string `yaml:"targetNamespace,omitempty"`
	// Wait instructs Flux to wait for all reconciled resources to become ready.
	Wait bool `yaml:"wait,omitempty"`
}

// ChartRefConfig references an existing Flux source (OCIRepository or HelmChart)
// for use as the chart source in chartRef mode.
type ChartRefConfig struct {
	Kind      string `yaml:"kind"` // "OCIRepository" or "HelmChart"
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

// ValuesFromConfig describes a single entry in spec.valuesFrom.
type ValuesFromConfig struct {
	Kind       string `yaml:"kind"` // "ConfigMap" or "Secret"
	Name       string `yaml:"name"`
	ValuesKey  string `yaml:"valuesKey,omitempty"` // defaults to "values.yaml" when empty
	TargetPath string `yaml:"targetPath,omitempty"`
	Optional   bool   `yaml:"optional,omitempty"`
}

// HelmReleaseConfig contains the configuration for a HelmRelease.
// ChartRef and Chart/Version/SourceRef are mutually exclusive:
// when ChartRef is non-nil the chart template fields are ignored.
type HelmReleaseConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	// Chart source via inline HelmChartTemplate (mutually exclusive with ChartRef).
	Chart     string                               `yaml:"chart,omitempty"`
	Version   string                               `yaml:"version,omitempty"`
	SourceRef helmv2.CrossNamespaceObjectReference `yaml:"sourceRef,omitempty"`
	// ChartRef references an existing OCIRepository or HelmChart (mutually exclusive with Chart/Version/SourceRef).
	ChartRef    *ChartRefConfig `yaml:"chartRef,omitempty"`
	Interval    string          `yaml:"interval"`
	ReleaseName string          `yaml:"releaseName,omitempty"`
	// TargetNamespace deploys Helm-managed resources into a namespace other than
	// the HelmRelease CR itself.
	TargetNamespace string `yaml:"targetNamespace,omitempty"`
	// DriftDetectionMode is "enabled", "warn", or "disabled". When empty, drift
	// detection is not configured.
	DriftDetectionMode string `yaml:"driftDetectionMode,omitempty"`
	// InstallCRDs controls CRD handling on install: "Skip", "Create", or "CreateReplace".
	InstallCRDs string `yaml:"installCRDs,omitempty"`
	// InstallRetries sets spec.install.remediation.retries.
	InstallRetries *int `yaml:"installRetries,omitempty"`
	// UpgradeCRDs controls CRD handling on upgrade: "Skip", "Create", or "CreateReplace".
	UpgradeCRDs string `yaml:"upgradeCRDs,omitempty"`
	// UpgradeRetries sets spec.upgrade.remediation.retries.
	UpgradeRetries *int `yaml:"upgradeRetries,omitempty"`
	// RemediateLastFailure sets spec.upgrade.remediation.remediateLastFailure.
	RemediateLastFailure *bool `yaml:"remediateLastFailure,omitempty"`
	// UpgradeCleanupOnFail sets spec.upgrade.cleanupOnFail.
	UpgradeCleanupOnFail bool `yaml:"upgradeCleanupOnFail,omitempty"`
	// RollbackCleanupOnFail sets spec.rollback.cleanupOnFail.
	RollbackCleanupOnFail bool `yaml:"rollbackCleanupOnFail,omitempty"`
	// ValuesFrom is a list of references to ConfigMaps or Secrets whose data
	// is merged into the Helm values.
	ValuesFrom []ValuesFromConfig `yaml:"valuesFrom,omitempty"`
	// Values is an inline map of Helm values. It is encoded as JSON and set
	// as spec.values. Takes precedence over ValuesFrom entries for the same keys.
	Values map[string]any `yaml:"values,omitempty"`
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
	Name          string                                         `yaml:"name"`
	Namespace     string                                         `yaml:"namespace"`
	ProviderRef   string                                         `yaml:"providerRef"`
	EventSources  []notificationv1.CrossNamespaceObjectReference `yaml:"eventSources"`
	EventSeverity string                                         `yaml:"eventSeverity,omitempty"`
}

// ReceiverConfig contains the configuration for a Receiver.
type ReceiverConfig struct {
	Name       string                                         `yaml:"name"`
	Namespace  string                                         `yaml:"namespace"`
	Type       string                                         `yaml:"type"`
	SecretName string                                         `yaml:"secretName"`
	Resources  []notificationv1.CrossNamespaceObjectReference `yaml:"resources"`
	Events     []string                                       `yaml:"events,omitempty"`
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
