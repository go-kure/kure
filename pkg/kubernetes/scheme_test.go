package kubernetes

import (
	"testing"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storv1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func TestSchemeInitialization(t *testing.T) {
	// Test that global variables are initialized
	if Scheme == nil {
		t.Fatal("expected Scheme to be initialized")
	}

	// Test that Codecs is usable (can't compare to nil directly)

	// Verify Scheme is a runtime.Scheme
	var _ *runtime.Scheme = Scheme

	// Verify Codecs is a CodecFactory
	var _ serializer.CodecFactory = Codecs
}

func TestRegisterSchemes_Success(t *testing.T) {
	// Test successful registration
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("expected RegisterSchemes to succeed, got error: %v", err)
	}

	// Test that subsequent calls return the same result (due to sync.Once)
	err2 := RegisterSchemes()
	if err2 != nil {
		t.Fatalf("expected RegisterSchemes to succeed on second call, got error: %v", err2)
	}
}

func TestRegisterSchemes_Idempotent(t *testing.T) {
	// Test that RegisterSchemes can be called multiple times safely
	for i := 0; i < 5; i++ {
		err := RegisterSchemes()
		if err != nil {
			t.Fatalf("RegisterSchemes failed on call %d: %v", i+1, err)
		}
	}
}

func TestScheme_RegisteredTypes(t *testing.T) {
	// First ensure schemes are registered
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test some core Kubernetes types are registered
	coreTypes := []runtime.Object{
		&corev1.Pod{},
		&corev1.Service{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.Namespace{},
		&corev1.ServiceAccount{},
	}

	for _, obj := range coreTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test some apps/v1 types are registered
	appsTypes := []runtime.Object{
		&appsv1.Deployment{},
		&appsv1.StatefulSet{},
		&appsv1.DaemonSet{},
	}

	for _, obj := range appsTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test some RBAC types are registered
	rbacTypes := []runtime.Object{
		&rbacv1.Role{},
		&rbacv1.ClusterRole{},
		&rbacv1.RoleBinding{},
		&rbacv1.ClusterRoleBinding{},
	}

	for _, obj := range rbacTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test some batch types are registered
	batchTypes := []runtime.Object{
		&batchv1.Job{},
		&batchv1.CronJob{},
	}

	for _, obj := range batchTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test some networking types are registered
	networkingTypes := []runtime.Object{
		&netv1.Ingress{},
		&netv1.NetworkPolicy{},
	}

	for _, obj := range networkingTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test some storage types are registered
	storageTypes := []runtime.Object{
		&storv1.StorageClass{},
		&storv1.VolumeAttachment{},
	}

	for _, obj := range storageTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}
}

func TestScheme_FluxTypes(t *testing.T) {
	// First ensure schemes are registered
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test some Flux source controller types
	fluxSourceTypes := []runtime.Object{
		&sourcev1.GitRepository{},
		&sourcev1.HelmRepository{},
		&sourcev1.Bucket{},
		&sourcev1beta2.OCIRepository{},
		&sourcev1beta2.HelmChart{},
	}

	for _, obj := range fluxSourceTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test some Flux kustomize controller types
	fluxKustomizeTypes := []runtime.Object{
		&kustv1.Kustomization{},
	}

	for _, obj := range fluxKustomizeTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test some Flux helm controller types
	fluxHelmTypes := []runtime.Object{
		&helmv2.HelmRelease{},
	}

	for _, obj := range fluxHelmTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test some Flux notification controller types
	fluxNotificationTypes := []runtime.Object{
		&notificationv1beta2.Receiver{},
	}

	for _, obj := range fluxNotificationTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test Flux operator types
	fluxOperatorTypes := []runtime.Object{
		&fluxv1.FluxInstance{},
	}

	for _, obj := range fluxOperatorTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}
}

func TestScheme_CertManagerTypes(t *testing.T) {
	// First ensure schemes are registered
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test cert-manager types
	certManagerTypes := []runtime.Object{
		&certv1.Certificate{},
		&certv1.ClusterIssuer{},
		&certv1.Issuer{},
	}

	for _, obj := range certManagerTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}

	// Test cert-manager ACME types
	certManagerAcmeTypes := []runtime.Object{
		&cmacme.Challenge{},
		&cmacme.Order{},
	}

	for _, obj := range certManagerAcmeTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}
}

func TestScheme_ExternalSecretsTypes(t *testing.T) {
	// First ensure schemes are registered
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test External Secrets types
	externalSecretsTypes := []runtime.Object{
		&esv1.ExternalSecret{},
		&esv1.SecretStore{},
		&esv1.ClusterSecretStore{},
	}

	for _, obj := range externalSecretsTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}
}

func TestScheme_MetalLBTypes(t *testing.T) {
	// First ensure schemes are registered
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test MetalLB types
	metallbTypes := []runtime.Object{
		&metallbv1beta1.IPAddressPool{},
		&metallbv1beta1.BGPPeer{},
		&metallbv1beta1.BGPAdvertisement{},
		&metallbv1beta1.L2Advertisement{},
		&metallbv1beta1.BFDProfile{},
	}

	for _, obj := range metallbTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}
}

func TestScheme_APIExtensionsTypes(t *testing.T) {
	// First ensure schemes are registered
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test API extensions types
	apiExtensionsTypes := []runtime.Object{
		&apiextensionsv1.CustomResourceDefinition{},
	}

	for _, obj := range apiExtensionsTypes {
		gvks, _, err := Scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("failed to get GVKs for %T: %v", obj, err)
		}
		if len(gvks) == 0 {
			t.Errorf("no GVKs found for %T", obj)
		}
	}
}

func TestCodecs_Creation(t *testing.T) {
	// Test that Codecs can be used for encoding/decoding

	// Test getting a decoder
	decoder := Codecs.UniversalDecoder()
	if decoder == nil {
		t.Error("expected non-nil universal decoder")
	}

	// Test getting an encoder
	encoder := Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	if encoder == nil {
		t.Error("expected non-nil legacy codec")
	}
}

func TestRegisterSchemes_ThreadSafety(t *testing.T) {
	// Test concurrent registration to ensure thread safety with sync.Once
	const goroutines = 10
	done := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			done <- RegisterSchemes()
		}()
	}

	// Collect all results
	var errors []error
	for i := 0; i < goroutines; i++ {
		if err := <-done; err != nil {
			errors = append(errors, err)
		}
	}

	// All calls should succeed since registration is protected by sync.Once
	if len(errors) > 0 {
		t.Errorf("expected all concurrent RegisterSchemes calls to succeed, got %d errors: %v", len(errors), errors)
	}
}
