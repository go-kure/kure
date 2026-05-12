package fluxcd

import (
	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	"github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitRepository setters

// SetGitRepositoryURL sets the repository clone URL.
func SetGitRepositoryURL(gr *sourcev1.GitRepository, url string) {
	gr.Spec.URL = url
}

// SetGitRepositorySecretRef attaches a Secret reference for authentication.
func SetGitRepositorySecretRef(gr *sourcev1.GitRepository, ref *meta.LocalObjectReference) {
	gr.Spec.SecretRef = ref
}

// SetGitRepositoryProvider specifies the hosting provider of the repository.
func SetGitRepositoryProvider(gr *sourcev1.GitRepository, provider string) {
	gr.Spec.Provider = provider
}

// SetGitRepositoryInterval sets the interval at which the repository is polled.
func SetGitRepositoryInterval(gr *sourcev1.GitRepository, interval metav1.Duration) {
	gr.Spec.Interval = interval
}

// SetGitRepositoryTimeout configures the timeout for Git operations.
func SetGitRepositoryTimeout(gr *sourcev1.GitRepository, timeout *metav1.Duration) {
	gr.Spec.Timeout = timeout
}

// SetGitRepositoryReference sets the revision reference for the repository.
func SetGitRepositoryReference(gr *sourcev1.GitRepository, ref *sourcev1.GitRepositoryRef) {
	gr.Spec.Reference = ref
}

// SetGitRepositoryVerification configures commit signature verification.
func SetGitRepositoryVerification(gr *sourcev1.GitRepository, ver *sourcev1.GitRepositoryVerification) {
	gr.Spec.Verification = ver
}

// SetGitRepositoryProxySecretRef attaches a proxy Secret reference.
func SetGitRepositoryProxySecretRef(gr *sourcev1.GitRepository, ref *meta.LocalObjectReference) {
	gr.Spec.ProxySecretRef = ref
}

// SetGitRepositoryIgnore sets the ignore pattern file contents.
func SetGitRepositoryIgnore(gr *sourcev1.GitRepository, ignore string) {
	gr.Spec.Ignore = &ignore
}

// SetGitRepositorySuspend toggles reconciliation for the repository.
func SetGitRepositorySuspend(gr *sourcev1.GitRepository, suspend bool) {
	gr.Spec.Suspend = suspend
}

// SetGitRepositoryRecurseSubmodules enables or disables submodule recursion.
func SetGitRepositoryRecurseSubmodules(gr *sourcev1.GitRepository, recurse bool) {
	gr.Spec.RecurseSubmodules = recurse
}

// AddGitRepositoryInclude appends an include rule to the repository spec.
func AddGitRepositoryInclude(gr *sourcev1.GitRepository, include sourcev1.GitRepositoryInclude) {
	gr.Spec.Include = append(gr.Spec.Include, include)
}

// HelmRepository setters

// SetHelmRepositoryURL sets the repository URL.
func SetHelmRepositoryURL(hr *sourcev1.HelmRepository, url string) {
	hr.Spec.URL = url
}

// SetHelmRepositorySecretRef attaches a Secret for authentication to the repository.
func SetHelmRepositorySecretRef(hr *sourcev1.HelmRepository, ref *meta.LocalObjectReference) {
	hr.Spec.SecretRef = ref
}

// SetHelmRepositoryCertSecretRef configures the certificate Secret reference.
func SetHelmRepositoryCertSecretRef(hr *sourcev1.HelmRepository, ref *meta.LocalObjectReference) {
	hr.Spec.CertSecretRef = ref
}

// SetHelmRepositoryPassCredentials toggles passing credentials to subdomains.
func SetHelmRepositoryPassCredentials(hr *sourcev1.HelmRepository, v bool) {
	hr.Spec.PassCredentials = v
}

// SetHelmRepositoryInterval sets how often the repository is polled.
func SetHelmRepositoryInterval(hr *sourcev1.HelmRepository, interval metav1.Duration) {
	hr.Spec.Interval = interval
}

// SetHelmRepositoryInsecure toggles skipping TLS verification.
func SetHelmRepositoryInsecure(hr *sourcev1.HelmRepository, insecure bool) {
	hr.Spec.Insecure = insecure
}

// SetHelmRepositoryTimeout configures the network timeout for repository requests.
func SetHelmRepositoryTimeout(hr *sourcev1.HelmRepository, timeout *metav1.Duration) {
	hr.Spec.Timeout = timeout
}

// SetHelmRepositorySuspend toggles reconciliation for the repository.
func SetHelmRepositorySuspend(hr *sourcev1.HelmRepository, suspend bool) {
	hr.Spec.Suspend = suspend
}

// SetHelmRepositoryAccessFrom sets access control for the repository.
func SetHelmRepositoryAccessFrom(hr *sourcev1.HelmRepository, access *acl.AccessFrom) {
	hr.Spec.AccessFrom = access
}

// SetHelmRepositoryType sets the repository type.
func SetHelmRepositoryType(hr *sourcev1.HelmRepository, typ string) {
	hr.Spec.Type = typ
}

// SetHelmRepositoryProvider specifies the provider name for the repository.
func SetHelmRepositoryProvider(hr *sourcev1.HelmRepository, provider string) {
	hr.Spec.Provider = provider
}

// Bucket setters

// SetBucketProvider sets the cloud provider for the bucket.
func SetBucketProvider(b *sourcev1.Bucket, provider string) {
	b.Spec.Provider = provider
}

// SetBucketName sets the bucket name.
func SetBucketName(b *sourcev1.Bucket, name string) {
	b.Spec.BucketName = name
}

// SetBucketEndpoint configures the bucket API endpoint.
func SetBucketEndpoint(b *sourcev1.Bucket, endpoint string) {
	b.Spec.Endpoint = endpoint
}

// SetBucketSTS sets the STS configuration for the bucket.
func SetBucketSTS(b *sourcev1.Bucket, sts *sourcev1.BucketSTSSpec) {
	b.Spec.STS = sts
}

// SetBucketInsecure toggles insecure TLS for bucket requests.
func SetBucketInsecure(b *sourcev1.Bucket, insecure bool) {
	b.Spec.Insecure = insecure
}

// SetBucketRegion sets the bucket region.
func SetBucketRegion(b *sourcev1.Bucket, region string) {
	b.Spec.Region = region
}

// SetBucketPrefix sets the bucket prefix path.
func SetBucketPrefix(b *sourcev1.Bucket, prefix string) {
	b.Spec.Prefix = prefix
}

// SetBucketSecretRef attaches credentials secret reference.
func SetBucketSecretRef(b *sourcev1.Bucket, ref *meta.LocalObjectReference) {
	b.Spec.SecretRef = ref
}

// SetBucketCertSecretRef sets the certificate secret for the bucket.
func SetBucketCertSecretRef(b *sourcev1.Bucket, ref *meta.LocalObjectReference) {
	b.Spec.CertSecretRef = ref
}

// SetBucketProxySecretRef attaches a proxy secret reference to the bucket.
func SetBucketProxySecretRef(b *sourcev1.Bucket, ref *meta.LocalObjectReference) {
	b.Spec.ProxySecretRef = ref
}

// SetBucketInterval sets how often the bucket is checked for updates.
func SetBucketInterval(b *sourcev1.Bucket, interval metav1.Duration) {
	b.Spec.Interval = interval
}

// SetBucketTimeout configures the timeout for bucket operations.
func SetBucketTimeout(b *sourcev1.Bucket, timeout *metav1.Duration) {
	b.Spec.Timeout = timeout
}

// SetBucketIgnore configures patterns to ignore from the bucket.
func SetBucketIgnore(b *sourcev1.Bucket, ignore string) {
	b.Spec.Ignore = &ignore
}

// SetBucketSuspend toggles reconciliation of the bucket source.
func SetBucketSuspend(b *sourcev1.Bucket, suspend bool) {
	b.Spec.Suspend = suspend
}

// HelmChart setters

// SetHelmChartChart sets the chart name on the HelmChart.
func SetHelmChartChart(hc *sourcev1.HelmChart, chart string) {
	hc.Spec.Chart = chart
}

// SetHelmChartVersion sets the chart version to fetch.
func SetHelmChartVersion(hc *sourcev1.HelmChart, version string) {
	hc.Spec.Version = version
}

// SetHelmChartSourceRef sets the source reference for the chart.
func SetHelmChartSourceRef(hc *sourcev1.HelmChart, ref sourcev1.LocalHelmChartSourceReference) {
	hc.Spec.SourceRef = ref
}

// SetHelmChartInterval configures how often the chart is reconciled.
func SetHelmChartInterval(hc *sourcev1.HelmChart, interval metav1.Duration) {
	hc.Spec.Interval = interval
}

// SetHelmChartReconcileStrategy sets the reconcile strategy for templating.
func SetHelmChartReconcileStrategy(hc *sourcev1.HelmChart, strategy string) {
	hc.Spec.ReconcileStrategy = strategy
}

// AddHelmChartValuesFile appends a values file to the chart specification.
func AddHelmChartValuesFile(hc *sourcev1.HelmChart, file string) {
	hc.Spec.ValuesFiles = append(hc.Spec.ValuesFiles, file)
}

// SetHelmChartValuesFiles replaces the values files list.
func SetHelmChartValuesFiles(hc *sourcev1.HelmChart, files []string) {
	hc.Spec.ValuesFiles = files
}

// SetHelmChartIgnoreMissingValuesFiles toggles ignoring missing values files.
func SetHelmChartIgnoreMissingValuesFiles(hc *sourcev1.HelmChart, ignore bool) {
	hc.Spec.IgnoreMissingValuesFiles = ignore
}

// SetHelmChartSuspend toggles reconciliation of the chart.
func SetHelmChartSuspend(hc *sourcev1.HelmChart, suspend bool) {
	hc.Spec.Suspend = suspend
}

// SetHelmChartVerify configures OCI signature verification for the chart.
func SetHelmChartVerify(hc *sourcev1.HelmChart, verify *sourcev1.OCIRepositoryVerification) {
	hc.Spec.Verify = verify
}

// OCIRepository setters

// SetOCIRepositoryURL sets the container registry URL.
func SetOCIRepositoryURL(or *sourcev1.OCIRepository, url string) {
	or.Spec.URL = url
}

// SetOCIRepositoryReference sets the tag or digest reference.
func SetOCIRepositoryReference(or *sourcev1.OCIRepository, ref *sourcev1.OCIRepositoryRef) {
	or.Spec.Reference = ref
}

// SetOCIRepositoryLayerSelector configures the layer selector used to pull images.
func SetOCIRepositoryLayerSelector(or *sourcev1.OCIRepository, sel *sourcev1.OCILayerSelector) {
	or.Spec.LayerSelector = sel
}

// SetOCIRepositoryProvider sets the provider name.
func SetOCIRepositoryProvider(or *sourcev1.OCIRepository, provider string) {
	or.Spec.Provider = provider
}

// SetOCIRepositorySecretRef attaches credentials secret reference.
func SetOCIRepositorySecretRef(or *sourcev1.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.SecretRef = ref
}

// SetOCIRepositoryVerify configures OCI signature verification for the repository.
func SetOCIRepositoryVerify(or *sourcev1.OCIRepository, verify *sourcev1.OCIRepositoryVerification) {
	or.Spec.Verify = verify
}

// SetOCIRepositoryServiceAccountName sets the service account used for pulls.
func SetOCIRepositoryServiceAccountName(or *sourcev1.OCIRepository, name string) {
	or.Spec.ServiceAccountName = name
}

// SetOCIRepositoryCertSecretRef configures the certificate secret reference.
func SetOCIRepositoryCertSecretRef(or *sourcev1.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.CertSecretRef = ref
}

// SetOCIRepositoryProxySecretRef attaches a proxy secret reference.
func SetOCIRepositoryProxySecretRef(or *sourcev1.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.ProxySecretRef = ref
}

// SetOCIRepositoryInterval sets how often the repository is pulled.
func SetOCIRepositoryInterval(or *sourcev1.OCIRepository, interval metav1.Duration) {
	or.Spec.Interval = interval
}

// SetOCIRepositoryTimeout configures the timeout for registry operations.
func SetOCIRepositoryTimeout(or *sourcev1.OCIRepository, timeout *metav1.Duration) {
	or.Spec.Timeout = timeout
}

// SetOCIRepositoryIgnore configures ignore rules for the repository.
func SetOCIRepositoryIgnore(or *sourcev1.OCIRepository, ignore string) {
	or.Spec.Ignore = &ignore
}

// SetOCIRepositoryInsecure toggles insecure pulls from the repository.
func SetOCIRepositoryInsecure(or *sourcev1.OCIRepository, insecure bool) {
	or.Spec.Insecure = insecure
}

// SetOCIRepositorySuspend toggles reconciliation for the OCIRepository.
func SetOCIRepositorySuspend(or *sourcev1.OCIRepository, suspend bool) {
	or.Spec.Suspend = suspend
}

// Kustomization setters

// SetKustomizationInterval updates the reconciliation interval.
func SetKustomizationInterval(k *kustv1.Kustomization, interval metav1.Duration) {
	k.Spec.Interval = interval
}

// SetKustomizationRetryInterval sets the retry interval.
func SetKustomizationRetryInterval(k *kustv1.Kustomization, interval metav1.Duration) {
	k.Spec.RetryInterval = &interval
}

// SetKustomizationPath sets the path field.
func SetKustomizationPath(k *kustv1.Kustomization, path string) {
	k.Spec.Path = path
}

// SetKustomizationKubeConfig specifies a kubeconfig reference.
func SetKustomizationKubeConfig(k *kustv1.Kustomization, ref *meta.KubeConfigReference) {
	k.Spec.KubeConfig = ref
}

// SetKustomizationSourceRef sets the source reference.
func SetKustomizationSourceRef(k *kustv1.Kustomization, ref kustv1.CrossNamespaceSourceReference) {
	k.Spec.SourceRef = ref
}

// SetKustomizationPrune sets the prune option.
func SetKustomizationPrune(k *kustv1.Kustomization, prune bool) {
	k.Spec.Prune = prune
}

// SetKustomizationDeletionPolicy sets the deletion policy.
func SetKustomizationDeletionPolicy(k *kustv1.Kustomization, policy string) {
	k.Spec.DeletionPolicy = policy
}

// AddKustomizationHealthCheck appends a health check reference.
func AddKustomizationHealthCheck(k *kustv1.Kustomization, ref meta.NamespacedObjectKindReference) {
	k.Spec.HealthChecks = append(k.Spec.HealthChecks, ref)
}

// AddKustomizationComponent adds a component path.
func AddKustomizationComponent(k *kustv1.Kustomization, component string) {
	k.Spec.Components = append(k.Spec.Components, component)
}

// AddKustomizationDependsOn appends a dependency reference.
func AddKustomizationDependsOn(k *kustv1.Kustomization, ref kustv1.DependencyReference) {
	k.Spec.DependsOn = append(k.Spec.DependsOn, ref)
}

// SetKustomizationServiceAccountName sets the service account name.
func SetKustomizationServiceAccountName(k *kustv1.Kustomization, name string) {
	k.Spec.ServiceAccountName = name
}

// SetKustomizationSuspend sets the suspend flag.
func SetKustomizationSuspend(k *kustv1.Kustomization, suspend bool) {
	k.Spec.Suspend = suspend
}

// SetKustomizationTargetNamespace overrides the target namespace.
func SetKustomizationTargetNamespace(k *kustv1.Kustomization, namespace string) {
	k.Spec.TargetNamespace = namespace
}

// SetKustomizationTimeout sets the timeout duration.
func SetKustomizationTimeout(k *kustv1.Kustomization, timeout metav1.Duration) {
	k.Spec.Timeout = &timeout
}

// SetKustomizationForce sets the force flag.
func SetKustomizationForce(k *kustv1.Kustomization, force bool) {
	k.Spec.Force = force
}

// SetKustomizationWait sets the wait flag.
func SetKustomizationWait(k *kustv1.Kustomization, wait bool) {
	k.Spec.Wait = wait
}

// AddKustomizationImage appends an image transformation.
func AddKustomizationImage(k *kustv1.Kustomization, img kustomize.Image) {
	k.Spec.Images = append(k.Spec.Images, img)
}

// AddKustomizationPatch appends a strategic merge or JSON patch.
func AddKustomizationPatch(k *kustv1.Kustomization, patch kustomize.Patch) {
	k.Spec.Patches = append(k.Spec.Patches, patch)
}

// SetKustomizationNamePrefix sets the name prefix.
func SetKustomizationNamePrefix(k *kustv1.Kustomization, prefix string) {
	k.Spec.NamePrefix = prefix
}

// SetKustomizationNameSuffix sets the name suffix.
func SetKustomizationNameSuffix(k *kustv1.Kustomization, suffix string) {
	k.Spec.NameSuffix = suffix
}

// SetKustomizationCommonMetadata sets common labels and annotations.
func SetKustomizationCommonMetadata(k *kustv1.Kustomization, cm *kustv1.CommonMetadata) {
	k.Spec.CommonMetadata = cm
}

// SetKustomizationDecryption sets the decryption configuration.
func SetKustomizationDecryption(k *kustv1.Kustomization, d *kustv1.Decryption) {
	k.Spec.Decryption = d
}

// SetKustomizationPostBuild sets the post build configuration.
func SetKustomizationPostBuild(k *kustv1.Kustomization, pb *kustv1.PostBuild) {
	k.Spec.PostBuild = pb
}

// CreatePostBuild returns a PostBuild with initialized fields.
func CreatePostBuild() *kustv1.PostBuild {
	return &kustv1.PostBuild{Substitute: map[string]string{}, SubstituteFrom: []kustv1.SubstituteReference{}}
}

// AddPostBuildSubstitute adds a substitute variable.
func AddPostBuildSubstitute(pb *kustv1.PostBuild, key, value string) {
	if pb.Substitute == nil {
		pb.Substitute = make(map[string]string)
	}
	pb.Substitute[key] = value
}

// AddPostBuildSubstituteFrom adds a substitution source reference.
func AddPostBuildSubstituteFrom(pb *kustv1.PostBuild, ref kustv1.SubstituteReference) {
	pb.SubstituteFrom = append(pb.SubstituteFrom, ref)
}

// CreateSubstituteReference constructs a SubstituteReference.
func CreateSubstituteReference(kind, name string, optional bool) kustv1.SubstituteReference {
	return kustv1.SubstituteReference{Kind: kind, Name: name, Optional: optional}
}

// CreateDecryption constructs a Decryption specification.
func CreateDecryption(provider string, secret *meta.LocalObjectReference) *kustv1.Decryption {
	return &kustv1.Decryption{Provider: provider, SecretRef: secret}
}

// CreateCommonMetadata constructs CommonMetadata with initialized maps.
func CreateCommonMetadata() *kustv1.CommonMetadata {
	return &kustv1.CommonMetadata{Annotations: map[string]string{}, Labels: map[string]string{}}
}

// AddCommonMetadataLabel adds a label to CommonMetadata.
func AddCommonMetadataLabel(cm *kustv1.CommonMetadata, key, value string) {
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	cm.Labels[key] = value
}

// AddCommonMetadataAnnotation adds an annotation to CommonMetadata.
func AddCommonMetadataAnnotation(cm *kustv1.CommonMetadata, key, value string) {
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	cm.Annotations[key] = value
}

// HelmRelease setters

// AddHelmReleaseLabel adds a label to the HelmRelease metadata.
func AddHelmReleaseLabel(obj *helmv2.HelmRelease, key, value string) {
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[key] = value
}

// AddHelmReleaseAnnotation adds an annotation to the HelmRelease metadata.
func AddHelmReleaseAnnotation(obj *helmv2.HelmRelease, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[key] = value
}

// SetHelmReleaseChart sets the inline HelmChartTemplate.
func SetHelmReleaseChart(obj *helmv2.HelmRelease, chart *helmv2.HelmChartTemplate) {
	obj.Spec.Chart = chart
}

// SetHelmReleaseChartRef sets the cross namespace chart reference.
func SetHelmReleaseChartRef(obj *helmv2.HelmRelease, ref *helmv2.CrossNamespaceSourceReference) {
	obj.Spec.ChartRef = ref
}

// SetHelmReleaseInterval sets the reconcile interval.
func SetHelmReleaseInterval(obj *helmv2.HelmRelease, interval metav1.Duration) {
	obj.Spec.Interval = interval
}

// SetHelmReleaseKubeConfig sets the KubeConfig reference.
func SetHelmReleaseKubeConfig(obj *helmv2.HelmRelease, cfg *meta.KubeConfigReference) {
	obj.Spec.KubeConfig = cfg
}

// SetHelmReleaseSuspend configures the suspend flag.
func SetHelmReleaseSuspend(obj *helmv2.HelmRelease, suspend bool) {
	obj.Spec.Suspend = suspend
}

// SetHelmReleaseReleaseName sets the Helm release name.
func SetHelmReleaseReleaseName(obj *helmv2.HelmRelease, name string) {
	obj.Spec.ReleaseName = name
}

// SetHelmReleaseTargetNamespace sets the target namespace of the release.
func SetHelmReleaseTargetNamespace(obj *helmv2.HelmRelease, ns string) {
	obj.Spec.TargetNamespace = ns
}

// SetHelmReleaseStorageNamespace sets the storage namespace of the release.
func SetHelmReleaseStorageNamespace(obj *helmv2.HelmRelease, ns string) {
	obj.Spec.StorageNamespace = ns
}

// AddHelmReleaseDependsOn appends a dependency to the HelmRelease.
func AddHelmReleaseDependsOn(obj *helmv2.HelmRelease, ref helmv2.DependencyReference) {
	obj.Spec.DependsOn = append(obj.Spec.DependsOn, ref)
}

// SetHelmReleaseTimeout sets the timeout for the Helm actions.
func SetHelmReleaseTimeout(obj *helmv2.HelmRelease, timeout metav1.Duration) {
	obj.Spec.Timeout = &timeout
}

// SetHelmReleaseMaxHistory sets the maximum history to retain.
func SetHelmReleaseMaxHistory(obj *helmv2.HelmRelease, h int) {
	obj.Spec.MaxHistory = &h
}

// SetHelmReleaseServiceAccountName sets the service account name.
func SetHelmReleaseServiceAccountName(obj *helmv2.HelmRelease, name string) {
	obj.Spec.ServiceAccountName = name
}

// SetHelmReleasePersistentClient sets the persistent client flag.
func SetHelmReleasePersistentClient(obj *helmv2.HelmRelease, b bool) {
	obj.Spec.PersistentClient = &b
}

// SetHelmReleaseDriftDetection sets the drift detection configuration.
func SetHelmReleaseDriftDetection(obj *helmv2.HelmRelease, dd *helmv2.DriftDetection) {
	obj.Spec.DriftDetection = dd
}

// CreateDriftDetection returns a DriftDetection with the given mode.
func CreateDriftDetection(mode helmv2.DriftDetectionMode) *helmv2.DriftDetection {
	return &helmv2.DriftDetection{Mode: mode}
}

// AddDriftDetectionIgnoreRule appends an ignore rule.
func AddDriftDetectionIgnoreRule(dd *helmv2.DriftDetection, rule helmv2.IgnoreRule) {
	dd.Ignore = append(dd.Ignore, rule)
}

// CreateIgnoreRule constructs an IgnoreRule with the given paths and optional target selector.
func CreateIgnoreRule(paths []string, target *kustomize.Selector) helmv2.IgnoreRule {
	return helmv2.IgnoreRule{Paths: paths, Target: target}
}

// SetHelmReleaseInstall sets the install configuration.
func SetHelmReleaseInstall(obj *helmv2.HelmRelease, install *helmv2.Install) {
	obj.Spec.Install = install
}

// SetHelmReleaseUpgrade sets the upgrade configuration.
func SetHelmReleaseUpgrade(obj *helmv2.HelmRelease, upgrade *helmv2.Upgrade) {
	obj.Spec.Upgrade = upgrade
}

// SetHelmReleaseRollback sets the rollback configuration.
func SetHelmReleaseRollback(obj *helmv2.HelmRelease, rollback *helmv2.Rollback) {
	obj.Spec.Rollback = rollback
}

// SetHelmReleaseUninstall sets the uninstall configuration.
func SetHelmReleaseUninstall(obj *helmv2.HelmRelease, uninstall *helmv2.Uninstall) {
	obj.Spec.Uninstall = uninstall
}

// SetHelmReleaseTest sets the test configuration.
func SetHelmReleaseTest(obj *helmv2.HelmRelease, test *helmv2.Test) {
	obj.Spec.Test = test
}

// AddHelmReleaseValuesFrom appends a valuesFrom reference.
func AddHelmReleaseValuesFrom(obj *helmv2.HelmRelease, ref helmv2.ValuesReference) {
	obj.Spec.ValuesFrom = append(obj.Spec.ValuesFrom, ref)
}

// SetHelmReleaseValues sets the values for the release.
func SetHelmReleaseValues(obj *helmv2.HelmRelease, values *apiextensionsv1.JSON) {
	obj.Spec.Values = values
}

// AddHelmReleasePostRenderer appends a post renderer.
func AddHelmReleasePostRenderer(obj *helmv2.HelmRelease, pr helmv2.PostRenderer) {
	obj.Spec.PostRenderers = append(obj.Spec.PostRenderers, pr)
}

// CreatePostRendererKustomize returns a Kustomize post-renderer with initialized slices.
func CreatePostRendererKustomize() *helmv2.Kustomize {
	return &helmv2.Kustomize{}
}

// AddPostRendererKustomizePatch appends a strategic merge or JSON patch.
func AddPostRendererKustomizePatch(k *helmv2.Kustomize, patch kustomize.Patch) {
	k.Patches = append(k.Patches, patch)
}

// AddPostRendererKustomizeImage appends an image transformation.
func AddPostRendererKustomizeImage(k *helmv2.Kustomize, img kustomize.Image) {
	k.Images = append(k.Images, img)
}

// SetHelmReleaseInstallRemediation sets the install remediation configuration.
func SetHelmReleaseInstallRemediation(obj *helmv2.HelmRelease, remediation *helmv2.InstallRemediation) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.Remediation = remediation
}

