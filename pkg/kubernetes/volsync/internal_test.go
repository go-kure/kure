package volsync

import (
	"testing"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
)

// Tests for sealed interface marker methods and nil-guard paths.
// These must be in the same package to access unexported methods.

func TestSourceMoverMarkers(t *testing.T) {
	// Exercise the marker methods to achieve coverage.
	// Each of these is a single-line unexported method.
	var restic *SourceResticConfig = &SourceResticConfig{}
	restic.isSourceMover()

	var rsync *SourceRsyncConfig = &SourceRsyncConfig{}
	rsync.isSourceMover()

	var rsyncTLS *SourceRsyncTLSConfig = &SourceRsyncTLSConfig{}
	rsyncTLS.isSourceMover()

	var rclone *SourceRcloneConfig = &SourceRcloneConfig{}
	rclone.isSourceMover()

	var syncthing *SourceSyncthingConfig = &SourceSyncthingConfig{}
	syncthing.isSourceMover()

	var ext *ExternalConfig = &ExternalConfig{}
	ext.isSourceMover()
	ext.isDestinationMover()

	var dstRestic *DestinationResticConfig = &DestinationResticConfig{}
	dstRestic.isDestinationMover()

	var dstRsync *DestinationRsyncConfig = &DestinationRsyncConfig{}
	dstRsync.isDestinationMover()

	var dstRsyncTLS *DestinationRsyncTLSConfig = &DestinationRsyncTLSConfig{}
	dstRsyncTLS.isDestinationMover()

	var dstRclone *DestinationRcloneConfig = &DestinationRcloneConfig{}
	dstRclone.isDestinationMover()
}

func TestSetReplicationSourceSchedule_NilRS(t *testing.T) {
	// Should not panic
	SetReplicationSourceSchedule(nil, "@daily")
}

func TestSetReplicationSourceManualTrigger_NilRS(t *testing.T) {
	SetReplicationSourceManualTrigger(nil, "now")
}

func TestSetReplicationSourceSourcePVC_NilRS(t *testing.T) {
	SetReplicationSourceSourcePVC(nil, "data-pvc")
}

func TestSetReplicationSourcePaused_NilRS(t *testing.T) {
	SetReplicationSourcePaused(nil, true)
}

func TestSetReplicationSourceMover_NilRS(t *testing.T) {
	SetReplicationSourceMover(nil, &SourceResticConfig{})
}

func TestSetReplicationSourceSchedule_ExistingTrigger(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	rs.Spec.Trigger = &volsyncv1alpha1.ReplicationSourceTriggerSpec{}
	SetReplicationSourceSchedule(rs, "@daily")
	if rs.Spec.Trigger.Schedule == nil || *rs.Spec.Trigger.Schedule != "@daily" {
		t.Error("schedule not set on existing trigger")
	}
}

func TestSetReplicationSourceManualTrigger_ExistingTrigger(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	rs.Spec.Trigger = &volsyncv1alpha1.ReplicationSourceTriggerSpec{}
	SetReplicationSourceManualTrigger(rs, "trigger-1")
	if rs.Spec.Trigger.Manual != "trigger-1" {
		t.Error("manual trigger not set on existing trigger")
	}
}

func TestSetReplicationSourceMover_Restic(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	cfg := &SourceResticConfig{Repository: "restic-creds"}
	SetReplicationSourceMover(rs, cfg)
	if rs.Spec.Restic == nil || rs.Spec.Restic.Repository != "restic-creds" {
		t.Errorf("Restic not set: %+v", rs.Spec.Restic)
	}
}

func TestSetReplicationSourceMover_Rsync(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	addr := "dst.example.com"
	cfg := &SourceRsyncConfig{Address: &addr}
	SetReplicationSourceMover(rs, cfg)
	if rs.Spec.Rsync == nil || rs.Spec.Rsync.Address == nil || *rs.Spec.Rsync.Address != addr {
		t.Errorf("Rsync not set: %+v", rs.Spec.Rsync)
	}
}

func TestSetReplicationSourceMover_RsyncTLS(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	key := "tls-key"
	cfg := &SourceRsyncTLSConfig{KeySecret: &key}
	SetReplicationSourceMover(rs, cfg)
	if rs.Spec.RsyncTLS == nil || rs.Spec.RsyncTLS.KeySecret == nil || *rs.Spec.RsyncTLS.KeySecret != key {
		t.Errorf("RsyncTLS not set: %+v", rs.Spec.RsyncTLS)
	}
}

