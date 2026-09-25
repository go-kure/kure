package certmanager_test

import (
	"fmt"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/kubernetes/certmanager"
)

// The Security Model blocks of docs/ARCHITECTURE.md are generated from these
// functions (scripts/gen-doc-examples.sh).

func Example_architectureSecrets() {
	// NEVER do this - a literal secret in the program that generates the manifests
	// is a literal secret in the Git repository those manifests are committed to.
	secret := kubernetes.CreateSecret("db-credentials", "default")
	secret.Data = map[string][]byte{"password": []byte("hunter2")} // WRONG

	// CORRECT approach - name the secret, let the cluster hold the value
	cert := certmanager.CreateCertificate("tls-cert", "default")
	cert.Spec.SecretName = "tls-cert-secret" // where cert-manager writes the key
	fmt.Println(len(secret.Data), cert.Spec.SecretName)
	// Output: 1 tls-cert-secret
}

func Example_architectureSecretReference() {
	// Standard pattern for secret references
	key := cmmeta.SecretKeySelector{
		LocalObjectReference: cmmeta.LocalObjectReference{Name: "secret-name"},
		Key:                  "key-name",
	}

	// The reference is a field on the upstream struct; there is no kure setter for it
	issuer := certmanager.CreateIssuer("vault", "default")
	issuer.Spec.Vault = &certv1.VaultIssuer{
		Server: "https://vault.example.com:8200",
		Path:   "pki/sign/example",
		Auth:   certv1.VaultAuth{TokenSecretRef: &key},
	}
	fmt.Println(issuer.Spec.Vault.Auth.TokenSecretRef.Name, issuer.Spec.Vault.Auth.TokenSecretRef.Key)
	// Output: secret-name key-name
}

func Example_architectureCertificate() {
	// ACME challenge configuration. SetClusterIssuerACME assigns a pointer-typed
	// field, which is what makes it admissible sugar — it takes the *ACMEIssuer,
	// built here as an upstream struct literal, and writes it straight to the spec.
	issuer := certmanager.CreateClusterIssuer("letsencrypt")
	certmanager.SetClusterIssuerACME(issuer, &cmacme.ACMEIssuer{
		Server: "https://acme-v02.api.letsencrypt.org/directory",
		Email:  "admin@example.com",
		Solvers: []cmacme.ACMEChallengeSolver{{
			DNS01: &cmacme.ACMEChallengeSolverDNS01{
				Cloudflare: &cmacme.ACMEIssuerDNS01ProviderCloudflare{
					APIToken: &cmmeta.SecretKeySelector{
						LocalObjectReference: cmmeta.LocalObjectReference{Name: "cloudflare-api-token"},
						Key:                  "token",
					},
				},
			},
		}},
	})

	// Certificate with DNS validation. IssuerRef is a plain field; DNSNames is a
	// slice, so it keeps an appender.
	cert := certmanager.CreateCertificate("api-tls", "default")
	cert.Spec.IssuerRef = cmmeta.IssuerReference{
		Name: "letsencrypt",
		Kind: "ClusterIssuer",
	}
	certmanager.AddCertificateDNSName(cert, "api.example.com")
	fmt.Println(issuer.Spec.ACME.Solvers[0].DNS01.Cloudflare.APIToken.Name, cert.Spec.IssuerRef.Kind, cert.Spec.DNSNames)
	// Output: cloudflare-api-token ClusterIssuer [api.example.com]
}
