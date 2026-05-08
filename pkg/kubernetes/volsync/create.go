package volsync

import (
	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateReplicationSource returns a new ReplicationSource with TypeMeta and
// ObjectMeta set. Spec fields are left zero; use the setters to populate them.
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

// CreateReplicationDestination returns a new ReplicationDestination with
// TypeMeta and ObjectMeta set. Spec fields are left zero; use the setters to
// populate them.
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

// ReplicationSource constructs a typed ReplicationSource from cfg. The Mover
// variant is dispatched via type switch; if Mover is nil, no mover spec is
// set and CRD validation will reject at apply time.
func ReplicationSource(cfg *ReplicationSourceConfig) *volsyncv1alpha1.ReplicationSource {
	if cfg == nil {
		return nil
	}
	rs := CreateReplicationSource(cfg.Name, cfg.Namespace)
	rs.Spec.SourcePVC = cfg.SourcePVC
	rs.Spec.Paused = cfg.Paused
	if cfg.Trigger != nil {
		rs.Spec.Trigger = &volsyncv1alpha1.ReplicationSourceTriggerSpec{
			Schedule: cfg.Trigger.Schedule,
			Manual:   cfg.Trigger.Manual,
		}
	}
	// Each case guards against typed-nil pointers stored in the interface:
	// `var m *SourceResticConfig; cfg.Mover = m` matches the case but `*m`
	// would panic. Treat typed-nil as "no mover for this variant" — same
	// effective behaviour as a nil interface value.
	switch m := cfg.Mover.(type) {
	case *SourceResticConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceResticSpec(*m)
			rs.Spec.Restic = &spec
		}
	case *SourceRsyncConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceRsyncSpec(*m)
			rs.Spec.Rsync = &spec
		}
	case *SourceRsyncTLSConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceRsyncTLSSpec(*m)
			rs.Spec.RsyncTLS = &spec
		}
	case *SourceRcloneConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceRcloneSpec(*m)
			rs.Spec.Rclone = &spec
		}
	case *SourceSyncthingConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationSourceSyncthingSpec(*m)
			rs.Spec.Syncthing = &spec
		}
	case *ExternalConfig:
		if m != nil {
			rs.Spec.External = &volsyncv1alpha1.ReplicationSourceExternalSpec{
				Provider:   m.Provider,
				Parameters: m.Parameters,
			}
		}
	}
	return rs
}

// ReplicationDestination constructs a typed ReplicationDestination from cfg.
// Mover is dispatched via type switch; if Mover is nil, no mover spec is set
// and CRD validation will reject at apply time.
func ReplicationDestination(cfg *ReplicationDestinationConfig) *volsyncv1alpha1.ReplicationDestination {
	if cfg == nil {
		return nil
	}
	rd := CreateReplicationDestination(cfg.Name, cfg.Namespace)
	rd.Spec.Paused = cfg.Paused
	if cfg.Trigger != nil {
		rd.Spec.Trigger = &volsyncv1alpha1.ReplicationDestinationTriggerSpec{
			Schedule: cfg.Trigger.Schedule,
			Manual:   cfg.Trigger.Manual,
		}
	}
	// Each case guards against typed-nil pointers stored in the interface;
	// see the matching comment in ReplicationSource.
	switch m := cfg.Mover.(type) {
	case *DestinationResticConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationDestinationResticSpec(*m)
			rd.Spec.Restic = &spec
		}
	case *DestinationRsyncConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationDestinationRsyncSpec(*m)
			rd.Spec.Rsync = &spec
		}
	case *DestinationRsyncTLSConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationDestinationRsyncTLSSpec(*m)
			rd.Spec.RsyncTLS = &spec
		}
	case *DestinationRcloneConfig:
		if m != nil {
			spec := volsyncv1alpha1.ReplicationDestinationRcloneSpec(*m)
			rd.Spec.Rclone = &spec
		}
	case *ExternalConfig:
		if m != nil {
			rd.Spec.External = &volsyncv1alpha1.ReplicationDestinationExternalSpec{
				Provider:   m.Provider,
				Parameters: m.Parameters,
			}
		}
	}
	return rd
}
