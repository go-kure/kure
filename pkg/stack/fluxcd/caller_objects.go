package fluxcd

import (
	"encoding"
	"encoding/base64"
	"encoding/json"
	"math"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/built"
	"github.com/go-kure/kure/internal/nilobj"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

const (
	// maxJSONDepth is how deep a value is followed: the nesting encoding/json
	// takes, 10,000 levels. Deeper, origin/main's own write of the object fails
	// with an error ("exceeded max depth"), so a refusal there loses nothing; up
	// to it main reads and writes the object, so a lower limit would refuse what
	// main accepts.
	maxJSONDepth = 10000

	// maxHeldDepth is how deep walkHeld follows a typed object, counted in the
	// pointers, interfaces, fields, elements and values it steps through, which
	// are several for each level of a value.
	maxHeldDepth = 4 * maxJSONDepth
)

// maxJSONNodes is how many values are visited in one object. An object
// is far smaller (the API server stores 1.5 MiB at most); a larger count
// is a value that shares its parts so often that its tree is huge.
// It is a var only so that a test can lower it through the setter in
// export_test.go; no in-process test that does so may be t.Parallel.
var maxJSONNodes = 1 << 22

// probeCallerObjects is the entry probe of the entry points that read the
// objects of the layouts of roots, and of the layouts below them
// (go-kure/kure#1245): it calls GetObjectKind, GetName, GetNamespace,
// GetLabels and GetAnnotations once on each object a caller put in a layout
// and on each object a typed List holds (built.ProbeHeld), and returns the
// refusal of the first whose method panics, naming its layout and its Go type.
// The reads that follow call these methods at many places that are not each
// guarded, so a method that panics must be found here, before the first of
// them. It runs after normalizeCallerObjects has succeeded, as a pass of its
// own: that walk calls no method of a caller's object. It changes no caller's
// object. The one write it makes is the one every read of OriginBundleObjects
// makes: the annotations a configMapGenerator entry sets are copied to its
// stand-in, which is kure's own object. Nil roots are skipped.
//
// The layout package keeps a copy of the same walk for its writers
// (probeCallerObjects in pkg/stack/layout/treecheck.go): it cannot import this
// package, and one shared helper would be new public API. Change both together,
// except for the loops over the origin records (OriginApplicationObjects and
// OriginBundleObjects): they are this copy's alone, because no writer of the
// layout package reads either record.
func probeCallerObjects(roots ...*layout.ManifestLayout) error {
	seen := map[*layout.ManifestLayout]bool{}
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		if l == nil || seen[l] {
			return nil
		}
		seen[l] = true
		for _, r := range l.Resources {
			if err := built.ProbeHeld(r); err != nil {
				return errors.Wrapf(err, "layout %q", l.FullRepoPath())
			}
		}
		// The origin records hold the objects a delivery intent is applied
		// to. They are the values the layouts hold, but a caller can drop one
		// from a layout and leave it in the record.
		for _, rec := range l.OriginApplicationObjects() {
			for _, o := range rec.Objects {
				if err := built.ProbeHeld(o); err != nil {
					return errors.Wrapf(err, "layout %q", l.FullRepoPath())
				}
			}
		}
		// The objects each bundle of a unit emitted are read by the check that a
		// bundle's patch selects none of the other bundles' objects, and the
		// record of them is a third place a caller can leave an object in.
		for _, b := range l.OriginBundles() {
			for _, o := range l.OriginBundleObjects(b) {
				// ProbeHeld leaves a nil to its reader, and the reader of this
				// record, the patch-scope check, calls a method on every
				// element: a nil is refused here, in the words callerObjects
				// uses for a nil resource (go-kure/kure#1297).
				if nilobj.Is(o) {
					return errors.Errorf("layout %q: bundle %q: a nil %T in its origin record", l.FullRepoPath(), b.Name, o)
				}
				if err := built.ProbeHeld(o); err != nil {
					return errors.Wrapf(err, "layout %q", l.FullRepoPath())
				}
			}
		}
		for _, child := range l.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := walk(root); err != nil {
			return err
		}
	}
	return nil
}

