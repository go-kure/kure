package application

import (
	"testing"
)

func TestValidate(t *testing.T) {
	as := &Bundle{Name: "", Applications: &[]*Application{}}
	if err := as.Validate(); err == nil {
		t.Fatalf("expected validation error")
	}
}
