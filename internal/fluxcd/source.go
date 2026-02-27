package fluxcd

import (
	"github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateGitRepository returns a new GitRepository resource with the provided
// name, namespace and spec.
func CreateGitRepository(name string, namespace string, spec sourcev1.GitRepositorySpec) *sourcev1.GitRepository {
	obj := &sourcev1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// CreateHelmRepository returns a new HelmRepository resource using the given
// specification.
func CreateHelmRepository(name string, namespace string, spec sourcev1.HelmRepositorySpec) *sourcev1.HelmRepository {
	obj := &sourcev1.HelmRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// CreateOCIRepository returns a new OCIRepository resource.
func CreateOCIRepository(name string, namespace string, spec sourcev1beta2.OCIRepositorySpec) *sourcev1beta2.OCIRepository {
	obj := &sourcev1beta2.OCIRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OCIRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// CreateBucket returns a new Bucket resource.
func CreateBucket(name, namespace string, spec sourcev1.BucketSpec) *sourcev1.Bucket {
	obj := &sourcev1.Bucket{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Bucket",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// CreateHelmChart returns a new HelmChart resource to pull and template a chart.
func CreateHelmChart(name, namespace string, spec sourcev1.HelmChartSpec) *sourcev1.HelmChart {
	obj := &sourcev1.HelmChart{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmChart",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// GitRepository helpers

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

// HelmRepository helpers

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

// Bucket helpers

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

// HelmChart helpers

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

// OCIRepository helpers

// SetOCIRepositoryURL sets the container registry URL.
func SetOCIRepositoryURL(or *sourcev1beta2.OCIRepository, url string) {
	or.Spec.URL = url
}

// SetOCIRepositoryReference sets the tag or digest reference.
func SetOCIRepositoryReference(or *sourcev1beta2.OCIRepository, ref *sourcev1beta2.OCIRepositoryRef) {
	or.Spec.Reference = ref
}

// SetOCIRepositoryLayerSelector configures the layer selector used to pull images.
func SetOCIRepositoryLayerSelector(or *sourcev1beta2.OCIRepository, sel *sourcev1beta2.OCILayerSelector) {
	or.Spec.LayerSelector = sel
}

// SetOCIRepositoryProvider sets the provider name.
func SetOCIRepositoryProvider(or *sourcev1beta2.OCIRepository, provider string) {
	or.Spec.Provider = provider
}

// SetOCIRepositorySecretRef attaches credentials secret reference.
func SetOCIRepositorySecretRef(or *sourcev1beta2.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.SecretRef = ref
}

// SetOCIRepositoryVerify configures OCI signature verification for the repository.
func SetOCIRepositoryVerify(or *sourcev1beta2.OCIRepository, verify *sourcev1.OCIRepositoryVerification) {
	or.Spec.Verify = verify
}

// SetOCIRepositoryServiceAccountName sets the service account used for pulls.
func SetOCIRepositoryServiceAccountName(or *sourcev1beta2.OCIRepository, name string) {
	or.Spec.ServiceAccountName = name
}

// SetOCIRepositoryCertSecretRef configures the certificate secret reference.
func SetOCIRepositoryCertSecretRef(or *sourcev1beta2.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.CertSecretRef = ref
}

// SetOCIRepositoryProxySecretRef attaches a proxy secret reference.
func SetOCIRepositoryProxySecretRef(or *sourcev1beta2.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.ProxySecretRef = ref
}

// SetOCIRepositoryInterval sets how often the repository is pulled.
func SetOCIRepositoryInterval(or *sourcev1beta2.OCIRepository, interval metav1.Duration) {
	or.Spec.Interval = interval
}

// SetOCIRepositoryTimeout configures the timeout for registry operations.
func SetOCIRepositoryTimeout(or *sourcev1beta2.OCIRepository, timeout *metav1.Duration) {
	or.Spec.Timeout = timeout
}

// SetOCIRepositoryIgnore configures ignore rules for the repository.
func SetOCIRepositoryIgnore(or *sourcev1beta2.OCIRepository, ignore string) {
	or.Spec.Ignore = &ignore
}

// SetOCIRepositoryInsecure toggles insecure pulls from the repository.
func SetOCIRepositoryInsecure(or *sourcev1beta2.OCIRepository, insecure bool) {
	or.Spec.Insecure = insecure
}

// SetOCIRepositorySuspend toggles reconciliation for the OCIRepository.
func SetOCIRepositorySuspend(or *sourcev1beta2.OCIRepository, suspend bool) {
	or.Spec.Suspend = suspend
}
