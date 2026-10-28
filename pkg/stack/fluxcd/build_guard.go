package fluxcd

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/go-kure/kure/pkg/errors"
)

// kustomizeAPIAudited is the sigs.k8s.io/kustomize/api version whose reads
// kustomizationFields and builtinConfigFields classify. The builtin plugin
// configs are internal to kustomize, so the walk reads them with structs of
// its own, which no test can compare with kustomize's; a version bump fails
// TestRemoteReference_FieldTablesMatchKustomize until the walk is audited
// again and this moves.
const kustomizeAPIAudited = "v0.21.1"

// buildGuard is the tree as a build reads it, but that it refuses a file
// kure cannot build as Flux does before kustomize reads its fields, and
// records the first in unread. kustomize reads a file it cannot read as one
// that is absent, so the build's own error does not name it; build reports
// unread instead. It refuses:
//
//   - a kustomization file that sets openapi: kustomize sets the global
//     OpenAPI schema from the field of every kustomization it loads
//     (krusty/kustomizer.go:79-88, internal/target/kusttarget.go:511-520);
//   - a file naming a remote reference in a field kustomize fetches through
//     (go-kure/kure#1168): an integration never reaches the network, and
//     kustomize clones a Git reference with the git on PATH, or requests a
//     URL, before any read of the tree (internal/loader/fileloader.go:293-332,
//     internal/target/kusttarget.go:418-441);
//   - a file it cannot check for remote references, which fails closed: a
//     kustomization or plugin config file that does not parse, a plugin that
//     is not builtin or a builtin it does not classify, and a directory of
//     plugin configs, which a kustomization builds and its transforms may
//     rewrite.
//
// A file's role comes from the field that names it, never from its content:
// a kustomization file by its name, as kustomize finds one
// (kusttarget.go:101-121), and a plugin config file by a generators,
// transformers or validators entry of the kustomization naming it, recorded
// in configs as the path kustomize reads, the entry joined to that
// kustomization's directory, or unchanged when absolute
// (fileloader.go:297-304). A data file named like a kustomization file is
// checked as one too, though kustomize reads it only as data. This holds as
// long as kustomize reads through no field the two tables leave out, which
// TestRemoteReference_FieldTablesMatchKustomize pins.
//
// It also records which reads load a kustomization (merged). kustomize loads
// a directory's kustomization by reading every kustomization file name in it
// and takes the one that exists (kusttarget.go:101-121); with no load
// restriction a missing name reaches ReadFile too
// (internal/loader/loadrestrictions.go:32-35). A generator reads its data
// file once.
type buildGuard struct {
	filesys.FileSystem
	tree    string
	rf      *resmap.Factory
	configs map[string]bool
	unread  error
	// read holds, per directory, the kustomization file names a read asked
	// for in it, and found those that exist; merges lists each file found
	// that may merge into a Secret (kustomization).
	read, found map[string]map[string]bool
	merges      []string
}

func newBuildGuard(fsys filesys.FileSystem, tree string) *buildGuard {
	return &buildGuard{
		FileSystem: fsys,
		tree:       tree,
		rf:         resmap.NewFactory(provider.NewDepProvider().GetResourceFactory()),
		configs:    map[string]bool{},
	}
}

func (g *buildGuard) ReadFile(name string) ([]byte, error) {
	b, err := g.FileSystem.ReadFile(name)
	isKustomization := slices.Contains(konfig.RecognizedKustomizationFileNames(), filepath.Base(name))
	if isKustomization {
		g.read = markRead(g.read, name)
	}
	if err != nil {
		return b, err
	}
	var why string
	if isKustomization {
		g.found = markRead(g.found, name)
		why = g.kustomization(name, b)
	}
	if why == "" && g.configs[filepath.Clean(name)] {
		why = g.pluginConfigs(b)
	}
	if why == "" {
		return b, nil
	}
	if g.unread == nil {
		g.unread = errors.Errorf("its build reads %s, %s", g.rel(name), why)
	}
	return nil, errors.Errorf("%s is not read: %s", name, why)
}

