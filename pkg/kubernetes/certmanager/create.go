package certmanager

import (
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

// AddACMEIssuerSolver appends a challenge solver to the issuer.
func AddACMEIssuerSolver(issuer *cmacme.ACMEIssuer, solver cmacme.ACMEChallengeSolver) {
	issuer.Solvers = append(issuer.Solvers, solver)
}

// Certificate converts the config to a cert-manager Certificate object.
func Certificate(cfg *CertificateConfig) *certv1.Certificate {
	if cfg == nil {
		return nil
	}
	obj := CreateCertificate(cfg.Name, cfg.Namespace)
	obj.Spec.SecretName = cfg.SecretName
	obj.Spec.IssuerRef = cfg.IssuerRef
	for _, dns := range cfg.DNSNames {
		AddCertificateDNSName(obj, dns)
	}
	if cfg.Duration != nil {
		SetCertificateDuration(obj, cfg.Duration)
	}
	if cfg.RenewBefore != nil {
		SetCertificateRenewBefore(obj, cfg.RenewBefore)
	}
	return obj
}

// Issuer converts the config to a cert-manager Issuer object.
func Issuer(cfg *IssuerConfig) *certv1.Issuer {
	if cfg == nil {
		return nil
	}
	obj := CreateIssuer(cfg.Name, cfg.Namespace)
	applyIssuerVariant(cfg.Variant, func(v *cmacme.ACMEIssuer) {
		SetIssuerACME(obj, v)
	}, func(v *certv1.CAIssuer) {
		SetIssuerCA(obj, v)
	})
	return obj
}

// ClusterIssuer converts the config to a cert-manager ClusterIssuer object.
func ClusterIssuer(cfg *ClusterIssuerConfig) *certv1.ClusterIssuer {
	if cfg == nil {
		return nil
	}
	obj := CreateClusterIssuer(cfg.Name)
	applyIssuerVariant(cfg.Variant, func(v *cmacme.ACMEIssuer) {
		SetClusterIssuerACME(obj, v)
	}, func(v *certv1.CAIssuer) {
		SetClusterIssuerCA(obj, v)
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
	acme := &cmacme.ACMEIssuer{
		Server:     cfg.Server,
		Email:      cfg.Email,
		PrivateKey: cfg.PrivateKey,
	}
	for _, s := range cfg.Solvers {
		solver := buildACMESolver(&s)
		if solver.HTTP01 != nil || solver.DNS01 != nil {
			AddACMEIssuerSolver(acme, solver)
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
			return buildHTTP01Solver(s)
		}
	case *DNS01SolverConfig:
		if s != nil {
			return buildDNS01Solver(s)
		}
	}
	return cmacme.ACMEChallengeSolver{}
}

// buildHTTP01Solver converts an HTTP01SolverConfig to an ACMEChallengeSolver.
// An empty ingress class leaves IngressClassName nil rather than pointing at
// the empty string, which cert-manager reads as a class named "".
func buildHTTP01Solver(cfg *HTTP01SolverConfig) cmacme.ACMEChallengeSolver {
	ingress := &cmacme.ACMEChallengeSolverHTTP01Ingress{ServiceType: cfg.ServiceType}
	if cfg.IngressClass != "" {
		class := cfg.IngressClass
		ingress.IngressClassName = &class
	}
	return cmacme.ACMEChallengeSolver{HTTP01: &cmacme.ACMEChallengeSolverHTTP01{Ingress: ingress}}
}

// buildDNS01Solver converts a DNS01SolverConfig to an ACMEChallengeSolver.
// Provider is a sealed sum (Cloudflare/Route53/Google); each case guards
// against typed-nil.
func buildDNS01Solver(cfg *DNS01SolverConfig) cmacme.ACMEChallengeSolver {
	switch p := cfg.Provider.(type) {
	case *CloudflareProviderConfig:
		if p != nil && p.APIToken != nil {
			token := *p.APIToken
			return cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{
				Cloudflare: &cmacme.ACMEIssuerDNS01ProviderCloudflare{
					Email:    p.Email,
					APIToken: &token,
				},
			}}
		}
	case *Route53ProviderConfig:
		if p != nil && p.SecretAccessKey != nil {
			return cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{
				Route53: &cmacme.ACMEIssuerDNS01ProviderRoute53{
					Region:          p.Region,
					SecretAccessKey: *p.SecretAccessKey,
				},
			}}
		}
	case *GoogleProviderConfig:
		if p != nil {
			return cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{
				CloudDNS: &cmacme.ACMEIssuerDNS01ProviderCloudDNS{
					Project:        p.Project,
					ServiceAccount: p.ServiceAccount,
				},
			}}
		}
	}
	return cmacme.ACMEChallengeSolver{}
}
