package cnpg_test

import (
	"fmt"

	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	machineryapi "github.com/cloudnative-pg/machinery/pkg/api"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/internal/cnpg"
)

// This example demonstrates creating a CNPG Database CR with extensions,
// which is the primary pattern for managing PostgreSQL databases via
// CloudNativePG.
func Example_composeDatabaseWithExtensions() {
	db := cnpg.CreateDatabase("app-db", "postgres-system", cnpgv1.DatabaseSpec{
		Name:       "app",
		Owner:      "app_user",
		ClusterRef: corev1.LocalObjectReference{Name: "pg-cluster"},
	})

	_ = cnpg.SetDatabaseReclaimPolicy(db, cnpgv1.DatabaseReclaimRetain)
	_ = cnpg.AddDatabaseLabel(db, "app", "myapp")

	_ = cnpg.AddDatabaseExtension(db, cnpgv1.ExtensionSpec{
		DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{
			Name:   "pg_stat_statements",
			Ensure: cnpgv1.EnsurePresent,
		},
	})
	_ = cnpg.AddDatabaseExtension(db, cnpgv1.ExtensionSpec{
		DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{
			Name:   "pgcrypto",
			Ensure: cnpgv1.EnsurePresent,
		},
	})

	fmt.Println("Name:", db.Name)
	fmt.Println("Kind:", db.Kind)
	fmt.Println("APIVersion:", db.APIVersion)
	fmt.Println("DB Name:", db.Spec.Name)
	fmt.Println("Owner:", db.Spec.Owner)
	fmt.Println("Cluster:", db.Spec.ClusterRef.Name)
	fmt.Println("ReclaimPolicy:", db.Spec.ReclaimPolicy)
	fmt.Println("Extensions:", len(db.Spec.Extensions))
	fmt.Println("Ext[0]:", db.Spec.Extensions[0].Name)
	fmt.Println("Ext[1]:", db.Spec.Extensions[1].Name)
	// Output:
	// Name: app-db
	// Kind: Database
	// APIVersion: postgresql.cnpg.io/v1
	// DB Name: app
	// Owner: app_user
	// Cluster: pg-cluster
	// ReclaimPolicy: retain
	// Extensions: 2
	// Ext[0]: pg_stat_statements
	// Ext[1]: pgcrypto
}

// This example demonstrates creating a CNPG ObjectStore CR with S3
// credentials, WAL compression, and retention policy — the standard
// pattern for configuring PostgreSQL backups to object storage.
func Example_composeObjectStoreWithS3() {
	os := cnpg.CreateObjectStore("pg-backup", "postgres-system", barmanv1.ObjectStoreSpec{
		Configuration: barmanapi.BarmanObjectStoreConfiguration{
			DestinationPath: "s3://pg-backups/production",
		},
	})

	_ = cnpg.SetObjectStoreEndpointURL(os, "https://s3.eu-west-1.amazonaws.com")
	_ = cnpg.SetObjectStoreS3Credentials(os, &barmanapi.S3Credentials{
		AccessKeyIDReference: &machineryapi.SecretKeySelector{
			LocalObjectReference: machineryapi.LocalObjectReference{Name: "aws-creds"},
			Key:                  "ACCESS_KEY_ID",
		},
		SecretAccessKeyReference: &machineryapi.SecretKeySelector{
			LocalObjectReference: machineryapi.LocalObjectReference{Name: "aws-creds"},
			Key:                  "SECRET_ACCESS_KEY",
		},
	})
	_ = cnpg.SetObjectStoreRetentionPolicy(os, "60d")
	_ = cnpg.SetObjectStoreWalConfig(os, &barmanapi.WalBackupConfiguration{
		Compression: "gzip",
		MaxParallel: 4,
	})
	_ = cnpg.AddObjectStoreEnvVar(os, corev1.EnvVar{
		Name:  "AWS_REGION",
		Value: "eu-west-1",
	})
	_ = cnpg.AddObjectStoreLabel(os, "backup-target", "production")

	fmt.Println("Name:", os.Name)
	fmt.Println("Kind:", os.Kind)
	fmt.Println("APIVersion:", os.APIVersion)
	fmt.Println("DestinationPath:", os.Spec.Configuration.DestinationPath)
	fmt.Println("EndpointURL:", os.Spec.Configuration.EndpointURL)
	fmt.Println("RetentionPolicy:", os.Spec.RetentionPolicy)
	fmt.Println("WAL Compression:", os.Spec.Configuration.Wal.Compression)
	fmt.Println("WAL MaxParallel:", os.Spec.Configuration.Wal.MaxParallel)
	fmt.Println("Env:", os.Spec.InstanceSidecarConfiguration.Env[0].Name)
	// Output:
	// Name: pg-backup
	// Kind: ObjectStore
	// APIVersion: barmancloud.cnpg.io/v1
	// DestinationPath: s3://pg-backups/production
	// EndpointURL: https://s3.eu-west-1.amazonaws.com
	// RetentionPolicy: 60d
	// WAL Compression: gzip
	// WAL MaxParallel: 4
	// Env: AWS_REGION
}
