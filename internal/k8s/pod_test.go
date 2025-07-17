package k8s

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreatePod(t *testing.T) {
	pod := CreatePod("test-pod", "default")

	if pod.Name != "test-pod" {
		t.Errorf("expected name %q got %q", "test-pod", pod.Name)
	}
	if pod.Namespace != "default" {
		t.Errorf("expected namespace %q got %q", "default", pod.Namespace)
	}
	if pod.Kind != "Pod" {
		t.Errorf("expected kind Pod got %q", pod.Kind)
	}
	if pod.APIVersion != "v1" {
		t.Errorf("expected apiVersion v1 got %q", pod.APIVersion)
	}
	if pod.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		t.Errorf("unexpected restart policy %v", pod.Spec.RestartPolicy)
	}
	if pod.Spec.TerminationGracePeriodSeconds == nil {
		t.Errorf("expected TerminationGracePeriodSeconds to be set")
	}
}

func TestPodFunctions(t *testing.T) {
	pod := CreatePod("app", "ns")
	if pod.Name != "app" || pod.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", pod.Namespace, pod.Name)
	}
	if pod.Kind != "Pod" {
		t.Errorf("unexpected kind %q", pod.Kind)
	}

	c := corev1.Container{Name: "c"}
	if err := AddPodContainer(pod, &c); err != nil {
		t.Fatalf("AddPodContainer returned error: %v", err)
	}
	if len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	if err := AddPodInitContainer(pod, &ic); err != nil {
		t.Fatalf("AddPodInitContainer returned error: %v", err)
	}
	if len(pod.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	if err := AddPodVolume(pod, &v); err != nil {
		t.Fatalf("AddPodVolume returned error: %v", err)
	}
	if len(pod.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	secret := corev1.LocalObjectReference{Name: "secret"}
	if err := AddPodImagePullSecret(pod, &secret); err != nil {
		t.Fatalf("AddPodImagePullSecret returned error: %v", err)
	}
	if len(pod.Spec.ImagePullSecrets) != 1 {
		t.Errorf("image pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	if err := AddPodToleration(pod, &tol); err != nil {
		t.Fatalf("AddPodToleration returned error: %v", err)
	}
	if len(pod.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{}}
	if err := AddPodTopologySpreadConstraints(pod, &tsc); err != nil {
		t.Fatalf("AddPodTopologySpreadConstraints returned error: %v", err)
	}
	if len(pod.Spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	if err := SetPodServiceAccountName(pod, "sa"); err != nil {
		t.Fatalf("SetPodServiceAccountName returned error: %v", err)
	}
	if pod.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account name not set")
	}

	sc := &corev1.PodSecurityContext{RunAsUser: func(i int64) *int64 { return &i }(1)}
	if err := SetPodSecurityContext(pod, sc); err != nil {
		t.Fatalf("SetPodSecurityContext returned error: %v", err)
	}
	if pod.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	if err := SetPodAffinity(pod, aff); err != nil {
		t.Fatalf("SetPodAffinity returned error: %v", err)
	}
	if pod.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	ns := map[string]string{"role": "db"}
	if err := SetPodNodeSelector(pod, ns); err != nil {
		t.Fatalf("SetPodNodeSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(pod.Spec.NodeSelector, ns) {
		t.Errorf("node selector not set")
	}

	ec := corev1.EphemeralContainer{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debug"}}
	if err := AddPodEphemeralContainer(pod, &ec); err != nil {
		t.Fatalf("AddPodEphemeralContainer returned error: %v", err)
	}
	if len(pod.Spec.EphemeralContainers) != 1 {
		t.Errorf("ephemeral container not added")
	}

	if err := SetPodHostNetwork(pod, true); err != nil {
		t.Fatalf("SetPodHostNetwork returned error: %v", err)
	}
	if !pod.Spec.HostNetwork {
		t.Errorf("host network not set")
	}

	if err := SetPodDNSPolicy(pod, corev1.DNSClusterFirstWithHostNet); err != nil {
		t.Fatalf("SetPodDNSPolicy returned error: %v", err)
	}
	if pod.Spec.DNSPolicy != corev1.DNSClusterFirstWithHostNet {
		t.Errorf("dns policy not set")
	}

	dnsCfg := &corev1.PodDNSConfig{Nameservers: []string{"8.8.8.8"}}
	if err := SetPodDNSConfig(pod, dnsCfg); err != nil {
		t.Fatalf("SetPodDNSConfig returned error: %v", err)
	}
	if pod.Spec.DNSConfig != dnsCfg {
		t.Errorf("dns config not set")
	}

	if err := SetPodHostname(pod, "myhost"); err != nil {
		t.Fatalf("SetPodHostname returned error: %v", err)
	}
	if pod.Spec.Hostname != "myhost" {
		t.Errorf("hostname not set")
	}

	if err := SetPodSubdomain(pod, "sub"); err != nil {
		t.Fatalf("SetPodSubdomain returned error: %v", err)
	}
	if pod.Spec.Subdomain != "sub" {
		t.Errorf("subdomain not set")
	}

	if err := SetPodRestartPolicy(pod, corev1.RestartPolicyOnFailure); err != nil {
		t.Fatalf("SetPodRestartPolicy returned error: %v", err)
	}
	if pod.Spec.RestartPolicy != corev1.RestartPolicyOnFailure {
		t.Errorf("restart policy not set")
	}

	if err := SetPodTerminationGracePeriod(pod, 15); err != nil {
		t.Fatalf("SetPodTerminationGracePeriod returned error: %v", err)
	}
	if pod.Spec.TerminationGracePeriodSeconds == nil || *pod.Spec.TerminationGracePeriodSeconds != 15 {
		t.Errorf("termination grace period not set")
	}

	if err := SetPodSchedulerName(pod, "sched"); err != nil {
		t.Fatalf("SetPodSchedulerName returned error: %v", err)
	}
	if pod.Spec.SchedulerName != "sched" {
		t.Errorf("scheduler name not set")
	}
}
