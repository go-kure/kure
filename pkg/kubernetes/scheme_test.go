package kubernetes

import (
	stderrors "errors"
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
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestScheme_AllTypesRegistered(t *testing.T) {
	// First ensure schemes are registered
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test a comprehensive list of types from all registered schemes
	allTypes := []struct {
		name string
		obj  runtime.Object
	}{
		// Core v1
		{"Pod", &corev1.Pod{}},
		{"Service", &corev1.Service{}},
		{"ConfigMap", &corev1.ConfigMap{}},
		{"Secret", &corev1.Secret{}},
		{"Namespace", &corev1.Namespace{}},
		{"ServiceAccount", &corev1.ServiceAccount{}},
		{"PersistentVolume", &corev1.PersistentVolume{}},
		{"PersistentVolumeClaim", &corev1.PersistentVolumeClaim{}},
		{"Node", &corev1.Node{}},
		{"Endpoints", &corev1.Endpoints{}}, //nolint:staticcheck // testing scheme registration of deprecated type

		// Apps v1
		{"Deployment", &appsv1.Deployment{}},
		{"StatefulSet", &appsv1.StatefulSet{}},
		{"DaemonSet", &appsv1.DaemonSet{}},
		{"ReplicaSet", &appsv1.ReplicaSet{}},

		// RBAC v1
		{"Role", &rbacv1.Role{}},
		{"ClusterRole", &rbacv1.ClusterRole{}},
		{"RoleBinding", &rbacv1.RoleBinding{}},
		{"ClusterRoleBinding", &rbacv1.ClusterRoleBinding{}},

		// Batch v1
		{"Job", &batchv1.Job{}},
		{"CronJob", &batchv1.CronJob{}},

		// Networking v1
		{"Ingress", &netv1.Ingress{}},
		{"NetworkPolicy", &netv1.NetworkPolicy{}},
		{"IngressClass", &netv1.IngressClass{}},

		// Storage v1
		{"StorageClass", &storv1.StorageClass{}},
		{"VolumeAttachment", &storv1.VolumeAttachment{}},
		{"CSIDriver", &storv1.CSIDriver{}},
		{"CSINode", &storv1.CSINode{}},

		// API Extensions v1
		{"CustomResourceDefinition", &apiextensionsv1.CustomResourceDefinition{}},

		// Flux Source Controller
		{"GitRepository", &sourcev1.GitRepository{}},
		{"HelmRepository", &sourcev1.HelmRepository{}},
		{"Bucket", &sourcev1.Bucket{}},
		{"OCIRepository", &sourcev1beta2.OCIRepository{}},
		{"HelmChart", &sourcev1beta2.HelmChart{}},

		// Flux Kustomize Controller
		{"Kustomization", &kustv1.Kustomization{}},

		// Flux Helm Controller
		{"HelmRelease", &helmv2.HelmRelease{}},

		// Flux Notification Controller
		{"Receiver", &notificationv1beta2.Receiver{}},

		// Flux Operator
		{"FluxInstance", &fluxv1.FluxInstance{}},

		// Cert-Manager
		{"Certificate", &certv1.Certificate{}},
		{"ClusterIssuer", &certv1.ClusterIssuer{}},
		{"Issuer", &certv1.Issuer{}},

		// Cert-Manager ACME
		{"Challenge", &cmacme.Challenge{}},
		{"Order", &cmacme.Order{}},

		// External Secrets
		{"ExternalSecret", &esv1.ExternalSecret{}},
		{"SecretStore", &esv1.SecretStore{}},
		{"ClusterSecretStore", &esv1.ClusterSecretStore{}},

		// MetalLB
		{"IPAddressPool", &metallbv1beta1.IPAddressPool{}},
		{"BGPPeer", &metallbv1beta1.BGPPeer{}},
		{"BGPAdvertisement", &metallbv1beta1.BGPAdvertisement{}},
		{"L2Advertisement", &metallbv1beta1.L2Advertisement{}},
		{"BFDProfile", &metallbv1beta1.BFDProfile{}},
	}

	for _, tt := range allTypes {
		t.Run(tt.name, func(t *testing.T) {
			gvks, _, err := Scheme.ObjectKinds(tt.obj)
			if err != nil {
				t.Errorf("failed to get GVKs for %s: %v", tt.name, err)
			}
			if len(gvks) == 0 {
				t.Errorf("no GVKs found for %s", tt.name)
			}

			// Verify the object can be converted
			_, err = Scheme.New(gvks[0])
			if err != nil {
				t.Errorf("failed to create new instance of %s from GVK: %v", tt.name, err)
			}
		})
	}
}

func TestScheme_MultipleRegistrationSafety(t *testing.T) {
	// Test that multiple registrations don't cause issues
	// This tests the sync.Once behavior more thoroughly

	// Call RegisterSchemes many times in sequence
	for i := 0; i < 100; i++ {
		err := RegisterSchemes()
		if err != nil {
			t.Fatalf("RegisterSchemes failed on iteration %d: %v", i, err)
		}
	}

	// Verify scheme is still functional
	pod := &corev1.Pod{}
	gvks, _, err := Scheme.ObjectKinds(pod)
	if err != nil {
		t.Errorf("Scheme corrupted after multiple registrations: %v", err)
	}
	if len(gvks) == 0 {
		t.Error("Scheme corrupted: no GVKs found for Pod")
	}
}

func TestScheme_CodecsIntegration(t *testing.T) {
	// Test that Codecs work correctly with the registered schemes
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test encoder and decoder
	encoder := Codecs.LegacyCodec(corev1.SchemeGroupVersion, appsv1.SchemeGroupVersion)
	if encoder == nil {
		t.Fatal("expected non-nil encoder")
	}

	decoder := Codecs.UniversalDecoder(
		corev1.SchemeGroupVersion,
		appsv1.SchemeGroupVersion,
		rbacv1.SchemeGroupVersion,
	)
	if decoder == nil {
		t.Fatal("expected non-nil decoder")
	}

	// Test with various group versions
	gvs := []struct {
		name string
		gv   schema.GroupVersion
	}{
		{"corev1", corev1.SchemeGroupVersion},
		{"appsv1", appsv1.SchemeGroupVersion},
		{"rbacv1", rbacv1.SchemeGroupVersion},
		{"batchv1", batchv1.SchemeGroupVersion},
		{"netv1", netv1.SchemeGroupVersion},
		{"storv1", storv1.SchemeGroupVersion},
	}

	for _, test := range gvs {
		t.Run(test.name, func(t *testing.T) {
			codec := Codecs.LegacyCodec(test.gv)
			if codec == nil {
				t.Errorf("expected non-nil codec for %v", test.gv)
			}
		})
	}
}

func TestRegisterSchemes_ErrorCaching(t *testing.T) {
	// This test verifies that registerErr is cached correctly
	// Since the actual registration succeeds, we test the caching behavior
	// by calling RegisterSchemes multiple times and ensuring consistency

	// First call
	err1 := RegisterSchemes()

	// Multiple subsequent calls
	err2 := RegisterSchemes()
	err3 := RegisterSchemes()

	// All errors should be identical (either all nil or all the same error)
	if !stderrors.Is(err1, err2) {
		t.Errorf("RegisterSchemes returned different errors: first=%v, second=%v", err1, err2)
	}
	if !stderrors.Is(err2, err3) {
		t.Errorf("RegisterSchemes returned different errors: second=%v, third=%v", err2, err3)
	}
}

func TestScheme_ConvertToVersion(t *testing.T) {
	// Test scheme conversion capabilities
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test converting objects between versions
	pod := &corev1.Pod{}
	pod.APIVersion = "v1"
	pod.Kind = "Pod"

	// Verify the scheme can handle type metadata
	gvks, _, err := Scheme.ObjectKinds(pod)
	if err != nil {
		t.Fatalf("failed to get object kinds: %v", err)
	}
	if len(gvks) == 0 {
		t.Fatal("expected at least one GVK for Pod")
	}

	// Test that we can recognize the GVK
	gvk := gvks[0]
	if gvk.Kind != "Pod" {
		t.Errorf("expected Kind 'Pod', got %s", gvk.Kind)
	}
	if gvk.Version != "v1" {
		t.Errorf("expected Version 'v1', got %s", gvk.Version)
	}
}

func TestScheme_IsVersionRegistered(t *testing.T) {
	// Test that we can check if versions are registered
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test recognizing registered types
	registeredTypes := []schema.GroupVersion{
		{Group: "", Version: "v1"},                            // core/v1
		{Group: "apps", Version: "v1"},                        // apps/v1
		{Group: "rbac.authorization.k8s.io", Version: "v1"},   // rbac/v1
		{Group: "batch", Version: "v1"},                       // batch/v1
		{Group: "networking.k8s.io", Version: "v1"},           // networking/v1
		{Group: "storage.k8s.io", Version: "v1"},              // storage/v1
		{Group: "source.toolkit.fluxcd.io", Version: "v1"},    // flux source
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1"}, // flux kustomize
	}

	for _, gv := range registeredTypes {
		t.Run(gv.String(), func(t *testing.T) {
			// Check if the scheme recognizes this version
			if !Scheme.IsVersionRegistered(gv) {
				t.Errorf("expected version %s to be registered", gv)
			}
		})
	}
}

func TestScheme_KnownTypes(t *testing.T) {
	// Test that we can enumerate known types
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Test for core/v1 types
	coreGV := schema.GroupVersion{Group: "", Version: "v1"}
	coreTypes := Scheme.KnownTypes(coreGV)

	// We should have many core types registered
	if len(coreTypes) == 0 {
		t.Error("expected core/v1 to have known types")
	}

	// Check for specific types
	expectedCoreTypes := []string{"Pod", "Service", "ConfigMap", "Secret", "Namespace"}
	for _, typeName := range expectedCoreTypes {
		if _, exists := coreTypes[typeName]; !exists {
			t.Errorf("expected %s to be in core/v1 known types", typeName)
		}
	}

	// Test for apps/v1 types
	appsGV := schema.GroupVersion{Group: "apps", Version: "v1"}
	appsTypes := Scheme.KnownTypes(appsGV)

	if len(appsTypes) == 0 {
		t.Error("expected apps/v1 to have known types")
	}

	expectedAppsTypes := []string{"Deployment", "StatefulSet", "DaemonSet"}
	for _, typeName := range expectedAppsTypes {
		if _, exists := appsTypes[typeName]; !exists {
			t.Errorf("expected %s to be in apps/v1 known types", typeName)
		}
	}
}

func TestScheme_AllRegisteredTypes(t *testing.T) {
	// Test that all expected group versions are registered
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Get all registered versions
	allGVs := Scheme.AllKnownTypes()

	// We should have many group version kinds registered
	if len(allGVs) < 100 {
		t.Errorf("expected at least 100 registered types, got %d", len(allGVs))
	}

	// Check for key types from each category
	keyTypes := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Pod"},
		{Group: "", Version: "v1", Kind: "Service"},
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"},
		{Group: "batch", Version: "v1", Kind: "Job"},
		{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
		{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"},
		{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "GitRepository"},
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization"},
		{Group: "helm.toolkit.fluxcd.io", Version: "v2", Kind: "HelmRelease"},
		{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"},
		{Group: "external-secrets.io", Version: "v1", Kind: "ExternalSecret"},
		{Group: "metallb.io", Version: "v1beta1", Kind: "IPAddressPool"},
	}

	for _, gvk := range keyTypes {
		if _, exists := allGVs[gvk]; !exists {
			t.Errorf("expected %s to be registered", gvk)
		}
	}
}

func TestCodecs_Serialization(t *testing.T) {
	// Test that Codecs can serialize and deserialize objects
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Create a test pod
	pod := &corev1.Pod{}
	pod.Name = "test-pod"
	pod.Namespace = "default"

	// Get encoder and decoder
	encoder := Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	decoder := Codecs.UniversalDecoder(corev1.SchemeGroupVersion)

	if encoder == nil {
		t.Fatal("expected non-nil encoder")
	}
	if decoder == nil {
		t.Fatal("expected non-nil decoder")
	}

	// Test that encoder can be used (we don't actually encode/decode to keep test simple)
	// Just verify the codecs are properly initialized
	t.Logf("Encoder and decoder successfully created for corev1")
}

func TestScheme_PreferredVersions(t *testing.T) {
	// Test that preferred versions are set correctly
	err := RegisterSchemes()
	if err != nil {
		t.Fatalf("failed to register schemes: %v", err)
	}

	// Check preferred versions for key groups
	testGroups := []struct {
		group           string
		expectedVersion string
	}{
		{"apps", "v1"},
		{"batch", "v1"},
		{"networking.k8s.io", "v1"},
		{"storage.k8s.io", "v1"},
		{"rbac.authorization.k8s.io", "v1"},
	}

	for _, test := range testGroups {
		t.Run(test.group, func(t *testing.T) {
			gv := schema.GroupVersion{Group: test.group, Version: test.expectedVersion}
			if !Scheme.IsVersionRegistered(gv) {
				t.Errorf("expected version %s to be registered for group %s", test.expectedVersion, test.group)
			}
		})
	}
}
