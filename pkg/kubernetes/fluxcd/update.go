package fluxcd

import (
	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourceWatcherv1beta1 "github.com/fluxcd/source-watcher/api/v2/v1beta1"
)

// SetGitRepositorySpec replaces the spec on the GitRepository object.
func SetGitRepositorySpec(obj *sourcev1.GitRepository, spec sourcev1.GitRepositorySpec) {
	obj.Spec = spec
}

// SetHelmRepositorySpec replaces the spec on the HelmRepository object.
func SetHelmRepositorySpec(obj *sourcev1.HelmRepository, spec sourcev1.HelmRepositorySpec) {
	obj.Spec = spec
}

// SetBucketSpec replaces the spec on the Bucket object.
func SetBucketSpec(obj *sourcev1.Bucket, spec sourcev1.BucketSpec) {
	obj.Spec = spec
}

// SetHelmChartSpec replaces the spec on the HelmChart object.
func SetHelmChartSpec(obj *sourcev1.HelmChart, spec sourcev1.HelmChartSpec) {
	obj.Spec = spec
}

// SetOCIRepositorySpec replaces the spec on the OCIRepository object.
func SetOCIRepositorySpec(obj *sourcev1.OCIRepository, spec sourcev1.OCIRepositorySpec) {
	obj.Spec = spec
}

// SetExternalArtifactSpec replaces the spec on the ExternalArtifact object.
func SetExternalArtifactSpec(obj *sourcev1.ExternalArtifact, spec sourcev1.ExternalArtifactSpec) {
	obj.Spec = spec
}

// SetArtifactGeneratorSpec replaces the spec on the ArtifactGenerator object.
func SetArtifactGeneratorSpec(obj *sourceWatcherv1beta1.ArtifactGenerator, spec sourceWatcherv1beta1.ArtifactGeneratorSpec) {
	obj.Spec = spec
}

// SetKustomizationSpec replaces the spec on the Kustomization object.
func SetKustomizationSpec(obj *kustv1.Kustomization, spec kustv1.KustomizationSpec) {
	obj.Spec = spec
}

// SetHelmReleaseSpec replaces the spec on the HelmRelease object.
func SetHelmReleaseSpec(obj *helmv2.HelmRelease, spec helmv2.HelmReleaseSpec) {
	obj.Spec = spec
}

// SetProviderSpec replaces the spec on the Provider object.
func SetProviderSpec(obj *notificationv1beta3.Provider, spec notificationv1beta3.ProviderSpec) {
	obj.Spec = spec
}

// SetAlertSpec replaces the spec on the Alert object.
func SetAlertSpec(obj *notificationv1beta3.Alert, spec notificationv1beta3.AlertSpec) {
	obj.Spec = spec
}

// SetReceiverSpec replaces the spec on the Receiver object.
func SetReceiverSpec(obj *notificationv1.Receiver, spec notificationv1.ReceiverSpec) {
	obj.Spec = spec
}

// SetImageUpdateAutomationSpec replaces the spec on the ImageUpdateAutomation object.
func SetImageUpdateAutomationSpec(obj *imagev1.ImageUpdateAutomation, spec imagev1.ImageUpdateAutomationSpec) {
	obj.Spec = spec
}

// SetResourceSetSpec replaces the spec on the ResourceSet object.
func SetResourceSetSpec(obj *fluxv1.ResourceSet, spec fluxv1.ResourceSetSpec) {
	obj.Spec = spec
}

// SetResourceSetInputProviderSpec replaces the spec on the ResourceSetInputProvider object.
func SetResourceSetInputProviderSpec(obj *fluxv1.ResourceSetInputProvider, spec fluxv1.ResourceSetInputProviderSpec) {
	obj.Spec = spec
}

// SetFluxInstanceSpec replaces the spec on the FluxInstance object.
func SetFluxInstanceSpec(obj *fluxv1.FluxInstance, spec fluxv1.FluxInstanceSpec) {
	obj.Spec = spec
}

// SetFluxReportSpec replaces the spec on the FluxReport object.
func SetFluxReportSpec(obj *fluxv1.FluxReport, spec fluxv1.FluxReportSpec) {
	obj.Spec = spec
}
