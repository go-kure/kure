package fluxcd

import (
	"bytes"
	_ "embed"
	"os"

	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	"github.com/fluxcd/pkg/tar"

	"github.com/go-kure/kure/pkg/errors"
)

// GotkVersion is the upstream flux2 release whose install manifests base
// (the release's manifests.tar.gz asset) is vendored as gotkManifestsTarGz.
// It is pinned to match the github.com/fluxcd/flux2/v2 Go module version in
// kure's go.mod, so gotk-mode bootstrap generation emits the controllers
// whose API types kure compiles against, without any network access.
//
// TestVendoredPinsMatchGoMod fails when this constant differs from go.mod, or
// when a controller image tag in the vendored bundle differs from the
// matching fluxcd/<controller>/api require (controllers without an API
// module are not compared, and the archive itself is not fingerprinted, so
// replacing it is still on the refresh procedure). To refresh after a flux2
// bump:
//  1. Bump github.com/fluxcd/flux2/v2 (and the fluxcd/* controller APIs,
//     which move together) in go.mod.
//  2. Download the matching manifests.tar.gz from the flux2 GitHub release
//     page:
//     https://github.com/fluxcd/flux2/releases/download/{version}/manifests.tar.gz
//  3. Replace pkg/stack/fluxcd/gotk_manifests.tar.gz with it.
//  4. Update this constant, and the version named in
//     pkg/stack/fluxcd/README.md ("currently **vX.Y.Z**").
//  5. Run the tests in this package.
const GotkVersion = "v2.9.5"

//go:embed gotk_manifests.tar.gz
var gotkManifestsTarGz []byte

// isPinnedGotkVersion reports whether a BootstrapConfig.FluxVersion selects
// the vendored bundle: unset, or GotkVersion with or without its leading "v".
func isPinnedGotkVersion(v string) bool {
	return v == "" || v == GotkVersion || "v"+v == GotkVersion
}

// gotkManifestsBase returns the manifests base install.Generate should build
// from for a BootstrapConfig.FluxVersion, and a cleanup to run afterwards: a
// fresh extraction of the vendored bundle for the pinned version, or "" (the
// upstream download) for any other version.
func gotkManifestsBase(fluxVersion string) (string, func(), error) {
	if !isPinnedGotkVersion(fluxVersion) {
		return "", func() {}, nil
	}
	dir, err := extractGotkManifests()
	if err != nil {
		return "", func() {}, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

// extractGotkManifests unpacks the vendored manifests base into a fresh
// temporary directory and returns it; the caller removes it. Each generation
// needs its own copy, because install.Generate writes option-dependent files
// (namespace, labels, kustomization) into the base before building it.
func extractGotkManifests() (string, error) {
	dir, err := manifestgen.MkdirTempAbs("", "kure-gotk-")
	if err != nil {
		return "", errors.Wrapf(err, "create temp dir for the vendored flux2 %s manifests", GotkVersion)
	}
	if err := tar.Untar(bytes.NewReader(gotkManifestsTarGz), dir, tar.WithMaxUntarSize(-1)); err != nil {
		_ = os.RemoveAll(dir)
		return "", errors.Wrapf(err, "unpack the vendored flux2 %s manifests", GotkVersion)
	}
	return dir, nil
}
