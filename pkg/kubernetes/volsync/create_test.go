package volsync_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"

	"github.com/go-kure/kure/pkg/kubernetes/volsync"
)

func strPtr(s string) *string { return &s }
func i32Ptr(i int32) *int32   { return &i }

func TestReplicationSource_Nil(t *testing.T) {
	if got := volsync.ReplicationSource(nil); got != nil {
		t.Errorf("expected nil for nil cfg, got %+v", got)
	}
}

func TestReplicationSource_Restic(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name:      "db-backup",
		Namespace: "data",
		SourcePVC: "postgres-data",
		Trigger:   &volsync.TriggerConfig{Schedule: strPtr("@hourly")},
		Mover: &volsync.SourceResticConfig{
			Repository: "restic-creds",
			ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
				CopyMethod: volsync.CopyMethodSnapshot,
			},
			Retain: &volsyncv1alpha1.ResticRetainPolicy{
				Daily:   i32Ptr(7),
				Weekly:  i32Ptr(4),
				Monthly: i32Ptr(12),
			},
		},
	})

	if rs == nil {
		t.Fatal("nil result")
	}
	if rs.Spec.Restic == nil {
		t.Fatal("restic spec not set")
	}
	if rs.Spec.Rsync != nil || rs.Spec.RsyncTLS != nil || rs.Spec.Rclone != nil ||
		rs.Spec.Syncthing != nil || rs.Spec.External != nil {
		t.Errorf("only Restic should be set, got: rsync=%v rsyncTLS=%v rclone=%v syncthing=%v external=%v",
			rs.Spec.Rsync, rs.Spec.RsyncTLS, rs.Spec.Rclone, rs.Spec.Syncthing, rs.Spec.External)
	}
	if rs.Spec.SourcePVC != "postgres-data" {
		t.Errorf("SourcePVC = %q, want postgres-data", rs.Spec.SourcePVC)
	}
	if rs.Spec.Trigger == nil || rs.Spec.Trigger.Schedule == nil || *rs.Spec.Trigger.Schedule != "@hourly" {
		t.Errorf("Trigger schedule mismatch: %+v", rs.Spec.Trigger)
	}
	if rs.Spec.Restic.Repository != "restic-creds" {
		t.Errorf("Repository = %q, want restic-creds", rs.Spec.Restic.Repository)
	}
	if rs.Spec.Restic.CopyMethod != volsync.CopyMethodSnapshot {
		t.Errorf("CopyMethod = %v, want Snapshot", rs.Spec.Restic.CopyMethod)
	}
	if rs.Spec.Restic.Retain == nil || rs.Spec.Restic.Retain.Daily == nil || *rs.Spec.Restic.Retain.Daily != 7 {
		t.Errorf("Retain.Daily mismatch: %+v", rs.Spec.Restic.Retain)
	}
}

func TestReplicationSource_Rsync(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "rsync-src", Namespace: "ns",
		Mover: &volsync.SourceRsyncConfig{
			Address: strPtr("dst.example.com"),
		},
	})
	if rs.Spec.Rsync == nil || rs.Spec.Rsync.Address == nil || *rs.Spec.Rsync.Address != "dst.example.com" {
		t.Errorf("rsync address not propagated: %+v", rs.Spec.Rsync)
	}
}

func TestReplicationSource_RsyncTLS(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "tls-src", Namespace: "ns",
		Mover: &volsync.SourceRsyncTLSConfig{
			KeySecret: strPtr("tls-key"),
		},
	})
	if rs.Spec.RsyncTLS == nil || rs.Spec.RsyncTLS.KeySecret == nil || *rs.Spec.RsyncTLS.KeySecret != "tls-key" {
		t.Errorf("rsyncTLS keySecret not propagated: %+v", rs.Spec.RsyncTLS)
	}
}

