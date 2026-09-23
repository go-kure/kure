package cnpg

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	machineryapi "github.com/cloudnative-pg/machinery/pkg/api"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
)

// The fixtures under testdata were written by the config-struct layer this
// package used to carry (Cluster(&ClusterConfig{...}), Database, ObjectStore,
// ScheduledBackup, Pooler) before it was retired. Each test below builds the
// same object on the generated constructor plus the upstream struct and must
// reproduce that output byte for byte. Every value the layer used to invent —
// enablePDB from the instance count, primaryUpdateStrategy, the S3 key names,
// the barman-cloud plugin entry, the pooler type and its empty pgbouncer
// block, an extension's ensure — is now a line the caller writes, and each is
// marked "formerly injected" where it appears.

var update = flag.Bool("update", false, "update golden files")

func goldenTest(t *testing.T, filename string, obj client.Object) {
	t.Helper()
	objects := []*client.Object{&obj}
	got, err := kureio.EncodeObjectsToYAMLWithOptions(objects, kureio.EncodeOptions{
		KubernetesFieldOrder: true,
	})
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}
	golden := filepath.Join("testdata", filename)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("updating golden file: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n\ngot:\n%s\nwant:\n%s", golden, got, want)
	}
}

// s3Credentials is the upstream pair of secret key references the old
// ObjectStoreOptions / S3CredentialOptions built from a secret name and two
// key names. The key names are the caller's choice now; the layer used to
// fill in ACCESS_KEY_ID and SECRET_ACCESS_KEY when they were left empty.
func s3Credentials(secret, accessKeyIDKey, secretAccessKeyKey string) *barmanapi.S3Credentials {
	return &barmanapi.S3Credentials{
		AccessKeyIDReference: &machineryapi.SecretKeySelector{
			LocalObjectReference: machineryapi.LocalObjectReference{Name: secret},
			Key:                  accessKeyIDKey,
		},
		SecretAccessKeyReference: &machineryapi.SecretKeySelector{
			LocalObjectReference: machineryapi.LocalObjectReference{Name: secret},
			Key:                  secretAccessKeyKey,
		},
	}
}

func TestGolden_ClusterFull(t *testing.T) {
	obj := CreateCluster("pg-main", "databases")
	obj.Spec = cnpgv1.ClusterSpec{
		Instances: 3,
		ImageName: "ghcr.io/cloudnative-pg/postgresql:16",
		// formerly injected: enablePDB was derived from Instances > 1
		EnablePDB: ptr.To(true),
		// formerly injected: primaryUpdateStrategy was pinned to unsupervised
		PrimaryUpdateStrategy: cnpgv1.PrimaryUpdateStrategyUnsupervised,
		StorageConfiguration:  cnpgv1.StorageConfiguration{Size: "10Gi"},
		InheritedMetadata: &cnpgv1.EmbeddedObjectMetadata{
			Labels:      map[string]string{"team": "backend"},
			Annotations: map[string]string{"owner": "platform"},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		},
		Backup: &cnpgv1.BackupConfiguration{
			RetentionPolicy: "30d",
			BarmanObjectStore: &barmanapi.BarmanObjectStoreConfiguration{
				DestinationPath:   "s3://bucket/pg-main/",
				EndpointURL:       "https://s3.example.com",
				BarmanCredentials: barmanapi.BarmanCredentials{AWS: s3Credentials("backup-creds", "MY_ACCESS_KEY", "MY_SECRET_KEY")},
			},
		},
		Monitoring: &cnpgv1.MonitoringConfiguration{
			EnablePodMonitor: true, //nolint:staticcheck // SA1019: the only upstream field that opts into PodMonitor creation
			CustomQueriesConfigMap: []cnpgv1.ConfigMapKeySelector{{
				LocalObjectReference: machineryapi.LocalObjectReference{Name: "custom-queries"},
				Key:                  "queries.yaml",
			}},
		},
		Bootstrap: &cnpgv1.BootstrapConfiguration{
			Recovery: &cnpgv1.BootstrapRecovery{Source: "pg-old"},
		},
		ExternalClusters: []cnpgv1.ExternalCluster{{
			Name:                 "pg-old",
			ConnectionParameters: map[string]string{"host": "pg-old.example.com", "user": "postgres"},
			BarmanObjectStore: &barmanapi.BarmanObjectStoreConfiguration{
				DestinationPath: "s3://bucket/pg-old/",
				EndpointURL:     "https://s3.example.com",
			},
		}},
		PostgresConfiguration: cnpgv1.PostgresConfiguration{
			Parameters: map[string]string{"max_connections": "200"},
			Synchronous: &cnpgv1.SynchronousReplicaConfiguration{
				Method:         cnpgv1.SynchronousReplicaConfigurationMethodAny,
				Number:         1,
				DataDurability: cnpgv1.DataDurabilityLevelRequired,
			},
		},
		// formerly injected: the plugin name and isWALArchiver came with ObjectStoreName
		Plugins: []cnpgv1.PluginConfiguration{{
			Name:          "barman-cloud.barmancloud.cnpg.io",
			IsWALArchiver: ptr.To(true),
			Parameters:    map[string]string{"objectStoreName": "pg-backup"},
		}},
		Affinity: cnpgv1.AffinityConfiguration{
			EnablePodAntiAffinity: ptr.To(true),
			TopologyKey:           "kubernetes.io/hostname",
			PodAntiAffinityType:   cnpgv1.PodAntiAffinityTypePreferred,
			NodeSelector:          map[string]string{"node-type": "db"},
		},
		Managed: &cnpgv1.ManagedConfiguration{
			Roles: []cnpgv1.RoleConfiguration{{
				Name:            "app_user",
				Comment:         "Application user",
				Login:           true,
				CreateDB:        true,
				Inherit:         ptr.To(false),
				ConnectionLimit: 10,
				PasswordSecret:  &cnpgv1.LocalObjectReference{Name: "app-creds"},
				InRoles:         []string{"pg_read_all_data"},
				Ensure:          cnpgv1.EnsureAbsent,
			}},
		},
	}
	goldenTest(t, "cluster-full.yaml", obj)
}

