package fluxcd

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScheduleHelpers(t *testing.T) {
	sc := CreateSchedule("@hourly")
	if err := SetScheduleTimeZone(&sc, "UTC"); err != nil {
		t.Fatalf("SetScheduleTimeZone returned error: %v", err)
	}
	if err := SetScheduleWindow(&sc, metav1.Duration{Duration: 0}); err != nil {
		t.Fatalf("SetScheduleWindow returned error: %v", err)
	}
	if sc.Cron != "@hourly" {
		t.Errorf("cron not set")
	}
	if sc.TimeZone != "UTC" {
		t.Errorf("tz not set")
	}
}
