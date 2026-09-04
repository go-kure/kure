package volsync

import (
	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
)

// AddSyncthingPeer appends a peer to a SourceSyncthingConfig.
func AddSyncthingPeer(cfg *SourceSyncthingConfig, address, id string, introducer bool) {
	if cfg == nil {
		panic("AddSyncthingPeer: cfg must not be nil")
	}
	cfg.Peers = append(cfg.Peers, volsyncv1alpha1.SyncthingPeer{
		Address:    address,
		ID:         id,
		Introducer: introducer,
	})
}
