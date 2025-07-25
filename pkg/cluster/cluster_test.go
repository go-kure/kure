package cluster

import (
	"testing"

	"github.com/go-kure/kure/pkg/cluster/api"
)

func TestBuildLayoutUsesDefaultFilePer(t *testing.T) {
	c := NewCluster("demo", "5m", "flux-system", nil)
	c.SetFilePer(api.FilePerKind)

	group := api.AppGroup{
		Name:      "core",
		Namespace: "default",
		Apps: []api.AppDeploymentConfig{{
			Name: "app",
		}},
	}

	c.AddAppSet(AppSet{AppGroup: group})

	manifests, _, _, err := c.BuildLayout(LayoutRules{})
	if err != nil {
		t.Fatalf("BuildLayout failed: %v", err)
	}
	if len(manifests) == 0 {
		t.Fatal("no manifest layouts returned")
	}
	if manifests[0].FilePer != api.FilePerKind {
		t.Fatalf("expected FilePerKind, got %q", manifests[0].FilePer)
	}
}
