package fluxcd

import (
	"encoding/json"
	"fmt"

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
	sourceWatcherv1beta1 "github.com/fluxcd/source-watcher/api/v2/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitRepository setters

// SetGitRepositorySecretRef attaches a Secret reference for authentication.
func SetGitRepositorySecretRef(gr *sourcev1.GitRepository, ref *meta.LocalObjectReference) {
	gr.Spec.SecretRef = ref
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

// AddGitRepositoryInclude appends an include rule to the repository spec.
func AddGitRepositoryInclude(gr *sourcev1.GitRepository, include sourcev1.GitRepositoryInclude) {
	gr.Spec.Include = append(gr.Spec.Include, include)
}

// AddGitRepositorySparseCheckoutPath appends a directory path to the sparse checkout list.
func AddGitRepositorySparseCheckoutPath(gr *sourcev1.GitRepository, path string) {
	gr.Spec.SparseCheckout = append(gr.Spec.SparseCheckout, path)
}

// HelmRepository setters

// SetHelmRepositorySecretRef attaches a Secret for authentication to the repository.
func SetHelmRepositorySecretRef(hr *sourcev1.HelmRepository, ref *meta.LocalObjectReference) {
	hr.Spec.SecretRef = ref
}

// SetHelmRepositoryCertSecretRef configures the certificate Secret reference.
func SetHelmRepositoryCertSecretRef(hr *sourcev1.HelmRepository, ref *meta.LocalObjectReference) {
	hr.Spec.CertSecretRef = ref
}

// SetHelmRepositoryTimeout configures the network timeout for repository requests.
func SetHelmRepositoryTimeout(hr *sourcev1.HelmRepository, timeout *metav1.Duration) {
	hr.Spec.Timeout = timeout
}

// SetHelmRepositoryAccessFrom sets access control for the repository.
func SetHelmRepositoryAccessFrom(hr *sourcev1.HelmRepository, access *acl.AccessFrom) {
	hr.Spec.AccessFrom = access
}

// Bucket setters

// SetBucketSTS sets the STS configuration for the bucket.
func SetBucketSTS(b *sourcev1.Bucket, sts *sourcev1.BucketSTSSpec) {
	b.Spec.STS = sts
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

// SetBucketTimeout configures the timeout for bucket operations.
func SetBucketTimeout(b *sourcev1.Bucket, timeout *metav1.Duration) {
	b.Spec.Timeout = timeout
}

// SetBucketIgnore configures patterns to ignore from the bucket.
func SetBucketIgnore(b *sourcev1.Bucket, ignore string) {
	b.Spec.Ignore = &ignore
}

// HelmChart setters

// AddHelmChartValuesFile appends a values file to the chart specification.
func AddHelmChartValuesFile(hc *sourcev1.HelmChart, file string) {
	hc.Spec.ValuesFiles = append(hc.Spec.ValuesFiles, file)
}

// SetHelmChartVerify configures OCI signature verification for the chart.
//
// source-controller/api v1.9 split the verification types: HelmChart.Spec.Verify
// is now *HelmChartVerification (distinct from OCIRepository's *OCIRepositoryVerification).
func SetHelmChartVerify(hc *sourcev1.HelmChart, verify *sourcev1.HelmChartVerification) {
	hc.Spec.Verify = verify
}

// OCIRepository setters

// SetOCIRepositoryReference sets the tag or digest reference.
func SetOCIRepositoryReference(or *sourcev1.OCIRepository, ref *sourcev1.OCIRepositoryRef) {
	or.Spec.Reference = ref
}

// SetOCIRepositoryLayerSelector configures the layer selector used to pull images.
func SetOCIRepositoryLayerSelector(or *sourcev1.OCIRepository, sel *sourcev1.OCILayerSelector) {
	or.Spec.LayerSelector = sel
}

// SetOCIRepositorySecretRef attaches credentials secret reference.
func SetOCIRepositorySecretRef(or *sourcev1.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.SecretRef = ref
}

// SetOCIRepositoryVerify configures OCI signature verification for the repository.
func SetOCIRepositoryVerify(or *sourcev1.OCIRepository, verify *sourcev1.OCIRepositoryVerification) {
	or.Spec.Verify = verify
}

// SetOCIRepositoryCertSecretRef configures the certificate secret reference.
func SetOCIRepositoryCertSecretRef(or *sourcev1.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.CertSecretRef = ref
}

// SetOCIRepositoryProxySecretRef attaches a proxy secret reference.
func SetOCIRepositoryProxySecretRef(or *sourcev1.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.ProxySecretRef = ref
}

// SetOCIRepositoryTimeout configures the timeout for registry operations.
func SetOCIRepositoryTimeout(or *sourcev1.OCIRepository, timeout *metav1.Duration) {
	or.Spec.Timeout = timeout
}

// SetOCIRepositoryIgnore configures ignore rules for the repository.
func SetOCIRepositoryIgnore(or *sourcev1.OCIRepository, ignore string) {
	or.Spec.Ignore = &ignore
}

// ExternalArtifact setters

// SetExternalArtifactSourceRef sets the source reference for the ExternalArtifact.
func SetExternalArtifactSourceRef(ea *sourcev1.ExternalArtifact, ref *meta.NamespacedObjectKindReference) {
	ea.Spec.SourceRef = ref
}

// ArtifactGenerator setters

// AddArtifactGeneratorSource appends a source reference to the ArtifactGenerator.
func AddArtifactGeneratorSource(ag *sourceWatcherv1beta1.ArtifactGenerator, source sourceWatcherv1beta1.SourceReference) {
	ag.Spec.Sources = append(ag.Spec.Sources, source)
}

// AddArtifactGeneratorOutputArtifact appends an output artifact to the ArtifactGenerator.
func AddArtifactGeneratorOutputArtifact(ag *sourceWatcherv1beta1.ArtifactGenerator, output sourceWatcherv1beta1.OutputArtifact) {
	ag.Spec.OutputArtifacts = append(ag.Spec.OutputArtifacts, output)
}

// CreateSourceReference constructs a SourceReference with required fields.
func CreateSourceReference(alias, name, kind string) sourceWatcherv1beta1.SourceReference {
	return sourceWatcherv1beta1.SourceReference{
		Alias: alias,
		Name:  name,
		Kind:  kind,
	}
}

// CreateOutputArtifact constructs an OutputArtifact with the given name.
func CreateOutputArtifact(name string) sourceWatcherv1beta1.OutputArtifact {
	return sourceWatcherv1beta1.OutputArtifact{Name: name}
}

// AddOutputArtifactCopyOperation appends a copy operation to an OutputArtifact.
func AddOutputArtifactCopyOperation(out *sourceWatcherv1beta1.OutputArtifact, op sourceWatcherv1beta1.CopyOperation) {
	out.Copy = append(out.Copy, op)
}

// CreateCopyOperation constructs a CopyOperation with required from and to paths.
// from format: "@<alias>/<glob-pattern>"; to format: "@artifact/<path>"
func CreateCopyOperation(from, to string) sourceWatcherv1beta1.CopyOperation {
	return sourceWatcherv1beta1.CopyOperation{From: from, To: to}
}

// AddCopyOperationExclude appends a glob pattern to the exclude list of a CopyOperation.
func AddCopyOperationExclude(op *sourceWatcherv1beta1.CopyOperation, pattern string) {
	op.Exclude = append(op.Exclude, pattern)
}

// Kustomization setters

// SetKustomizationRetryInterval sets the retry interval.
func SetKustomizationRetryInterval(k *kustv1.Kustomization, interval metav1.Duration) {
	k.Spec.RetryInterval = &interval
}

// SetKustomizationKubeConfig specifies a kubeconfig reference.
func SetKustomizationKubeConfig(k *kustv1.Kustomization, ref *meta.KubeConfigReference) {
	k.Spec.KubeConfig = ref
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

// SetKustomizationTimeout sets the timeout duration.
func SetKustomizationTimeout(k *kustv1.Kustomization, timeout metav1.Duration) {
	k.Spec.Timeout = &timeout
}

// AddKustomizationHealthCheckExpr appends a CEL-based custom health check expression.
func AddKustomizationHealthCheckExpr(k *kustv1.Kustomization, check kustomize.CustomHealthCheck) {
	k.Spec.HealthCheckExprs = append(k.Spec.HealthCheckExprs, check)
}

// CreateCustomHealthCheck constructs a CustomHealthCheck with required fields.
// current is the CEL expression for the desired-state condition.
func CreateCustomHealthCheck(apiVersion, kind, current string) kustomize.CustomHealthCheck {
	return kustomize.CustomHealthCheck{
		APIVersion: apiVersion,
		Kind:       kind,
		HealthCheckExpressions: kustomize.HealthCheckExpressions{
			Current: current,
		},
	}
}

// AddKustomizationImage appends an image transformation.
func AddKustomizationImage(k *kustv1.Kustomization, img kustomize.Image) {
	k.Spec.Images = append(k.Spec.Images, img)
}

// AddKustomizationPatch appends a strategic merge or JSON patch.
func AddKustomizationPatch(k *kustv1.Kustomization, patch kustomize.Patch) {
	k.Spec.Patches = append(k.Spec.Patches, patch)
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
//
// Object labels and annotations use the generic kubernetes.AddLabel /
// kubernetes.AddAnnotation over metav1.Object. The AddCommonMetadata* helpers
// above are unrelated: CommonMetadata is a Kustomization spec sub-type, not a
// metav1.Object, so the generic helpers cannot reach it.

// SetHelmReleaseChart sets the inline HelmChartTemplate.
func SetHelmReleaseChart(obj *helmv2.HelmRelease, chart *helmv2.HelmChartTemplate) {
	obj.Spec.Chart = chart
}

// SetHelmReleaseChartRef sets the cross namespace chart reference.
func SetHelmReleaseChartRef(obj *helmv2.HelmRelease, ref *helmv2.CrossNamespaceSourceReference) {
	obj.Spec.ChartRef = ref
}

// SetHelmReleaseKubeConfig sets the KubeConfig reference.
func SetHelmReleaseKubeConfig(obj *helmv2.HelmRelease, cfg *meta.KubeConfigReference) {
	obj.Spec.KubeConfig = cfg
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

// SetHelmReleaseValuesFromMap marshals values to JSON and sets them on the
// HelmRelease. A map that does not marshal — a channel, a function, a NaN —
// is a programming error and panics; a sugar helper returns no error under the
// builder contract. Values decoded from YAML or JSON always marshal. If yours
// can hold something that does not, marshal it yourself and pass the result to
// SetHelmReleaseValues, which takes the already-encoded JSON.
func SetHelmReleaseValuesFromMap(obj *helmv2.HelmRelease, values map[string]any) {
	raw, err := json.Marshal(values)
	if err != nil {
		panic(fmt.Sprintf("SetHelmReleaseValuesFromMap: %v", err))
	}
	obj.Spec.Values = &apiextensionsv1.JSON{Raw: raw}
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

// SetHelmReleaseCommonMetadata sets the common labels and annotations applied to all rendered resources.
func SetHelmReleaseCommonMetadata(obj *helmv2.HelmRelease, cm *helmv2.CommonMetadata) {
	obj.Spec.CommonMetadata = cm
}

// AddHelmReleaseHealthCheckExpr appends a CEL-based health check expression to the HelmRelease.
func AddHelmReleaseHealthCheckExpr(obj *helmv2.HelmRelease, check kustomize.CustomHealthCheck) {
	obj.Spec.HealthCheckExprs = append(obj.Spec.HealthCheckExprs, check)
}

// SetHelmReleaseInstallTimeout sets the timeout for the Helm install action.
func SetHelmReleaseInstallTimeout(obj *helmv2.HelmRelease, timeout *metav1.Duration) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.Timeout = timeout
}

// SetHelmReleaseInstallCRDs sets the CRD policy for the Helm install action.
func SetHelmReleaseInstallCRDs(obj *helmv2.HelmRelease, policy helmv2.CRDsPolicy) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.CRDs = policy
}

// SetHelmReleaseInstallCreateNamespace configures namespace creation during install.
func SetHelmReleaseInstallCreateNamespace(obj *helmv2.HelmRelease, create bool) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.CreateNamespace = create
}

// SetHelmReleaseInstallDisableSchemaValidation disables JSON schema validation during install.
func SetHelmReleaseInstallDisableSchemaValidation(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.DisableSchemaValidation = disable
}

// SetHelmReleaseInstallDisableOpenAPIValidation disables OpenAPI validation during install.
func SetHelmReleaseInstallDisableOpenAPIValidation(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.DisableOpenAPIValidation = disable
}

// SetHelmReleaseInstallDisableHooks prevents hooks from running during install.
func SetHelmReleaseInstallDisableHooks(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.DisableHooks = disable
}

// SetHelmReleaseInstallDisableWait disables waiting for resources to be ready after install.
func SetHelmReleaseInstallDisableWait(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.DisableWait = disable
}

// SetHelmReleaseInstallDisableWaitForJobs disables waiting for jobs after install.
func SetHelmReleaseInstallDisableWaitForJobs(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.DisableWaitForJobs = disable
}

// SetHelmReleaseInstallDisableTakeOwnership disables taking ownership of existing resources during install.
func SetHelmReleaseInstallDisableTakeOwnership(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.DisableTakeOwnership = disable
}

// SetHelmReleaseInstallReplace re-uses the release name if it is a deleted release in history.
func SetHelmReleaseInstallReplace(obj *helmv2.HelmRelease, replace bool) {
	if obj.Spec.Install == nil {
		obj.Spec.Install = &helmv2.Install{}
	}
	obj.Spec.Install.Replace = replace
}

// SetHelmReleaseUpgradeTimeout sets the timeout for the Helm upgrade action.
func SetHelmReleaseUpgradeTimeout(obj *helmv2.HelmRelease, timeout *metav1.Duration) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.Timeout = timeout
}

// SetHelmReleaseUpgradeCRDs sets the CRD policy for the Helm upgrade action.
func SetHelmReleaseUpgradeCRDs(obj *helmv2.HelmRelease, policy helmv2.CRDsPolicy) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.CRDs = policy
}

// SetHelmReleaseUpgradeDisableSchemaValidation disables JSON schema validation during upgrade.
func SetHelmReleaseUpgradeDisableSchemaValidation(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.DisableSchemaValidation = disable
}

// SetHelmReleaseUpgradeDisableOpenAPIValidation disables OpenAPI validation during upgrade.
func SetHelmReleaseUpgradeDisableOpenAPIValidation(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.DisableOpenAPIValidation = disable
}

// SetHelmReleaseUpgradeDisableHooks prevents hooks from running during upgrade.
func SetHelmReleaseUpgradeDisableHooks(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.DisableHooks = disable
}

// SetHelmReleaseUpgradeDisableWait disables waiting for resources to be ready after upgrade.
func SetHelmReleaseUpgradeDisableWait(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.DisableWait = disable
}

// SetHelmReleaseUpgradeDisableWaitForJobs disables waiting for jobs after upgrade.
func SetHelmReleaseUpgradeDisableWaitForJobs(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.DisableWaitForJobs = disable
}

// SetHelmReleaseUpgradeDisableTakeOwnership disables taking ownership of existing resources during upgrade.
func SetHelmReleaseUpgradeDisableTakeOwnership(obj *helmv2.HelmRelease, disable bool) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.DisableTakeOwnership = disable
}

// SetHelmReleaseUpgradeForce forces resource updates through a replacement strategy during upgrade.
func SetHelmReleaseUpgradeForce(obj *helmv2.HelmRelease, force bool) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.Force = force
}