// normalizeCallerObjects makes every unstructured object a caller put in the
// layouts of roots, and of the layouts below them, one kure can read
// (go-kure/kure#1201). apimachinery requires such an object to hold JSON
// typed values only, and panics in every Nested* read and in DeepCopy on any
// other, a plain int say. An object that holds only the types
// runtime.DeepCopyJSONValue accepts is left as it is. Another is replaced by
// a copy in which each value of a shape listed at jsonContentOf is its JSON
// typed counterpart. Its Object field is set to the new map, and the caller's
// map is not changed.
//
// A value of any other shape refuses the call, with the layout, the object
// and the field named, and no object is replaced then. So does a value that
// holds itself, one nested deeper than maxJSONDepth and a call whose objects
// need more than maxJSONNodes values converted or checked in all: that count is
// one for the whole call, as a map shared by many objects is expanded once for
// each. The walk itself calls no method of a value the caller built,
// also not to read a typed List that holds it: a marshaler could not be
// stopped on a value that holds itself. The methods of a caller's objects are
// called by a separate pass that runs after the walk succeeds
// (probeCallerObjects).
//
// The objects are those in each layout's Resources, and the unstructured
// objects a typed List in them holds, directly or through the typed Lists it
// holds (built.HeldObjects). A plain unstructured List such a List holds, or a
// typed object holds in a field, is covered with its items and with its own
// Object map. Nil roots are skipped.
//
// A type that embeds an unstructured object, as a resource or as an item of a
// typed List, is not normalized: the layout and the walker's records of an
// object's origin hold the wrapper, so a copy in its place would lose what is
// done to the wrapper. It is refused, naming its type, when its map needs
// conversion (refuseWrapper), and left as it is when it does not.
//
// It returns the function that puts every replaced Object map back, for a
// call that a later check refuses: a refused call leaves the tree as the
// caller gave it. The caller does not change the tree during the call: an
// object whose Object the caller has set to another map since is left with
// that one, but a change inside the replacement is put back with it.
func normalizeCallerObjects(roots ...*layout.ManifestLayout) (undo func(), err error) {
	var replace []replacement
	seenLayout := map[*layout.ManifestLayout]bool{}
	seenObject := map[*unstructured.Unstructured]bool{}
	seenList := map[*unstructured.UnstructuredList]bool{}
	budget := &convBudget{}
	refuse := func(l *layout.ManifestLayout, u *unstructured.Unstructured, cause error) error {
		kind, namespace, name := identityOf(u)
		return errors.Wrapf(cause, "layout %q: %s %q", l.FullRepoPath(), kind, crKey(namespace, name))
	}
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		if l == nil || seenLayout[l] {
			return nil
		}
		seenLayout[l] = true
		for i, r := range l.Resources {
			if err := refuseWrapper(l, i, r, budget); err != nil {
				return err
			}
			objs, others, bad, err := callerObjects(r, budget)
			if err != nil {
				if bad != nil {
					return refuse(l, bad, err)
				}
				// Name the object or field that is the cause, when an item is.
				for _, u := range objs {
					if _, convErr := jsonContentOf(u.Object, budget); convErr != nil {
						return refuse(l, u, convErr)
					}
				}
				return errors.Wrapf(err, "layout %q: read list items of resource %d", l.FullRepoPath(), i)
			}
			for _, o := range others {
				if err := refuseWrapper(l, i, o, budget); err != nil {
					return err
				}
			}
			// A plain unstructured List a typed List holds has its own map beside
			// its items. It is no client.Object, so it is never a resource.
			for _, o := range others {
				ul, ok := o.(*unstructured.UnstructuredList)
				if !ok || seenList[ul] {
					continue
				}
				seenList[ul] = true
				own := &unstructured.Unstructured{Object: ul.Object}
				content, err := jsonContentOf(own.Object, budget)
				if err != nil {
					return refuse(l, own, err)
				}
				if content != nil {
					replace = append(replace, replacement{obj: own, list: ul, held: ul.Object, parsed: content})
				}
			}
			for _, u := range objs {
				if seenObject[u] {
					continue
				}
				seenObject[u] = true
				content, err := jsonContentOf(u.Object, budget)
				if err != nil {
					return refuse(l, u, err)
				}
				if content != nil {
					replace = append(replace, replacement{obj: u, held: u.Object, parsed: content})
				}
			}
		}
		for _, c := range l.Children {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := walk(root); err != nil {
			return func() {}, err
		}
	}
	for _, r := range replace {
		r.set(r.parsed)
	}
	return func() {
		for _, r := range replace {
			// Only the replacement is put back: a caller who has set another
			// map since keeps it.
			if sameMap(r.current(), r.parsed) {
				r.set(r.held)
			}
		}
	}, nil
}

// replacement is the map of obj that normalizeCallerObjects replaced, with the
// map it held. When list is set, obj only stands for the Object map of that
// plain unstructured List, and the map is put in the List.
type replacement struct {
	obj          *unstructured.Unstructured
	list         *unstructured.UnstructuredList
	held, parsed map[string]any
}

func (r replacement) current() map[string]any {
	if r.list != nil {
		return r.list.Object
	}
	return r.obj.Object
}

func (r replacement) set(m map[string]any) {
	if r.list != nil {
		r.list.Object = m
		return
	}
	r.obj.Object = m
}

