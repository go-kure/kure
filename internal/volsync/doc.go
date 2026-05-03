// Package volsync contains the internal builders for VolSync resources.
// Public callers should use github.com/go-kure/kure/pkg/kubernetes/volsync.
//
// The internal layer is intentionally thin: per-mover Specs are upstream
// types reused directly via defined-type wrappers in the public package, so
// only the resource constructors (ReplicationSource, ReplicationDestination)
// live here.
package volsync
