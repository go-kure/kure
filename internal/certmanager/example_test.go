package certmanager_test

import (
	"fmt"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/internal/certmanager"
)

// This example demonstrates composing a ClusterIssuer with an ACME
// configuration and a matching Certificate, which is a common pattern
// for automated TLS in Kubernetes.
func Example_composeClusterIssuerAndCertificate() {
	// --- ClusterIssuer with ACME (Let's Encrypt) ---
	privateKey := cmmeta.SecretKeySelector{
		LocalObjectReference: cmmeta.LocalObjectReference{Name: "letsencrypt-account-key"},
		Key:                  "tls.key",
	}
	acme := certmanager.CreateACMEIssuer(
		"https://acme-v02.api.letsencrypt.org/directory",
		"admin@example.com",
		privateKey,
	)

	solver := certmanager.CreateACMEHTTP01Solver(corev1.ServiceTypeClusterIP, "nginx")
	certmanager.AddACMEIssuerSolver(acme, solver)

	issuer := certmanager.CreateClusterIssuer("letsencrypt-prod", certv1.IssuerSpec{})
	certmanager.SetClusterIssuerACME(issuer, acme)
	certmanager.AddClusterIssuerLabel(issuer, "env", "production")

	// --- Certificate referencing the ClusterIssuer ---
	cert := certmanager.CreateCertificate("app-tls", "default", certv1.CertificateSpec{
		SecretName: "app-tls-secret",
	})
	certmanager.SetCertificateIssuerRef(cert, cmmeta.ObjectReference{
		Name:  issuer.Name,
		Kind:  "ClusterIssuer",
		Group: "cert-manager.io",
	})
	certmanager.AddCertificateDNSName(cert, "app.example.com")
	certmanager.AddCertificateDNSName(cert, "www.example.com")
	certmanager.AddCertificateLabel(cert, "env", "production")

	fmt.Println("Issuer:", issuer.Name)
	fmt.Println("Issuer Kind:", issuer.Kind)
	fmt.Println("ACME Server:", issuer.Spec.ACME.Server)
	fmt.Println("Certificate:", cert.Name)
	fmt.Println("Certificate Namespace:", cert.Namespace)
	fmt.Println("Secret:", cert.Spec.SecretName)
	fmt.Println("DNS Names:", cert.Spec.DNSNames)
	fmt.Println("Issuer Ref:", cert.Spec.IssuerRef.Name)
	// Output:
	// Issuer: letsencrypt-prod
	// Issuer Kind: ClusterIssuer
	// ACME Server: https://acme-v02.api.letsencrypt.org/directory
	// Certificate: app-tls
	// Certificate Namespace: default
	// Secret: app-tls-secret
	// DNS Names: [app.example.com www.example.com]
	// Issuer Ref: letsencrypt-prod
}
