package fluxcd

import (
	"testing"
	"time"

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

func TestSetGitRepositoryURL(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	SetGitRepositoryURL(obj, "https://github.com/example/repo")
	if obj.Spec.URL != "https://github.com/example/repo" {
		t.Errorf("got URL %q", obj.Spec.URL)
	}
}

func TestSetGitRepositorySecretRef(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	ref := &meta.LocalObjectReference{Name: "my-secret"}
	SetGitRepositorySecretRef(obj, ref)
	if obj.Spec.SecretRef != ref {
		t.Error("SecretRef not set")
	}
}

func TestSetGitRepositoryProvider(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	SetGitRepositoryProvider(obj, "github")
	if obj.Spec.Provider != "github" {
		t.Errorf("got provider %q", obj.Spec.Provider)
	}
}

func TestSetGitRepositoryInterval(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetGitRepositoryInterval(obj, d)
	if obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestSetGitRepositoryTimeout(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	d := metav1.Duration{Duration: 30 * time.Second}
	SetGitRepositoryTimeout(obj, &d)
	if obj.Spec.Timeout == nil || *obj.Spec.Timeout != d {
		t.Error("Timeout not set")
	}
}

func TestSetGitRepositoryReference(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	ref := &sourcev1.GitRepositoryRef{Branch: "main"}
	SetGitRepositoryReference(obj, ref)
	if obj.Spec.Reference != ref {
		t.Error("Reference not set")
	}
}

func TestSetGitRepositoryVerification(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	ver := &sourcev1.GitRepositoryVerification{SecretRef: meta.LocalObjectReference{Name: "cosign-pub-keys"}}
	SetGitRepositoryVerification(obj, ver)
	if obj.Spec.Verification != ver {
		t.Error("Verification not set")
	}
}

func TestSetGitRepositoryProxySecretRef(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	ref := &meta.LocalObjectReference{Name: "proxy-secret"}
	SetGitRepositoryProxySecretRef(obj, ref)
	if obj.Spec.ProxySecretRef != ref {
		t.Error("ProxySecretRef not set")
	}
}

func TestSetGitRepositoryIgnore(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	SetGitRepositoryIgnore(obj, "*.tmp")
	if obj.Spec.Ignore == nil || *obj.Spec.Ignore != "*.tmp" {
		t.Error("Ignore not set")
	}
}

func TestSetGitRepositorySuspend(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	SetGitRepositorySuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

func TestSetGitRepositoryRecurseSubmodules(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	SetGitRepositoryRecurseSubmodules(obj, true)
	if !obj.Spec.RecurseSubmodules {
		t.Error("RecurseSubmodules not set")
	}
}

func TestAddGitRepositoryInclude(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	inc := sourcev1.GitRepositoryInclude{GitRepositoryRef: meta.LocalObjectReference{Name: "other-repo"}}
	AddGitRepositoryInclude(obj, inc)
	if len(obj.Spec.Include) != 1 {
		t.Fatalf("expected 1 include, got %d", len(obj.Spec.Include))
	}
	if obj.Spec.Include[0].GitRepositoryRef.Name != "other-repo" {
		t.Error("Include not set correctly")
	}
}

func TestSetGitRepositorySparseCheckout(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	paths := []string{"deploy/", "config/"}
	SetGitRepositorySparseCheckout(obj, paths)
	if len(obj.Spec.SparseCheckout) != 2 {
		t.Fatalf("expected 2 sparse checkout paths, got %d", len(obj.Spec.SparseCheckout))
	}
	if obj.Spec.SparseCheckout[0] != "deploy/" {
		t.Errorf("got SparseCheckout[0] %q", obj.Spec.SparseCheckout[0])
	}
}

func TestAddGitRepositorySparseCheckoutPath(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	AddGitRepositorySparseCheckoutPath(obj, "apps/")
	AddGitRepositorySparseCheckoutPath(obj, "infra/")
	if len(obj.Spec.SparseCheckout) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(obj.Spec.SparseCheckout))
	}
	if obj.Spec.SparseCheckout[1] != "infra/" {
		t.Errorf("got SparseCheckout[1] %q", obj.Spec.SparseCheckout[1])
	}
}

func TestSetGitRepositoryServiceAccountName(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	SetGitRepositoryServiceAccountName(obj, "flux-sa")
	if obj.Spec.ServiceAccountName != "flux-sa" {
		t.Errorf("got ServiceAccountName %q", obj.Spec.ServiceAccountName)
	}
}

// HelmRepository setters

func TestSetHelmRepositoryURL(t *testing.T) {
	obj := CreateHelmRepository("bitnami", "ns")
	SetHelmRepositoryURL(obj, "https://charts.bitnami.com/bitnami")
	if obj.Spec.URL != "https://charts.bitnami.com/bitnami" {
		t.Errorf("got URL %q", obj.Spec.URL)
	}
}

func TestSetHelmRepositorySecretRef(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	ref := &meta.LocalObjectReference{Name: "helm-secret"}
	SetHelmRepositorySecretRef(obj, ref)
	if obj.Spec.SecretRef != ref {
		t.Error("SecretRef not set")
	}
}

func TestSetHelmRepositoryCertSecretRef(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	ref := &meta.LocalObjectReference{Name: "cert-secret"}
	SetHelmRepositoryCertSecretRef(obj, ref)
	if obj.Spec.CertSecretRef != ref {
		t.Error("CertSecretRef not set")
	}
}

func TestSetHelmRepositoryPassCredentials(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	SetHelmRepositoryPassCredentials(obj, true)
	if !obj.Spec.PassCredentials {
		t.Error("PassCredentials not set")
	}
}

func TestSetHelmRepositoryInterval(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	d := metav1.Duration{Duration: 10 * time.Minute}
	SetHelmRepositoryInterval(obj, d)
	if obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestSetHelmRepositoryInsecure(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	SetHelmRepositoryInsecure(obj, true)
	if !obj.Spec.Insecure {
		t.Error("Insecure not set")
	}
}

func TestSetHelmRepositoryTimeout(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	d := metav1.Duration{Duration: 60 * time.Second}
	SetHelmRepositoryTimeout(obj, &d)
	if obj.Spec.Timeout == nil || *obj.Spec.Timeout != d {
		t.Error("Timeout not set")
	}
}

func TestSetHelmRepositorySuspend(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	SetHelmRepositorySuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

func TestSetHelmRepositoryAccessFrom(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	access := &acl.AccessFrom{NamespaceSelectors: []acl.NamespaceSelector{{MatchLabels: map[string]string{"app": "test"}}}}
	SetHelmRepositoryAccessFrom(obj, access)
	if obj.Spec.AccessFrom != access {
		t.Error("AccessFrom not set")
	}
}

func TestSetHelmRepositoryType(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	SetHelmRepositoryType(obj, "oci")
	if obj.Spec.Type != "oci" {
		t.Errorf("got type %q", obj.Spec.Type)
	}
}

func TestSetHelmRepositoryProvider(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	SetHelmRepositoryProvider(obj, "aws")
	if obj.Spec.Provider != "aws" {
		t.Errorf("got provider %q", obj.Spec.Provider)
	}
}

// Bucket setters

func TestSetBucketProvider(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	SetBucketProvider(obj, "aws")
	if obj.Spec.Provider != "aws" {
		t.Errorf("got provider %q", obj.Spec.Provider)
	}
}

func TestSetBucketName(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	SetBucketName(obj, "my-bucket")
	if obj.Spec.BucketName != "my-bucket" {
		t.Errorf("got BucketName %q", obj.Spec.BucketName)
	}
}

func TestSetBucketEndpoint(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	SetBucketEndpoint(obj, "https://s3.example.com")
	if obj.Spec.Endpoint != "https://s3.example.com" {
		t.Errorf("got Endpoint %q", obj.Spec.Endpoint)
	}
}

func TestSetBucketSTS(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	sts := &sourcev1.BucketSTSSpec{Provider: "aws"}
	SetBucketSTS(obj, sts)
	if obj.Spec.STS != sts {
		t.Error("STS not set")
	}
}

func TestSetBucketInsecure(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	SetBucketInsecure(obj, true)
	if !obj.Spec.Insecure {
		t.Error("Insecure not set")
	}
}

func TestSetBucketRegion(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	SetBucketRegion(obj, "us-east-1")
	if obj.Spec.Region != "us-east-1" {
		t.Errorf("got Region %q", obj.Spec.Region)
	}
}

func TestSetBucketPrefix(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	SetBucketPrefix(obj, "kube/")
	if obj.Spec.Prefix != "kube/" {
		t.Errorf("got Prefix %q", obj.Spec.Prefix)
	}
}

func TestSetBucketSecretRef(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	ref := &meta.LocalObjectReference{Name: "creds"}
	SetBucketSecretRef(obj, ref)
	if obj.Spec.SecretRef != ref {
		t.Error("SecretRef not set")
	}
}

func TestSetBucketCertSecretRef(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	ref := &meta.LocalObjectReference{Name: "cert"}
	SetBucketCertSecretRef(obj, ref)
	if obj.Spec.CertSecretRef != ref {
		t.Error("CertSecretRef not set")
	}
}

func TestSetBucketProxySecretRef(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	ref := &meta.LocalObjectReference{Name: "proxy"}
	SetBucketProxySecretRef(obj, ref)
	if obj.Spec.ProxySecretRef != ref {
		t.Error("ProxySecretRef not set")
	}
}

func TestSetBucketInterval(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetBucketInterval(obj, d)
	if obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestSetBucketTimeout(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	d := metav1.Duration{Duration: 30 * time.Second}
	SetBucketTimeout(obj, &d)
	if obj.Spec.Timeout == nil || *obj.Spec.Timeout != d {
		t.Error("Timeout not set")
	}
}

func TestSetBucketIgnore(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	SetBucketIgnore(obj, "*.log")
	if obj.Spec.Ignore == nil || *obj.Spec.Ignore != "*.log" {
		t.Error("Ignore not set")
	}
}

func TestSetBucketSuspend(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	SetBucketSuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

// HelmChart setters

func TestSetHelmChartChart(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	SetHelmChartChart(obj, "nginx")
	if obj.Spec.Chart != "nginx" {
		t.Errorf("got Chart %q", obj.Spec.Chart)
	}
}

func TestSetHelmChartVersion(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	SetHelmChartVersion(obj, "1.2.3")
	if obj.Spec.Version != "1.2.3" {
		t.Errorf("got Version %q", obj.Spec.Version)
	}
}

func TestSetHelmChartSourceRef(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	ref := sourcev1.LocalHelmChartSourceReference{Kind: "HelmRepository", Name: "bitnami"}
	SetHelmChartSourceRef(obj, ref)
	if obj.Spec.SourceRef.Name != "bitnami" {
		t.Errorf("got SourceRef.Name %q", obj.Spec.SourceRef.Name)
	}
}

func TestSetHelmChartInterval(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetHelmChartInterval(obj, d)
	if obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestSetHelmChartReconcileStrategy(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	SetHelmChartReconcileStrategy(obj, "ChartVersion")
	if obj.Spec.ReconcileStrategy != "ChartVersion" {
		t.Errorf("got ReconcileStrategy %q", obj.Spec.ReconcileStrategy)
	}
}

func TestAddHelmChartValuesFile(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	AddHelmChartValuesFile(obj, "values-prod.yaml")
	if len(obj.Spec.ValuesFiles) != 1 || obj.Spec.ValuesFiles[0] != "values-prod.yaml" {
		t.Error("ValuesFiles not appended")
	}
}

func TestSetHelmChartValuesFiles(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	files := []string{"values.yaml", "values-prod.yaml"}
	SetHelmChartValuesFiles(obj, files)
	if len(obj.Spec.ValuesFiles) != 2 {
		t.Errorf("expected 2 files, got %d", len(obj.Spec.ValuesFiles))
	}
}

func TestSetHelmChartIgnoreMissingValuesFiles(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	SetHelmChartIgnoreMissingValuesFiles(obj, true)
	if !obj.Spec.IgnoreMissingValuesFiles {
		t.Error("IgnoreMissingValuesFiles not set")
	}
}

func TestSetHelmChartSuspend(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	SetHelmChartSuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

func TestSetHelmChartVerify(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	verify := &sourcev1.OCIRepositoryVerification{Provider: "cosign"}
	SetHelmChartVerify(obj, verify)
	if obj.Spec.Verify != verify {
		t.Error("Verify not set")
	}
}

// OCIRepository setters

func TestSetOCIRepositoryURL(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	SetOCIRepositoryURL(obj, "oci://registry.example.com/repo")
	if obj.Spec.URL != "oci://registry.example.com/repo" {
		t.Errorf("got URL %q", obj.Spec.URL)
	}
}

func TestSetOCIRepositoryReference(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	ref := &sourcev1.OCIRepositoryRef{Tag: "latest"}
	SetOCIRepositoryReference(obj, ref)
	if obj.Spec.Reference != ref {
		t.Error("Reference not set")
	}
}

func TestSetOCIRepositoryLayerSelector(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	sel := &sourcev1.OCILayerSelector{MediaType: "application/vnd.cncf.flux.content.v1.tar+gzip"}
	SetOCIRepositoryLayerSelector(obj, sel)
	if obj.Spec.LayerSelector != sel {
		t.Error("LayerSelector not set")
	}
}

func TestSetOCIRepositoryProvider(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	SetOCIRepositoryProvider(obj, "aws")
	if obj.Spec.Provider != "aws" {
		t.Errorf("got Provider %q", obj.Spec.Provider)
	}
}

func TestSetOCIRepositorySecretRef(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	ref := &meta.LocalObjectReference{Name: "oci-creds"}
	SetOCIRepositorySecretRef(obj, ref)
	if obj.Spec.SecretRef != ref {
		t.Error("SecretRef not set")
	}
}

func TestSetOCIRepositoryVerify(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	verify := &sourcev1.OCIRepositoryVerification{Provider: "cosign"}
	SetOCIRepositoryVerify(obj, verify)
	if obj.Spec.Verify != verify {
		t.Error("Verify not set")
	}
}

func TestSetOCIRepositoryServiceAccountName(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	SetOCIRepositoryServiceAccountName(obj, "flux-sa")
	if obj.Spec.ServiceAccountName != "flux-sa" {
		t.Errorf("got ServiceAccountName %q", obj.Spec.ServiceAccountName)
	}
}

func TestSetOCIRepositoryCertSecretRef(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	ref := &meta.LocalObjectReference{Name: "cert-secret"}
	SetOCIRepositoryCertSecretRef(obj, ref)
	if obj.Spec.CertSecretRef != ref {
		t.Error("CertSecretRef not set")
	}
}

func TestSetOCIRepositoryProxySecretRef(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	ref := &meta.LocalObjectReference{Name: "proxy-secret"}
	SetOCIRepositoryProxySecretRef(obj, ref)
	if obj.Spec.ProxySecretRef != ref {
		t.Error("ProxySecretRef not set")
	}
}

func TestSetOCIRepositoryInterval(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetOCIRepositoryInterval(obj, d)
	if obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestSetOCIRepositoryTimeout(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	d := metav1.Duration{Duration: 60 * time.Second}
	SetOCIRepositoryTimeout(obj, &d)
	if obj.Spec.Timeout == nil || *obj.Spec.Timeout != d {
		t.Error("Timeout not set")
	}
}

func TestSetOCIRepositoryIgnore(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	SetOCIRepositoryIgnore(obj, "*.tmp")
	if obj.Spec.Ignore == nil || *obj.Spec.Ignore != "*.tmp" {
		t.Error("Ignore not set")
	}
}

func TestSetOCIRepositoryInsecure(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	SetOCIRepositoryInsecure(obj, true)
	if !obj.Spec.Insecure {
		t.Error("Insecure not set")
	}
}

func TestSetOCIRepositorySuspend(t *testing.T) {
	obj := CreateOCIRepository("oci-repo", "ns")
	SetOCIRepositorySuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

// ExternalArtifact setters

func TestSetExternalArtifactSourceRef(t *testing.T) {
	obj := CreateExternalArtifact("ea", "ns")
	ref := &meta.NamespacedObjectKindReference{
		APIVersion: "source.toolkit.fluxcd.io/v1",
		Kind:       "OCIRepository",
		Name:       "my-oci-repo",
		Namespace:  "flux-system",
	}
	SetExternalArtifactSourceRef(obj, ref)
	if obj.Spec.SourceRef != ref {
		t.Error("SourceRef not set")
	}
}

// ArtifactGenerator setters

func TestAddArtifactGeneratorSource(t *testing.T) {
	ag := CreateArtifactGenerator("ag", "flux-system")
	src := CreateSourceReference("apps", "my-git-repo", "GitRepository")
	AddArtifactGeneratorSource(ag, src)
	if len(ag.Spec.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(ag.Spec.Sources))
	}
	if ag.Spec.Sources[0].Alias != "apps" {
		t.Errorf("got Alias %q", ag.Spec.Sources[0].Alias)
	}
	if ag.Spec.Sources[0].Kind != "GitRepository" {
		t.Errorf("got Kind %q", ag.Spec.Sources[0].Kind)
	}
}

func TestAddArtifactGeneratorOutputArtifact(t *testing.T) {
	ag := CreateArtifactGenerator("ag", "flux-system")
	out := CreateOutputArtifact("combined")
	op := CreateCopyOperation("@apps/deploy/", "@artifact/deploy/")
	AddOutputArtifactCopyOperation(&out, op)
	AddArtifactGeneratorOutputArtifact(ag, out)
	if len(ag.Spec.OutputArtifacts) != 1 {
		t.Fatalf("expected 1 output artifact, got %d", len(ag.Spec.OutputArtifacts))
	}
	if ag.Spec.OutputArtifacts[0].Name != "combined" {
		t.Errorf("got Name %q", ag.Spec.OutputArtifacts[0].Name)
	}
	if len(ag.Spec.OutputArtifacts[0].Copy) != 1 {
		t.Fatalf("expected 1 copy op, got %d", len(ag.Spec.OutputArtifacts[0].Copy))
	}
}

func TestCreateSourceReference(t *testing.T) {
	ref := CreateSourceReference("infra", "infra-repo", "OCIRepository")
	if ref.Alias != "infra" {
		t.Errorf("got Alias %q", ref.Alias)
	}
	if ref.Name != "infra-repo" {
		t.Errorf("got Name %q", ref.Name)
	}
	if ref.Kind != "OCIRepository" {
		t.Errorf("got Kind %q", ref.Kind)
	}
}

func TestSetSourceReferenceNamespace(t *testing.T) {
	ref := CreateSourceReference("apps", "my-repo", "GitRepository")
	SetSourceReferenceNamespace(&ref, "other-ns")
	if ref.Namespace != "other-ns" {
		t.Errorf("got Namespace %q", ref.Namespace)
	}
}

func TestCreateOutputArtifact(t *testing.T) {
	out := CreateOutputArtifact("merged")
	if out.Name != "merged" {
		t.Errorf("got Name %q", out.Name)
	}
}

func TestSetOutputArtifactRevision(t *testing.T) {
	out := CreateOutputArtifact("out")
	SetOutputArtifactRevision(&out, "@apps")
	if out.Revision != "@apps" {
		t.Errorf("got Revision %q", out.Revision)
	}
}

func TestSetOutputArtifactOriginRevision(t *testing.T) {
	out := CreateOutputArtifact("out")
	SetOutputArtifactOriginRevision(&out, "@apps")
	if out.OriginRevision != "@apps" {
		t.Errorf("got OriginRevision %q", out.OriginRevision)
	}
}

func TestAddOutputArtifactCopyOperation(t *testing.T) {
	out := CreateOutputArtifact("out")
	op := CreateCopyOperation("@apps/deploy/", "@artifact/deploy/")
	AddOutputArtifactCopyOperation(&out, op)
	if len(out.Copy) != 1 {
		t.Fatalf("expected 1 copy op, got %d", len(out.Copy))
	}
	if out.Copy[0].From != "@apps/deploy/" {
		t.Errorf("got From %q", out.Copy[0].From)
	}
}

func TestCreateCopyOperation(t *testing.T) {
	op := CreateCopyOperation("@apps/manifests/", "@artifact/manifests/")
	if op.From != "@apps/manifests/" {
		t.Errorf("got From %q", op.From)
	}
	if op.To != "@artifact/manifests/" {
		t.Errorf("got To %q", op.To)
	}
}

func TestAddCopyOperationExclude(t *testing.T) {
	op := CreateCopyOperation("@apps/", "@artifact/")
	AddCopyOperationExclude(&op, "*.tmp")
	AddCopyOperationExclude(&op, ".git/")
	if len(op.Exclude) != 2 {
		t.Fatalf("expected 2 excludes, got %d", len(op.Exclude))
	}
	if op.Exclude[0] != "*.tmp" {
		t.Errorf("got Exclude[0] %q", op.Exclude[0])
	}
}

func TestSetCopyOperationStrategy(t *testing.T) {
	op := CreateCopyOperation("@apps/", "@artifact/")
	SetCopyOperationStrategy(&op, sourceWatcherv1beta1.MergeStrategy)
	if op.Strategy != sourceWatcherv1beta1.MergeStrategy {
		t.Errorf("got Strategy %q", op.Strategy)
	}
}

// Kustomization setters

func TestSetKustomizationInterval(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	d := metav1.Duration{Duration: 10 * time.Minute}
	SetKustomizationInterval(obj, d)
	if obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestSetKustomizationRetryInterval(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	d := metav1.Duration{Duration: 2 * time.Minute}
	SetKustomizationRetryInterval(obj, d)
	if obj.Spec.RetryInterval == nil || *obj.Spec.RetryInterval != d {
		t.Error("RetryInterval not set")
	}
}

func TestSetKustomizationPath(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationPath(obj, "./deploy/prod")
	if obj.Spec.Path != "./deploy/prod" {
		t.Errorf("got Path %q", obj.Spec.Path)
	}
}

func TestSetKustomizationKubeConfig(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	secretRef := &meta.SecretKeyReference{Name: "kube-cfg"}
	ref := &meta.KubeConfigReference{SecretRef: secretRef}
	SetKustomizationKubeConfig(obj, ref)
	if obj.Spec.KubeConfig != ref {
		t.Error("KubeConfig not set")
	}
}

func TestSetKustomizationSourceRef(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	ref := kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "my-repo"}
	SetKustomizationSourceRef(obj, ref)
	if obj.Spec.SourceRef.Name != "my-repo" {
		t.Errorf("got SourceRef.Name %q", obj.Spec.SourceRef.Name)
	}
}

func TestSetKustomizationPrune(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationPrune(obj, true)
	if !obj.Spec.Prune {
		t.Error("Prune not set")
	}
}

func TestSetKustomizationDeletionPolicy(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationDeletionPolicy(obj, "delete")
	if obj.Spec.DeletionPolicy != "delete" {
		t.Errorf("got DeletionPolicy %q", obj.Spec.DeletionPolicy)
	}
}

func TestAddKustomizationHealthCheck(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	ref := meta.NamespacedObjectKindReference{Kind: "Deployment", Name: "app", Namespace: "default"}
	AddKustomizationHealthCheck(obj, ref)
	if len(obj.Spec.HealthChecks) != 1 {
		t.Fatalf("expected 1 health check, got %d", len(obj.Spec.HealthChecks))
	}
	if obj.Spec.HealthChecks[0].Name != "app" {
		t.Error("HealthCheck not set correctly")
	}
}

func TestAddKustomizationComponent(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	AddKustomizationComponent(obj, "./components/monitoring")
	if len(obj.Spec.Components) != 1 || obj.Spec.Components[0] != "./components/monitoring" {
		t.Error("Component not appended")
	}
}

func TestAddKustomizationDependsOn(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	ref := kustv1.DependencyReference{Name: "infra"}
	AddKustomizationDependsOn(obj, ref)
	if len(obj.Spec.DependsOn) != 1 || obj.Spec.DependsOn[0].Name != "infra" {
		t.Error("DependsOn not appended")
	}
}

func TestSetKustomizationServiceAccountName(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationServiceAccountName(obj, "flux-sa")
	if obj.Spec.ServiceAccountName != "flux-sa" {
		t.Errorf("got ServiceAccountName %q", obj.Spec.ServiceAccountName)
	}
}

func TestSetKustomizationSuspend(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationSuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

func TestSetKustomizationTargetNamespace(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationTargetNamespace(obj, "production")
	if obj.Spec.TargetNamespace != "production" {
		t.Errorf("got TargetNamespace %q", obj.Spec.TargetNamespace)
	}
}

func TestSetKustomizationTimeout(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetKustomizationTimeout(obj, d)
	if obj.Spec.Timeout == nil || *obj.Spec.Timeout != d {
		t.Error("Timeout not set")
	}
}

func TestSetKustomizationForce(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationForce(obj, true)
	if !obj.Spec.Force {
		t.Error("Force not set")
	}
}

func TestSetKustomizationWait(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationWait(obj, true)
	if !obj.Spec.Wait {
		t.Error("Wait not set")
	}
}

func TestAddKustomizationImage(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	img := kustomize.Image{Name: "nginx", NewTag: "1.25"}
	AddKustomizationImage(obj, img)
	if len(obj.Spec.Images) != 1 || obj.Spec.Images[0].Name != "nginx" {
		t.Error("Image not appended")
	}
}

func TestAddKustomizationPatch(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	patch := kustomize.Patch{Patch: `[{"op":"add","path":"/metadata/labels/env","value":"prod"}]`}
	AddKustomizationPatch(obj, patch)
	if len(obj.Spec.Patches) != 1 {
		t.Error("Patch not appended")
	}
}

func TestSetKustomizationNamePrefix(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationNamePrefix(obj, "prod-")
	if obj.Spec.NamePrefix != "prod-" {
		t.Errorf("got NamePrefix %q", obj.Spec.NamePrefix)
	}
}

func TestSetKustomizationNameSuffix(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationNameSuffix(obj, "-v2")
	if obj.Spec.NameSuffix != "-v2" {
		t.Errorf("got NameSuffix %q", obj.Spec.NameSuffix)
	}
}

func TestSetKustomizationCommonMetadata(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	cm := CreateCommonMetadata()
	SetKustomizationCommonMetadata(obj, cm)
	if obj.Spec.CommonMetadata != cm {
		t.Error("CommonMetadata not set")
	}
}

func TestSetKustomizationDecryption(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	d := CreateDecryption("sops", &meta.LocalObjectReference{Name: "age-key"})
	SetKustomizationDecryption(obj, d)
	if obj.Spec.Decryption != d {
		t.Error("Decryption not set")
	}
}

func TestSetKustomizationPostBuild(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	pb := CreatePostBuild()
	SetKustomizationPostBuild(obj, pb)
	if obj.Spec.PostBuild != pb {
		t.Error("PostBuild not set")
	}
}

// PostBuild helpers

func TestCreatePostBuild(t *testing.T) {
	pb := CreatePostBuild()
	if pb == nil {
		t.Fatal("expected non-nil PostBuild")
	}
	if pb.Substitute == nil {
		t.Error("expected initialized Substitute map")
	}
	if pb.SubstituteFrom == nil {
		t.Error("expected initialized SubstituteFrom slice")
	}
}

func TestAddPostBuildSubstitute(t *testing.T) {
	pb := CreatePostBuild()
	AddPostBuildSubstitute(pb, "ENV", "prod")
	if pb.Substitute["ENV"] != "prod" {
		t.Error("Substitute key not set")
	}
}

func TestAddPostBuildSubstitute_NilMap(t *testing.T) {
	pb := &kustv1.PostBuild{}
	AddPostBuildSubstitute(pb, "ENV", "prod")
	if pb.Substitute["ENV"] != "prod" {
		t.Error("Substitute key not set after nil init")
	}
}

func TestAddPostBuildSubstituteFrom(t *testing.T) {
	pb := CreatePostBuild()
	ref := kustv1.SubstituteReference{Kind: "ConfigMap", Name: "vars"}
	AddPostBuildSubstituteFrom(pb, ref)
	if len(pb.SubstituteFrom) != 1 || pb.SubstituteFrom[0].Name != "vars" {
		t.Error("SubstituteFrom not appended")
	}
}

func TestCreateSubstituteReference(t *testing.T) {
	ref := CreateSubstituteReference("ConfigMap", "my-vars", true)
	if ref.Kind != "ConfigMap" || ref.Name != "my-vars" || !ref.Optional {
		t.Errorf("unexpected SubstituteReference: %+v", ref)
	}
}

func TestCreateDecryption(t *testing.T) {
	secret := &meta.LocalObjectReference{Name: "age-key"}
	d := CreateDecryption("sops", secret)
	if d == nil || d.Provider != "sops" || d.SecretRef != secret {
		t.Errorf("unexpected Decryption: %+v", d)
	}
}

func TestCreateCommonMetadata(t *testing.T) {
	cm := CreateCommonMetadata()
	if cm == nil {
		t.Fatal("expected non-nil CommonMetadata")
	}
	if cm.Labels == nil || cm.Annotations == nil {
		t.Error("expected initialized maps")
	}
}

func TestAddCommonMetadataLabel(t *testing.T) {
	cm := CreateCommonMetadata()
	AddCommonMetadataLabel(cm, "env", "prod")
	if cm.Labels["env"] != "prod" {
		t.Error("label not set")
	}
}

func TestAddCommonMetadataLabel_NilMap(t *testing.T) {
	cm := &kustv1.CommonMetadata{}
	AddCommonMetadataLabel(cm, "env", "prod")
	if cm.Labels["env"] != "prod" {
		t.Error("label not set after nil init")
	}
}

func TestAddCommonMetadataAnnotation(t *testing.T) {
	cm := CreateCommonMetadata()
	AddCommonMetadataAnnotation(cm, "note", "value")
	if cm.Annotations["note"] != "value" {
		t.Error("annotation not set")
	}
}

func TestAddCommonMetadataAnnotation_NilMap(t *testing.T) {
	cm := &kustv1.CommonMetadata{}
	AddCommonMetadataAnnotation(cm, "note", "value")
	if cm.Annotations["note"] != "value" {
		t.Error("annotation not set after nil init")
	}
}

func TestSetKustomizationIgnoreMissingComponents(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	SetKustomizationIgnoreMissingComponents(obj, true)
	if !obj.Spec.IgnoreMissingComponents {
		t.Error("expected IgnoreMissingComponents to be true")
	}
	SetKustomizationIgnoreMissingComponents(obj, false)
	if obj.Spec.IgnoreMissingComponents {
		t.Error("expected IgnoreMissingComponents to be false")
	}
}

func TestAddKustomizationHealthCheckExpr(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	chk := CreateCustomHealthCheck("example.io/v1", "MyApp", "status.ready == true")
	AddKustomizationHealthCheckExpr(obj, chk)
	if len(obj.Spec.HealthCheckExprs) != 1 {
		t.Fatalf("expected 1 health check expr, got %d", len(obj.Spec.HealthCheckExprs))
	}
	if obj.Spec.HealthCheckExprs[0].APIVersion != "example.io/v1" {
		t.Errorf("got APIVersion %q", obj.Spec.HealthCheckExprs[0].APIVersion)
	}
	if obj.Spec.HealthCheckExprs[0].Current != "status.ready == true" {
		t.Errorf("got Current %q", obj.Spec.HealthCheckExprs[0].Current)
	}
}

func TestCreateCustomHealthCheck(t *testing.T) {
	chk := CreateCustomHealthCheck("apps/v1", "Deployment", "status.availableReplicas > 0")
	if chk.APIVersion != "apps/v1" {
		t.Errorf("got APIVersion %q", chk.APIVersion)
	}
	if chk.Kind != "Deployment" {
		t.Errorf("got Kind %q", chk.Kind)
	}
	if chk.HealthCheckExpressions.Current != "status.availableReplicas > 0" {
		t.Errorf("got Current %q", chk.HealthCheckExpressions.Current)
	}
}

func TestSetCustomHealthCheckInProgress(t *testing.T) {
	chk := CreateCustomHealthCheck("apps/v1", "Deployment", "status.ready")
	SetCustomHealthCheckInProgress(&chk, "status.observedGeneration < status.generation")
	if chk.HealthCheckExpressions.InProgress != "status.observedGeneration < status.generation" {
		t.Errorf("got InProgress %q", chk.HealthCheckExpressions.InProgress)
	}
}

func TestSetCustomHealthCheckFailed(t *testing.T) {
	chk := CreateCustomHealthCheck("apps/v1", "Deployment", "status.ready")
	SetCustomHealthCheckFailed(&chk, "status.conditions.filter(c, c.type == 'Failed').size() > 0")
	if chk.HealthCheckExpressions.Failed != "status.conditions.filter(c, c.type == 'Failed').size() > 0" {
		t.Errorf("got Failed %q", chk.HealthCheckExpressions.Failed)
	}
}

// HelmRelease setters

func TestAddHelmReleaseLabel(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	AddHelmReleaseLabel(obj, "env", "prod")
	if obj.Labels["env"] != "prod" {
		t.Error("label not set")
	}
}

func TestAddHelmReleaseLabel_NilMap(t *testing.T) {
	obj := &helmv2.HelmRelease{}
	AddHelmReleaseLabel(obj, "env", "prod")
	if obj.Labels["env"] != "prod" {
		t.Error("label not set after nil init")
	}
}

func TestAddHelmReleaseAnnotation(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	AddHelmReleaseAnnotation(obj, "note", "value")
	if obj.Annotations["note"] != "value" {
		t.Error("annotation not set")
	}
}

func TestAddHelmReleaseAnnotation_NilMap(t *testing.T) {
	obj := &helmv2.HelmRelease{}
	AddHelmReleaseAnnotation(obj, "note", "value")
	if obj.Annotations["note"] != "value" {
		t.Error("annotation not set after nil init")
	}
}

func TestSetHelmReleaseChart(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	chart := &helmv2.HelmChartTemplate{Spec: helmv2.HelmChartTemplateSpec{Chart: "nginx"}}
	SetHelmReleaseChart(obj, chart)
	if obj.Spec.Chart != chart {
		t.Error("Chart not set")
	}
}

func TestSetHelmReleaseChartRef(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	ref := &helmv2.CrossNamespaceSourceReference{Kind: "HelmChart", Name: "my-chart"}
	SetHelmReleaseChartRef(obj, ref)
	if obj.Spec.ChartRef != ref {
		t.Error("ChartRef not set")
	}
}

func TestSetHelmReleaseInterval(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	d := metav1.Duration{Duration: 10 * time.Minute}
	SetHelmReleaseInterval(obj, d)
	if obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestSetHelmReleaseKubeConfig(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	secretRef := &meta.SecretKeyReference{Name: "kube-cfg"}
	cfg := &meta.KubeConfigReference{SecretRef: secretRef}
	SetHelmReleaseKubeConfig(obj, cfg)
	if obj.Spec.KubeConfig != cfg {
		t.Error("KubeConfig not set")
	}
}

func TestSetHelmReleaseSuspend(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseSuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

func TestSetHelmReleaseReleaseName(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseReleaseName(obj, "my-release")
	if obj.Spec.ReleaseName != "my-release" {
		t.Errorf("got ReleaseName %q", obj.Spec.ReleaseName)
	}
}

func TestSetHelmReleaseTargetNamespace(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseTargetNamespace(obj, "production")
	if obj.Spec.TargetNamespace != "production" {
		t.Errorf("got TargetNamespace %q", obj.Spec.TargetNamespace)
	}
}

func TestSetHelmReleaseStorageNamespace(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseStorageNamespace(obj, "storage-ns")
	if obj.Spec.StorageNamespace != "storage-ns" {
		t.Errorf("got StorageNamespace %q", obj.Spec.StorageNamespace)
	}
}

func TestAddHelmReleaseDependsOn(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	ref := helmv2.DependencyReference{Name: "infra"}
	AddHelmReleaseDependsOn(obj, ref)
	if len(obj.Spec.DependsOn) != 1 || obj.Spec.DependsOn[0].Name != "infra" {
		t.Error("DependsOn not appended")
	}
}

func TestSetHelmReleaseTimeout(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetHelmReleaseTimeout(obj, d)
	if obj.Spec.Timeout == nil || *obj.Spec.Timeout != d {
		t.Error("Timeout not set")
	}
}

func TestSetHelmReleaseMaxHistory(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseMaxHistory(obj, 5)
	if obj.Spec.MaxHistory == nil || *obj.Spec.MaxHistory != 5 {
		t.Error("MaxHistory not set")
	}
}

func TestSetHelmReleaseServiceAccountName(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseServiceAccountName(obj, "flux-sa")
	if obj.Spec.ServiceAccountName != "flux-sa" {
		t.Errorf("got ServiceAccountName %q", obj.Spec.ServiceAccountName)
	}
}

func TestSetHelmReleasePersistentClient(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleasePersistentClient(obj, true)
	if obj.Spec.PersistentClient == nil || !*obj.Spec.PersistentClient {
		t.Error("PersistentClient not set")
	}
}

func TestSetHelmReleaseDriftDetection(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	dd := CreateDriftDetection(helmv2.DriftDetectionEnabled)
	SetHelmReleaseDriftDetection(obj, dd)
	if obj.Spec.DriftDetection != dd {
		t.Error("DriftDetection not set")
	}
}

func TestCreateDriftDetection(t *testing.T) {
	dd := CreateDriftDetection(helmv2.DriftDetectionEnabled)
	if dd == nil || dd.Mode != helmv2.DriftDetectionEnabled {
		t.Errorf("unexpected DriftDetection: %+v", dd)
	}
}

func TestAddDriftDetectionIgnoreRule(t *testing.T) {
	dd := CreateDriftDetection(helmv2.DriftDetectionEnabled)
	rule := CreateIgnoreRule([]string{"/spec/replicas"}, nil)
	AddDriftDetectionIgnoreRule(dd, rule)
	if len(dd.Ignore) != 1 {
		t.Fatalf("expected 1 ignore rule, got %d", len(dd.Ignore))
	}
	if dd.Ignore[0].Paths[0] != "/spec/replicas" {
		t.Error("IgnoreRule paths not set")
	}
}

func TestCreateIgnoreRule(t *testing.T) {
	rule := CreateIgnoreRule([]string{"/metadata/annotations"}, nil)
	if len(rule.Paths) != 1 || rule.Paths[0] != "/metadata/annotations" {
		t.Error("IgnoreRule paths not set")
	}
}

func TestSetHelmReleaseInstall(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	install := &helmv2.Install{CreateNamespace: true}
	SetHelmReleaseInstall(obj, install)
	if obj.Spec.Install != install {
		t.Error("Install not set")
	}
}

func TestSetHelmReleaseUpgrade(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	upgrade := &helmv2.Upgrade{CleanupOnFail: true}
	SetHelmReleaseUpgrade(obj, upgrade)
	if obj.Spec.Upgrade != upgrade {
		t.Error("Upgrade not set")
	}
}

func TestSetHelmReleaseRollback(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	rollback := &helmv2.Rollback{CleanupOnFail: true}
	SetHelmReleaseRollback(obj, rollback)
	if obj.Spec.Rollback != rollback {
		t.Error("Rollback not set")
	}
}

func TestSetHelmReleaseUninstall(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	uninstall := &helmv2.Uninstall{KeepHistory: true}
	SetHelmReleaseUninstall(obj, uninstall)
	if obj.Spec.Uninstall != uninstall {
		t.Error("Uninstall not set")
	}
}

func TestSetHelmReleaseTest(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	test := &helmv2.Test{Enable: true}
	SetHelmReleaseTest(obj, test)
	if obj.Spec.Test != test {
		t.Error("Test not set")
	}
}

func TestAddHelmReleaseValuesFrom(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	vf := helmv2.ValuesReference{Kind: "ConfigMap", Name: "values"}
	AddHelmReleaseValuesFrom(obj, vf)
	if len(obj.Spec.ValuesFrom) != 1 || obj.Spec.ValuesFrom[0].Name != "values" {
		t.Error("ValuesFrom not appended")
	}
}

func TestSetHelmReleaseValues(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	values := &apiextensionsv1.JSON{Raw: []byte(`{"replicas":2}`)}
	SetHelmReleaseValues(obj, values)
	if obj.Spec.Values != values {
		t.Error("Values not set")
	}
}

func TestAddHelmReleasePostRenderer(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	k := CreatePostRendererKustomize()
	pr := helmv2.PostRenderer{Kustomize: k}
	AddHelmReleasePostRenderer(obj, pr)
	if len(obj.Spec.PostRenderers) != 1 {
		t.Fatalf("expected 1 post renderer, got %d", len(obj.Spec.PostRenderers))
	}
}

func TestCreatePostRendererKustomize(t *testing.T) {
	k := CreatePostRendererKustomize()
	if k == nil {
		t.Fatal("expected non-nil Kustomize")
	}
}

func TestAddPostRendererKustomizePatch(t *testing.T) {
	k := CreatePostRendererKustomize()
	patch := kustomize.Patch{Patch: `[{"op":"add","path":"/metadata/labels/env","value":"prod"}]`}
	AddPostRendererKustomizePatch(k, patch)
	if len(k.Patches) != 1 {
		t.Error("Patch not appended to Kustomize post renderer")
	}
}

func TestAddPostRendererKustomizeImage(t *testing.T) {
	k := CreatePostRendererKustomize()
	img := kustomize.Image{Name: "nginx", NewTag: "1.25"}
	AddPostRendererKustomizeImage(k, img)
	if len(k.Images) != 1 || k.Images[0].Name != "nginx" {
		t.Error("Image not appended to Kustomize post renderer")
	}
}

func TestSetHelmReleaseInstallRemediation(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	r := CreateInstallRemediation(3)
	SetHelmReleaseInstallRemediation(obj, r)
	if obj.Spec.Install == nil || obj.Spec.Install.Remediation != r {
		t.Error("InstallRemediation not set")
	}
}

func TestSetHelmReleaseInstallRemediation_CreatesInstall(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	if obj.Spec.Install != nil {
		t.Fatal("expected nil Install initially")
	}
	r := CreateInstallRemediation(2)
	SetHelmReleaseInstallRemediation(obj, r)
	if obj.Spec.Install == nil {
		t.Fatal("Install should have been created")
	}
	if obj.Spec.Install.Remediation != r {
		t.Error("Remediation not set")
	}
}

func TestSetHelmReleaseUpgradeRemediation(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	r := CreateUpgradeRemediation(3)
	SetHelmReleaseUpgradeRemediation(obj, r)
	if obj.Spec.Upgrade == nil || obj.Spec.Upgrade.Remediation != r {
		t.Error("UpgradeRemediation not set")
	}
}

func TestSetHelmReleaseUpgradeRemediation_CreatesUpgrade(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	if obj.Spec.Upgrade != nil {
		t.Fatal("expected nil Upgrade initially")
	}
	r := CreateUpgradeRemediation(1)
	SetHelmReleaseUpgradeRemediation(obj, r)
	if obj.Spec.Upgrade == nil {
		t.Fatal("Upgrade should have been created")
	}
	if obj.Spec.Upgrade.Remediation != r {
		t.Error("Remediation not set")
	}
}

func TestCreateInstallRemediation(t *testing.T) {
	r := CreateInstallRemediation(5)
	if r == nil || r.Retries != 5 {
		t.Errorf("unexpected InstallRemediation: %+v", r)
	}
}

func TestCreateUpgradeRemediation(t *testing.T) {
	r := CreateUpgradeRemediation(3)
	if r == nil || r.Retries != 3 {
		t.Errorf("unexpected UpgradeRemediation: %+v", r)
	}
}

func TestSetInstallRemediationIgnoreTestFailures(t *testing.T) {
	r := CreateInstallRemediation(2)
	SetInstallRemediationIgnoreTestFailures(r, true)
	if r.IgnoreTestFailures == nil || !*r.IgnoreTestFailures {
		t.Error("IgnoreTestFailures not set")
	}
}

func TestSetInstallRemediationRemediateLastFailure(t *testing.T) {
	r := CreateInstallRemediation(2)
	SetInstallRemediationRemediateLastFailure(r, true)
	if r.RemediateLastFailure == nil || !*r.RemediateLastFailure {
		t.Error("RemediateLastFailure not set")
	}
}

func TestSetUpgradeRemediationIgnoreTestFailures(t *testing.T) {
	r := CreateUpgradeRemediation(2)
	SetUpgradeRemediationIgnoreTestFailures(r, true)
	if r.IgnoreTestFailures == nil || !*r.IgnoreTestFailures {
		t.Error("IgnoreTestFailures not set")
	}
}

func TestSetUpgradeRemediationRemediateLastFailure(t *testing.T) {
	r := CreateUpgradeRemediation(2)
	SetUpgradeRemediationRemediateLastFailure(r, true)
	if r.RemediateLastFailure == nil || !*r.RemediateLastFailure {
		t.Error("RemediateLastFailure not set")
	}
}

func TestSetUpgradeRemediationStrategy(t *testing.T) {
	r := CreateUpgradeRemediation(2)
	SetUpgradeRemediationStrategy(r, helmv2.RollbackRemediationStrategy)
	if r.Strategy == nil || *r.Strategy != helmv2.RollbackRemediationStrategy {
		t.Error("Strategy not set")
	}
}

func TestSetHelmReleaseWaitStrategy(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	ws := CreateWaitStrategy(helmv2.WaitStrategyName("Ready"))
	SetHelmReleaseWaitStrategy(obj, ws)
	if obj.Spec.WaitStrategy != ws {
		t.Error("WaitStrategy not set")
	}
}

func TestCreateWaitStrategy(t *testing.T) {
	ws := CreateWaitStrategy(helmv2.WaitStrategyName("Ready"))
	if ws == nil {
		t.Fatal("expected non-nil WaitStrategy")
	}
}

func TestSetHelmReleaseCommonMetadata(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	cm := &helmv2.CommonMetadata{Labels: map[string]string{"env": "prod"}}
	SetHelmReleaseCommonMetadata(obj, cm)
	if obj.Spec.CommonMetadata != cm {
		t.Error("CommonMetadata not set")
	}
}

func TestAddHelmReleaseHealthCheckExpr(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	chk := CreateCustomHealthCheck("apps/v1", "Deployment", "status.ready")
	AddHelmReleaseHealthCheckExpr(obj, chk)
	if len(obj.Spec.HealthCheckExprs) != 1 {
		t.Fatalf("expected 1 health check expr, got %d", len(obj.Spec.HealthCheckExprs))
	}
}

func TestSetHelmReleaseInstallTimeout(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	d := &metav1.Duration{Duration: 5 * 60 * 1e9}
	SetHelmReleaseInstallTimeout(obj, d)
	if obj.Spec.Install == nil || obj.Spec.Install.Timeout != d {
		t.Error("Install.Timeout not set")
	}
}

func TestSetHelmReleaseInstallTimeout_CreatesInstall(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	if obj.Spec.Install != nil {
		t.Fatal("expected nil Install before setter")
	}
	d := &metav1.Duration{Duration: 1e9}
	SetHelmReleaseInstallTimeout(obj, d)
	if obj.Spec.Install == nil {
		t.Fatal("expected Install to be created")
	}
}

func TestSetHelmReleaseInstallCRDs(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseInstallCRDs(obj, helmv2.Create)
	if obj.Spec.Install == nil || obj.Spec.Install.CRDs != helmv2.Create {
		t.Errorf("got Install.CRDs %q", obj.Spec.Install.CRDs)
	}
}

func TestSetHelmReleaseInstallCreateNamespace(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseInstallCreateNamespace(obj, true)
	if !obj.Spec.Install.CreateNamespace {
		t.Error("expected Install.CreateNamespace true")
	}
}

func TestSetHelmReleaseInstallDisableSchemaValidation(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseInstallDisableSchemaValidation(obj, true)
	if !obj.Spec.Install.DisableSchemaValidation {
		t.Error("expected Install.DisableSchemaValidation true")
	}
}

func TestSetHelmReleaseInstallDisableOpenAPIValidation(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseInstallDisableOpenAPIValidation(obj, true)
	if !obj.Spec.Install.DisableOpenAPIValidation {
		t.Error("expected Install.DisableOpenAPIValidation true")
	}
}

func TestSetHelmReleaseInstallDisableHooks(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseInstallDisableHooks(obj, true)
	if !obj.Spec.Install.DisableHooks {
		t.Error("expected Install.DisableHooks true")
	}
}

func TestSetHelmReleaseInstallDisableWait(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseInstallDisableWait(obj, true)
	if !obj.Spec.Install.DisableWait {
		t.Error("expected Install.DisableWait true")
	}
}

func TestSetHelmReleaseInstallDisableWaitForJobs(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseInstallDisableWaitForJobs(obj, true)
	if !obj.Spec.Install.DisableWaitForJobs {
		t.Error("expected Install.DisableWaitForJobs true")
	}
}

func TestSetHelmReleaseInstallDisableTakeOwnership(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseInstallDisableTakeOwnership(obj, true)
	if !obj.Spec.Install.DisableTakeOwnership {
		t.Error("expected Install.DisableTakeOwnership true")
	}
}

func TestSetHelmReleaseInstallReplace(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseInstallReplace(obj, true)
	if !obj.Spec.Install.Replace {
		t.Error("expected Install.Replace true")
	}
}

func TestSetHelmReleaseUpgradeTimeout(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	d := &metav1.Duration{Duration: 10 * 60 * 1e9}
	SetHelmReleaseUpgradeTimeout(obj, d)
	if obj.Spec.Upgrade == nil || obj.Spec.Upgrade.Timeout != d {
		t.Error("Upgrade.Timeout not set")
	}
}

func TestSetHelmReleaseUpgradeTimeout_CreatesUpgrade(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	if obj.Spec.Upgrade != nil {
		t.Fatal("expected nil Upgrade before setter")
	}
	SetHelmReleaseUpgradeTimeout(obj, &metav1.Duration{Duration: 1e9})
	if obj.Spec.Upgrade == nil {
		t.Fatal("expected Upgrade to be created")
	}
}

func TestSetHelmReleaseUpgradeCRDs(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradeCRDs(obj, helmv2.CreateReplace)
	if obj.Spec.Upgrade == nil || obj.Spec.Upgrade.CRDs != helmv2.CreateReplace {
		t.Errorf("got Upgrade.CRDs %q", obj.Spec.Upgrade.CRDs)
	}
}

func TestSetHelmReleaseUpgradeDisableSchemaValidation(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradeDisableSchemaValidation(obj, true)
	if !obj.Spec.Upgrade.DisableSchemaValidation {
		t.Error("expected Upgrade.DisableSchemaValidation true")
	}
}

func TestSetHelmReleaseUpgradeDisableOpenAPIValidation(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradeDisableOpenAPIValidation(obj, true)
	if !obj.Spec.Upgrade.DisableOpenAPIValidation {
		t.Error("expected Upgrade.DisableOpenAPIValidation true")
	}
}

func TestSetHelmReleaseUpgradeDisableHooks(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradeDisableHooks(obj, true)
	if !obj.Spec.Upgrade.DisableHooks {
		t.Error("expected Upgrade.DisableHooks true")
	}
}

func TestSetHelmReleaseUpgradeDisableWait(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradeDisableWait(obj, true)
	if !obj.Spec.Upgrade.DisableWait {
		t.Error("expected Upgrade.DisableWait true")
	}
}

func TestSetHelmReleaseUpgradeDisableWaitForJobs(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradeDisableWaitForJobs(obj, true)
	if !obj.Spec.Upgrade.DisableWaitForJobs {
		t.Error("expected Upgrade.DisableWaitForJobs true")
	}
}

func TestSetHelmReleaseUpgradeDisableTakeOwnership(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradeDisableTakeOwnership(obj, true)
	if !obj.Spec.Upgrade.DisableTakeOwnership {
		t.Error("expected Upgrade.DisableTakeOwnership true")
	}
}

func TestSetHelmReleaseUpgradeForce(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradeForce(obj, true)
	if !obj.Spec.Upgrade.Force {
		t.Error("expected Upgrade.Force true")
	}
}

func TestSetHelmReleaseUpgradePreserveValues(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradePreserveValues(obj, true)
	if !obj.Spec.Upgrade.PreserveValues {
		t.Error("expected Upgrade.PreserveValues true")
	}
}

func TestSetHelmReleaseUpgradeCleanupOnFail(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	SetHelmReleaseUpgradeCleanupOnFail(obj, true)
	if !obj.Spec.Upgrade.CleanupOnFail {
		t.Error("expected Upgrade.CleanupOnFail true")
	}
}

// Provider setters

func TestSetProviderType(t *testing.T) {
	obj := CreateProvider("slack", "ns")
	SetProviderType(obj, "slack")
	if obj.Spec.Type != "slack" {
		t.Errorf("got Type %q", obj.Spec.Type)
	}
}

func TestSetProviderInterval(t *testing.T) {
	obj := CreateProvider("slack", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetProviderInterval(obj, d)
	if obj.Spec.Interval == nil || *obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestSetProviderChannel(t *testing.T) {
	obj := CreateProvider("slack", "ns")
	SetProviderChannel(obj, "#alerts")
	if obj.Spec.Channel != "#alerts" {
		t.Errorf("got Channel %q", obj.Spec.Channel)
	}
}

func TestSetProviderUsername(t *testing.T) {
	obj := CreateProvider("slack", "ns")
	SetProviderUsername(obj, "fluxbot")
	if obj.Spec.Username != "fluxbot" {
		t.Errorf("got Username %q", obj.Spec.Username)
	}
}

func TestSetProviderAddress(t *testing.T) {
	obj := CreateProvider("generic", "ns")
	SetProviderAddress(obj, "https://hook.example.com/webhook")
	if obj.Spec.Address != "https://hook.example.com/webhook" {
		t.Errorf("got Address %q", obj.Spec.Address)
	}
}

func TestSetProviderTimeout(t *testing.T) {
	obj := CreateProvider("slack", "ns")
	d := metav1.Duration{Duration: 30 * time.Second}
	SetProviderTimeout(obj, d)
	if obj.Spec.Timeout == nil || *obj.Spec.Timeout != d {
		t.Error("Timeout not set")
	}
}

func TestSetProviderProxy(t *testing.T) {
	obj := CreateProvider("slack", "ns")
	SetProviderProxy(obj, "https://proxy.example.com:8080")
	if obj.Spec.Proxy != "https://proxy.example.com:8080" {
		t.Errorf("got Proxy %q", obj.Spec.Proxy)
	}
}

func TestSetProviderSecretRef(t *testing.T) {
	obj := CreateProvider("slack", "ns")
	ref := &meta.LocalObjectReference{Name: "slack-token"}
	SetProviderSecretRef(obj, ref)
	if obj.Spec.SecretRef != ref {
		t.Error("SecretRef not set")
	}
}

func TestSetProviderCertSecretRef(t *testing.T) {
	obj := CreateProvider("generic", "ns")
	ref := &meta.LocalObjectReference{Name: "ca-cert"}
	SetProviderCertSecretRef(obj, ref)
	if obj.Spec.CertSecretRef != ref {
		t.Error("CertSecretRef not set")
	}
}

func TestSetProviderSuspend(t *testing.T) {
	obj := CreateProvider("slack", "ns")
	SetProviderSuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

// Alert setters

func TestSetAlertProviderRef(t *testing.T) {
	obj := CreateAlert("alert", "ns")
	ref := meta.LocalObjectReference{Name: "slack-provider"}
	SetAlertProviderRef(obj, ref)
	if obj.Spec.ProviderRef.Name != "slack-provider" {
		t.Errorf("got ProviderRef.Name %q", obj.Spec.ProviderRef.Name)
	}
}

func TestAddAlertEventSource(t *testing.T) {
	obj := CreateAlert("alert", "ns")
	src := notificationv1.CrossNamespaceObjectReference{Kind: "Kustomization", Name: "app"}
	AddAlertEventSource(obj, src)
	if len(obj.Spec.EventSources) != 1 || obj.Spec.EventSources[0].Name != "app" {
		t.Error("EventSource not appended")
	}
}

func TestAddAlertInclusion(t *testing.T) {
	obj := CreateAlert("alert", "ns")
	AddAlertInclusion(obj, ".*error.*")
	if len(obj.Spec.InclusionList) != 1 || obj.Spec.InclusionList[0] != ".*error.*" {
		t.Error("InclusionList not appended")
	}
}

func TestAddAlertExclusion(t *testing.T) {
	obj := CreateAlert("alert", "ns")
	AddAlertExclusion(obj, ".*debug.*")
	if len(obj.Spec.ExclusionList) != 1 || obj.Spec.ExclusionList[0] != ".*debug.*" {
		t.Error("ExclusionList not appended")
	}
}

func TestAddAlertEventMetadata(t *testing.T) {
	obj := CreateAlert("alert", "ns")
	AddAlertEventMetadata(obj, "cluster", "prod-eu")
	if obj.Spec.EventMetadata["cluster"] != "prod-eu" {
		t.Error("EventMetadata not set")
	}
}

func TestAddAlertEventMetadata_NilMap(t *testing.T) {
	obj := &notificationv1beta3.Alert{}
	AddAlertEventMetadata(obj, "cluster", "prod-eu")
	if obj.Spec.EventMetadata["cluster"] != "prod-eu" {
		t.Error("EventMetadata not set after nil init")
	}
}

func TestSetAlertEventSeverity(t *testing.T) {
	obj := CreateAlert("alert", "ns")
	SetAlertEventSeverity(obj, "error")
	if obj.Spec.EventSeverity != "error" {
		t.Errorf("got EventSeverity %q", obj.Spec.EventSeverity)
	}
}

func TestSetAlertSummary(t *testing.T) {
	obj := CreateAlert("alert", "ns")
	SetAlertSummary(obj, "Flux reconciliation error")
	if obj.Spec.Summary != "Flux reconciliation error" {
		t.Errorf("got Summary %q", obj.Spec.Summary)
	}
}

func TestSetAlertSuspend(t *testing.T) {
	obj := CreateAlert("alert", "ns")
	SetAlertSuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

// Receiver setters

func TestSetReceiverType(t *testing.T) {
	obj := CreateReceiver("receiver", "ns")
	SetReceiverType(obj, "github")
	if obj.Spec.Type != "github" {
		t.Errorf("got Type %q", obj.Spec.Type)
	}
}

func TestSetReceiverInterval(t *testing.T) {
	obj := CreateReceiver("receiver", "ns")
	d := metav1.Duration{Duration: 1 * time.Hour}
	SetReceiverInterval(obj, d)
	if obj.Spec.Interval == nil || *obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestAddReceiverEvent(t *testing.T) {
	obj := CreateReceiver("receiver", "ns")
	AddReceiverEvent(obj, "ping")
	if len(obj.Spec.Events) != 1 || obj.Spec.Events[0] != "ping" {
		t.Error("Event not appended")
	}
}

func TestAddReceiverResource(t *testing.T) {
	obj := CreateReceiver("receiver", "ns")
	ref := notificationv1.CrossNamespaceObjectReference{Kind: "GitRepository", Name: "app-repo"}
	AddReceiverResource(obj, ref)
	if len(obj.Spec.Resources) != 1 || obj.Spec.Resources[0].Name != "app-repo" {
		t.Error("Resource not appended")
	}
}

func TestSetReceiverSecretRef(t *testing.T) {
	obj := CreateReceiver("receiver", "ns")
	ref := meta.LocalObjectReference{Name: "webhook-token"}
	SetReceiverSecretRef(obj, ref)
	if obj.Spec.SecretRef.Name != "webhook-token" {
		t.Errorf("got SecretRef.Name %q", obj.Spec.SecretRef.Name)
	}
}

func TestSetReceiverSuspend(t *testing.T) {
	obj := CreateReceiver("receiver", "ns")
	SetReceiverSuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

// ImageUpdateAutomation setters

func TestSetImageUpdateAutomationSourceRef(t *testing.T) {
	obj := CreateImageUpdateAutomation("iua", "ns")
	ref := CreateCrossNamespaceSourceReference("source.toolkit.fluxcd.io/v1", "GitRepository", "app-repo", "flux-system")
	SetImageUpdateAutomationSourceRef(obj, ref)
	if obj.Spec.SourceRef.Name != "app-repo" {
		t.Errorf("got SourceRef.Name %q", obj.Spec.SourceRef.Name)
	}
}

func TestSetImageUpdateAutomationGitSpec(t *testing.T) {
	obj := CreateImageUpdateAutomation("iua", "ns")
	author := CreateCommitUser("Flux", "flux@example.com")
	commitSpec := CreateCommitSpec(author)
	gs := CreateGitSpec(commitSpec, nil, nil)
	SetImageUpdateAutomationGitSpec(obj, gs)
	if obj.Spec.GitSpec != gs {
		t.Error("GitSpec not set")
	}
}

func TestSetImageUpdateAutomationInterval(t *testing.T) {
	obj := CreateImageUpdateAutomation("iua", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetImageUpdateAutomationInterval(obj, d)
	if obj.Spec.Interval != d {
		t.Error("Interval not set")
	}
}

func TestSetImageUpdateAutomationPolicySelector(t *testing.T) {
	obj := CreateImageUpdateAutomation("iua", "ns")
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "myapp"}}
	SetImageUpdateAutomationPolicySelector(obj, sel)
	if obj.Spec.PolicySelector != sel {
		t.Error("PolicySelector not set")
	}
}

func TestSetImageUpdateAutomationUpdateStrategy(t *testing.T) {
	obj := CreateImageUpdateAutomation("iua", "ns")
	strategy := CreateUpdateStrategy("Setters", "./")
	SetImageUpdateAutomationUpdateStrategy(obj, strategy)
	if obj.Spec.Update != strategy {
		t.Error("UpdateStrategy not set")
	}
}

func TestSetImageUpdateAutomationSuspend(t *testing.T) {
	obj := CreateImageUpdateAutomation("iua", "ns")
	SetImageUpdateAutomationSuspend(obj, true)
	if !obj.Spec.Suspend {
		t.Error("Suspend not set")
	}
}

func TestCreateCrossNamespaceSourceReference(t *testing.T) {
	ref := CreateCrossNamespaceSourceReference("v1", "GitRepository", "repo", "flux-system")
	if ref.APIVersion != "v1" || ref.Kind != "GitRepository" || ref.Name != "repo" || ref.Namespace != "flux-system" {
		t.Errorf("unexpected CrossNamespaceSourceReference: %+v", ref)
	}
}

func TestCreateGitCheckoutSpec(t *testing.T) {
	gitRef := sourcev1.GitRepositoryRef{Branch: "main"}
	spec := CreateGitCheckoutSpec(gitRef)
	if spec == nil || spec.Reference.Branch != "main" {
		t.Errorf("unexpected GitCheckoutSpec: %+v", spec)
	}
}

func TestSetGitCheckoutReference(t *testing.T) {
	gitRef := sourcev1.GitRepositoryRef{Branch: "main"}
	spec := CreateGitCheckoutSpec(gitRef)
	newRef := sourcev1.GitRepositoryRef{Tag: "v1.0.0"}
	SetGitCheckoutReference(spec, newRef)
	if spec.Reference.Tag != "v1.0.0" {
		t.Errorf("got Reference.Tag %q", spec.Reference.Tag)
	}
}

func TestCreateCommitUser(t *testing.T) {
	u := CreateCommitUser("Flux", "flux@example.com")
	if u.Name != "Flux" || u.Email != "flux@example.com" {
		t.Errorf("unexpected CommitUser: %+v", u)
	}
}

func TestCreateSigningKey(t *testing.T) {
	sk := CreateSigningKey("gpg-key")
	if sk == nil || sk.SecretRef.Name != "gpg-key" {
		t.Errorf("unexpected SigningKey: %+v", sk)
	}
}

func TestCreateCommitSpec(t *testing.T) {
	author := CreateCommitUser("Flux", "flux@example.com")
	spec := CreateCommitSpec(author)
	if spec.Author.Name != "Flux" {
		t.Errorf("unexpected CommitSpec: %+v", spec)
	}
}

func TestSetCommitSigningKey(t *testing.T) {
	author := CreateCommitUser("Flux", "flux@example.com")
	spec := CreateCommitSpec(author)
	sk := CreateSigningKey("gpg-key")
	SetCommitSigningKey(&spec, sk)
	if spec.SigningKey != sk {
		t.Error("SigningKey not set")
	}
}

func TestSetCommitMessageTemplate(t *testing.T) {
	author := CreateCommitUser("Flux", "flux@example.com")
	spec := CreateCommitSpec(author)
	SetCommitMessageTemplate(&spec, "Update {{.AutomationObject}} images")
	if spec.MessageTemplate != "Update {{.AutomationObject}} images" {
		t.Errorf("got MessageTemplate %q", spec.MessageTemplate)
	}
}

func TestSetCommitMessageTemplateValues(t *testing.T) {
	author := CreateCommitUser("Flux", "flux@example.com")
	spec := CreateCommitSpec(author)
	values := map[string]string{"env": "prod"}
	SetCommitMessageTemplateValues(&spec, values)
	if spec.MessageTemplateValues["env"] != "prod" {
		t.Error("MessageTemplateValues not set")
	}
}

func TestAddCommitMessageTemplateValue(t *testing.T) {
	author := CreateCommitUser("Flux", "flux@example.com")
	spec := CreateCommitSpec(author)
	AddCommitMessageTemplateValue(&spec, "env", "prod")
	if spec.MessageTemplateValues["env"] != "prod" {
		t.Error("MessageTemplateValues key not set")
	}
}

func TestAddCommitMessageTemplateValue_NilMap(t *testing.T) {
	spec := imagev1.CommitSpec{}
	AddCommitMessageTemplateValue(&spec, "env", "prod")
	if spec.MessageTemplateValues["env"] != "prod" {
		t.Error("MessageTemplateValues key not set after nil init")
	}
}

func TestSetCommitAuthor(t *testing.T) {
	spec := imagev1.CommitSpec{}
	author := CreateCommitUser("Bot", "bot@example.com")
	SetCommitAuthor(&spec, author)
	if spec.Author.Name != "Bot" {
		t.Errorf("got Author.Name %q", spec.Author.Name)
	}
}

func TestCreatePushSpec(t *testing.T) {
	opts := map[string]string{"force": "true"}
	ps := CreatePushSpec("main", "refs/heads/main", opts)
	if ps == nil || ps.Branch != "main" || ps.Refspec != "refs/heads/main" {
		t.Errorf("unexpected PushSpec: %+v", ps)
	}
	if ps.Options["force"] != "true" {
		t.Error("Options not set")
	}
}

func TestSetPushBranch(t *testing.T) {
	ps := CreatePushSpec("main", "", nil)
	SetPushBranch(ps, "feature")
	if ps.Branch != "feature" {
		t.Errorf("got Branch %q", ps.Branch)
	}
}

func TestSetPushRefspec(t *testing.T) {
	ps := CreatePushSpec("", "", nil)
	SetPushRefspec(ps, "refs/heads/main")
	if ps.Refspec != "refs/heads/main" {
		t.Errorf("got Refspec %q", ps.Refspec)
	}
}

func TestSetPushOptions(t *testing.T) {
	ps := CreatePushSpec("main", "", nil)
	opts := map[string]string{"atomic": "true"}
	SetPushOptions(ps, opts)
	if ps.Options["atomic"] != "true" {
		t.Error("Options not set")
	}
}

func TestAddPushOption(t *testing.T) {
	ps := CreatePushSpec("main", "", nil)
	AddPushOption(ps, "atomic", "true")
	if ps.Options["atomic"] != "true" {
		t.Error("Option not added")
	}
}

func TestAddPushOption_NilMap(t *testing.T) {
	ps := &imagev1.PushSpec{}
	AddPushOption(ps, "force", "true")
	if ps.Options["force"] != "true" {
		t.Error("Option not added after nil init")
	}
}

func TestCreateGitSpec(t *testing.T) {
	author := CreateCommitUser("Flux", "flux@example.com")
	commit := CreateCommitSpec(author)
	checkout := CreateGitCheckoutSpec(sourcev1.GitRepositoryRef{Branch: "main"})
	push := CreatePushSpec("main", "", nil)
	gs := CreateGitSpec(commit, checkout, push)
	if gs == nil {
		t.Fatal("expected non-nil GitSpec")
	}
	if gs.Commit.Author.Name != "Flux" {
		t.Error("GitSpec.Commit not set")
	}
	if gs.Checkout != checkout {
		t.Error("GitSpec.Checkout not set")
	}
	if gs.Push != push {
		t.Error("GitSpec.Push not set")
	}
}

func TestSetGitSpecCheckout(t *testing.T) {
	author := CreateCommitUser("Flux", "flux@example.com")
	commit := CreateCommitSpec(author)
	gs := CreateGitSpec(commit, nil, nil)
	checkout := CreateGitCheckoutSpec(sourcev1.GitRepositoryRef{Branch: "main"})
	SetGitSpecCheckout(gs, checkout)
	if gs.Checkout != checkout {
		t.Error("Checkout not set")
	}
}

func TestSetGitSpecCommit(t *testing.T) {
	author := CreateCommitUser("Flux", "flux@example.com")
	commit := CreateCommitSpec(author)
	gs := CreateGitSpec(commit, nil, nil)
	newAuthor := CreateCommitUser("Bot", "bot@example.com")
	newCommit := CreateCommitSpec(newAuthor)
	SetGitSpecCommit(gs, newCommit)
	if gs.Commit.Author.Name != "Bot" {
		t.Error("Commit not updated")
	}
}

func TestSetGitSpecPush(t *testing.T) {
	author := CreateCommitUser("Flux", "flux@example.com")
	commit := CreateCommitSpec(author)
	gs := CreateGitSpec(commit, nil, nil)
	push := CreatePushSpec("main", "", nil)
	SetGitSpecPush(gs, push)
	if gs.Push != push {
		t.Error("Push not set")
	}
}

func TestCreateUpdateStrategy(t *testing.T) {
	s := CreateUpdateStrategy("Setters", "./")
	if s == nil || string(s.Strategy) != "Setters" || s.Path != "./" {
		t.Errorf("unexpected UpdateStrategy: %+v", s)
	}
}

func TestSetUpdateStrategyName(t *testing.T) {
	s := CreateUpdateStrategy("Setters", "./")
	SetUpdateStrategyName(s, "Regex")
	if string(s.Strategy) != "Regex" {
		t.Errorf("got Strategy %q", s.Strategy)
	}
}

func TestSetUpdateStrategyPath(t *testing.T) {
	s := CreateUpdateStrategy("Setters", "./")
	SetUpdateStrategyPath(s, "./apps")
	if s.Path != "./apps" {
		t.Errorf("got Path %q", s.Path)
	}
}

func TestCreateImageRef(t *testing.T) {
	ref := CreateImageRef("nginx", "1.25", "sha256:abc")
	if ref.Name != "nginx" || ref.Tag != "1.25" || ref.Digest != "sha256:abc" {
		t.Errorf("unexpected ImageRef: %+v", ref)
	}
}

func TestSetImageRefDigest(t *testing.T) {
	ref := CreateImageRef("nginx", "1.25", "")
	SetImageRefDigest(&ref, "sha256:abc")
	if ref.Digest != "sha256:abc" {
		t.Errorf("got Digest %q", ref.Digest)
	}
}

func TestSetImageRefTag(t *testing.T) {
	ref := CreateImageRef("nginx", "", "")
	SetImageRefTag(&ref, "1.25")
	if ref.Tag != "1.25" {
		t.Errorf("got Tag %q", ref.Tag)
	}
}

func TestSetImageRefName(t *testing.T) {
	ref := CreateImageRef("", "", "")
	SetImageRefName(&ref, "nginx")
	if ref.Name != "nginx" {
		t.Errorf("got Name %q", ref.Name)
	}
}

func TestAddObservedPolicy(t *testing.T) {
	obj := CreateImageUpdateAutomation("iua", "ns")
	ref := CreateImageRef("nginx", "1.25", "")
	AddObservedPolicy(obj, "nginx-policy", ref)
	if obj.Status.ObservedPolicies["nginx-policy"].Name != "nginx" {
		t.Error("ObservedPolicy not set")
	}
}

func TestAddObservedPolicy_NilMap(t *testing.T) {
	obj := &imagev1.ImageUpdateAutomation{}
	ref := CreateImageRef("nginx", "1.25", "")
	AddObservedPolicy(obj, "nginx-policy", ref)
	if obj.Status.ObservedPolicies["nginx-policy"].Name != "nginx" {
		t.Error("ObservedPolicy not set after nil init")
	}
}

func TestSetObservedPolicies(t *testing.T) {
	obj := CreateImageUpdateAutomation("iua", "ns")
	policies := imagev1.ObservedPolicies{
		"nginx-policy": CreateImageRef("nginx", "1.25", ""),
	}
	SetObservedPolicies(obj, policies)
	if obj.Status.ObservedPolicies["nginx-policy"].Name != "nginx" {
		t.Error("ObservedPolicies not set")
	}
}

// ResourceSet setters

func TestAddResourceSetInput(t *testing.T) {
	obj := CreateResourceSet("rs", "ns")
	input := fluxv1.ResourceSetInput{"env": {Raw: []byte(`"prod"`)}}
	AddResourceSetInput(obj, input)
	if len(obj.Spec.Inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(obj.Spec.Inputs))
	}
}

func TestAddResourceSetInputFrom(t *testing.T) {
	obj := CreateResourceSet("rs", "ns")
	ref := fluxv1.InputProviderReference{Name: "github-provider"}
	AddResourceSetInputFrom(obj, ref)
	if len(obj.Spec.InputsFrom) != 1 || obj.Spec.InputsFrom[0].Name != "github-provider" {
		t.Error("InputsFrom not appended")
	}
}

func TestAddResourceSetResource(t *testing.T) {
	obj := CreateResourceSet("rs", "ns")
	r := &apiextensionsv1.JSON{Raw: []byte(`{"kind":"ConfigMap"}`)}
	AddResourceSetResource(obj, r)
	if len(obj.Spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(obj.Spec.Resources))
	}
}

func TestSetResourceSetResourcesTemplate(t *testing.T) {
	obj := CreateResourceSet("rs", "ns")
	SetResourceSetResourcesTemplate(obj, "{{ .inputs | toJSON }}")
	if obj.Spec.ResourcesTemplate != "{{ .inputs | toJSON }}" {
		t.Errorf("got ResourcesTemplate %q", obj.Spec.ResourcesTemplate)
	}
}

func TestAddResourceSetDependency(t *testing.T) {
	obj := CreateResourceSet("rs", "ns")
	dep := fluxv1.Dependency{Name: "infra-rs"}
	AddResourceSetDependency(obj, dep)
	if len(obj.Spec.DependsOn) != 1 || obj.Spec.DependsOn[0].Name != "infra-rs" {
		t.Error("Dependency not appended")
	}
}

func TestSetResourceSetServiceAccountName(t *testing.T) {
	obj := CreateResourceSet("rs", "ns")
	SetResourceSetServiceAccountName(obj, "flux-sa")
	if obj.Spec.ServiceAccountName != "flux-sa" {
		t.Errorf("got ServiceAccountName %q", obj.Spec.ServiceAccountName)
	}
}

func TestSetResourceSetWait(t *testing.T) {
	obj := CreateResourceSet("rs", "ns")
	SetResourceSetWait(obj, true)
	if !obj.Spec.Wait {
		t.Error("Wait not set")
	}
}

func TestSetResourceSetCommonMetadata(t *testing.T) {
	obj := CreateResourceSet("rs", "ns")
	cm := &fluxv1.CommonMetadata{Labels: map[string]string{"env": "prod"}}
	SetResourceSetCommonMetadata(obj, cm)
	if obj.Spec.CommonMetadata != cm {
		t.Error("CommonMetadata not set")
	}
}

// ResourceSetInputProvider setters

func TestSetResourceSetInputProviderType(t *testing.T) {
	obj := CreateResourceSetInputProvider("ip", "ns")
	SetResourceSetInputProviderType(obj, "GitHubOrg")
	if obj.Spec.Type != "GitHubOrg" {
		t.Errorf("got Type %q", obj.Spec.Type)
	}
}

func TestSetResourceSetInputProviderURL(t *testing.T) {
	obj := CreateResourceSetInputProvider("ip", "ns")
	SetResourceSetInputProviderURL(obj, "https://api.github.com")
	if obj.Spec.URL != "https://api.github.com" {
		t.Errorf("got URL %q", obj.Spec.URL)
	}
}

func TestSetResourceSetInputProviderServiceAccountName(t *testing.T) {
	obj := CreateResourceSetInputProvider("ip", "ns")
	SetResourceSetInputProviderServiceAccountName(obj, "flux-sa")
	if obj.Spec.ServiceAccountName != "flux-sa" {
		t.Errorf("got ServiceAccountName %q", obj.Spec.ServiceAccountName)
	}
}

func TestSetResourceSetInputProviderSecretRef(t *testing.T) {
	obj := CreateResourceSetInputProvider("ip", "ns")
	ref := &meta.LocalObjectReference{Name: "github-token"}
	SetResourceSetInputProviderSecretRef(obj, ref)
	if obj.Spec.SecretRef != ref {
		t.Error("SecretRef not set")
	}
}

func TestSetResourceSetInputProviderCertSecretRef(t *testing.T) {
	obj := CreateResourceSetInputProvider("ip", "ns")
	ref := &meta.LocalObjectReference{Name: "ca-cert"}
	SetResourceSetInputProviderCertSecretRef(obj, ref)
	if obj.Spec.CertSecretRef != ref {
		t.Error("CertSecretRef not set")
	}
}

func TestAddResourceSetInputProviderSchedule(t *testing.T) {
	obj := CreateResourceSetInputProvider("ip", "ns")
	s := CreateSchedule("0 * * * *")
	AddResourceSetInputProviderSchedule(obj, s)
	if len(obj.Spec.Schedule) != 1 || obj.Spec.Schedule[0].Cron != "0 * * * *" {
		t.Error("Schedule not appended")
	}
}

// FluxInstance setters

func TestAddFluxInstanceComponent(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	c := fluxv1.Component("source-controller")
	AddFluxInstanceComponent(obj, c)
	if len(obj.Spec.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(obj.Spec.Components))
	}
}

func TestSetFluxInstanceDistribution(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	dist := fluxv1.Distribution{Version: "v2.3.0", Registry: "ghcr.io/fluxcd"}
	SetFluxInstanceDistribution(obj, dist)
	if obj.Spec.Distribution.Version != "v2.3.0" {
		t.Errorf("got Distribution.Version %q", obj.Spec.Distribution.Version)
	}
}

func TestSetFluxInstanceDistributionVariant(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	SetFluxInstanceDistributionVariant(obj, "enterprise-distroless")
	if obj.Spec.Distribution.Variant != "enterprise-distroless" {
		t.Errorf("got Variant %q", obj.Spec.Distribution.Variant)
	}
}

func TestSetFluxInstanceCommonMetadata(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	cm := &fluxv1.CommonMetadata{Labels: map[string]string{"managed-by": "flux-operator"}}
	SetFluxInstanceCommonMetadata(obj, cm)
	if obj.Spec.CommonMetadata != cm {
		t.Error("CommonMetadata not set")
	}
}

func TestSetFluxInstanceCluster(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	cluster := &fluxv1.Cluster{Domain: "cluster.local"}
	SetFluxInstanceCluster(obj, cluster)
	if obj.Spec.Cluster != cluster {
		t.Error("Cluster not set")
	}
}

func TestSetFluxInstanceSharding(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	shard := &fluxv1.Sharding{Key: "sharding.fluxcd.io/key"}
	SetFluxInstanceSharding(obj, shard)
	if obj.Spec.Sharding != shard {
		t.Error("Sharding not set")
	}
}

func TestSetFluxInstanceStorage(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	st := &fluxv1.Storage{Class: "standard", Size: "1Gi"}
	SetFluxInstanceStorage(obj, st)
	if obj.Spec.Storage != st {
		t.Error("Storage not set")
	}
}

func TestSetFluxInstanceKustomize(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	k := &fluxv1.Kustomize{}
	SetFluxInstanceKustomize(obj, k)
	if obj.Spec.Kustomize != k {
		t.Error("Kustomize not set")
	}
}

func TestSetFluxInstanceWait(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	SetFluxInstanceWait(obj, true)
	if obj.Spec.Wait == nil || !*obj.Spec.Wait {
		t.Error("Wait not set")
	}
}

func TestSetFluxInstanceMigrateResources(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	SetFluxInstanceMigrateResources(obj, true)
	if obj.Spec.MigrateResources == nil || !*obj.Spec.MigrateResources {
		t.Error("MigrateResources not set")
	}
}

func TestSetFluxInstanceSync(t *testing.T) {
	obj := CreateFluxInstance("flux", "flux-system")
	sync := &fluxv1.Sync{Kind: "GitRepository", Name: "flux-system", URL: "https://github.com/example/fleet"}
	SetFluxInstanceSync(obj, sync)
	if obj.Spec.Sync != sync {
		t.Error("Sync not set")
	}
}

// FluxReport setters

func TestSetFluxReportDistribution(t *testing.T) {
	obj := CreateFluxReport("report", "flux-system")
	dist := fluxv1.FluxDistributionStatus{Entitlement: "oss", Version: "v2.3.0"}
	SetFluxReportDistribution(obj, dist)
	if obj.Spec.Distribution.Version != "v2.3.0" {
		t.Errorf("got Distribution.Version %q", obj.Spec.Distribution.Version)
	}
}

func TestSetFluxReportCluster(t *testing.T) {
	obj := CreateFluxReport("report", "flux-system")
	c := &fluxv1.ClusterInfo{ServerVersion: "1.29"}
	SetFluxReportCluster(obj, c)
	if obj.Spec.Cluster != c {
		t.Error("Cluster not set")
	}
}

func TestSetFluxReportOperator(t *testing.T) {
	obj := CreateFluxReport("report", "flux-system")
	op := &fluxv1.OperatorInfo{APIVersion: "v1"}
	SetFluxReportOperator(obj, op)
	if obj.Spec.Operator != op {
		t.Error("Operator not set")
	}
}

func TestAddFluxReportComponentStatus(t *testing.T) {
	obj := CreateFluxReport("report", "flux-system")
	cs := fluxv1.FluxComponentStatus{Name: "source-controller"}
	AddFluxReportComponentStatus(obj, cs)
	if len(obj.Spec.ComponentsStatus) != 1 || obj.Spec.ComponentsStatus[0].Name != "source-controller" {
		t.Error("ComponentStatus not appended")
	}
}

func TestAddFluxReportReconcilerStatus(t *testing.T) {
	obj := CreateFluxReport("report", "flux-system")
	rs := fluxv1.FluxReconcilerStatus{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "GitRepository"}
	AddFluxReportReconcilerStatus(obj, rs)
	if len(obj.Spec.ReconcilersStatus) != 1 {
		t.Fatalf("expected 1 reconciler status, got %d", len(obj.Spec.ReconcilersStatus))
	}
}

func TestSetFluxReportSyncStatus(t *testing.T) {
	obj := CreateFluxReport("report", "flux-system")
	s := &fluxv1.FluxSyncStatus{ID: "flux-system/flux-system"}
	SetFluxReportSyncStatus(obj, s)
	if obj.Spec.SyncStatus != s {
		t.Error("SyncStatus not set")
	}
}

// Schedule helpers

func TestCreateSchedule(t *testing.T) {
	s := CreateSchedule("0 * * * *")
	if s.Cron != "0 * * * *" {
		t.Errorf("got Cron %q", s.Cron)
	}
}

func TestSetScheduleTimeZone(t *testing.T) {
	s := CreateSchedule("0 * * * *")
	SetScheduleTimeZone(&s, "Europe/Brussels")
	if s.TimeZone != "Europe/Brussels" {
		t.Errorf("got TimeZone %q", s.TimeZone)
	}
}

func TestSetScheduleWindow(t *testing.T) {
	s := CreateSchedule("0 * * * *")
	d := metav1.Duration{Duration: 10 * time.Minute}
	SetScheduleWindow(&s, d)
	if s.Window != d {
		t.Error("Window not set")
	}
}
