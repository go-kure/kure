package fluxcd

import "runtime/debug"

// SyncVerifyPatch exposes syncVerifyPatch to the external tests, which write a
// patch for a pattern validateSourceVerify refuses to show why it does.
var SyncVerifyPatch = (*BootstrapGenerator).syncVerifyPatch

// KustomizeAPIModule and KustomizeAPIAudited expose the module a binary's
// kustomize must be and the version of it that counts, to the external tests
// that build the build information a binary may report.
const (
	KustomizeAPIModule  = kustomizeAPIModule
	KustomizeAPIAudited = kustomizeAPIAudited
)

// SetAfterFirstDefinitionsCheck makes f run once per tree, after the first
// build's check of the schema's definitions passed, until the returned
// function restores the real one (none): the external tests add a definition
// there, which no test can do between two builds of a tree otherwise.
func SetAfterFirstDefinitionsCheck(f func()) (restore func()) {
	orig := afterFirstDefinitionsCheck
	afterFirstDefinitionsCheck = f
	return func() { afterFirstDefinitionsCheck = orig }
}

// SetBuildInfo makes the binary report info as its build information until
// the returned function restores the real one.
func SetBuildInfo(info *debug.BuildInfo) (restore func()) {
	orig := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return info, true }
	return func() { readBuildInfo = orig }
}
