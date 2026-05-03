package volsync

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
)

// CreateReplicationDestination returns a typed ReplicationDestination with
// TypeMeta and ObjectMeta populated. Spec fields are filled by the public
// package's dispatcher.
func CreateReplicationDestination(name, namespace string) *volsyncv1alpha1.ReplicationDestination {
	return &volsyncv1alpha1.ReplicationDestination{
		TypeMeta: metav1.TypeMeta{
			APIVersion: volsyncv1alpha1.GroupVersion.String(),
			Kind:       "ReplicationDestination",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
