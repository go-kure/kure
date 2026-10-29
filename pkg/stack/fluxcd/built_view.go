package fluxcd

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"path"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/built"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// builtView reads the Flux Kustomizations of one layout tree, and what the
// apply of each writes, from what their builds make as kustomize-controller
// builds them (go-kure/kure#1094), where kustomizationTree reads them as the
// tree holds them. The reconcile-order checks judge it (orderGraph,
// checkPlacedReconcileOrder); the "umbrellas do not wait" rewrite and
// UmbrellaTerminals still read the tree as held.
//
// A writer is a Kustomization whose build is the tree's: the Flux bootstrap
// (key ""), which applies the root directory; every member whose spec.path is
// a directory of the tree; and every other one that some writer writes, on
// this cluster (no spec.kubeConfig), whose spec.path is a directory of the
// tree and whose spec.sourceRef names the Source of the writer that writes it
// (writer), as readsLocalBuild reads one. Each writer's result is what its
// apply writes:
//
//   - the bootstrap's, and that of a writer that transforms nothing
//     (transformsBuild) and substitutes nothing, is what its directories hold
//     as kustomize builds them (buildScope, builtItems);
//   - a writer with spec.patches, spec.components, a name prefix or suffix or
//     a target namespace is built as kustomize-controller builds it
//     (treeBuilds), its patches applied, its local config dropped and its
//     objects renamed and moved as the build does;
//   - a writer's spec.postBuild substitution runs over its result as the
//     controller runs it (substituteBuild).
//
// A writer is built from its spec as its own writer's result holds it, so a
// patched or substituted Kustomization builds as it is applied. A build that
// fails, and a substitution refused, refuse the tree: the controller's build
// fails the same way, and what the Kustomization applies cannot be read.
//
// Every Flux Kustomization a result holds is read as it is held there, under
// its identity as built. One two writers apply with different graph-relevant
// fields (graphFields) is refused: they keep overwriting each other. A copy
// an apply skips is neither compared nor read (applies). One no result holds
// is read as the tree holds it. An object a health check names is looked up
// in the results too (objectWriter).
type builtView struct {
	t *kustomizationTree
	// writers lists the writers' keys in the order they were found, "" first;
	// writerSpec holds the spec each was built from (nil for the bootstrap),
	// and results what its build made.
	writers    []string
	writerSpec map[string]*kustv1.Kustomization
	results    map[string][]substituted
	// occurrences lists, per key, each writer whose result holds the Flux
	// Kustomization with that key, and the object as it holds it.
	occurrences map[string][]occurrence
	// builtFrom holds, per writer other than the bootstrap, the copy its
	// build was run from.
	builtFrom map[string]builtCopy
	specs     map[string]*kustv1.Kustomization
	specErrs  map[string]error
	// sources are what the object index reads (objectIndex), on the first
	// health check on an object; objectsErr is the error of that read.
	sources     []objectSource
	objectsRead bool
	objectsErr  error
}

// occurrence is a Flux Kustomization as one writer's result holds it.
type occurrence struct {
	writer string
	obj    client.Object
}

// builtCopy is the copy a writer was built from: an occurrence, or, held,
// a member's spec as the tree holds it, where no result held it then.
type builtCopy struct {
	occurrence
	held bool
}

// newBuiltView builds every writer of the tree t indexes and reads their
// results. The builds run here, in a directory removed before it returns.
func newBuiltView(t *kustomizationTree) (*builtView, error) {
	v := &builtView{
		t:           t,
		writerSpec:  map[string]*kustv1.Kustomization{},
		results:     map[string][]substituted{},
		occurrences: map[string][]occurrence{},
		builtFrom:   map[string]builtCopy{},
		specs:       map[string]*kustv1.Kustomization{},
		specErrs:    map[string]error{},
	}
	builds := newTreeBuilds(t.root)
	defer builds.close()
	if err := v.find(builds); err != nil {
		return nil, err
	}
	if err := v.agree(); err != nil {
		return nil, err
	}
	return v, nil
}

