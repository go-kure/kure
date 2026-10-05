package argocd

import (
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestGenerateFromCluster_DirName: under the ArgoCD workflow an umbrella
// child's source.path follows Bundle.DirName, and the Application keeps the
// bundle's name. On a bundle rendered in its node's directory, as under the
// default rules, DirName has no effect.
func TestGenerateFromCluster_DirName(t *testing.T) {
	infra := &stack.Bundle{Name: "shop-infra", DirName: "00-infra"}
	shop := &stack.Bundle{Name: "shop", DirName: "10-shop", Children: []*stack.Bundle{infra}}
	apps := &stack.Node{Name: "apps", Bundle: shop}
	r := &stack.Node{Name: "r", Children: []*stack.Node{apps}}
	apps.SetParent(r)

	objs, err := Engine().GenerateFromCluster(&stack.Cluster{Name: "demo", Node: r}, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	want := map[string]string{"shop": "r/apps", "shop-infra": "r/apps/00-infra"}
	if got := appPaths(t, objs); !pathsEqual(got, want) {
		t.Errorf("Application paths = %v, want %v", got, want)
	}
}

// TestGenerateFromCluster_RootDirName: the root node's bundle has a directory
// inside the root node's under the default rules, named by its DirName. Its
// Application's source.path follows, and so do the paths below it.
func TestGenerateFromCluster_RootDirName(t *testing.T) {
	c := argoTestCluster()
	c.Node.Bundle.DirName = "00-platform"
	c.Node.Bundle.Children = []*stack.Bundle{{Name: "platform-db", DirName: "00-db"}}
	objs, err := Engine().GenerateFromCluster(c, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	want := map[string]string{
		"platform-bundle": "platform/00-platform",
		"platform-db":     "platform/00-platform/00-db",
		"web-bundle":      "platform/web",
		"web-db":          "platform/web/web-db",
	}
	if got := appPaths(t, objs); !pathsEqual(got, want) {
		t.Errorf("Application paths = %v, want %v", got, want)
	}
}

// TestCreateLayoutWithResources_DirNameInTheArgoDirectory: the Applications'
// directory must be free, and the check compares the directory the root
// node's bundle gets. A DirName that names it is refused, also in another
// case (a DirName is no object name and may hold upper case), and the error
// names the bundle by its Name. A bundle named like that directory is written
// once its DirName names another.
func TestCreateLayoutWithResources_DirNameInTheArgoDirectory(t *testing.T) {
	for _, dirName := range []string{"argocd", "ArgoCD"} {
		c := argoTestCluster()
		c.Node.Bundle.DirName = dirName
		want := `bundle "platform-bundle" is rendered to "platform/` + dirName + `", the directory the ArgoCD Applications are written to`
		_, err := Engine().CreateLayoutWithResources(c, layout.DefaultLayoutRules())
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("DirName %q: got %v, want the refusal %q", dirName, err, want)
		}
		if err != nil && !strings.Contains(err.Error(), "DirName") {
			t.Errorf("DirName %q: error %q does not say the directory is named by DirName", dirName, err)
		}
	}

	c := argoTestCluster()
	c.Node.Bundle.Name = "argocd"
	c.Node.Bundle.DirName = "00-platform"
	result, err := Engine().CreateLayoutWithResources(c, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("bundle argocd with DirName 00-platform: %v", err)
	}
	if err := result.(*layout.ManifestLayout).WriteToDisk(t.TempDir()); err != nil {
		t.Errorf("bundle argocd with DirName 00-platform, WriteToDisk: %v", err)
	}
}

// TestGenerateFromCluster_SameDirNameRefused: two umbrella children with one
// directory name would get Applications with one source.path; they are
// refused, naming both bundles.
func TestGenerateFromCluster_SameDirNameRefused(t *testing.T) {
	shop := &stack.Bundle{Name: "shop", Children: []*stack.Bundle{
		{Name: "shop-infra", DirName: "00-base"},
		{Name: "shop-services", DirName: "00-base"},
	}}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "r", Bundle: shop}}
	_, err := Engine().GenerateFromCluster(c, layout.DefaultLayoutRules())
	if err == nil {
		t.Fatal("GenerateFromCluster accepted two bundles with one directory")
	}
	// The umbrella is the root node's bundle, with its directory inside r.
	for _, want := range []string{`"shop/shop-infra"`, `"shop/shop-services"`, `"r/shop/00-base"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %s", err, want)
		}
	}
}