func TestSetReplicationSourceMover_Syncthing(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	cfg := &SourceSyncthingConfig{
		Peers: []volsyncv1alpha1.SyncthingPeer{{ID: "peer-id"}},
	}
	SetReplicationSourceMover(rs, cfg)
	if rs.Spec.Syncthing == nil || len(rs.Spec.Syncthing.Peers) != 1 {
		t.Errorf("Syncthing not set: %+v", rs.Spec.Syncthing)
	}
}

func TestSetReplicationSourceMover_External(t *testing.T) {
	rs := &volsyncv1alpha1.ReplicationSource{}
	cfg := &ExternalConfig{Provider: "example.com/foo", Parameters: map[string]string{"k": "v"}}
	SetReplicationSourceMover(rs, cfg)
	if rs.Spec.External == nil || rs.Spec.External.Provider != "example.com/foo" {
		t.Errorf("External not set: %+v", rs.Spec.External)
	}
}

func TestSetReplicationDestinationSchedule_NilRD(t *testing.T) {
	SetReplicationDestinationSchedule(nil, "@daily")
}

func TestSetReplicationDestinationManualTrigger_NilRD(t *testing.T) {
	SetReplicationDestinationManualTrigger(nil, "now")
}

func TestSetReplicationDestinationPaused_NilRD(t *testing.T) {
	SetReplicationDestinationPaused(nil, true)
}

func TestSetReplicationDestinationMover_NilRD(t *testing.T) {
	SetReplicationDestinationMover(nil, &DestinationResticConfig{})
}

func TestSetReplicationDestinationSchedule_ExistingTrigger(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	rd.Spec.Trigger = &volsyncv1alpha1.ReplicationDestinationTriggerSpec{}
	SetReplicationDestinationSchedule(rd, "@weekly")
	if rd.Spec.Trigger.Schedule == nil || *rd.Spec.Trigger.Schedule != "@weekly" {
		t.Error("schedule not set on existing trigger")
	}
}

func TestSetReplicationDestinationManualTrigger_ExistingTrigger(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	rd.Spec.Trigger = &volsyncv1alpha1.ReplicationDestinationTriggerSpec{}
	SetReplicationDestinationManualTrigger(rd, "trigger-1")
	if rd.Spec.Trigger.Manual != "trigger-1" {
		t.Error("manual trigger not set on existing trigger")
	}
}

func TestSetReplicationDestinationMover_Restic(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	cfg := &DestinationResticConfig{Repository: "restic-creds"}
	SetReplicationDestinationMover(rd, cfg)
	if rd.Spec.Restic == nil || rd.Spec.Restic.Repository != "restic-creds" {
		t.Errorf("Restic not set: %+v", rd.Spec.Restic)
	}
}

func TestSetReplicationDestinationMover_Rsync(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	key := "ssh-key"
	cfg := &DestinationRsyncConfig{SSHKeys: &key}
	SetReplicationDestinationMover(rd, cfg)
	if rd.Spec.Rsync == nil || rd.Spec.Rsync.SSHKeys == nil || *rd.Spec.Rsync.SSHKeys != key {
		t.Errorf("Rsync not set: %+v", rd.Spec.Rsync)
	}
}

func TestSetReplicationDestinationMover_RsyncTLS(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	key := "tls-key"
	cfg := &DestinationRsyncTLSConfig{KeySecret: &key}
	SetReplicationDestinationMover(rd, cfg)
	if rd.Spec.RsyncTLS == nil || rd.Spec.RsyncTLS.KeySecret == nil || *rd.Spec.RsyncTLS.KeySecret != key {
		t.Errorf("RsyncTLS not set: %+v", rd.Spec.RsyncTLS)
	}
}

func TestSetReplicationDestinationMover_Rclone(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	rcloneCfg := "rclone-config-secret"
	cfg := &DestinationRcloneConfig{RcloneConfig: &rcloneCfg}
	SetReplicationDestinationMover(rd, cfg)
	if rd.Spec.Rclone == nil || rd.Spec.Rclone.RcloneConfig == nil || *rd.Spec.Rclone.RcloneConfig != rcloneCfg {
		t.Errorf("Rclone not set: %+v", rd.Spec.Rclone)
	}
}

func TestSetReplicationDestinationMover_External(t *testing.T) {
	rd := &volsyncv1alpha1.ReplicationDestination{}
	cfg := &ExternalConfig{Provider: "example.com/foo", Parameters: map[string]string{"k": "v"}}
	SetReplicationDestinationMover(rd, cfg)
	if rd.Spec.External == nil || rd.Spec.External.Provider != "example.com/foo" {
		t.Errorf("External not set: %+v", rd.Spec.External)
	}
}

func TestAddSyncthingPeer_NilCfg(t *testing.T) {
	// Should not panic
	AddSyncthingPeer(nil, "tcp://peer:22000", "PEER-ID", true)
}
