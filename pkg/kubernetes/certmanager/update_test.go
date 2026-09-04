package certmanager

import (
	"testing"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// TestMetadataViaGenericHelpers covers what the per-kind
// Add<Kind>Label/Add<Kind>Annotation helpers used to do for every kind in this
// package: the generic helpers work over metav1.Object, so one pair reaches all
// three.
func TestMetadataViaGenericHelpers(t *testing.T) {
	cert := Certificate(&CertificateConfig{
		Name:       "test-cert",
		Namespace:  "default",
		SecretName: "tls",
		IssuerRef:  cmmeta.IssuerReference{Name: "issuer"},
	})
	kubernetes.AddLabel(cert, "app", "test")
	kubernetes.AddAnnotation(cert, "note", "value")
	if cert.Labels["app"] != "test" || cert.Annotations["note"] != "value" {
		t.Errorf("Certificate metadata: labels=%v annotations=%v", cert.Labels, cert.Annotations)
	}

	issuer := Issuer(&IssuerConfig{Name: "test-issuer", Namespace: "default"})
	kubernetes.AddLabel(issuer, "env", "prod")
	kubernetes.AddAnnotation(issuer, "example.com/key", "value")
	if issuer.Labels["env"] != "prod" || issuer.Annotations["example.com/key"] != "value" {
		t.Errorf("Issuer metadata: labels=%v annotations=%v", issuer.Labels, issuer.Annotations)
	}

	ci := ClusterIssuer(&ClusterIssuerConfig{Name: "test-cluster-issuer"})
	kubernetes.AddLabel(ci, "env", "prod")
	kubernetes.AddAnnotation(ci, "example.com/key", "value")
	if ci.Labels["env"] != "prod" || ci.Annotations["example.com/key"] != "value" {
		t.Errorf("ClusterIssuer metadata: labels=%v annotations=%v", ci.Labels, ci.Annotations)
	}
}

func TestSetCertificateDuration(t *testing.T) {
	cfg := &CertificateConfig{
		Name:       "test-cert",
		Namespace:  "default",
		SecretName: "tls",
		IssuerRef:  cmmeta.IssuerReference{Name: "issuer"},
	}
	cert := Certificate(cfg)

	dur := &metav1.Duration{Duration: 720 * 3600_000_000_000} // 720h
	SetCertificateDuration(cert, dur)
}

func TestSetIssuerACME(t *testing.T) {
	cfg := &IssuerConfig{
		Name:      "test-issuer",
		Namespace: "default",
	}
	issuer := Issuer(cfg)

	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	SetIssuerACME(issuer, acme)
	if issuer.Spec.IssuerConfig.ACME == nil || issuer.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Error("expected ACME config to be set")
	}
}

func TestSetIssuerCA(t *testing.T) {
	cfg := &IssuerConfig{
		Name:      "test-issuer",
		Namespace: "default",
	}
	issuer := Issuer(cfg)

	ca := &certv1.CAIssuer{SecretName: "ca-secret"}
	SetIssuerCA(issuer, ca)
	if issuer.Spec.IssuerConfig.CA == nil || issuer.Spec.IssuerConfig.CA.SecretName != "ca-secret" {
		t.Error("expected CA config to be set")
	}
}

func TestSetClusterIssuerACME(t *testing.T) {
	cfg := &ClusterIssuerConfig{
		Name: "test-cluster-issuer",
	}
	ci := ClusterIssuer(cfg)

	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	SetClusterIssuerACME(ci, acme)
	if ci.Spec.IssuerConfig.ACME == nil || ci.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Error("expected ACME config to be set")
	}
}

func TestSetClusterIssuerCA(t *testing.T) {
	cfg := &ClusterIssuerConfig{
		Name: "test-cluster-issuer",
	}
	ci := ClusterIssuer(cfg)

	ca := &certv1.CAIssuer{SecretName: "ca-secret"}
	SetClusterIssuerCA(ci, ca)
	if ci.Spec.IssuerConfig.CA == nil || ci.Spec.IssuerConfig.CA.SecretName != "ca-secret" {
		t.Error("expected CA config to be set")
	}
}

func TestSetCertificateRenewBefore(t *testing.T) {
	cfg := &CertificateConfig{
		Name:       "test-cert",
		Namespace:  "default",
		SecretName: "tls",
		IssuerRef:  cmmeta.IssuerReference{Name: "issuer"},
	}
	cert := Certificate(cfg)

	dur := &metav1.Duration{Duration: 24 * 3600_000_000_000} // 24h
	SetCertificateRenewBefore(cert, dur)
	if cert.Spec.RenewBefore == nil || cert.Spec.RenewBefore.Duration != dur.Duration {
		t.Errorf("expected RenewBefore %v, got %v", dur.Duration, cert.Spec.RenewBefore)
	}
}

func TestIssuerVariant_MarkerMethods(t *testing.T) {
	var v IssuerVariant
	v = &ACMEConfig{}
	v.isIssuerVariant()
	v = &CAConfig{}
	v.isIssuerVariant()
	_ = v
}

func TestACMESolver_MarkerMethods(t *testing.T) {
	var s ACMESolver
	s = &HTTP01SolverConfig{}
	s.isACMESolver()
	s = &DNS01SolverConfig{}
	s.isACMESolver()
	_ = s
}

func TestDNS01Provider_MarkerMethods(t *testing.T) {
	var p DNS01Provider
	p = &CloudflareProviderConfig{}
	p.isDNS01Provider()
	p = &Route53ProviderConfig{}
	p.isDNS01Provider()
	p = &GoogleProviderConfig{}
	p.isDNS01Provider()
	_ = p
}
