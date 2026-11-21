// Package gotk carries the Flux toolkit install manifests kure vendors: the
// manifests.tar.gz asset of one flux2 release, embedded so that gotk-mode
// bootstrap generation, and the tests that read the toolkit's
// CustomResourceDefinitions out of it, need no network.
//
// It sits at the module root rather than in pkg/stack/fluxcd because that
// package and the test helpers its own tests import both read the bundle; a
// package importing a helper that imports it back is an import cycle.
package gotk

import (
	"bytes"
	_ "embed"
	"io"
)

// Version is the flux2 release whose manifests.tar.gz asset is embedded. It is
// pinned to the github.com/fluxcd/flux2/v2 module version in kure's go.mod, so
// the controllers the bundle installs are the ones whose API types kure
// compiles against; pkg/stack/fluxcd's TestVendoredPinsMatchGoMod fails when
// the two differ, or when a controller image tag in the bundle differs from
// the matching fluxcd/<controller>/api require. The archive itself is not
// fingerprinted. After a flux2 bump, scripts/sync-gotk-pin.sh makes the
// companion edits (Renovate runs it on the bump's own branch); by hand:
//  1. Bump github.com/fluxcd/flux2/v2 (and the fluxcd/* controller APIs, which
//     move together) in go.mod.
//  2. Run ./scripts/sync-gotk-pin.sh. It downloads the matching asset from the
//     flux2 release page
//     (https://github.com/fluxcd/flux2/releases/download/{version}/manifests.tar.gz),
//     replaces internal/gotk/manifests.tar.gz, updates this constant and the
//     version named in pkg/stack/fluxcd/docs/bootstrap-generation.md
//     ("currently **vX.Y.Z**"), and regenerates the stored bootstrap outputs
//     when only their bundle digest and version lines change.
//  3. Run the tests under internal/ and pkg/stack/fluxcd. The script exits
//     before the download when the constant and the page already name the
//     go.mod version, so to force a refresh of the archive alone, replace it
//     by hand and run `GOWORK=off go test ./pkg/stack/fluxcd/ -update`.
const Version = "v2.10.1"

//go:embed manifests.tar.gz
var manifests []byte

// Open returns a reader over the embedded bundle: a gzip-compressed tar whose
// entries are the release's flat manifest files (<controller>.yaml, rbac.yaml,
// policies.yaml). Every call starts at the beginning.
func Open() io.Reader {
	return bytes.NewReader(manifests)
}
