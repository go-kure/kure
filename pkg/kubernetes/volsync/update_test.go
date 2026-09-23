package volsync_test

import (
	"testing"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"

	"github.com/go-kure/kure/pkg/kubernetes/volsync"
)

func TestAddSyncthingPeer(t *testing.T) {
	spec := &volsyncv1alpha1.ReplicationSourceSyncthingSpec{}
	volsync.AddSyncthingPeer(spec, "tcp://peer:22000", "PEER-ID", true)
	if len(spec.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(spec.Peers))
	}
	if spec.Peers[0].ID != "PEER-ID" || spec.Peers[0].Address != "tcp://peer:22000" || !spec.Peers[0].Introducer {
		t.Errorf("peer not set correctly: %+v", spec.Peers[0])
	}
}

func TestAddSyncthingPeer_Accumulates(t *testing.T) {
	spec := &volsyncv1alpha1.ReplicationSourceSyncthingSpec{}
	volsync.AddSyncthingPeer(spec, "tcp://a:22000", "A", false)
	volsync.AddSyncthingPeer(spec, "tcp://b:22000", "B", true)
	if len(spec.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(spec.Peers))
	}
	if spec.Peers[0].ID != "A" || spec.Peers[1].ID != "B" {
		t.Errorf("peers out of order: %+v", spec.Peers)
	}
}

func TestAddSyncthingPeer_OnAssignedMover(t *testing.T) {
	rs := volsync.CreateReplicationSource("sync-src", "data")
	rs.Spec.Syncthing = &volsyncv1alpha1.ReplicationSourceSyncthingSpec{}
	volsync.AddSyncthingPeer(rs.Spec.Syncthing, "tcp://peer:22000", "PEER-ID", false)
	if len(rs.Spec.Syncthing.Peers) != 1 {
		t.Fatalf("expected the peer on the assigned mover, got %+v", rs.Spec.Syncthing)
	}
}

func TestAddSyncthingPeer_NilSpecPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic on nil spec")
		}
	}()
	volsync.AddSyncthingPeer(nil, "tcp://peer:22000", "PEER-ID", false)
}
