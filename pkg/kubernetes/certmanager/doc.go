// Package certmanager exposes the generated constructors and the admissible
// sugar for cert-manager resources: Certificate, Issuer and ClusterIssuer.
// Each constructor returns a controller-runtime object carrying identity
// only; the upstream cert-manager struct is the construction API.
//
// ## Constructors
//
// Create<Kind> is generated from the registered scheme and emits apiVersion,
// kind, metadata.name and, for a namespaced kind, metadata.namespace. Spec
// fields are the caller's own assignments:
//
//	cert := certmanager.CreateCertificate("my-cert", "default")
//	cert.Spec = certv1.CertificateSpec{
//	        SecretName: "my-cert-tls",
//	        IssuerRef:  cmmeta.IssuerReference{Name: "letsencrypt", Kind: "ClusterIssuer"},
//	        DNSNames:   []string{"example.com"},
//	}
//
// An issuer's one-of (ACME, CA, Vault, SelfSigned, Venafi) is the upstream
// IssuerConfig struct: set the arm you want, as a pointer.
//
//	issuer := certmanager.CreateClusterIssuer("letsencrypt")
//	certmanager.SetClusterIssuerACME(issuer, &cmacme.ACMEIssuer{
//	        Server: "https://acme-v02.api.letsencrypt.org/directory",
//	        Email:  "admin@example.com",
//	})
//
// ## Update helpers
//
// Functions prefixed with Set or Add write exactly the one field they name
// and fall into the builder contract's admitted classes: AddCertificateDNSName
// and AddACMEIssuerSolver append, the remaining Set* helpers assign a
// pointer-typed field. Labels and annotations use the generic
// kubernetes.AddLabel / kubernetes.AddAnnotation over metav1.Object; this
// package carries no per-kind metadata helpers.
//
// The config-struct layer this package used to carry — Certificate, Issuer and
// ClusterIssuer taking a *Config, and the sealed IssuerVariant, ACMESolver and
// DNS01Provider sums — was retired by release 2 of the builder contract; see
// docs/builder-contract-release-2.md for the field-by-field mapping.
package certmanager
