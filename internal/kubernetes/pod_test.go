package kubernetes

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
	if pod.Spec.RestartPolicy != "" {
		t.Errorf("unexpected restart policy %v", pod.Spec.RestartPolicy)
	}
	if pod.Spec.TerminationGracePeriodSeconds != nil {
		t.Errorf("expected TerminationGracePeriodSeconds to be nil")
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
	AddPodContainer(pod, &c)
	if len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	AddPodInitContainer(pod, &ic)
	if len(pod.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	AddPodVolume(pod, &v)
	if len(pod.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	secret := corev1.LocalObjectReference{Name: "secret"}
	AddPodImagePullSecret(pod, &secret)
	if len(pod.Spec.ImagePullSecrets) != 1 {
		t.Errorf("image pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	AddPodToleration(pod, &tol)
	if len(pod.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{}}
	AddPodTopologySpreadConstraints(pod, &tsc)
	if len(pod.Spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	SetPodServiceAccountName(pod, "sa")
	if pod.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account name not set")
	}

	sc := &corev1.PodSecurityContext{RunAsUser: func(i int64) *int64 { return &i }(1)}
	SetPodSecurityContext(pod, sc)
	if pod.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	SetPodAffinity(pod, aff)
	if pod.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	ns := map[string]string{"role": "db"}
	SetPodNodeSelector(pod, ns)
	if !reflect.DeepEqual(pod.Spec.NodeSelector, ns) {
		t.Errorf("node selector not set")
	}

	ec := corev1.EphemeralContainer{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debug"}}
	AddPodEphemeralContainer(pod, &ec)
	if len(pod.Spec.EphemeralContainers) != 1 {
		t.Errorf("ephemeral container not added")
	}

	SetPodHostNetwork(pod, true)
	if !pod.Spec.HostNetwork {
		t.Errorf("host network not set")
	}

	SetPodHostPID(pod, true)
	if !pod.Spec.HostPID {
		t.Errorf("host pid not set")
	}

	SetPodHostIPC(pod, true)
	if !pod.Spec.HostIPC {
		t.Errorf("host ipc not set")
	}

	SetPodDNSPolicy(pod, corev1.DNSClusterFirstWithHostNet)
	if pod.Spec.DNSPolicy != corev1.DNSClusterFirstWithHostNet {
		t.Errorf("dns policy not set")
	}

	dnsCfg := &corev1.PodDNSConfig{Nameservers: []string{"8.8.8.8"}}
	SetPodDNSConfig(pod, dnsCfg)
	if pod.Spec.DNSConfig != dnsCfg {
		t.Errorf("dns config not set")
	}

	SetPodHostname(pod, "myhost")
	if pod.Spec.Hostname != "myhost" {
		t.Errorf("hostname not set")
	}

	SetPodSubdomain(pod, "sub")
	if pod.Spec.Subdomain != "sub" {
		t.Errorf("subdomain not set")
	}

	SetPodRestartPolicy(pod, corev1.RestartPolicyOnFailure)
	if pod.Spec.RestartPolicy != corev1.RestartPolicyOnFailure {
		t.Errorf("restart policy not set")
	}

	SetPodTerminationGracePeriod(pod, 15)
	if pod.Spec.TerminationGracePeriodSeconds == nil || *pod.Spec.TerminationGracePeriodSeconds != 15 {
		t.Errorf("termination grace period not set")
	}

	SetPodSchedulerName(pod, "sched")
	if pod.Spec.SchedulerName != "sched" {
		t.Errorf("scheduler name not set")
	}
}

func TestSetPodPriorityClassName(t *testing.T) {
	pod := CreatePod("p", "ns")
	SetPodPriorityClassName(pod, "high")
	if pod.Spec.PriorityClassName != "high" {
		t.Errorf("priority class name not set")
	}
}
