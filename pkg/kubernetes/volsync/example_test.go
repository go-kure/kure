package volsync_test

import (
	"fmt"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"github.com/go-kure/kure/pkg/kubernetes/volsync"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

func ExampleCreateReplicationSource() {
	obj := volsync.CreateReplicationSource("db-backup", "data")
	fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name)
	// Output: ReplicationSource data/db-backup
}

func ExampleCreateReplicationSource_restic() {
	rs := volsync.CreateReplicationSource("db-backup", "data")
	rs.Spec = volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: "postgres-data",
		Trigger:   &volsyncv1alpha1.ReplicationSourceTriggerSpec{Schedule: ptr.To("@hourly")},
		Restic: &volsyncv1alpha1.ReplicationSourceResticSpec{
			Repository: "restic-creds",
			ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
				CopyMethod: volsync.CopyMethodSnapshot,
			},
			Retain: &volsyncv1alpha1.ResticRetainPolicy{
				Daily:   ptr.To[int32](7),
				Weekly:  ptr.To[int32](4),
				Monthly: ptr.To[int32](12),
			},
		},
	}
	fmt.Println(rs.Spec.SourcePVC, *rs.Spec.Trigger.Schedule, rs.Spec.Restic.CopyMethod)
	// Output: postgres-data @hourly Snapshot
}

func ExampleCreateReplicationDestination() {
	capacity := resource.MustParse("10Gi")

	rd := volsync.CreateReplicationDestination("db-restore", "dr")
	rd.Spec = volsyncv1alpha1.ReplicationDestinationSpec{
		Trigger: &volsyncv1alpha1.ReplicationDestinationTriggerSpec{Manual: "restore-1"},
		Restic: &volsyncv1alpha1.ReplicationDestinationResticSpec{
			Repository: "restic-creds",
			ReplicationDestinationVolumeOptions: volsyncv1alpha1.ReplicationDestinationVolumeOptions{
				CopyMethod:  volsync.CopyMethodSnapshot,
				Capacity:    &capacity,
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
		},
	}
	fmt.Println(rd.Spec.Trigger.Manual, rd.Spec.Restic.Capacity.String())
	// Output: restore-1 10Gi
}

func ExampleCreateReplicationSource_modify() {
	rs := volsync.CreateReplicationSource("db-backup", "data")

	rs.Spec.Paused = false
	rs.Spec.Trigger = &volsyncv1alpha1.ReplicationSourceTriggerSpec{Manual: "go"}
	fmt.Println(rs.Spec.Trigger.Manual)
	// Output: go
}

func ExampleAddSyncthingPeer() {
	rs := volsync.CreateReplicationSource("db-backup", "data")

	rs.Spec.Syncthing = &volsyncv1alpha1.ReplicationSourceSyncthingSpec{}
	volsync.AddSyncthingPeer(rs.Spec.Syncthing, "tcp://peer:22000", "PEER-ID", false)
	fmt.Println(rs.Spec.Syncthing.Peers[0].ID, rs.Spec.Syncthing.Peers[0].Address)
	// Output: PEER-ID tcp://peer:22000
}
