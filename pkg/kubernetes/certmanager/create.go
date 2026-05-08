package certmanager

import (
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCertificate returns a new Certificate with TypeMeta and ObjectMeta set.
func CreateCertificate(name, namespace string) *certv1.Certificate {
	return &certv1.Certificate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Certificate",
			APIVersion: certv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateIssuer returns a new Issuer with TypeMeta and ObjectMeta set.
func CreateIssuer(name, namespace string) *certv1.Issuer {
	return &certv1.Issuer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Issuer",
			APIVersion: certv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateClusterIssuer returns a new ClusterIssuer with TypeMeta and ObjectMeta set.
// ClusterIssuer is cluster-scoped so namespace is not set.
func CreateClusterIssuer(name string) *certv1.ClusterIssuer {
	return &certv1.ClusterIssuer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterIssuer",
			APIVersion: certv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

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

// Certificate converts the config to a cert-manager Certificate object.
func Certificate(cfg *CertificateConfig) *certv1.Certificate {
	if cfg == nil {
		return nil
	}
	obj := CreateCertificate(cfg.Name, cfg.Namespace)
	obj.Spec.SecretName = cfg.SecretName
	SetCertificateIssuerRef(obj, cfg.IssuerRef)
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
	acme := CreateACMEIssuer(cfg.Server, cfg.Email, cfg.PrivateKey)
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
			return CreateACMEHTTP01Solver(s.ServiceType, s.IngressClass)
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
			return CreateACMEDNS01SolverCloudflare(p.Email, *p.APIToken)
		}
	case *Route53ProviderConfig:
		if p != nil && p.SecretAccessKey != nil {
			return CreateACMEDNS01SolverRoute53(p.Region, *p.SecretAccessKey)
		}
	case *GoogleProviderConfig:
		if p != nil {
			return CreateACMEDNS01SolverGoogle(p.Project, p.ServiceAccount)
		}
	}
	return cmacme.ACMEChallengeSolver{}
}
