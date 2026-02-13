package kubernetes

import (
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateCronJob(t *testing.T) {
	cj := CreateCronJob("my-app", "default", "*/5 * * * *")
	if cj.Name != "my-app" || cj.Namespace != "default" {
		t.Fatalf("metadata mismatch: %s/%s", cj.Namespace, cj.Name)
	}
	if cj.Kind != "CronJob" {
		t.Errorf("unexpected kind %q", cj.Kind)
	}
	if cj.Labels["app"] != "my-app" {
		t.Errorf("expected label app=my-app, got %v", cj.Labels)
	}
	if cj.Spec.Schedule != "*/5 * * * *" {
		t.Errorf("expected schedule */5 * * * *, got %q", cj.Spec.Schedule)
	}
	if cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("expected restart policy Never, got %q", cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy)
	}
}

func TestCronJobNilErrors(t *testing.T) {
	if err := SetCronJobPodSpec(nil, &corev1.PodSpec{}); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobPodSpec")
	}
	if err := AddCronJobContainer(nil, &corev1.Container{Name: "c"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobContainer")
	}
	if err := AddCronJobInitContainer(nil, &corev1.Container{Name: "c"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobInitContainer")
	}
	if err := AddCronJobVolume(nil, &corev1.Volume{Name: "v"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobVolume")
	}
	if err := AddCronJobImagePullSecret(nil, &corev1.LocalObjectReference{Name: "s"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobImagePullSecret")
	}
	if err := AddCronJobToleration(nil, &corev1.Toleration{Key: "k"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobToleration")
	}
	if err := AddCronJobTopologySpreadConstraint(nil, &corev1.TopologySpreadConstraint{}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobTopologySpreadConstraint")
	}
	if err := SetCronJobServiceAccountName(nil, "sa"); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobServiceAccountName")
	}
	if err := SetCronJobSecurityContext(nil, &corev1.PodSecurityContext{}); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobSecurityContext")
	}
	if err := SetCronJobAffinity(nil, &corev1.Affinity{}); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobAffinity")
	}
	if err := SetCronJobNodeSelector(nil, map[string]string{}); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobNodeSelector")
	}
	if err := SetCronJobSchedule(nil, "* * * * *"); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobSchedule")
	}
	if err := SetCronJobConcurrencyPolicy(nil, batchv1.ForbidConcurrent); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobConcurrencyPolicy")
	}
	if err := SetCronJobSuspend(nil, true); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobSuspend")
	}
	if err := SetCronJobSuccessfulJobsHistoryLimit(nil, 3); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobSuccessfulJobsHistoryLimit")
	}
	if err := SetCronJobFailedJobsHistoryLimit(nil, 1); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobFailedJobsHistoryLimit")
	}
	if err := SetCronJobStartingDeadlineSeconds(nil, 60); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobStartingDeadlineSeconds")
	}
	tz := "UTC"
	if err := SetCronJobTimeZone(nil, &tz); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobTimeZone")
	}
}

func TestCronJobNilArgErrors(t *testing.T) {
	cj := CreateCronJob("test", "default", "* * * * *")
	if err := SetCronJobPodSpec(cj, nil); err == nil {
		t.Error("expected error for nil PodSpec")
	}
	if err := AddCronJobContainer(cj, nil); err == nil {
		t.Error("expected error for nil Container")
	}
	if err := AddCronJobInitContainer(cj, nil); err == nil {
		t.Error("expected error for nil InitContainer")
	}
	if err := AddCronJobVolume(cj, nil); err == nil {
		t.Error("expected error for nil Volume")
	}
	if err := AddCronJobImagePullSecret(cj, nil); err == nil {
		t.Error("expected error for nil ImagePullSecret")
	}
	if err := AddCronJobToleration(cj, nil); err == nil {
		t.Error("expected error for nil Toleration")
	}
}