// SetHelmReleaseUpgradePreserveValues makes Helm reuse the last release's values during upgrade.
func SetHelmReleaseUpgradePreserveValues(obj *helmv2.HelmRelease, preserve bool) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.PreserveValues = preserve
}

// SetHelmReleaseUpgradeCleanupOnFail allows deletion of new resources when upgrade fails.
func SetHelmReleaseUpgradeCleanupOnFail(obj *helmv2.HelmRelease, cleanup bool) {
	if obj.Spec.Upgrade == nil {
		obj.Spec.Upgrade = &helmv2.Upgrade{}
	}
	obj.Spec.Upgrade.CleanupOnFail = cleanup
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

// SetProviderInterval configures the interval at which events are sent.
func SetProviderInterval(provider *notificationv1beta3.Provider, d metav1.Duration) {
	provider.Spec.Interval = &d
}

// SetProviderTimeout sets the timeout for sending notifications.
func SetProviderTimeout(provider *notificationv1beta3.Provider, d metav1.Duration) {
	provider.Spec.Timeout = &d
}

// SetProviderSecretRef attaches a Secret reference to the provider.
func SetProviderSecretRef(provider *notificationv1beta3.Provider, ref *meta.LocalObjectReference) {
	provider.Spec.SecretRef = ref
}

// SetProviderCertSecretRef attaches a certificate Secret reference to the provider.
func SetProviderCertSecretRef(provider *notificationv1beta3.Provider, ref *meta.LocalObjectReference) {
	provider.Spec.CertSecretRef = ref
}

// Alert setters

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

// Receiver setters

// SetReceiverInterval configures how often resources are scanned.
func SetReceiverInterval(receiver *notificationv1.Receiver, d metav1.Duration) {
	receiver.Spec.Interval = &d
}

// AddReceiverEvent appends an event to the receiver specification.
func AddReceiverEvent(receiver *notificationv1.Receiver, event string) {
	receiver.Spec.Events = append(receiver.Spec.Events, event)
}

// AddReceiverResource registers a resource reference on the receiver.
//
// notification-controller/api v1.9 changed Receiver.Spec.Resources to []ReceiverResource,
// which embeds CrossNamespaceObjectReference plus an optional CEL Filter. The reference is
// wrapped with no filter to preserve prior behavior.
func AddReceiverResource(receiver *notificationv1.Receiver, ref notificationv1.CrossNamespaceObjectReference) {
	receiver.Spec.Resources = append(receiver.Spec.Resources, notificationv1.ReceiverResource{CrossNamespaceObjectReference: ref})
}

// SetReceiverSecretRef adds a Secret reference to the receiver.
//
// notification-controller/api v1.9 changed Receiver.Spec.SecretRef to a pointer.
func SetReceiverSecretRef(receiver *notificationv1.Receiver, ref meta.LocalObjectReference) {
	receiver.Spec.SecretRef = &ref
}

// ImageUpdateAutomation setters

// SetImageUpdateAutomationGitSpec sets the git specification for the automation.
func SetImageUpdateAutomationGitSpec(auto *imagev1.ImageUpdateAutomation, spec *imagev1.GitSpec) {
	auto.Spec.GitSpec = spec
}

// SetImageUpdateAutomationPolicySelector sets the policy selector.
func SetImageUpdateAutomationPolicySelector(auto *imagev1.ImageUpdateAutomation, selector *metav1.LabelSelector) {
	auto.Spec.PolicySelector = selector
}

// SetImageUpdateAutomationUpdateStrategy sets the update strategy.
func SetImageUpdateAutomationUpdateStrategy(auto *imagev1.ImageUpdateAutomation, strategy *imagev1.UpdateStrategy) {
	auto.Spec.Update = strategy
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

// AddCommitMessageTemplateValue adds a single key/value pair to the template values map.
func AddCommitMessageTemplateValue(spec *imagev1.CommitSpec, key, value string) {
	if spec.MessageTemplateValues == nil {
		spec.MessageTemplateValues = make(map[string]string)
	}
	spec.MessageTemplateValues[key] = value
}

// CreatePushSpec returns a PushSpec.
func CreatePushSpec(branch, refspec string, options map[string]string) *imagev1.PushSpec {
	return &imagev1.PushSpec{Branch: branch, Refspec: refspec, Options: options}
}

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

// SetGitSpecPush sets the push spec.
func SetGitSpecPush(spec *imagev1.GitSpec, push *imagev1.PushSpec) { spec.Push = push }

// CreateUpdateStrategy creates an UpdateStrategy struct.
func CreateUpdateStrategy(strategy imagev1.UpdateStrategyName, path string) *imagev1.UpdateStrategy {
	return &imagev1.UpdateStrategy{Strategy: strategy, Path: path}
}

// CreateImageRef constructs an ImageRef.
func CreateImageRef(name, tag, digest string) imagev1.ImageRef {
	return imagev1.ImageRef{Name: name, Tag: tag, Digest: digest}
}

// AddObservedPolicy records an observed policy in the automation status.
func AddObservedPolicy(auto *imagev1.ImageUpdateAutomation, name string, ref imagev1.ImageRef) {
	if auto.Status.ObservedPolicies == nil {
		auto.Status.ObservedPolicies = make(imagev1.ObservedPolicies)
	}
	auto.Status.ObservedPolicies[name] = ref
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

// AddResourceSetDependency appends a dependency.
func AddResourceSetDependency(rs *fluxv1.ResourceSet, dep fluxv1.Dependency) {
	rs.Spec.DependsOn = append(rs.Spec.DependsOn, dep)
}

// SetResourceSetCommonMetadata sets the common metadata.
func SetResourceSetCommonMetadata(rs *fluxv1.ResourceSet, cm *fluxv1.CommonMetadata) {
	rs.Spec.CommonMetadata = cm
}

// ResourceSetInputProvider setters

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