// rel is name as the tree holds it.
func (g *buildGuard) rel(name string) string {
	if rel, err := filepath.Rel(g.tree, name); err == nil {
		return filepath.ToSlash(rel)
	}
	return name
}

// kustomization checks the kustomization file name, parsed as kustomize
// parses it (types/kustomization.go:340-357), and records the plugin config
// files it names. A file it passes is recorded in merges when it may merge
// into a Secret: it holds a secretGenerator entry with behavior merge, or a
// generators entry, whose generator may merge too. The parse is kustomize's
// own (YAML to JSON, then encoding/json, which matches keys in any case), so
// it reads the values kustomize reads (aliases, merge keys and the last of
// duplicate keys included), and kustomize compares the behavior exactly
// (types/generationbehavior.go:35-46), as here. A file that does not parse is
// refused, so no merge in it is missed.
func (g *buildGuard) kustomization(name string, b []byte) string {
	var k kusttypes.Kustomization
	if err := k.Unmarshal(b); err != nil {
		return fmt.Sprintf("which kure cannot parse as a kustomization file to check it for remote references (%v)", err)
	}
	if len(k.OpenAPI) > 0 {
		return "which sets openapi: kustomize would keep that schema for every later build in the process, and kure, which does not reset it around each build as kustomize-controller does, cannot build it as Flux does; drop the field"
	}
	dir := filepath.Dir(name)
	for _, field := range slices.Sorted(maps.Keys(kustomizationFields)) {
		f := kustomizationFields[field]
		if f.values == nil {
			continue
		}
		for _, v := range f.values(&k) {
			if why := g.entry(dir, field, f.reads, v); why != "" {
				return why
			}
		}
	}
	merges := slices.ContainsFunc(k.SecretGenerator, func(gen kusttypes.SecretArgs) bool {
		return gen.Behavior == kusttypes.BehaviorMerge.String()
	})
	if merges || len(k.Generators) > 0 {
		g.merges = append(g.merges, name)
	}
	return ""
}

// merged returns the first kustomization file the build loaded as one, a
// component's included, that may merge into a Secret (kustomization): a file
// in a directory where every kustomization file name was read and exactly one
// exists. A file only a generator reads as data, or one no build reads,
// merges nothing. A directory holding two kustomization files is no
// kustomization a build can load (kusttarget.go:118-120), so files read
// there are data.
func (g *buildGuard) merged() (string, bool) {
	for _, file := range g.merges {
		if g.loaded(file) {
			return file, true
		}
	}
	return "", false
}

// loaded reports whether the build loaded file, which it read, as a
// kustomization (merged).
func (g *buildGuard) loaded(file string) bool {
	dir := filepath.Dir(file)
	return len(g.read[dir]) == len(konfig.RecognizedKustomizationFileNames()) && len(g.found[dir]) == 1
}

// markRead adds name's base to the names m holds for its directory.
func markRead(m map[string]map[string]bool, name string) map[string]map[string]bool {
	if m == nil {
		m = map[string]map[string]bool{}
	}
	dir := filepath.Dir(name)
	if m[dir] == nil {
		m[dir] = map[string]bool{}
	}
	m[dir][filepath.Base(name)] = true
	return m
}

// entry checks one value of a kustomization field kustomize reads as reads.
func (g *buildGuard) entry(dir, field string, reads fieldReads, v string) string {
	switch reads {
	case noReads:
	case loads:
		if isRemoteFile(v) {
			return remote(field, v)
		}
	case accumulates:
		if isRemoteFile(v) || isRepoURL(v) {
			return remote(field, v)
		}
	case pluginEntries:
		// An entry kustomize's resource factory reads is an inline config
		// (kusttarget.go:300-311,347-357); any other is a path it
		// accumulates.
		if rm, err := g.rf.NewResMapFromBytes([]byte(v)); err == nil {
			return g.configsIn(rm)
		}
		if isRemoteFile(v) || isRepoURL(v) {
			return remote(field, v)
		}
		p := v
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		p = filepath.Clean(p)
		if g.IsDir(p) {
			return fmt.Sprintf("whose %s entry %s is a directory: kure does not check the plugin configs a kustomization builds for remote references", field, g.rel(p))
		}
		g.configs[p] = true
	}
	return ""
}

