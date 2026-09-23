package cnpg

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
)

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

func TestGolden_ClusterFull(t *testing.T) {
	connLimit := int64(10)
	inherit := false
	obj, err := Cluster(&ClusterConfig{
		Name:      "pg-main",
		Namespace: "databases",
		Options: &ClusterOptions{
			Instances:            3,
			ImageName:            "ghcr.io/cloudnative-pg/postgresql:16",
			StorageSize:          "10Gi",
			InheritedLabels:      map[string]string{"team": "backend"},
			InheritedAnnotations: map[string]string{"owner": "platform"},
			Resources: &ResourceOptions{
				RequestsCPU:    "250m",
				RequestsMemory: "512Mi",
				LimitsCPU:      "1",
				LimitsMemory:   "2Gi",
			},
			Backup: &BackupOptions{
				DestinationPath: "s3://bucket/pg-main/",
				EndpointURL:     "https://s3.example.com",
				RetentionPolicy: "30d",
				S3Credentials: &S3CredentialOptions{
					SecretName:         "backup-creds",
					AccessKeyIDKey:     "MY_ACCESS_KEY",
					SecretAccessKeyKey: "MY_SECRET_KEY",
				},
			},
			Monitoring: &MonitoringOptions{
				EnablePodMonitor:       true,
				CustomQueriesConfigMap: []ConfigMapKeyRefOptions{{Name: "custom-queries", Key: "queries.yaml"}},
			},
			Bootstrap: &BootstrapOptions{RecoverySource: "pg-old"},
			ExternalClusters: []ExternalClusterOptions{{
				Name:                 "pg-old",
				ConnectionParameters: map[string]string{"host": "pg-old.example.com", "user": "postgres"},
				BarmanObjectStore: map[string]any{
					"destinationPath": "s3://bucket/pg-old/",
					"endpointURL":     "https://s3.example.com",
				},
			}},
			PostgresParams:  map[string]string{"max_connections": "200"},
			Synchronous:     &SynchronousOptions{Method: "any", Number: 1, DataDurability: "required"},
			ObjectStoreName: "pg-backup",
			Affinity: &AffinityOptions{
				EnablePodAntiAffinity: true,
				TopologyKey:           "kubernetes.io/hostname",
				PodAntiAffinityType:   "preferred",
				NodeSelector:          map[string]string{"node-type": "db"},
			},
			ManagedRoles: []ManagedRoleOptions{{
				Name:            "app_user",
				Comment:         "Application user",
				Login:           true,
				Superuser:       false,
				CreateDB:        true,
				CreateRole:      false,
				Replication:     false,
				Inherit:         &inherit,
				ConnectionLimit: &connLimit,
				PasswordSecret:  "app-creds",
				InRoles:         []string{"pg_read_all_data"},
				Ensure:          "absent",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	goldenTest(t, "cluster-full.yaml", obj)
}

func TestGolden_ClusterMinimal(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name:      "pg-single",
		Namespace: "databases",
		Options:   &ClusterOptions{Instances: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	goldenTest(t, "cluster-minimal.yaml", obj)
}

func TestGolden_ClusterBackupDefaultKeys(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name:      "pg-replica",
		Namespace: "databases",
		Options: &ClusterOptions{
			Instances: 2,
			Backup: &BackupOptions{
				DestinationPath: "s3://bucket/pg-replica/",
				S3Credentials:   &S3CredentialOptions{SecretName: "backup-creds"},
			},
			Bootstrap: &BootstrapOptions{PgBasebackupSource: "pg-main"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	goldenTest(t, "cluster-backup-default-keys.yaml", obj)
}

func TestGolden_Database(t *testing.T) {
	obj := Database(&DatabaseConfig{
		Name:      "pg-main-appdb",
		Namespace: "databases",
		Options: &DatabaseOptions{
			ClusterName:   "pg-main",
			DBName:        "appdb",
			Owner:         "app_user",
			ReclaimPolicy: "delete",
			Ensure:        "absent",
			Extensions: []ExtensionOptions{
				{Name: "pg_stat_statements"},
				{Name: "pgvector", Ensure: "absent"},
			},
		},
	})
	goldenTest(t, "database.yaml", obj)
}

func TestGolden_ObjectStore(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{
		Name:      "backup-store",
		Namespace: "databases",
		Options: &ObjectStoreOptions{
			DestinationPath:    "s3://bucket/pg/",
			EndpointURL:        "https://s3.example.com",
			ServerName:         "pg-main",
			SecretName:         "backup-creds",
			AccessKeyIDKey:     "MY_ACCESS_KEY",
			SecretAccessKeyKey: "MY_SECRET_KEY",
			RetentionPolicy:    "30d",
		},
	})
	goldenTest(t, "objectstore.yaml", obj)
}

func TestGolden_ObjectStoreDefaultKeys(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{
		Name:      "backup-store",
		Namespace: "databases",
		Options: &ObjectStoreOptions{
			DestinationPath: "s3://bucket/pg/",
			SecretName:      "backup-creds",
		},
	})
	goldenTest(t, "objectstore-default-keys.yaml", obj)
}

func TestGolden_ScheduledBackup(t *testing.T) {
	obj := ScheduledBackup(&ScheduledBackupConfig{
		Name:      "daily-backup",
		Namespace: "databases",
		Spec: cnpgv1.ScheduledBackupSpec{
			Schedule: "0 0 2 * * *",
			Cluster:  cnpgv1.LocalObjectReference{Name: "pg-main"},
			Method:   cnpgv1.BackupMethodPlugin,
		},
	})
	goldenTest(t, "scheduledbackup.yaml", obj)
}

func TestGolden_PoolerFull(t *testing.T) {
	obj := Pooler(&PoolerConfig{
		Name:      "pg-main-pooler-ro",
		Namespace: "databases",
		Options: &PoolerOptions{
			ClusterName: "pg-main",
			Instances:   2,
			Type:        "ro",
			PgBouncer: &PgBouncerOptions{
				PoolMode:   "transaction",
				Parameters: map[string]string{"max_client_conn": "100"},
			},
		},
	})
	goldenTest(t, "pooler-full.yaml", obj)
}

func TestGolden_PoolerDefault(t *testing.T) {
	obj := Pooler(&PoolerConfig{
		Name:      "pg-main-pooler",
		Namespace: "databases",
		Options:   &PoolerOptions{ClusterName: "pg-main"},
	})
	goldenTest(t, "pooler-default.yaml", obj)
}
