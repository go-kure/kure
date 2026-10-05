package stack

import (
	"strings"
	"testing"
)

// TestDeepCopyBundle_DirName: the fluent builder copies bundles, and a copy
// must get the same directory as the bundle it was copied from.
func TestDeepCopyBundle_DirName(t *testing.T) {
	got := deepCopyBundle(&Bundle{Name: "shop-infra", DirName: "00-infra"})
	if got.DirName != "00-infra" {
		t.Errorf("copy has DirName %q, want 00-infra", got.DirName)
	}
}

// TestBundleValidate_DirName: a DirName names one directory, so it must be
// one path segment (ValidateDirectoryName), on the bundle and on every
// umbrella descendant, and the error names the bundle by its path and the
// field. It is no object name: a name no Kustomization could carry is a valid
// directory name. Empty means the bundle's Name and is not checked.
func TestBundleValidate_DirName(t *testing.T) {
	valid := []string{"", "00-infra", "shop-infra", "Infra_00.d"}
	invalid := map[string]string{
		".":         `"." is not a directory name of its own`,
		"..":        `".." is not a directory name of its own`,
		"a/b":       `"a/b" contains a path separator`,
		"/infra":    `"/infra" contains a path separator`,
		"../infra":  `"../infra" contains a path separator`,
		`..\infra`:  `"..\\infra" contains a path separator`,
		"in\x00fra": "contains a NUL byte",
	}
	shop := func(own, child string) *Bundle {
		return &Bundle{Name: "shop", DirName: own, Children: []*Bundle{{Name: "shop-infra", DirName: child}}}
	}
	for _, dirName := range valid {
		if err := shop(dirName, "").Validate(); err != nil {
			t.Errorf("bundle with DirName %q: %v", dirName, err)
		}
		if err := shop("", dirName).Validate(); err != nil {
			t.Errorf("umbrella child with DirName %q: %v", dirName, err)
		}
	}
	for dirName, want := range invalid {
		for path, b := range map[string]*Bundle{"shop": shop(dirName, ""), "shop/shop-infra": shop("", dirName)} {
			err := b.Validate()
			if err == nil {
				t.Errorf("bundle %q: DirName %q was accepted", path, dirName)
				continue
			}
			for _, part := range []string{"Bundle '" + path + "'", "field 'dirName'", want} {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("bundle %q, DirName %q: error %q does not contain %s", path, dirName, err, part)
				}
			}
		}
	}

	// ValidateCluster, which every walk runs first, reaches a node's bundle.
	c := &Cluster{Name: "demo", Node: &Node{Name: "apps", Bundle: shop("", "a/b")}}
	if err := ValidateCluster(c); err == nil || !strings.Contains(err.Error(), `"a/b" contains a path separator`) {
		t.Errorf("ValidateCluster: got %v, want the refusal of DirName %q", err, "a/b")
	}
}