// SetHelmReleaseUpgradeRemediation sets the upgrade remediation configuration.
func SetHelmReleaseUpgradeRemediation(obj *helmv2.HelmRelease, remediation *helmv2.UpgradeRemediation) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.Remediation = remediation
}

// CreateInstallRemediation returns an InstallRemediation with the given retries.
func CreateInstallRemediation(retries int) *helmv2.InstallRemediation {
	return &helmv2.InstallRemediation{
		Retries: retries,
	}
}

// CreateUpgradeRemediation returns an UpgradeRemediation with the given retries.
func CreateUpgradeRemediation(retries int) *helmv2.UpgradeRemediation {
	return &helmv2.UpgradeRemediation{
		Retries: retries,
	}
}

// SetInstallRemediationIgnoreTestFailures sets the IgnoreTestFailures flag on install remediation.
func SetInstallRemediationIgnoreTestFailures(r *helmv2.InstallRemediation, ignore bool) {
	r.IgnoreTestFailures = &ignore
}

// SetInstallRemediationRemediateLastFailure sets the RemediateLastFailure flag on install remediation.
func SetInstallRemediationRemediateLastFailure(r *helmv2.InstallRemediation, remediate bool) {
	r.RemediateLastFailure = &remediate
}

// SetUpgradeRemediationIgnoreTestFailures sets the IgnoreTestFailures flag on upgrade remediation.
func SetUpgradeRemediationIgnoreTestFailures(r *helmv2.UpgradeRemediation, ignore bool) {
	r.IgnoreTestFailures = &ignore
}

