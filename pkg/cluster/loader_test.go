package cluster

import (
	"os"
	"testing"
)

func TestLoadClusterConfigFromYAML(t *testing.T) {
	data := []byte("name: demo\ninterval: 5m\nsourceRef: flux-system\n")
	f, err := os.CreateTemp(t.TempDir(), "cfg*.yaml")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	f.Close()

	cfg, err := LoadClusterConfigFromYAML(f.Name())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Name != "demo" || cfg.Interval != "5m" || cfg.SourceRef != "flux-system" {
		t.Fatalf("unexpected config %+v", cfg)
	}
}
