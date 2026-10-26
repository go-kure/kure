package fluxcd

// SyncVerifyPatch exposes syncVerifyPatch to the external tests, which write a
// patch for a pattern validateSourceVerify refuses to show why it does.
var SyncVerifyPatch = (*BootstrapGenerator).syncVerifyPatch
