package k8s

import (
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
	Scheme = runtime.NewScheme()
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	// Register core Kubernetes types
	_ = corev1.AddToScheme(Scheme)
	_ = appsv1.AddToScheme(Scheme)
	_ = rbacv1.AddToScheme(Scheme)
	_ = batchv1.AddToScheme(Scheme)
	_ = netv1.AddToScheme(Scheme)
	_ = storv1.AddToScheme(Scheme)
	_ = apiextensionsv1.AddToScheme(Scheme)
	_ = cmacme.AddToScheme(Scheme)
	_ = certv1.AddToScheme(Scheme)
	_ = cmmeta.AddToScheme(Scheme)
	_ = fluxv1.AddToScheme(Scheme)
	_ = helmv2.AddToScheme(Scheme)
	_ = imagev1.AddToScheme(Scheme)
	_ = kustv1.AddToScheme(Scheme)
	_ = notificationv1.AddToScheme(Scheme)
	_ = notificationv1beta2.AddToScheme(Scheme)
	_ = sourcev1.AddToScheme(Scheme)
	_ = sourcev1beta2.AddToScheme(Scheme)
	_ = esv1.AddToScheme(Scheme)
	_ = metallbv1beta1.AddToScheme(Scheme)
}