func TestCronJobTopologySpreadConstraints(t *testing.T) {
	t.Run("nil constraint", func(t *testing.T) {
		cj := CreateCronJob("test", "default", "* * * * *")
		if err := AddCronJobTopologySpreadConstraint(cj, nil); err != nil {
			t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
		}
		if len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints) != 0 {
			t.Errorf("expected no constraints, got %d", len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints))
		}
	})

	t.Run("append single constraint", func(t *testing.T) {
		cj := CreateCronJob("test", "default", "* * * * *")
		c := corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "topology.kubernetes.io/zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		}
		if err := AddCronJobTopologySpreadConstraint(cj, &c); err != nil {
			t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
		}
		if len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
			t.Fatalf("expected 1 constraint, got %d", len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints[0], c) {
			t.Errorf("constraint mismatch: got %+v, want %+v", cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints[0], c)
		}
	})

	t.Run("append additional constraint", func(t *testing.T) {
		cj := CreateCronJob("test", "default", "* * * * *")
		first := corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		}
		second := corev1.TopologySpreadConstraint{
			MaxSkew:           2,
			TopologyKey:       "hostname",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		}
		if err := AddCronJobTopologySpreadConstraint(cj, &first); err != nil {
			t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
		}
		if err := AddCronJobTopologySpreadConstraint(cj, &second); err != nil {
			t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
		}
		if len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints) != 2 {
			t.Fatalf("expected 2 constraints, got %d", len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints[0], first) {
			t.Errorf("first constraint mismatch")
		}
		if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints[1], second) {
			t.Errorf("second constraint mismatch")
		}
	})
}

func TestCronJobFunctions(t *testing.T) {
	cj := CreateCronJob("app", "ns", "* * * * *")
	if cj.Name != "app" || cj.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", cj.Namespace, cj.Name)
	}
	if cj.Kind != "CronJob" {
		t.Errorf("unexpected kind %q", cj.Kind)
	}

	c := corev1.Container{Name: "c"}
	if err := AddCronJobContainer(cj, &c); err != nil {
		t.Fatalf("AddCronJobContainer returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Containers) != 1 || cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Name != "c" {
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

	secret := corev1.LocalObjectReference{Name: "secret"}
	if err := AddCronJobImagePullSecret(cj, &secret); err != nil {
		t.Fatalf("AddCronJobImagePullSecret returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("image pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	if err := AddCronJobToleration(cj, &tol); err != nil {
		t.Fatalf("AddCronJobToleration returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{}}
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
		t.Errorf("service account name not set")
	}

	sc := &corev1.PodSecurityContext{RunAsUser: func(i int64) *int64 { return &i }(1)}
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

	ns := map[string]string{"role": "db"}
	if err := SetCronJobNodeSelector(cj, ns); err != nil {
		t.Fatalf("SetCronJobNodeSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.NodeSelector, ns) {
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
		t.Errorf("concurrency policy not set")
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
		t.Errorf("successful jobs history limit not set")
	}

	if err := SetCronJobFailedJobsHistoryLimit(cj, 2); err != nil {
		t.Fatalf("SetCronJobFailedJobsHistoryLimit returned error: %v", err)
	}
	if cj.Spec.FailedJobsHistoryLimit == nil || *cj.Spec.FailedJobsHistoryLimit != 2 {
		t.Errorf("failed jobs history limit not set")
	}

	if err := SetCronJobStartingDeadlineSeconds(cj, 60); err != nil {
		t.Fatalf("SetCronJobStartingDeadlineSeconds returned error: %v", err)
	}
	if cj.Spec.StartingDeadlineSeconds == nil || *cj.Spec.StartingDeadlineSeconds != 60 {
		t.Errorf("starting deadline seconds not set")
	}

	tz := "UTC"
	if err := SetCronJobTimeZone(cj, &tz); err != nil {
		t.Fatalf("SetCronJobTimeZone returned error: %v", err)
	}
	if cj.Spec.TimeZone == nil || *cj.Spec.TimeZone != "UTC" {
		t.Errorf("timezone not set")
	}
}

func TestSetCronJobPodSpec(t *testing.T) {
	cj := CreateCronJob("test", "default", "* * * * *")
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "test", Image: "nginx"},
		},
	}
	if err := SetCronJobPodSpec(cj, spec); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Containers) != 1 {
		t.Fatal("expected PodSpec to be assigned")
	}
}
