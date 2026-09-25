package cnpg_test

import (
	"fmt"

	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	machineryapi "github.com/cloudnative-pg/machinery/pkg/api"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/kubernetes/cnpg"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

func ExampleCreateCluster() {
	obj := cnpg.CreateCluster("pg-main", "databases")
	cl := cnpg.CreateClusterImageCatalog("postgres-images")
	fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name, cl.Kind, cl.Name)
	// Output: Cluster databases/pg-main ClusterImageCatalog postgres-images
}

func ExampleAddClusterManagedRole() {
	cluster := cnpg.CreateCluster("pg-main", "databases")
	cluster.Spec = cnpgv1.ClusterSpec{
		Instances:             3,
		ImageName:             "ghcr.io/cloudnative-pg/postgresql:16",
		EnablePDB:             ptr.To(true),
		PrimaryUpdateStrategy: cnpgv1.PrimaryUpdateStrategyUnsupervised,
		StorageConfiguration:  cnpgv1.StorageConfiguration{Size: "10Gi"},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")},
		},
	}

	kubernetes.AddLabel(cluster, "env", "prod")
	cnpg.AddClusterManagedRole(cluster, cnpgv1.RoleConfiguration{Name: "appuser"})
	fmt.Println(cluster.Spec.Instances, cluster.Labels["env"], cluster.Spec.Managed.Roles[0].Name)
	// Output: 3 prod appuser
}

func ExampleCreateCluster_walArchive() {
	cluster := cnpg.CreateCluster("pg-main", "databases")

	cluster.Spec.Plugins = []cnpgv1.PluginConfiguration{{
		Name:          "barman-cloud.barmancloud.cnpg.io",
		IsWALArchiver: ptr.To(true),
		Parameters:    map[string]string{"objectStoreName": "pg-backup"},
	}}
	fmt.Println(cluster.Spec.Plugins[0].Name, *cluster.Spec.Plugins[0].IsWALArchiver)
	// Output: barman-cloud.barmancloud.cnpg.io true
}

func ExampleAddDatabaseExtension() {
	db := cnpg.CreateDatabase("app-db", "databases")
	db.Spec = cnpgv1.DatabaseSpec{
		ClusterRef: corev1.LocalObjectReference{Name: "pg-main"},
		Name:       "appdb",
		Owner:      "appuser",
	}

	cnpg.AddDatabaseExtension(db, cnpgv1.ExtensionSpec{
		DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{Name: "pgcrypto", Ensure: cnpgv1.EnsurePresent},
	})
	fmt.Println(db.Spec.Name, db.Spec.Extensions[0].Name, db.Spec.Extensions[0].Ensure)
	// Output: appdb pgcrypto present
}

func ExampleSetObjectStoreS3Credentials() {
	store := cnpg.CreateObjectStore("backup-store", "databases")
	store.Spec = barmanv1.ObjectStoreSpec{
		Configuration: barmanapi.BarmanObjectStoreConfiguration{
			DestinationPath: "s3://my-bucket/backups",
		},
		RetentionPolicy: "30d",
	}

	cnpg.SetObjectStoreS3Credentials(store, &barmanapi.S3Credentials{
		AccessKeyIDReference: &machineryapi.SecretKeySelector{
			LocalObjectReference: machineryapi.LocalObjectReference{Name: "backup-creds"},
			Key:                  "ACCESS_KEY_ID",
		},
		SecretAccessKeyReference: &machineryapi.SecretKeySelector{
			LocalObjectReference: machineryapi.LocalObjectReference{Name: "backup-creds"},
			Key:                  "SECRET_ACCESS_KEY",
		},
	})
	fmt.Println(store.Spec.Configuration.DestinationPath, store.Spec.Configuration.AWS.AccessKeyIDReference.Key)
	// Output: s3://my-bucket/backups ACCESS_KEY_ID
}

func ExampleSetScheduledBackupImmediate() {
	backup := cnpg.CreateScheduledBackup("daily-backup", "databases")
	backup.Spec = cnpgv1.ScheduledBackupSpec{
		Schedule: "0 0 2 * * *",
		Cluster:  cnpgv1.LocalObjectReference{Name: "pg-main"},
		Method:   cnpgv1.BackupMethodPlugin,
	}
	cnpg.SetScheduledBackupPluginConfiguration(backup, "barman-cloud.barmancloud.cnpg.io", map[string]string{"objectStoreName": "backup-store"})
	cnpg.SetScheduledBackupImmediate(backup, true)
	fmt.Println(backup.Spec.Schedule, backup.Spec.PluginConfiguration.Name, *backup.Spec.Immediate)
	// Output: 0 0 2 * * * barman-cloud.barmancloud.cnpg.io true
}

func ExampleCreatePooler() {
	pooler := cnpg.CreatePooler("pg-main-pooler", "databases")
	pooler.Spec = cnpgv1.PoolerSpec{
		Cluster:   cnpgv1.LocalObjectReference{Name: "pg-main"},
		Type:      cnpgv1.PoolerTypeRW,
		Instances: ptr.To[int32](2),
		PgBouncer: &cnpgv1.PgBouncerSpec{PoolMode: cnpgv1.PgBouncerPoolModeTransaction},
	}
	fmt.Println(pooler.Spec.Type, *pooler.Spec.Instances, pooler.Spec.PgBouncer.PoolMode)
	// Output: rw 2 transaction
}

func ExampleCreateCluster_monitoring() {
	cluster := cnpg.CreateCluster("pg-main", "databases")

	cluster.Spec.Monitoring = &cnpgv1.MonitoringConfiguration{
		EnablePodMonitor: true, //nolint:staticcheck
	}
	fmt.Println(cluster.Spec.Monitoring.EnablePodMonitor) //nolint:staticcheck
	// Output: true
}

func ExampleSetScheduledBackupSuspend() {
	cluster := cnpg.CreateCluster("pg-main", "databases")
	db := cnpg.CreateDatabase("app-db", "databases")
	store := cnpg.CreateObjectStore("backup-store", "databases")
	backup := cnpg.CreateScheduledBackup("daily-backup", "databases")
	role := cnpgv1.RoleConfiguration{Name: "appuser"}
	walConfig := &barmanapi.WalBackupConfiguration{}
	dataConfig := &barmanapi.DataBackupConfiguration{}

	// Labels and annotations use the generic helpers, which work over any object
	// with ObjectMeta -- this package carries no per-kind metadata helpers
	kubernetes.AddLabel(cluster, "app", "my-app")
	kubernetes.AddAnnotation(db, "note", "production")

	// Cluster
	cnpg.AddClusterManagedRole(cluster, role)

	// Database
	db.Spec.ClusterRef = corev1.LocalObjectReference{Name: "pg-main"}
	db.Spec.Owner = "appuser"
	db.Spec.ReclaimPolicy = cnpgv1.DatabaseReclaimDelete
	db.Spec.Ensure = cnpgv1.EnsurePresent

	// ObjectStore
	cnpg.AddObjectStoreEnvVar(store, corev1.EnvVar{Name: "AWS_REGION", Value: "eu-west-1"})
	cnpg.SetObjectStoreWalConfig(store, walConfig)
	cnpg.SetObjectStoreDataConfig(store, dataConfig)

	// ScheduledBackup
	cnpg.SetScheduledBackupSuspend(backup, true)
	backup.Spec.BackupOwnerReference = "self"

	fmt.Println(db.Spec.ReclaimPolicy, store.Spec.InstanceSidecarConfiguration.Env[0].Name, *backup.Spec.Suspend)
	// Output: delete AWS_REGION true
}
