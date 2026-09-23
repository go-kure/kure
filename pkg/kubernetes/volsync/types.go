package volsync

import (
	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
)

// CopyMethod re-exports the upstream CopyMethodType for caller convenience.
type CopyMethod = volsyncv1alpha1.CopyMethodType

const (
	CopyMethodDirect   = volsyncv1alpha1.CopyMethodDirect
	CopyMethodNone     = volsyncv1alpha1.CopyMethodNone
	CopyMethodClone    = volsyncv1alpha1.CopyMethodClone
	CopyMethodSnapshot = volsyncv1alpha1.CopyMethodSnapshot
)
