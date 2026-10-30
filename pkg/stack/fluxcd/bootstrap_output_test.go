package fluxcd_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// bootstrapOutputConfigs are the configs whose whole bootstrap output is
// stored under testdata/bootstrap-output, one per mode and source kind. They
// set no source access option: the files were written before those options
// existed, and a config that does not set them must keep producing them byte
// for byte. The *-labels configs set BootstrapConfig.Labels and nothing else
// new (go-kure/kure#1180); every other one leaves it unset.
func bootstrapOutputConfigs() map[string]*stack.BootstrapConfig {
	config := func(mode, kind, url, ref string) *stack.BootstrapConfig {
		return &stack.BootstrapConfig{
			Enabled:     true,
			FluxMode:    mode,
			FluxVersion: fluxstack.GotkVersion,
			Registry:    "ghcr.io/fluxcd",
			SourceKind:  kind,
			SourceURL:   url,
			SourceRef:   ref,
		}
	}
	const oci, git = "oci://registry.example.com/fleet", "https://git.example.com/fleet.git"
	named := config(fluxstack.DefaultFluxMode, "OCIRepository", oci, "v1.2.3")
	named.SyncName = "fleet"
	gotkNamed := config(fluxstack.ModeGotk, "OCIRepository", oci, "v1.2.3")
	gotkNamed.SyncName = "fleet"
	labelled := func(c *stack.BootstrapConfig) *stack.BootstrapConfig {
		c.Labels = bootstrapLabelsInput()
		return c
	}
	return map[string]*stack.BootstrapConfig{
		"gotk-oci-labels":          labelled(config(fluxstack.ModeGotk, "OCIRepository", oci, "v1.2.3")),
		"gotk-git-labels":          labelled(config(fluxstack.ModeGotk, "GitRepository", git, "main")),
		"flux-operator-oci-labels": labelled(config(fluxstack.DefaultFluxMode, "OCIRepository", oci, "v1.2.3")),
		"flux-operator-git-labels": labelled(config(fluxstack.DefaultFluxMode, "GitRepository", git, "main")),

		"gotk-oci":                config(fluxstack.ModeGotk, "OCIRepository", oci, "v1.2.3"),
		"gotk-oci-named":          gotkNamed,
		"gotk-oci-default-ref":    config(fluxstack.ModeGotk, "", oci, ""),
		"gotk-git":                config(fluxstack.ModeGotk, "GitRepository", git, "main"),
		"flux-operator-oci":       config(fluxstack.DefaultFluxMode, "OCIRepository", oci, "v1.2.3"),
		"flux-operator-oci-named": named,
		"flux-operator-git":       config(fluxstack.DefaultFluxMode, "GitRepository", git, "main"),
		"flux-operator-no-source": config(fluxstack.DefaultFluxMode, "", "", ""),
		"gotk-no-source":          config(fluxstack.ModeGotk, "", "", ""),
	}
}

// TestBootstrap_OutputUnchanged renders each config through GenerateBootstrap
// and compares the result with the stored file.
func TestBootstrap_OutputUnchanged(t *testing.T) {
	for name, config := range bootstrapOutputConfigs() {
		t.Run(name, func(t *testing.T) {
			objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(
				config, &stack.Node{Name: "prod"}, layout.LayoutRules{ClusterName: "demo"})
			if err != nil {
				t.Fatalf("GenerateBootstrap: %v", err)
			}
			compareWithStored(t, filepath.Join("testdata", "bootstrap-output", name+".txt"), bootstrapBytes(t, objs))
		})
	}
}

// bootstrapBytes returns objs as one text. The objects of the install bundle,
// the vendored Flux or flux-operator release, are summarised by a count and a
// digest of their encoding, so the stored file stays readable and still
// changes if they do; the objects the generator builds itself follow in full,
// in output order.
//
// An object is part of the install bundle when it is neither a Flux source,
// a Flux Kustomization nor a FluxInstance.
func bootstrapBytes(t *testing.T, objs []client.Object) []byte {
	t.Helper()
	encode := func(objs []client.Object) []byte {
		ptrs := make([]*client.Object, len(objs))
		for i := range objs {
			ptrs[i] = &objs[i]
		}
		data, err := kureio.EncodeObjectsToYAMLWithOptions(ptrs, kureio.EncodeOptions{KubernetesFieldOrder: true})
		if err != nil {
			t.Fatalf("encoding the bootstrap output: %v", err)
		}
		return data
	}
	var install, built []client.Object
	for _, obj := range objs {
		switch obj.GetObjectKind().GroupVersionKind().Kind {
		case "OCIRepository", "GitRepository", "Kustomization", "FluxInstance":
			built = append(built, obj)
		default:
			install = append(install, obj)
		}
	}
	var out bytes.Buffer
	kinds := map[string]int{}
	for _, obj := range install {
		kinds[obj.GetObjectKind().GroupVersionKind().Kind]++
	}
	names := make([]string, 0, len(kinds))
	for k := range kinds {
		names = append(names, k)
	}
	sort.Strings(names)
	fmt.Fprintf(&out, "=== install bundle: %d objects, sha256 %x ===\n", len(install), sha256.Sum256(encode(install)))
	for _, k := range names {
		fmt.Fprintf(&out, "%s: %d\n", k, kinds[k])
	}
	fmt.Fprintf(&out, "=== generated: %d objects ===\n", len(built))
	if len(built) > 0 {
		out.Write(encode(built))
	}
	return out.Bytes()
}
