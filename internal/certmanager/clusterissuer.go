package certmanager

import (
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateClusterIssuer returns a new ClusterIssuer with the provided name and spec.
func CreateClusterIssuer(name string, spec certv1.IssuerSpec) *certv1.ClusterIssuer {
	obj := &certv1.ClusterIssuer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterIssuer",
			APIVersion: certv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}
	return obj
}

// AddClusterIssuerLabel adds or updates a label on the ClusterIssuer metadata.
func AddClusterIssuerLabel(obj *certv1.ClusterIssuer, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddClusterIssuerAnnotation adds or updates an annotation on the ClusterIssuer metadata.
func AddClusterIssuerAnnotation(obj *certv1.ClusterIssuer, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// SetClusterIssuerACME sets the ACME config on the ClusterIssuer.
func SetClusterIssuerACME(obj *certv1.ClusterIssuer, acme *cmacme.ACMEIssuer) {
	obj.Spec.IssuerConfig.ACME = acme
}