// find runs the builds of every writer: the bootstrap's, then the members'
// in the order the results reach them (one no result reaches, from its spec
// as the tree holds it), then the other Kustomizations' whose build is the
// tree's, in the order the results reach them. A member is built from the
// first copy known whose apply does not skip it (applies), else the first;
// agree refuses the tree when what a member was built from, a skipped copy or
// its spec as the tree holds it, differs from the first copy that is applied.
// find refuses a member built from a copy on another cluster or Source
// (onThisTree), and one built from its spec as the tree holds it that no
// apply creates once every writer is known: nothing applies what it builds.
// A member's writer is a member or the bootstrap, so every member's result is
// known before another's locality is read (local).
func (v *builtView) find(builds *treeBuilds) error {
	if err := v.add(builds, "", nil); err != nil {
		return err
	}
	for i := 0; i < len(v.writers); i++ {
		for _, key := range v.heldKeys(v.writers[i]) {
			if _, done := v.writerSpec[key]; done || v.t.byKey[key] == nil {
				continue
			}
			occ := v.occurrences[key]
			from := occ[0]
			if j := slices.IndexFunc(occ, v.applies); j >= 0 {
				from = occ[j]
			}
			k, err := typedKustomization(from.obj)
			if err != nil {
				return err
			}
			if err := v.onThisTree(key, k, from.writer); err != nil {
				return err
			}
			v.builtFrom[key] = builtCopy{occurrence: from}
			if err := v.add(builds, key, k); err != nil {
				return err
			}
		}
	}
	for _, k := range v.t.kusts {
		key := crKey(k.Namespace, k.Name)
		// A member a result reached was built from that copy, also where
		// its spec.path is no directory of the tree and its build recorded
		// nothing: it is not built again from its spec as the tree holds it.
		if _, done := v.builtFrom[key]; !done {
			v.builtFrom[key] = builtCopy{occurrence: occurrence{writer: key, obj: k}, held: true}
			if err := v.add(builds, key, k); err != nil {
				return err
			}
		}
	}
	decided := map[string]bool{}
	for i := 0; i < len(v.writers); i++ {
		for _, key := range v.heldKeys(v.writers[i]) {
			if decided[key] || v.t.byKey[key] != nil {
				continue
			}
			// Decided once built: a writer found later may apply a copy of
			// one every copy known so far skips.
			if k := v.local(key); k != nil {
				decided[key] = true
				if err := v.add(builds, key, k); err != nil {
					return err
				}
			}
		}
	}
	for _, k := range v.t.kusts {
		key := crKey(k.Namespace, k.Name)
		if !v.builtFrom[key].held || slices.ContainsFunc(v.occurrences[key], func(o occurrence) bool { return o.writer != key && v.applies(o) }) {
			continue
		}
		return errors.Errorf("Flux Kustomization %q is created by no apply: no build holds a copy of it that its apply does not skip, so it never exists, and what it was built from, its spec as the tree holds it, is applied by nothing: let a build create it, or drop it from the tree", key)
	}
	return nil
}

// onThisTree refuses the member with key when k, the copy of it the apply of
// writer w makes, which it is built from, sets spec.kubeConfig or names
// another Source than its spec as the tree holds it: its build is then not
// the tree's, on this cluster, and what it applies cannot be read.
func (v *builtView) onThisTree(key string, k *kustv1.Kustomization, w string) error {
	held := v.t.byKey[key]
	switch {
	case k.Spec.KubeConfig != nil:
		return errors.Errorf("Flux Kustomization %q is applied by %s with spec.kubeConfig, so it applies on another cluster what another tree may hold, and what it applies cannot be read; let no build set it on a Kustomization of the tree", key, describeWriter(w))
	case sourceRefKey(k.Spec.SourceRef, k.Namespace) != sourceRefKey(held.Spec.SourceRef, held.Namespace):
		return errors.Errorf("Flux Kustomization %q is applied by %s with spec.sourceRef %s, not %s as the tree holds it, so it builds another Source's directory, and what it applies cannot be read; let no build change it on a Kustomization of the tree", key, describeWriter(w), sourceRefKey(k.Spec.SourceRef, k.Namespace), sourceRefKey(held.Spec.SourceRef, held.Namespace))
	}
	return nil
}

