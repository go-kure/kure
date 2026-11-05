package fluxcd

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// The class of a dependsOn entry as a built Flux Kustomization holds it
// (specDeps): what the entry makes its Kustomization wait for.
const (
	// classExist waits only for the dependency to exist: an entry whose
	// readyExpr is exactly "true" under AdditiveCELDependencyCheckOff.
	classExist = 0
	// classExpr carries a readyExpr: kure evaluates no CEL, and with the
	// controller's additive CEL gate off an expression replaces its own Ready
	// check, so the entry proves no Ready.
	classExpr = 1
	// classPlain has no readyExpr: it waits for the dependency's Ready.
	classPlain = 2
)

// classSet is a set of dependsOn classes. A Kustomization may hold several
// entries on one dependency, so the class of "its entry on T" is a set.
type classSet uint8

func (s classSet) has(c int) bool     { return s&(1<<c) != 0 }
func (s classSet) only(c int) bool    { return s == 1<<c }
func (s classSet) atLeast(c int) bool { return s>>c != 0 }

// classOf is the class of the entry d.
func classOf(d depRef) int {
	switch {
	case d.plain:
		return classPlain
	case d.exist:
		return classExist
	}
	return classExpr
}

// holds reports whether s satisfies an obligation of class need: class 2 asks
// for a plain entry, class 1 for any entry that waits for Ready or an
// expression, class 0 for any entry.
func (s classSet) holds(need int) bool {
	switch need {
	case classPlain:
		return s.has(classPlain)
	case classExpr:
		return s.atLeast(classExpr)
	}
	return s != 0
}

// viewSnapshot is what one builtView says, as plain values: nothing in it
// points into the tree or the view. The rewrite mutates the typed objects the
// layout holds, which a later build reads and a view's specs may alias, so a
// snapshot is taken before it and compared with one taken after.
type viewSnapshot struct {
	rescue bool
	// ids is every Flux Kustomization identity the view holds: the members,
	// the others the tree holds, and every identity a build wrote. The
	// obligation and field checks read it, so a held namesake that no build
	// makes is still judged.
	ids map[string]bool
	// built is the identities the view builds: the members and every identity
	// a build wrote, not the others the tree only holds. checkSameIdentities
	// reads it: a namesake held in a directory no writer builds says nothing of
	// which identities the builds make, and must not stand in for one that a
	// patch moved (go-kure/kure#1243).
	built map[string]bool
	// classes holds, per identity and per dependency, the classes of the
	// identity's entries on that dependency.
	classes map[string]map[string]classSet
	// fields holds, per identity, specFields of its spec without dependsOn.
	fields map[string]map[string]string
	// spec holds, per identity, specJSON of its spec: the whole spec, dependsOn
	// included. copiesAnUmbrella compares it.
	spec map[string]string
	// held holds, per identity, specJSON of its spec as the tree holds it: the
	// Kustomization as the layout writes it, before any patch of a build is
	// applied to it, where spec is the one the view shows (a patch of an
	// ancestor's build has been applied). A copy a build makes from the
	// directory that holds an umbrella has the held one, not the applied one
	// (go-kure/kure#1272). copiesAnUmbrella compares it.
	held map[string]string
}

// depSnapshot is the snapshot of every view of one tree
// (newBuiltViews), in the order it returns them.
type depSnapshot struct {
	views []*viewSnapshot
}

// newDepSnapshot builds the tree t indexes and snapshots each view. Its error
// is any refusal newBuiltViews gives, or an unreadable member.
func newDepSnapshot(t *kustomizationTree, existenceOnly bool) (*depSnapshot, error) {
	views, err := newBuiltViews(t, existenceOnly)
	if err != nil {
		return nil, err
	}
	s := &depSnapshot{}
	for _, v := range views {
		if err := v.unbuiltPatcher(); err != nil {
			return nil, err
		}
		vs, err := snapshotView(v)
		if err != nil {
			return nil, err
		}
		s.views = append(s.views, vs)
	}
	return s, nil
}

