// Package certmanager exposes helper functions for constructing resources used by
// the cert-manager project.  Each function returns a fully initialized
// controller-runtime object that can be serialized to YAML or modified further by
// the calling application.
//
// ## Overview
//
// The package mirrors the constructors and setters found under
// `internal/certmanager` so applications can build cert-manager manifests
// programmatically without depending on the internal packages directly.  All
// builders operate on configuration structures defined in this package and
// convert them to the appropriate cert-manager custom resources.
//
// Resources covered include `Certificate`, `Issuer`, and `ClusterIssuer`, with
// ACME and CA issuer configurations.
//
// ## Constructors
//
// Constructors accept a configuration struct and return the corresponding
// cert-manager object.  A minimal example creating a `Certificate` looks like:
//
//	cert := certmanager.Certificate(&certmanager.CertificateConfig{
//	        Name:       "my-cert",
//	        Namespace:  "default",
//	        SecretName: "my-cert-tls",
//	        IssuerRef:  cmmeta.ObjectReference{Name: "letsencrypt", Kind: "ClusterIssuer"},
//	        DNSNames:   []string{"example.com"},
//	})
//
// ## Update helpers
//
// Additional functions prefixed with `Set` or `Add` expose granular control over
// the generated objects.  They delegate to the internal package to perform the
// actual mutations while keeping the public API stable.  For example:
//
//	cert := certmanager.Certificate(&certmanager.CertificateConfig{...})
//	certmanager.SetCertificateSpec(cert, certv1.CertificateSpec{SecretName: "new-secret"})
//
// This package aims to provide a convenient typed interface for applications
// that need to generate cert-manager manifests at runtime or as part of a build
// pipeline.
package certmanager