func TestGolden_ClusterMinimal(t *testing.T) {
	obj := CreateCluster("pg-single", "databases")
	obj.Spec = cnpgv1.ClusterSpec{
		Instances: 1,
		// formerly injected: enablePDB false because Instances was 1
		EnablePDB: ptr.To(false),
		// formerly injected: primaryUpdateStrategy was pinned to unsupervised
		PrimaryUpdateStrategy: cnpgv1.PrimaryUpdateStrategyUnsupervised,
	}
	goldenTest(t, "cluster-minimal.yaml", obj)
}

func TestGolden_ClusterBackupDefaultKeys(t *testing.T) {
	obj := CreateCluster("pg-replica", "databases")
	obj.Spec = cnpgv1.ClusterSpec{
		Instances: 2,
		// formerly injected: enablePDB true because Instances > 1
		EnablePDB: ptr.To(true),
		// formerly injected: primaryUpdateStrategy was pinned to unsupervised
		PrimaryUpdateStrategy: cnpgv1.PrimaryUpdateStrategyUnsupervised,
		Backup: &cnpgv1.BackupConfiguration{
			BarmanObjectStore: &barmanapi.BarmanObjectStoreConfiguration{
				DestinationPath: "s3://bucket/pg-replica/",
				// formerly injected: the key names ACCESS_KEY_ID / SECRET_ACCESS_KEY
				BarmanCredentials: barmanapi.BarmanCredentials{AWS: s3Credentials("backup-creds", "ACCESS_KEY_ID", "SECRET_ACCESS_KEY")},
			},
		},
		Bootstrap: &cnpgv1.BootstrapConfiguration{
			PgBaseBackup: &cnpgv1.BootstrapPgBaseBackup{Source: "pg-main"},
		},
	}
	goldenTest(t, "cluster-backup-default-keys.yaml", obj)
}

func TestGolden_Database(t *testing.T) {
	obj := CreateDatabase("pg-main-appdb", "databases")
	obj.Spec = cnpgv1.DatabaseSpec{
		ClusterRef:    corev1.LocalObjectReference{Name: "pg-main"},
		Name:          "appdb",
		Owner:         "app_user",
		ReclaimPolicy: cnpgv1.DatabaseReclaimDelete,
		Ensure:        cnpgv1.EnsureAbsent,
	}
	// formerly injected: an extension without Ensure: "absent" got ensure: present
	AddDatabaseExtension(obj, cnpgv1.ExtensionSpec{
		DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{Name: "pg_stat_statements", Ensure: cnpgv1.EnsurePresent},
	})
	AddDatabaseExtension(obj, cnpgv1.ExtensionSpec{
		DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{Name: "pgvector", Ensure: cnpgv1.EnsureAbsent},
	})
	goldenTest(t, "database.yaml", obj)
}

func TestGolden_ObjectStore(t *testing.T) {
	obj := CreateObjectStore("backup-store", "databases")
	obj.Spec = barmanv1.ObjectStoreSpec{
		Configuration: barmanapi.BarmanObjectStoreConfiguration{
			DestinationPath: "s3://bucket/pg/",
			EndpointURL:     "https://s3.example.com",
			ServerName:      "pg-main",
		},
		RetentionPolicy: "30d",
	}
	SetObjectStoreS3Credentials(obj, s3Credentials("backup-creds", "MY_ACCESS_KEY", "MY_SECRET_KEY"))
	goldenTest(t, "objectstore.yaml", obj)
}

