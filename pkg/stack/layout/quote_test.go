package layout_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// yaml11Names are names kustomize's YAML reader takes for something other than
// a string when written plain: a YAML 1.1 bool, null or number
// (go-kure/kure#896).
var yaml11Names = []string{"y", "yes", "on", "no", "off", "true", "null", "1", "1e3", "0x1f"}

// TestWriters_QuoteEntriesKustomizeReadsAsOtherTypes: a child directory, a
// ConfigMapGenerator or a generator file named like a YAML bool, null or number,
// and a resource file whose name YAML reads as something else, are written so
// that kustomize reads each as the same string, and every writer's output
// builds.
func TestWriters_QuoteEntriesKustomizeReadsAsOtherTypes(t *testing.T) {
	trees := map[string]func() *layout.ManifestLayout{
		"child directories": func() *layout.ManifestLayout {
			p := &layout.ManifestLayout{Name: "p", Namespace: "."}
			for _, n := range yaml11Names {
				p.Children = append(p.Children, cmLayout(n, p.FullRepoPath()))
			}
			return p
		},
		// A resource file is named after its object, and a ClusterRole's name
		// may hold " #" or ": ", which YAML reads as a comment or a mapping.
		"resource files": func() *layout.ManifestLayout {
			p := &layout.ManifestLayout{Name: "p", Namespace: "."}
			for _, n := range []string{"a #b", "a: b"} {
				obj := &unstructured.Unstructured{}
				obj.SetAPIVersion("rbac.authorization.k8s.io/v1")
				obj.SetKind("ClusterRole")
				obj.SetName(n)
				p.Resources = append(p.Resources, obj)
			}
			return p
		},
		"generators and their files": func() *layout.ManifestLayout {
			p := &layout.ManifestLayout{Name: "p", Namespace: "."}
			for _, n := range yaml11Names {
				p.ExtraFiles = append(p.ExtraFiles, layout.ExtraFile{Name: n, Content: []byte("k: v\n")})
				p.ConfigMapGenerators = append(p.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: n, Files: []string{n}})
			}
			return p
		},
	}
	for name, tree := range trees {
		for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
			t.Run(name+"/"+writer, func(t *testing.T) {
				requireBuilds(t, writer, writtenFiles(t, writer, layout.Config{}, tree()))
			})
		}
	}
}