// SetUpgradeRemediationRemediateLastFailure sets the RemediateLastFailure flag on upgrade remediation.
func SetUpgradeRemediationRemediateLastFailure(r *helmv2.UpgradeRemediation, remediate bool) {
	r.RemediateLastFailure = &remediate
}

// SetUpgradeRemediationStrategy sets the remediation strategy on upgrade remediation.
func SetUpgradeRemediationStrategy(r *helmv2.UpgradeRemediation, strategy helmv2.RemediationStrategy) {
	r.Strategy = &strategy
}

// SetHelmReleaseWaitStrategy sets the wait strategy for the HelmRelease.
func SetHelmReleaseWaitStrategy(obj *helmv2.HelmRelease, strategy *helmv2.WaitStrategy) {
	obj.Spec.WaitStrategy = strategy
}

// CreateWaitStrategy returns a WaitStrategy with the given name.
func CreateWaitStrategy(name helmv2.WaitStrategyName) *helmv2.WaitStrategy {
	return &helmv2.WaitStrategy{Name: name}
}

// Provider setters

// SetProviderType sets the notification provider type.
func SetProviderType(provider *notificationv1beta3.Provider, t string) {
	provider.Spec.Type = t
}

// SetProviderInterval configures the interval at which events are sent.
func SetProviderInterval(provider *notificationv1beta3.Provider, d metav1.Duration) {
	provider.Spec.Interval = &d
}