// sameMap reports whether a and b are the same map, not two with equal content.
func sameMap(a, b map[string]any) bool {
	return a != nil && b != nil && reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

// refuseWrapper refuses r, resource i of layout l or an item of it, when it is
// a wrapper that embeds an unstructured object (embeddedUnstructured) whose map
// holds a value that is not JSON typed, or one that embeds an unstructured List
// with such a value in its own map or in an item. Such a wrapper cannot be normalized:
// the layout and the records the walker made of its origin hold the wrapper,
// and a copy put in its place would no longer carry what is done to the
// wrapper. So it is refused, naming its type, and a wrapper that holds only
// JSON typed values is left as it is. An object that is no wrapper is no
// concern of this check.
func refuseWrapper(l *layout.ManifestLayout, i int, r apiruntime.Object, budget *convBudget) error {
	embedded, st, err := embeddedUnstructured(r)
	if err != nil {
		return errors.Wrapf(err, "layout %q: resource %d", l.FullRepoPath(), i)
	}
	for _, obj := range embedded {
		content, err := jsonContentOf(obj.Object, budget)
		if err == nil && content == nil {
			continue
		}
		if err == nil {
			err = errors.New("holds a value that is not JSON typed")
		}
		kind, namespace, name := identityOf(obj)
		return errors.Wrapf(err, "layout %q: %s %q is held by a %s that embeds it, and is not normalized; hold it as a *unstructured.Unstructured to have it normalized", l.FullRepoPath(), kind, crKey(namespace, name), st)
	}
	return nil
}

// callerObjects returns the unstructured objects r holds as a caller's own: r
// itself, or the unstructured items of a typed List r, which a read of the
// List returns as they are, not as copies.
//
// A typed List is read from what it writes, and writing it marshals what it
// holds. So before that every value reachable from it is walked (walkHeld),
// and a typed List the walk cannot read, one that holds itself or one that
// holds an unstructured object with a value that marshalling would call a
// method of, is refused. The object that fails is returned as bad.
//
// others are the objects a typed List holds that are not unstructured, Lists
// included, which the caller looks at for a wrapper that embeds an unstructured
// object or List.
func callerObjects(r client.Object, budget *convBudget) (objs []*unstructured.Unstructured, others []apiruntime.Object, bad *unstructured.Unstructured, err error) {
	if r == nil {
		return nil, nil, nil, nil
	}
	// A typed nil holds no object, and a method called on it may panic, here and
	// in the phases after this one: it is refused, not passed on unread. An
	// unstructured one is no exception: its accessors dereference it.
	if v := reflect.ValueOf(r); v.Kind() == reflect.Pointer && v.IsNil() {
		return nil, nil, nil, errors.Errorf("a nil %T", r)
	}
	if o, ok := r.(*unstructured.Unstructured); ok {
		return []*unstructured.Unstructured{o}, nil, nil, nil
	}
	// The walk is bounded, so it goes first: the cycle check below revisits a
	// List held twice each time it is held.
	held, bad, err := walkHeld(r, budget, false)
	if err != nil {
		return nil, nil, bad, err
	}
	if err := built.RefuseItemCycle(r); err != nil {
		return nil, nil, nil, err
	}
	objs, others, err = built.HeldObjects(r)
	if err != nil || len(held.found) == 0 && len(held.lists) == 0 && held.refusal == nil {
		return objs, others, nil, err
	}
	// Any typed object that is no List is copied whole, so the objects it holds in
	// a field are normalized as the items of a List are: the caller's own object
	// is changed, with the same undo on a refusal, and one that cannot be
	// replaced and needs a change is refused. seenObject skips one that is
	// returned twice.
	isList, err := built.IsList(r)
	if err != nil {
		return objs, others, nil, err
	}
	if !isList {
		if held.refusal != nil {
			return objs, others, held.refusal.bad, held.refusal.err
		}
		return append(objs, held.found...), appendLists(others, held.lists), nil, nil
	}
	// A typed List is read from the items it writes, which HeldObjects returned:
	// what it holds in another field is read by nothing, and refuses nothing. An
	// item that is no List is copied whole as any other typed object is, so it is
	// looked at the same way. An item that is a List has its own items among
	// these.
	var lists []*unstructured.UnstructuredList
	for _, o := range others {
		if isList, err := built.IsList(o); err != nil {
			return objs, others, nil, err
		} else if isList {
			continue
		}
		inner, bad, err := walkHeld(o, budget, true)
		if err != nil {
			return objs, others, bad, err
		}
		if inner.refusal != nil {
			return objs, others, inner.refusal.bad, inner.refusal.err
		}
		objs = append(objs, inner.found...)
		lists = append(lists, inner.lists...)
	}
	return objs, appendLists(others, lists), nil, nil
}

// appendLists adds the plain unstructured Lists held in a field to others, where
// normalizeCallerObjects covers the Object map of each as it does one that a
// typed List holds.
func appendLists(others []apiruntime.Object, lists []*unstructured.UnstructuredList) []apiruntime.Object {
	for _, l := range lists {
		others = append(others, l)
	}
	return others
}

// identityOf returns the kind, namespace and name of u for a refusal, read
// from the fields as they are. The accessors of Unstructured format a value
// that is no string, and a value that holds itself has no end when formatted.
func identityOf(u *unstructured.Unstructured) (kind, namespace, name string) {
	kind, _ = u.Object["kind"].(string)
	meta, _ := u.Object["metadata"].(map[string]any)
	namespace, _ = meta["namespace"].(string)
	name, _ = meta["name"].(string)
	return kind, namespace, name
}

// jsonTyped reports whether v holds only the types runtime.DeepCopyJSONValue
// accepts: a string, an int64, a float64, a bool, nil, a json.Number, and a
// map[string]any or a []any of them, nested no deeper than maxJSONDepth and
// with no more than maxJSONNodes values. A value that is not is not typed
// here, and is converted or refused (jsonContentOf). It must not accept more:
// a value it calls typed is read as it is, and apimachinery panics on any
// other.
func jsonTyped(v any, depth int) bool {
	nodes := 0
	return jsonTypedCount(v, depth, &nodes)
}

func jsonTypedCount(v any, depth int, nodes *int) bool {
	if *nodes++; depth > maxJSONDepth || *nodes > maxJSONNodes {
		return false
	}
	switch v := v.(type) {
	case string, int64, float64, bool, nil, json.Number:
		return true
	case map[string]any:
		// The root is level 1, so a map at depth maxJSONDepth is level
		// maxJSONDepth+1, which encoding/json does not take. A nil map is null.
		if v != nil && depth >= maxJSONDepth {
			return false
		}
		for _, e := range v {
			if !jsonTypedCount(e, depth+1, nodes) {
				return false
			}
		}
		return true
	case []any:
		if v != nil && depth >= maxJSONDepth {
			return false
		}
		return !slices.ContainsFunc(v, func(e any) bool { return !jsonTypedCount(e, depth+1, nodes) })
	}
	return false
}

// jsonContentOf returns a copy of the object m, with each value of an accepted
// shape as its JSON typed counterpart, or nil for an object that needs no
// change (jsonTyped). The accepted shapes are closed:
//   - the types runtime.DeepCopyJSONValue accepts, which are kept as they are
//     (a json.Number included);
//   - a Go integer kind (int, int8 to int64, uint, uint8 to uint64, uintptr) as
//     an int64, so an integer above 2^53 keeps its exact value; a uint64 above
//     the largest int64 is refused, as it has no int64;
//   - a float32 as the float64 of its shortest 32-bit decimal form, as
//     encoding/json writes it (float32(0.1) is 0.1);
//   - a byte slice as its base64 string, as encoding/json writes it, which a
//     list of more than maxJSONNodes bytes is not;
//   - a named string, bool, integer or float kind as its base type;
//   - a map with string keys and a slice of any accepted element, element by
//     element, a nil map or slice staying nil; an interface holds an element;
//   - nothing else.
//
// Everything else is refused with the field named: a struct, a pointer, an
// array, a channel, a function, a map with other keys, and any type with a
// marshaler (json.RawMessage and net.IP among them). The conversion is made
// from the Go values and never marshals one, so it ends on every value. A slice
// longer than the values left of the budget is refused before it is copied.
func jsonContentOf(m map[string]any, budget *convBudget) (map[string]any, error) {
	if jsonTyped(m, 0) {
		return nil, nil
	}
	c := &converter{open: map[openKey]bool{}, budget: budget}
	v, err := c.convert(reflect.ValueOf(m), nil, 0)
	if err != nil {
		return nil, err
	}
	content, _ := v.(map[string]any)
	return content, nil
}

var (
	unstructuredType     = reflect.TypeFor[unstructured.Unstructured]()
	unstructuredListType = reflect.TypeFor[unstructured.UnstructuredList]()
)

// walkHeld walks every value reachable from the typed object r, before
// anything marshals it, and fails closed. It looks for the unstructured objects
// r holds, wherever it holds them, and checks each for what marshalling it
// would call a method of or would not end on (reachable). Interfaces and
// non-nil pointers are followed; the unexported fields marshalling leaves out
// are not. A type of r that marshals itself, a map key included, is refused
// unless its package is trusted (checkType). Each pointer, map and slice is
// followed once on a path: one that holds itself is refused, as is a value
// nested deeper than maxHeldDepth, a walk of more than maxJSONNodes values and
// a value that cannot be read. When
// an unstructured object fails, bad is a wrapper of its Object map, to name it
// in the refusal.
//
// held has the *unstructured.Unstructured objects r holds in a field, a
// runtime.RawExtension's Object included, that the walk reached and checked.
// A copy of r follows each of them, so the caller normalizes them as it does the
// items of a typed List (go-kure/kure#1252). Whether r is copied is for the
// caller to say: a typed List is read from the items it writes, so what it holds
// elsewhere is neither copied nor refused. So the walk only reports, in
// held.refusal, an object that cannot be replaced and needs a change.
//
// With collect set the walk does not charge the objects it reaches to budget: it
// is a second walk of a value that the first one has checked and charged.
func walkHeld(r apiruntime.Object, budget *convBudget, collect bool) (held heldFields, bad *unstructured.Unstructured, err error) {
	w := &heldWalk{open: map[heldKey]bool{}, budget: budget, collect: collect}
	err = w.walk(reflect.ValueOf(r), nil, 0)
	return heldFields{found: w.found, lists: w.lists, refusal: w.refusal}, w.bad, err
}

// heldFields is what walkHeld reports of the unstructured objects held in a
// field of a typed object.
type heldFields struct {
	// found are the objects a copy of the typed object follows, held by pointer
	// or in a field of a value that is addressed, so that the caller can replace
	// the Object map of each.
	found []*unstructured.Unstructured
	// lists are the plain unstructured Lists held the same way. Their items are
	// in found; the Object map of the List itself is replaced as the one of a
	// plain List that a typed List holds (normalizeCallerObjects).
	lists []*unstructured.UnstructuredList
	// refusal is the first object held by value in a map or an interface, which
	// cannot be replaced, and holds a value that is not JSON typed: bad names it.
	refusal *heldRefusal
}

type heldRefusal struct {
	bad *unstructured.Unstructured
	err error
}

// trail is where a value is in the object being read, for a refusal that names
// it. A step holds its own text (or its index) and a link to its parent, so one
// level deeper costs one small value: the path is made by String, when a
// refusal needs it, not copied at every level, which costs the square of the
// depth. The nil trail is the root.
type trail struct {
	parent  *trail
	text    string
	index   int
	indexed bool

	// blank is set while the path up to and including this step is empty, so a
	// key below an empty key at the root is still the first of its path.
	blank bool
}

// isBlank reports whether the path of t is empty.
func (t *trail) isBlank() bool { return t == nil || t.blank }

// then returns the place one step below t, which text adds to its path.
func (t *trail) then(text string) *trail {
	return &trail{parent: t, text: text, blank: t.isBlank() && text == ""}
}

// at returns the place of element i of the slice at t.
func (t *trail) at(i int) *trail { return &trail{parent: t, index: i, indexed: true} }

// key returns the place of the entry k of the map at t, joined to its path by
// a dot unless the path is empty.
func (t *trail) key(k string) *trail {
	if t.isBlank() {
		return t.then(k)
	}
	return t.then("." + k)
}

// String returns the path of t: the steps from the root, in order.
func (t *trail) String() string {
	var steps []*trail
	for s := t; s != nil; s = s.parent {
		steps = append(steps, s)
	}
	var b strings.Builder
	for i := len(steps) - 1; i >= 0; i-- {
		if s := steps[i]; s.indexed {
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(s.index))
			b.WriteByte(']')
		} else {
			b.WriteString(s.text)
		}
	}
	return b.String()
}

