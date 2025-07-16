package certmanager

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

func TestACMEHelpers(t *testing.T) {
	key := cmmeta.SecretKeySelector{LocalObjectReference: cmmeta.LocalObjectReference{Name: "key"}}
	acmeIssuer := CreateACMEIssuer("https://acme.example.com", "test@example.com", key)

	if acmeIssuer.Server != "https://acme.example.com" || acmeIssuer.Email != "test@example.com" {
		t.Fatalf("unexpected issuer fields")
	}

	solver := CreateACMEHTTP01Solver(corev1.ServiceTypeNodePort, "nginx")
	AddACMEIssuerSolver(acmeIssuer, solver)
	if len(acmeIssuer.Solvers) != 1 || acmeIssuer.Solvers[0].HTTP01 == nil {
		t.Fatalf("solver not added")
	}

	cfTok := cmmeta.SecretKeySelector{LocalObjectReference: cmmeta.LocalObjectReference{Name: "cloudflare"}}
	dnsSolver := CreateACMEDNS01SolverCloudflare("me@example.com", cfTok)
	AddACMEIssuerSolver(acmeIssuer, dnsSolver)
	if acmeIssuer.Solvers[1].DNS01 == nil || acmeIssuer.Solvers[1].DNS01.Cloudflare == nil {
		t.Fatalf("dns solver not added")
	}

	route53Key := cmmeta.SecretKeySelector{LocalObjectReference: cmmeta.LocalObjectReference{Name: "aws"}}
	r53 := CreateACMEDNS01SolverRoute53("us-east-1", route53Key)
	AddACMEIssuerSolver(acmeIssuer, r53)
	if acmeIssuer.Solvers[2].DNS01.Route53.Region != "us-east-1" {
		t.Fatalf("route53 region mismatch")
	}

	gsa := cmmeta.SecretKeySelector{LocalObjectReference: cmmeta.LocalObjectReference{Name: "gcp"}}
	gsolver := CreateACMEDNS01SolverGoogle("my-project", &gsa)
	if gsolver.DNS01.CloudDNS == nil || gsolver.DNS01.CloudDNS.Project != "my-project" {
		t.Fatalf("gcp solver not configured")
	}
}