// SetProviderChannel specifies the target channel for notifications.
func SetProviderChannel(provider *notificationv1beta3.Provider, channel string) {
	provider.Spec.Channel = channel
}

// SetProviderUsername configures the username on the provider spec.
func SetProviderUsername(provider *notificationv1beta3.Provider, username string) {
	provider.Spec.Username = username
}

// SetProviderAddress sets the provider address.
func SetProviderAddress(provider *notificationv1beta3.Provider, address string) {
	provider.Spec.Address = address
}

// SetProviderTimeout sets the timeout for sending notifications.
func SetProviderTimeout(provider *notificationv1beta3.Provider, d metav1.Duration) {
	provider.Spec.Timeout = &d
}

// SetProviderProxy sets the HTTP proxy used when sending events.
func SetProviderProxy(provider *notificationv1beta3.Provider, proxy string) {
	provider.Spec.Proxy = proxy
}

// SetProviderSecretRef attaches a Secret reference to the provider.
func SetProviderSecretRef(provider *notificationv1beta3.Provider, ref *meta.LocalObjectReference) {
	provider.Spec.SecretRef = ref
}

// SetProviderCertSecretRef attaches a certificate Secret reference to the provider.
func SetProviderCertSecretRef(provider *notificationv1beta3.Provider, ref *meta.LocalObjectReference) {
	provider.Spec.CertSecretRef = ref
}

