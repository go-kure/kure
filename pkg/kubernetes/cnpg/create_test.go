package cnpg

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
)

func TestCluster_Success(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name:      "pg-main",
		Namespace: "databases",
		Options: &ClusterOptions{
			Instances:   3,
			ImageName:   "ghcr.io/cloudnative-pg/postgresql:16",
			StorageSize: "10Gi",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj == nil {
		t.Fatal("expected non-nil Cluster")
	}
	if obj.Name != "pg-main" {
		t.Errorf("expected Name 'pg-main', got %s", obj.Name)
	}
	if obj.Namespace != "databases" {
		t.Errorf("expected Namespace 'databases', got %s", obj.Namespace)
	}
	if obj.Spec.Instances != 3 {
		t.Errorf("expected Instances 3, got %d", obj.Spec.Instances)
	}
	if obj.Spec.StorageConfiguration.Size != "10Gi" {
		t.Errorf("expected StorageSize '10Gi', got %s", obj.Spec.StorageConfiguration.Size)
	}
}

func TestCluster_EnablePDB(t *testing.T) {
	single, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{Instances: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if *single.Spec.EnablePDB {
		t.Error("single-instance cluster should have EnablePDB=false")
	}

	multi, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{Instances: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !*multi.Spec.EnablePDB {
		t.Error("multi-instance cluster should have EnablePDB=true")
	}
}

func TestCluster_ManagedRoles(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			ManagedRoles: []ManagedRoleOptions{
				{Name: "app_user", Login: true, PasswordSecret: "app-creds", Comment: "Application user"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if obj.Spec.Managed == nil || len(obj.Spec.Managed.Roles) != 1 {
		t.Fatal("expected 1 managed role")
	}
	role := obj.Spec.Managed.Roles[0]
	if role.Name != "app_user" || !role.Login || role.Comment != "Application user" {
		t.Errorf("unexpected role: %+v", role)
	}
	if role.PasswordSecret == nil || role.PasswordSecret.Name != "app-creds" {
		t.Errorf("expected PasswordSecret 'app-creds', got %v", role.PasswordSecret)
	}
}

func TestCluster_ObjectStorePlugin(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances:       1,
			ObjectStoreName: "pg-backup",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(obj.Spec.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(obj.Spec.Plugins))
	}
	if obj.Spec.Plugins[0].Parameters["objectStoreName"] != "pg-backup" {
		t.Errorf("unexpected plugin params: %v", obj.Spec.Plugins[0].Parameters)
	}
}

func TestCluster_NilConfig(t *testing.T) {
	obj, err := Cluster(nil)
	if err != nil {
		t.Fatal(err)
	}
	if obj != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestDatabase_Success(t *testing.T) {
	obj := Database(&DatabaseConfig{
		Name:      "pg-appdb",
		Namespace: "databases",
		Options: &DatabaseOptions{
			ClusterName: "pg",
			DBName:      "appdb",
			Owner:       "app_user",
		},
	})
	if obj == nil {
		t.Fatal("expected non-nil Database")
	}
	if obj.Name != "pg-appdb" {
		t.Errorf("expected Name 'pg-appdb', got %s", obj.Name)
	}
	if obj.Spec.Name != "appdb" {
		t.Errorf("expected Spec.Name 'appdb', got %s", obj.Spec.Name)
	}
	if obj.Spec.Owner != "app_user" {
		t.Errorf("expected Owner 'app_user', got %s", obj.Spec.Owner)
	}
}

func TestDatabase_Extensions(t *testing.T) {
	obj := Database(&DatabaseConfig{
		Name: "pg-db", Namespace: "ns",
		Options: &DatabaseOptions{
			ClusterName: "pg",
			DBName:      "db",
			Extensions: []ExtensionOptions{
				{Name: "pg_stat_statements", Ensure: ""},
				{Name: "pgvector", Ensure: "absent"},
			},
		},
	})
	if len(obj.Spec.Extensions) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(obj.Spec.Extensions))
	}
	if obj.Spec.Extensions[0].Ensure != cnpgv1.EnsurePresent {
		t.Errorf("expected first extension ensure=present, got %s", obj.Spec.Extensions[0].Ensure)
	}
	if obj.Spec.Extensions[1].Ensure != cnpgv1.EnsureAbsent {
		t.Errorf("expected second extension ensure=absent, got %s", obj.Spec.Extensions[1].Ensure)
	}
}

func TestDatabase_NilConfig(t *testing.T) {
	if Database(nil) != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestObjectStore_Success(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{
		Name:      "backup-store",
		Namespace: "databases",
		Options: &ObjectStoreOptions{
			DestinationPath: "s3://my-bucket/pg/",
			RetentionPolicy: "30d",
			SecretName:      "backup-creds",
		},
	})
	if obj == nil {
		t.Fatal("expected non-nil ObjectStore")
	}
	if obj.Name != "backup-store" {
		t.Errorf("expected Name 'backup-store', got %s", obj.Name)
	}
	if obj.Spec.Configuration.DestinationPath != "s3://my-bucket/pg/" {
		t.Errorf("unexpected DestinationPath: %s", obj.Spec.Configuration.DestinationPath)
	}
	if obj.Spec.Configuration.AWS == nil {
		t.Fatal("expected S3 credentials to be set")
	}
	if obj.Spec.Configuration.AWS.AccessKeyIDReference.Key != "ACCESS_KEY_ID" {
		t.Errorf("unexpected access key ID key: %s", obj.Spec.Configuration.AWS.AccessKeyIDReference.Key)
	}
}

func TestObjectStore_NoCredentials(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{
		Name: "store", Namespace: "ns",
		Options: &ObjectStoreOptions{DestinationPath: "s3://bucket/path/"},
	})
	if obj.Spec.Configuration.AWS != nil {
		t.Error("expected no S3 credentials when SecretName is empty")
	}
}

func TestObjectStore_NilConfig(t *testing.T) {
	if ObjectStore(nil) != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestScheduledBackup_Success(t *testing.T) {
	cfg := &ScheduledBackupConfig{
		Name:      "daily-backup",
		Namespace: "databases",
		Spec:      cnpgv1.ScheduledBackupSpec{Schedule: "0 2 * * *"},
	}

	obj := ScheduledBackup(cfg)

	if obj == nil {
		t.Fatal("expected non-nil ScheduledBackup")
	}
	if obj.Name != "daily-backup" {
		t.Errorf("expected Name 'daily-backup', got %s", obj.Name)
	}
	if obj.Spec.Schedule != "0 2 * * *" {
		t.Errorf("expected Schedule '0 2 * * *', got %s", obj.Spec.Schedule)
	}
}

func TestScheduledBackup_NilConfig(t *testing.T) {
	if ScheduledBackup(nil) != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestCluster_Resources(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Resources: &ResourceOptions{
				RequestsCPU:    "250m",
				RequestsMemory: "512Mi",
				LimitsCPU:      "1",
				LimitsMemory:   "2Gi",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj == nil {
		t.Fatal("expected non-nil cluster")
	}
	if obj.Spec.Resources.Requests.Cpu().IsZero() {
		t.Error("expected non-zero CPU request")
	}
	if obj.Spec.Resources.Requests.Memory().IsZero() {
		t.Error("expected non-zero memory request")
	}
	if obj.Spec.Resources.Limits.Cpu().IsZero() {
		t.Error("expected non-zero CPU limit")
	}
	if obj.Spec.Resources.Limits.Memory().IsZero() {
		t.Error("expected non-zero memory limit")
	}
}

func TestCluster_Resources_OnlyRequests(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Resources: &ResourceOptions{
				RequestsCPU:    "250m",
				RequestsMemory: "512Mi",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.Resources.Requests == nil {
		t.Error("expected non-nil Requests")
	}
	if obj.Spec.Resources.Limits != nil {
		t.Error("expected nil Limits")
	}
}

func TestCluster_Resources_OnlyLimits(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Resources: &ResourceOptions{
				LimitsCPU:    "1",
				LimitsMemory: "2Gi",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.Resources.Limits == nil {
		t.Error("expected non-nil Limits")
	}
	if obj.Spec.Resources.Requests != nil {
		t.Error("expected nil Requests")
	}
}

func TestCluster_Resources_InvalidCPURequest(t *testing.T) {
	_, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Resources: &ResourceOptions{RequestsCPU: "not-a-quantity"},
		},
	})
	if err == nil {
		t.Error("expected error for invalid CPU request")
	}
}

func TestCluster_Resources_InvalidMemoryRequest(t *testing.T) {
	_, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Resources: &ResourceOptions{RequestsMemory: "not-a-quantity"},
		},
	})
	if err == nil {
		t.Error("expected error for invalid memory request")
	}
}

func TestCluster_Resources_InvalidCPULimit(t *testing.T) {
	_, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Resources: &ResourceOptions{LimitsCPU: "not-a-quantity"},
		},
	})
	if err == nil {
		t.Error("expected error for invalid CPU limit")
	}
}

