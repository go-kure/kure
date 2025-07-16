package k8s

import (
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
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
	if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("unexpected restart policy %v", job.Spec.Template.Spec.RestartPolicy)
	}
	if len(job.Spec.Template.Spec.Containers) != 0 {
		t.Errorf("expected no containers")
	}
}

func TestJobFunctions(t *testing.T) {
	job := CreateJob("job", "ns")

	c := corev1.Container{Name: "c"}
	AddJobContainer(job, &c)
	if len(job.Spec.Template.Spec.Containers) != 1 || job.Spec.Template.Spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	AddJobInitContainer(job, &ic)
	if len(job.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	AddJobVolume(job, &v)
	if len(job.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	secret := corev1.LocalObjectReference{Name: "pull"}
	AddJobImagePullSecret(job, &secret)
	if len(job.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	AddJobToleration(job, &tol)
	if len(job.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{}}
	AddJobTopologySpreadConstraint(job, &tsc)
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
}

func TestCreateCronJob(t *testing.T) {
	cj := CreateCronJob("cron", "ns", "* * * * *")
	if cj.Name != "cron" || cj.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", cj.Namespace, cj.Name)
	}
	if cj.Kind != "CronJob" {
		t.Errorf("unexpected kind %q", cj.Kind)
	}
	if cj.Spec.Schedule != "* * * * *" {
		t.Errorf("schedule not set")
	}
	if cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("unexpected restart policy")
	}
}

func TestCronJobFunctions(t *testing.T) {
	cj := CreateCronJob("cron", "ns", "* * * * *")

	c := corev1.Container{Name: "c"}
	AddCronJobContainer(cj, &c)
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	AddCronJobInitContainer(cj, &ic)
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	AddCronJobVolume(cj, &v)
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	sec := corev1.LocalObjectReference{Name: "pull"}
	AddCronJobImagePullSecret(cj, &sec)
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	AddCronJobToleration(cj, &tol)
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{}}
	AddCronJobTopologySpreadConstraint(cj, &tsc)
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	SetCronJobServiceAccountName(cj, "sa")
	if cj.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account not set")
	}

	sc := &corev1.PodSecurityContext{}
	SetCronJobSecurityContext(cj, sc)
	if cj.Spec.JobTemplate.Spec.Template.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	SetCronJobAffinity(cj, aff)
	if cj.Spec.JobTemplate.Spec.Template.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	sel := map[string]string{"role": "db"}
	SetCronJobNodeSelector(cj, sel)
	if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.NodeSelector, sel) {
		t.Errorf("node selector not set")
	}

	SetCronJobSchedule(cj, "*/5 * * * *")
	if cj.Spec.Schedule != "*/5 * * * *" {
		t.Errorf("schedule not updated")
	}

	SetCronJobConcurrencyPolicy(cj, batchv1.ForbidConcurrent)
	if cj.Spec.ConcurrencyPolicy != batchv1.ForbidConcurrent {
		t.Errorf("policy not set")
	}

	SetCronJobSuspend(cj, true)
	if cj.Spec.Suspend == nil || !*cj.Spec.Suspend {
		t.Errorf("suspend not set")
	}

	SetCronJobSuccessfulJobsHistoryLimit(cj, 1)
	if cj.Spec.SuccessfulJobsHistoryLimit == nil || *cj.Spec.SuccessfulJobsHistoryLimit != 1 {
		t.Errorf("success limit not set")
	}

	SetCronJobFailedJobsHistoryLimit(cj, 2)
	if cj.Spec.FailedJobsHistoryLimit == nil || *cj.Spec.FailedJobsHistoryLimit != 2 {
		t.Errorf("failed limit not set")
	}

	SetCronJobStartingDeadlineSeconds(cj, 60)
	if cj.Spec.StartingDeadlineSeconds == nil || *cj.Spec.StartingDeadlineSeconds != 60 {
		t.Errorf("deadline not set")
	}
}