// SetProviderSuspend sets the suspend flag on the provider.
func SetProviderSuspend(provider *notificationv1beta3.Provider, suspend bool) {
	provider.Spec.Suspend = suspend
}

// Alert setters

// SetAlertProviderRef sets the provider reference for an alert.
func SetAlertProviderRef(alert *notificationv1beta3.Alert, ref meta.LocalObjectReference) {
	alert.Spec.ProviderRef = ref
}

// AddAlertEventSource appends an event source to the alert specification.
func AddAlertEventSource(alert *notificationv1beta3.Alert, ref notificationv1.CrossNamespaceObjectReference) {
	alert.Spec.EventSources = append(alert.Spec.EventSources, ref)
}

// AddAlertInclusion adds a regex pattern to the inclusion list.
func AddAlertInclusion(alert *notificationv1beta3.Alert, regex string) {
	alert.Spec.InclusionList = append(alert.Spec.InclusionList, regex)
}

// AddAlertExclusion adds a regex pattern to the exclusion list.
func AddAlertExclusion(alert *notificationv1beta3.Alert, regex string) {
	alert.Spec.ExclusionList = append(alert.Spec.ExclusionList, regex)
}

// AddAlertEventMetadata sets a metadata key/value on the alert.
func AddAlertEventMetadata(alert *notificationv1beta3.Alert, key, value string) {
	if alert.Spec.EventMetadata == nil {
		alert.Spec.EventMetadata = make(map[string]string)
	}
	alert.Spec.EventMetadata[key] = value
}