// local returns, for the Kustomization with key that is no member, the spec
// its writer's result holds when its build is the tree's: it is on this
// cluster, a writer other than the bootstrap writes it, and its
// spec.sourceRef names that writer's Source (sourceRefKey). nil otherwise,
// also when that spec cannot be read in typed form: its writer's apply then
// fails (go-kure/kure#1094 A4).
func (v *builtView) local(key string) *kustv1.Kustomization {
	w := v.writer(key, false)
	if w == "" {
		return nil
	}
	var k *kustv1.Kustomization
	var from occurrence
	for _, o := range v.occurrences[key] {
		if o.writer == w {
			k, _ = typedKustomization(o.obj)
			from = o
			break
		}
	}
	ws := v.writerSpec[w]
	if k == nil || k.Spec.KubeConfig != nil || sourceRefKey(k.Spec.SourceRef, k.Namespace) != sourceRefKey(ws.Spec.SourceRef, ws.Namespace) {
		return nil
	}
	v.builtFrom[key] = builtCopy{occurrence: from}
	return k
}

// add runs the build of the writer with key, from its spec k (nil for the
// bootstrap), and records its result. A Kustomization whose spec.path is no
// directory of the tree builds nothing of it, and is no writer.
func (v *builtView) add(builds *treeBuilds, key string, k *kustv1.Kustomization) error {
	var objs []client.Object
	var err error
	switch {
	case k == nil:
		objs, err = heldBuild(v.t.root)
	case v.t.layoutAt[path.Clean(k.Spec.Path)] == nil:
		return nil
	case transformsBuild(k):
		// A tree the writers refuse is no build's failure: nothing applies it.
		if _, err := builds.tree(); err != nil {
			return err
		}
		objs, err = builds.build(k)
	default:
		objs, err = heldBuild(v.t.layoutAt[path.Clean(k.Spec.Path)])
	}
	if err != nil {
		if k == nil {
			return err
		}
		var u unreadBuild
		if stderrors.As(err, &u) {
			return errors.Wrapf(u.error, "Flux Kustomization %q (spec.path %q) is not read, so what it applies cannot be read", key, k.Spec.Path)
		}
		return errors.Wrapf(err, "Flux Kustomization %q (spec.path %q) does not build as kustomize-controller builds it, so it never applies, and what it would apply cannot be read", key, k.Spec.Path)
	}
	res := make([]substituted, 0, len(objs))
	if k == nil {
		for _, obj := range objs {
			res = append(res, substituted{obj: obj})
		}
	} else if res, err = substituteBuild(k, objs); err != nil {
		return err
	}
	v.writers = append(v.writers, key)
	v.writerSpec[key] = k
	v.results[key] = res
	for _, r := range res {
		if _, ok := fluxKustomizationPath(r.obj); ok {
			held := crKey(r.obj.GetNamespace(), r.obj.GetName())
			v.occurrences[held] = append(v.occurrences[held], occurrence{writer: key, obj: r.obj})
		}
	}
	return nil
}

