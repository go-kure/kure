package certmanager

import (
	"testing"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// TestMetadataViaGenericHelpers covers what the per-kind
// Add<Kind>Label/Add<Kind>Annotation helpers used to do for every kind in this
// package: the generic helpers work over metav1.Object, so one pair reaches all
// three.
func TestMetadataViaGenericHelpers(t *testing.T) {
	cert := CreateCertificate("test-cert", "default")
	kubernetes.AddLabel(cert, "app", "test")
	kubernetes.AddAnnotation(cert, "note", "value")
	if cert.Labels["app"] != "test" || cert.Annotations["note"] != "value" {
		t.Errorf("Certificate metadata: labels=%v annotations=%v", cert.Labels, cert.Annotations)
	}

	issuer := CreateIssuer("test-issuer", "default")
	kubernetes.AddLabel(issuer, "env", "prod")
	kubernetes.AddAnnotation(issuer, "example.com/key", "value")
	if issuer.Labels["env"] != "prod" || issuer.Annotations["example.com/key"] != "value" {
		t.Errorf("Issuer metadata: labels=%v annotations=%v", issuer.Labels, issuer.Annotations)
	}

	ci := CreateClusterIssuer("test-cluster-issuer")
	kubernetes.AddLabel(ci, "env", "prod")
	kubernetes.AddAnnotation(ci, "example.com/key", "value")
	if ci.Labels["env"] != "prod" || ci.Annotations["example.com/key"] != "value" {
		t.Errorf("ClusterIssuer metadata: labels=%v annotations=%v", ci.Labels, ci.Annotations)
	}
}

func TestAddCertificateDNSName(t *testing.T) {
	cert := CreateCertificate("test-cert", "default")
	AddCertificateDNSName(cert, "example.com")
	AddCertificateDNSName(cert, "www.example.com")
	if len(cert.Spec.DNSNames) != 2 || cert.Spec.DNSNames[1] != "www.example.com" {
		t.Errorf("expected two DNS names in order, got %v", cert.Spec.DNSNames)
	}
}

func TestSetCertificateDuration(t *testing.T) {
	cert := CreateCertificate("test-cert", "default")
	dur := &metav1.Duration{Duration: 720 * 3600_000_000_000} // 720h
	SetCertificateDuration(cert, dur)
	if cert.Spec.Duration == nil || cert.Spec.Duration.Duration != dur.Duration {
		t.Errorf("expected Duration %v, got %v", dur.Duration, cert.Spec.Duration)
	}
}

func TestSetCertificateRenewBefore(t *testing.T) {
	cert := CreateCertificate("test-cert", "default")
	dur := &metav1.Duration{Duration: 24 * 3600_000_000_000} // 24h
	SetCertificateRenewBefore(cert, dur)
	if cert.Spec.RenewBefore == nil || cert.Spec.RenewBefore.Duration != dur.Duration {
		t.Errorf("expected RenewBefore %v, got %v", dur.Duration, cert.Spec.RenewBefore)
	}
}

func TestSetIssuerACME(t *testing.T) {
	issuer := CreateIssuer("test-issuer", "default")
	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	SetIssuerACME(issuer, acme)
	if issuer.Spec.IssuerConfig.ACME == nil || issuer.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Error("expected ACME config to be set")
	}
}

func TestSetIssuerCA(t *testing.T) {
	issuer := CreateIssuer("test-issuer", "default")
	ca := &certv1.CAIssuer{SecretName: "ca-secret"}
	SetIssuerCA(issuer, ca)
	if issuer.Spec.IssuerConfig.CA == nil || issuer.Spec.IssuerConfig.CA.SecretName != "ca-secret" {
		t.Error("expected CA config to be set")
	}
}

func TestSetClusterIssuerACME(t *testing.T) {
	ci := CreateClusterIssuer("test-cluster-issuer")
	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	SetClusterIssuerACME(ci, acme)
	if ci.Spec.IssuerConfig.ACME == nil || ci.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Error("expected ACME config to be set")
	}
}

func TestSetClusterIssuerCA(t *testing.T) {
	ci := CreateClusterIssuer("test-cluster-issuer")
	ca := &certv1.CAIssuer{SecretName: "ca-secret"}
	SetClusterIssuerCA(ci, ca)
	if ci.Spec.IssuerConfig.CA == nil || ci.Spec.IssuerConfig.CA.SecretName != "ca-secret" {
		t.Error("expected CA config to be set")
	}
}

func TestAddACMEIssuerSolver(t *testing.T) {
	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	AddACMEIssuerSolver(acme, cmacme.ACMEChallengeSolver{HTTP01: &cmacme.ACMEChallengeSolverHTTP01{}})
	AddACMEIssuerSolver(acme, cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{}})
	if len(acme.Solvers) != 2 || acme.Solvers[0].HTTP01 == nil || acme.Solvers[1].DNS01 == nil {
		t.Errorf("expected an HTTP-01 then a DNS-01 solver, got %+v", acme.Solvers)
	}
}
