package metallb

import (
	"testing"

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
	if err := SetBFDProfileDetectMultiplier(p, 2); err != nil {
		t.Fatalf("SetBFDProfileDetectMultiplier returned error: %v", err)
	}
	if err := SetBFDProfileEchoInterval(p, 50); err != nil {
		t.Fatalf("SetBFDProfileEchoInterval returned error: %v", err)
	}
	if err := SetBFDProfileEchoMode(p, true); err != nil {
		t.Fatalf("SetBFDProfileEchoMode returned error: %v", err)
	}
	if err := SetBFDProfilePassiveMode(p, true); err != nil {
		t.Fatalf("SetBFDProfilePassiveMode returned error: %v", err)
	}

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

	if err := SetBFDProfileDetectMultiplier(nil, 1); err == nil {
		t.Errorf("expected error when profile nil")
	}
	if err := SetBFDProfileEchoInterval(nil, 1); err == nil {
		t.Errorf("expected error when profile nil")
	}
	if err := SetBFDProfileEchoMode(nil, true); err == nil {
		t.Errorf("expected error when profile nil")
	}
	if err := SetBFDProfilePassiveMode(nil, true); err == nil {
		t.Errorf("expected error when profile nil")
	}
}