// heldBuild returns what the directories a kustomize build of l includes
// hold, as the build holds them (buildScope, builtItems), each resource that
// is no List read as the tree's writers write it to its file
// (writtenObject). The writers clean only the top-level object of a resource
// (kio.EncodeObjectsToYAML), so a List's items are written as they marshal,
// which is how builtItems reads them. A tree built as kustomize-controller
// builds it is read from the files layout.WriteToDisk writes (treeBuilds),
// so every object the view reads is in the one form Flux reads: a field the
// writers strip moves nothing at apply.
func heldBuild(l *layout.ManifestLayout) ([]client.Object, error) {
	var out []client.Object
	for _, b := range buildScope(l) {
		written := *b
		written.Resources = make([]client.Object, 0, len(b.Resources))
		for _, r := range b.Resources {
			if r == nil {
				continue
			}
			items, err := built.Objects(r)
			if err != nil {
				return nil, errors.Wrapf(err, "layout %q: read list items", b.FullRepoPath())
			}
			if len(items) == 1 && items[0].Object == runtime.Object(r) {
				if r, err = writtenObject(r); err != nil {
					return nil, errors.Wrapf(err, "layout %q", b.FullRepoPath())
				}
			}
			written.Resources = append(written.Resources, r)
		}
		objs, err := builtItems(&written)
		if err != nil {
			return nil, err
		}
		out = append(out, objs...)
	}
	return out, nil
}