func TestReplicationSource_Rclone(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "rclone-src", Namespace: "ns",
		Mover: &volsync.SourceRcloneConfig{
			RcloneConfig:        strPtr("rclone-config-secret"),
			RcloneConfigSection: strPtr("backup"),
			RcloneDestPath:      strPtr("remote:bucket/path"),
		},
	})
	if rs.Spec.Rclone == nil ||
		rs.Spec.Rclone.RcloneConfig == nil || *rs.Spec.Rclone.RcloneConfig != "rclone-config-secret" {
		t.Errorf("rclone config not propagated: %+v", rs.Spec.Rclone)
	}
}

func TestReplicationSource_Syncthing(t *testing.T) {
	cap := resource.MustParse("1Gi")
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "sync-src", Namespace: "ns",
		Mover: &volsync.SourceSyncthingConfig{
			Peers: []volsyncv1alpha1.SyncthingPeer{
				{Address: "tcp://peer:22000", ID: "PEER-ID-XX"},
			},
			ConfigCapacity: &cap,
		},
	})
	if rs.Spec.Syncthing == nil || len(rs.Spec.Syncthing.Peers) != 1 {
		t.Errorf("syncthing peers not propagated: %+v", rs.Spec.Syncthing)
	}
}

func TestReplicationSource_External(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "ext-src", Namespace: "ns",
		Mover: &volsync.ExternalConfig{
			Provider:   "example.com/foo",
			Parameters: map[string]string{"k": "v"},
		},
	})
	if rs.Spec.External == nil || rs.Spec.External.Provider != "example.com/foo" {
		t.Errorf("external provider not propagated: %+v", rs.Spec.External)
	}
	if rs.Spec.External.Parameters["k"] != "v" {
		t.Errorf("external params not propagated: %+v", rs.Spec.External)
	}
}

func TestReplicationSource_NilMover(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "no-mover", Namespace: "ns",
	})
	if rs.Spec.Restic != nil || rs.Spec.Rsync != nil || rs.Spec.RsyncTLS != nil ||
		rs.Spec.Rclone != nil || rs.Spec.Syncthing != nil || rs.Spec.External != nil {
		t.Errorf("expected no mover spec set, got: %+v", rs.Spec)
	}
}

func TestReplicationDestination_Nil(t *testing.T) {
	if got := volsync.ReplicationDestination(nil); got != nil {
		t.Errorf("expected nil for nil cfg, got %+v", got)
	}
}

func TestReplicationDestination_Restic(t *testing.T) {
	cap := resource.MustParse("10Gi")
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "db-restore", Namespace: "dr",
		Trigger: &volsync.TriggerConfig{Manual: "restore-1"},
		Mover: &volsync.DestinationResticConfig{
			Repository: "restic-creds",
			ReplicationDestinationVolumeOptions: volsyncv1alpha1.ReplicationDestinationVolumeOptions{
				CopyMethod:  volsync.CopyMethodSnapshot,
				Capacity:    &cap,
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
		},
	})
	if rd == nil || rd.Spec.Restic == nil {
		t.Fatal("destination restic spec not set")
	}
	if rd.Spec.Restic.Repository != "restic-creds" {
		t.Errorf("Repository = %q", rd.Spec.Restic.Repository)
	}
	if rd.Spec.Restic.CopyMethod != volsync.CopyMethodSnapshot {
		t.Errorf("CopyMethod = %v", rd.Spec.Restic.CopyMethod)
	}
	if rd.Spec.Trigger == nil || rd.Spec.Trigger.Manual != "restore-1" {
		t.Errorf("manual trigger not set: %+v", rd.Spec.Trigger)
	}
}

func TestReplicationDestination_Rsync(t *testing.T) {
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "rsync-dst", Namespace: "ns",
		Mover: &volsync.DestinationRsyncConfig{
			SSHKeys: strPtr("ssh-secret"),
		},
	})
	if rd.Spec.Rsync == nil || rd.Spec.Rsync.SSHKeys == nil || *rd.Spec.Rsync.SSHKeys != "ssh-secret" {
		t.Errorf("dst rsync sshKeys not propagated: %+v", rd.Spec.Rsync)
	}
}

