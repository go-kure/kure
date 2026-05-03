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
	intcm.SetCertificateIssuerRef(obj, cfg.IssuerRef)
	for _, dns := range cfg.DNSNames {
		intcm.AddCertificateDNSName(obj, dns)
	}
	if cfg.Duration != nil {
		intcm.SetCertificateDuration(obj, cfg.Duration)
	}
	if cfg.RenewBefore != nil {
		intcm.SetCertificateRenewBefore(obj, cfg.RenewBefore)
	}
	return obj
}

// Issuer converts the config to a cert-manager Issuer object.
func Issuer(cfg *IssuerConfig) *certv1.Issuer {
	if cfg == nil {
		return nil
	}
	obj := intcm.CreateIssuer(cfg.Name, cfg.Namespace, certv1.IssuerSpec{})
	applyIssuerVariant(cfg.Variant, func(v *cmacme.ACMEIssuer) {
		intcm.SetIssuerACME(obj, v)
	}, func(v *certv1.CAIssuer) {
		intcm.SetIssuerCA(obj, v)
	})
	return obj
}

// ClusterIssuer converts the config to a cert-manager ClusterIssuer object.
func ClusterIssuer(cfg *ClusterIssuerConfig) *certv1.ClusterIssuer {
	if cfg == nil {
		return nil
	}
	obj := intcm.CreateClusterIssuer(cfg.Name, certv1.IssuerSpec{})
	applyIssuerVariant(cfg.Variant, func(v *cmacme.ACMEIssuer) {
		intcm.SetClusterIssuerACME(obj, v)
	}, func(v *certv1.CAIssuer) {
		intcm.SetClusterIssuerCA(obj, v)
	})
	return obj
}

// applyIssuerVariant dispatches on the IssuerVariant sum and invokes the
// matching setter. Each case guards against typed-nil pointers stored in the
// interface (a `var v *ACMEConfig` would match the case but `*v` would panic);
// typed-nil is treated as "no variant set" — same effective behaviour as a
// nil interface.
func applyIssuerVariant(v IssuerVariant, setACME func(*cmacme.ACMEIssuer), setCA func(*certv1.CAIssuer)) {
	switch x := v.(type) {
	case *ACMEConfig:
		if x != nil {
			setACME(buildACMEIssuer(x))
		}
	case *CAConfig:
		if x != nil {
			setCA(&certv1.CAIssuer{SecretName: x.SecretName})
		}
	}
}

// buildACMEIssuer converts an ACMEConfig to an ACMEIssuer, including solvers.
func buildACMEIssuer(cfg *ACMEConfig) *cmacme.ACMEIssuer {
	acme := intcm.CreateACMEIssuer(cfg.Server, cfg.Email, cfg.PrivateKey)
	for _, s := range cfg.Solvers {
		solver := buildACMESolver(&s)
		if solver.HTTP01 != nil || solver.DNS01 != nil {
			intcm.AddACMEIssuerSolver(acme, solver)
		}
	}
	return acme
}

// buildACMESolver converts an ACMESolverConfig to an ACMEChallengeSolver.
// The Solver field is a sealed sum (HTTP01SolverConfig or DNS01SolverConfig);
// each case guards against typed-nil.
func buildACMESolver(cfg *ACMESolverConfig) cmacme.ACMEChallengeSolver {
	if cfg == nil {
		return cmacme.ACMEChallengeSolver{}
	}
	switch s := cfg.Solver.(type) {
	case *HTTP01SolverConfig:
		if s != nil {
			return intcm.CreateACMEHTTP01Solver(s.ServiceType, s.IngressClass)
		}
	case *DNS01SolverConfig:
		if s != nil {
			return buildDNS01Solver(s)
		}
	}
	return cmacme.ACMEChallengeSolver{}
}

// buildDNS01Solver converts a DNS01SolverConfig to an ACMEChallengeSolver.
// Provider is a sealed sum (Cloudflare/Route53/Google); each case guards
// against typed-nil.
func buildDNS01Solver(cfg *DNS01SolverConfig) cmacme.ACMEChallengeSolver {
	switch p := cfg.Provider.(type) {
	case *CloudflareProviderConfig:
		if p != nil && p.APIToken != nil {
			return intcm.CreateACMEDNS01SolverCloudflare(p.Email, *p.APIToken)
		}
	case *Route53ProviderConfig:
		if p != nil && p.SecretAccessKey != nil {
			return intcm.CreateACMEDNS01SolverRoute53(p.Region, *p.SecretAccessKey)
		}
	case *GoogleProviderConfig:
		if p != nil {
			return intcm.CreateACMEDNS01SolverGoogle(p.Project, p.ServiceAccount)
		}
	}
	return cmacme.ACMEChallengeSolver{}
}