// heldKey names a pointer, a map or a slice being walked.
type heldKey struct {
	typ reflect.Type
	ptr uintptr
	len int
}

type heldWalk struct {
	nodes   int
	open    map[heldKey]bool
	checked map[reflect.Type]bool
	// zeroChecked holds the types whose IsZero method is trusted (checkZero).
	zeroChecked map[reflect.Type]bool
	bad         *unstructured.Unstructured
	found       []*unstructured.Unstructured
	lists       []*unstructured.UnstructuredList
	refusal     *heldRefusal

	// budget is the call's: reachable charges the unstructured objects this walk
	// finds to it. nodes counts the typed values of this one resource.
	budget *convBudget
	// collect makes the walk charge nothing to budget (see walkHeld).
	collect bool
}

// scalarKind reports whether a value of kind k holds nothing to walk into.
func scalarKind(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String,
		reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return true
	default:
		return false
	}
}

var runtimeObjectType = reflect.TypeFor[apiruntime.Object]()

// isObjectType reports whether a value of type t, or the pointer to it, is a
// runtime.Object, as the reader of a typed List's items takes it.
func isObjectType(t reflect.Type) bool {
	return t.Implements(runtimeObjectType) || reflect.PointerTo(t).Implements(runtimeObjectType)
}

func (w *heldWalk) walk(v reflect.Value, at *trail, depth int) error {
	if w.nodes++; w.nodes > maxJSONNodes {
		return errors.Errorf("%s holds more than %d values", whereTyped(at), maxJSONNodes)
	}
	if depth > maxHeldDepth {
		return errors.Errorf("%s holds a value nested deeper than %d steps", whereTyped(at), maxHeldDepth)
	}
	switch v.Kind() {
	case reflect.Invalid:
		return nil
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return w.walk(v.Elem(), at, depth)
	default:
	}
	if err := w.checkType(v.Type(), at, ""); err != nil {
		return err
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		if done, err := w.enter(v, heldKey{typ: v.Type(), ptr: v.Pointer()}, at); done || err != nil {
			return err
		}
		defer delete(w.open, heldKey{typ: v.Type(), ptr: v.Pointer()})
		return w.walk(v.Elem(), at, depth+1)
	case reflect.Map:
		// A key is written by its marshaler, if it has one, and a type is checked
		// before any shortcut. A key is not formatted: it may hold itself.
		if err := w.checkType(v.Type().Key(), at, "[key]"); err != nil {
			return err
		}
		if err := w.checkType(v.Type().Elem(), at, "[key]"); err != nil {
			return err
		}
		// An interface key holds any type: the keys are walked, as the values are.
		keys := v.Type().Key().Kind() == reflect.Interface
		if v.IsNil() || scalarKind(v.Type().Elem().Kind()) && !keys {
			return nil
		}
		key := heldKey{typ: v.Type(), ptr: v.Pointer()}
		if _, err := w.enter(v, key, at); err != nil {
			return err
		}
		defer delete(w.open, key)
		entry := at.then("[key]")
		for it := v.MapRange(); it.Next(); {
			if keys {
				if err := w.walk(it.Key(), entry, depth+1); err != nil {
					return err
				}
			}
			if err := w.walk(it.Value(), entry, depth+1); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice, reflect.Array:
		if err := w.checkType(v.Type().Elem(), at, "[]"); err != nil {
			return err
		}
		// A scalar is walked only when it is an object: the reader then makes a
		// slot for each, so each is counted.
		if v.Kind() == reflect.Slice && v.IsNil() || scalarKind(v.Type().Elem().Kind()) && !isObjectType(v.Type().Elem()) {
			return nil
		}
		if v.Len() > maxJSONNodes-w.nodes {
			return errors.Errorf("%s holds more than %d values", whereTyped(at), maxJSONNodes)
		}
		if v.Kind() == reflect.Slice && v.Len() > 0 {
			key := heldKey{typ: v.Type(), ptr: v.Pointer(), len: v.Len()}
			if _, err := w.enter(v, key, at); err != nil {
				return err
			}
			defer delete(w.open, key)
		}
		for i := 0; i < v.Len(); i++ {
			if err := w.walk(v.Index(i), at.at(i), depth+1); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		return w.walkStruct(v, at, depth)
	default:
		// A scalar, a channel, a function: nothing in it for marshalling to call.
		return nil
	}
}

// checkType refuses a type that marshals itself unless its package is trusted.
// Such a method may write what the walk cannot see, from a private field say,
// and may call anything. A package is trusted when it is one of
// trustedStdPackages, or in k8s.io, sigs.k8s.io, github.com/fluxcd or
// github.com/go-kure, and is not a test package. A type of no package, a struct that embeds a
// marshaler for one, is not trusted. A named struct of another package that
// declares no marshaler and only promotes it from an embedded trusted type
// (a type that embeds *unstructured.Unstructured) is trusted: the call is the
// embedded type's.
func (w *heldWalk) checkType(t reflect.Type, at *trail, suffix string) error {
	if t.Kind() == reflect.Interface || w.checked[t] {
		return nil
	}
	if w.checked == nil {
		w.checked = map[reflect.Type]bool{}
	}
	if !hasMarshaler(t) || trustedPackage(t) || trustedType(t) || promotedFromTrusted(t) {
		w.checked[t] = true
		return nil
	}
	return errors.Errorf("%s holds a %s, which marshals itself and is not from a trusted package", whereTyped(at)+suffix, t)
}

var (
	// trustedPackagePrefixes are the module paths whose marshalers are trusted.
	trustedPackagePrefixes = []string{"k8s.io/", "sigs.k8s.io/", "github.com/fluxcd/", "github.com/go-kure/"}

	// trustedTypes are single types of a package that is not trusted, whose
	// marshaler or IsZero method was read. XDSResource (a Cilium Envoy config's
	// resource) writes its embedded protobuf Any with protojson and nothing else
	// of the Envoy config's. protojson resolves the Any's type URL in the global
	// protobuf registry, so a message type a caller registered there is still
	// run, as it is for the same config on main. EndpointSelector (a Cilium
	// policy's selector, which kure's own CiliumNetworkPolicy builders emit) has
	// an IsZero that only tests one field for nil: `n.LabelSelector == nil`
	// (cilium v1.20.4, pkg/policy/api/selector.go). Add a type here only after
	// reading its method; a whole package is never trusted for one type.
	trustedTypes = []string{
		"github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2.XDSResource",
		"github.com/cilium/cilium/pkg/policy/api.EndpointSelector",
	}

	// trustedStdPackages are the standard library packages that define a
	// marshaler. A path with no dot is not enough: package main and a caller's
	// module of a short name have none.
	trustedStdPackages = []string{"encoding/json", "encoding/json/jsontext", "encoding/json/v2", "log/slog", "math/big", "net", "net/netip", "net/url", "regexp", "time"}
)

// marshalerMethods are the methods that make a type marshal itself.
var marshalerMethods = []string{"MarshalJSON", "MarshalText", "MarshalYAML"}

// promotedFromTrusted reports whether every marshaler method of the named
// struct t is promoted from an embedded field of a trusted type, and t declares
// none of its own.
//
// It depends on the Go runtime: a method t declares has a source file, and one
// the compiler writes to promote a method of an embedded field has the file
// "<autogenerated>" (isPromotion). Where this cannot be told, t is not trusted:
// the check fails closed, and a refusal follows. So does a type that embeds
// itself, directly or through other types.
//
// The check has no limit of its own: each type is judged once, so the work is
// linear in the types t reaches, however many paths lead to one. A judgment
// does not depend on the path to the type: a type that is trusted has no
// embedded type on that path to depend on, and one that was refused because it
// reached a type on the path is itself on a cycle, refused from any start.
func promotedFromTrusted(t reflect.Type) bool {
	return promotedFrom(t, map[reflect.Type]bool{}, map[reflect.Type]bool{})
}

func promotedFrom(t reflect.Type, onPath, judged map[reflect.Type]bool) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || t.Name() == "" || onPath[t] {
		return false
	}
	if trusted, ok := judged[t]; ok {
		return trusted
	}
	onPath[t] = true
	defer delete(onPath, t)
	trusted := promotesOnlyTrusted(t, onPath, judged)
	judged[t] = trusted
	return trusted
}

func promotesOnlyTrusted(t reflect.Type, onPath, judged map[reflect.Type]bool) bool {
	found := false
	for _, pt := range []reflect.Type{t, reflect.PointerTo(t)} {
		for _, name := range marshalerMethods {
			m, ok := pt.MethodByName(name)
			if !ok {
				continue
			}
			found = true
			if !isPromotion(m.Func.Pointer()) {
				return false
			}
		}
	}
	if !found {
		return false
	}
	embedsMarshaler := false
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.Anonymous || !hasMarshaler(f.Type) {
			continue
		}
		embedsMarshaler = true
		if !trustedPackage(f.Type) && !promotedFrom(f.Type, onPath, judged) {
			return false
		}
	}
	return embedsMarshaler
}

