package metallb

import "testing"

import (
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

func TestCreateBFDProfile(t *testing.T) {
	prof := CreateBFDProfile("prof", "default", metallbv1beta1.BFDProfileSpec{})
	if prof.Name != "prof" || prof.Namespace != "default" {
		t.Fatalf("unexpected metadata")
	}
}

func TestBFDProfileHelpers(t *testing.T) {
	p := CreateBFDProfile("prof", "ns", metallbv1beta1.BFDProfileSpec{})
	SetBFDProfileDetectMultiplier(p, 2)
	SetBFDProfileEchoInterval(p, 50)
	SetBFDProfileEchoMode(p, true)
	SetBFDProfilePassiveMode(p, true)

	if p.Spec.DetectMultiplier == nil || *p.Spec.DetectMultiplier != 2 {
		t.Errorf("detect multiplier not set")
	}
	if p.Spec.EchoInterval == nil || *p.Spec.EchoInterval != 50 {
		t.Errorf("echo interval not set")
	}
	if p.Spec.EchoMode == nil || !*p.Spec.EchoMode {
		t.Errorf("echo mode not set")
	}
	if p.Spec.PassiveMode == nil || !*p.Spec.PassiveMode {
		t.Errorf("passive mode not set")
	}
}
