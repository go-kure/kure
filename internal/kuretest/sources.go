package kuretest

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"golang.org/x/tools/go/packages"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/go-kure/kure/internal/crds"
	"github.com/go-kure/kure/internal/crdvalidate"
	"github.com/go-kure/kure/internal/gotk"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// source says where the definitions for one module's kinds come from, or
// why there are none. Exactly one field is set; TestEveryKindHasOneSource
// holds the table to that and to the registry.
type source struct {
	// Dir is the directory the module ships its definitions in, relative to
	// the module root. One canonical directory per module: a module that
	// keeps copies elsewhere (release history, a dev environment, the Flux
	// definitions flux-operator embeds) is read in one place only, so a kind
	// can never be defined twice.
	Dir string
	// Bundle marks a Flux toolkit module. The API modules ship no manifests;
	// their definitions come from the flux2 install bundle in internal/gotk,
	// which is the release the kinds table pins them at.
	Bundle bool
	// Uncovered says why no definition exists for the module's kinds. Such a
	// kind gets ObjectMeta validation only.
	Uncovered string
}

// builtin is the reason for the apiserver's own kinds.
const builtin = "built-in kind: the apiserver validates it in Go, there is no schema to hold it to"

// modules is the table of every module the kinds registry draws on.
var modules = map[string]source{
	"github.com/backube/volsync":                        {Dir: "config/crd/bases"},
	"github.com/cert-manager/cert-manager":              {Dir: "deploy/crds"},
	"github.com/cilium/cilium":                          {Dir: "pkg/k8s/apis/cilium.io/client/crds/v2"},
	"github.com/cloudnative-pg/cloudnative-pg":          {Dir: "config/crd/bases"},
	"github.com/cloudnative-pg/plugin-barman-cloud":     {Dir: "config/crd/bases"},
	"github.com/controlplaneio-fluxcd/flux-operator":    {Dir: "config/crd/bases"},
	"go.universe.tf/metallb":                            {Dir: "config/crd/bases"},
	"sigs.k8s.io/gateway-api":                           {Dir: "config/crd/standard"},
	"github.com/fluxcd/helm-controller/api":             {Bundle: true},
	"github.com/fluxcd/image-automation-controller/api": {Bundle: true},
	"github.com/fluxcd/kustomize-controller/api":        {Bundle: true},
	"github.com/fluxcd/notification-controller/api":     {Bundle: true},
	"github.com/fluxcd/source-controller/api":           {Bundle: true},
	"github.com/fluxcd/source-watcher/api/v2":           {Bundle: true},
	"k8s.io/api":                     {Uncovered: builtin},
	"k8s.io/apiextensions-apiserver": {Uncovered: builtin},
	"github.com/external-secrets/external-secrets/apis":                      {Uncovered: "the API module ships no definitions; they are generated into the operator's deploy tree"},
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring": {Uncovered: "the API module ships no definitions; they are generated into the operator's bundle"},
}

var (
	buildOnce sync.Once
	built     *crdvalidate.Validator
	buildErr  error
)

// validator builds the pinned validator once per test binary.
func validator() (*crdvalidate.Validator, error) {
	buildOnce.Do(func() { built, buildErr = build() })
	return built, buildErr
}

func build() (*crdvalidate.Validator, error) {
	dirs, err := moduleDirs()
	if err != nil {
		return nil, err
	}
	var defs []*apiextensionsv1.CustomResourceDefinition
	for _, module := range sortedModules() {
		src := modules[module]
		if src.Dir == "" {
			continue
		}
		dir := filepath.Join(dirs[module], src.Dir)
		found, err := crds.Definitions(dir)
		if err != nil {
			return nil, errors.Wrapf(err, "kuretest: %s", module)
		}
		if len(found) == 0 {
			return nil, errors.Errorf("kuretest: %s ships no definitions under %s", module, dir)
		}
		for _, d := range found {
			defs = append(defs, d.CRD)
		}
	}
	bundled, err := crds.Archive(gotk.Open())
	if err != nil {
		return nil, errors.Wrap(err, "kuretest: the Flux bundle")
	}
	for _, d := range bundled {
		defs = append(defs, d.CRD)
	}
	v, err := crdvalidate.New(defs)
	if err != nil {
		return nil, errors.Wrap(err, "kuretest")
	}
	return v, nil
}

// moduleDirs resolves the directory of every module in the registry, through
// the same go list the generator uses, and holds each to the version the
// registry was generated from: the definitions read must be the ones the
// compiled types came from.
func moduleDirs() (map[string]string, error) {
	byPath := map[string]kubernetes.KindInfo{}
	var paths []string
	for _, k := range kubernetes.Kinds() {
		if _, seen := byPath[k.ImportPath]; seen {
			continue
		}
		byPath[k.ImportPath] = k
		paths = append(paths, k.ImportPath)
	}
	sort.Strings(paths)
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedModule}, paths...)
	if err != nil {
		return nil, errors.Wrap(err, "kuretest: list the pinned modules")
	}
	dirs := map[string]string{}
	for _, p := range pkgs {
		for _, e := range p.Errors {
			return nil, errors.Errorf("kuretest: %s: %s", p.PkgPath, e.Msg)
		}
		k, ok := byPath[p.PkgPath]
		if !ok {
			return nil, errors.Errorf("kuretest: go list returned %s, which no registered kind imports", p.PkgPath)
		}
		m := p.Module
		if m == nil || m.Dir == "" {
			return nil, errors.Errorf("kuretest: %s: no module directory; run the tests from the module root with GOWORK=off", p.PkgPath)
		}
		if m.Path != k.Module || m.Version != k.ModuleVersion {
			return nil, errors.Errorf("kuretest: %s resolves to %s %s, but the kinds table was generated from %s %s; regenerate it", p.PkgPath, m.Path, m.Version, k.Module, k.ModuleVersion)
		}
		if prev, dup := dirs[m.Path]; dup && prev != m.Dir {
			return nil, errors.Errorf("kuretest: %s resolves to both %s and %s", m.Path, prev, m.Dir)
		}
		dirs[m.Path] = m.Dir
	}
	if len(dirs) != len(sortedModules()) {
		return nil, fmt.Errorf("kuretest: resolved %d modules, the table names %d", len(dirs), len(sortedModules()))
	}
	return dirs, nil
}

func sortedModules() []string {
	out := make([]string, 0, len(modules))
	for m := range modules {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
