package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreatePodSpec(t *testing.T) {
	spec := CreatePodSpec()

	if spec.RestartPolicy != corev1.RestartPolicyAlways {
		t.Errorf("unexpected restart policy %v", spec.RestartPolicy)
	}
	if spec.TerminationGracePeriodSeconds == nil {
		t.Errorf("expected TerminationGracePeriodSeconds to be set")
	}
	if len(spec.Containers) != 0 {
		t.Errorf("expected no containers")
	}
}

func TestPodSpecFunctions(t *testing.T) {
	spec := CreatePodSpec()

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

	SetPodSpecServiceAccountName(spec, "sa")
	if spec.ServiceAccountName != "sa" {
		t.Errorf("service account not set")
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

	sel := map[string]string{"role": "db"}
	SetPodSpecNodeSelector(spec, sel)
	if !reflect.DeepEqual(spec.NodeSelector, sel) {
		t.Errorf("node selector not set")
	}

	SetPodSpecPriorityClassName(spec, "high")
	if spec.PriorityClassName != "high" {
		t.Errorf("priority class name not set")
	}

	SetPodSpecHostNetwork(spec, true)
	if !spec.HostNetwork {
		t.Errorf("host network not set")
	}

	SetPodSpecHostPID(spec, true)
	if !spec.HostPID {
		t.Errorf("host pid not set")
	}

	SetPodSpecHostIPC(spec, true)
	if !spec.HostIPC {
		t.Errorf("host ipc not set")
	}

	SetPodSpecDNSPolicy(spec, corev1.DNSClusterFirstWithHostNet)
	if spec.DNSPolicy != corev1.DNSClusterFirstWithHostNet {
		t.Errorf("dns policy not set")
	}

	dnsCfg := &corev1.PodDNSConfig{Nameservers: []string{"8.8.8.8"}}
	SetPodSpecDNSConfig(spec, dnsCfg)
	if spec.DNSConfig != dnsCfg {
		t.Errorf("dns config not set")
	}

	SetPodSpecHostname(spec, "myhost")
	if spec.Hostname != "myhost" {
		t.Errorf("hostname not set")
	}

	SetPodSpecSubdomain(spec, "sub")
	if spec.Subdomain != "sub" {
		t.Errorf("subdomain not set")
	}

	SetPodSpecRestartPolicy(spec, corev1.RestartPolicyOnFailure)
	if spec.RestartPolicy != corev1.RestartPolicyOnFailure {
		t.Errorf("restart policy not set")
	}

	SetPodSpecTerminationGracePeriod(spec, 15)
	if spec.TerminationGracePeriodSeconds == nil || *spec.TerminationGracePeriodSeconds != 15 {
		t.Errorf("termination grace period not set")
	}

	SetPodSpecSchedulerName(spec, "sched")
	if spec.SchedulerName != "sched" {
		t.Errorf("scheduler name not set")
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
	assertPanics(t, func() { SetPodSpecServiceAccountName(nil, "sa") })
	assertPanics(t, func() { SetPodSpecSecurityContext(nil, &corev1.PodSecurityContext{}) })
	assertPanics(t, func() { SetPodSpecAffinity(nil, &corev1.Affinity{}) })
	assertPanics(t, func() { SetPodSpecNodeSelector(nil, map[string]string{"k": "v"}) })
	assertPanics(t, func() { SetPodSpecPriorityClassName(nil, "high") })
	assertPanics(t, func() { SetPodSpecHostNetwork(nil, true) })
	assertPanics(t, func() { SetPodSpecHostPID(nil, true) })
	assertPanics(t, func() { SetPodSpecHostIPC(nil, true) })
	assertPanics(t, func() { SetPodSpecDNSPolicy(nil, corev1.DNSClusterFirst) })
	assertPanics(t, func() { SetPodSpecDNSConfig(nil, &corev1.PodDNSConfig{}) })
	assertPanics(t, func() { SetPodSpecHostname(nil, "host") })
	assertPanics(t, func() { SetPodSpecSubdomain(nil, "sub") })
	assertPanics(t, func() { SetPodSpecRestartPolicy(nil, corev1.RestartPolicyAlways) })
	assertPanics(t, func() { SetPodSpecTerminationGracePeriod(nil, 30) })
	assertPanics(t, func() { SetPodSpecSchedulerName(nil, "sched") })
}
