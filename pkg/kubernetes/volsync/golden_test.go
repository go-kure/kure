package volsync_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes/volsync"
)

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
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name:      "db-backup",
		Namespace: "data",
		SourcePVC: "postgres-data",
		Paused:    true,
		Trigger:   &volsync.TriggerConfig{Schedule: strPtr("@hourly")},
		Mover: &volsync.SourceResticConfig{
			Repository:        "restic-creds",
			PruneIntervalDays: i32Ptr(7),
			ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
				CopyMethod:       volsync.CopyMethodSnapshot,
				StorageClassName: strPtr("fast"),
			},
			Retain: &volsyncv1alpha1.ResticRetainPolicy{Daily: i32Ptr(7), Weekly: i32Ptr(4), Monthly: i32Ptr(12)},
		},
	})
	goldenTest(t, "replicationsource-restic.yaml", rs)
}

func TestGolden_ReplicationSourceRsync(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "rsync-src", Namespace: "data", SourcePVC: "data",
		Trigger: &volsync.TriggerConfig{Manual: "sync-1"},
		Mover:   &volsync.SourceRsyncConfig{Address: strPtr("dst.example.com"), SSHKeys: strPtr("ssh-secret")},
	})
	goldenTest(t, "replicationsource-rsync.yaml", rs)
}

func TestGolden_ReplicationSourceRsyncTLS(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "tls-src", Namespace: "data", SourcePVC: "data",
		Mover: &volsync.SourceRsyncTLSConfig{KeySecret: strPtr("tls-key"), Address: strPtr("dst.example.com")},
	})
	goldenTest(t, "replicationsource-rsynctls.yaml", rs)
}

func TestGolden_ReplicationSourceRclone(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "rclone-src", Namespace: "data", SourcePVC: "data",
		Mover: &volsync.SourceRcloneConfig{
			RcloneConfig:        strPtr("rclone-config-secret"),
			RcloneConfigSection: strPtr("backup"),
			RcloneDestPath:      strPtr("remote:bucket/path"),
		},
	})
	goldenTest(t, "replicationsource-rclone.yaml", rs)
}

func TestGolden_ReplicationSourceSyncthing(t *testing.T) {
	capacity := resource.MustParse("1Gi")
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "sync-src", Namespace: "data", SourcePVC: "data",
		Mover: &volsync.SourceSyncthingConfig{
			Peers:          []volsyncv1alpha1.SyncthingPeer{{Address: "tcp://peer:22000", ID: "PEER-ID-XX", Introducer: true}},
			ConfigCapacity: &capacity,
		},
	})
	goldenTest(t, "replicationsource-syncthing.yaml", rs)
}

func TestGolden_ReplicationSourceExternal(t *testing.T) {
	rs := volsync.ReplicationSource(&volsync.ReplicationSourceConfig{
		Name: "ext-src", Namespace: "data", SourcePVC: "data",
		Mover: &volsync.ExternalConfig{Provider: "example.com/foo", Parameters: map[string]string{"k": "v"}},
	})
	goldenTest(t, "replicationsource-external.yaml", rs)
}

func TestGolden_ReplicationDestinationRestic(t *testing.T) {
	capacity := resource.MustParse("10Gi")
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name:      "db-restore",
		Namespace: "dr",
		Paused:    true,
		Trigger:   &volsync.TriggerConfig{Manual: "restore-1"},
		Mover: &volsync.DestinationResticConfig{
			Repository: "restic-creds",
			ReplicationDestinationVolumeOptions: volsyncv1alpha1.ReplicationDestinationVolumeOptions{
				CopyMethod:  volsync.CopyMethodSnapshot,
				Capacity:    &capacity,
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
		},
	})
	goldenTest(t, "replicationdestination-restic.yaml", rd)
}

func TestGolden_ReplicationDestinationRsync(t *testing.T) {
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "rsync-dst", Namespace: "dr",
		Trigger: &volsync.TriggerConfig{Schedule: strPtr("@daily")},
		Mover:   &volsync.DestinationRsyncConfig{SSHKeys: strPtr("ssh-secret")},
	})
	goldenTest(t, "replicationdestination-rsync.yaml", rd)
}

func TestGolden_ReplicationDestinationRsyncTLS(t *testing.T) {
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "tls-dst", Namespace: "dr",
		Mover: &volsync.DestinationRsyncTLSConfig{KeySecret: strPtr("psk")},
	})
	goldenTest(t, "replicationdestination-rsynctls.yaml", rd)
}

func TestGolden_ReplicationDestinationRclone(t *testing.T) {
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "rclone-dst", Namespace: "dr",
		Mover: &volsync.DestinationRcloneConfig{RcloneConfig: strPtr("rclone-config-secret")},
	})
	goldenTest(t, "replicationdestination-rclone.yaml", rd)
}

func TestGolden_ReplicationDestinationExternal(t *testing.T) {
	rd := volsync.ReplicationDestination(&volsync.ReplicationDestinationConfig{
		Name: "ext-dst", Namespace: "dr",
		Mover: &volsync.ExternalConfig{Provider: "example.com/foo"},
	})
	goldenTest(t, "replicationdestination-external.yaml", rd)
}