// SetAlertEventSeverity sets the severity level for events.
func SetAlertEventSeverity(alert *notificationv1beta3.Alert, sev string) {
	alert.Spec.EventSeverity = sev
}

// SetAlertSummary sets the alert summary message.
func SetAlertSummary(alert *notificationv1beta3.Alert, summary string) {
	alert.Spec.Summary = summary
}

// SetAlertSuspend toggles the suspend flag for the alert.
func SetAlertSuspend(alert *notificationv1beta3.Alert, suspend bool) {
	alert.Spec.Suspend = suspend
}

// Receiver setters

// SetReceiverType sets the receiver type.
func SetReceiverType(receiver *notificationv1.Receiver, t string) {
	receiver.Spec.Type = t
}

// SetReceiverInterval configures how often resources are scanned.
func SetReceiverInterval(receiver *notificationv1.Receiver, d metav1.Duration) {
	receiver.Spec.Interval = &d
}

// AddReceiverEvent appends an event to the receiver specification.
func AddReceiverEvent(receiver *notificationv1.Receiver, event string) {
	receiver.Spec.Events = append(receiver.Spec.Events, event)
}

// AddReceiverResource registers a resource reference on the receiver.
func AddReceiverResource(receiver *notificationv1.Receiver, ref notificationv1.CrossNamespaceObjectReference) {
	receiver.Spec.Resources = append(receiver.Spec.Resources, ref)
}

// SetReceiverSecretRef adds a Secret reference to the receiver.
func SetReceiverSecretRef(receiver *notificationv1.Receiver, ref meta.LocalObjectReference) {
	receiver.Spec.SecretRef = ref
}

// SetReceiverSuspend toggles the suspend flag for the receiver.
func SetReceiverSuspend(receiver *notificationv1.Receiver, suspend bool) {
	receiver.Spec.Suspend = suspend
}

// ImageUpdateAutomation setters

// SetImageUpdateAutomationSourceRef sets the source reference for the automation.
func SetImageUpdateAutomationSourceRef(auto *imagev1.ImageUpdateAutomation, ref imagev1.CrossNamespaceSourceReference) {
	auto.Spec.SourceRef = ref
}

// SetImageUpdateAutomationGitSpec sets the git specification for the automation.
func SetImageUpdateAutomationGitSpec(auto *imagev1.ImageUpdateAutomation, spec *imagev1.GitSpec) {
	auto.Spec.GitSpec = spec
}

// SetImageUpdateAutomationInterval sets the reconcile interval.
func SetImageUpdateAutomationInterval(auto *imagev1.ImageUpdateAutomation, interval metav1.Duration) {
	auto.Spec.Interval = interval
}

// SetImageUpdateAutomationPolicySelector sets the policy selector.
func SetImageUpdateAutomationPolicySelector(auto *imagev1.ImageUpdateAutomation, selector *metav1.LabelSelector) {
	auto.Spec.PolicySelector = selector
}

// SetImageUpdateAutomationUpdateStrategy sets the update strategy.
func SetImageUpdateAutomationUpdateStrategy(auto *imagev1.ImageUpdateAutomation, strategy *imagev1.UpdateStrategy) {
	auto.Spec.Update = strategy
}

// SetImageUpdateAutomationSuspend sets the suspend flag.
func SetImageUpdateAutomationSuspend(auto *imagev1.ImageUpdateAutomation, suspend bool) {
	auto.Spec.Suspend = suspend
}

// CreateCrossNamespaceSourceReference creates a new cross namespace source reference.
func CreateCrossNamespaceSourceReference(apiVersion, kind, name, namespace string) imagev1.CrossNamespaceSourceReference {
	return imagev1.CrossNamespaceSourceReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		Namespace:  namespace,
	}
}

// CreateGitCheckoutSpec creates a new GitCheckoutSpec.
func CreateGitCheckoutSpec(ref sourcev1.GitRepositoryRef) *imagev1.GitCheckoutSpec {
	return &imagev1.GitCheckoutSpec{Reference: ref}
}

// SetGitCheckoutReference sets the reference of the checkout spec.
func SetGitCheckoutReference(spec *imagev1.GitCheckoutSpec, ref sourcev1.GitRepositoryRef) {
	spec.Reference = ref
}

// CreateCommitUser returns a CommitUser struct.
func CreateCommitUser(name, email string) imagev1.CommitUser {
	return imagev1.CommitUser{Name: name, Email: email}
}

// CreateSigningKey returns a SigningKey with the secret reference populated.
func CreateSigningKey(secretName string) *imagev1.SigningKey {
	return &imagev1.SigningKey{SecretRef: meta.LocalObjectReference{Name: secretName}}
}

// CreateCommitSpec creates a CommitSpec with the given author.
func CreateCommitSpec(author imagev1.CommitUser) imagev1.CommitSpec {
	return imagev1.CommitSpec{Author: author}
}

// SetCommitSigningKey sets the signing key for a CommitSpec.
func SetCommitSigningKey(spec *imagev1.CommitSpec, key *imagev1.SigningKey) {
	spec.SigningKey = key
}