// heldKeys lists, once each and in its result's order, the keys of the Flux
// Kustomizations the result of writer w holds.
func (v *builtView) heldKeys(w string) []string {
	var out []string
	for _, r := range v.results[w] {
		if _, ok := fluxKustomizationPath(r.obj); !ok {
			continue
		}
		if key := crKey(r.obj.GetNamespace(), r.obj.GetName()); !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	return out
}

// graphFields are the fields of a Flux Kustomization the reconcile-order
// checks read, or that decide what its build makes: images among them, which
// Flux's generator writes into the build (fluxcd/pkg/kustomize v1.44.0
// kustomize_generator.go:296-331), where an image transformer may rewrite any field a
// component's configuration names.
var graphFields = []string{
	"dependsOn", "healthChecks", "healthCheckExprs", "wait", "path", "sourceRef", "kubeConfig",
	"patches", "components", "ignoreMissingComponents", "namePrefix", "nameSuffix", "targetNamespace", "postBuild", "commonMetadata",
	"images",
}

// agree refuses a Flux Kustomization two writers apply with different
// graphFields: each applies its own, so the cluster holds whichever applied
// last, and neither is what the checks could judge. A copy its writer's apply
// skips (applies) overwrites nothing. A member built (find) from what no
// apply creates, such a copy or its spec as the tree holds it where no
// result held it then, builds what that says: it is refused too where that
// and the first applied copy have different graphFields.
func (v *builtView) agree() error {
	var keys []string
	for _, w := range v.writers {
		for _, key := range v.heldKeys(w) {
			if !slices.Contains(keys, key) {
				keys = append(keys, key)
			}
		}
	}
	for _, key := range keys {
		occ := slices.DeleteFunc(slices.Clone(v.occurrences[key]), func(o occurrence) bool { return !v.applies(o) })
		if from, ok := v.builtFrom[key]; ok && (from.held || !v.applies(from.occurrence)) && len(occ) > 0 {
			differ, err := differingFields(from.occurrence, occ[0])
			if err != nil {
				return err
			}
			if len(differ) > 0 {
				built := fmt.Sprintf("the copy %s writes, whose apply skips it", describeWriter(from.writer))
				if from.held {
					built = "its spec as the tree holds it, since no build had written it yet"
				}
				return errors.Errorf("Flux Kustomization %q is built from %s, and %s applies it with different %s: what its build applies cannot be read; let one build write it, or make both write the same spec",
					key, built, describeWriter(occ[0].writer), strings.Join(differ, ", "))
			}
		}
		if len(occ) < 2 {
			continue
		}
		for _, o := range occ[1:] {
			differ, err := differingFields(occ[0], o)
			if err != nil {
				return err
			}
			if len(differ) > 0 {
				return errors.Errorf("Flux Kustomization %q is written by %s and by %s with different %s: both apply it, and each overwrites what the other applied, so what it waits for cannot be read; let one build write it, or make both write the same spec",
					key, describeWriter(occ[0].writer), describeWriter(o.writer), strings.Join(differ, ", "))
			}
		}
	}
	return nil
}

// differingFields lists, as spec.<field>, the graphFields a and b's objects
// set differently.
func differingFields(a, b occurrence) ([]string, error) {
	first, err := specFields(a.obj)
	if err != nil {
		return nil, err
	}
	fields, err := specFields(b.obj)
	if err != nil {
		return nil, err
	}
	var differ []string
	for _, f := range graphFields {
		if first[f] != fields[f] {
			differ = append(differ, "spec."+f)
		}
	}
	return differ, nil
}

// specFields returns each of graphFields of obj's spec as JSON, "" for one
// it does not set, read from obj as written so that one that cannot be read
// in typed form is compared too.
func specFields(obj client.Object) (map[string]string, error) {
	content, err := writtenContent(obj)
	if err != nil {
		return nil, err
	}
	spec, _ := content["spec"].(map[string]any)
	out := map[string]string{}
	for _, f := range graphFields {
		value, ok := spec[f]
		if !ok {
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, errors.Wrapf(err, "read spec.%s of Flux Kustomization %q", f, crKey(obj.GetNamespace(), obj.GetName()))
		}
		out[f] = string(raw)
	}
	return out, nil
}

// describeWriter names writer w in a refusal.
func describeWriter(w string) string {
	if w == "" {
		return "the Flux bootstrap, which applies the root directory"
	}
	return fmt.Sprintf("the build of Flux Kustomization %q", w)
}

// isMember reports whether the Kustomization with key is a member.
func (v *builtView) isMember(key string) bool {
	return v.t.byKey[key] != nil
}

// judged reports whether checkTreeOrder refuses a cycle through the
// Kustomization with key: a member, or one a build makes under an identity
// the tree holds no Kustomization by (a name prefix or suffix, a target
// namespace, a patch or a substitution changed it). That one may be a member
// under its built identity, and the tree cannot tell whose it is. One every
// apply skips (applies) is never created, so no cycle runs through it.
func (v *builtView) judged(key string) bool {
	return v.isMember(key) || slices.ContainsFunc(v.occurrences[key], v.applies) && v.t.otherObjs[key] == nil
}

// spec returns the Flux Kustomization with key as the results hold it, the
// first copy an apply does not skip (applies); for a member every apply
// skips, the first copy, and for another nil: no apply creates it, so it
// waits for nothing. When no result holds it, it is read as the tree holds
// it; nil when neither holds it. One the results hold that cannot be read in
// typed form is an error for a member, and nil for another: its writer's
// apply fails, a stall and no cycle (go-kure/kure#1094 A4).
func (v *builtView) spec(key string) (*kustv1.Kustomization, error) {
	if k, ok := v.specs[key]; ok {
		return k, v.specErrs[key]
	}
	var k *kustv1.Kustomization
	var err error
	switch occ := v.occurrences[key]; {
	case len(occ) > 0:
		i := slices.IndexFunc(occ, v.applies)
		switch {
		case i >= 0:
			k, err = typedKustomization(occ[i].obj)
		case v.isMember(key):
			k, err = typedKustomization(occ[0].obj)
		}
		if err != nil && !v.isMember(key) {
			k, err = nil, nil
		}
	case v.isMember(key):
		k = v.t.byKey[key]
	default:
		k = v.t.otherSpecs[key]
	}
	v.specs[key], v.specErrs[key] = k, err
	return k, err
}

// members returns the members in the order the tree lists them, each as
// spec reads it.
func (v *builtView) members() ([]*kustv1.Kustomization, error) {
	out := make([]*kustv1.Kustomization, 0, len(v.t.kusts))
	for _, m := range v.t.kusts {
		k, err := v.spec(crKey(m.Namespace, m.Name))
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

// writer names the writer whose apply creates the Flux Kustomization with
// key, other than itself: "" when the bootstrap writes it, or no writer does;
// else the innermost member that does (the longest spec.path), else, unless
// membersOnly, the innermost other. A writer whose apply skips its copy
// (applies) does not write it. Another writer that also writes it is no
// alternate creator: which applies first depends on controller settings the
// tree does not carry.
func (v *builtView) writer(key string, membersOnly bool) string {
	best, bestLen, bestMember := "", -1, false
	for _, o := range v.occurrences[key] {
		if o.writer == key || !v.applies(o) {
			continue
		}
		if o.writer == "" {
			return ""
		}
		member := v.isMember(o.writer)
		if membersOnly && !member || bestMember && !member {
			continue
		}
		if n := len(v.writerSpec[o.writer].Spec.Path); member && !bestMember || n > bestLen {
			best, bestLen, bestMember = o.writer, n, member
		}
	}
	return best
}

// creator names the member whose apply creates the member with key, as
// checkReconcileOrder reads it: "" when the bootstrap does, or no member.
func (v *builtView) creator(key string) string {
	return v.writer(key, true)
}

// written lists every Flux Kustomization the apply of the Kustomization with
// key writes, other than itself: the members its result holds, in the order
// the tree lists them, then the others, in its result's order. One whose
// build is not the tree's writes nothing the tree holds.
func (v *builtView) written(key string) []string {
	if key == "" {
		return nil
	}
	var members, others []string
	for _, held := range v.heldKeys(key) {
		switch {
		case held == key:
		case v.isMember(held):
			members = append(members, held)
		default:
			others = append(others, held)
		}
	}
	slices.SortStableFunc(members, func(a, b string) int { return v.memberIndex(a) - v.memberIndex(b) })
	return append(members, others...)
}

// applied lists the members written lists.
func (v *builtView) applied(key string) []string {
	return slices.DeleteFunc(v.written(key), func(k string) bool { return !v.isMember(k) })
}

func (v *builtView) memberIndex(key string) int {
	return slices.IndexFunc(v.t.kusts, func(k *kustv1.Kustomization) bool { return crKey(k.Namespace, k.Name) == key })
}

// unskipped returns keys, Flux Kustomizations the apply of writer w writes,
// without those w's apply skips (excludedFromApply): kustomize-controller
// leaves a skipped object out of what its wait waits for
// (kustomization_controller.go:1089-1097). The mark is read on the object as
// w's result holds it, its patches and substitution applied, with w's
// spec.commonMetadata set over it, as Flux sets it before applying
// (:916-917). Only the wait reads this: what w writes (written) is unchanged,
// and so are the creation and existence edges onto a skipped object, which
// some other apply may still create unmarked (the bootstrap applies what it
// reaches without w's commonMetadata).
func (v *builtView) unskipped(w string, keys []string) []string {
	var out []string
	for _, key := range keys {
		o := occurrence{writer: w}
		if i := slices.IndexFunc(v.occurrences[key], func(o occurrence) bool { return o.writer == w }); i >= 0 {
			o = v.occurrences[key][i]
		}
		if v.applies(o) {
			out = append(out, key)
		}
	}
	return out
}

// applies reports whether the apply of o's writer writes o's object: it
// skips one whose mark (excludedFromApply), with the writer's
// spec.commonMetadata set over it, says so (fluxcd/pkg/ssa v0.83.0
// manager_apply.go:695-697). An occurrence with no object is applied.
func (v *builtView) applies(o occurrence) bool {
	var labels, annotations map[string]string
	if o.obj != nil {
		labels, annotations = o.obj.GetLabels(), o.obj.GetAnnotations()
	}
	if k := v.writerSpec[o.writer]; k != nil && k.Spec.CommonMetadata != nil {
		labels, annotations = setOver(labels, k.Spec.CommonMetadata.Labels), setOver(annotations, k.Spec.CommonMetadata.Annotations)
	}
	return !excludedFromApply(labels) && !excludedFromApply(annotations)
}

// specDeps returns k's spec.dependsOn as keys, an entry without namespace in
// k's.
func specDeps(k *kustv1.Kustomization) []string {
	var out []string
	for _, d := range k.Spec.DependsOn {
		ns := d.Namespace
		if ns == "" {
			ns = k.Namespace
		}
		out = append(out, crKey(ns, d.Name))
	}
	return out
}
