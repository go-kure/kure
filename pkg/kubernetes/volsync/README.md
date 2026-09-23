# VolSync Builders - ReplicationSource and ReplicationDestination

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/volsync.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/volsync)

The `volsync` package provides the generated constructors and the admissible sugar for VolSync (`volsync.backube/v1alpha1`) resources. It is the canonical entry point for building `ReplicationSource` and `ReplicationDestination` objects in kure.

## Overview

VolSync replicates persistent volume data between Kubernetes clusters. Each replication direction (source and destination) selects exactly one *mover* — the data-transfer backend — from a sum of choices: Restic, Rsync (legacy SSH), RsyncTLS, Rclone, Syncthing (source-only), or an External passthrough.

The upstream spec encodes that one-of as a pointer per arm (`Spec.Restic`, `Spec.Rsync`, `Spec.RsyncTLS`, `Spec.Rclone`, `Spec.Syncthing`, `Spec.External`); set the one you mean and leave the others nil. VolSync rejects a spec with two arms at apply time. See [`docs/ARCHITECTURE.md` § One-of Constraints](/concepts/architecture/#one-of-constraints) for how kure treats these.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Both kinds are namespaced and take `(name, namespace)`.

```go
obj := volsync.CreateReplicationSource("db-backup", "data")
```

There is no second construction path. The config-struct layer this package used to carry (`volsync.ReplicationSource(&volsync.ReplicationSourceConfig{...})`, `ReplicationDestination`, the `TriggerConfig` and the sealed `SourceMover` / `DestinationMover` sums with their nine defined-over-upstream mover types) was retired by release 2 of the builder contract: the movers were the upstream specs under another name, and the sealed interfaces walled off nothing the upstream struct did not already carry. The [release 2 migration notes](/concepts/builder-contract-release-2/) map every removed field to the upstream field that replaces it. No hand-written `Create*` helper for a spec fragment remains either — a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the migration ledgers.

## Supported Resources

| Resource | Movers |
|---|---|
| `ReplicationSource` | Restic · Rsync · RsyncTLS · Rclone · Syncthing · External |
| `ReplicationDestination` | Restic · Rsync · RsyncTLS · Rclone · External |

## ReplicationSource

```go
import (
    volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
    "k8s.io/utils/ptr"

    "github.com/go-kure/kure/pkg/kubernetes/volsync"
)

rs := volsync.CreateReplicationSource("db-backup", "data")
rs.Spec = volsyncv1alpha1.ReplicationSourceSpec{
    SourcePVC: "postgres-data",
    Trigger:   &volsyncv1alpha1.ReplicationSourceTriggerSpec{Schedule: ptr.To("@hourly")},
    Restic: &volsyncv1alpha1.ReplicationSourceResticSpec{
        Repository: "restic-creds",
        ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
            CopyMethod: volsync.CopyMethodSnapshot,
        },
        Retain: &volsyncv1alpha1.ResticRetainPolicy{
            Daily:   ptr.To[int32](7),
            Weekly:  ptr.To[int32](4),
            Monthly: ptr.To[int32](12),
        },
    },
}
```

## ReplicationDestination

```go
capacity := resource.MustParse("10Gi")

rd := volsync.CreateReplicationDestination("db-restore", "dr")
rd.Spec = volsyncv1alpha1.ReplicationDestinationSpec{
    Trigger: &volsyncv1alpha1.ReplicationDestinationTriggerSpec{Manual: "restore-1"},
    Restic: &volsyncv1alpha1.ReplicationDestinationResticSpec{
        Repository: "restic-creds",
        ReplicationDestinationVolumeOptions: volsyncv1alpha1.ReplicationDestinationVolumeOptions{
            CopyMethod:  volsync.CopyMethodSnapshot,
            Capacity:    &capacity,
            AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
        },
    },
}
```

## Movers

| Mover | Spec field | Purpose |
|---|---|---|
| `Restic` | `Spec.Restic` | Encrypted, deduplicated snapshots to a restic repository (S3, B2, etc.). |
| `Rsync` | `Spec.Rsync` | Legacy SSH-based rsync. Prefer `RsyncTLS` for new deployments. |
| `RsyncTLS` | `Spec.RsyncTLS` | rsync over a TLS pre-shared key. |
| `Rclone` | `Spec.Rclone` | Sync to/from any rclone-supported backend (S3, GCS, Azure, etc.). |
| `Syncthing` | `Spec.Syncthing` | Continuous bidirectional sync over the Syncthing protocol (source resource only). |
| `External` | `Spec.External` | Passthrough for custom replication providers — opaque `provider` + `parameters`. |

## Modifying a resource

The upstream struct is the only mutation surface this package offers. To change a built resource, assign the field:

```go
rs.Spec.Paused = false
rs.Spec.Trigger = &volsyncv1alpha1.ReplicationSourceTriggerSpec{Manual: "go"}
```

Switching a mover on an existing resource means clearing the other arms yourself — five on a source, four on a destination, which has no Syncthing mover — exactly the multi-field write [the builder contract](/api-reference/kubernetes-builders/) forbids a `Set<Field>` helper from hiding.

The one appender remains, because a peer list is a slice the contract admits sugar for; it takes the upstream Syncthing spec, which is the value assigned to `Spec.Syncthing`:

```go
rs.Spec.Syncthing = &volsyncv1alpha1.ReplicationSourceSyncthingSpec{}
volsync.AddSyncthingPeer(rs.Spec.Syncthing, "tcp://peer:22000", "PEER-ID", false)
```

`CopyMethod` and its four constants (`CopyMethodDirect`, `CopyMethodNone`, `CopyMethodClone`, `CopyMethodSnapshot`) re-export the upstream copy-method type for convenience; `volsyncv1alpha1.CopyMethodSnapshot` is the same value.

## Related Packages

- [kubernetes-builders](/api-reference/kubernetes-builders/) — broader resource builder family
- [stack](/api-reference/stack/) — domain model that produces Kubernetes resources
- [Builder contract: release 2 migration notes](/concepts/builder-contract-release-2/) — the retired config-struct layer, field by field
