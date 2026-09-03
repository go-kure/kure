package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodSpecFunctions(t *testing.T) {
	spec := &corev1.PodSpec{}

	c := corev1.Container{Name: "c"}
	AddPodSpecContainer(spec, &c)
	if len(spec.Containers) != 1 || spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	AddPodSpecInitContainer(spec, &ic)
	if len(spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	ec := corev1.EphemeralContainer{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debug"}}
	AddPodSpecEphemeralContainer(spec, &ec)
	if len(spec.EphemeralContainers) != 1 {
		t.Errorf("ephemeral container not added")
	}

	v := corev1.Volume{Name: "vol"}
	AddPodSpecVolume(spec, &v)
	if len(spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	sec := corev1.LocalObjectReference{Name: "pull"}
	AddPodSpecImagePullSecret(spec, &sec)
	if len(spec.ImagePullSecrets) != 1 {
		t.Errorf("pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	AddPodSpecToleration(spec, &tol)
	if len(spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{}}
	AddPodSpecTopologySpreadConstraints(spec, &tsc)
	if len(spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	sc := &corev1.PodSecurityContext{}
	SetPodSpecSecurityContext(spec, sc)
	if spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	SetPodSpecAffinity(spec, aff)
	if spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	dnsCfg := &corev1.PodDNSConfig{Nameservers: []string{"8.8.8.8"}}
	SetPodSpecDNSConfig(spec, dnsCfg)
	if spec.DNSConfig != dnsCfg {
		t.Errorf("dns config not set")
	}

	SetPodSpecTerminationGracePeriod(spec, 15)
	if spec.TerminationGracePeriodSeconds == nil || *spec.TerminationGracePeriodSeconds != 15 {
		t.Errorf("termination grace period not set")
	}
}

// TestPodSpecAppendersAccumulate covers repeated appends: the helpers add to
// what is already on the field rather than replacing it.
func TestPodSpecAppendersAccumulate(t *testing.T) {
	spec := &corev1.PodSpec{}

	AddPodSpecContainer(spec, &corev1.Container{Name: "first"})
	AddPodSpecContainer(spec, &corev1.Container{Name: "second"})
	if len(spec.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(spec.Containers))
	}
	if spec.Containers[0].Name != "first" || spec.Containers[1].Name != "second" {
		t.Errorf("containers out of order: %q, %q", spec.Containers[0].Name, spec.Containers[1].Name)
	}

	AddPodSpecTopologySpreadConstraints(spec, &corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule})
	AddPodSpecTopologySpreadConstraints(spec, &corev1.TopologySpreadConstraint{MaxSkew: 2, TopologyKey: "hostname", WhenUnsatisfiable: corev1.DoNotSchedule})
	if len(spec.TopologySpreadConstraints) != 2 {
		t.Fatalf("expected 2 constraints, got %d", len(spec.TopologySpreadConstraints))
	}
	if spec.TopologySpreadConstraints[1].TopologyKey != "hostname" {
		t.Errorf("second constraint mismatch: %q", spec.TopologySpreadConstraints[1].TopologyKey)
	}
}

func TestPodSpecNilGuards(t *testing.T) {
	assertPanics(t, func() { AddPodSpecContainer(nil, &corev1.Container{Name: "c"}) })
	assertPanics(t, func() { AddPodSpecInitContainer(nil, &corev1.Container{Name: "c"}) })
	assertPanics(t, func() {
		AddPodSpecEphemeralContainer(nil, &corev1.EphemeralContainer{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "e"}})
	})
	assertPanics(t, func() { AddPodSpecVolume(nil, &corev1.Volume{Name: "v"}) })
	assertPanics(t, func() { AddPodSpecImagePullSecret(nil, &corev1.LocalObjectReference{Name: "s"}) })
	assertPanics(t, func() { AddPodSpecToleration(nil, &corev1.Toleration{Key: "k"}) })
	assertPanics(t, func() { AddPodSpecTopologySpreadConstraints(nil, &corev1.TopologySpreadConstraint{MaxSkew: 1}) })

	assertPanics(t, func() { SetPodSpecSecurityContext(nil, &corev1.PodSecurityContext{}) })
	assertPanics(t, func() { SetPodSpecAffinity(nil, &corev1.Affinity{}) })
	assertPanics(t, func() { SetPodSpecDNSConfig(nil, &corev1.PodDNSConfig{}) })
	assertPanics(t, func() { SetPodSpecTerminationGracePeriod(nil, 30) })
}

func TestPodSpecNilArgGuards(t *testing.T) {
	spec := &corev1.PodSpec{}

	assertPanics(t, func() { AddPodSpecContainer(spec, nil) })
	assertPanics(t, func() { AddPodSpecInitContainer(spec, nil) })
	assertPanics(t, func() { AddPodSpecEphemeralContainer(spec, nil) })
	assertPanics(t, func() { AddPodSpecVolume(spec, nil) })
	assertPanics(t, func() { AddPodSpecImagePullSecret(spec, nil) })
	assertPanics(t, func() { AddPodSpecToleration(spec, nil) })
	assertPanics(t, func() { AddPodSpecTopologySpreadConstraints(spec, nil) })
}