// isPromotion reports whether the function at pc is a wrapper the compiler
// wrote to promote a method of an embedded field. A function it cannot find, or
// one with a source file, is not.
func isPromotion(pc uintptr) bool {
	f := runtime.FuncForPC(pc)
	if f == nil {
		return false
	}
	file, _ := f.FileLine(f.Entry())
	return file == "<autogenerated>"
}

func trustedPackage(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return trustedPath(t.PkgPath())
}

// trustedType reports whether t, or the type it points to, is one of trustedTypes.
func trustedType(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name() != "" && slices.Contains(trustedTypes, t.PkgPath()+"."+t.Name())
}

func trustedPath(p string) bool {
	switch {
	case p == "" || strings.HasSuffix(p, "_test"):
		return false
	case slices.Contains(trustedStdPackages, p):
		return true
	}
	return slices.ContainsFunc(trustedPackagePrefixes, func(prefix string) bool { return strings.HasPrefix(p, prefix) })
}

// enter marks the pointer, map or slice v, with its key, as open on the path.
// A key already open is a value that holds itself: it is refused.
func (w *heldWalk) enter(v reflect.Value, key heldKey, at *trail) (done bool, err error) {
	if w.open[key] {
		return true, errors.Errorf("%s holds itself", whereTyped(at))
	}
	w.open[key] = true
	return false, nil
}

