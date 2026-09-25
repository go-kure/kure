package certmanager_test

import (
	"fmt"
	"time"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/kubernetes/certmanager"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

func ExampleCreateCertificate() {
	obj := certmanager.CreateCertificate("my-cert", "default")
	cl := certmanager.CreateClusterIssuer("letsencrypt-prod")
	fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name, cl.Kind, cl.Name)
	// Output: Certificate default/my-cert ClusterIssuer letsencrypt-prod
}

func ExampleCreateCertificate_spec() {
	cert := certmanager.CreateCertificate("my-cert", "default")
	cert.Spec = certv1.CertificateSpec{
		SecretName: "my-cert-tls",
		IssuerRef:  cmmeta.IssuerReference{Name: "letsencrypt", Kind: "ClusterIssuer"},
		DNSNames:   []string{"example.com", "www.example.com"},
		Duration:   &metav1.Duration{Duration: 2160 * time.Hour},
	}
	fmt.Println(cert.Spec.SecretName, cert.Spec.DNSNames, cert.Spec.Duration.Duration)
	// Output: my-cert-tls [example.com www.example.com] 2160h0m0s
}

func ExampleSetIssuerACME() {
	issuer := certmanager.CreateIssuer("letsencrypt", "default")
	certmanager.SetIssuerACME(issuer, &cmacme.ACMEIssuer{
		Server: "https://acme-v02.api.letsencrypt.org/directory",
		Email:  "admin@example.com",
		PrivateKey: cmmeta.SecretKeySelector{
			LocalObjectReference: cmmeta.LocalObjectReference{Name: "letsencrypt-account-key"},
		},
		Solvers: []cmacme.ACMEChallengeSolver{{
			HTTP01: &cmacme.ACMEChallengeSolverHTTP01{
				Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{IngressClassName: ptr.To("nginx")},
			},
		}},
	})
	fmt.Println(issuer.Spec.ACME.Email, *issuer.Spec.ACME.Solvers[0].HTTP01.Ingress.IngressClassName)
	// Output: admin@example.com nginx
}

func ExampleAddACMEIssuerSolver() {
	issuer := certmanager.CreateIssuer("letsencrypt", "default")

	acme := &cmacme.ACMEIssuer{Server: "https://acme-v02.api.letsencrypt.org/directory", Email: "admin@example.com"}
	certmanager.AddACMEIssuerSolver(acme, cmacme.ACMEChallengeSolver{
		DNS01: &cmacme.ACMEChallengeSolverDNS01{
			Cloudflare: &cmacme.ACMEIssuerDNS01ProviderCloudflare{
				Email: "admin@example.com",
				APIToken: &cmmeta.SecretKeySelector{
					LocalObjectReference: cmmeta.LocalObjectReference{Name: "cf-api-token"},
					Key:                  "api-token",
				},
			},
		},
	})
	certmanager.SetIssuerACME(issuer, acme)
	fmt.Println(issuer.Spec.ACME.Solvers[0].DNS01.Cloudflare.APIToken.Name)
	// Output: cf-api-token
}

func ExampleCreateIssuer_otherArms() {
	vault := certmanager.CreateIssuer("vault", "default")
	vault.Spec.Vault = &certv1.VaultIssuer{Server: "https://vault.example.com", Path: "pki/sign/example"}

	selfSigned := certmanager.CreateIssuer("self-signed", "default")
	selfSigned.Spec.SelfSigned = &certv1.SelfSignedIssuer{}
	fmt.Println(vault.Spec.Vault.Path, selfSigned.Spec.SelfSigned != nil)
	// Output: pki/sign/example true
}

func ExampleSetClusterIssuerCA() {
	clusterIssuer := certmanager.CreateClusterIssuer("letsencrypt-prod")
	certmanager.SetClusterIssuerCA(clusterIssuer, &certv1.CAIssuer{SecretName: "ca-key-pair"})
	fmt.Println(clusterIssuer.Spec.CA.SecretName)
	// Output: ca-key-pair
}

func ExampleAddCertificateDNSName() {
	cert := certmanager.CreateCertificate("my-cert", "default")
	issuer := certmanager.CreateIssuer("letsencrypt", "default")
	caIssuer := certmanager.CreateIssuer("internal-ca", "default")
	clusterIssuer := certmanager.CreateClusterIssuer("letsencrypt-prod")
	caClusterIssuer := certmanager.CreateClusterIssuer("internal-ca")
	acmeConfig := &cmacme.ACMEIssuer{Server: "https://acme-v02.api.letsencrypt.org/directory"}
	caConfig := &certv1.CAIssuer{SecretName: "ca-key-pair"}

	// Replace the whole Certificate spec
	cert.Spec = certv1.CertificateSpec{SecretName: "my-cert-tls", DNSNames: []string{"example.com"}}

	// Append to the DNS names, set the pointer-typed durations
	certmanager.AddCertificateDNSName(cert, "alt.example.com")
	certmanager.SetCertificateDuration(cert, &metav1.Duration{Duration: 2160 * time.Hour})
	certmanager.SetCertificateRenewBefore(cert, &metav1.Duration{Duration: 360 * time.Hour})

	// Labels and annotations use the generic helpers, which work over any object
	// with ObjectMeta -- this package carries no per-kind metadata helpers
	kubernetes.AddLabel(cert, "app", "my-app")
	kubernetes.AddAnnotation(issuer, "note", "production")

	// Update issuer configuration: one arm per issuer
	certmanager.SetIssuerACME(issuer, acmeConfig)
	certmanager.SetIssuerCA(caIssuer, caConfig)
	certmanager.SetClusterIssuerACME(clusterIssuer, acmeConfig)
	certmanager.SetClusterIssuerCA(caClusterIssuer, caConfig)

	fmt.Println(cert.Spec.DNSNames, cert.Spec.RenewBefore.Duration, caIssuer.Spec.CA.SecretName)
	// Output: [example.com alt.example.com] 360h0m0s ca-key-pair
}
