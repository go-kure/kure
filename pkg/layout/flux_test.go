package layout_test

import (
	"os"
	"path/filepath"
	"testing"

	cllayout "github.com/go-kure/kure/pkg/cluster/layout"
	"github.com/go-kure/kure/pkg/layout"
)

func TestFluxLayoutWriteWithConfig(t *testing.T) {
	fl := &layout.FluxLayout{Name: "app", TargetPath: "demo/app"}

	cfg := cllayout.DefaultLayoutConfig()
	cfg.FluxDir = "flux"
	cfg.KustomizationFileName = func(name string) string { return name + ".flux.yaml" }

	dir := t.TempDir()
	if err := cllayout.WriteFlux(dir, cfg, fl); err != nil {
		t.Fatalf("write with config: %v", err)
	}

	expected := filepath.Join(dir, "flux", "demo", "app", "app.flux.yaml")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected file not written: %v", err)
	}
}
