package volsync

import (
	"testing"
)

// Tests for sealed interface marker methods.
// These must be in the same package to access unexported methods.

func TestSourceMoverMarkers(t *testing.T) {
	// Exercise the marker methods to achieve coverage.
	// Each of these is a single-line unexported method.
	var restic *SourceResticConfig = &SourceResticConfig{}
	restic.isSourceMover()

	var rsync *SourceRsyncConfig = &SourceRsyncConfig{}
	rsync.isSourceMover()

	var rsyncTLS *SourceRsyncTLSConfig = &SourceRsyncTLSConfig{}
	rsyncTLS.isSourceMover()

	var rclone *SourceRcloneConfig = &SourceRcloneConfig{}
	rclone.isSourceMover()

	var syncthing *SourceSyncthingConfig = &SourceSyncthingConfig{}
	syncthing.isSourceMover()

	var ext *ExternalConfig = &ExternalConfig{}
	ext.isSourceMover()
	ext.isDestinationMover()

	var dstRestic *DestinationResticConfig = &DestinationResticConfig{}
	dstRestic.isDestinationMover()

	var dstRsync *DestinationRsyncConfig = &DestinationRsyncConfig{}
	dstRsync.isDestinationMover()

	var dstRsyncTLS *DestinationRsyncTLSConfig = &DestinationRsyncTLSConfig{}
	dstRsyncTLS.isDestinationMover()

	var dstRclone *DestinationRcloneConfig = &DestinationRcloneConfig{}
	dstRclone.isDestinationMover()
}