func (w *heldWalk) walkStruct(v reflect.Value, at *trail, depth int) error {
	switch v.Type() {
	case unstructuredType, unstructuredListType:
		if !v.CanInterface() {
			return errors.Errorf("%s holds an unstructured object that cannot be read", whereTyped(at))
		}
		m, _ := v.FieldByName("Object").Interface().(map[string]any)
		if !w.collect {
			if err := reachable(m, w.budget); err != nil {
				w.bad = &unstructured.Unstructured{Object: m}
				return err
			}
		}
		isList := v.Type() == unstructuredListType
		if isList {
			if err := w.walk(v.FieldByName("Items"), at.then(".Items"), depth+1); err != nil {
				return err
			}
		}
		// A copy of the typed object that holds this one follows it, and
		// apimachinery's copy of an Unstructured panics on a value that is not JSON
		// typed. So it is handed to the caller to normalize, as the unstructured
		// items of a typed List are: the caller's object is changed the same way,
		// its Object map replaced, and put back with the same undo when the call
		// is refused. One that is not addressable (held by value in a map or an
		// interface) cannot be replaced, so it is to be refused unless it needs no
		// change: the walk records it, and the caller refuses it once it knows that
		// the typed object is copied.
		if v.CanAddr() {
			if isList {
				w.lists = append(w.lists, v.Addr().Interface().(*unstructured.UnstructuredList))
			} else {
				w.found = append(w.found, v.Addr().Interface().(*unstructured.Unstructured))
			}
		} else if w.refusal == nil && !jsonTyped(m, 0) {
			w.refusal = &heldRefusal{
				bad: &unstructured.Unstructured{Object: m},
				err: errors.Errorf("%s holds an unstructured object by value, with a value that is not JSON typed: hold it as a *unstructured.Unstructured to have it normalized", whereTyped(at)),
			}
		}
		return nil
	}
	// Marshalling leaves out the private fields of a struct. A type that
	// marshals itself and could write one is refused by checkType.
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if !f.IsExported() && !f.Anonymous {
			continue
		}
		fat := at.then("." + f.Name)
		if err := w.checkZero(f, v.Field(i), fat); err != nil {
			return err
		}
		// A field tagged json:"-" is judged like any other. The default encoder
		// would not write it, but a marshaler may (RawExtension writes its Object),
		// and so may kure's own code (a List compares the items it writes with
		// Items). Standing down there is where two holes were found.
		if err := w.walk(v.Field(i), fat, depth+1); err != nil {
			return err
		}
	}
	return nil
}

