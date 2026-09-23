package volsync

import (
	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
)

// AddSyncthingPeer appends a peer to a ReplicationSource's Syncthing mover
// spec, the value assigned to rs.Spec.Syncthing.
func AddSyncthingPeer(spec *volsyncv1alpha1.ReplicationSourceSyncthingSpec, address, id string, introducer bool) {
	if spec == nil {
		panic("AddSyncthingPeer: spec must not be nil")
	}
	spec.Peers = append(spec.Peers, volsyncv1alpha1.SyncthingPeer{
		Address:    address,
		ID:         id,
		Introducer: introducer,
	})
}
