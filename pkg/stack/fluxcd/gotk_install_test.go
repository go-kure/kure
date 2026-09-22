package fluxcd

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
)

// go-kure/kure#794: gotk-mode generation builds from the vendored flux2
// bundle, and every pin in this package matches go.mod.

// goModRequires reads the require directives of kure's go.mod (module path ->
// version) with a line scan, so the test adds no module dependency.
func goModRequires(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("open go.mod: %v", err)
	}
	defer f.Close()
	reqs := map[string]string{}
	inBlock := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		switch {
		case line == "require (":
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case strings.HasPrefix(line, "require "):
			line = strings.TrimPrefix(line, "require ")
		case !inBlock:
			continue
		}
		if fields := strings.Fields(line); len(fields) == 2 {
			reqs[fields[0]] = fields[1]
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	return reqs
}

func TestVendoredPinsMatchGoMod(t *testing.T) {
	reqs := goModRequires(t)
	for _, pin := range []struct{ module, constant string }{
		{"github.com/fluxcd/flux2/v2", GotkVersion},
		{"github.com/controlplaneio-fluxcd/flux-operator", FluxOperatorVersion},
	} {
		if got := reqs[pin.module]; got != pin.constant {
			t.Errorf("go.mod requires %s %s, but the vendored bundle is pinned to %s: re-vendor it (see the constant's doc comment)", pin.module, got, pin.constant)
		}
	}

	// Every controller image in the vendored gotk bundle must be the release
	// whose API module kure compiles against.
	dir, err := extractGotkManifests()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no manifests in the vendored bundle (err %v)", err)
	}
	image := regexp.MustCompile(`image: (?:[^/\s]+/)*fluxcd/([a-z-]+):(v[0-9][^\s"]*)`)
	checked := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range image.FindAllStringSubmatch(string(data), -1) {
			controller, tag := m[1], m[2]
			module := ""
			for mod := range reqs {
				if mod == "github.com/fluxcd/"+controller+"/api" || strings.HasPrefix(mod, "github.com/fluxcd/"+controller+"/api/v") {
					module = mod
				}
			}
			if module == "" {
				continue // no API module required for this controller
			}
			checked++
			if reqs[module] != tag {
				t.Errorf("vendored bundle runs %s:%s, but go.mod requires %s %s", controller, tag, module, reqs[module])
			}
		}
	}
	if checked < 5 {
		t.Errorf("only %d controller images matched a go.mod API module; the image pattern no longer matches the bundle", checked)
	}
}

func TestGotkManifestsBase(t *testing.T) {
	for _, v := range []string{"", GotkVersion, strings.TrimPrefix(GotkVersion, "v")} {
		base, cleanup, err := gotkManifestsBase(v)
		if err != nil {
			t.Fatalf("gotkManifestsBase(%q) error = %v", v, err)
		}
		// A non-empty base makes install.Generate skip its download
		// (flux2 pkg/manifestgen/install.Generate fetches only when the base
		// is empty), so this is what keeps the pinned path offline.
		if base == "" {
			t.Fatalf("gotkManifestsBase(%q) returned no base; install.Generate would download", v)
		}
		if _, err := os.Stat(filepath.Join(base, "source-controller.yaml")); err != nil {
			t.Errorf("gotkManifestsBase(%q): vendored bundle not extracted: %v", v, err)
		}
		cleanup()
		if _, err := os.Stat(base); !os.IsNotExist(err) {
			t.Errorf("cleanup left %s behind (err %v)", base, err)
		}
	}
	for _, v := range []string{"latest", "v2.0.0"} {
		base, cleanup, err := gotkManifestsBase(v)
		cleanup()
		if err != nil || base != "" {
			t.Errorf("gotkManifestsBase(%q) = %q, %v; want the upstream download (empty base)", v, base, err)
		}
	}
}

func TestGotkComponents_PinnedSpellingsAgree(t *testing.T) {
	// Unset, "vX.Y.Z" and "X.Y.Z" all select the vendored bundle and produce
	// identical objects, compared in full.
	bg := NewBootstrapGenerator()
	var want []client.Object
	for _, v := range []string{"", GotkVersion, strings.TrimPrefix(GotkVersion, "v")} {
		got, err := bg.generateGotkComponents(&stack.BootstrapConfig{FluxVersion: v})
		if err != nil {
			t.Fatalf("FluxVersion %q: %v", v, err)
		}
		if want == nil {
			want = got
			if len(want) == 0 {
				t.Fatal("no objects generated")
			}
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("FluxVersion %q produced different objects than FluxVersion %q", v, "")
		}
	}
}

func TestGotkManifestsBase_IndependentExtractions(t *testing.T) {
	// install.Generate writes option-dependent files into the base, so two
	// generations must never share one: each call gets its own directory.
	a, cleanA, err := gotkManifestsBase("")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanA()
	b, cleanB, err := gotkManifestsBase("")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanB()
	if a == b {
		t.Fatalf("two extractions share %s", a)
	}
}
