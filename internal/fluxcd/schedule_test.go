package fluxcd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestScheduleHelpers(t *testing.T) {
	sc := CreateSchedule("@hourly")
	SetScheduleTimeZone(&sc, "UTC")
	SetScheduleWindow(&sc, metav1.Duration{Duration: 0})
	if sc.Cron != "@hourly" {
		t.Errorf("cron not set")
	}
	if sc.TimeZone != "UTC" {
		t.Errorf("tz not set")
	}
}
