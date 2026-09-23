// Package volsync exposes the generated constructors and the admissible sugar
// for VolSync (volsync.backube/v1alpha1) resources: ReplicationSource and
// ReplicationDestination. Each constructor returns a controller-runtime
// object carrying identity only; the upstream volsyncv1alpha1 struct is the
// construction API.
//
// A replication's mover is the upstream one-of: a pointer per arm on the
// spec (Restic, Rsync, RsyncTLS, Rclone, Syncthing on a source, External).
// Set the one you mean and leave the others nil.
//
//	rs := volsync.CreateReplicationSource("db-backup", "data")
//	rs.Spec = volsyncv1alpha1.ReplicationSourceSpec{
//	    SourcePVC: "postgres-data",
//	    Trigger:   &volsyncv1alpha1.ReplicationSourceTriggerSpec{Schedule: ptr.To("@hourly")},
//	    Restic: &volsyncv1alpha1.ReplicationSourceResticSpec{
//	        Repository: "restic-creds",
//	        ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
//	            CopyMethod: volsync.CopyMethodSnapshot,
//	        },
//	    },
//	}
//
// AddSyncthingPeer appends to a Syncthing mover's peer list; CopyMethod and
// its constants re-export the upstream copy-method type.
//
// The config-struct layer this package used to carry — ReplicationSource and
// ReplicationDestination taking a *Config whose Mover was a sealed
// SourceMover / DestinationMover sum — was retired by release 2 of the
// builder contract; see docs/builder-contract-release-2.md for the
// field-by-field mapping.
package volsync
