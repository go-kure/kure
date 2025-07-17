package fluxcd

import (
	"errors"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateSchedule returns a Schedule with the given cron expression.
func CreateSchedule(cron string) fluxv1.Schedule {
	return fluxv1.Schedule{Cron: cron}
}

// SetScheduleTimeZone sets the time zone on the schedule.
func SetScheduleTimeZone(s *fluxv1.Schedule, tz string) error {
	if s == nil {
		return errors.New("nil Schedule")
	}
	s.TimeZone = tz
	return nil
}

// SetScheduleWindow sets the execution window.
func SetScheduleWindow(s *fluxv1.Schedule, d metav1.Duration) error {
	if s == nil {
		return errors.New("nil Schedule")
	}
	s.Window = d
	return nil
}
