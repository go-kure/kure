# CNPG Builders - CloudNativePG Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/cnpg.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/cnpg)

The `cnpg` package provides the generated constructors and the admissible sugar for CloudNativePG (CNPG) and Barman Cloud Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Every kind is built the same way: the generated `Create<Kind>` wrapper gives you an object carrying identity only, and the upstream CNPG struct is the construction API — set `Spec` fields directly, or through the few `Set*`/`Add*` helpers the builder contract admits.

Each block on this page is the body of an `Example` function in `example_test.go`, which `go test`
runs: it imports this package as `cnpg`, `github.com/go-kure/kure/pkg/kubernetes`, the upstream
APIs as `cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"`,
`barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"`,
`barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"` and
`machineryapi "github.com/cloudnative-pg/machinery/pkg/api"`, `corev1 "k8s.io/api/core/v1"`,
`k8s.io/apimachinery/pkg/api/resource`, `k8s.io/utils/ptr`, and `fmt` for the line that prints
what the example built.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`.

<!-- doc-example: pkg/kubernetes/cnpg ExampleCreateCluster -->
```go
obj := cnpg.CreateCluster("pg-main", "databases")
cl := cnpg.CreateClusterImageCatalog("postgres-images")
fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name, cl.Kind, cl.Name)
```
<!-- doc-example:end -->

There is no second construction path.
The config-struct layer this package used to carry (`cnpg.Cluster(&cnpg.ClusterConfig{Options: &cnpg.ClusterOptions{...}})`, `Database`, `ObjectStore`, `ScheduledBackup`, `Pooler` and their twenty-one `*Config` / `*Options` types) was retired by release 2 of the builder contract: it reached 15 of a `Cluster`'s 55 spec fields, 6 of a `Database`'s 23 and 4 of a `Pooler`'s 9, and it invented values the caller never asked for — `enablePDB` from the instance count, a pinned `primaryUpdateStrategy`, the `ACCESS_KEY_ID` / `SECRET_ACCESS_KEY` key names, the barman-cloud plugin entry, a pooler `type` coerced to `rw`, an extension's `ensure`. Those are the caller's own lines now. <!-- doc-api-refs:ignore names the retired config-struct layer -->
The [release 2 migration notes](/concepts/builder-contract-release-2/) map every removed field to the upstream field that replaces it. No hand-written `Create*` helper for a spec fragment remains either — a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the migration ledgers.

## Supported Resources

### Cluster

<!-- doc-example: pkg/kubernetes/cnpg ExampleAddClusterManagedRole -->
```go
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
```
<!-- doc-example:end -->

Quantities are parsed at the call site — `resource.MustParse` for a literal, `resource.ParseQuantity` when the text comes from configuration — the same ruling the release 1 `SetResourceRequest` / `SetResourceLimit` helpers follow. A WAL archive through the barman-cloud plugin is one entry in `Spec.Plugins`:

<!-- doc-example: pkg/kubernetes/cnpg ExampleCreateCluster_walArchive -->
```go
cluster := cnpg.CreateCluster("pg-main", "databases")

cluster.Spec.Plugins = []cnpgv1.PluginConfiguration{{
    Name:          "barman-cloud.barmancloud.cnpg.io",
    IsWALArchiver: ptr.To(true),
    Parameters:    map[string]string{"objectStoreName": "pg-backup"},
}}
fmt.Println(cluster.Spec.Plugins[0].Name, *cluster.Spec.Plugins[0].IsWALArchiver)
```
<!-- doc-example:end -->

### Database

<!-- doc-example: pkg/kubernetes/cnpg ExampleAddDatabaseExtension -->
```go
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
```
<!-- doc-example:end -->

`Ensure` and `ReclaimPolicy` are the upstream constants (`cnpgv1.EnsureAbsent`, `cnpgv1.DatabaseReclaimDelete`); leaving them empty leaves the field out and lets the operator default it.

### ObjectStore

<!-- doc-example: pkg/kubernetes/cnpg ExampleSetObjectStoreS3Credentials -->
```go
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
```
<!-- doc-example:end -->

### ScheduledBackup

<!-- doc-example: pkg/kubernetes/cnpg ExampleSetScheduledBackupImmediate -->
```go
backup := cnpg.CreateScheduledBackup("daily-backup", "databases")
backup.Spec = cnpgv1.ScheduledBackupSpec{
    Schedule: "0 0 2 * * *",
    Cluster:  cnpgv1.LocalObjectReference{Name: "pg-main"},
    Method:   cnpgv1.BackupMethodPlugin,
}
cnpg.SetScheduledBackupPluginConfiguration(backup, "barman-cloud.barmancloud.cnpg.io", map[string]string{"objectStoreName": "backup-store"})
cnpg.SetScheduledBackupImmediate(backup, true)
fmt.Println(backup.Spec.Schedule, backup.Spec.PluginConfiguration.Name, *backup.Spec.Immediate)
```
<!-- doc-example:end -->

### Pooler

<!-- doc-example: pkg/kubernetes/cnpg ExampleCreatePooler -->
```go
pooler := cnpg.CreatePooler("pg-main-pooler", "databases")
pooler.Spec = cnpgv1.PoolerSpec{
    Cluster:   cnpgv1.LocalObjectReference{Name: "pg-main"},
    Type:      cnpgv1.PoolerTypeRW,
    Instances: ptr.To[int32](2),
    PgBouncer: &cnpgv1.PgBouncerSpec{PoolMode: cnpgv1.PgBouncerPoolModeTransaction},
}
fmt.Println(pooler.Spec.Type, *pooler.Spec.Instances, pooler.Spec.PgBouncer.PoolMode)
```
<!-- doc-example:end -->

`PgBouncer` is a required pointer upstream (no `omitempty`), so a pooler always carries a `pgbouncer:` block — an empty `&cnpgv1.PgBouncerSpec{}` when there is nothing to set.

### Monitoring

<!-- doc-example: pkg/kubernetes/cnpg ExampleCreateCluster_monitoring -->
```go
cluster := cnpg.CreateCluster("pg-main", "databases")

cluster.Spec.Monitoring = &cnpgv1.MonitoringConfiguration{
    EnablePodMonitor: true, //nolint:staticcheck
}
fmt.Println(cluster.Spec.Monitoring != nil)
```
<!-- doc-example:end -->

`EnablePodMonitor` opts into the operator's built-in `PodMonitor` creation. That upstream field is deprecated (no replacement API exists yet upstream) — the operator's own deprecation notice recommends creating the `PodMonitor` resource manually instead once that path lands. Until then it is the only way to request pod-level metrics scraping through this struct.

## Modifier Functions

The `Add*` and `Set*` helpers write one spec field each; everything else is a
direct field assignment:

<!-- doc-example: pkg/kubernetes/cnpg ExampleSetScheduledBackupSuspend -->
```go
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
```
<!-- doc-example:end -->

## Related Packages

- [stack](/api-reference/stack/) - Domain model that produces Kubernetes resources
- [Builder contract: release 2 migration notes](/concepts/builder-contract-release-2/) - the retired config-struct layer, field by field
