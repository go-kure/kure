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

	if err := SetPodSpecServiceAccountName(spec, "sa"); err != nil {
		t.Fatalf("SetPodSpecServiceAccountName returned error: %v", err)
	}
	if spec.ServiceAccountName != "sa" {
		t.Errorf("service account not set")
	}

	sc := &corev1.PodSecurityContext{}
	if err := SetPodSpecSecurityContext(spec, sc); err != nil {
		t.Fatalf("SetPodSpecSecurityContext returned error: %v", err)
	}
	if spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	if err := SetPodSpecAffinity(spec, aff); err != nil {
		t.Fatalf("SetPodSpecAffinity returned error: %v", err)
	}
	if spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	sel := map[string]string{"role": "db"}
	if err := SetPodSpecNodeSelector(spec, sel); err != nil {
		t.Fatalf("SetPodSpecNodeSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(spec.NodeSelector, sel) {
		t.Errorf("node selector not set")
	}

	if err := SetPodSpecPriorityClassName(spec, "high"); err != nil {
		t.Fatalf("SetPodSpecPriorityClassName returned error: %v", err)
	}
	if spec.PriorityClassName != "high" {
		t.Errorf("priority class name not set")
	}

	if err := SetPodSpecHostNetwork(spec, true); err != nil {
		t.Fatalf("SetPodSpecHostNetwork returned error: %v", err)
	}
	if !spec.HostNetwork {
		t.Errorf("host network not set")
	}

	if err := SetPodSpecHostPID(spec, true); err != nil {
		t.Fatalf("SetPodSpecHostPID returned error: %v", err)
	}
	if !spec.HostPID {
		t.Errorf("host pid not set")
	}

	if err := SetPodSpecHostIPC(spec, true); err != nil {
		t.Fatalf("SetPodSpecHostIPC returned error: %v", err)
	}
	if !spec.HostIPC {
		t.Errorf("host ipc not set")
	}

	if err := SetPodSpecDNSPolicy(spec, corev1.DNSClusterFirstWithHostNet); err != nil {
		t.Fatalf("SetPodSpecDNSPolicy returned error: %v", err)
	}
	if spec.DNSPolicy != corev1.DNSClusterFirstWithHostNet {
		t.Errorf("dns policy not set")
	}

	dnsCfg := &corev1.PodDNSConfig{Nameservers: []string{"8.8.8.8"}}
	if err := SetPodSpecDNSConfig(spec, dnsCfg); err != nil {
		t.Fatalf("SetPodSpecDNSConfig returned error: %v", err)
	}
	if spec.DNSConfig != dnsCfg {
		t.Errorf("dns config not set")
	}

	if err := SetPodSpecHostname(spec, "myhost"); err != nil {
		t.Fatalf("SetPodSpecHostname returned error: %v", err)
	}
	if spec.Hostname != "myhost" {
		t.Errorf("hostname not set")
	}

	if err := SetPodSpecSubdomain(spec, "sub"); err != nil {
		t.Fatalf("SetPodSpecSubdomain returned error: %v", err)
	}
	if spec.Subdomain != "sub" {
		t.Errorf("subdomain not set")
	}

	if err := SetPodSpecRestartPolicy(spec, corev1.RestartPolicyOnFailure); err != nil {
		t.Fatalf("SetPodSpecRestartPolicy returned error: %v", err)
	}
	if spec.RestartPolicy != corev1.RestartPolicyOnFailure {
		t.Errorf("restart policy not set")
	}

	if err := SetPodSpecTerminationGracePeriod(spec, 15); err != nil {
		t.Fatalf("SetPodSpecTerminationGracePeriod returned error: %v", err)
	}
	if spec.TerminationGracePeriodSeconds == nil || *spec.TerminationGracePeriodSeconds != 15 {
		t.Errorf("termination grace period not set")
	}

	if err := SetPodSpecSchedulerName(spec, "sched"); err != nil {
		t.Fatalf("SetPodSpecSchedulerName returned error: %v", err)
	}
	if spec.SchedulerName != "sched" {
		t.Errorf("scheduler name not set")
	}
}
