package layout_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// yaml11Names are names kustomize's YAML reader takes for something other than
// a string when written plain: a YAML 1.1 bool, null or number
// (go-kure/kure#896).
var yaml11Names = []string{"y", "yes", "on", "no", "off", "true", "null", "1", "1e3", "0x1f"}

// TestWriters_QuoteEntriesKustomizeReadsAsOtherTypes: a child directory, a
// ConfigMapGenerator or a generator file named like a YAML bool, null or number
// is written so that kustomize reads it as a string, and every writer's output
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
