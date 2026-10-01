package fluxcd

import (
	"os"

	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	"github.com/fluxcd/pkg/tar"

	"github.com/go-kure/kure/internal/gotk"
	"github.com/go-kure/kure/pkg/errors"
)

// GotkVersion is the upstream flux2 release whose install manifests base
// (the release's manifests.tar.gz asset) is vendored in internal/gotk, so
// gotk-mode bootstrap generation emits the controllers whose API types kure
// compiles against, without any network access. The pin, and the procedure
// for refreshing it after a flux2 bump, live next to the bundle;
// TestVendoredPinsMatchGoMod fails when the pin and go.mod disagree.
const GotkVersion = gotk.Version

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
	if err := tar.Untar(gotk.Open(), dir, tar.WithMaxUntarSize(-1)); err != nil {
		_ = os.RemoveAll(dir)
		return "", errors.Wrapf(err, "unpack the vendored flux2 %s manifests", GotkVersion)
	}
	return dir, nil
}
