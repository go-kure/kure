// Package externalsecrets exposes the generated constructors and the
// admissible sugar for External Secrets Operator resources: ExternalSecret,
// SecretStore and ClusterSecretStore. Each constructor returns a
// controller-runtime object carrying identity only; the upstream
// external-secrets struct is the construction API.
//
// ## Constructors
//
// Create<Kind> is generated from the registered scheme and emits apiVersion,
// kind, metadata.name and, for a namespaced kind, metadata.namespace. Spec
// fields are the caller's own assignments:
//
//	es := externalsecrets.CreateExternalSecret("my-secret", "default")
//	es.Spec.SecretStoreRef = esv1.SecretStoreRef{Name: "vault", Kind: "ClusterSecretStore"}
//	externalsecrets.AddExternalSecretData(es, esv1.ExternalSecretData{
//	        SecretKey: "password",
//	        RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "secret/data/myapp"},
//	})
//
// ## Update helpers
//
// Functions prefixed with Set or Add write exactly the one field they name
// and fall into the builder contract's admitted classes: AddExternalSecretData
// and AddDataFrom append, SetSecretStoreProvider, SetClusterSecretStoreProvider
// and SetRefreshInterval assign a pointer-typed field. Labels and annotations
// use the generic kubernetes.AddLabel / kubernetes.AddAnnotation over
// metav1.Object; this package carries no per-kind metadata helpers.
//
// The config-struct layer this package used to carry — ExternalSecret,
// SecretStore and ClusterSecretStore taking a *Config — was retired by
// release 2 of the builder contract; see docs/builder-contract-release-2.md
// for the field-by-field mapping.
package externalsecrets
