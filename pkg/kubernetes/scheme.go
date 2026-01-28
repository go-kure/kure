package kubernetes

import (
	"sync"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
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

var (
	Scheme       = runtime.NewScheme()
	Codecs       = serializer.NewCodecFactory(Scheme)
	registerOnce sync.Once
	registerErr  error
)

// addSchemeFunc is a function that adds types to a scheme
type addSchemeFunc func(*runtime.Scheme) error

// RegisterSchemes adds all Kubernetes and Flux custom resource schemes to Scheme.
// The registration is performed only once. The first non-nil error returned by
// any AddToScheme call is cached and returned on subsequent invocations.
func RegisterSchemes() error {
	registerOnce.Do(func() {
		registerErr = registerAllSchemes()
	})
	return registerErr
}

// registerAllSchemes registers all schemes and returns the first error encountered
func registerAllSchemes() error {
	// List of all AddToScheme functions to register
	schemeFuncs := []addSchemeFunc{
		corev1.AddToScheme,
		appsv1.AddToScheme,
		rbacv1.AddToScheme,
		batchv1.AddToScheme,
		netv1.AddToScheme,
		storv1.AddToScheme,
		apiextensionsv1.AddToScheme,
		cmacme.AddToScheme,
		certv1.AddToScheme,
		cmmeta.AddToScheme,
		fluxv1.AddToScheme,
		helmv2.AddToScheme,
		imagev1.AddToScheme,
		kustv1.AddToScheme,
		notificationv1.AddToScheme,
		notificationv1beta2.AddToScheme,
		sourcev1.AddToScheme,
		sourcev1beta2.AddToScheme,
		esv1.AddToScheme,
		metallbv1beta1.AddToScheme,
	}

	// Register each scheme, returning the first error
	for _, addScheme := range schemeFuncs {
		if err := addScheme(Scheme); err != nil {
			return err
		}
	}

	return nil
}
