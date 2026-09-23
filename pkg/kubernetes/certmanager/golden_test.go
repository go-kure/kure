package certmanager

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
)

// The fixtures under testdata were written by the config-struct layer this
// package used to carry (Certificate(&CertificateConfig{...}) and friends)
// before it was retired. Each test below builds the same object on the
// generated constructor plus the upstream struct and must reproduce that
// output byte for byte.

var update = flag.Bool("update", false, "update golden files")

func goldenTest(t *testing.T, filename string, obj client.Object) {
	t.Helper()
	objects := []*client.Object{&obj}
	got, err := kureio.EncodeObjectsToYAMLWithOptions(objects, kureio.EncodeOptions{
		KubernetesFieldOrder: true,
	})
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}
	golden := filepath.Join("testdata", filename)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("updating golden file: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n\ngot:\n%s\nwant:\n%s", golden, got, want)
	}
}

func secretKey(name, key string) cmmeta.SecretKeySelector {
	return cmmeta.SecretKeySelector{
		LocalObjectReference: cmmeta.LocalObjectReference{Name: name},
		Key:                  key,
	}
}

func TestGolden_Certificate(t *testing.T) {
	obj := CreateCertificate("api-tls", "default")
	obj.Spec = certv1.CertificateSpec{
		SecretName:  "api-tls-secret",
		IssuerRef:   cmmeta.IssuerReference{Name: "letsencrypt", Kind: "ClusterIssuer", Group: "cert-manager.io"},
		DNSNames:    []string{"api.example.com", "www.example.com"},
		Duration:    &metav1.Duration{Duration: 2160 * time.Hour},
		RenewBefore: &metav1.Duration{Duration: 360 * time.Hour},
	}
	goldenTest(t, "certificate.yaml", obj)
}

func TestGolden_IssuerACMEHTTP01(t *testing.T) {
	obj := CreateIssuer("letsencrypt", "cert-manager")
	SetIssuerACME(obj, &cmacme.ACMEIssuer{
		Server:     "https://acme-v02.api.letsencrypt.org/directory",
		Email:      "admin@example.com",
		PrivateKey: secretKey("letsencrypt-account-key", "tls.key"),
		Solvers: []cmacme.ACMEChallengeSolver{{
			HTTP01: &cmacme.ACMEChallengeSolverHTTP01{
				Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{
					ServiceType:      corev1.ServiceTypeNodePort,
					IngressClassName: ptr.To("nginx"),
				},
			},
		}},
	})
	goldenTest(t, "issuer-acme-http01.yaml", obj)
}

func TestGolden_IssuerACMEDNS01(t *testing.T) {
	cfToken := secretKey("cloudflare-api-token", "api-token")
	gcpSA := secretKey("clouddns-sa", "key.json")
	obj := CreateIssuer("letsencrypt-dns", "cert-manager")
	acme := &cmacme.ACMEIssuer{
		Server: "https://acme-v02.api.letsencrypt.org/directory",
		Email:  "admin@example.com",
	}
	AddACMEIssuerSolver(acme, cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{
		Cloudflare: &cmacme.ACMEIssuerDNS01ProviderCloudflare{Email: "dns@example.com", APIToken: &cfToken},
	}})
	AddACMEIssuerSolver(acme, cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{
		Route53: &cmacme.ACMEIssuerDNS01ProviderRoute53{
			Region:          "eu-west-1",
			SecretAccessKey: secretKey("route53-credentials", "secret-access-key"),
		},
	}})
	AddACMEIssuerSolver(acme, cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{
		CloudDNS: &cmacme.ACMEIssuerDNS01ProviderCloudDNS{Project: "my-project", ServiceAccount: &gcpSA},
	}})
	SetIssuerACME(obj, acme)
	goldenTest(t, "issuer-acme-dns01.yaml", obj)
}

func TestGolden_IssuerCA(t *testing.T) {
	obj := CreateIssuer("internal-ca", "default")
	SetIssuerCA(obj, &certv1.CAIssuer{SecretName: "ca-key-pair"})
	goldenTest(t, "issuer-ca.yaml", obj)
}

func TestGolden_ClusterIssuerACME(t *testing.T) {
	obj := CreateClusterIssuer("letsencrypt-prod")
	SetClusterIssuerACME(obj, &cmacme.ACMEIssuer{
		Server: "https://acme-v02.api.letsencrypt.org/directory",
		Email:  "admin@example.com",
		Solvers: []cmacme.ACMEChallengeSolver{{
			HTTP01: &cmacme.ACMEChallengeSolverHTTP01{Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{}},
		}},
	})
	goldenTest(t, "clusterissuer-acme.yaml", obj)
}

func TestGolden_ClusterIssuerCA(t *testing.T) {
	obj := CreateClusterIssuer("cluster-ca")
	SetClusterIssuerCA(obj, &certv1.CAIssuer{SecretName: "cluster-ca-key-pair"})
	goldenTest(t, "clusterissuer-ca.yaml", obj)
}
