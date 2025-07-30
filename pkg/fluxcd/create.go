package fluxcd

import (
	"time"

	intfluxcd "github.com/go-kure/kure/internal/fluxcd"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewGitRepository converts the config to a GitRepository object.
func NewGitRepository(cfg *GitRepositoryConfig) *sourcev1.GitRepository {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateGitRepository(cfg.Name, cfg.Namespace, sourcev1.GitRepositorySpec{})
	intfluxcd.SetGitRepositoryURL(obj, cfg.URL)
	intfluxcd.SetGitRepositoryInterval(obj, metav1.Duration{Duration: parseDurationOrDefault(cfg.Interval)})
	if cfg.Ref != "" {
		intfluxcd.SetGitRepositoryReference(obj, &sourcev1.GitRepositoryRef{Branch: cfg.Ref})
	}
	return obj
}

// NewHelmRepository converts the config to a HelmRepository object.
func NewHelmRepository(cfg *HelmRepositoryConfig) *sourcev1.HelmRepository {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateHelmRepository(cfg.Name, cfg.Namespace, sourcev1.HelmRepositorySpec{})
	intfluxcd.SetHelmRepositoryURL(obj, cfg.URL)
	return obj
}

// NewBucket converts the config to a Bucket object.
func NewBucket(cfg *BucketConfig) *sourcev1.Bucket {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateBucket(cfg.Name, cfg.Namespace, sourcev1.BucketSpec{})
	intfluxcd.SetBucketName(obj, cfg.BucketName)
	intfluxcd.SetBucketEndpoint(obj, cfg.Endpoint)
	intfluxcd.SetBucketInterval(obj, metav1.Duration{Duration: parseDurationOrDefault(cfg.Interval)})
	if cfg.Provider != "" {
		intfluxcd.SetBucketProvider(obj, cfg.Provider)
	}
	return obj
}

// NewHelmChart converts the config to a HelmChart object.
func NewHelmChart(cfg *HelmChartConfig) *sourcev1.HelmChart {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateHelmChart(cfg.Name, cfg.Namespace, sourcev1.HelmChartSpec{})
	intfluxcd.SetHelmChartChart(obj, cfg.Chart)
	intfluxcd.SetHelmChartSourceRef(obj, cfg.SourceRef)
	intfluxcd.SetHelmChartInterval(obj, metav1.Duration{Duration: parseDurationOrDefault(cfg.Interval)})
	if cfg.Version != "" {
		intfluxcd.SetHelmChartVersion(obj, cfg.Version)
	}
	return obj
}

// NewOCIRepository converts the config to an OCIRepository object.
func NewOCIRepository(cfg *OCIRepositoryConfig) *sourcev1beta2.OCIRepository {
	if cfg == nil {
		return nil
	}
	spec := sourcev1beta2.OCIRepositorySpec{
		URL:       cfg.URL,
		Reference: &sourcev1beta2.OCIRepositoryRef{Tag: cfg.Ref},
		Interval:  metav1.Duration{Duration: parseDurationOrDefault(cfg.Interval)},
	}
	return intfluxcd.CreateOCIRepository(cfg.Name, cfg.Namespace, spec)
}

// NewKustomization converts the config to a Kustomization object.
func NewKustomization(cfg *KustomizationConfig) *kustv1.Kustomization {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateKustomization(cfg.Name, cfg.Namespace, kustv1.KustomizationSpec{Prune: cfg.Prune})
	intfluxcd.SetKustomizationInterval(obj, metav1.Duration{Duration: parseDurationOrDefault(cfg.Interval)})
	intfluxcd.SetKustomizationSourceRef(obj, cfg.SourceRef)
	if cfg.Path != "" {
		intfluxcd.SetKustomizationPath(obj, cfg.Path)
	}
	return obj
}

// NewHelmRelease converts the config to a HelmRelease object.
func NewHelmRelease(cfg *HelmReleaseConfig) *helmv2.HelmRelease {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateHelmRelease(cfg.Name, cfg.Namespace, helmv2.HelmReleaseSpec{})
	chart := helmv2.HelmChartTemplate{
		Spec: helmv2.HelmChartTemplateSpec{
			Chart:     cfg.Chart,
			Version:   cfg.Version,
			SourceRef: cfg.SourceRef,
		},
	}
	intfluxcd.SetHelmReleaseChart(obj, &chart)
	intfluxcd.SetHelmReleaseInterval(obj, metav1.Duration{Duration: parseDurationOrDefault(cfg.Interval)})
	if cfg.ReleaseName != "" {
		intfluxcd.SetHelmReleaseReleaseName(obj, cfg.ReleaseName)
	}
	return obj
}

// NewProvider converts the config to a notification Provider object.
func NewProvider(cfg *ProviderConfig) *notificationv1beta2.Provider {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateProvider(cfg.Name, cfg.Namespace, notificationv1beta2.ProviderSpec{})
	intfluxcd.SetProviderType(obj, cfg.Type)
	if cfg.Channel != "" {
		intfluxcd.SetProviderChannel(obj, cfg.Channel)
	}
	if cfg.Address != "" {
		intfluxcd.SetProviderAddress(obj, cfg.Address)
	}
	return obj
}

// NewAlert converts the config to an Alert object.
func NewAlert(cfg *AlertConfig) *notificationv1beta2.Alert {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateAlert(cfg.Name, cfg.Namespace, notificationv1beta2.AlertSpec{})
	intfluxcd.SetAlertProviderRef(obj, meta.LocalObjectReference{Name: cfg.ProviderRef})
	for _, es := range cfg.EventSources {
		intfluxcd.AddAlertEventSource(obj, es)
	}
	if cfg.EventSeverity != "" {
		intfluxcd.SetAlertEventSeverity(obj, cfg.EventSeverity)
	}
	return obj
}

// NewReceiver converts the config to a Receiver object.
func NewReceiver(cfg *ReceiverConfig) *notificationv1beta2.Receiver {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateReceiver(cfg.Name, cfg.Namespace, notificationv1beta2.ReceiverSpec{})
	intfluxcd.SetReceiverType(obj, cfg.Type)
	intfluxcd.SetReceiverSecretRef(obj, meta.LocalObjectReference{Name: cfg.SecretName})
	for _, r := range cfg.Resources {
		intfluxcd.AddReceiverResource(obj, r)
	}
	for _, e := range cfg.Events {
		intfluxcd.AddReceiverEvent(obj, e)
	}
	return obj
}

// NewImageUpdateAutomation converts the config to an ImageUpdateAutomation object.
func NewImageUpdateAutomation(cfg *ImageUpdateAutomationConfig) *imagev1.ImageUpdateAutomation {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateImageUpdateAutomation(cfg.Name, cfg.Namespace, imagev1.ImageUpdateAutomationSpec{})
	intfluxcd.SetImageUpdateAutomationSourceRef(obj, cfg.SourceRef)
	intfluxcd.SetImageUpdateAutomationInterval(obj, metav1.Duration{Duration: parseDurationOrDefault(cfg.Interval)})
	return obj
}

// NewResourceSet converts the config to a ResourceSet object.
func NewResourceSet(cfg *ResourceSetConfig) *fluxv1.ResourceSet {
	if cfg == nil {
		return nil
	}
	return intfluxcd.CreateResourceSet(cfg.Name, cfg.Namespace, fluxv1.ResourceSetSpec{})
}

// NewResourceSetInputProvider converts the config to a ResourceSetInputProvider object.
func NewResourceSetInputProvider(cfg *ResourceSetInputProviderConfig) *fluxv1.ResourceSetInputProvider {
	if cfg == nil {
		return nil
	}
	obj := intfluxcd.CreateResourceSetInputProvider(cfg.Name, cfg.Namespace, fluxv1.ResourceSetInputProviderSpec{})
	intfluxcd.SetResourceSetInputProviderType(obj, cfg.Type)
	if cfg.URL != "" {
		intfluxcd.SetResourceSetInputProviderURL(obj, cfg.URL)
	}
	return obj
}

// NewFluxInstance converts the config to a FluxInstance object.
func NewFluxInstance(cfg *FluxInstanceConfig) *fluxv1.FluxInstance {
	if cfg == nil {
		return nil
	}
	spec := fluxv1.FluxInstanceSpec{Distribution: fluxv1.Distribution{Version: cfg.Version, Registry: cfg.Registry}}
	return intfluxcd.CreateFluxInstance(cfg.Name, cfg.Namespace, spec)
}

// NewFluxReport converts the config to a FluxReport object.
func NewFluxReport(cfg *FluxReportConfig) *fluxv1.FluxReport {
	if cfg == nil {
		return nil
	}
	spec := fluxv1.FluxReportSpec{Distribution: fluxv1.FluxDistributionStatus{Entitlement: cfg.Entitlement, Status: cfg.Status}}
	return intfluxcd.CreateFluxReport(cfg.Name, cfg.Namespace, spec)
}

func parseDurationOrDefault(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}
