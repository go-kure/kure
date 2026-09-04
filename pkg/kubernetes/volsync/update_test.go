package volsync_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/kubernetes/volsync"
)

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

func TestAddSyncthingPeer_Accumulates(t *testing.T) {
	cfg := &volsync.SourceSyncthingConfig{}
	volsync.AddSyncthingPeer(cfg, "tcp://a:22000", "A", false)
	volsync.AddSyncthingPeer(cfg, "tcp://b:22000", "B", true)
	if len(cfg.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(cfg.Peers))
	}
	if cfg.Peers[0].ID != "A" || cfg.Peers[1].ID != "B" {
		t.Errorf("peers out of order: %+v", cfg.Peers)
	}
}

func TestAddSyncthingPeer_NilConfigPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic on nil cfg")
		}
	}()
	volsync.AddSyncthingPeer(nil, "tcp://peer:22000", "PEER-ID", false)
}
