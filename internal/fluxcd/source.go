package fluxcd

import (
	acl "github.com/fluxcd/pkg/apis/acl"
	meta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
func SetGitRepositoryURL(gr *sourcev1.GitRepository, url string) {
	gr.Spec.URL = url
}

func SetGitRepositorySecretRef(gr *sourcev1.GitRepository, ref *meta.LocalObjectReference) {
	gr.Spec.SecretRef = ref
}

func SetGitRepositoryProvider(gr *sourcev1.GitRepository, provider string) {
	gr.Spec.Provider = provider
}

func SetGitRepositoryInterval(gr *sourcev1.GitRepository, interval metav1.Duration) {
	gr.Spec.Interval = interval
}

func SetGitRepositoryTimeout(gr *sourcev1.GitRepository, timeout *metav1.Duration) {
	gr.Spec.Timeout = timeout
}

func SetGitRepositoryReference(gr *sourcev1.GitRepository, ref *sourcev1.GitRepositoryRef) {
	gr.Spec.Reference = ref
}

func SetGitRepositoryVerification(gr *sourcev1.GitRepository, ver *sourcev1.GitRepositoryVerification) {
	gr.Spec.Verification = ver
}

func SetGitRepositoryProxySecretRef(gr *sourcev1.GitRepository, ref *meta.LocalObjectReference) {
	gr.Spec.ProxySecretRef = ref
}

func SetGitRepositoryIgnore(gr *sourcev1.GitRepository, ignore string) {
	gr.Spec.Ignore = &ignore
}

func SetGitRepositorySuspend(gr *sourcev1.GitRepository, suspend bool) {
	gr.Spec.Suspend = suspend
}

func SetGitRepositoryRecurseSubmodules(gr *sourcev1.GitRepository, recurse bool) {
	gr.Spec.RecurseSubmodules = recurse
}

func AddGitRepositoryInclude(gr *sourcev1.GitRepository, include sourcev1.GitRepositoryInclude) {
	gr.Spec.Include = append(gr.Spec.Include, include)
}

// HelmRepository helpers
func SetHelmRepositoryURL(hr *sourcev1.HelmRepository, url string) {
	hr.Spec.URL = url
}

func SetHelmRepositorySecretRef(hr *sourcev1.HelmRepository, ref *meta.LocalObjectReference) {
	hr.Spec.SecretRef = ref
}

func SetHelmRepositoryCertSecretRef(hr *sourcev1.HelmRepository, ref *meta.LocalObjectReference) {
	hr.Spec.CertSecretRef = ref
}

func SetHelmRepositoryPassCredentials(hr *sourcev1.HelmRepository, v bool) {
	hr.Spec.PassCredentials = v
}

func SetHelmRepositoryInterval(hr *sourcev1.HelmRepository, interval metav1.Duration) {
	hr.Spec.Interval = interval
}

func SetHelmRepositoryInsecure(hr *sourcev1.HelmRepository, insecure bool) {
	hr.Spec.Insecure = insecure
}

func SetHelmRepositoryTimeout(hr *sourcev1.HelmRepository, timeout *metav1.Duration) {
	hr.Spec.Timeout = timeout
}

func SetHelmRepositorySuspend(hr *sourcev1.HelmRepository, suspend bool) {
	hr.Spec.Suspend = suspend
}

func SetHelmRepositoryAccessFrom(hr *sourcev1.HelmRepository, access *acl.AccessFrom) {
	hr.Spec.AccessFrom = access
}

func SetHelmRepositoryType(hr *sourcev1.HelmRepository, typ string) {
	hr.Spec.Type = typ
}

func SetHelmRepositoryProvider(hr *sourcev1.HelmRepository, provider string) {
	hr.Spec.Provider = provider
}

// Bucket helpers
func SetBucketProvider(b *sourcev1.Bucket, provider string) {
	b.Spec.Provider = provider
}

func SetBucketName(b *sourcev1.Bucket, name string) {
	b.Spec.BucketName = name
}

func SetBucketEndpoint(b *sourcev1.Bucket, endpoint string) {
	b.Spec.Endpoint = endpoint
}

func SetBucketSTS(b *sourcev1.Bucket, sts *sourcev1.BucketSTSSpec) {
	b.Spec.STS = sts
}

func SetBucketInsecure(b *sourcev1.Bucket, insecure bool) {
	b.Spec.Insecure = insecure
}

func SetBucketRegion(b *sourcev1.Bucket, region string) {
	b.Spec.Region = region
}

func SetBucketPrefix(b *sourcev1.Bucket, prefix string) {
	b.Spec.Prefix = prefix
}

func SetBucketSecretRef(b *sourcev1.Bucket, ref *meta.LocalObjectReference) {
	b.Spec.SecretRef = ref
}

func SetBucketCertSecretRef(b *sourcev1.Bucket, ref *meta.LocalObjectReference) {
	b.Spec.CertSecretRef = ref
}

func SetBucketProxySecretRef(b *sourcev1.Bucket, ref *meta.LocalObjectReference) {
	b.Spec.ProxySecretRef = ref
}

func SetBucketInterval(b *sourcev1.Bucket, interval metav1.Duration) {
	b.Spec.Interval = interval
}

func SetBucketTimeout(b *sourcev1.Bucket, timeout *metav1.Duration) {
	b.Spec.Timeout = timeout
}

func SetBucketIgnore(b *sourcev1.Bucket, ignore string) {
	b.Spec.Ignore = &ignore
}

func SetBucketSuspend(b *sourcev1.Bucket, suspend bool) {
	b.Spec.Suspend = suspend
}

// HelmChart helpers
func SetHelmChartChart(hc *sourcev1.HelmChart, chart string) {
	hc.Spec.Chart = chart
}

func SetHelmChartVersion(hc *sourcev1.HelmChart, version string) {
	hc.Spec.Version = version
}

func SetHelmChartSourceRef(hc *sourcev1.HelmChart, ref sourcev1.LocalHelmChartSourceReference) {
	hc.Spec.SourceRef = ref
}

func SetHelmChartInterval(hc *sourcev1.HelmChart, interval metav1.Duration) {
	hc.Spec.Interval = interval
}

func SetHelmChartReconcileStrategy(hc *sourcev1.HelmChart, strategy string) {
	hc.Spec.ReconcileStrategy = strategy
}

func AddHelmChartValuesFile(hc *sourcev1.HelmChart, file string) {
	hc.Spec.ValuesFiles = append(hc.Spec.ValuesFiles, file)
}

func SetHelmChartValuesFiles(hc *sourcev1.HelmChart, files []string) {
	hc.Spec.ValuesFiles = files
}

func SetHelmChartIgnoreMissingValuesFiles(hc *sourcev1.HelmChart, ignore bool) {
	hc.Spec.IgnoreMissingValuesFiles = ignore
}

func SetHelmChartSuspend(hc *sourcev1.HelmChart, suspend bool) {
	hc.Spec.Suspend = suspend
}

func SetHelmChartVerify(hc *sourcev1.HelmChart, verify *sourcev1.OCIRepositoryVerification) {
	hc.Spec.Verify = verify
}

// OCIRepository helpers
func SetOCIRepositoryURL(or *sourcev1beta2.OCIRepository, url string) {
	or.Spec.URL = url
}

func SetOCIRepositoryReference(or *sourcev1beta2.OCIRepository, ref *sourcev1beta2.OCIRepositoryRef) {
	or.Spec.Reference = ref
}

func SetOCIRepositoryLayerSelector(or *sourcev1beta2.OCIRepository, sel *sourcev1beta2.OCILayerSelector) {
	or.Spec.LayerSelector = sel
}

func SetOCIRepositoryProvider(or *sourcev1beta2.OCIRepository, provider string) {
	or.Spec.Provider = provider
}

func SetOCIRepositorySecretRef(or *sourcev1beta2.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.SecretRef = ref
}

func SetOCIRepositoryVerify(or *sourcev1beta2.OCIRepository, verify *sourcev1.OCIRepositoryVerification) {
	or.Spec.Verify = verify
}

func SetOCIRepositoryServiceAccountName(or *sourcev1beta2.OCIRepository, name string) {
	or.Spec.ServiceAccountName = name
}

func SetOCIRepositoryCertSecretRef(or *sourcev1beta2.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.CertSecretRef = ref
}

func SetOCIRepositoryProxySecretRef(or *sourcev1beta2.OCIRepository, ref *meta.LocalObjectReference) {
	or.Spec.ProxySecretRef = ref
}

func SetOCIRepositoryInterval(or *sourcev1beta2.OCIRepository, interval metav1.Duration) {
	or.Spec.Interval = interval
}

func SetOCIRepositoryTimeout(or *sourcev1beta2.OCIRepository, timeout *metav1.Duration) {
	or.Spec.Timeout = timeout
}

func SetOCIRepositoryIgnore(or *sourcev1beta2.OCIRepository, ignore string) {
	or.Spec.Ignore = &ignore
}

func SetOCIRepositoryInsecure(or *sourcev1beta2.OCIRepository, insecure bool) {
	or.Spec.Insecure = insecure
}

func SetOCIRepositorySuspend(or *sourcev1beta2.OCIRepository, suspend bool) {
	or.Spec.Suspend = suspend
}
