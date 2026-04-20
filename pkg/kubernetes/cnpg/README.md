# CNPG Builders - CloudNativePG Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/cnpg.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/cnpg)

The `cnpg` package provides strongly-typed constructor functions for creating CloudNativePG (CNPG) and Barman Cloud Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each function takes a configuration struct and returns a fully initialized custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

## Supported Resources

### Cluster

```go
import "github.com/go-kure/kure/pkg/kubernetes/cnpg"

cluster := cnpg.Cluster(&cnpg.ClusterConfig{
    Name:      "pg-main",
    Namespace: "databases",
    Spec:      cnpgv1.ClusterSpec{Instances: 3},
})

cnpg.AddClusterLabel(cluster, "env", "prod")
cnpg.AddClusterManagedRole(cluster, cnpgv1.RoleConfiguration{Name: "appuser"})
```

### Database

```go
db := cnpg.Database(&cnpg.DatabaseConfig{
    Name:      "app-db",
    Namespace: "databases",
    Spec:      cnpgv1.DatabaseSpec{Name: "appdb"},
})

cnpg.SetDatabaseClusterRef(db, "pg-main")
cnpg.SetDatabaseOwner(db, "appuser")
cnpg.AddDatabaseExtension(db, cnpgv1.ExtensionSpec{Name: "pgcrypto"})
```

### ObjectStore

```go
store := cnpg.ObjectStore(&cnpg.ObjectStoreConfig{
    Name:      "backup-store",
    Namespace: "databases",
    Spec:      barmanv1.ObjectStoreSpec{},
})

cnpg.SetObjectStoreDestinationPath(store, "s3://my-bucket/backups")
cnpg.SetObjectStoreS3Credentials(store, &barmanapi.S3Credentials{...})
cnpg.SetObjectStoreRetentionPolicy(store, "30d")
```

### ScheduledBackup

```go
backup := cnpg.ScheduledBackup(&cnpg.ScheduledBackupConfig{
    Name:      "daily-backup",
    Namespace: "databases",
    Spec:      cnpgv1.ScheduledBackupSpec{Schedule: "0 2 * * *"},
})

cnpg.SetScheduledBackupMethod(backup, cnpgv1.BackupMethodBarmanObjectStore)
cnpg.SetScheduledBackupImmediate(backup, true)
```

## Modifier Functions

All `Add*` and `Set*` functions from the internal package are re-exported here:

```go
// Labels and annotations
cnpg.AddClusterLabel(cluster, "app", "my-app")
cnpg.AddDatabaseAnnotation(db, "note", "production")

// Cluster
cnpg.AddClusterManagedRole(cluster, role)

// Database
cnpg.SetDatabaseClusterRef(db, "pg-main")
cnpg.SetDatabaseOwner(db, "appuser")
cnpg.SetDatabaseReclaimPolicy(db, cnpgv1.DatabaseReclaimDelete)
cnpg.SetDatabaseEnsure(db, cnpgv1.EnsurePresent)

// ObjectStore
cnpg.SetObjectStoreWalConfig(store, walConfig)
cnpg.SetObjectStoreDataConfig(store, dataConfig)

// ScheduledBackup
cnpg.SetScheduledBackupSuspend(backup, true)
cnpg.SetScheduledBackupBackupOwnerReference(backup, "self")
```

## Related Packages

- [stack](/api-reference/stack/) - Domain model that produces Kubernetes resources
