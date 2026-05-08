package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateJob(t *testing.T) {
	job := CreateJob("job", "ns")
	if job.Name != "job" || job.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", job.Namespace, job.Name)
	}
	if job.Kind != "Job" {
		t.Errorf("unexpected kind %q", job.Kind)
	}
	if job.Spec.Template.Spec.RestartPolicy != "" {
		t.Errorf("unexpected restart policy %v", job.Spec.Template.Spec.RestartPolicy)
	}
	if len(job.Spec.Template.Spec.Containers) != 0 {
		t.Errorf("expected no containers")
	}
}

func TestJobFunctions(t *testing.T) {
	job := CreateJob("job", "ns")

	c := corev1.Container{Name: "c"}
	if err := AddJobContainer(job, &c); err != nil {
		t.Fatalf("AddJobContainer returned error: %v", err)
	}
	if len(job.Spec.Template.Spec.Containers) != 1 || job.Spec.Template.Spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	if err := AddJobInitContainer(job, &ic); err != nil {
		t.Fatalf("AddJobInitContainer returned error: %v", err)
	}
	if len(job.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	if err := AddJobVolume(job, &v); err != nil {
		t.Fatalf("AddJobVolume returned error: %v", err)
	}
	if len(job.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	secret := corev1.LocalObjectReference{Name: "pull"}
	if err := AddJobImagePullSecret(job, &secret); err != nil {
		t.Fatalf("AddJobImagePullSecret returned error: %v", err)
	}
	if len(job.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	if err := AddJobToleration(job, &tol); err != nil {
		t.Fatalf("AddJobToleration returned error: %v", err)
	}
	if len(job.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{}}
	if err := AddJobTopologySpreadConstraint(job, &tsc); err != nil {
		t.Fatalf("AddJobTopologySpreadConstraint returned error: %v", err)
	}
	if len(job.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	SetJobServiceAccountName(job, "sa")
	if job.Spec.Template.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account not set")
	}

	sc := &corev1.PodSecurityContext{}
	SetJobSecurityContext(job, sc)
	if job.Spec.Template.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	SetJobAffinity(job, aff)
	if job.Spec.Template.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	sel := map[string]string{"role": "db"}
	SetJobNodeSelector(job, sel)
	if !reflect.DeepEqual(job.Spec.Template.Spec.NodeSelector, sel) {
		t.Errorf("node selector not set")
	}

	SetJobCompletions(job, 2)
	if job.Spec.Completions == nil || *job.Spec.Completions != 2 {
		t.Errorf("completions not set")
	}

	SetJobParallelism(job, 3)
	if job.Spec.Parallelism == nil || *job.Spec.Parallelism != 3 {
		t.Errorf("parallelism not set")
	}

	SetJobBackoffLimit(job, 4)
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 4 {
		t.Errorf("backoff limit not set")
	}

	SetJobTTLSecondsAfterFinished(job, 30)
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 30 {
		t.Errorf("ttl not set")
	}

	ad := int64(100)
	SetJobActiveDeadlineSeconds(job, &ad)
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 100 {
		t.Errorf("active deadline not set")
	}
}

func TestJobNilGuards(t *testing.T) {
	ad := int64(1)

	// Functions with secondary nil checks — still return errors
	if err := SetJobPodSpec(nil, &corev1.PodSpec{}); err == nil {
		t.Error("SetJobPodSpec(nil) should return error")
	}
	if err := AddJobContainer(nil, &corev1.Container{}); err == nil {
		t.Error("AddJobContainer(nil) should return error")
	}
	if err := AddJobInitContainer(nil, &corev1.Container{}); err == nil {
		t.Error("AddJobInitContainer(nil) should return error")
	}
	if err := AddJobVolume(nil, &corev1.Volume{}); err == nil {
		t.Error("AddJobVolume(nil) should return error")
	}
	if err := AddJobImagePullSecret(nil, &corev1.LocalObjectReference{}); err == nil {
		t.Error("AddJobImagePullSecret(nil) should return error")
	}
	if err := AddJobToleration(nil, &corev1.Toleration{}); err == nil {
		t.Error("AddJobToleration(nil) should return error")
	}
	if err := AddJobTopologySpreadConstraint(nil, &corev1.TopologySpreadConstraint{}); err == nil {
		t.Error("AddJobTopologySpreadConstraint(nil) should return error")
	}

	// Functions that now panic on nil receiver
	assertPanics(t, func() { SetJobServiceAccountName(nil, "sa") })
	assertPanics(t, func() { SetJobSecurityContext(nil, nil) })
	assertPanics(t, func() { SetJobAffinity(nil, nil) })
	assertPanics(t, func() { SetJobNodeSelector(nil, nil) })
	assertPanics(t, func() { SetJobCompletions(nil, 1) })
	assertPanics(t, func() { SetJobParallelism(nil, 1) })
	assertPanics(t, func() { SetJobBackoffLimit(nil, 1) })
	assertPanics(t, func() { SetJobTTLSecondsAfterFinished(nil, 1) })
	assertPanics(t, func() { SetJobActiveDeadlineSeconds(nil, &ad) })
}
