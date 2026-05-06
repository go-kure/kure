package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
)

// ObjectStoreOptions is the domain-friendly input for ObjectStore construction.
// It covers the S3 credential and destination path fields without exposing
// barman-cloud or machinery API types to callers.
type ObjectStoreOptions struct {
	DestinationPath string
	EndpointURL     string
	ServerName      string
	// SecretName is the K8s secret containing S3 credentials. Empty means no S3 creds.
	SecretName string
	// AccessKeyIDKey is the key within SecretName for the access key ID (default "ACCESS_KEY_ID").
	AccessKeyIDKey string
	// SecretAccessKeyKey is the key within SecretName for the secret (default "SECRET_ACCESS_KEY").
	SecretAccessKeyKey string
	RetentionPolicy    string
}

// ObjectStoreConfig describes a Barman Cloud ObjectStore resource.
type ObjectStoreConfig struct {
	Name      string
	Namespace string
	Options   *ObjectStoreOptions
}

// ExtensionOptions is the domain-friendly input for a single CNPG Database extension.
type ExtensionOptions struct {
	Name string
	// Ensure is "absent" or "" (present).
	Ensure string
}

// DatabaseOptions is the domain-friendly input for Database construction.
type DatabaseOptions struct {
	ClusterName string
	DBName      string
	Owner       string
	// ReclaimPolicy is "delete" or "" (retain).
	ReclaimPolicy string
	// Ensure is "absent" or "" (present).
	Ensure     string
	Extensions []ExtensionOptions
}

// DatabaseConfig describes a CNPG Database resource.
type DatabaseConfig struct {
	Name      string
	Namespace string
	Options   *DatabaseOptions
}

// S3CredentialOptions specifies an S3 secret reference for backup.
type S3CredentialOptions struct {
	SecretName         string
	AccessKeyIDKey     string // default "ACCESS_KEY_ID"
	SecretAccessKeyKey string // default "SECRET_ACCESS_KEY"
}

// BackupOptions configures the inline barmanObjectStore backup on a Cluster.
type BackupOptions struct {
	DestinationPath string
	EndpointURL     string
	RetentionPolicy string
	S3Credentials   *S3CredentialOptions
}

// ResourceOptions holds CPU/memory requests and limits as quantity strings (e.g. "500m", "1Gi").
type ResourceOptions struct {
	RequestsCPU    string
	RequestsMemory string
	LimitsCPU      string
	LimitsMemory   string
}

// ConfigMapKeyRefOptions references a key in a ConfigMap.
type ConfigMapKeyRefOptions struct {
	Name string
	Key  string
}

// MonitoringOptions configures the CNPG Cluster monitoring section.
type MonitoringOptions struct {
	EnablePodMonitor       bool
	CustomQueriesConfigMap []ConfigMapKeyRefOptions
}

// BootstrapOptions configures the CNPG Cluster bootstrap section.
// At most one of RecoverySource or PgBasebackupSource may be set.
type BootstrapOptions struct {
	RecoverySource     string
	PgBasebackupSource string
}

// SynchronousOptions configures synchronous replication on a Cluster.
type SynchronousOptions struct {
	Method          string // e.g. "any" or "first"
	Number          int32
	DataDurability  string // "required" or "preferred"
	MaxStandbyDelay int32
}

// AffinityOptions configures pod and node affinity/anti-affinity on a Cluster.
type AffinityOptions struct {
	EnablePodAntiAffinity bool
	TopologyKey           string
	PodAntiAffinityType   string
	NodeSelector          map[string]string
}

// ManagedRoleOptions configures a single entry in spec.managed.roles[].
type ManagedRoleOptions struct {
	Name        string
	Comment     string
	Login       bool
	Superuser   bool
	CreateDB    bool
	CreateRole  bool
	Replication bool
	// Inherit nil means omit (CNPG defaults to true).
	Inherit *bool
	// ConnectionLimit nil means omit (CNPG defaults to -1).
	ConnectionLimit *int64
	PasswordSecret  string
	InRoles         []string
	// Ensure is "absent" or "" (present).
	Ensure string
}

// ExternalClusterOptions defines an external cluster for bootstrap or replica sources.
// BarmanObjectStore is passed as a map and translated via JSON round-trip to avoid
// exposing barman-cloud types to callers.
type ExternalClusterOptions struct {
	Name                 string
	ConnectionParameters map[string]string
	BarmanObjectStore    map[string]any
}

// ClusterOptions is the domain-friendly input for Cluster construction.
// It covers all fields used by crane's postgresql component handler without
// exposing cnpgv1, barmanApi, or machinery API types to callers.
type ClusterOptions struct {
	Instances   int32
	ImageName   string
	StorageSize string // e.g. "10Gi"; stored as string, matches cnpgv1.StorageConfiguration.Size

	InheritedLabels      map[string]string
	InheritedAnnotations map[string]string

	Resources  *ResourceOptions
	Backup     *BackupOptions
	Monitoring *MonitoringOptions
	Bootstrap  *BootstrapOptions

	ExternalClusters []ExternalClusterOptions

	PostgresParams map[string]string
	Synchronous    *SynchronousOptions

	// ObjectStoreName non-empty adds the barman-cloud WAL archiver plugin entry.
	ObjectStoreName string

	Affinity     *AffinityOptions
	ManagedRoles []ManagedRoleOptions
}

// ClusterConfig describes a CNPG Cluster resource.
type ClusterConfig struct {
	Name      string
	Namespace string
	Options   *ClusterOptions
}

// ScheduledBackupConfig describes a CNPG ScheduledBackup resource.
// It retains the raw spec type since crane does not use it; promoting it to
// domain types is deferred until there is a concrete caller.
type ScheduledBackupConfig struct {
	Name      string                     `yaml:"name"`
	Namespace string                     `yaml:"namespace"`
	Spec      cnpgv1.ScheduledBackupSpec `yaml:"spec"`
}

// PgBouncerOptions is the domain-friendly configuration for the PgBouncer
// connection pooler. It covers the fields used by crane without exposing the
// cnpgv1.PgBouncerSpec type to callers.
type PgBouncerOptions struct {
	// PoolMode is the connection pooling mode: "session" or "transaction".
	// When empty, CNPG defaults to "session".
	PoolMode string
	// Parameters are extra PgBouncer configuration key-value pairs.
	Parameters map[string]string
}

// PoolerOptions is the domain-friendly input for Pooler construction.
type PoolerOptions struct {
	ClusterName string
	// Instances is the number of Pooler replicas. Zero means omit the field
	// and let CNPG default to 1.
	Instances int32
	// Type is the pooler type: "rw" (read-write, default) or "ro" (read-only).
	// Defaults to "rw" if empty or unrecognised.
	Type string
	// PgBouncer holds pgBouncer-specific configuration.
	PgBouncer *PgBouncerOptions
}

// PoolerConfig describes a CNPG Pooler resource.
type PoolerConfig struct {
	Name      string
	Namespace string
	Options   *PoolerOptions
}
