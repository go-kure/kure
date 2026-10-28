package fluxcd

import (
	"path"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/internal/built"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// objectSource is one apply that may write objects of the tree, as the
// object index reads it (objectIndex): a writer's result, or, for a
// Kustomization on this cluster whose build is not the tree's but whose
// spec.path is a directory of it, what that directory holds.
type objectSource struct {
	// key is the writer's or the Kustomization's key, "" for the Flux
	// bootstrap; writer is false for one whose build is not the tree's.
	key    string
	writer bool
	// spec is the Kustomization the apply is built from, nil for the
	// bootstrap.
	spec    *kustv1.Kustomization
	objects []builtObject
	// heldScope lists the layouts of an apply read as the tree holds them
	// (heldBuild): the bootstrap, a writer whose build transforms nothing,
	// and every Kustomization whose build is not the tree's. Their
	// configMapGenerator entries are applied under a content hash the read
	// does not compute. A writer built as kustomize-controller builds it
	// (treeBuilds) has none: its result holds what its generators make.
	heldScope []*layout.ManifestLayout
	// unreadable marks a Kustomization that cannot be read in typed form,
	// so what its apply writes is not known.
	unreadable bool
}

// builtObject is one object of a source, under the identity it is applied
// with.
type builtObject struct {
	group, kind, namespace, name string
	// unresolved marks an identity that may become another value at apply:
	// one written with "$" or "ENC[" (unresolved), or one a substitution
	// reads from the cluster (substituted.unknownIdentity).
	unresolved bool
	// labels and annotations are as applied, but for the skip keys under a
	// substituteFrom (substituted.writtenLabels, writtenAnnotations), which
	// they hold as written.
	labels, annotations map[string]string
	// varies marks an object whose text as written, before any substitution,
	// holds "$" or "ENC[" (writtenVaries): its apply may change its marks.
	varies bool
}

// objectIndex reads, once, what every source of the tree applies.
func (v *builtView) objectIndex() error {
	if v.objectsRead {
		return v.objectsErr
	}
	v.objectsRead = true
	v.objectsErr = v.readObjects()
	return v.objectsErr
}

func (v *builtView) readObjects() error {
	for _, w := range v.writers {
		k := v.writerSpec[w]
		s := objectSource{key: w, writer: true, spec: k}
		switch {
		case k == nil:
			s.heldScope = buildScope(v.t.root)
		case !transformsBuild(k):
			s.heldScope = buildScope(v.t.layoutAt[path.Clean(k.Spec.Path)])
		}
		for _, r := range v.results[w] {
			o, err := readBuiltObject(r)
			if err != nil {
				return errors.Wrapf(err, "%s: read an object as applied", describeWriter(w))
			}
			s.objects = append(s.objects, o)
		}
		v.sources = append(v.sources, s)
	}
	var keys []string
	seen := map[string]bool{}
	for _, w := range v.writers {
		for _, key := range v.heldKeys(w) {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	for _, key := range v.t.otherKeys {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		if _, ok := v.writerSpec[key]; ok || v.isMember(key) {
			continue
		}
		k, created := v.nonWriterSpec(key)
		if !created {
			continue
		}
		if k == nil {
			v.sources = append(v.sources, objectSource{key: key, unreadable: true})
			continue
		}
		l := v.t.layoutAt[path.Clean(k.Spec.Path)]
		if k.Spec.KubeConfig != nil || l == nil {
			continue
		}
		objs, err := heldBuild(l)
		if err != nil {
			return err
		}
		s := objectSource{key: key, spec: k, heldScope: buildScope(l)}
		for _, obj := range objs {
			o, err := readBuiltObject(substituted{obj: obj})
			if err != nil {
				return errors.Wrapf(err, "layout %q: read an object as written", l.FullRepoPath())
			}
			s.objects = append(s.objects, o)
		}
		v.sources = append(v.sources, s)
	}
	return nil
}

// nonWriterSpec returns the Kustomization with key, which is no writer, in
// typed form, as the first copy the results hold that an apply does not skip
// (applies) holds it, else, where no result holds it, as the tree holds it;
// nil when it cannot be read so. created is false when the results hold only
// copies their applies skip: no apply creates it, and it applies nothing.
func (v *builtView) nonWriterSpec(key string) (k *kustv1.Kustomization, created bool) {
	if occ := v.occurrences[key]; len(occ) > 0 {
		i := slices.IndexFunc(occ, v.applies)
		if i < 0 {
			return nil, false
		}
		k, err := typedKustomization(occ[i].obj)
		if err != nil {
			return nil, true
		}
		return k, true
	}
	return v.t.otherSpecs[key], true
}

func readBuiltObject(r substituted) (builtObject, error) {
	kind, apiVersion, err := built.WrittenType(r.obj)
	if err != nil {
		return builtObject{}, err
	}
	obj := r.obj
	labels, annotations := r.writtenLabels.over(obj.GetLabels()), r.writtenAnnotations.over(obj.GetAnnotations())
	// The object as read is as written unless substituteBuild substituted
	// it, which records what it was written as (substituted.varies).
	content, err := writtenContent(obj)
	if err != nil {
		return builtObject{}, err
	}
	varies, err := writtenVaries(content)
	if err != nil {
		return builtObject{}, err
	}
	return builtObject{
		group: apiGroup(apiVersion), kind: kind, namespace: obj.GetNamespace(), name: obj.GetName(),
		unresolved: r.unknownIdentity || unresolved(apiVersion) || unresolved(kind) || unresolved(obj.GetNamespace()) || unresolved(obj.GetName()),
		labels:     labels, annotations: annotations,
		varies: r.varies || varies,
	}, nil
}

// excludes reports whether s's apply skips o whatever the cluster holds
// (excludedFromApply), with s's spec.commonMetadata set over o's labels and
// annotations, so never creates it (fluxcd/pkg/ssa v0.83.0
// manager_apply.go:695-697). It reads that only when the apply cannot change
// it: o as written holds no "$" or "ENC[" (builtObject.varies), and nor does
// that commonMetadata, which a Kustomization read as the tree holds it may
// carry unsubstituted.
func (s objectSource) excludes(o builtObject) bool {
	if o.varies {
		return false
	}
	labels, annotations := o.labels, o.annotations
	if s.spec != nil && s.spec.Spec.CommonMetadata != nil {
		cm := s.spec.Spec.CommonMetadata
		if mapVaries(cm.Labels) || mapVaries(cm.Annotations) {
			return false
		}
		labels, annotations = setOver(labels, cm.Labels), setOver(annotations, cm.Annotations)
	}
	return excludedFromApply(labels) || excludedFromApply(annotations)
}

// mapVaries reports whether a key or a value of m holds "$" or "ENC["
// (unresolved).
func mapVaries(m map[string]string) bool {
	for k, v := range m {
		if unresolved(k) || unresolved(v) {
			return true
		}
	}
	return false
}

// objectWriters names the writers whose applies write the object a health
// check on apiVersion, kind, namespace and name names, sorted, only when
// they are certain: nil otherwise, and the check adds no edge. The check
// waits for whichever of them creates the object first, so one writer is an
// edge and several are alternatives (orderGraph.next); a writer the set
// misses would be one more way for the check to be met, and an edge or an
// alternative that may not exist would refuse a tree that converges. It
// matches by group, kind, namespace and name as applied, read from each
// writer's result (builtView): patched, renamed, moved and substituted as
// its build makes it, the version ignored as Flux resolves it; an empty
// apiVersion is apps/v1, as Flux reads it (kustomize-controller v1.10.1
// internal/inventory/inventory.go:166-169). They are certain when only
// writers other than the Flux bootstrap apply a match, and each match's
// labels and annotations, with its writer's spec.commonMetadata set over
// them, do not let the apply skip it (skipsApply; a skip a substitution reads
// from the cluster is read as written, builtObject). A match its apply
// excludes (objectSource.excludes) is never created and counts for no
// apply. A match written with "$" or "ENC[" anywhere, or under a
// commonMetadata that holds one, is never read as excluded, as its apply may
// change its marks; and an IfNotPresent one leaves the writers uncertain.
//
// The writers stay uncertain when, a match its apply excludes aside:
//   - the check names no namespace, or a match is applied with none: a
//     namespaced object may be applied in another namespace than its build
//     says, and a cluster-scoped one may exist before any apply;
//   - the bootstrap, or a Kustomization whose build is not the tree's, may
//     apply a match too (objectSource), or one that cannot be read may;
//   - a Kustomization whose build is not the tree's transforms it
//     (mayTransformInto);
//   - an apply other than the writers' holds an object whose identity may
//     change at apply (builtObject.unresolved), whatever it names, as it may
//     take the checked identity;
//   - a configMapGenerator entry an apply read as held runs (heldScope) has a
//     name the check's starts with, as kustomize appends a content hash, or
//     one that may change at apply;
//   - Kubernetes creates the object itself (kubernetesCreated).
//
// An object a controller creates, from a HelmRelease's chart, say, or a build
// that is no directory of the tree, is not modelled: the check assumes an
// object the tree writes is created only by the applies of the tree's
// directories.
func (v *builtView) objectWriters(apiVersion, kind, namespace, name string) ([]string, error) {
	if err := v.objectIndex(); err != nil {
		return nil, err
	}
	if apiVersion == "" {
		apiVersion = "apps/v1"
	}
	group := apiGroup(apiVersion)
	if namespace == "" || kubernetesCreated(group, kind, name) || v.generatorMayName(group, kind, name) {
		return nil, nil
	}
	var writers []string
	for _, s := range v.sources {
		if s.unreadable || !s.writer && s.mayTransformInto(group, kind) {
			return nil, nil
		}
		for _, o := range s.objects {
			if o.unresolved || o.group != group || o.kind != kind || o.name != name {
				continue
			}
			// A copy its apply excludes is never created: it is no match,
			// wherever it is applied, and competes with no writer.
			if s.excludes(o) {
				continue
			}
			if o.namespace == "" {
				return nil, nil
			}
			if o.namespace != namespace {
				continue
			}
			if !s.writer || s.key == "" {
				return nil, nil
			}
			labels, annotations := o.labels, o.annotations
			if cm := s.spec.Spec.CommonMetadata; cm != nil {
				labels, annotations = setOver(labels, cm.Labels), setOver(annotations, cm.Annotations)
			}
			if skipsApply(labels) || skipsApply(annotations) {
				return nil, nil
			}
			if !slices.Contains(writers, s.key) {
				writers = append(writers, s.key)
			}
		}
	}
	if len(writers) == 0 {
		return nil, nil
	}
	for _, s := range v.sources {
		if s.writer && slices.Contains(writers, s.key) {
			continue
		}
		for _, o := range s.objects {
			if o.unresolved {
				return nil, nil
			}
		}
	}
	slices.Sort(writers)
	return writers, nil
}

// generatorMayName reports whether a generator whose output the object
// index does not read as applied may make the object of group, kind and
// name: a configMapGenerator entry of a layout an apply reads as held
// (objectSource.heldScope), whose name the object's is or starts with, or
// which may change at apply. No Secret is one: a build that merges into a
// Secret is not read (treeBuilds.build), and a build read as the tree holds
// it (heldBuild) merges none, as the kustomization.yaml kure writes lists
// only the layout's resource files and child directories, and an extra file
// takes no kustomization file name of its layout's directory.
func (v *builtView) generatorMayName(group, kind, name string) bool {
	if group != "" || kind != "ConfigMap" {
		return false
	}
	for _, s := range v.sources {
		for _, l := range s.heldScope {
			for _, gen := range l.ConfigMapGenerators {
				if name == gen.Name || strings.HasPrefix(name, gen.Name+"-") || unresolved(gen.Name) {
					return true
				}
			}
		}
	}
	return false
}

// mayTransformInto reports whether s, a Kustomization whose build is not the
// tree's, may apply an object of group and kind under an identity the
// directory it names does not show: with spec.components, which may add any
// object; with spec.patches, when that directory holds any object or
// configMapGenerator entry, local config included, as a patch may change a
// kind or keep what the build drops (patchesBuild); otherwise, with a name
// prefix or suffix or a target namespace, when it holds an object of group
// and kind, or, for a ConfigMap, a configMapGenerator entry.
func (s objectSource) mayTransformInto(group, kind string) bool {
	k := s.spec
	if k == nil || !transformsBuild(k) {
		return false
	}
	if len(k.Spec.Components) > 0 {
		return true
	}
	if len(k.Spec.Patches) > 0 {
		for _, l := range s.heldScope {
			if len(l.Resources) > 0 || len(l.ConfigMapGenerators) > 0 {
				return true
			}
		}
		return false
	}
	for _, o := range s.objects {
		if o.group == group && o.kind == kind {
			return true
		}
	}
	if group == "" && kind == "ConfigMap" {
		for _, l := range s.heldScope {
			if len(l.ConfigMapGenerators) > 0 {
				return true
			}
		}
	}
	return false
}
