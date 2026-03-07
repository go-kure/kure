// Package externalsecrets exposes helper functions for constructing
// external-secrets resources.  Each function returns a fully initialized
// controller-runtime object that can be serialized to YAML or modified
// further by the calling application.
//
// ## Overview
//
// The package mirrors the constructors and setters found under
// `internal/externalsecrets` so applications can build external-secrets
// manifests programmatically without depending on the internal packages
// directly.  All builders operate on configuration structures defined in
// this package and convert them to the appropriate external-secrets custom
// resources.
//
// Resources covered include [ExternalSecret], [SecretStore], and
// [ClusterSecretStore].
//
// ## Constructors
//
// Constructors follow the form `<Type>` and accept a configuration struct.
// A minimal example creating an ExternalSecret looks like:
//
//	es := externalsecrets.ExternalSecret(&externalsecrets.ExternalSecretConfig{
//	        Name:      "my-secret",
//	        Namespace: "default",
//	        SecretStoreRef: esv1.SecretStoreRef{
//	                Name: "vault",
//	                Kind: "ClusterSecretStore",
//	        },
//	})
//
// ## Update helpers
//
// Additional functions prefixed with `Set` or `Add` expose granular control
// over the generated objects.  They delegate to the internal package to
// perform the actual mutations while keeping the public API stable.
package externalsecrets
