package certmanager

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
)

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
	obj := Certificate(&CertificateConfig{
		Name:        "api-tls",
		Namespace:   "default",
		SecretName:  "api-tls-secret",
		IssuerRef:   cmmeta.IssuerReference{Name: "letsencrypt", Kind: "ClusterIssuer", Group: "cert-manager.io"},
		DNSNames:    []string{"api.example.com", "www.example.com"},
		Duration:    &metav1.Duration{Duration: 2160 * time.Hour},
		RenewBefore: &metav1.Duration{Duration: 360 * time.Hour},
	})
	goldenTest(t, "certificate.yaml", obj)
}

func TestGolden_IssuerACMEHTTP01(t *testing.T) {
	obj := Issuer(&IssuerConfig{
		Name:      "letsencrypt",
		Namespace: "cert-manager",
		Variant: &ACMEConfig{
			Server:     "https://acme-v02.api.letsencrypt.org/directory",
			Email:      "admin@example.com",
			PrivateKey: secretKey("letsencrypt-account-key", "tls.key"),
			Solvers: []ACMESolverConfig{
				{Solver: &HTTP01SolverConfig{ServiceType: corev1.ServiceTypeNodePort, IngressClass: "nginx"}},
			},
		},
	})
	goldenTest(t, "issuer-acme-http01.yaml", obj)
}

func TestGolden_IssuerACMEDNS01(t *testing.T) {
	cfToken := secretKey("cloudflare-api-token", "api-token")
	awsKey := secretKey("route53-credentials", "secret-access-key")
	gcpSA := secretKey("clouddns-sa", "key.json")
	obj := Issuer(&IssuerConfig{
		Name:      "letsencrypt-dns",
		Namespace: "cert-manager",
		Variant: &ACMEConfig{
			Server: "https://acme-v02.api.letsencrypt.org/directory",
			Email:  "admin@example.com",
			Solvers: []ACMESolverConfig{
				{Solver: &DNS01SolverConfig{Provider: &CloudflareProviderConfig{Email: "dns@example.com", APIToken: &cfToken}}},
				{Solver: &DNS01SolverConfig{Provider: &Route53ProviderConfig{Region: "eu-west-1", SecretAccessKey: &awsKey}}},
				{Solver: &DNS01SolverConfig{Provider: &GoogleProviderConfig{Project: "my-project", ServiceAccount: &gcpSA}}},
			},
		},
	})
	goldenTest(t, "issuer-acme-dns01.yaml", obj)
}

func TestGolden_IssuerCA(t *testing.T) {
	obj := Issuer(&IssuerConfig{
		Name:      "internal-ca",
		Namespace: "default",
		Variant:   &CAConfig{SecretName: "ca-key-pair"},
	})
	goldenTest(t, "issuer-ca.yaml", obj)
}

func TestGolden_ClusterIssuerACME(t *testing.T) {
	obj := ClusterIssuer(&ClusterIssuerConfig{
		Name: "letsencrypt-prod",
		Variant: &ACMEConfig{
			Server: "https://acme-v02.api.letsencrypt.org/directory",
			Email:  "admin@example.com",
			Solvers: []ACMESolverConfig{
				{Solver: &HTTP01SolverConfig{}},
			},
		},
	})
	goldenTest(t, "clusterissuer-acme.yaml", obj)
}

func TestGolden_ClusterIssuerCA(t *testing.T) {
	obj := ClusterIssuer(&ClusterIssuerConfig{
		Name:    "cluster-ca",
		Variant: &CAConfig{SecretName: "cluster-ca-key-pair"},
	})
	goldenTest(t, "clusterissuer-ca.yaml", obj)
}
