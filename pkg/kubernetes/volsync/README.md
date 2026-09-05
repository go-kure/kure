# VolSync Builders - ReplicationSource and ReplicationDestination

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/volsync.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/volsync)

The `volsync` package provides strongly-typed constructor functions for VolSync (`volsync.backube/v1alpha1`) resources. It is the canonical entry point for building `ReplicationSource` and `ReplicationDestination` objects in kure.

## Overview

VolSync replicates persistent volume data between Kubernetes clusters. Each replication direction (source and destination) selects exactly one *mover* — the data-transfer backend — from a sum of choices: Restic, Rsync (legacy SSH), RsyncTLS, Rclone, Syncthing (source-only), or an External passthrough.

This package encodes the mover one-of as a **sealed-interface sum type**: `Mover` on the parent Config holds exactly one variant. Setting two movers is a compile error; setting none is detected at construction. See [`docs/ARCHITECTURE.md` § One-of Constraints](/concepts/architecture/#one-of-constraints-sealed-interfaces) for the rationale.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

```go
obj := volsync.CreateReplicationSource("db-backup", "data")
```

The config-struct builders (`volsync.ReplicationSource(&volsync.ReplicationSourceConfig{...})`) are a separate, opinionated layer on top of the same upstream types; they are unchanged by the generated constructors. No hand-written `Create*` helper for a spec fragment remains — a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.

## Supported Resources

| Resource | Movers |
|---|---|
| `ReplicationSource` | Restic · Rsync · RsyncTLS · Rclone · Syncthing · External |
| `ReplicationDestination` | Restic · Rsync · RsyncTLS · Rclone · External |

## ReplicationSource

```go
import (
    volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"

    "github.com/go-kure/kure/pkg/kubernetes/volsync"
)

schedule := "@hourly"
rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
    Name:      "db-backup",
    Namespace: "data",
    SourcePVC: "postgres-data",
    Trigger:   &volsync.TriggerConfig{Schedule: &schedule},
    Mover: &volsync.SourceResticConfig{
        Repository: "restic-creds",
        ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
            CopyMethod: volsync.CopyMethodSnapshot,
        },
        Retain: &volsyncv1alpha1.ResticRetainPolicy{
            Daily:   ptr.Int32(7),
            Weekly:  ptr.Int32(4),
            Monthly: ptr.Int32(12),
        },
    },
})
```

## ReplicationDestination

```go
rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
    Name:      "db-restore",
    Namespace: "dr",
    Trigger:   &volsync.TriggerConfig{Manual: "restore-1"},
    Mover: &volsync.DestinationResticConfig{
        Repository: "restic-creds",
        ReplicationDestinationVolumeOptions: volsyncv1alpha1.ReplicationDestinationVolumeOptions{
            CopyMethod:  volsync.CopyMethodSnapshot,
            Capacity:    resource.MustParse("10Gi").DeepCopy(),
            AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
        },
    },
})
```

## Movers

| Mover | Purpose |
|---|---|
| `Restic` | Encrypted, deduplicated snapshots to a restic repository (S3, B2, etc.). |
| `Rsync` | Legacy SSH-based rsync. Prefer `RsyncTLS` for new deployments. |
| `RsyncTLS` | rsync over a TLS pre-shared key. |
| `Rclone` | Sync to/from any rclone-supported backend (S3, GCS, Azure, etc.). |
| `Syncthing` | Continuous bidirectional sync over the Syncthing protocol (source resource only). |
| `External` | Passthrough for custom replication providers — opaque `provider` + `parameters`. |

## Modifying a resource

The config struct is the only mutation surface this package offers. Trigger,
paused, source PVC and the mover variant are all fields on
`ReplicationSourceConfig` / `ReplicationDestinationConfig`, so build the
resource the way you want it:

```go
schedule := "@daily"
rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
    Name:      "data-backup",
    Namespace: "apps",
    SourcePVC: "data",
    Paused:    true,
    Trigger:   &volsync.TriggerConfig{Schedule: &schedule},
    Mover:     &volsync.SourceRcloneConfig{ /* ... */ },
})
```

To change a built resource, assign the field:

```go
rs.Spec.Paused = false
rs.Spec.Trigger = &volsyncv1alpha1.ReplicationSourceTriggerSpec{Manual: "go"}
```

The per-field `SetReplicationSource*` / `SetReplicationDestination*` helpers are <!-- doc-api-refs:ignore names removed helper families to say they are gone -->
gone. They were bare field assignments, and the mover ones duplicated the
constructor's type switch — the constructor is the one place that knows a
mover variant clears its siblings. Switching a mover on an existing resource
means clearing the other five arms yourself, which is exactly the multi-field
write [the builder contract](/api-reference/kubernetes-builders/) forbids a
`Set<Field>` helper from hiding.

The one appender remains, because a peer list is a slice the contract admits
sugar for:

```go
volsync.AddSyncthingPeer(syncCfg, "tcp://peer:22000", "PEER-ID", false)
```

## Related Packages

- [kubernetes-builders](/api-reference/kubernetes-builders/) — broader resource builder family
- [stack](/api-reference/stack/) — domain model that produces Kubernetes resources