func TestGolden_ObjectStoreDefaultKeys(t *testing.T) {
	obj := CreateObjectStore("backup-store", "databases")
	obj.Spec.Configuration.DestinationPath = "s3://bucket/pg/"
	// formerly injected: the key names ACCESS_KEY_ID / SECRET_ACCESS_KEY
	SetObjectStoreS3Credentials(obj, s3Credentials("backup-creds", "ACCESS_KEY_ID", "SECRET_ACCESS_KEY"))
	goldenTest(t, "objectstore-default-keys.yaml", obj)
}

func TestGolden_ScheduledBackup(t *testing.T) {
	obj := CreateScheduledBackup("daily-backup", "databases")
	obj.Spec = cnpgv1.ScheduledBackupSpec{
		Schedule: "0 0 2 * * *",
		Cluster:  cnpgv1.LocalObjectReference{Name: "pg-main"},
		Method:   cnpgv1.BackupMethodPlugin,
	}
	goldenTest(t, "scheduledbackup.yaml", obj)
}

func TestGolden_PoolerFull(t *testing.T) {
	obj := CreatePooler("pg-main-pooler-ro", "databases")
	obj.Spec = cnpgv1.PoolerSpec{
		Cluster:   cnpgv1.LocalObjectReference{Name: "pg-main"},
		Type:      cnpgv1.PoolerTypeRO,
		Instances: ptr.To[int32](2),
		PgBouncer: &cnpgv1.PgBouncerSpec{
			PoolMode:   cnpgv1.PgBouncerPoolModeTransaction,
			Parameters: map[string]string{"max_client_conn": "100"},
		},
	}
	goldenTest(t, "pooler-full.yaml", obj)
}

func TestGolden_PoolerDefault(t *testing.T) {
	obj := CreatePooler("pg-main-pooler", "databases")
	obj.Spec = cnpgv1.PoolerSpec{
		Cluster: cnpgv1.LocalObjectReference{Name: "pg-main"},
		// formerly injected: any Type other than "ro" became rw
		Type: cnpgv1.PoolerTypeRW,
		// formerly injected: pgbouncer was always an empty block
		PgBouncer: &cnpgv1.PgBouncerSpec{},
	}
	goldenTest(t, "pooler-default.yaml", obj)
}

// TestGolden_ClusterAffinityDisabled pins the one CNPG pointer the retired
// layer always wrote whatever its value: EnablePodAntiAffinity, set whenever
// Affinity was supplied, including false. CNPG applies anti-affinity unless
// the field is explicitly false, so leaving it nil for a false input would
// turn a disabled setting back on. The fixture was written by the retired
// Cluster(&ClusterConfig{...}) builder with
// Affinity: &AffinityOptions{EnablePodAntiAffinity: false,
// TopologyKey: "kubernetes.io/hostname"}.
func TestGolden_ClusterAffinityDisabled(t *testing.T) {
	obj := CreateCluster("pg-no-paa", "databases")
	obj.Spec = cnpgv1.ClusterSpec{
		Instances: 1,
		// formerly injected: enablePDB false because Instances was 1
		EnablePDB: ptr.To(false),
		// formerly injected: primaryUpdateStrategy was pinned to unsupervised
		PrimaryUpdateStrategy: cnpgv1.PrimaryUpdateStrategyUnsupervised,
		Affinity: cnpgv1.AffinityConfiguration{
			EnablePodAntiAffinity: ptr.To(false),
			TopologyKey:           "kubernetes.io/hostname",
		},
	}
	goldenTest(t, "cluster-affinity-disabled.yaml", obj)
}

// TestGolden_ClusterEmptyBootstrap pins the bootstrap guard of the retired
// layer: a BootstrapOptions with neither source set left Spec.Bootstrap nil,
// which CNPG defaults to initdb. A bootstrap block with an empty recovery or
// pg_basebackup arm would select that mode instead. The fixture was written by
// the retired Cluster(&ClusterConfig{...}) builder with
// Bootstrap: &BootstrapOptions{}.
func TestGolden_ClusterEmptyBootstrap(t *testing.T) {
	obj := CreateCluster("pg-initdb", "databases")
	obj.Spec = cnpgv1.ClusterSpec{
		Instances: 1,
		// formerly injected: enablePDB false because Instances was 1
		EnablePDB: ptr.To(false),
		// formerly injected: primaryUpdateStrategy was pinned to unsupervised
		PrimaryUpdateStrategy: cnpgv1.PrimaryUpdateStrategyUnsupervised,
		// no Bootstrap: neither source was set
	}
	goldenTest(t, "cluster-empty-bootstrap.yaml", obj)
}