// pluginConfigs checks a plugin config file.
func (g *buildGuard) pluginConfigs(b []byte) string {
	rm, err := g.rf.NewResMapFromBytes(b)
	if err != nil {
		return fmt.Sprintf("a plugin config file kure cannot parse to check it for remote references (%v)", err)
	}
	return g.configsIn(rm)
}

// configsIn checks plugin configs, which kustomize loads as builtins when
// their group is empty and version builtin (internal/plugins/loader/
// loader.go:178-182) and refuses otherwise under Flux's options
// (:204-209).
func (g *buildGuard) configsIn(rm resmap.ResMap) string {
	for _, r := range rm.Resources() {
		gvk := r.GetGvk()
		if gvk.Group != "" || gvk.Version != konfig.BuiltinPluginApiVersion {
			return fmt.Sprintf("which configures plugin %s %s, which is not builtin: Flux builds with builtin plugins only and refuses it too, and kure does not check it for remote references", r.GetApiVersion(), r.GetKind())
		}
		fields, ok := builtinConfigFields[gvk.Kind]
		if !ok {
			return fmt.Sprintf("which configures builtin plugin %s, whose fields kure does not check for remote references", gvk.Kind)
		}
		if fields == nil {
			continue
		}
		j, err := r.MarshalJSON()
		if err != nil {
			return fmt.Sprintf("which configures builtin plugin %s, which kure cannot read to check it for remote references (%v)", gvk.Kind, err)
		}
		refs, err := fields(j)
		if err != nil {
			return fmt.Sprintf("which configures builtin plugin %s, which kure cannot read to check it for remote references (%v)", gvk.Kind, err)
		}
		for _, ref := range refs {
			if isRemoteFile(ref.value) {
				return fmt.Sprintf("which configures builtin plugin %s, %s", gvk.Kind, remote(ref.field, ref.value))
			}
		}
	}
	return ""
}

func remote(field, ref string) string {
	return fmt.Sprintf("whose %s names the remote reference %s: kustomize would fetch it, and an integration never reaches the network", field, ref)
}

