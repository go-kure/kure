# CNPG Builders - CloudNativePG Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/cnpg.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/cnpg)

The `cnpg` package provides strongly-typed constructor functions for creating CloudNativePG (CNPG) and Barman Cloud Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each config-struct builder takes a configuration struct and returns a populated custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification. Domain-friendly option structs keep operator API types out of consumer configuration.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

```go
obj := cnpg.CreateCluster("pg-main", "databases")
cl := cnpg.CreateClusterImageCatalog("postgres-images")
```

The config-struct builders (`cnpg.Cluster(&cnpg.ClusterConfig{...})`) are a separate, opinionated layer on top of the same upstream types; they are unchanged by the generated constructors. The hand-written `Create*` helpers for spec fragments that remain in this package are legacy and are removed by the prune work item of the builder-contract epic.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.

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

### Monitoring

```go
cluster, err := cnpg.Cluster(&cnpg.ClusterConfig{
    Name:      "pg-main",
    Namespace: "databases",
    Options: &cnpg.ClusterOptions{
        Monitoring: &cnpg.MonitoringOptions{
            EnablePodMonitor: true,
        },
    },
})
if err != nil {
    // handle error
}
```

`MonitoringOptions.EnablePodMonitor` opts into the operator's built-in `PodMonitor` creation via
the upstream `MonitoringConfiguration.EnablePodMonitor` field. That upstream field is deprecated
(no replacement API exists yet upstream) — the operator's own deprecation notice recommends
creating the `PodMonitor` resource manually instead once that path lands. Until then this is the
only way to request pod-level metrics scraping through this builder.

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
