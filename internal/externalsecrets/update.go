package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetRefreshInterval sets the polling interval on an ExternalSecret.
func SetRefreshInterval(obj *esv1.ExternalSecret, d metav1.Duration) {
	obj.Spec.RefreshInterval = &d
}

// SetTarget sets the target secret configuration on an ExternalSecret.
func SetTarget(obj *esv1.ExternalSecret, target esv1.ExternalSecretTarget) {
	obj.Spec.Target = target
}

// AddDataFrom appends a dataFrom source to an ExternalSecret.
func AddDataFrom(obj *esv1.ExternalSecret, source esv1.ExternalSecretDataFromRemoteRef) {
	obj.Spec.DataFrom = append(obj.Spec.DataFrom, source)
}
