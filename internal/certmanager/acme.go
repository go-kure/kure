package certmanager

import (
	corev1 "k8s.io/api/core/v1"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

// CreateACMEIssuer returns an ACMEIssuer with the mandatory fields set.
func CreateACMEIssuer(server, email string, key cmmeta.SecretKeySelector) *cmacme.ACMEIssuer {
	return &cmacme.ACMEIssuer{
		Email:                       email,
		Server:                      server,
		PrivateKey:                  key,
		Solvers:                     []cmacme.ACMEChallengeSolver{},
		SkipTLSVerify:               false,
		DisableAccountKeyGeneration: false,
	}
}

// AddACMEIssuerSolver appends a challenge solver to the issuer.
func AddACMEIssuerSolver(issuer *cmacme.ACMEIssuer, solver cmacme.ACMEChallengeSolver) {
	issuer.Solvers = append(issuer.Solvers, solver)
}

// CreateACMEHTTP01Solver creates a solver using HTTP01 via ingress class.
func CreateACMEHTTP01Solver(serviceType corev1.ServiceType, class string) cmacme.ACMEChallengeSolver {
	solver := cmacme.ACMEChallengeSolver{
		HTTP01: &cmacme.ACMEChallengeSolverHTTP01{
			Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{
				ServiceType: serviceType,
			},
		},
	}
	if class != "" {
		solver.HTTP01.Ingress.IngressClassName = &class
	}
	return solver
}

// CreateACMEDNS01SolverCloudflare creates a DNS01 solver for Cloudflare.
func CreateACMEDNS01SolverCloudflare(email string, token cmmeta.SecretKeySelector) cmacme.ACMEChallengeSolver {
	provider := &cmacme.ACMEChallengeSolverDNS01{
		Cloudflare: &cmacme.ACMEIssuerDNS01ProviderCloudflare{
			Email:    email,
			APIToken: &token,
		},
	}
	return cmacme.ACMEChallengeSolver{DNS01: provider}
}

// CreateACMEDNS01SolverRoute53 creates a DNS01 solver for AWS Route53.
func CreateACMEDNS01SolverRoute53(region string, key cmmeta.SecretKeySelector) cmacme.ACMEChallengeSolver {
	provider := &cmacme.ACMEChallengeSolverDNS01{
		Route53: &cmacme.ACMEIssuerDNS01ProviderRoute53{
			Region:          region,
			SecretAccessKey: key,
		},
	}
	return cmacme.ACMEChallengeSolver{DNS01: provider}
}

// CreateACMEDNS01SolverGoogle creates a DNS01 solver for Google CloudDNS.
func CreateACMEDNS01SolverGoogle(project string, sa *cmmeta.SecretKeySelector) cmacme.ACMEChallengeSolver {
	provider := &cmacme.ACMEChallengeSolverDNS01{
		CloudDNS: &cmacme.ACMEIssuerDNS01ProviderCloudDNS{
			Project:        project,
			ServiceAccount: sa,
		},
	}
	return cmacme.ACMEChallengeSolver{DNS01: provider}
}