func TestReplicationDestination_RsyncTLS(t *testing.T) {
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "tls-dst", Namespace: "ns",
		Mover: &volsync.DestinationRsyncTLSConfig{
			KeySecret: strPtr("psk"),
		},
	})
	if rd.Spec.RsyncTLS == nil || rd.Spec.RsyncTLS.KeySecret == nil || *rd.Spec.RsyncTLS.KeySecret != "psk" {
		t.Errorf("dst rsyncTLS keySecret not propagated: %+v", rd.Spec.RsyncTLS)
	}
}

func TestReplicationDestination_Rclone(t *testing.T) {
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "rclone-dst", Namespace: "ns",
		Mover: &volsync.DestinationRcloneConfig{
			RcloneConfig: strPtr("rclone-config-secret"),
		},
	})
	if rd.Spec.Rclone == nil || rd.Spec.Rclone.RcloneConfig == nil || *rd.Spec.Rclone.RcloneConfig != "rclone-config-secret" {
		t.Errorf("dst rclone config not propagated: %+v", rd.Spec.Rclone)
	}
}

func TestReplicationDestination_External(t *testing.T) {
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "ext-dst", Namespace: "ns",
		Mover: &volsync.ExternalConfig{Provider: "example.com/foo"},
	})
	if rd.Spec.External == nil || rd.Spec.External.Provider != "example.com/foo" {
		t.Errorf("dst external not propagated: %+v", rd.Spec.External)
	}
}

func TestReplicationDestination_NilMover(t *testing.T) {
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "no-mover", Namespace: "ns",
	})
	if rd.Spec.Restic != nil || rd.Spec.Rsync != nil || rd.Spec.RsyncTLS != nil ||
		rd.Spec.Rclone != nil || rd.Spec.External != nil {
		t.Errorf("expected no mover spec set, got: %+v", rd.Spec)
	}
}

func TestReplicationSource_TypedNilMoverDoesNotPanic(t *testing.T) {
	// A typed-nil pointer stored in the interface must not panic the dispatcher.
	var resticNil *volsync.SourceResticConfig
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "n", Namespace: "ns", Mover: resticNil,
	})
	if rs == nil {
		t.Fatal("nil result")
	}
	if rs.Spec.Restic != nil {
		t.Errorf("typed-nil Restic should leave spec.Restic unset")
	}

	var externalNil *volsync.ExternalConfig
	rs = volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "n", Namespace: "ns", Mover: externalNil,
	})
	if rs == nil || rs.Spec.External != nil {
		t.Errorf("typed-nil External should leave spec.External unset, got: %+v", rs)
	}
}

func TestReplicationDestination_TypedNilMoverDoesNotPanic(t *testing.T) {
	var rsyncNil *volsync.DestinationRsyncConfig
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "n", Namespace: "ns", Mover: rsyncNil,
	})
	if rd == nil {
		t.Fatal("nil result")
	}
	if rd.Spec.Rsync != nil {
		t.Errorf("typed-nil Rsync should leave spec.Rsync unset")
	}
}

// Compile-time check: each per-mover Config must satisfy its mover interface.
// If any line fails to compile, the sealed-interface invariant is broken.
var _ volsync.SourceMover = (*volsync.SourceResticConfig)(nil)
var _ volsync.SourceMover = (*volsync.SourceRsyncConfig)(nil)
var _ volsync.SourceMover = (*volsync.SourceRsyncTLSConfig)(nil)
var _ volsync.SourceMover = (*volsync.SourceRcloneConfig)(nil)
var _ volsync.SourceMover = (*volsync.SourceSyncthingConfig)(nil)
var _ volsync.SourceMover = (*volsync.ExternalConfig)(nil)
var _ volsync.DestinationMover = (*volsync.DestinationResticConfig)(nil)
var _ volsync.DestinationMover = (*volsync.DestinationRsyncConfig)(nil)
var _ volsync.DestinationMover = (*volsync.DestinationRsyncTLSConfig)(nil)
var _ volsync.DestinationMover = (*volsync.DestinationRcloneConfig)(nil)
var _ volsync.DestinationMover = (*volsync.ExternalConfig)(nil)
