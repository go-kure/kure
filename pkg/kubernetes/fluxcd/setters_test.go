package fluxcd

import (
	jsonPkg "encoding/json"
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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// GitRepository setters

func TestSetGitRepositorySecretRef(t *testing.T) {
	obj := CreateGitRepository("repo", "ns")
	ref := &meta.LocalObjectReference{Name: "my-secret"}
	SetGitRepositorySecretRef(obj, ref)
	if obj.Spec.SecretRef != ref {
		t.Error("SecretRef not set")
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

// HelmRepository setters

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

func TestSetHelmRepositoryTimeout(t *testing.T) {
	obj := CreateHelmRepository("repo", "ns")
	d := metav1.Duration{Duration: 60 * time.Second}
	SetHelmRepositoryTimeout(obj, &d)
	if obj.Spec.Timeout == nil || *obj.Spec.Timeout != d {
		t.Error("Timeout not set")
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

func TestHelmRepository_HTTP(t *testing.T) {
	hr := CreateHelmRepository("bitnami", "flux-system")
	hr.Spec.URL = "https://charts.bitnami.com/bitnami"
	hr.Spec.Type = "default"
	hr.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
	hr.Spec.PassCredentials = true
	SetHelmRepositoryTimeout(hr, &metav1.Duration{Duration: 60 * time.Second})
	SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "bitnami-auth"})
	goldenTest(t, "helmrepository_http.yaml", hr)
}

func TestHelmRepository_OCI(t *testing.T) {
	hr := CreateHelmRepository("ghcr-charts", "flux-system")
	hr.Spec.URL = "oci://ghcr.io/example/charts"
	hr.Spec.Type = "oci"
	hr.Spec.Provider = "generic"
	hr.Spec.Insecure = false
	hr.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
	SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "ghcr-auth"})
	goldenTest(t, "helmrepository_oci.yaml", hr)
}

// Bucket setters

func TestSetBucketSTS(t *testing.T) {
	obj := CreateBucket("bucket", "ns")
	sts := &sourcev1.BucketSTSSpec{Provider: "aws"}
	SetBucketSTS(obj, sts)
	if obj.Spec.STS != sts {
		t.Error("STS not set")
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

// HelmChart setters

func TestAddHelmChartValuesFile(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	AddHelmChartValuesFile(obj, "values-prod.yaml")
	if len(obj.Spec.ValuesFiles) != 1 || obj.Spec.ValuesFiles[0] != "values-prod.yaml" {
		t.Error("ValuesFiles not appended")
	}
}

func TestSetHelmChartVerify(t *testing.T) {
	obj := CreateHelmChart("chart", "ns")
	verify := &sourcev1.HelmChartVerification{Provider: "cosign"}
	SetHelmChartVerify(obj, verify)
	if obj.Spec.Verify != verify {
		t.Error("Verify not set")
	}
}

// OCIRepository setters

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

func TestCreateOutputArtifact(t *testing.T) {
	out := CreateOutputArtifact("merged")
	if out.Name != "merged" {
		t.Errorf("got Name %q", out.Name)
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

// Kustomization setters

func TestSetKustomizationRetryInterval(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	d := metav1.Duration{Duration: 2 * time.Minute}
	SetKustomizationRetryInterval(obj, d)
	if obj.Spec.RetryInterval == nil || *obj.Spec.RetryInterval != d {
		t.Error("RetryInterval not set")
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

func TestSetKustomizationTimeout(t *testing.T) {
	obj := CreateKustomization("ks", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetKustomizationTimeout(obj, d)
	if obj.Spec.Timeout == nil || *obj.Spec.Timeout != d {
		t.Error("Timeout not set")
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

// HelmRelease setters

// TestHelmReleaseMetadataViaGenericHelpers covers what AddHelmReleaseLabel and
// AddHelmReleaseAnnotation used to do, including the nil-map path on a
// HelmRelease not built by CreateHelmRelease.
func TestHelmReleaseMetadataViaGenericHelpers(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	kubernetes.AddLabel(obj, "env", "prod")
	kubernetes.AddAnnotation(obj, "note", "value")
	if obj.Labels["env"] != "prod" || obj.Annotations["note"] != "value" {
		t.Errorf("metadata not set: labels=%v annotations=%v", obj.Labels, obj.Annotations)
	}

	bare := &helmv2.HelmRelease{}
	kubernetes.AddLabel(bare, "env", "prod")
	kubernetes.AddAnnotation(bare, "note", "value")
	if bare.Labels["env"] != "prod" || bare.Annotations["note"] != "value" {
		t.Errorf("metadata not set after nil init: labels=%v annotations=%v", bare.Labels, bare.Annotations)
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

func TestSetHelmReleaseKubeConfig(t *testing.T) {
	obj := CreateHelmRelease("hr", "ns")
	secretRef := &meta.SecretKeyReference{Name: "kube-cfg"}
	cfg := &meta.KubeConfigReference{SecretRef: secretRef}
	SetHelmReleaseKubeConfig(obj, cfg)
	if obj.Spec.KubeConfig != cfg {
		t.Error("KubeConfig not set")
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

func TestSetHelmReleaseValuesFromMap(t *testing.T) {
	hr := CreateHelmRelease("redis", "apps")
	SetHelmReleaseValuesFromMap(hr, map[string]any{
		"replicaCount": 3,
		"image":        map[string]any{"tag": "latest"},
	})
	if hr.Spec.Values == nil {
		t.Fatal("values nil")
	}
	var got map[string]any
	_ = jsonPkg.Unmarshal(hr.Spec.Values.Raw, &got)
	if got["replicaCount"] != float64(3) {
		t.Errorf("got %v", got["replicaCount"])
	}
}

func TestHelmRelease_FullSpec(t *testing.T) {
	hr := CreateHelmRelease("redis", "apps")
	hr.Spec.ReleaseName = "redis-prod"
	hr.Spec.TargetNamespace = "apps"
	hr.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
	SetHelmReleaseChart(hr, &helmv2.HelmChartTemplate{
		Spec: helmv2.HelmChartTemplateSpec{
			Chart:   "redis",
			Version: "19.0.0",
			SourceRef: helmv2.CrossNamespaceObjectReference{
				Kind:      "HelmRepository",
				Name:      "bitnami",
				Namespace: "flux-system",
			},
		},
	})
	SetHelmReleaseValuesFromMap(hr, map[string]any{
		"replicaCount": 3,
		"auth":         map[string]any{"enabled": true},
	})
	AddHelmReleaseValuesFrom(hr, helmv2.ValuesReference{
		Kind: "ConfigMap", Name: "redis-defaults",
	})
	SetHelmReleaseDriftDetection(hr, CreateDriftDetection(helmv2.DriftDetectionEnabled))
	SetHelmReleaseCommonMetadata(hr, &helmv2.CommonMetadata{
		Labels: map[string]string{"app": "redis"},
	})
	SetHelmReleaseInstallCRDs(hr, helmv2.CreateReplace)
	SetHelmReleaseInstallRemediation(hr, CreateInstallRemediation(3))
	SetHelmReleaseUpgradeCRDs(hr, helmv2.CreateReplace)
	SetHelmReleaseUpgradeRemediation(hr, CreateUpgradeRemediation(3))
	k := CreatePostRendererKustomize()
	AddPostRendererKustomizeImage(k, kustomize.Image{Name: "redis", NewTag: "7.0"})
	AddHelmReleasePostRenderer(hr, helmv2.PostRenderer{Kustomize: k})
	goldenTest(t, "helmrelease_full_spec.yaml", hr)
}

// Provider setters

func TestSetProviderInterval(t *testing.T) {
	obj := CreateProvider("slack", "ns")
	d := metav1.Duration{Duration: 5 * time.Minute}
	SetProviderInterval(obj, d)
	if obj.Spec.Interval == nil || *obj.Spec.Interval != d {
		t.Error("Interval not set")
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

// Alert setters

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

// Receiver setters

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

// ImageUpdateAutomation setters

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

func TestCreateImageRef(t *testing.T) {
	ref := CreateImageRef("nginx", "1.25", "sha256:abc")
	if ref.Name != "nginx" || ref.Tag != "1.25" || ref.Digest != "sha256:abc" {
		t.Errorf("unexpected ImageRef: %+v", ref)
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

func TestAddResourceSetDependency(t *testing.T) {
	obj := CreateResourceSet("rs", "ns")
	dep := fluxv1.Dependency{Name: "infra-rs"}
	AddResourceSetDependency(obj, dep)
	if len(obj.Spec.DependsOn) != 1 || obj.Spec.DependsOn[0].Name != "infra-rs" {
		t.Error("Dependency not appended")
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
