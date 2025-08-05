package fluxcd

import (
	intfluxcd "github.com/go-kure/kure/internal/fluxcd"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	meta "github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func SetOCIRepositorySpec(obj *sourcev1beta2.OCIRepository, spec sourcev1beta2.OCIRepositorySpec) {
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
func SetProviderSpec(obj *notificationv1beta2.Provider, spec notificationv1beta2.ProviderSpec) {
	obj.Spec = spec
}

// SetAlertSpec replaces the spec on the Alert object.
func SetAlertSpec(obj *notificationv1beta2.Alert, spec notificationv1beta2.AlertSpec) {
	obj.Spec = spec
}

// SetReceiverSpec replaces the spec on the Receiver object.
func SetReceiverSpec(obj *notificationv1beta2.Receiver, spec notificationv1beta2.ReceiverSpec) {
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

// Wrapper helpers for internal functions so they are exported from this package.

// AddFluxInstanceComponent delegates to the internal helper.
func AddFluxInstanceComponent(obj *fluxv1.FluxInstance, c fluxv1.Component) error {
	return intfluxcd.AddFluxInstanceComponent(obj, c)
}

// SetFluxInstanceDistribution delegates to the internal helper.
func SetFluxInstanceDistribution(obj *fluxv1.FluxInstance, dist fluxv1.Distribution) error {
	return intfluxcd.SetFluxInstanceDistribution(obj, dist)
}

// SetFluxInstanceCommonMetadata delegates to the internal helper.
func SetFluxInstanceCommonMetadata(obj *fluxv1.FluxInstance, cm *fluxv1.CommonMetadata) error {
	return intfluxcd.SetFluxInstanceCommonMetadata(obj, cm)
}

// SetFluxInstanceCluster delegates to the internal helper.
func SetFluxInstanceCluster(obj *fluxv1.FluxInstance, cluster *fluxv1.Cluster) error {
	return intfluxcd.SetFluxInstanceCluster(obj, cluster)
}

// SetFluxInstanceSharding delegates to the internal helper.
func SetFluxInstanceSharding(obj *fluxv1.FluxInstance, shard *fluxv1.Sharding) error {
	return intfluxcd.SetFluxInstanceSharding(obj, shard)
}

// SetFluxInstanceStorage delegates to the internal helper.
func SetFluxInstanceStorage(obj *fluxv1.FluxInstance, st *fluxv1.Storage) error {
	return intfluxcd.SetFluxInstanceStorage(obj, st)
}

// SetFluxInstanceKustomize delegates to the internal helper.
func SetFluxInstanceKustomize(obj *fluxv1.FluxInstance, k *fluxv1.Kustomize) error {
	return intfluxcd.SetFluxInstanceKustomize(obj, k)
}

// SetFluxInstanceWait delegates to the internal helper.
func SetFluxInstanceWait(obj *fluxv1.FluxInstance, wait bool) error {
	return intfluxcd.SetFluxInstanceWait(obj, wait)
}

// SetFluxInstanceMigrateResources delegates to the internal helper.
func SetFluxInstanceMigrateResources(obj *fluxv1.FluxInstance, m bool) error {
	return intfluxcd.SetFluxInstanceMigrateResources(obj, m)
}

// SetFluxInstanceSync delegates to the internal helper.
func SetFluxInstanceSync(obj *fluxv1.FluxInstance, sync *fluxv1.Sync) error {
	return intfluxcd.SetFluxInstanceSync(obj, sync)
}

// AddFluxReportComponentStatus delegates to the internal helper.
func AddFluxReportComponentStatus(fr *fluxv1.FluxReport, cs fluxv1.FluxComponentStatus) error {
	return intfluxcd.AddFluxReportComponentStatus(fr, cs)
}

// AddFluxReportReconcilerStatus delegates to the internal helper.
func AddFluxReportReconcilerStatus(fr *fluxv1.FluxReport, rs fluxv1.FluxReconcilerStatus) error {
	return intfluxcd.AddFluxReportReconcilerStatus(fr, rs)
}

// SetFluxReportDistribution delegates to the internal helper.
func SetFluxReportDistribution(fr *fluxv1.FluxReport, dist fluxv1.FluxDistributionStatus) error {
	return intfluxcd.SetFluxReportDistribution(fr, dist)
}

// SetFluxReportCluster delegates to the internal helper.
func SetFluxReportCluster(fr *fluxv1.FluxReport, c *fluxv1.ClusterInfo) error {
	return intfluxcd.SetFluxReportCluster(fr, c)
}

// SetFluxReportOperator delegates to the internal helper.
func SetFluxReportOperator(fr *fluxv1.FluxReport, op *fluxv1.OperatorInfo) error {
	return intfluxcd.SetFluxReportOperator(fr, op)
}

// SetFluxReportSyncStatus delegates to the internal helper.
func SetFluxReportSyncStatus(fr *fluxv1.FluxReport, s *fluxv1.FluxSyncStatus) error {
	return intfluxcd.SetFluxReportSyncStatus(fr, s)
}

// AddResourceSetInput delegates to the internal helper.
func AddResourceSetInput(rs *fluxv1.ResourceSet, in fluxv1.ResourceSetInput) error {
	return intfluxcd.AddResourceSetInput(rs, in)
}

// AddResourceSetInputFrom delegates to the internal helper.
func AddResourceSetInputFrom(rs *fluxv1.ResourceSet, ref fluxv1.InputProviderReference) error {
	return intfluxcd.AddResourceSetInputFrom(rs, ref)
}

// AddResourceSetResource delegates to the internal helper.
func AddResourceSetResource(rs *fluxv1.ResourceSet, r *apiextensionsv1.JSON) error {
	return intfluxcd.AddResourceSetResource(rs, r)
}

// SetResourceSetResourcesTemplate delegates to the internal helper.
func SetResourceSetResourcesTemplate(rs *fluxv1.ResourceSet, tpl string) error {
	return intfluxcd.SetResourceSetResourcesTemplate(rs, tpl)
}

// AddResourceSetDependency delegates to the internal helper.
func AddResourceSetDependency(rs *fluxv1.ResourceSet, dep fluxv1.Dependency) error {
	return intfluxcd.AddResourceSetDependency(rs, dep)
}

// SetResourceSetServiceAccountName delegates to the internal helper.
func SetResourceSetServiceAccountName(rs *fluxv1.ResourceSet, name string) error {
	return intfluxcd.SetResourceSetServiceAccountName(rs, name)
}

// SetResourceSetWait delegates to the internal helper.
func SetResourceSetWait(rs *fluxv1.ResourceSet, wait bool) error {
	return intfluxcd.SetResourceSetWait(rs, wait)
}

// SetResourceSetCommonMetadata delegates to the internal helper.
func SetResourceSetCommonMetadata(rs *fluxv1.ResourceSet, cm *fluxv1.CommonMetadata) error {
	return intfluxcd.SetResourceSetCommonMetadata(rs, cm)
}

// SetResourceSetInputProviderType delegates to the internal helper.
func SetResourceSetInputProviderType(obj *fluxv1.ResourceSetInputProvider, typ string) error {
	return intfluxcd.SetResourceSetInputProviderType(obj, typ)
}

// SetResourceSetInputProviderURL delegates to the internal helper.
func SetResourceSetInputProviderURL(obj *fluxv1.ResourceSetInputProvider, url string) error {
	return intfluxcd.SetResourceSetInputProviderURL(obj, url)
}

// SetResourceSetInputProviderServiceAccountName delegates to the internal helper.
func SetResourceSetInputProviderServiceAccountName(obj *fluxv1.ResourceSetInputProvider, name string) error {
	return intfluxcd.SetResourceSetInputProviderServiceAccountName(obj, name)
}

// SetResourceSetInputProviderSecretRef delegates to the internal helper.
func SetResourceSetInputProviderSecretRef(obj *fluxv1.ResourceSetInputProvider, ref *meta.LocalObjectReference) error {
	return intfluxcd.SetResourceSetInputProviderSecretRef(obj, ref)
}

// SetResourceSetInputProviderCertSecretRef delegates to the internal helper.
func SetResourceSetInputProviderCertSecretRef(obj *fluxv1.ResourceSetInputProvider, ref *meta.LocalObjectReference) error {
	return intfluxcd.SetResourceSetInputProviderCertSecretRef(obj, ref)
}

// AddResourceSetInputProviderSchedule delegates to the internal helper.
func AddResourceSetInputProviderSchedule(obj *fluxv1.ResourceSetInputProvider, s fluxv1.Schedule) error {
	return intfluxcd.AddResourceSetInputProviderSchedule(obj, s)
}

// Additional helpers for notification resources.

// SetProviderType delegates to the internal helper.
func SetProviderType(provider *notificationv1beta2.Provider, t string) {
	intfluxcd.SetProviderType(provider, t)
}

// SetProviderInterval delegates to the internal helper.
func SetProviderInterval(provider *notificationv1beta2.Provider, d metav1.Duration) {
	intfluxcd.SetProviderInterval(provider, d)
}

// SetProviderChannel delegates to the internal helper.
func SetProviderChannel(provider *notificationv1beta2.Provider, channel string) {
	intfluxcd.SetProviderChannel(provider, channel)
}

// SetProviderUsername delegates to the internal helper.
func SetProviderUsername(provider *notificationv1beta2.Provider, username string) {
	intfluxcd.SetProviderUsername(provider, username)
}

// SetProviderAddress delegates to the internal helper.
func SetProviderAddress(provider *notificationv1beta2.Provider, address string) {
	intfluxcd.SetProviderAddress(provider, address)
}

// SetProviderTimeout delegates to the internal helper.
func SetProviderTimeout(provider *notificationv1beta2.Provider, d metav1.Duration) {
	intfluxcd.SetProviderTimeout(provider, d)
}

// SetProviderProxy delegates to the internal helper.
func SetProviderProxy(provider *notificationv1beta2.Provider, proxy string) {
	intfluxcd.SetProviderProxy(provider, proxy)
}

// SetProviderSecretRef delegates to the internal helper.
func SetProviderSecretRef(provider *notificationv1beta2.Provider, ref *meta.LocalObjectReference) {
	intfluxcd.SetProviderSecretRef(provider, ref)
}

// SetProviderCertSecretRef delegates to the internal helper.
func SetProviderCertSecretRef(provider *notificationv1beta2.Provider, ref *meta.LocalObjectReference) {
	intfluxcd.SetProviderCertSecretRef(provider, ref)
}

// SetProviderSuspend delegates to the internal helper.
func SetProviderSuspend(provider *notificationv1beta2.Provider, suspend bool) {
	intfluxcd.SetProviderSuspend(provider, suspend)
}
