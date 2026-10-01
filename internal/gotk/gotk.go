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
// fingerprinted, so replacing it stays on the refresh procedure. To refresh
// after a flux2 bump:
//  1. Bump github.com/fluxcd/flux2/v2 (and the fluxcd/* controller APIs, which
//     move together) in go.mod.
//  2. Download the matching asset from the flux2 release page:
//     https://github.com/fluxcd/flux2/releases/download/{version}/manifests.tar.gz
//  3. Replace internal/gotk/manifests.tar.gz with it.
//  4. Update this constant, and the version named in
//     pkg/stack/fluxcd/README.md ("currently **vX.Y.Z**").
//  5. Run the tests under internal/ and pkg/stack/fluxcd.
const Version = "v2.9.5"

//go:embed manifests.tar.gz
var manifests []byte

// Open returns a reader over the embedded bundle: a gzip-compressed tar whose
// entries are the release's flat manifest files (<controller>.yaml, rbac.yaml,
// policies.yaml). Every call starts at the beginning.
func Open() io.Reader {
	return bytes.NewReader(manifests)
}