// unbuiltPatcher refuses a view in which a Flux Kustomization the tree holds
// as a non-member, that patches what it builds, builds a directory of the
// tree from a Source a member names, and is not built there: the Flux
// bootstrap may apply it, so its patches may land on what the view shows, and
// the view does not apply them. Such a view says nothing of the entries it
// shows plain. One on another Source, or on another cluster, builds what the
// tree does not hold.
//
// Every copy the tree holds under the identity is judged, not the one
// otherSpecs keeps: the tree may hold a patching copy and a plain one of a
// key in two directories, and which of them otherSpecs keeps depends on the
// order the tree lists them in (go-kure/kure#1243).
//
// The copies the tree holds are not all a build makes: a local-config
// Kustomization that a patch keeps (builtItems) is in no copy list, only in
// the result of the writer that keeps it. The occurrences of the view, those
// the apply of their writer writes, are judged the same way.
func (v *builtView) unbuiltPatcher() error {
	sources := map[string]bool{}
	for _, m := range v.t.kusts {
		sources[sourceRefKey(m.Spec.SourceRef, m.Namespace)] = true
	}
	patchesTree := func(k *kustv1.Kustomization) bool {
		return patchesBuild(k) && k.Spec.KubeConfig == nil && v.t.directoryAt(k.Spec.Path) != nil && sources[sourceRefKey(k.Spec.SourceRef, k.Namespace)]
	}
	for _, key := range sortedKeys(v.t.otherSpecs) {
		if _, built := v.writerSpec[key]; built {
			continue
		}
		for _, k := range v.t.copies[key] {
			if patchesTree(k) {
				return errors.Errorf("Flux Kustomization %q patches what it builds, a directory of the tree, and the build of the tree does not read it as a writer", key)
			}
		}
	}
	for _, key := range sortedKeys(v.occurrences) {
		if _, built := v.writerSpec[key]; built {
			continue
		}
		for _, o := range v.occurrences[key] {
			if o.obj == nil || o.writer == key || !v.applies(o) {
				continue
			}
			if k, err := typedKustomization(o.obj); err == nil && patchesTree(k) {
				return errors.Errorf("Flux Kustomization %q on Source %q patches what it builds, a directory of the tree, and the build of the tree does not read it as a writer", key, sourceRefKey(k.Spec.SourceRef, k.Namespace))
			}
		}
	}
	return nil
}

