package fluxcd

import (
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FluxReport setter tests
func TestSetFluxReportDistribution(t *testing.T) {
	fr := CreateFluxReport("test", "default", fluxv1.FluxReportSpec{})
	err := SetFluxReportDistribution(fr, fluxv1.FluxDistributionStatus{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetFluxReportCluster(t *testing.T) {
	fr := CreateFluxReport("test", "default", fluxv1.FluxReportSpec{})
	err := SetFluxReportCluster(fr, &fluxv1.ClusterInfo{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetFluxReportOperator(t *testing.T) {
	fr := CreateFluxReport("test", "default", fluxv1.FluxReportSpec{})
	err := SetFluxReportOperator(fr, &fluxv1.OperatorInfo{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetFluxReportSyncStatus(t *testing.T) {
	fr := CreateFluxReport("test", "default", fluxv1.FluxReportSpec{})
	err := SetFluxReportSyncStatus(fr, &fluxv1.FluxSyncStatus{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// HelmRelease setter tests
func TestSetHelmReleaseChart(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	SetHelmReleaseChart(hr, &helmv2.HelmChartTemplate{})
	if hr.Spec.Chart == nil {
		t.Fatal("expected Chart to be set")
	}
}

func TestSetHelmReleaseChartRef(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	SetHelmReleaseChartRef(hr, &helmv2.CrossNamespaceSourceReference{})
	if hr.Spec.ChartRef == nil {
		t.Fatal("expected ChartRef to be set")
	}
}

func TestSetHelmReleaseDriftDetection(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	SetHelmReleaseDriftDetection(hr, &helmv2.DriftDetection{})
	if hr.Spec.DriftDetection == nil {
		t.Fatal("expected DriftDetection to be set")
	}
}

func TestSetHelmReleaseInstall(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	SetHelmReleaseInstall(hr, &helmv2.Install{})
	if hr.Spec.Install == nil {
		t.Fatal("expected Install to be set")
	}
}

func TestSetHelmReleaseUpgrade(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	SetHelmReleaseUpgrade(hr, &helmv2.Upgrade{})
	if hr.Spec.Upgrade == nil {
		t.Fatal("expected Upgrade to be set")
	}
}

func TestSetHelmReleaseRollback(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	SetHelmReleaseRollback(hr, &helmv2.Rollback{})
	if hr.Spec.Rollback == nil {
		t.Fatal("expected Rollback to be set")
	}
}

func TestSetHelmReleaseUninstall(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	SetHelmReleaseUninstall(hr, &helmv2.Uninstall{})
	if hr.Spec.Uninstall == nil {
		t.Fatal("expected Uninstall to be set")
	}
}

func TestSetHelmReleaseTest(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	SetHelmReleaseTest(hr, &helmv2.Test{})
	if hr.Spec.Test == nil {
		t.Fatal("expected Test to be set")
	}
}

// ImageUpdateAutomation setter tests
func TestSetImageUpdateAutomationGitSpec(t *testing.T) {
	auto := CreateImageUpdateAutomation("test", "default", imagev1.ImageUpdateAutomationSpec{})
	SetImageUpdateAutomationGitSpec(auto, &imagev1.GitSpec{})
	if auto.Spec.GitSpec == nil {
		t.Fatal("expected GitSpec to be set")
	}
}

func TestSetGitCheckoutReference(t *testing.T) {
	spec := &imagev1.GitCheckoutSpec{}
	SetGitCheckoutReference(spec, sourcev1.GitRepositoryRef{Branch: "main"})
	if spec.Reference.Branch != "main" {
		t.Fatal("expected Reference.Branch to be 'main'")
	}
}

func TestSetCommitMessageTemplate(t *testing.T) {
	spec := &imagev1.CommitSpec{}
	SetCommitMessageTemplate(spec, "test template")
	if spec.MessageTemplate != "test template" {
		t.Fatal("expected MessageTemplate to be set")
	}
}

func TestSetCommitMessageTemplateValues(t *testing.T) {
	spec := &imagev1.CommitSpec{}
	vals := map[string]string{"key": "value"}
	SetCommitMessageTemplateValues(spec, vals)
	if spec.MessageTemplateValues["key"] != "value" {
		t.Fatal("expected MessageTemplateValues to be set")
	}
}

func TestSetCommitAuthor(t *testing.T) {
	spec := &imagev1.CommitSpec{}
	author := imagev1.CommitUser{Name: "test", Email: "test@example.com"}
	SetCommitAuthor(spec, author)
	if spec.Author.Name != "test" {
		t.Fatal("expected Author to be set")
	}
}

func TestSetPushBranch(t *testing.T) {
	spec := &imagev1.PushSpec{}
	SetPushBranch(spec, "main")
	if spec.Branch != "main" {
		t.Fatal("expected Branch to be 'main'")
	}
}

func TestSetPushRefspec(t *testing.T) {
	spec := &imagev1.PushSpec{}
	SetPushRefspec(spec, "refs/heads/main")
	if spec.Refspec != "refs/heads/main" {
		t.Fatal("expected Refspec to be set")
	}
}

func TestSetPushOptions(t *testing.T) {
	spec := &imagev1.PushSpec{}
	opts := map[string]string{"key": "value"}
	SetPushOptions(spec, opts)
	if spec.Options["key"] != "value" {
		t.Fatal("expected Options to be set")
	}
}

func TestSetGitSpecCheckout(t *testing.T) {
	spec := &imagev1.GitSpec{}
	checkout := &imagev1.GitCheckoutSpec{}
	SetGitSpecCheckout(spec, checkout)
	if spec.Checkout == nil {
		t.Fatal("expected Checkout to be set")
	}
}

func TestSetGitSpecCommit(t *testing.T) {
	spec := &imagev1.GitSpec{}
	commit := imagev1.CommitSpec{}
	SetGitSpecCommit(spec, commit)
}

func TestSetGitSpecPush(t *testing.T) {
	spec := &imagev1.GitSpec{}
	push := &imagev1.PushSpec{}
	SetGitSpecPush(spec, push)
	if spec.Push == nil {
		t.Fatal("expected Push to be set")
	}
}

func TestSetImageRefDigest(t *testing.T) {
	ref := &imagev1.ImageRef{}
	SetImageRefDigest(ref, "sha256:abc")
	if ref.Digest != "sha256:abc" {
		t.Fatal("expected Digest to be set")
	}
}

func TestSetImageRefTag(t *testing.T) {
	ref := &imagev1.ImageRef{}
	SetImageRefTag(ref, "v1.0.0")
	if ref.Tag != "v1.0.0" {
		t.Fatal("expected Tag to be set")
	}
}

func TestSetImageRefName(t *testing.T) {
	ref := &imagev1.ImageRef{}
	SetImageRefName(ref, "nginx")
	if ref.Name != "nginx" {
		t.Fatal("expected Name to be set")
	}
}

func TestSetObservedPolicies(t *testing.T) {
	auto := CreateImageUpdateAutomation("test", "default", imagev1.ImageUpdateAutomationSpec{})
	policies := imagev1.ObservedPolicies{"test": imagev1.ImageRef{}}
	SetObservedPolicies(auto, policies)
	if len(auto.Status.ObservedPolicies) != 1 {
		t.Fatal("expected ObservedPolicies to be set")
	}
}

// Kustomization setter tests
func TestSetKustomizationKubeConfig(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationKubeConfig(k, &meta.KubeConfigReference{})
	if k.Spec.KubeConfig == nil {
		t.Fatal("expected KubeConfig to be set")
	}
}

func TestSetKustomizationSourceRef(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationSourceRef(k, kustv1.CrossNamespaceSourceReference{})
}

func TestSetKustomizationServiceAccountName(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationServiceAccountName(k, "test-sa")
	if k.Spec.ServiceAccountName != "test-sa" {
		t.Fatal("expected ServiceAccountName to be set")
	}
}

func TestSetKustomizationSuspend(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationSuspend(k, true)
	if !k.Spec.Suspend {
		t.Fatal("expected Suspend to be true")
	}
}

func TestSetKustomizationTargetNamespace(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationTargetNamespace(k, "target")
	if k.Spec.TargetNamespace != "target" {
		t.Fatal("expected TargetNamespace to be set")
	}
}

func TestSetKustomizationTimeout(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	timeout := metav1.Duration{Duration: 60}
	SetKustomizationTimeout(k, timeout)
	if k.Spec.Timeout == nil {
		t.Fatal("expected Timeout to be set")
	}
}

func TestSetKustomizationForce(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationForce(k, true)
	if !k.Spec.Force {
		t.Fatal("expected Force to be true")
	}
}

func TestSetKustomizationWait(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationWait(k, true)
	if !k.Spec.Wait {
		t.Fatal("expected Wait to be true")
	}
}

func TestSetKustomizationNamePrefix(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationNamePrefix(k, "prefix-")
	if k.Spec.NamePrefix != "prefix-" {
		t.Fatal("expected NamePrefix to be set")
	}
}

func TestSetKustomizationNameSuffix(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationNameSuffix(k, "-suffix")
	if k.Spec.NameSuffix != "-suffix" {
		t.Fatal("expected NameSuffix to be set")
	}
}

func TestSetKustomizationCommonMetadata(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationCommonMetadata(k, &kustv1.CommonMetadata{})
	if k.Spec.CommonMetadata == nil {
		t.Fatal("expected CommonMetadata to be set")
	}
}

func TestSetKustomizationDecryption(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationDecryption(k, &kustv1.Decryption{})
	if k.Spec.Decryption == nil {
		t.Fatal("expected Decryption to be set")
	}
}

func TestSetKustomizationPostBuild(t *testing.T) {
	k := CreateKustomization("test", "default", kustv1.KustomizationSpec{})
	SetKustomizationPostBuild(k, &kustv1.PostBuild{})
	if k.Spec.PostBuild == nil {
		t.Fatal("expected PostBuild to be set")
	}
}

// Notification setter tests
func TestSetProviderType(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	SetProviderType(p, "slack")
	if p.Spec.Type != "slack" {
		t.Fatal("expected Type to be 'slack'")
	}
}

func TestSetProviderInterval(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	interval := metav1.Duration{Duration: 60}
	SetProviderInterval(p, interval)
	if p.Spec.Interval == nil {
		t.Fatal("expected Interval to be set")
	}
}

func TestSetProviderChannel(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	SetProviderChannel(p, "#general")
	if p.Spec.Channel != "#general" {
		t.Fatal("expected Channel to be set")
	}
}

func TestSetProviderUsername(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	SetProviderUsername(p, "bot")
	if p.Spec.Username != "bot" {
		t.Fatal("expected Username to be set")
	}
}

func TestSetProviderAddress(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	SetProviderAddress(p, "https://slack.com")
	if p.Spec.Address != "https://slack.com" {
		t.Fatal("expected Address to be set")
	}
}

func TestSetProviderTimeout(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	timeout := metav1.Duration{Duration: 30}
	SetProviderTimeout(p, timeout)
	if p.Spec.Timeout == nil {
		t.Fatal("expected Timeout to be set")
	}
}

func TestSetProviderProxy(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	SetProviderProxy(p, "http://proxy:8080")
	if p.Spec.Proxy != "http://proxy:8080" {
		t.Fatal("expected Proxy to be set")
	}
}

func TestSetProviderSecretRef(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	SetProviderSecretRef(p, &meta.LocalObjectReference{Name: "secret"})
	if p.Spec.SecretRef == nil {
		t.Fatal("expected SecretRef to be set")
	}
}

func TestSetProviderCertSecretRef(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	SetProviderCertSecretRef(p, &meta.LocalObjectReference{Name: "cert"})
	if p.Spec.CertSecretRef == nil {
		t.Fatal("expected CertSecretRef to be set")
	}
}

func TestSetProviderSuspend(t *testing.T) {
	p := CreateProvider("test", "default", notificationv1beta2.ProviderSpec{})
	SetProviderSuspend(p, true)
	if !p.Spec.Suspend {
		t.Fatal("expected Suspend to be true")
	}
}

func TestSetReceiverType(t *testing.T) {
	r := CreateReceiver("test", "default", notificationv1beta2.ReceiverSpec{})
	SetReceiverType(r, "github")
	if r.Spec.Type != "github" {
		t.Fatal("expected Type to be 'github'")
	}
}

func TestSetReceiverInterval(t *testing.T) {
	r := CreateReceiver("test", "default", notificationv1beta2.ReceiverSpec{})
	interval := metav1.Duration{Duration: 60}
	SetReceiverInterval(r, interval)
	if r.Spec.Interval == nil {
		t.Fatal("expected Interval to be set")
	}
}

func TestSetReceiverSecretRef(t *testing.T) {
	r := CreateReceiver("test", "default", notificationv1beta2.ReceiverSpec{})
	SetReceiverSecretRef(r, meta.LocalObjectReference{Name: "secret"})
	if r.Spec.SecretRef.Name != "secret" {
		t.Fatal("expected SecretRef to be set")
	}
}

func TestSetReceiverSuspend(t *testing.T) {
	r := CreateReceiver("test", "default", notificationv1beta2.ReceiverSpec{})
	SetReceiverSuspend(r, true)
	if !r.Spec.Suspend {
		t.Fatal("expected Suspend to be true")
	}
}

// ResourceSet setter tests
func TestSetResourceSetResourcesTemplate(t *testing.T) {
	rs := CreateResourceSet("test", "default", fluxv1.ResourceSetSpec{})
	err := SetResourceSetResourcesTemplate(rs, "template")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rs.Spec.ResourcesTemplate != "template" {
		t.Fatal("expected ResourcesTemplate to be set")
	}
}

func TestSetResourceSetServiceAccountName(t *testing.T) {
	rs := CreateResourceSet("test", "default", fluxv1.ResourceSetSpec{})
	err := SetResourceSetServiceAccountName(rs, "test-sa")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rs.Spec.ServiceAccountName != "test-sa" {
		t.Fatal("expected ServiceAccountName to be set")
	}
}

func TestSetResourceSetWait(t *testing.T) {
	rs := CreateResourceSet("test", "default", fluxv1.ResourceSetSpec{})
	err := SetResourceSetWait(rs, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !rs.Spec.Wait {
		t.Fatal("expected Wait to be true")
	}
}

func TestSetResourceSetCommonMetadata(t *testing.T) {
	rs := CreateResourceSet("test", "default", fluxv1.ResourceSetSpec{})
	err := SetResourceSetCommonMetadata(rs, &fluxv1.CommonMetadata{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rs.Spec.CommonMetadata == nil {
		t.Fatal("expected CommonMetadata to be set")
	}
}

// ResourceSetInputProvider setter tests
func TestSetResourceSetInputProviderType(t *testing.T) {
	obj := CreateResourceSetInputProvider("test", "default", fluxv1.ResourceSetInputProviderSpec{})
	err := SetResourceSetInputProviderType(obj, "http")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if obj.Spec.Type != "http" {
		t.Fatal("expected Type to be set")
	}
}

func TestSetResourceSetInputProviderURL(t *testing.T) {
	obj := CreateResourceSetInputProvider("test", "default", fluxv1.ResourceSetInputProviderSpec{})
	err := SetResourceSetInputProviderURL(obj, "http://example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if obj.Spec.URL != "http://example.com" {
		t.Fatal("expected URL to be set")
	}
}

func TestSetResourceSetInputProviderServiceAccountName(t *testing.T) {
	obj := CreateResourceSetInputProvider("test", "default", fluxv1.ResourceSetInputProviderSpec{})
	err := SetResourceSetInputProviderServiceAccountName(obj, "test-sa")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if obj.Spec.ServiceAccountName != "test-sa" {
		t.Fatal("expected ServiceAccountName to be set")
	}
}

func TestSetResourceSetInputProviderSecretRef(t *testing.T) {
	obj := CreateResourceSetInputProvider("test", "default", fluxv1.ResourceSetInputProviderSpec{})
	err := SetResourceSetInputProviderSecretRef(obj, &meta.LocalObjectReference{Name: "secret"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if obj.Spec.SecretRef == nil {
		t.Fatal("expected SecretRef to be set")
	}
}

func TestSetResourceSetInputProviderCertSecretRef(t *testing.T) {
	obj := CreateResourceSetInputProvider("test", "default", fluxv1.ResourceSetInputProviderSpec{})
	err := SetResourceSetInputProviderCertSecretRef(obj, &meta.LocalObjectReference{Name: "cert"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if obj.Spec.CertSecretRef == nil {
		t.Fatal("expected CertSecretRef to be set")
	}
}

// Schedule setter tests
func TestSetScheduleTimeZone(t *testing.T) {
	s := CreateSchedule("0 0 * * *")
	err := SetScheduleTimeZone(&s, "UTC")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.TimeZone != "UTC" {
		t.Fatal("expected TimeZone to be set")
	}
}

func TestSetScheduleWindow(t *testing.T) {
	s := CreateSchedule("0 0 * * *")
	window := metav1.Duration{Duration: 60}
	err := SetScheduleWindow(&s, window)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// Source controller setter tests - GitRepository
func TestSetGitRepositoryURL(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	SetGitRepositoryURL(gr, "https://github.com/test/repo")
	if gr.Spec.URL != "https://github.com/test/repo" {
		t.Fatal("expected URL to be set")
	}
}

func TestSetGitRepositorySecretRef(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	SetGitRepositorySecretRef(gr, &meta.LocalObjectReference{Name: "secret"})
	if gr.Spec.SecretRef == nil {
		t.Fatal("expected SecretRef to be set")
	}
}

func TestSetGitRepositoryProvider(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	SetGitRepositoryProvider(gr, "github")
	if gr.Spec.Provider != "github" {
		t.Fatal("expected Provider to be set")
	}
}

func TestSetGitRepositoryInterval(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	interval := metav1.Duration{Duration: 60}
	SetGitRepositoryInterval(gr, interval)
}

func TestSetGitRepositoryTimeout(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	timeout := metav1.Duration{Duration: 30}
	SetGitRepositoryTimeout(gr, &timeout)
	if gr.Spec.Timeout == nil {
		t.Fatal("expected Timeout to be set")
	}
}

func TestSetGitRepositoryReference(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Branch: "main"})
	if gr.Spec.Reference == nil {
		t.Fatal("expected Reference to be set")
	}
}

func TestSetGitRepositoryVerification(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	SetGitRepositoryVerification(gr, &sourcev1.GitRepositoryVerification{})
	if gr.Spec.Verification == nil {
		t.Fatal("expected Verification to be set")
	}
}

func TestSetGitRepositoryProxySecretRef(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	SetGitRepositoryProxySecretRef(gr, &meta.LocalObjectReference{Name: "proxy"})
	if gr.Spec.ProxySecretRef == nil {
		t.Fatal("expected ProxySecretRef to be set")
	}
}

func TestSetGitRepositoryIgnore(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	SetGitRepositoryIgnore(gr, "*.txt")
	if gr.Spec.Ignore == nil {
		t.Fatal("expected Ignore to be set")
	}
}

func TestSetGitRepositorySuspend(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	SetGitRepositorySuspend(gr, true)
	if !gr.Spec.Suspend {
		t.Fatal("expected Suspend to be true")
	}
}

func TestSetGitRepositoryRecurseSubmodules(t *testing.T) {
	gr := CreateGitRepository("test", "default", sourcev1.GitRepositorySpec{})
	SetGitRepositoryRecurseSubmodules(gr, true)
	if !gr.Spec.RecurseSubmodules {
		t.Fatal("expected RecurseSubmodules to be true")
	}
}

// HelmRepository setter tests
func TestSetHelmRepositoryURL(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	SetHelmRepositoryURL(hr, "https://charts.example.com")
	if hr.Spec.URL != "https://charts.example.com" {
		t.Fatal("expected URL to be set")
	}
}

func TestSetHelmRepositorySecretRef(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "secret"})
	if hr.Spec.SecretRef == nil {
		t.Fatal("expected SecretRef to be set")
	}
}

func TestSetHelmRepositoryCertSecretRef(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	SetHelmRepositoryCertSecretRef(hr, &meta.LocalObjectReference{Name: "cert"})
	if hr.Spec.CertSecretRef == nil {
		t.Fatal("expected CertSecretRef to be set")
	}
}

func TestSetHelmRepositoryPassCredentials(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	SetHelmRepositoryPassCredentials(hr, true)
	if !hr.Spec.PassCredentials {
		t.Fatal("expected PassCredentials to be true")
	}
}

func TestSetHelmRepositoryInterval(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	interval := metav1.Duration{Duration: 60}
	SetHelmRepositoryInterval(hr, interval)
}

func TestSetHelmRepositoryInsecure(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	SetHelmRepositoryInsecure(hr, true)
	if !hr.Spec.Insecure {
		t.Fatal("expected Insecure to be true")
	}
}

func TestSetHelmRepositoryTimeout(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	timeout := metav1.Duration{Duration: 30}
	SetHelmRepositoryTimeout(hr, &timeout)
	if hr.Spec.Timeout == nil {
		t.Fatal("expected Timeout to be set")
	}
}

func TestSetHelmRepositorySuspend(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	SetHelmRepositorySuspend(hr, true)
	if !hr.Spec.Suspend {
		t.Fatal("expected Suspend to be true")
	}
}

func TestSetHelmRepositoryType(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	SetHelmRepositoryType(hr, "oci")
	if hr.Spec.Type != "oci" {
		t.Fatal("expected Type to be set")
	}
}

func TestSetHelmRepositoryProvider(t *testing.T) {
	hr := CreateHelmRepository("test", "default", sourcev1.HelmRepositorySpec{})
	SetHelmRepositoryProvider(hr, "aws")
	if hr.Spec.Provider != "aws" {
		t.Fatal("expected Provider to be set")
	}
}

// Bucket setter tests
func TestSetBucketProvider(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketProvider(b, "aws")
	if b.Spec.Provider != "aws" {
		t.Fatal("expected Provider to be set")
	}
}

func TestSetBucketName(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketName(b, "my-bucket")
	if b.Spec.BucketName != "my-bucket" {
		t.Fatal("expected BucketName to be set")
	}
}

func TestSetBucketEndpoint(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketEndpoint(b, "s3.amazonaws.com")
	if b.Spec.Endpoint != "s3.amazonaws.com" {
		t.Fatal("expected Endpoint to be set")
	}
}

func TestSetBucketSTS(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketSTS(b, &sourcev1.BucketSTSSpec{})
	if b.Spec.STS == nil {
		t.Fatal("expected STS to be set")
	}
}

func TestSetBucketInsecure(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketInsecure(b, true)
	if !b.Spec.Insecure {
		t.Fatal("expected Insecure to be true")
	}
}

func TestSetBucketRegion(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketRegion(b, "us-west-2")
	if b.Spec.Region != "us-west-2" {
		t.Fatal("expected Region to be set")
	}
}

func TestSetBucketPrefix(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketPrefix(b, "prefix/")
	if b.Spec.Prefix != "prefix/" {
		t.Fatal("expected Prefix to be set")
	}
}

func TestSetBucketSecretRef(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketSecretRef(b, &meta.LocalObjectReference{Name: "secret"})
	if b.Spec.SecretRef == nil {
		t.Fatal("expected SecretRef to be set")
	}
}

func TestSetBucketCertSecretRef(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketCertSecretRef(b, &meta.LocalObjectReference{Name: "cert"})
	if b.Spec.CertSecretRef == nil {
		t.Fatal("expected CertSecretRef to be set")
	}
}

func TestSetBucketProxySecretRef(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketProxySecretRef(b, &meta.LocalObjectReference{Name: "proxy"})
	if b.Spec.ProxySecretRef == nil {
		t.Fatal("expected ProxySecretRef to be set")
	}
}

func TestSetBucketInterval(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	interval := metav1.Duration{Duration: 60}
	SetBucketInterval(b, interval)
}

func TestSetBucketTimeout(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	timeout := metav1.Duration{Duration: 30}
	SetBucketTimeout(b, &timeout)
	if b.Spec.Timeout == nil {
		t.Fatal("expected Timeout to be set")
	}
}

func TestSetBucketIgnore(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketIgnore(b, "*.txt")
	if b.Spec.Ignore == nil {
		t.Fatal("expected Ignore to be set")
	}
}

func TestSetBucketSuspend(t *testing.T) {
	b := CreateBucket("test", "default", sourcev1.BucketSpec{})
	SetBucketSuspend(b, true)
	if !b.Spec.Suspend {
		t.Fatal("expected Suspend to be true")
	}
}

// HelmChart setter tests
func TestSetHelmChartChart(t *testing.T) {
	hc := CreateHelmChart("test", "default", sourcev1.HelmChartSpec{})
	SetHelmChartChart(hc, "nginx")
	if hc.Spec.Chart != "nginx" {
		t.Fatal("expected Chart to be set")
	}
}

func TestSetHelmChartVersion(t *testing.T) {
	hc := CreateHelmChart("test", "default", sourcev1.HelmChartSpec{})
	SetHelmChartVersion(hc, "1.0.0")
	if hc.Spec.Version != "1.0.0" {
		t.Fatal("expected Version to be set")
	}
}

func TestSetHelmChartSourceRef(t *testing.T) {
	hc := CreateHelmChart("test", "default", sourcev1.HelmChartSpec{})
	SetHelmChartSourceRef(hc, sourcev1.LocalHelmChartSourceReference{})
}

func TestSetHelmChartInterval(t *testing.T) {
	hc := CreateHelmChart("test", "default", sourcev1.HelmChartSpec{})
	interval := metav1.Duration{Duration: 60}
	SetHelmChartInterval(hc, interval)
}

func TestSetHelmChartReconcileStrategy(t *testing.T) {
	hc := CreateHelmChart("test", "default", sourcev1.HelmChartSpec{})
	SetHelmChartReconcileStrategy(hc, "ChartVersion")
	if hc.Spec.ReconcileStrategy != "ChartVersion" {
		t.Fatal("expected ReconcileStrategy to be set")
	}
}

func TestSetHelmChartValuesFiles(t *testing.T) {
	hc := CreateHelmChart("test", "default", sourcev1.HelmChartSpec{})
	SetHelmChartValuesFiles(hc, []string{"values.yaml"})
	if len(hc.Spec.ValuesFiles) != 1 {
		t.Fatal("expected ValuesFiles to be set")
	}
}

func TestSetHelmChartIgnoreMissingValuesFiles(t *testing.T) {
	hc := CreateHelmChart("test", "default", sourcev1.HelmChartSpec{})
	SetHelmChartIgnoreMissingValuesFiles(hc, true)
	if !hc.Spec.IgnoreMissingValuesFiles {
		t.Fatal("expected IgnoreMissingValuesFiles to be true")
	}
}

func TestSetHelmChartSuspend(t *testing.T) {
	hc := CreateHelmChart("test", "default", sourcev1.HelmChartSpec{})
	SetHelmChartSuspend(hc, true)
	if !hc.Spec.Suspend {
		t.Fatal("expected Suspend to be true")
	}
}

func TestSetHelmChartVerify(t *testing.T) {
	hc := CreateHelmChart("test", "default", sourcev1.HelmChartSpec{})
	SetHelmChartVerify(hc, &sourcev1.OCIRepositoryVerification{})
	if hc.Spec.Verify == nil {
		t.Fatal("expected Verify to be set")
	}
}

// OCIRepository setter tests
func TestSetOCIRepositoryURL(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryURL(or, "oci://ghcr.io/test")
	if or.Spec.URL != "oci://ghcr.io/test" {
		t.Fatal("expected URL to be set")
	}
}

func TestSetOCIRepositoryReference(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryReference(or, &sourcev1beta2.OCIRepositoryRef{Tag: "latest"})
	if or.Spec.Reference == nil {
		t.Fatal("expected Reference to be set")
	}
}

func TestSetOCIRepositoryLayerSelector(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryLayerSelector(or, &sourcev1beta2.OCILayerSelector{})
	if or.Spec.LayerSelector == nil {
		t.Fatal("expected LayerSelector to be set")
	}
}

func TestSetOCIRepositoryProvider(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryProvider(or, "generic")
	if or.Spec.Provider != "generic" {
		t.Fatal("expected Provider to be set")
	}
}

func TestSetOCIRepositorySecretRef(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositorySecretRef(or, &meta.LocalObjectReference{Name: "secret"})
	if or.Spec.SecretRef == nil {
		t.Fatal("expected SecretRef to be set")
	}
}

func TestSetOCIRepositoryVerify(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryVerify(or, &sourcev1.OCIRepositoryVerification{})
	if or.Spec.Verify == nil {
		t.Fatal("expected Verify to be set")
	}
}

func TestSetOCIRepositoryServiceAccountName(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryServiceAccountName(or, "test-sa")
	if or.Spec.ServiceAccountName != "test-sa" {
		t.Fatal("expected ServiceAccountName to be set")
	}
}

func TestSetOCIRepositoryCertSecretRef(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryCertSecretRef(or, &meta.LocalObjectReference{Name: "cert"})
	if or.Spec.CertSecretRef == nil {
		t.Fatal("expected CertSecretRef to be set")
	}
}

func TestSetOCIRepositoryProxySecretRef(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryProxySecretRef(or, &meta.LocalObjectReference{Name: "proxy"})
	if or.Spec.ProxySecretRef == nil {
		t.Fatal("expected ProxySecretRef to be set")
	}
}

func TestSetOCIRepositoryInterval(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	interval := metav1.Duration{Duration: 60}
	SetOCIRepositoryInterval(or, interval)
}

func TestSetOCIRepositoryTimeout(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	timeout := metav1.Duration{Duration: 30}
	SetOCIRepositoryTimeout(or, &timeout)
	if or.Spec.Timeout == nil {
		t.Fatal("expected Timeout to be set")
	}
}

func TestSetOCIRepositoryIgnore(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryIgnore(or, "*.txt")
	if or.Spec.Ignore == nil {
		t.Fatal("expected Ignore to be set")
	}
}

func TestSetOCIRepositoryInsecure(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryInsecure(or, true)
	if !or.Spec.Insecure {
		t.Fatal("expected Insecure to be true")
	}
}

func TestSetOCIRepositorySuspend(t *testing.T) {
	or := CreateOCIRepository("test", "default", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositorySuspend(or, true)
	if !or.Spec.Suspend {
		t.Fatal("expected Suspend to be true")
	}
}

// Test nil cases for error handling
func TestAddResourceSetResource_NilResource(t *testing.T) {
	rs := CreateResourceSet("test", "default", fluxv1.ResourceSetSpec{})
	err := AddResourceSetResource(rs, nil)
	if err == nil {
		t.Fatal("expected error for nil resource")
	}
}

func TestAddResourceSetResource_ValidResource(t *testing.T) {
	rs := CreateResourceSet("test", "default", fluxv1.ResourceSetSpec{})
	resource := &apiextensionsv1.JSON{Raw: []byte("{}")}
	err := AddResourceSetResource(rs, resource)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
