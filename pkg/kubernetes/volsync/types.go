package volsync

import (
	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
)

// SourceMover is implemented by exactly the per-mover Config types valid for a
// ReplicationSource. The marker method is unexported to seal the interface;
// external packages cannot satisfy it.
type SourceMover interface {
	isSourceMover()
}

// DestinationMover is the analogous sealed interface for ReplicationDestination.
// Note: Syncthing is source-only upstream (bidirectional sync uses a single
// ReplicationSource), so SourceSyncthingConfig has no destination counterpart.
type DestinationMover interface {
	isDestinationMover()
}

// ReplicationSourceConfig is the public input for ReplicationSource construction.
// Mover holds exactly one variant; the constructor dispatches on the concrete type.
type ReplicationSourceConfig struct {
	Name      string
	Namespace string
	SourcePVC string
	Paused    bool
	Trigger   *TriggerConfig
	Mover     SourceMover
}

// ReplicationDestinationConfig is the public input for ReplicationDestination
// construction. Mover holds exactly one variant; the constructor dispatches on
// the concrete type.
type ReplicationDestinationConfig struct {
	Name      string
	Namespace string
	Paused    bool
	Trigger   *TriggerConfig
	Mover     DestinationMover
}

// TriggerConfig is the schedule/manual trigger for both source and destination.
// Schedule is a cron expression (or @hourly/@daily/etc.); Manual is an opaque
// token bumped by the caller to request a one-shot sync.
type TriggerConfig struct {
	Schedule *string
	Manual   string
}

// ExternalConfig is the passthrough mover for custom replication providers.
// Implements both SourceMover and DestinationMover.
type ExternalConfig struct {
	Provider   string
	Parameters map[string]string
}

func (*ExternalConfig) isSourceMover()      {}
func (*ExternalConfig) isDestinationMover() {}

// Per-mover Configs are defined types over upstream Specs. Field shape is
// identical to upstream so callers can populate them directly. Internal cast
// at construction time keeps the public surface free of upstream pointer wiring.
//
// Source-side movers:

type SourceResticConfig volsyncv1alpha1.ReplicationSourceResticSpec

func (*SourceResticConfig) isSourceMover() {}

type SourceRsyncConfig volsyncv1alpha1.ReplicationSourceRsyncSpec

func (*SourceRsyncConfig) isSourceMover() {}

type SourceRsyncTLSConfig volsyncv1alpha1.ReplicationSourceRsyncTLSSpec

func (*SourceRsyncTLSConfig) isSourceMover() {}

type SourceRcloneConfig volsyncv1alpha1.ReplicationSourceRcloneSpec

func (*SourceRcloneConfig) isSourceMover() {}

type SourceSyncthingConfig volsyncv1alpha1.ReplicationSourceSyncthingSpec

func (*SourceSyncthingConfig) isSourceMover() {}

// Destination-side movers:

type DestinationResticConfig volsyncv1alpha1.ReplicationDestinationResticSpec

func (*DestinationResticConfig) isDestinationMover() {}

type DestinationRsyncConfig volsyncv1alpha1.ReplicationDestinationRsyncSpec

func (*DestinationRsyncConfig) isDestinationMover() {}

type DestinationRsyncTLSConfig volsyncv1alpha1.ReplicationDestinationRsyncTLSSpec

func (*DestinationRsyncTLSConfig) isDestinationMover() {}

type DestinationRcloneConfig volsyncv1alpha1.ReplicationDestinationRcloneSpec

func (*DestinationRcloneConfig) isDestinationMover() {}

// CopyMethod re-exports the upstream CopyMethodType for caller convenience.
type CopyMethod = volsyncv1alpha1.CopyMethodType

const (
	CopyMethodDirect   = volsyncv1alpha1.CopyMethodDirect
	CopyMethodNone     = volsyncv1alpha1.CopyMethodNone
	CopyMethodClone    = volsyncv1alpha1.CopyMethodClone
	CopyMethodSnapshot = volsyncv1alpha1.CopyMethodSnapshot
)