// snapshotView snapshots v.
func snapshotView(v *builtView) (*viewSnapshot, error) {
	s := &viewSnapshot{
		rescue:  v.rescue,
		ids:     map[string]bool{},
		built:   map[string]bool{},
		classes: map[string]map[string]classSet{},
		fields:  map[string]map[string]string{},
		spec:    map[string]string{},
		held:    map[string]string{},
	}
	for _, k := range v.t.kusts {
		s.ids[crKey(k.Namespace, k.Name)] = true
		s.built[crKey(k.Namespace, k.Name)] = true
	}
	for _, key := range v.t.otherKeys {
		s.ids[key] = true
	}
	for key := range v.occurrences {
		s.ids[key] = true
		s.built[key] = true
	}
	for id := range s.ids {
		k, err := v.spec(id)
		if err != nil {
			return nil, err
		}
		if k == nil {
			continue
		}
		byT := map[string]classSet{}
		for _, d := range specDeps(k, v.existenceOnly) {
			byT[d.key] |= 1 << classOf(d)
		}
		s.classes[id] = byT
		fields, err := specFields(k)
		if err != nil {
			return nil, err
		}
		delete(fields, "dependsOn")
		s.fields[id] = fields
		if s.spec[id], err = specJSON(k); err != nil {
			return nil, err
		}
		held := v.t.otherSpecs[id]
		if v.isMember(id) {
			held = v.t.byKey[id]
		}
		if held == nil {
			continue
		}
		if s.held[id], err = specJSON(held); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// proves reports whether, in every view, owner's entries on target are all
// plain: owner's Ready follows target's. An owner a view does not hold, or
// holds without an entry on target, proves nothing. A nil snapshot proves
// nothing.
func (s *depSnapshot) proves(owner, target string) bool {
	return s.every(owner, target, classPlain)
}

// existence reports whether, in every view, owner's entries on target are all
// existence-only.
func (s *depSnapshot) existence(owner, target string) bool {
	return s.every(owner, target, classExist)
}

// every reports whether, in each view, the classes of owner's entries on
// target are exactly {c}. Views are combined here, and only here: a mixed set
// ({0, 2}) or a view that differs is neither. Obligations are never combined
// (checkAgainstPre).
func (s *depSnapshot) every(owner, target string, c int) bool {
	if s == nil || len(s.views) == 0 {
		return false
	}
	for _, v := range s.views {
		if !v.classes[owner][target].only(c) {
			return false
		}
	}
	return true
}

// rewriteFacts is what the rewrite of umbrellasDoNotWait recorded for the
// checks that judge its result.
type rewriteFacts struct {
	// terminals holds the terminal leaves of every umbrella of the tree.
	terminals map[string][]string
	// held holds, per member of the tree, the classes of every dependsOn entry
	// it held before the rewrite, on each dependency: what the fallback
	// (checkWrittenEntries) asks the built tree to keep.
	held map[string]map[string]classSet
}

// checkRewriteResult judges the rewritten tree by the views built from it
// (views), against pre, the snapshot taken before the rewrite, or, without one
// (the pre-rewrite build failed), against the tree as it holds it.
//
// The pass may change the meaning of a tree in one way: a wait on an umbrella
// becomes a wait on its terminal leaves. So in each view every identity waits
// for at least what it waited for before, and an umbrella has no wait of its
// own left (go-kure/kure#1226, go-kure/kure#1221).
func (t *kustomizationTree) checkRewriteResult(pre *depSnapshot, views []*builtView, f rewriteFacts) error {
	if pre != nil && len(views) != len(pre.views) {
		return refuseRewrite("the tree builds as %d view(s) before the rewrite and %d after it: what its Kustomizations wait for cannot be compared", len(pre.views), len(views))
	}
	for i, v := range views {
		q, err := snapshotView(v)
		if err != nil {
			return err
		}
		var p *viewSnapshot
		if pre != nil {
			if p = pre.views[i]; p.rescue != q.rescue {
				return refuseRewrite("the views of the tree before and after the rewrite differ in which Kustomizations they read as writers: what its Kustomizations wait for cannot be compared")
			}
			if err := checkSameIdentities(p, q); err != nil {
				return err
			}
		}
		if err := t.checkNoWaitOnUmbrella(q, f); err != nil {
			return err
		}
		if p != nil {
			err = t.checkObligations(p, q, f)
		} else {
			err = t.checkWrittenEntries(v, q, f)
		}
		if err != nil {
			return err
		}
		if err := t.checkBuiltUmbrellas(v, q); err != nil {
			return err
		}
		if err := t.checkReadinessFields(v, p, q, f); err != nil {
			return err
		}
	}
	return nil
}

func refuseRewrite(format string, args ...any) error {
	return errors.Wrapf(ErrUmbrellaDependency, "UmbrellasDoNotWait: "+format, args...)
}

// checkNoWaitOnUmbrella is U1: no identity of the view holds an entry that
// waits for Ready on an umbrella of the tree. The rewrite leaves only
// existence-only entries there; one that waits for Ready is satisfied once the
// umbrella applies, before its leaves are Ready, and is written by something
// the rewrite did not see (a patch).
func (t *kustomizationTree) checkNoWaitOnUmbrella(q *viewSnapshot, f rewriteFacts) error {
	for _, x := range sortedKeys(q.classes) {
		for _, target := range sortedKeys(q.classes[x]) {
			terms, umbrella := f.terminals[target]
			if umbrella && q.classes[x][target].atLeast(classExpr) {
				return refuseRewrite("Flux Kustomization %q depends on %q, which applies Flux Kustomizations, after the rewrite: a dependsOn that something other than the tree's own spec adds (a patch, say) is not rewritten, and %q does not wait, so %q would not wait for what %q applies; name its terminal leaves %q instead", x, target, target, x, target, terms)
			}
		}
	}
	return nil
}

// checkBuiltUmbrellas is U3: an identity the view builds that is no member and
// whose apply writes Flux Kustomizations (a member renamed by a name prefix,
// say) is an umbrella the pass never saw, so nothing was rewritten for a wait
// on it. A wait for Ready on it is refused. The one exception is an object the
// tree holds as another Kustomization, an application's own, a leaf by design,
// whose applied copy has the graphFields it was held with.
func (t *kustomizationTree) checkBuiltUmbrellas(v *builtView, q *viewSnapshot) error {
	for _, x := range sortedKeys(q.ids) {
		if v.isMember(x) || len(v.written(x)) == 0 {
			continue
		}
		var by string
		for _, y := range sortedKeys(q.classes) {
			if q.classes[y][x].atLeast(classExpr) {
				by = y
				break
			}
		}
		if by == "" {
			continue
		}
		stands, err := t.heldStandsFor(v, x, false)
		if err != nil {
			return err
		}
		if stands {
			continue
		}
		return refuseRewrite("Flux Kustomization %q depends on %q, which a build makes and which applies Flux Kustomizations: it is an umbrella the tree does not hold (a renamed copy, say), so its terminal leaves are not known and the rewrite cannot name them; it would become Ready before what it applies", by, x)
	}
	return nil
}

// heldStandsFor reports whether the tree holds x as another Kustomization (an
// application's own, a leaf by design) whose applied copy in v has the
// graphFields it was held with, or has no applied copy at all: the copy is the
// object the tree holds, not a Kustomization a build made under its name.
//
// fallback is set where the tree could not be built before the rewrite
// (checkWrittenEntries): no earlier build says which object the copy came from,
// so a held object with the copy's graphFields may be a clone of the copy that
// a build made, and a patch may have taken a wait from the copy
// (go-kure/kure#1247). There the held object stands only if no build that may
// have changed what its directory holds writes it: every occurrence of x is
// written by the Flux bootstrap or by a Kustomization that transforms nothing
// (transformsBuild) and substitutes nothing (spec.postBuild). A held object
// whose applied copy a transforming Kustomization writes is refused, though the
// controller applies the held object; the refusal is the price of not guessing,
// and a comparison of the two on the graphFields cannot replace it, because a
// patch may set a graphField that no object of the tree has.
func (t *kustomizationTree) heldStandsFor(v *builtView, x string, fallback bool) (bool, error) {
	held := t.otherObjs[x]
	if held == nil {
		return false, nil
	}
	applied, err := v.spec(x)
	if err != nil || applied == nil {
		return err == nil, err
	}
	differ, err := differingFields(occurrence{obj: held}, occurrence{obj: applied})
	if err != nil || len(differ) > 0 || !fallback {
		return len(differ) == 0, err
	}
	return v.writtenByPlainWritersOnly(x), nil
}

// writtenByPlainWritersOnly reports whether x has an occurrence in v and every
// one is written by the Flux bootstrap or by a Kustomization whose build
// changes nothing it reads: no patch, component, name prefix or suffix, target
// namespace (transformsBuild) or postBuild.
func (v *builtView) writtenByPlainWritersOnly(x string) bool {
	occ := v.occurrences[x]
	return len(occ) > 0 && !slices.ContainsFunc(occ, func(o occurrence) bool {
		if o.writer == "" {
			return false
		}
		k := v.writerSpec[o.writer]
		return k == nil || transformsBuild(k) || k.Spec.PostBuild != nil
	})
}

// checkSameIdentities refuses a view q that builds other identities than p,
// the same view before the rewrite. A patch may copy a dependsOn entry into
// the metadata (a JSON6902 copy of /spec/dependsOn/0/name into /metadata/name),
// so what a rewritten list holds can change which identities the build makes.
func checkSameIdentities(p, q *viewSnapshot) error {
	for _, id := range sortedKeys(p.built) {
		if !q.built[id] {
			return refuseRewrite("the build of the tree makes Flux Kustomization %q before the rewrite but not after it: the rewritten dependsOn changes what a patch does, and what the Kustomizations wait for cannot be compared", id)
		}
	}
	for _, id := range sortedKeys(q.built) {
		if !p.built[id] {
			return refuseRewrite("the build of the tree makes Flux Kustomization %q after the rewrite but not before it: the rewritten dependsOn changes what a patch does, and what the Kustomizations wait for cannot be compared", id)
		}
	}
	return nil
}

// checkObligations is U2: what each identity of p waited for before the
// rewrite, q waits for as well, each umbrella replaced by its terminal leaves.
// A dependency the view does not build is kept too: it may exist outside the
// tree, and a patch that drops the entry drops a real wait.
func (t *kustomizationTree) checkObligations(p, q *viewSnapshot, f rewriteFacts) error {
	for _, x := range sortedKeys(p.classes) {
		for _, target := range sortedKeys(p.classes[x]) {
			ps := p.classes[x][target]
			terms, umbrella := f.terminals[target]
			if !umbrella {
				if need := obligation(ps, false); !q.classes[x][target].holds(need) {
					return refuseRewrite("Flux Kustomization %q waits for %q before the rewrite and not after it (class %d required, %s held): a patch landing on the rewritten dependsOn drops it", x, target, need, describeClasses(q.classes[x][target]))
				}
				continue
			}
			need := obligation(ps, true)
			if need == classExist {
				// Kept as it is, or replaced by waits on the leaves.
				if q.classes[x][target] != 0 || allHold(q.classes[x], terms, classExist) {
					continue
				}
				return refuseRewrite("Flux Kustomization %q depends on %q, which applies Flux Kustomizations, before the rewrite and on neither it nor its terminal leaves %q after it: a patch landing on the rewritten dependsOn drops it", x, target, terms)
			}
			for _, term := range terms {
				if !q.classes[x][term].holds(need) {
					return refuseRewrite("Flux Kustomization %q depends on %q, which applies Flux Kustomizations, before the rewrite and does not wait for its terminal leaf %q after it (class %d required, %s held): a patch landing on the rewritten dependsOn drops it, and %q would become Ready before %q", x, target, term, need, describeClasses(q.classes[x][term]), x, term)
				}
			}
		}
	}
	return nil
}

// checkReadinessFields is U4: the rewrite changes dependsOn, and wait and
// healthChecks of an umbrella. Anything else that differs after it was moved by
// a patch that read dependsOn (a copy of an entry into a health check, say).
// The fields before are those of p, or, without one, the tree's own spec.
func (t *kustomizationTree) checkReadinessFields(v *builtView, p, q *viewSnapshot, f rewriteFacts) error {
	before := map[string]map[string]string{}
	if p != nil {
		before = p.fields
	} else {
		for _, k := range t.kusts {
			held, err := specFields(k)
			if err != nil {
				return err
			}
			delete(held, "dependsOn")
			before[crKey(k.Namespace, k.Name)] = held
		}
	}
	for _, x := range sortedKeys(before) {
		_, treeUmbrella := f.terminals[x]
		// A build may make a copy of an umbrella, with the wait the pass
		// cleared on the umbrella it copies; that wait may go, and no other
		// field may differ. A copy is a non-member that applies Flux
		// Kustomizations and, before the rewrite, had the whole spec of an
		// umbrella of the tree, applied or as the tree holds it
		// (go-kure/kure#1247, go-kure/kure#1272): a patch may rename it,
		// and may not give it another path, nor a wait, a health check or
		// a dependency of its own, nor change any other field of its spec.
		// A Kustomization the tree holds as a leaf that applies Flux
		// Kustomizations keeps its own.
		var specs, held map[string]string
		if p != nil {
			specs, held = p.spec, p.held
		}
		copied := !v.isMember(x) && len(v.written(x)) > 0 && copiesAnUmbrella(specs, held, x, f.terminals)
		if err := checkFields(x, before[x], q.fields[x], treeUmbrella, copied); err != nil {
			return err
		}
	}
	return nil
}

// obligation is the class q must hold where p held the classes ps: 2 when a
// plain entry was there (each entry of a mixed set is judged on its own); else
// 1 when an expression was, which kure cannot compare with another, and where
// the entry names an umbrella the rewrite wrote plain entries in its place, so
// 2; else 0.
func obligation(ps classSet, umbrella bool) int {
	switch {
	case ps.has(classPlain):
		return classPlain
	case ps.has(classExpr):
		if umbrella {
			return classPlain
		}
		return classExpr
	}
	return classExist
}

// allHold reports whether by holds, on each key of keys, a set satisfying need.
func allHold(by map[string]classSet, keys []string, need int) bool {
	for _, k := range keys {
		if !by[k].holds(need) {
			return false
		}
	}
	return len(keys) > 0
}

// copiesAnUmbrella reports whether x, in specs (the whole spec of each identity
// before the rewrite, specJSON), has the spec of an umbrella of the tree. A copy
// of an umbrella made by a build has the umbrella's spec as it was before the
// rewrite, wait, health checks and dependencies included, so the ones it loses
// are the ones it inherited; a Kustomization with a field of its own does not.
// That spec is the umbrella's applied one (specs), or the one the tree holds it
// with (held): a patch of an ancestor's build changes the first and not the copy
// a build makes from the directory that holds the umbrella (go-kure/kure#1272).
// The specs are compared as the typed Kustomization encodes them, with the keys
// in the order encoding/json gives them: a duration written 60s or 1m, and an
// empty or an absent list, compare equal, and only a value the typed form keeps
// can tell two specs apart. A Kustomization with the whole spec of an umbrella
// is a duplicate of it, and loses what the umbrella loses. Without a recorded
// umbrella or x, it is not a copy: a Kustomization that cannot be compared is
// judged as one the tree keeps.
func copiesAnUmbrella(specs, held map[string]string, x string, umbrellas map[string][]string) bool {
	mine, ok := specs[x]
	if !ok {
		return false
	}
	for u := range umbrellas {
		if u == x {
			continue
		}
		if theirs, ok := specs[u]; ok && mine == theirs {
			return true
		}
		if theirs, ok := held[u]; ok && mine == theirs {
			return true
		}
	}
	return false
}

// checkFields is U4 for identity x: before and after are its fields without
// dependsOn, before and after the rewrite. An umbrella of the tree has no wait
// and no health checks after it, whatever it had before; so may a copy of one
// a build makes (copied), where they are no more than they were. The other
// fields are as they were.
func checkFields(x string, before, after map[string]string, umbrella, copied bool) error {
	for _, field := range graphFields {
		if field == "dependsOn" {
			continue
		}
		if field == "wait" || field == "healthChecks" {
			switch {
			case umbrella && after[field] != "":
				return refuseRewrite("Flux Kustomization %q applies Flux Kustomizations and must not wait, and a build gives it spec.%s: a patch sets it; under UmbrellasDoNotWait an umbrella waits through dependsOn on its terminal leaves only", x, field)
			case umbrella || copied && after[field] == "":
				continue
			}
		}
		if before[field] != after[field] {
			return refuseRewrite("Flux Kustomization %q has a different spec.%s after the rewrite than before it: a patch reads its dependsOn and moves an entry into it, so what it waits for cannot be compared", x, field)
		}
	}
	return nil
}

// checkWrittenEntries is the judgment of obligations when the tree could not be
// built before the rewrite (a patch that fails on the list as generated, but
// succeeds on the rewritten one, say): there is no view to read them from, so
// it asks for what the pass wrote and what the tree holds.
//
//   - every member holds, on each target it held an entry on, an entry at least
//     as strong, and on each terminal leaf of an umbrella target the entry the
//     pass wrote there;
//   - every identity the view builds that applies is one the tree holds, or a
//     namesake whose every occurrence applies and whose applied copy has the
//     graphFields it was held with (go-kure/kure#1221).
//
// Its readiness fields are judged against the tree's own spec
// (checkReadinessFields).
func (t *kustomizationTree) checkWrittenEntries(v *builtView, q *viewSnapshot, f rewriteFacts) error {
	// There is no view before the rewrite, so the tree as it held its members
	// stands for it: every entry a member held, and the leaves the pass wrote
	// for each umbrella among them, is asked of the built tree.
	if err := t.checkObligations(&viewSnapshot{classes: f.held}, q, f); err != nil {
		return err
	}
	for _, id := range sortedKeys(q.ids) {
		// A held non-member whose every occurrence applies stands for what the
		// build makes under its identity (U3 compares them). One an apply skips
		// creates nothing, so a copy the controller does apply under that
		// identity would go unjudged.
		stands, err := t.heldStandsFor(v, id, true)
		if err != nil {
			return err
		}
		stands = stands && !slices.ContainsFunc(v.occurrences[id], func(o occurrence) bool { return !v.applies(o) })
		if t.byKey[id] != nil || stands {
			continue
		}
		if slices.ContainsFunc(v.occurrences[id], v.applies) {
			return refuseRewrite("the build of the tree makes Flux Kustomization %q, which the tree does not hold, and the tree could not be built before the rewrite to compare what it waits for: a patch that builds only on the rewritten dependsOn may drop a wait", id)
		}
	}
	return nil
}

// describeClasses names the classes in s, for a refusal.
func describeClasses(s classSet) string {
	if s == 0 {
		return "no entry"
	}
	var parts []string
	for c, name := range []string{"existence-only", "readyExpr", "plain"} {
		if s.has(c) {
			parts = append(parts, name)
		}
	}
	return fmt.Sprintf("only %s", strings.Join(parts, " and "))
}

// sortedKeys returns the keys of m in order.
func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
