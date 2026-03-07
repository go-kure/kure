package certmanager

import (
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	intcm "github.com/go-kure/kure/internal/certmanager"
)

// Certificate converts the config to a cert-manager Certificate object.
func Certificate(cfg *CertificateConfig) *certv1.Certificate {
	if cfg == nil {
		return nil
	}
	obj := intcm.CreateCertificate(cfg.Name, cfg.Namespace, certv1.CertificateSpec{
		SecretName: cfg.SecretName,
	})
	intcm.SetCertificateIssuerRef(obj, cfg.IssuerRef) //nolint:errcheck,gosec // obj is freshly created
	for _, dns := range cfg.DNSNames {
		intcm.AddCertificateDNSName(obj, dns) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.Duration != nil {
		intcm.SetCertificateDuration(obj, cfg.Duration) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.RenewBefore != nil {
		intcm.SetCertificateRenewBefore(obj, cfg.RenewBefore) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}

// Issuer converts the config to a cert-manager Issuer object.
func Issuer(cfg *IssuerConfig) *certv1.Issuer {
	if cfg == nil {
		return nil
	}
	obj := intcm.CreateIssuer(cfg.Name, cfg.Namespace, certv1.IssuerSpec{})
	if cfg.ACME != nil {
		acme := buildACMEIssuer(cfg.ACME)
		intcm.SetIssuerACME(obj, acme) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.CA != nil {
		intcm.SetIssuerCA(obj, &certv1.CAIssuer{SecretName: cfg.CA.SecretName}) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}

// ClusterIssuer converts the config to a cert-manager ClusterIssuer object.
func ClusterIssuer(cfg *ClusterIssuerConfig) *certv1.ClusterIssuer {
	if cfg == nil {
		return nil
	}
	obj := intcm.CreateClusterIssuer(cfg.Name, certv1.IssuerSpec{})
	if cfg.ACME != nil {
		acme := buildACMEIssuer(cfg.ACME)
		intcm.SetClusterIssuerACME(obj, acme) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.CA != nil {
		obj.Spec.IssuerConfig.CA = &certv1.CAIssuer{SecretName: cfg.CA.SecretName}
	}
	return obj
}

// buildACMEIssuer converts an ACMEConfig to an ACMEIssuer, including solvers.
func buildACMEIssuer(cfg *ACMEConfig) *cmacme.ACMEIssuer {
	acme := intcm.CreateACMEIssuer(cfg.Server, cfg.Email, cfg.PrivateKey)
	for _, s := range cfg.Solvers {
		solver := buildACMESolver(&s)
		intcm.AddACMEIssuerSolver(acme, solver)
	}
	return acme
}

// buildACMESolver converts an ACMESolverConfig to an ACMEChallengeSolver.
func buildACMESolver(cfg *ACMESolverConfig) cmacme.ACMEChallengeSolver {
	if cfg.HTTP01 != nil {
		return intcm.CreateACMEHTTP01Solver(cfg.HTTP01.ServiceType, cfg.HTTP01.IngressClass)
	}
	if cfg.DNS01 != nil {
		return buildDNS01Solver(cfg.DNS01)
	}
	return cmacme.ACMEChallengeSolver{}
}

// buildDNS01Solver converts a DNS01SolverConfig to an ACMEChallengeSolver.
func buildDNS01Solver(cfg *DNS01SolverConfig) cmacme.ACMEChallengeSolver {
	switch cfg.Provider {
	case "cloudflare":
		if cfg.APIToken != nil {
			return intcm.CreateACMEDNS01SolverCloudflare(cfg.Email, *cfg.APIToken)
		}
	case "route53":
		if cfg.SecretAccessKey != nil {
			return intcm.CreateACMEDNS01SolverRoute53(cfg.Region, *cfg.SecretAccessKey)
		}
	case "clouddns":
		return intcm.CreateACMEDNS01SolverGoogle(cfg.Project, cfg.ServiceAccount)
	}
	return cmacme.ACMEChallengeSolver{}
}
