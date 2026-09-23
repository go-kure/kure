// Package cnpg exposes the generated constructors and the admissible sugar
// for CloudNativePG (CNPG) and Barman Cloud plugin resources: Cluster,
// Database, Pooler, ScheduledBackup, ObjectStore and the image catalogs. Each
// constructor returns a controller-runtime object carrying identity only; the
// upstream CNPG struct is the construction API.
//
// ## Constructors
//
// Create<Kind> is generated from the registered scheme and emits apiVersion,
// kind, metadata.name and metadata.namespace. Spec fields are the caller's own
// assignments:
//
//	cluster := cnpg.CreateCluster("pg-main", "databases")
//	cluster.Spec = cnpgv1.ClusterSpec{
//	        Instances:            3,
//	        StorageConfiguration: cnpgv1.StorageConfiguration{Size: "10Gi"},
//	}
//
// ## Update helpers
//
// Functions prefixed with Set or Add write exactly the one field they name and
// fall into the builder contract's admitted classes: AddClusterManagedRole,
// AddDatabaseExtension and AddObjectStoreEnvVar append, the Set* helpers assign
// a pointer-typed field. Labels and annotations use the generic
// kubernetes.AddLabel / kubernetes.AddAnnotation over metav1.Object; this
// package carries no per-kind metadata helpers.
//
//	kubernetes.AddLabel(cluster, "env", "prod")
//	cnpg.AddClusterManagedRole(cluster, cnpgv1.RoleConfiguration{Name: "app"})
//
// The config-struct layer this package used to carry — Cluster, Database,
// ObjectStore, ScheduledBackup and Pooler taking a *Config with *Options — was
// retired by release 2 of the builder contract, and with it every value the
// layer invented: enablePDB derived from the instance count, the pinned
// primaryUpdateStrategy, the ACCESS_KEY_ID / SECRET_ACCESS_KEY key names, the
// barman-cloud plugin entry, the pooler type coercion and the extension ensure
// default. See docs/builder-contract-release-2.md for the field-by-field
// mapping.
package cnpg
