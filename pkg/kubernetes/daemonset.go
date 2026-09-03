package kubernetes

import (
	appsv1 "k8s.io/api/apps/v1"
)

// SetDaemonSetRevisionHistoryLimit sets the revision history limit.
func SetDaemonSetRevisionHistoryLimit(ds *appsv1.DaemonSet, limit *int32) {
	if ds == nil {
		panic("SetDaemonSetRevisionHistoryLimit: ds must not be nil")
	}
	ds.Spec.RevisionHistoryLimit = limit
}
