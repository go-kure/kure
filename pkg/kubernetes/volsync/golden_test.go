package volsync_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes/volsync"
)

// The fixtures under testdata were written by the config-struct layer this
// package used to carry (ReplicationSource(&ReplicationSourceConfig{...}) and
// ReplicationDestination, dispatching on the sealed SourceMover /
// DestinationMover sums) before it was retired. Each test below builds the
// same object on the generated constructor plus the upstream struct and must
// reproduce that output byte for byte.

var update = flag.Bool("update", false, "update golden files")

func goldenTest(t *testing.T, filename string, obj client.Object) {
	t.Helper()
	objects := []*client.Object{&obj}
	got, err := kureio.EncodeObjectsToYAMLWithOptions(objects, kureio.EncodeOptions{
		KubernetesFieldOrder: true,
	})
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}
	golden := filepath.Join("testdata", filename)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("updating golden file: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n\ngot:\n%s\nwant:\n%s", golden, got, want)
	}
}

func TestGolden_ReplicationSourceRestic(t *testing.T) {
	rs := volsync.CreateReplicationSource("db-backup", "data")
	rs.Spec = volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: "postgres-data",
		Paused:    true,
		Trigger:   &volsyncv1alpha1.ReplicationSourceTriggerSpec{Schedule: ptr.To("@hourly")},
		Restic: &volsyncv1alpha1.ReplicationSourceResticSpec{
			Repository:        "restic-creds",
			PruneIntervalDays: ptr.To[int32](7),
			ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
				CopyMethod:       volsync.CopyMethodSnapshot,
				StorageClassName: ptr.To("fast"),
			},
			Retain: &volsyncv1alpha1.ResticRetainPolicy{Daily: ptr.To[int32](7), Weekly: ptr.To[int32](4), Monthly: ptr.To[int32](12)},
		},
	}
	goldenTest(t, "replicationsource-restic.yaml", rs)
}

func TestGolden_ReplicationSourceRsync(t *testing.T) {
	rs := volsync.CreateReplicationSource("rsync-src", "data")
	rs.Spec = volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: "data",
		Trigger:   &volsyncv1alpha1.ReplicationSourceTriggerSpec{Manual: "sync-1"},
		Rsync:     &volsyncv1alpha1.ReplicationSourceRsyncSpec{Address: ptr.To("dst.example.com"), SSHKeys: ptr.To("ssh-secret")},
	}
	goldenTest(t, "replicationsource-rsync.yaml", rs)
}

func TestGolden_ReplicationSourceRsyncTLS(t *testing.T) {
	rs := volsync.CreateReplicationSource("tls-src", "data")
	rs.Spec = volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: "data",
		RsyncTLS:  &volsyncv1alpha1.ReplicationSourceRsyncTLSSpec{KeySecret: ptr.To("tls-key"), Address: ptr.To("dst.example.com")},
	}
	goldenTest(t, "replicationsource-rsynctls.yaml", rs)
}

func TestGolden_ReplicationSourceRclone(t *testing.T) {
	rs := volsync.CreateReplicationSource("rclone-src", "data")
	rs.Spec = volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: "data",
		Rclone: &volsyncv1alpha1.ReplicationSourceRcloneSpec{
			RcloneConfig:        ptr.To("rclone-config-secret"),
			RcloneConfigSection: ptr.To("backup"),
			RcloneDestPath:      ptr.To("remote:bucket/path"),
		},
	}
	goldenTest(t, "replicationsource-rclone.yaml", rs)
}

func TestGolden_ReplicationSourceSyncthing(t *testing.T) {
	capacity := resource.MustParse("1Gi")
	rs := volsync.CreateReplicationSource("sync-src", "data")
	rs.Spec.SourcePVC = "data"
	rs.Spec.Syncthing = &volsyncv1alpha1.ReplicationSourceSyncthingSpec{ConfigCapacity: &capacity}
	volsync.AddSyncthingPeer(rs.Spec.Syncthing, "tcp://peer:22000", "PEER-ID-XX", true)
	goldenTest(t, "replicationsource-syncthing.yaml", rs)
}

func TestGolden_ReplicationSourceExternal(t *testing.T) {
	rs := volsync.CreateReplicationSource("ext-src", "data")
	rs.Spec = volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: "data",
		External:  &volsyncv1alpha1.ReplicationSourceExternalSpec{Provider: "example.com/foo", Parameters: map[string]string{"k": "v"}},
	}
	goldenTest(t, "replicationsource-external.yaml", rs)
}

func TestGolden_ReplicationDestinationRestic(t *testing.T) {
	capacity := resource.MustParse("10Gi")
	rd := volsync.CreateReplicationDestination("db-restore", "dr")
	rd.Spec = volsyncv1alpha1.ReplicationDestinationSpec{
		Paused:  true,
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
	goldenTest(t, "replicationdestination-restic.yaml", rd)
}

func TestGolden_ReplicationDestinationRsync(t *testing.T) {
	rd := volsync.CreateReplicationDestination("rsync-dst", "dr")
	rd.Spec = volsyncv1alpha1.ReplicationDestinationSpec{
		Trigger: &volsyncv1alpha1.ReplicationDestinationTriggerSpec{Schedule: ptr.To("@daily")},
		Rsync:   &volsyncv1alpha1.ReplicationDestinationRsyncSpec{SSHKeys: ptr.To("ssh-secret")},
	}
	goldenTest(t, "replicationdestination-rsync.yaml", rd)
}

func TestGolden_ReplicationDestinationRsyncTLS(t *testing.T) {
	rd := volsync.CreateReplicationDestination("tls-dst", "dr")
	rd.Spec.RsyncTLS = &volsyncv1alpha1.ReplicationDestinationRsyncTLSSpec{KeySecret: ptr.To("psk")}
	goldenTest(t, "replicationdestination-rsynctls.yaml", rd)
}

func TestGolden_ReplicationDestinationRclone(t *testing.T) {
	rd := volsync.CreateReplicationDestination("rclone-dst", "dr")
	rd.Spec.Rclone = &volsyncv1alpha1.ReplicationDestinationRcloneSpec{RcloneConfig: ptr.To("rclone-config-secret")}
	goldenTest(t, "replicationdestination-rclone.yaml", rd)
}

func TestGolden_ReplicationDestinationExternal(t *testing.T) {
	rd := volsync.CreateReplicationDestination("ext-dst", "dr")
	rd.Spec.External = &volsyncv1alpha1.ReplicationDestinationExternalSpec{Provider: "example.com/foo"}
	goldenTest(t, "replicationdestination-external.yaml", rd)
}
