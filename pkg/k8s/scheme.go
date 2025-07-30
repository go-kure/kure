package k8s

import (
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storv1 "k8s.io/api/storage/v1"
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
	_ = cmacme.AddToScheme(Scheme)
	_ = cmmeta.AddToScheme(Scheme)
}
