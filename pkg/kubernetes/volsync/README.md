# VolSync Builders - ReplicationSource and ReplicationDestination

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/volsync.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/volsync)

The `volsync` package provides strongly-typed constructor functions for VolSync (`volsync.backube/v1alpha1`) resources. It is the canonical entry point for building `ReplicationSource` and `ReplicationDestination` objects in kure.

## Overview

VolSync replicates persistent volume data between Kubernetes clusters. Each replication direction (source and destination) selects exactly one *mover* — the data-transfer backend — from a sum of choices: Restic, Rsync (legacy SSH), RsyncTLS, Rclone, Syncthing (source-only), or an External passthrough.

This package encodes the mover one-of as a **sealed-interface sum type**: `Mover` on the parent Config holds exactly one variant. Setting two movers is a compile error; setting none is detected at construction. See [`docs/ARCHITECTURE.md` § One-of Constraints](/concepts/architecture/#one-of-constraints-sealed-interfaces) for the rationale.

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

## Modifier Functions

```go
// Trigger
volsync.SetReplicationSourceSchedule(rs, "@daily")
volsync.SetReplicationSourceManualTrigger(rs, "go")
volsync.SetReplicationDestinationSchedule(rd, "@weekly")
volsync.SetReplicationDestinationManualTrigger(rd, "go")

// Spec fields
volsync.SetReplicationSourceSourcePVC(rs, "data")
volsync.SetReplicationSourcePaused(rs, true)
volsync.SetReplicationDestinationPaused(rd, true)

// Replace mover (clears any existing mover first)
volsync.SetReplicationSourceMover(rs, &volsync.SourceRcloneConfig{ /* ... */ })
volsync.SetReplicationDestinationMover(rd, &volsync.DestinationResticConfig{ /* ... */ })

// Syncthing peers
volsync.AddSyncthingPeer(syncCfg, "tcp://peer:22000", "PEER-ID", false)
```

## Related Packages

- [kubernetes-builders](/api-reference/kubernetes-builders/) — broader resource builder family
- [stack](/api-reference/stack/) — domain model that produces Kubernetes resources
