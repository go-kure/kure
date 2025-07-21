package bootstrap

import "testing"

func TestNewFluxBootstrap(t *testing.T) {
	fl, err := NewFluxBootstrap("prod", "custom", "10m", "clusters/prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fl.Interval != "10m" || fl.SourceRef != "custom" {
		t.Fatalf("fields not set from params")
	}
	if fl.TargetPath != "clusters/prod" {
		t.Fatalf("unexpected target path %s", fl.TargetPath)
	}
}
