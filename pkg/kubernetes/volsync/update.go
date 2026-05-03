package volsync

import (
	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
)

// SetReplicationSourceSchedule sets the cron schedule on a ReplicationSource.
// Replaces any existing trigger.
func SetReplicationSourceSchedule(rs *volsyncv1alpha1.ReplicationSource, schedule string) {
	if rs == nil {
		return
	}
	if rs.Spec.Trigger == nil {
		rs.Spec.Trigger = &volsyncv1alpha1.ReplicationSourceTriggerSpec{}
	}
	s := schedule
	rs.Spec.Trigger.Schedule = &s
}

// SetReplicationSourceManualTrigger sets the manual trigger token on a
// ReplicationSource. Replaces any existing trigger.
func SetReplicationSourceManualTrigger(rs *volsyncv1alpha1.ReplicationSource, manual string) {
	if rs == nil {
		return
	}
	if rs.Spec.Trigger == nil {
		rs.Spec.Trigger = &volsyncv1alpha1.ReplicationSourceTriggerSpec{}
	}
	rs.Spec.Trigger.Manual = manual
}

// SetReplicationSourceSourcePVC sets the source PVC name.
func SetReplicationSourceSourcePVC(rs *volsyncv1alpha1.ReplicationSource, pvc string) {
	if rs == nil {
		return
	}
	rs.Spec.SourcePVC = pvc
}

// SetReplicationSourcePaused sets the paused flag.
func SetReplicationSourcePaused(rs *volsyncv1alpha1.ReplicationSource, paused bool) {
	if rs == nil {
		return
	}
	rs.Spec.Paused = paused
}

// SetReplicationSourceMover replaces the mover spec on a ReplicationSource. Any
// previously set mover is cleared. The mover must be a sealed SourceMover
// variant; nil clears all movers.
func SetReplicationSourceMover(rs *volsyncv1alpha1.ReplicationSource, mover SourceMover) {
	if rs == nil {
		return
	}
	rs.Spec.Restic = nil
	rs.Spec.Rsync = nil
	rs.Spec.RsyncTLS = nil
	rs.Spec.Rclone = nil
	rs.Spec.Syncthing = nil
	rs.Spec.External = nil
	switch m := mover.(type) {
	case *SourceResticConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceResticSpec(*m)
			rs.Spec.Restic = &spec
		}
	case *SourceRsyncConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceRsyncSpec(*m)
			rs.Spec.Rsync = &spec
		}
	case *SourceRsyncTLSConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceRsyncTLSSpec(*m)
			rs.Spec.RsyncTLS = &spec
		}
	case *SourceRcloneConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceRcloneSpec(*m)
			rs.Spec.Rclone = &spec
		}
	case *SourceSyncthingConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceSyncthingSpec(*m)
			rs.Spec.Syncthing = &spec
		}
	case *ExternalConfig:
		if m != nil {
			rs.Spec.External = &volsyncv1alpha1.ReplicationSourceExternalSpec{
				Provider:   m.Provider,
				Parameters: m.Parameters,
			}
		}
	}
}

// SetReplicationDestinationSchedule sets the cron schedule on a
// ReplicationDestination. Replaces any existing trigger.
func SetReplicationDestinationSchedule(rd *volsyncv1alpha1.ReplicationDestination, schedule string) {
	if rd == nil {
		return
	}
	if rd.Spec.Trigger == nil {
		rd.Spec.Trigger = &volsyncv1alpha1.ReplicationDestinationTriggerSpec{}
	}
	s := schedule
	rd.Spec.Trigger.Schedule = &s
}

// SetReplicationDestinationManualTrigger sets the manual trigger token on a
// ReplicationDestination. Replaces any existing trigger.
func SetReplicationDestinationManualTrigger(rd *volsyncv1alpha1.ReplicationDestination, manual string) {
	if rd == nil {
		return
	}
	if rd.Spec.Trigger == nil {
		rd.Spec.Trigger = &volsyncv1alpha1.ReplicationDestinationTriggerSpec{}
	}
	rd.Spec.Trigger.Manual = manual
}

// SetReplicationDestinationPaused sets the paused flag.
func SetReplicationDestinationPaused(rd *volsyncv1alpha1.ReplicationDestination, paused bool) {
	if rd == nil {
		return
	}
	rd.Spec.Paused = paused
}

// SetReplicationDestinationMover replaces the mover spec on a
// ReplicationDestination. Any previously set mover is cleared. nil clears all.
func SetReplicationDestinationMover(rd *volsyncv1alpha1.ReplicationDestination, mover DestinationMover) {
	if rd == nil {
		return
	}
	rd.Spec.Restic = nil
	rd.Spec.Rsync = nil
	rd.Spec.RsyncTLS = nil
	rd.Spec.Rclone = nil
	rd.Spec.External = nil
	switch m := mover.(type) {
	case *DestinationResticConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationDestinationResticSpec(*m)
			rd.Spec.Restic = &spec
		}
	case *DestinationRsyncConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationDestinationRsyncSpec(*m)
			rd.Spec.Rsync = &spec
		}
	case *DestinationRsyncTLSConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationDestinationRsyncTLSSpec(*m)
			rd.Spec.RsyncTLS = &spec
		}
	case *DestinationRcloneConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationDestinationRcloneSpec(*m)
			rd.Spec.Rclone = &spec
		}
	case *ExternalConfig:
		if m != nil {
			rd.Spec.External = &volsyncv1alpha1.ReplicationDestinationExternalSpec{
				Provider:   m.Provider,
				Parameters: m.Parameters,
			}
		}
	}
}

// AddSyncthingPeer appends a peer to a SourceSyncthingConfig.
func AddSyncthingPeer(cfg *SourceSyncthingConfig, address, id string, introducer bool) {
	if cfg == nil {
		return
	}
	cfg.Peers = append(cfg.Peers, volsyncv1alpha1.SyncthingPeer{
		Address:    address,
		ID:         id,
		Introducer: introducer,
	})
}
