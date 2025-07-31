package kubernetes

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

	if err := SetJobServiceAccountName(job, "sa"); err != nil {
		t.Fatalf("SetJobServiceAccountName returned error: %v", err)
	}
	if job.Spec.Template.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account not set")
	}

	sc := &corev1.PodSecurityContext{}
	if err := SetJobSecurityContext(job, sc); err != nil {
		t.Fatalf("SetJobSecurityContext returned error: %v", err)
	}
	if job.Spec.Template.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	if err := SetJobAffinity(job, aff); err != nil {
		t.Fatalf("SetJobAffinity returned error: %v", err)
	}
	if job.Spec.Template.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	sel := map[string]string{"role": "db"}
	if err := SetJobNodeSelector(job, sel); err != nil {
		t.Fatalf("SetJobNodeSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(job.Spec.Template.Spec.NodeSelector, sel) {
		t.Errorf("node selector not set")
	}

	if err := SetJobCompletions(job, 2); err != nil {
		t.Fatalf("SetJobCompletions returned error: %v", err)
	}
	if job.Spec.Completions == nil || *job.Spec.Completions != 2 {
		t.Errorf("completions not set")
	}

	if err := SetJobParallelism(job, 3); err != nil {
		t.Fatalf("SetJobParallelism returned error: %v", err)
	}
	if job.Spec.Parallelism == nil || *job.Spec.Parallelism != 3 {
		t.Errorf("parallelism not set")
	}

	if err := SetJobBackoffLimit(job, 4); err != nil {
		t.Fatalf("SetJobBackoffLimit returned error: %v", err)
	}
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 4 {
		t.Errorf("backoff limit not set")
	}

	if err := SetJobTTLSecondsAfterFinished(job, 30); err != nil {
		t.Fatalf("SetJobTTLSecondsAfterFinished returned error: %v", err)
	}
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 30 {
		t.Errorf("ttl not set")
	}

	ad := int64(100)
	if err := SetJobActiveDeadlineSeconds(job, &ad); err != nil {
		t.Fatalf("SetJobActiveDeadlineSeconds returned error: %v", err)
	}
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 100 {
		t.Errorf("active deadline not set")
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
	if cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy != "" {
		t.Errorf("unexpected restart policy")
	}
}

func TestCronJobFunctions(t *testing.T) {
	cj := CreateCronJob("cron", "ns", "* * * * *")

	c := corev1.Container{Name: "c"}
	if err := AddCronJobContainer(cj, &c); err != nil {
		t.Fatalf("AddCronJobContainer returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	if err := AddCronJobInitContainer(cj, &ic); err != nil {
		t.Fatalf("AddCronJobInitContainer returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	if err := AddCronJobVolume(cj, &v); err != nil {
		t.Fatalf("AddCronJobVolume returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	sec := corev1.LocalObjectReference{Name: "pull"}
	if err := AddCronJobImagePullSecret(cj, &sec); err != nil {
		t.Fatalf("AddCronJobImagePullSecret returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	if err := AddCronJobToleration(cj, &tol); err != nil {
		t.Fatalf("AddCronJobToleration returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{}}
	if err := AddCronJobTopologySpreadConstraint(cj, &tsc); err != nil {
		t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	if err := SetCronJobServiceAccountName(cj, "sa"); err != nil {
		t.Fatalf("SetCronJobServiceAccountName returned error: %v", err)
	}
	if cj.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account not set")
	}

	sc := &corev1.PodSecurityContext{}
	if err := SetCronJobSecurityContext(cj, sc); err != nil {
		t.Fatalf("SetCronJobSecurityContext returned error: %v", err)
	}
	if cj.Spec.JobTemplate.Spec.Template.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	if err := SetCronJobAffinity(cj, aff); err != nil {
		t.Fatalf("SetCronJobAffinity returned error: %v", err)
	}
	if cj.Spec.JobTemplate.Spec.Template.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	sel := map[string]string{"role": "db"}
	if err := SetCronJobNodeSelector(cj, sel); err != nil {
		t.Fatalf("SetCronJobNodeSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.NodeSelector, sel) {
		t.Errorf("node selector not set")
	}

	if err := SetCronJobSchedule(cj, "*/5 * * * *"); err != nil {
		t.Fatalf("SetCronJobSchedule returned error: %v", err)
	}
	if cj.Spec.Schedule != "*/5 * * * *" {
		t.Errorf("schedule not updated")
	}

	if err := SetCronJobConcurrencyPolicy(cj, batchv1.ForbidConcurrent); err != nil {
		t.Fatalf("SetCronJobConcurrencyPolicy returned error: %v", err)
	}
	if cj.Spec.ConcurrencyPolicy != batchv1.ForbidConcurrent {
		t.Errorf("policy not set")
	}

	if err := SetCronJobSuspend(cj, true); err != nil {
		t.Fatalf("SetCronJobSuspend returned error: %v", err)
	}
	if cj.Spec.Suspend == nil || !*cj.Spec.Suspend {
		t.Errorf("suspend not set")
	}

	if err := SetCronJobSuccessfulJobsHistoryLimit(cj, 1); err != nil {
		t.Fatalf("SetCronJobSuccessfulJobsHistoryLimit returned error: %v", err)
	}
	if cj.Spec.SuccessfulJobsHistoryLimit == nil || *cj.Spec.SuccessfulJobsHistoryLimit != 1 {
		t.Errorf("success limit not set")
	}

	if err := SetCronJobFailedJobsHistoryLimit(cj, 2); err != nil {
		t.Fatalf("SetCronJobFailedJobsHistoryLimit returned error: %v", err)
	}
	if cj.Spec.FailedJobsHistoryLimit == nil || *cj.Spec.FailedJobsHistoryLimit != 2 {
		t.Errorf("failed limit not set")
	}

	if err := SetCronJobStartingDeadlineSeconds(cj, 60); err != nil {
		t.Fatalf("SetCronJobStartingDeadlineSeconds returned error: %v", err)
	}
	if cj.Spec.StartingDeadlineSeconds == nil || *cj.Spec.StartingDeadlineSeconds != 60 {
		t.Errorf("deadline not set")
	}

	tz := "UTC"
	if err := SetCronJobTimeZone(cj, &tz); err != nil {
		t.Fatalf("SetCronJobTimeZone returned error: %v", err)
	}
	if cj.Spec.TimeZone == nil || *cj.Spec.TimeZone != "UTC" {
		t.Errorf("timezone not set")
	}
}
