package volsync

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
)

// CreateReplicationSource returns a typed ReplicationSource with TypeMeta and
// ObjectMeta populated. Spec fields are left zero — the public package fills
// them via the dispatcher in pkg/kubernetes/volsync/create.go.
func CreateReplicationSource(name, namespace string) *volsyncv1alpha1.ReplicationSource {
	return &volsyncv1alpha1.ReplicationSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: volsyncv1alpha1.GroupVersion.String(),
			Kind:       "ReplicationSource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