func TestCluster_Resources_InvalidMemoryLimit(t *testing.T) {
	_, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Resources: &ResourceOptions{LimitsMemory: "not-a-quantity"},
		},
	})
	if err == nil {
		t.Error("expected error for invalid memory limit")
	}
}

func TestCluster_BackupWithS3(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Backup: &BackupOptions{
				DestinationPath: "s3://bucket/pg/",
				EndpointURL:     "https://s3.example.com",
				RetentionPolicy: "30d",
				S3Credentials: &S3CredentialOptions{
					SecretName:         "backup-creds",
					AccessKeyIDKey:     "MY_ACCESS_KEY",
					SecretAccessKeyKey: "MY_SECRET_KEY",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.Backup == nil {
		t.Fatal("expected non-nil Backup")
	}
	if obj.Spec.Backup.BarmanObjectStore == nil {
		t.Fatal("expected non-nil BarmanObjectStore")
	}
	if obj.Spec.Backup.BarmanObjectStore.AWS == nil {
		t.Fatal("expected non-nil S3 credentials")
	}
	if obj.Spec.Backup.BarmanObjectStore.AWS.AccessKeyIDReference.Key != "MY_ACCESS_KEY" {
		t.Errorf("unexpected access key: %s", obj.Spec.Backup.BarmanObjectStore.AWS.AccessKeyIDReference.Key)
	}
}

func TestCluster_BackupWithDefaultS3Keys(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Backup: &BackupOptions{
				DestinationPath: "s3://bucket/pg/",
				S3Credentials:   &S3CredentialOptions{SecretName: "creds"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.Backup.BarmanObjectStore.AWS.AccessKeyIDReference.Key != "ACCESS_KEY_ID" {
		t.Errorf("expected default access key name, got %s", obj.Spec.Backup.BarmanObjectStore.AWS.AccessKeyIDReference.Key)
	}
	if obj.Spec.Backup.BarmanObjectStore.AWS.SecretAccessKeyReference.Key != "SECRET_ACCESS_KEY" {
		t.Errorf("expected default secret key name, got %s", obj.Spec.Backup.BarmanObjectStore.AWS.SecretAccessKeyReference.Key)
	}
}

func TestCluster_Monitoring(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Monitoring: &MonitoringOptions{
				EnablePodMonitor: true, //nolint:staticcheck
				CustomQueriesConfigMap: []ConfigMapKeyRefOptions{
					{Name: "custom-queries", Key: "queries.yaml"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.Monitoring == nil {
		t.Fatal("expected non-nil Monitoring")
	}
	if !obj.Spec.Monitoring.EnablePodMonitor { //nolint:staticcheck
		t.Error("expected EnablePodMonitor=true")
	}
	if len(obj.Spec.Monitoring.CustomQueriesConfigMap) != 1 {
		t.Fatalf("expected 1 custom query, got %d", len(obj.Spec.Monitoring.CustomQueriesConfigMap))
	}
}

func TestCluster_BootstrapRecovery(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Bootstrap: &BootstrapOptions{RecoverySource: "pg-old"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.Bootstrap == nil || obj.Spec.Bootstrap.Recovery == nil {
		t.Fatal("expected non-nil Bootstrap.Recovery")
	}
	if obj.Spec.Bootstrap.Recovery.Source != "pg-old" {
		t.Errorf("unexpected recovery source: %s", obj.Spec.Bootstrap.Recovery.Source)
	}
}

func TestCluster_BootstrapPgBaseBackup(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			Bootstrap: &BootstrapOptions{PgBasebackupSource: "pg-primary"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.Bootstrap == nil || obj.Spec.Bootstrap.PgBaseBackup == nil {
		t.Fatal("expected non-nil Bootstrap.PgBaseBackup")
	}
	if obj.Spec.Bootstrap.PgBaseBackup.Source != "pg-primary" {
		t.Errorf("unexpected pgbasebackup source: %s", obj.Spec.Bootstrap.PgBaseBackup.Source)
	}
}

func TestCluster_ExternalClusters(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			ExternalClusters: []ExternalClusterOptions{
				{
					Name:                 "pg-old",
					ConnectionParameters: map[string]string{"host": "pg-old.example.com"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obj.Spec.ExternalClusters) != 1 {
		t.Fatalf("expected 1 external cluster, got %d", len(obj.Spec.ExternalClusters))
	}
	if obj.Spec.ExternalClusters[0].Name != "pg-old" {
		t.Errorf("unexpected external cluster name: %s", obj.Spec.ExternalClusters[0].Name)
	}
}

func TestCluster_ExternalClusters_WithBarmanObjectStore(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			ExternalClusters: []ExternalClusterOptions{
				{
					Name: "pg-old",
					BarmanObjectStore: map[string]any{
						"destinationPath": "s3://bucket/pg-old/",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obj.Spec.ExternalClusters) != 1 {
		t.Fatalf("expected 1 external cluster, got %d", len(obj.Spec.ExternalClusters))
	}
	if obj.Spec.ExternalClusters[0].BarmanObjectStore == nil {
		t.Fatal("expected non-nil BarmanObjectStore")
	}
}

func TestCluster_PostgresParams(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances:      1,
			PostgresParams: map[string]string{"max_connections": "200"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.PostgresConfiguration.Parameters["max_connections"] != "200" {
		t.Error("PostgresParams not set")
	}
}

func TestCluster_Synchronous(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 3,
			Synchronous: &SynchronousOptions{
				Method:         "any",
				Number:         1,
				DataDurability: "required",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.PostgresConfiguration.Synchronous == nil {
		t.Fatal("expected non-nil Synchronous")
	}
	if string(obj.Spec.PostgresConfiguration.Synchronous.Method) != "any" {
		t.Errorf("unexpected Synchronous.Method: %s", obj.Spec.PostgresConfiguration.Synchronous.Method)
	}
}

func TestCluster_Affinity(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 3,
			Affinity: &AffinityOptions{
				EnablePodAntiAffinity: true,
				TopologyKey:           "kubernetes.io/hostname",
				PodAntiAffinityType:   "preferred",
				NodeSelector:          map[string]string{"node-type": "db"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.Affinity.EnablePodAntiAffinity == nil || !*obj.Spec.Affinity.EnablePodAntiAffinity {
		t.Error("expected EnablePodAntiAffinity=true")
	}
	if obj.Spec.Affinity.TopologyKey != "kubernetes.io/hostname" {
		t.Errorf("unexpected TopologyKey: %s", obj.Spec.Affinity.TopologyKey)
	}
}

func TestCluster_ManagedRoles_FullOptions(t *testing.T) {
	connLimit := int64(10)
	inheritFalse := false
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			ManagedRoles: []ManagedRoleOptions{
				{
					Name:            "readonly",
					Login:           true,
					ConnectionLimit: &connLimit,
					Inherit:         &inheritFalse,
					Ensure:          "absent",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obj.Spec.Managed.Roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(obj.Spec.Managed.Roles))
	}
	role := obj.Spec.Managed.Roles[0]
	if role.ConnectionLimit != 10 {
		t.Errorf("expected ConnectionLimit=10, got %d", role.ConnectionLimit)
	}
}

func TestCluster_InheritedMetadata(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances:       1,
			InheritedLabels: map[string]string{"team": "backend"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Spec.InheritedMetadata == nil {
		t.Fatal("expected non-nil InheritedMetadata")
	}
	if obj.Spec.InheritedMetadata.Labels["team"] != "backend" {
		t.Error("InheritedLabels not set")
	}
}

func TestCluster_NilOptions(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{Name: "pg", Namespace: "ns"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj == nil {
		t.Fatal("expected non-nil cluster")
	}
	if obj.Spec.Instances != 0 {
		t.Errorf("expected 0 instances, got %d", obj.Spec.Instances)
	}
}