// SetCommitMessageTemplate sets the message template for a CommitSpec.
func SetCommitMessageTemplate(spec *imagev1.CommitSpec, tpl string) {
	spec.MessageTemplate = tpl
}

// SetCommitMessageTemplateValues replaces the message template values map.
func SetCommitMessageTemplateValues(spec *imagev1.CommitSpec, values map[string]string) {
	spec.MessageTemplateValues = values
}

// AddCommitMessageTemplateValue adds a single key/value pair to the template values map.
func AddCommitMessageTemplateValue(spec *imagev1.CommitSpec, key, value string) {
	if spec.MessageTemplateValues == nil {
		spec.MessageTemplateValues = make(map[string]string)
	}
	spec.MessageTemplateValues[key] = value
}

// SetCommitAuthor sets the author of the commit spec.
func SetCommitAuthor(spec *imagev1.CommitSpec, author imagev1.CommitUser) {
	spec.Author = author
}

// CreatePushSpec returns a PushSpec.
func CreatePushSpec(branch, refspec string, options map[string]string) *imagev1.PushSpec {
	return &imagev1.PushSpec{Branch: branch, Refspec: refspec, Options: options}
}

// SetPushBranch sets the branch for the push spec.
func SetPushBranch(spec *imagev1.PushSpec, branch string) { spec.Branch = branch }

// SetPushRefspec sets the refspec for the push spec.
func SetPushRefspec(spec *imagev1.PushSpec, refspec string) { spec.Refspec = refspec }

// SetPushOptions replaces the options map for the push spec.
func SetPushOptions(spec *imagev1.PushSpec, opts map[string]string) { spec.Options = opts }

// AddPushOption adds a single option to the push spec.
func AddPushOption(spec *imagev1.PushSpec, key, value string) {
	if spec.Options == nil {
		spec.Options = make(map[string]string)
	}
	spec.Options[key] = value
}

// CreateGitSpec creates a GitSpec struct.
func CreateGitSpec(commit imagev1.CommitSpec, checkout *imagev1.GitCheckoutSpec, push *imagev1.PushSpec) *imagev1.GitSpec {
	return &imagev1.GitSpec{Checkout: checkout, Commit: commit, Push: push}
}

// SetGitSpecCheckout sets the checkout spec.
func SetGitSpecCheckout(spec *imagev1.GitSpec, checkout *imagev1.GitCheckoutSpec) {
	spec.Checkout = checkout
}

// SetGitSpecCommit sets the commit spec.
func SetGitSpecCommit(spec *imagev1.GitSpec, commit imagev1.CommitSpec) { spec.Commit = commit }

// SetGitSpecPush sets the push spec.
func SetGitSpecPush(spec *imagev1.GitSpec, push *imagev1.PushSpec) { spec.Push = push }

// CreateUpdateStrategy creates an UpdateStrategy struct.
func CreateUpdateStrategy(strategy imagev1.UpdateStrategyName, path string) *imagev1.UpdateStrategy {
	return &imagev1.UpdateStrategy{Strategy: strategy, Path: path}
}

// SetUpdateStrategyName sets the strategy name.
func SetUpdateStrategyName(spec *imagev1.UpdateStrategy, name imagev1.UpdateStrategyName) {
	spec.Strategy = name
}

// SetUpdateStrategyPath sets the update path.
func SetUpdateStrategyPath(spec *imagev1.UpdateStrategy, path string) { spec.Path = path }

// CreateImageRef constructs an ImageRef.
func CreateImageRef(name, tag, digest string) imagev1.ImageRef {
	return imagev1.ImageRef{Name: name, Tag: tag, Digest: digest}
}

// SetImageRefDigest sets the digest on an ImageRef.
func SetImageRefDigest(ref *imagev1.ImageRef, digest string) { ref.Digest = digest }

// SetImageRefTag sets the tag on an ImageRef.
func SetImageRefTag(ref *imagev1.ImageRef, tag string) { ref.Tag = tag }

// SetImageRefName sets the name on an ImageRef.
func SetImageRefName(ref *imagev1.ImageRef, name string) { ref.Name = name }

// AddObservedPolicy records an observed policy in the automation status.
func AddObservedPolicy(auto *imagev1.ImageUpdateAutomation, name string, ref imagev1.ImageRef) {
	if auto.Status.ObservedPolicies == nil {
		auto.Status.ObservedPolicies = make(imagev1.ObservedPolicies)
	}
	auto.Status.ObservedPolicies[name] = ref
}

// SetObservedPolicies sets the observed policies map.
func SetObservedPolicies(auto *imagev1.ImageUpdateAutomation, policies imagev1.ObservedPolicies) {
	auto.Status.ObservedPolicies = policies
}

// ResourceSet setters

// AddResourceSetInput appends an input to the ResourceSet.
func AddResourceSetInput(rs *fluxv1.ResourceSet, in fluxv1.ResourceSetInput) {
	rs.Spec.Inputs = append(rs.Spec.Inputs, in)
}

// AddResourceSetInputFrom appends an input provider reference.
func AddResourceSetInputFrom(rs *fluxv1.ResourceSet, ref fluxv1.InputProviderReference) {
	rs.Spec.InputsFrom = append(rs.Spec.InputsFrom, ref)
}

// AddResourceSetResource appends a resource to reconcile.
func AddResourceSetResource(rs *fluxv1.ResourceSet, r *apiextensionsv1.JSON) {
	rs.Spec.Resources = append(rs.Spec.Resources, r)
}

// SetResourceSetResourcesTemplate sets the resources template.
func SetResourceSetResourcesTemplate(rs *fluxv1.ResourceSet, tpl string) {
	rs.Spec.ResourcesTemplate = tpl
}

// AddResourceSetDependency appends a dependency.
func AddResourceSetDependency(rs *fluxv1.ResourceSet, dep fluxv1.Dependency) {
	rs.Spec.DependsOn = append(rs.Spec.DependsOn, dep)
}

// SetResourceSetServiceAccountName sets the service account name.
func SetResourceSetServiceAccountName(rs *fluxv1.ResourceSet, name string) {
	rs.Spec.ServiceAccountName = name
}

// SetResourceSetWait sets the wait flag.
func SetResourceSetWait(rs *fluxv1.ResourceSet, wait bool) {
	rs.Spec.Wait = wait
}