var isZeroerType = reflect.TypeFor[interface{ IsZero() bool }]()

// checkZero refuses the field f, of value fv, when marshalling would call the
// IsZero method of a type that is not trusted: encoding/json calls it for a
// field tagged omitzero, as it calls a marshaler. A caller's method may not end
// (it may format a map that holds itself), and that is a stack overflow, which
// nothing recovers. So the method is not called: the object is refused.
//
// The judged type is the one whose method runs: the field's type, or for a
// field of an interface type that declares IsZero, the type of the value the
// interface holds (a nil interface is not called). The judgment depends on that
// type alone, so each is judged once and the work stays linear in the types
// reached. A type that only promotes IsZero from an embedded type is judged by
// its own package, so it is refused unless it is trusted itself.
func (w *heldWalk) checkZero(f reflect.StructField, fv reflect.Value, at *trail) error {
	tag := f.Tag.Get("json")
	if !omitzeroTag(tag) {
		return nil
	}
	t := f.Type
	if f.Anonymous {
		// encoding/json: "Anonymous struct fields are usually marshaled as if their
		// inner exported fields were fields in the outer struct", so an embedded
		// struct, or pointer to one, with no name in its tag is flattened, and its
		// own IsZero is not called (an unexported one too, for it may have exported
		// fields). "An anonymous struct field with a name given in its JSON tag is
		// treated as having that name, rather than being anonymous", and "anonymous
		// struct fields of interface type are treated the same as having that type
		// as the name": both are fields, so they are judged. An unexported embedded
		// field that is not a struct is ignored.
		et := t
		if et.Kind() == reflect.Pointer {
			et = et.Elem()
		}
		name, _, _ := strings.Cut(tag, ",")
		if (!f.IsExported() && et.Kind() != reflect.Struct) || (name == "" && et.Kind() == reflect.Struct) {
			return nil
		}
	} else if !f.IsExported() {
		// encoding/json writes no unexported field that is not embedded.
		return nil
	}
	// encoding/json does not call IsZero on a nil pointer, nor on an interface
	// that is nil or holds a nil pointer: the field is zero without it.
	switch {
	case t.Kind() == reflect.Interface:
		if !t.Implements(isZeroerType) || fv.IsNil() || (fv.Elem().Kind() == reflect.Pointer && fv.Elem().IsNil()) {
			return nil
		}
		t = fv.Elem().Type()
	case t.Kind() == reflect.Pointer && fv.IsNil():
		return nil
	case !t.Implements(isZeroerType) && !reflect.PointerTo(t).Implements(isZeroerType):
		return nil
	}
	if w.zeroChecked[t] {
		return nil
	}
	if !trustedPackage(t) && !trustedType(t) {
		return errors.Errorf("%s holds a %s, whose IsZero method is called for its omitzero tag and is not from a trusted package", whereTyped(at), t)
	}
	if w.zeroChecked == nil {
		w.zeroChecked = map[reflect.Type]bool{}
	}
	w.zeroChecked[t] = true
	return nil
}

// omitzeroTag reports whether the json tag of a field makes encoding/json call
// its IsZero: a tag of "-" leaves the field out, and the options follow the name.
func omitzeroTag(tag string) bool {
	if tag == "-" {
		return false
	}
	_, opts, _ := strings.Cut(tag, ",")
	return slices.Contains(strings.Split(opts, ","), "omitzero")
}

func whereTyped(at *trail) string {
	return "the typed object" + at.String()
}

// reachable reports a value in m that marshalling it would call a method of,
// or would not end on: one with a marshaler, one that holds itself, one nested
// too deep or with too many values, and a struct, a pointer or an array, which
// marshalling follows into fields that may hold any of those. It leaves the
// shapes that marshalling only fails on, a channel say, to the conversion.
func reachable(m map[string]any, budget *convBudget) error {
	if jsonTyped(m, 0) {
		return nil
	}
	c := &converter{open: map[openKey]bool{}, reach: true, budget: budget}
	_, err := c.convert(reflect.ValueOf(m), nil, 0)
	return err
}

// openKey names a map or a slice being converted: a slice by its first
// element and its length, as two slices of one array are two values.
type openKey struct {
	ptr uintptr
	len int
}

// convBudget counts the values, and the bytes of byte slices, the converter
// visits in one call of
// normalizeCallerObjects, for every object it converts or checks (jsonContentOf
// and reachable). The converter expands a shared map once per occurrence, so
// the work is the size of the expansion, not of what the caller allocated, and
// N objects that share one map would each pay for all of it. One budget for the
// call bounds them together. Plain JSON objects (jsonTyped) are not charged:
// they are kept as they are, and the layout of them is the caller's to size.
type convBudget struct{ used int }

type converter struct {
	budget *convBudget
	open   map[openKey]bool

	// reach makes the walk the check of reachable: it refuses what marshalling
	// would call or not end on, and converts nothing else.
	reach bool
}

var (
	jsonNumberType    = reflect.TypeFor[json.Number]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
	yamlMarshalerType = reflect.TypeFor[interface{ MarshalYAML() (any, error) }]()
)

// hasMarshaler reports whether a value of type t, or one addressed, marshals
// itself, which the converter refuses and never calls.
func hasMarshaler(t reflect.Type) bool {
	for _, t := range []reflect.Type{t, reflect.PointerTo(t)} {
		if t.Implements(jsonMarshalerType) || t.Implements(textMarshalerType) || t.Implements(yamlMarshalerType) {
			return true
		}
	}
	return false
}

