package volsync_test

import (
	"testing"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"

	"github.com/go-kure/kure/pkg/kubernetes/volsync"
)

func TestSetReplicationSourceSchedule(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	volsync.SetReplicationSourceSchedule(rs, "@daily")
	if rs.Spec.Trigger == nil || rs.Spec.Trigger.Schedule == nil || *rs.Spec.Trigger.Schedule != "@daily" {
		t.Errorf("schedule not set: %+v", rs.Spec.Trigger)
	}
}

func TestSetReplicationSourceManualTrigger(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	volsync.SetReplicationSourceManualTrigger(rs, "now")
	if rs.Spec.Trigger == nil || rs.Spec.Trigger.Manual != "now" {
		t.Errorf("manual trigger not set: %+v", rs.Spec.Trigger)
	}
}

func TestSetReplicationSourceSourcePVC(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	volsync.SetReplicationSourceSourcePVC(rs, "data-pvc")
	if rs.Spec.SourcePVC != "data-pvc" {
		t.Errorf("SourcePVC = %q", rs.Spec.SourcePVC)
	}
}

func TestSetReplicationSourcePaused(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	volsync.SetReplicationSourcePaused(rs, true)
	if !rs.Spec.Paused {
		t.Errorf("Paused not set")
	}
}

func TestSetReplicationSourceMover_ReplacesOldMover(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	rs.Spec.Restic = &volsyncv1alpha1.ReplicationSourceResticSpec{Repository: "old"}
	volsync.SetReplicationSourceMover(rs, &volsync.SourceRcloneConfig{
		RcloneConfig: strPtr("new"),
	})
	if rs.Spec.Restic != nil {
		t.Errorf("expected Restic to be cleared")
	}
	if rs.Spec.Rclone == nil || rs.Spec.Rclone.RcloneConfig == nil || *rs.Spec.Rclone.RcloneConfig != "new" {
		t.Errorf("rclone not set: %+v", rs.Spec.Rclone)
	}
}

func TestSetReplicationSourceMover_NilClearsAll(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	rs.Spec.Restic = &volsyncv1alpha1.ReplicationSourceResticSpec{Repository: "old"}
	volsync.SetReplicationSourceMover(rs, nil)
	if rs.Spec.Restic != nil {
		t.Errorf("expected Restic to be cleared")
	}
}

func TestSetReplicationDestinationSchedule(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	volsync.SetReplicationDestinationSchedule(rd, "@weekly")
	if rd.Spec.Trigger == nil || rd.Spec.Trigger.Schedule == nil || *rd.Spec.Trigger.Schedule != "@weekly" {
		t.Errorf("schedule not set: %+v", rd.Spec.Trigger)
	}
}

func TestSetReplicationDestinationManualTrigger(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	volsync.SetReplicationDestinationManualTrigger(rd, "go")
	if rd.Spec.Trigger == nil || rd.Spec.Trigger.Manual != "go" {
		t.Errorf("manual trigger not set: %+v", rd.Spec.Trigger)
	}
}

func TestSetReplicationDestinationPaused(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	volsync.SetReplicationDestinationPaused(rd, true)
	if !rd.Spec.Paused {
		t.Errorf("Paused not set")
	}
}

func TestSetReplicationDestinationMover_ReplacesOldMover(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	rd.Spec.Restic = &volsyncv1alpha1.ReplicationDestinationResticSpec{Repository: "old"}
	volsync.SetReplicationDestinationMover(rd, &volsync.DestinationRcloneConfig{
		RcloneConfig: strPtr("new"),
	})
	if rd.Spec.Restic != nil {
		t.Errorf("expected Restic to be cleared")
	}
	if rd.Spec.Rclone == nil || rd.Spec.Rclone.RcloneConfig == nil || *rd.Spec.Rclone.RcloneConfig != "new" {
		t.Errorf("rclone not set: %+v", rd.Spec.Rclone)
	}
}

func TestSetReplicationSourceMover_TypedNilDoesNotPanic(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	var rcloneNil *volsync.SourceRcloneConfig
	volsync.SetReplicationSourceMover(rs, rcloneNil)
	if rs.Spec.Rclone != nil {
		t.Errorf("typed-nil Rclone should leave spec.Rclone unset")
	}
}

func TestSetReplicationDestinationMover_TypedNilDoesNotPanic(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	var resticNil *volsync.DestinationResticConfig
	volsync.SetReplicationDestinationMover(rd, resticNil)
	if rd.Spec.Restic != nil {
		t.Errorf("typed-nil Restic should leave spec.Restic unset")
	}
}

func TestAddSyncthingPeer(t *testing.T) {
	cfg := &volsync.SourceSyncthingConfig{}
	volsync.AddSyncthingPeer(cfg, "tcp://peer:22000", "PEER-ID", true)
	if len(cfg.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(cfg.Peers))
	}
	if cfg.Peers[0].ID != "PEER-ID" || cfg.Peers[0].Address != "tcp://peer:22000" || !cfg.Peers[0].Introducer {
		t.Errorf("peer not set correctly: %+v", cfg.Peers[0])
	}
}