// SetResourceSetCommonMetadata sets the common metadata.
func SetResourceSetCommonMetadata(rs *fluxv1.ResourceSet, cm *fluxv1.CommonMetadata) {
	rs.Spec.CommonMetadata = cm
}

// ResourceSetInputProvider setters

// SetResourceSetInputProviderType sets the provider type.
func SetResourceSetInputProviderType(obj *fluxv1.ResourceSetInputProvider, typ string) {
	obj.Spec.Type = typ
}

// SetResourceSetInputProviderURL sets the provider URL.
func SetResourceSetInputProviderURL(obj *fluxv1.ResourceSetInputProvider, url string) {
	obj.Spec.URL = url
}

// SetResourceSetInputProviderServiceAccountName sets the service account name.
func SetResourceSetInputProviderServiceAccountName(obj *fluxv1.ResourceSetInputProvider, name string) {
	obj.Spec.ServiceAccountName = name
}

// SetResourceSetInputProviderSecretRef sets the secret reference.
func SetResourceSetInputProviderSecretRef(obj *fluxv1.ResourceSetInputProvider, ref *meta.LocalObjectReference) {
	obj.Spec.SecretRef = ref
}

// SetResourceSetInputProviderCertSecretRef sets the certificate secret reference.
func SetResourceSetInputProviderCertSecretRef(obj *fluxv1.ResourceSetInputProvider, ref *meta.LocalObjectReference) {
	obj.Spec.CertSecretRef = ref
}

// AddResourceSetInputProviderSchedule appends a schedule to the provider.
func AddResourceSetInputProviderSchedule(obj *fluxv1.ResourceSetInputProvider, s fluxv1.Schedule) {
	obj.Spec.Schedule = append(obj.Spec.Schedule, s)
}

// FluxInstance setters

// AddFluxInstanceComponent appends a component to the FluxInstance spec.
func AddFluxInstanceComponent(obj *fluxv1.FluxInstance, c fluxv1.Component) {
	obj.Spec.Components = append(obj.Spec.Components, c)
}

// SetFluxInstanceDistribution sets the distribution of the FluxInstance.
func SetFluxInstanceDistribution(obj *fluxv1.FluxInstance, dist fluxv1.Distribution) {
	obj.Spec.Distribution = dist
}

// SetFluxInstanceDistributionVariant sets the distribution variant.
// Valid values: upstream-alpine, enterprise-alpine, enterprise-distroless, enterprise-distroless-fips.
func SetFluxInstanceDistributionVariant(obj *fluxv1.FluxInstance, variant string) {
	obj.Spec.Distribution.Variant = variant
}

// SetFluxInstanceCommonMetadata sets the common metadata.
func SetFluxInstanceCommonMetadata(obj *fluxv1.FluxInstance, cm *fluxv1.CommonMetadata) {
	obj.Spec.CommonMetadata = cm
}

// SetFluxInstanceCluster sets the cluster information.
func SetFluxInstanceCluster(obj *fluxv1.FluxInstance, cluster *fluxv1.Cluster) {
	obj.Spec.Cluster = cluster
}

// SetFluxInstanceSharding sets the sharding specification.
func SetFluxInstanceSharding(obj *fluxv1.FluxInstance, shard *fluxv1.Sharding) {
	obj.Spec.Sharding = shard
}

// SetFluxInstanceStorage sets the storage specification.
func SetFluxInstanceStorage(obj *fluxv1.FluxInstance, st *fluxv1.Storage) {
	obj.Spec.Storage = st
}

// SetFluxInstanceKustomize sets the kustomize specification.
func SetFluxInstanceKustomize(obj *fluxv1.FluxInstance, k *fluxv1.Kustomize) {
	obj.Spec.Kustomize = k
}

// SetFluxInstanceWait sets the wait flag.
func SetFluxInstanceWait(obj *fluxv1.FluxInstance, wait bool) {
	obj.Spec.Wait = &wait
}

// SetFluxInstanceMigrateResources sets the migrateResources flag.
func SetFluxInstanceMigrateResources(obj *fluxv1.FluxInstance, m bool) {
	obj.Spec.MigrateResources = &m
}

// SetFluxInstanceSync sets the sync configuration.
func SetFluxInstanceSync(obj *fluxv1.FluxInstance, sync *fluxv1.Sync) {
	obj.Spec.Sync = sync
}

// FluxReport setters

// SetFluxReportDistribution sets the distribution status.
func SetFluxReportDistribution(fr *fluxv1.FluxReport, dist fluxv1.FluxDistributionStatus) {
	fr.Spec.Distribution = dist
}

// SetFluxReportCluster sets the cluster info.
func SetFluxReportCluster(fr *fluxv1.FluxReport, c *fluxv1.ClusterInfo) {
	fr.Spec.Cluster = c
}

// SetFluxReportOperator sets the operator info.
func SetFluxReportOperator(fr *fluxv1.FluxReport, op *fluxv1.OperatorInfo) {
	fr.Spec.Operator = op
}

// AddFluxReportComponentStatus appends a component status.
func AddFluxReportComponentStatus(fr *fluxv1.FluxReport, cs fluxv1.FluxComponentStatus) {
	fr.Spec.ComponentsStatus = append(fr.Spec.ComponentsStatus, cs)
}

// AddFluxReportReconcilerStatus appends a reconciler status.
func AddFluxReportReconcilerStatus(fr *fluxv1.FluxReport, rs fluxv1.FluxReconcilerStatus) {
	fr.Spec.ReconcilersStatus = append(fr.Spec.ReconcilersStatus, rs)
}

// SetFluxReportSyncStatus sets the sync status.
func SetFluxReportSyncStatus(fr *fluxv1.FluxReport, s *fluxv1.FluxSyncStatus) {
	fr.Spec.SyncStatus = s
}

// Schedule helpers

// CreateSchedule returns a Schedule with the given cron expression.
func CreateSchedule(cron string) fluxv1.Schedule {
	return fluxv1.Schedule{Cron: cron}
}

// SetScheduleTimeZone sets the time zone on the schedule.
func SetScheduleTimeZone(s *fluxv1.Schedule, tz string) {
	s.TimeZone = tz
}

// SetScheduleWindow sets the execution window.
func SetScheduleWindow(s *fluxv1.Schedule, d metav1.Duration) {
	s.Window = d
}