func (c *converter) convert(v reflect.Value, at *trail, depth int) (any, error) {
	// An interface and the value it holds are one value, as jsonTyped counts them.
	if v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	if c.budget.used++; c.budget.used > maxJSONNodes {
		return nil, errors.Errorf("%s holds more than %d values", where(at), maxJSONNodes)
	}
	if v.Kind() == reflect.Invalid || v.Kind() == reflect.Interface {
		return nil, nil
	}
	if v.Type() == jsonNumberType {
		return json.Number(v.String()), nil
	}
	if hasMarshaler(v.Type()) {
		return nil, unsupported(at, v, "it marshals itself")
	}
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := v.Uint()
		if u > math.MaxInt64 {
			if c.reach {
				return nil, nil
			}
			return nil, errors.Errorf("%s holds %d, which is above the largest int64", where(at), u)
		}
		return int64(u), nil
	case reflect.Float32:
		// As encoding/json writes it, in 32 bits, and reads it back: float32(0.1)
		// is 0.1, not 0.10000000149011612.
		f, err := strconv.ParseFloat(strconv.FormatFloat(v.Float(), 'g', -1, 32), 64)
		if err != nil {
			return nil, errors.Wrapf(err, "%s holds a float32 that is no number", where(at))
		}
		return f, nil
	case reflect.Float64:
		return v.Float(), nil
	case reflect.String:
		return v.String(), nil
	case reflect.Map:
		return c.convertMap(v, at, depth)
	case reflect.Slice:
		return c.convertSlice(v, at, depth)
	case reflect.Chan, reflect.Func, reflect.Complex64, reflect.Complex128, reflect.UnsafePointer:
		if c.reach {
			return nil, nil
		}
		return nil, unsupported(at, v, "")
	default:
		return nil, unsupported(at, v, "")
	}
}

func (c *converter) convertMap(v reflect.Value, at *trail, depth int) (any, error) {
	if hasMarshaler(v.Type().Key()) || (v.Type().Key().Kind() != reflect.String && !c.reach) {
		return nil, unsupported(at, v, "its keys are no plain strings")
	}
	if v.IsNil() {
		return nil, nil
	}
	// The root is level 1 (depth 0), so the map at depth maxJSONDepth is level
	// maxJSONDepth+1: the first one encoding/json does not take.
	if depth >= maxJSONDepth {
		return nil, errors.Errorf("%s holds a value nested deeper than %d levels", where(at), maxJSONDepth)
	}
	key := openKey{ptr: v.Pointer()}
	if c.open[key] {
		return nil, errors.Errorf("%s holds itself", where(at))
	}
	c.open[key] = true
	defer delete(c.open, key)
	// The count is checked before the keys are collected and sorted, as for a
	// slice: the work is not in proportion to a map that must be refused.
	if v.Len() > maxJSONNodes-c.budget.used {
		return nil, errors.Errorf("%s holds more than %d values", where(at), maxJSONNodes)
	}
	keys := v.MapKeys()
	slices.SortFunc(keys, func(a, b reflect.Value) int {
		switch x, y := a.String(), b.String(); {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	})
	out := make(map[string]any, len(keys))
	for _, k := range keys {
		e, err := c.convert(v.MapIndex(k), at.key(k.String()), depth+1)
		if err != nil {
			return nil, err
		}
		out[k.String()] = e
	}
	return out, nil
}

func (c *converter) convertSlice(v reflect.Value, at *trail, depth int) (any, error) {
	if v.IsNil() {
		return nil, nil
	}
	if c.isByteSlice(v.Type()) {
		// As encoding/json writes it: a base64 string, which is no level of
		// nesting, so this goes before the depth check: one below maxJSONDepth
		// maps is kept, as encoding/json keeps it. The text is made once for
		// each occurrence of the slice, so its bytes count against the call's
		// budget as values do, and the check (reach) charges them too: it ends
		// the call as the conversion would, before the text is made.
		if v.Len() > maxJSONNodes-c.budget.used {
			return nil, errors.Errorf("%s holds more than %d bytes", where(at), maxJSONNodes)
		}
		c.budget.used += v.Len()
		if c.reach {
			return nil, nil
		}
		return base64.StdEncoding.EncodeToString(v.Bytes()), nil
	}
	// As for a map: the slice at depth maxJSONDepth is level maxJSONDepth+1.
	if depth >= maxJSONDepth {
		return nil, errors.Errorf("%s holds a value nested deeper than %d levels", where(at), maxJSONDepth)
	}
	// The count is checked before the allocation: a slice of a type that needs no
	// storage can be as long as an int holds.
	if v.Len() > maxJSONNodes-c.budget.used {
		return nil, errors.Errorf("%s holds more than %d values", where(at), maxJSONNodes)
	}
	if v.Len() > 0 {
		key := openKey{ptr: v.Pointer(), len: v.Len()}
		if c.open[key] {
			return nil, errors.Errorf("%s holds itself", where(at))
		}
		c.open[key] = true
		defer delete(c.open, key)
	}
	out := make([]any, v.Len())
	for i := range out {
		e, err := c.convert(v.Index(i), at.at(i), depth+1)
		if err != nil {
			return nil, err
		}
		out[i] = e
	}
	return out, nil
}

// isByteSlice reports whether encoding/json writes a slice of type t as a
// base64 string: its elements are bytes, and a byte has no marshaler.
func (c *converter) isByteSlice(t reflect.Type) bool {
	return t.Elem().Kind() == reflect.Uint8 && !hasMarshaler(t.Elem())
}

// unsupported refuses a value of a shape jsonContentOf does not accept.
func unsupported(at *trail, v reflect.Value, why string) error {
	if why != "" {
		why = ": " + why
	}
	return errors.Errorf("%s holds a %s, which kure does not convert to JSON%s", where(at), v.Type(), why)
}

func where(at *trail) string {
	if path := at.String(); path != "" {
		return path
	}
	return "the object"
}
