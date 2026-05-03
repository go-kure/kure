// Package volsync provides builders for VolSync (volsync.backube/v1alpha1)
// resources: ReplicationSource and ReplicationDestination.
//
// The package follows the kure builder convention (see
// pkg/kubernetes/certmanager) for file split and modifier shape, but encodes
// the mover one-of as a sealed-interface sum type rather than a multi-pointer
// Config. See docs/ARCHITECTURE.md § "One-of Constraints (Sealed Interfaces)"
// for the rationale.
//
// Example:
//
//	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
//	    Name: "db-backup", Namespace: "data",
//	    SourcePVC: "postgres-data",
//	    Trigger: &volsync.TriggerConfig{Schedule: ptr.String("@hourly")},
//	    Mover: &volsync.SourceResticConfig{
//	        Repository: "restic-creds",
//	        ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
//	            CopyMethod: volsync.CopyMethodSnapshot,
//	        },
//	    },
//	})
package volsync
