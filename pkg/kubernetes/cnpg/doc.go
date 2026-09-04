// Package cnpg exposes helper functions for constructing resources used by
// CloudNativePG (CNPG) and the Barman Cloud plugin.  Each function returns a
// fully initialized controller-runtime object that can be serialized to YAML or
// modified further by the calling application.
//
// ## Overview
//
// The package mirrors the constructors and setters found under
// `internal/cnpg` so applications can build CNPG manifests programmatically
// without depending on the internal packages directly.  All constructors accept
// a configuration struct.
//
// Resources covered include `Cluster`, `Database`, `ObjectStore`, and
// `ScheduledBackup`.
//
// ## Constructors
//
// Constructors accept a configuration struct and return the corresponding CNPG
// object.  A minimal example creating a `Cluster` looks like:
//
//	cluster := cnpg.Cluster(&cnpg.ClusterConfig{
//	        Name:      "pg-main",
//	        Namespace: "databases",
//	        Spec:      cnpgv1.ClusterSpec{Instances: 3},
//	})
//
// ## Update helpers
//
// Additional functions prefixed with `Set` or `Add` expose granular control
// over the generated objects.  Each writes exactly the one spec field it names.
// Labels and annotations use the generic kubernetes.AddLabel /
// kubernetes.AddAnnotation over metav1.Object; this package carries no per-kind
// metadata helpers.  For example:
//
//	cluster := cnpg.Cluster(&cnpg.ClusterConfig{...})
//	kubernetes.AddLabel(cluster, "env", "prod")
//	cnpg.AddClusterManagedRole(cluster, cnpgv1.RoleConfiguration{Name: "app"})
package cnpg
