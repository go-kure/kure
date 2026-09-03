package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodSpecFunctions(t *testing.T) {
	spec := &corev1.PodSpec{}

	c := corev1.Container{Name: "c"}
	if err := AddPodSpecContainer(spec, &c); err != nil {
		t.Fatalf("AddPodSpecContainer returned error: %v", err)
	}
	if len(spec.Containers) != 1 || spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	if err := AddPodSpecInitContainer(spec, &ic); err != nil {
		t.Fatalf("AddPodSpecInitContainer returned error: %v", err)
	}
	if len(spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	ec := corev1.EphemeralContainer{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debug"}}
	if err := AddPodSpecEphemeralContainer(spec, &ec); err != nil {
		t.Fatalf("AddPodSpecEphemeralContainer returned error: %v", err)
	}
	if len(spec.EphemeralContainers) != 1 {
		t.Errorf("ephemeral container not added")
	}

	v := corev1.Volume{Name: "vol"}
	if err := AddPodSpecVolume(spec, &v); err != nil {
		t.Fatalf("AddPodSpecVolume returned error: %v", err)
	}
	if len(spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	sec := corev1.LocalObjectReference{Name: "pull"}
	if err := AddPodSpecImagePullSecret(spec, &sec); err != nil {
		t.Fatalf("AddPodSpecImagePullSecret returned error: %v", err)
	}
	if len(spec.ImagePullSecrets) != 1 {
		t.Errorf("pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	if err := AddPodSpecToleration(spec, &tol); err != nil {
		t.Fatalf("AddPodSpecToleration returned error: %v", err)
	}
	if len(spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{}}
	if err := AddPodSpecTopologySpreadConstraints(spec, &tsc); err != nil {
		t.Fatalf("AddPodSpecTopologySpreadConstraints returned error: %v", err)
	}
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

func TestPodSpecNilGuards(t *testing.T) {
	// Functions that still return error (secondary nil checks) — keep error-based tests
	t.Run("AddPodSpecContainer nil spec", func(t *testing.T) {
		if err := AddPodSpecContainer(nil, &corev1.Container{Name: "c"}); err == nil {
			t.Error("AddPodSpecContainer(nil) should return error")
		}
	})
	t.Run("AddPodSpecInitContainer nil spec", func(t *testing.T) {
		if err := AddPodSpecInitContainer(nil, &corev1.Container{Name: "c"}); err == nil {
			t.Error("AddPodSpecInitContainer(nil) should return error")
		}
	})
	t.Run("AddPodSpecEphemeralContainer nil spec", func(t *testing.T) {
		if err := AddPodSpecEphemeralContainer(nil, &corev1.EphemeralContainer{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "e"}}); err == nil {
			t.Error("AddPodSpecEphemeralContainer(nil) should return error")
		}
	})
	t.Run("AddPodSpecVolume nil spec", func(t *testing.T) {
		if err := AddPodSpecVolume(nil, &corev1.Volume{Name: "v"}); err == nil {
			t.Error("AddPodSpecVolume(nil) should return error")
		}
	})
	t.Run("AddPodSpecImagePullSecret nil spec", func(t *testing.T) {
		if err := AddPodSpecImagePullSecret(nil, &corev1.LocalObjectReference{Name: "s"}); err == nil {
			t.Error("AddPodSpecImagePullSecret(nil) should return error")
		}
	})
	t.Run("AddPodSpecToleration nil spec", func(t *testing.T) {
		if err := AddPodSpecToleration(nil, &corev1.Toleration{Key: "k"}); err == nil {
			t.Error("AddPodSpecToleration(nil) should return error")
		}
	})
	t.Run("AddPodSpecTopologySpreadConstraints nil spec", func(t *testing.T) {
		if err := AddPodSpecTopologySpreadConstraints(nil, &corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{}}); err == nil {
			t.Error("AddPodSpecTopologySpreadConstraints(nil) should return error")
		}
	})

	// Functions now panic on nil receiver
	assertPanics(t, func() { SetPodSpecSecurityContext(nil, &corev1.PodSecurityContext{}) })
	assertPanics(t, func() { SetPodSpecAffinity(nil, &corev1.Affinity{}) })
	assertPanics(t, func() { SetPodSpecDNSConfig(nil, &corev1.PodDNSConfig{}) })
	assertPanics(t, func() { SetPodSpecTerminationGracePeriod(nil, 30) })
}