// isRemoteFile is kustomize's loader.IsRemoteFile (internal/loader/
// fileloader.go:23-26): the URLs its Load requests over HTTP instead of
// reading the tree (:318-332).
func isRemoteFile(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// isRepoURL is whether kustomize parses s as a Git repository, which its
// loader clones when it accumulates s as a directory
// (internal/loader/fileloader.go:157-165), through the exported wrapper of
// the same parser (resource/origin.go:51-61). A panic is read as a
// repository, failing closed. A local path shaped like a repository URL is
// read as one too, though kustomize reads a well-formed file of that name
// first (kusttarget.go:422).
func isRepoURL(s string) (repo bool) {
	defer func() {
		if recover() != nil {
			repo = true
		}
	}()
	return (&resource.Origin{}).Append(s).Repo != ""
}

// fieldReads is how kustomize reads the values of a kustomization field.
type fieldReads int

const (
	// noReads: kustomize reads no file through the field.
	noReads fieldReads = iota
	// loads: each value is a file kustomize Loads, requesting an http(s) URL.
	loads
	// accumulates: each value is a file or a directory, which kustomize
	// may also clone as a Git repository (kusttarget.go:418-441).
	accumulates
	// pluginEntries: each value is an inline plugin config or one
	// kustomize accumulates.
	pluginEntries
)

type kustomizationField struct {
	reads  fieldReads
	values func(*kusttypes.Kustomization) []string
}

// kustomizationFields is every field of kustomize's Kustomization by its
// JSON name, with how kustomize reads files through it. openapi is checked
// apart, and the helm fields reach no read: kustomize-controller disables
// helm (fluxcd/pkg/kustomize v1.44.0 kustomize_generator.go:698-699), so
// the generator's Config refuses before any Load
// (internal/builtins/HelmChartInflationGenerator.go:46-52).
var kustomizationFields = map[string]kustomizationField{
	"apiVersion": {}, "kind": {}, "metadata": {}, "openapi": {},
	"namePrefix": {}, "nameSuffix": {}, "namespace": {},
	"commonLabels": {}, "labels": {}, "commonAnnotations": {},
	"images": {}, "imageTags": {}, "replicas": {}, "vars": {},
	"sortOptions": {}, "generatorOptions": {}, "buildMetadata": {},
	"helmGlobals": {}, "helmCharts": {}, "helmChartInflationGenerator": {},

	"resources":  {accumulates, func(k *kusttypes.Kustomization) []string { return k.Resources }},
	"bases":      {accumulates, func(k *kusttypes.Kustomization) []string { return k.Bases }}, //nolint:staticcheck // SA1019: kustomize still reads it, folded into resources
	"components": {accumulates, func(k *kusttypes.Kustomization) []string { return k.Components }},

	"generators":   {pluginEntries, func(k *kusttypes.Kustomization) []string { return k.Generators }},
	"transformers": {pluginEntries, func(k *kusttypes.Kustomization) []string { return k.Transformers }},
	"validators":   {pluginEntries, func(k *kusttypes.Kustomization) []string { return k.Validators }},

	// internal/accumulator/loadconfigfromcrds.go:36
	"crds": {loads, func(k *kusttypes.Kustomization) []string { return k.Crds }},
	// internal/plugins/builtinconfig/loaddefaultconfig.go:17
	"configurations": {loads, func(k *kusttypes.Kustomization) []string { return k.Configurations }},
	// internal/builtins/PatchTransformer.go:46
	"patches": {loads, func(k *kusttypes.Kustomization) []string { return patchPaths(k.Patches) }},
	// internal/builtins/PatchJson6902Transformer.go:42
	"patchesJson6902": {loads, func(k *kusttypes.Kustomization) []string { return patchPaths(k.PatchesJson6902) }}, //nolint:staticcheck // SA1019: kustomize still reads it
	// internal/builtins/PatchStrategicMergeTransformer.go:53-64: an entry
	// that is not patch content is a path.
	"patchesStrategicMerge": {loads, func(k *kusttypes.Kustomization) []string {
		return strategicMergePaths(k.PatchesStrategicMerge) //nolint:staticcheck // SA1019: kustomize still reads it
	}},
	// internal/builtins/ReplacementTransformer.go:33
	"replacements": {loads, func(k *kusttypes.Kustomization) []string {
		var paths []string
		for _, r := range k.Replacements {
			paths = append(paths, r.Path)
		}
		return paths
	}},
	// kv/kv.go:76-95, after FixKustomization folds env into envs
	// (types/kustomization.go:244-256).
	"configMapGenerator": {loads, func(k *kusttypes.Kustomization) []string {
		var paths []string
		for _, g := range k.ConfigMapGenerator {
			paths = append(paths, generatorPaths(g.KvPairSources)...)
		}
		return paths
	}},
	"secretGenerator": {loads, func(k *kusttypes.Kustomization) []string {
		var paths []string
		for _, g := range k.SecretGenerator {
			paths = append(paths, generatorPaths(g.KvPairSources)...)
		}
		return paths
	}},
}

func patchPaths(patches []kusttypes.Patch) []string {
	var paths []string
	for _, p := range patches {
		paths = append(paths, p.Path)
	}
	return paths
}

func strategicMergePaths(patches []kusttypes.PatchStrategicMerge) []string {
	var paths []string
	for _, p := range patches {
		paths = append(paths, string(p))
	}
	return paths
}

func generatorPaths(s kusttypes.KvPairSources) []string {
	paths := append(fileSourcePaths(s.FileSources), s.EnvSources...)
	if s.EnvSource != "" {
		paths = append(paths, s.EnvSource)
	}
	return paths
}

// fileSourcePaths are the paths kustomize Loads for generator file sources,
// split as generators.ParseFileSource splits them
// (internal/generators/utils.go:109-124); a source it refuses is read by
// nothing.
func fileSourcePaths(sources []string) []string {
	var paths []string
	for _, s := range sources {
		switch n := strings.Count(s, "="); {
		case n == 0:
			paths = append(paths, s)
		case n == 1 && !strings.HasPrefix(s, "=") && !strings.HasSuffix(s, "="):
			paths = append(paths, s[strings.Index(s, "=")+1:])
		}
	}
	return paths
}

type loadRef struct{ field, value string }

// builtinConfigFields is every builtin plugin kustomize registers
// (krusty.GetBuiltinPluginNames, internal/plugins/builtinhelpers/
// builtins.go:61-115) by kind, with the values its config names that
// kustomize Loads; nil where it Loads none (HelmChartInflationGenerator
// reaches none, as above).
var builtinConfigFields = map[string]func([]byte) ([]loadRef, error){
	"AnnotationsTransformer": nil, "HashTransformer": nil, "IAMPolicyGenerator": nil,
	"ImageTagTransformer": nil, "LabelTransformer": nil, "NamespaceTransformer": nil,
	"PrefixSuffixTransformer": nil, "PrefixTransformer": nil, "SuffixTransformer": nil,
	"ReplicaCountTransformer": nil, "HelmChartInflationGenerator": nil,

	// internal/builtins/PatchTransformer.go:25,46
	"PatchTransformer": configRefs(func(c struct {
		Path string `json:"path"`
	}) []loadRef {
		return []loadRef{{"path", c.Path}}
	}),
	// internal/builtins/PatchJson6902Transformer.go:21,42
	"PatchJson6902Transformer": configRefs(func(c struct {
		Path string `json:"path"`
	}) []loadRef {
		return []loadRef{{"path", c.Path}}
	}),
	// internal/builtins/PatchStrategicMergeTransformer.go:15,53-64
	"PatchStrategicMergeTransformer": configRefs(func(c struct {
		Paths []kusttypes.PatchStrategicMerge `json:"paths"`
	}) []loadRef {
		return refsOf("paths", strategicMergePaths(c.Paths))
	}),
	// internal/builtins/ReplacementTransformer.go:16,33
	"ReplacementTransformer": configRefs(func(c struct {
		Replacements []kusttypes.ReplacementField `json:"replacements"`
	}) []loadRef {
		var refs []loadRef
		for _, r := range c.Replacements {
			refs = append(refs, loadRef{"replacements path", r.Path})
		}
		return refs
	}),
	// internal/builtins/ValueAddTransformer.go:29,58-59
	"ValueAddTransformer": configRefs(func(c struct {
		TargetFilePath string `json:"targetFilePath"`
	}) []loadRef {
		return []loadRef{{"targetFilePath", c.TargetFilePath}}
	}),
	// internal/builtins/ConfigMapGenerator.go:17-32 and SecretGenerator.go:
	// 17-32 pass the config to kv.Loader.Load, which reads envs and files
	// but not env (kv/kv.go:39-61): env is folded into envs only for a
	// kustomization's generators.
	"ConfigMapGenerator": configRefs(generatorConfigRefs),
	"SecretGenerator":    configRefs(generatorConfigRefs),
}

func generatorConfigRefs(c struct {
	Files []string `json:"files"`
	Envs  []string `json:"envs"`
}) []loadRef {
	return append(refsOf("files", fileSourcePaths(c.Files)), refsOf("envs", c.Envs)...)
}

func refsOf(field string, values []string) []loadRef {
	refs := make([]loadRef, 0, len(values))
	for _, v := range values {
		refs = append(refs, loadRef{field, v})
	}
	return refs
}

// configRefs reads a builtin config's JSON as T and returns the values get
// names.
func configRefs[T any](get func(T) []loadRef) func([]byte) ([]loadRef, error) {
	return func(b []byte) ([]loadRef, error) {
		var c T
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, errors.Wrap(err, "read the plugin config")
		}
		return get(c), nil
	}
}
