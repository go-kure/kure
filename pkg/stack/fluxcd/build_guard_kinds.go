package fluxcd

import (
	"regexp"
	"strings"

	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// kindKey is the apiVersion and kind of an object. The two stay apart, as
// strings that share the bytes of the file's nodes, so a long value that many
// kinds repeat through an alias is held once.
type kindKey struct{ apiVersion, kind string }

const (
	// maxListDepth is how deep loadedKinds follows a List into the Lists it
	// holds. kustomize follows them without end; a build with more is not read.
	maxListDepth = 8

	// maxKindBytes is the longest apiVersion or kind loadedKinds records. A
	// longer one names no kind (a Kubernetes group is at most 253 bytes), and
	// every kind recorded is hashed and asked about. One met on an object is
	// refused, and on any other mapping skipped.
	maxKindBytes = 1024
)

// loadedKinds records the apiVersion and kind of each document of b that is an
// object, and of each item of a List, and returns why the build is not read
// when it cannot tell, or "". kustomize reads the scope of a kind as it loads
// the object (name references, vars), before an overlay may delete the object,
// so the objects the build ends with are not all it asked about
// (go-kure/kure#1235).
//
// It reads the YAML nodes, never the objects: b is any file the build reads,
// data a generator only copies included, so nothing may expand an alias (a
// cyclic anchor would not end), and the several documents of a patch file may
// name one object twice, which kustomize accepts. The file is cut into
// documents the way kustomize cuts it (splitDocuments), and one document is
// parsed from each part. A document that is no mapping (a list of patch
// operations, a scalar, an empty document), or has no scalar apiVersion and
// kind, holds no kind and is skipped; so is a part that fails to parse, since
// kustomize fails a resource file at such a part, and reads no kind from a
// data file. A value given as an alias, and a mapping given as an alias, are
// read through one hop. The mappings a merge key (<<) names are followed
// through their whole chain (mergeChain), for the fields apiVersion, kind and
// items and for a metadata key. A merge chain that leads back to itself ends,
// and an object that reaches one is not read; so is a kind that ends in List
// (kustomize reads one as its items) holding Lists more than maxListDepth deep,
// but only when it looks like an object (it has a metadata key, or sits in a
// List that has one, or holds below the depth cap an item that has one), so a
// data file with a cyclic merge chain or deep Lists still builds. A member of a
// merge cycle takes the apiVersion, kind and items it lacks from the other
// members when they offer one value, whichever member the file enters by; when
// they offer several, any mapping that reaches the cycle is not read, data or
// object. An apiVersion or kind over maxKindBytes on an object is not read
// either, and a data file with one is skipped.
// The false refusal left is a List that deep, an object that reaches a cyclic
// merge chain, or a mapping that reaches an ambiguous one, in a file that is no
// resource.
func (g *buildGuard) loadedKinds(b []byte) string {
	w := newKindWalk()
	for _, part := range splitDocuments(string(b)) {
		var doc kyaml.Node
		if err := kyaml.NewDecoder(strings.NewReader(part)).Decode(&doc); err != nil {
			continue
		}
		if doc.Kind == kyaml.DocumentNode && len(doc.Content) == 1 {
			if why := g.noteKinds(doc.Content[0], 0, false, w); why != "" {
				return why
			}
		}
	}
	return ""
}

// separatorLine is a line that starts a new document for kustomize.
var separatorLine = regexp.MustCompile(`\n---.*\n`)

// splitDocuments cuts a file into the parts kustomize parses one document
// from: at every line that starts with "---", before any YAML is read, so text
// after a document end marker ("...") does not hide the parts behind it. It
// follows kyaml's kio.ByteReader: carriage returns are dropped, every part but
// the last keeps its final newline, and a separator line with anything but a
// comment after the dashes ends the split (kustomize fails the file there).
func splitDocuments(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var parts []string
	prev := 0
	for _, loc := range separatorLine.FindAllStringIndex(s, -1) {
		parts = append(parts, s[prev:loc[0]]+"\n")
		if rest := strings.TrimSpace(s[loc[0]+4 : loc[1]]); rest != "" && rest[0] != '#' {
			return parts
		}
		prev = loc[1]
	}
	return append(parts, s[prev:])
}

// kindVisit is everything a visit of noteKinds depends on: the mapping read
// (an alias already resolved), how deep in Lists it sits, and whether an
// enclosing List or the mapping itself is an object.
type kindVisit struct {
	n      *kyaml.Node
	depth  int
	object bool
}

// kindWalk is what one loadedKinds call remembers, and forgets with the call,
// so concurrent builds share nothing: the visits made, and what each mapping
// and each merge value gives together with everything it merges, however deep.
type kindWalk struct {
	seen map[kindVisit]bool
	// merged is what a mapping, or a sequence of mappings a merge key names,
	// gives with its whole merge chain: black in the walk of mergeChain. It
	// depends on the node alone, never on the visit that asked for it.
	merged map[*kyaml.Node]mergeContribution
	// gray is the nodes mergeChain has reached and not yet settled, during one
	// of its calls and empty between them. A node in neither merged nor gray is
	// white.
	gray map[*kyaml.Node]*chainFrame
	// held is the set of nodes holdsObject has read, apart from seen: what lies
	// below the depth cap is asked about without being walked, so a node met
	// there is still walked when an alias brings it to a shallower depth.
	held map[*kyaml.Node]bool
}

func newKindWalk() *kindWalk {
	return &kindWalk{
		seen:   map[kindVisit]bool{},
		merged: map[*kyaml.Node]mergeContribution{},
		gray:   map[*kyaml.Node]*chainFrame{},
		held:   map[*kyaml.Node]bool{},
	}
}

// mergeContribution is what a mapping, with the mappings its merge keys name
// at any depth, gives the mapping that names it: the first value found of each
// key and whether a metadata key is found anywhere. cyclic is whether the
// chain leads back to itself, which is then no chain with an end to read:
// hasMetadata is exact, and a field a member of the cycle lacks is the one
// value the cycle offers of it, whichever member was reached first. When the
// cycle offers several, ambiguous says the mapping cannot be read: whatever it
// holds first would depend on the order the file is read in.
type mergeContribution struct {
	meta      objectMeta
	cyclic    bool
	ambiguous bool
}

// offered is the value the mappings of a merge cycle offer of one field: the
// first, and whether another differs. Values are told apart by key, which is
// cheap to compare.
type offered[K comparable, V any] struct {
	key           K
	val           V
	set, conflict bool
}

func (o *offered[K, V]) add(k K, v V) {
	if !o.set {
		o.key, o.val, o.set = k, v, true
	} else if o.key != k {
		o.conflict = true
	}
}

func (o *offered[K, V]) join(p offered[K, V]) {
	if p.set {
		o.add(p.key, p.val)
	}
	o.conflict = o.conflict || p.conflict
}

// stringKey tells two strings apart in constant time however long they are:
// by length, the first maxKindBytes+1 bytes and whether they end in List,
// which is all a string over maxKindBytes is asked: it names no kind
// whichever it is, but a kind that ends in List is still read as its items.
type stringKey struct {
	n    int
	head string
	list bool
}

func keyOf(s string) stringKey {
	return stringKey{len(s), s[:min(len(s), maxKindBytes+1)], strings.HasSuffix(s, "List")}
}

// chainFrame is a node of the merge graph in mergeChain's walk. A mapping has
// an edge to the value of each of its merge keys, in order, and a sequence one
// to each mapping it holds. own is what the mapping sets itself; meta adds what
// the nodes it has settled give, first found first; the offers are the values
// of own and of those nodes, which a cycle shares between its members.
type chainFrame struct {
	node      *kyaml.Node
	own, meta objectMeta
	api, kind offered[stringKey, string]
	items     offered[*kyaml.Node, *kyaml.Node]
	succ      []*kyaml.Node
	next      int
	index     int
	low       int
	// looped is whether the frame names itself or a node still open; cyclic
	// whether the chain reaches a cycle at all.
	looped, cyclic, ambiguous bool
}

// offer adds the fields m sets to the values f offers.
func (f *chainFrame) offer(m objectMeta) {
	if m.hasAPIVersion {
		f.api.add(keyOf(m.apiVersion), m.apiVersion)
	}
	if m.hasKind {
		f.kind.add(keyOf(m.kind), m.kind)
	}
	if m.items != nil {
		n := resolve(m.items)
		f.items.add(n, n)
	}
}

// take absorbs a settled node's contribution c into f.
func (f *chainFrame) take(c mergeContribution) {
	f.meta.absorb(c.meta)
	f.offer(c.meta)
	f.cyclic = f.cyclic || c.cyclic
	f.ambiguous = f.ambiguous || c.ambiguous
}

// mergeChain is the contribution of the mapping or sequence root with its
// merge chain followed in full, worked out once for the call. It is the same as
// absorbing the mappings in order, each with its own merge keys followed, so a
// mapping that is named many times, or a sequence of mappings, costs its edges
// once.
//
// The merge graph may lead back to itself, and a chain may be long, so the
// walk is Tarjan's strongly connected components with an explicit stack, not
// recursion: a node is white (in neither w.merged nor w.gray), gray (reached,
// its component not closed) or black (in w.merged), and never judged by how
// deep the walk is. A merge key that names a gray node closes a cycle, and a
// merge key that names a cyclic node reaches one. When the component of a node
// closes, its members are all cyclic if any of them is, and they all get a
// hasMetadata that is the OR over the members, so no member of a cycle misses a
// metadata key another one holds, whichever it was reached from.
//
// A cycle has no first member, so what a member lacks of apiVersion, kind and
// items is not what a fold from the node it was entered at finds. Each member
// keeps its own keys; a field it lacks is the one value the whole component
// offers (the members' own keys and what they have settled), computed once for
// the component. When the component offers several, the member is ambiguous,
// and so is every node that reaches it.
func (w *kindWalk) mergeChain(g *buildGuard, root *kyaml.Node) mergeContribution {
	if c, ok := w.merged[root]; ok {
		return c
	}
	var calls, open []*chainFrame
	index := 0
	enter := func(n *kyaml.Node) {
		f := &chainFrame{node: n, index: index, low: index}
		index++
		if n.Kind == kyaml.MappingNode {
			var merges []*kyaml.Node
			g.readKeys(&f.meta, n, &merges)
			f.own = f.meta
			f.offer(f.own)
			for _, v := range merges {
				if v = resolve(v); v != nil && (v.Kind == kyaml.MappingNode || v.Kind == kyaml.SequenceNode) {
					f.succ = append(f.succ, v)
				}
			}
		} else {
			f.succ = mergeSources(n)
		}
		w.gray[n] = f
		calls = append(calls, f)
		open = append(open, f)
	}
	enter(root)
	for len(calls) > 0 {
		f := calls[len(calls)-1]
		if f.next < len(f.succ) {
			s := f.succ[f.next]
			f.next++
			g.kindAbsorbs++
			if c, ok := w.merged[s]; ok {
				f.take(c)
			} else if sf, ok := w.gray[s]; ok {
				f.cyclic, f.looped = true, true
				f.low = min(f.low, sf.index)
			} else {
				enter(s)
			}
			continue
		}
		calls = calls[:len(calls)-1]
		if f.low == f.index {
			// The component of f is closed: the nodes opened since f.
			at := len(open) - 1
			for open[at] != f {
				at--
			}
			members := open[at:]
			open = open[:at]
			looped := len(members) > 1 || f.looped
			var cyclic, metadata, ambiguous bool
			var api, kind offered[stringKey, string]
			var items offered[*kyaml.Node, *kyaml.Node]
			var lacksAPI, lacksKind, lacksItems bool
			for _, mf := range members {
				cyclic = cyclic || mf.cyclic
				metadata = metadata || mf.meta.hasMetadata
				ambiguous = ambiguous || mf.ambiguous
				api.join(mf.api)
				kind.join(mf.kind)
				items.join(mf.items)
				// A sequence of merge values sets no key, so it lacks all three:
				// what it gives is cached for whoever merges it later.
				lacksAPI = lacksAPI || !mf.own.hasAPIVersion
				lacksKind = lacksKind || !mf.own.hasKind
				lacksItems = lacksItems || mf.own.items == nil
			}
			// A cycle has no first member: each member keeps its own keys and
			// takes of the others only the one value the cycle offers, since a
			// fold from where the walk entered would depend on that. When a
			// member lacks a field the cycle offers several values of, the cycle
			// is ambiguous, and so are all its members and what reaches them.
			// So a fold that found a value of a field the member lacks found
			// the one the cycle offers, and what is left to fill is what the
			// fold did not reach.
			if looped {
				ambiguous = ambiguous || api.conflict && lacksAPI || kind.conflict && lacksKind || items.conflict && lacksItems
			}
			for _, mf := range members {
				m := mf.meta
				if looped {
					if !m.hasAPIVersion && api.set && !api.conflict {
						m.hasAPIVersion, m.apiVersion = true, api.val
					}
					if !m.hasKind && kind.set && !kind.conflict {
						m.hasKind, m.kind = true, kind.val
					}
					if m.items == nil && items.set && !items.conflict {
						m.items = items.val
					}
				}
				m.hasMetadata = metadata
				w.merged[mf.node] = mergeContribution{meta: m, cyclic: cyclic, ambiguous: ambiguous}
				delete(w.gray, mf.node)
			}
		}
		if len(calls) > 0 {
			p := calls[len(calls)-1]
			if c, ok := w.merged[f.node]; ok {
				p.take(c)
			} else {
				// Still open: f and p are in one component.
				p.meta.absorb(f.meta)
				p.low = min(p.low, f.low)
			}
		}
	}
	return w.merged[root]
}

// noteKinds records the kind of the node n when it is a mapping, and of its
// items when its kind ends in List. object is whether n, or a List it sits in,
// has a metadata key: only then is a bound that stops the read a refusal.
//
// w.seen holds the visits of one loadedKinds call: of a mapping, and of the
// sequence of items of a List. A visit met again gives the result of the
// first: a refusal ended the call, and the kinds it recorded are recorded. So
// a mapping or a sequence many aliases name is read once per depth and object
// flag, and the Lists are walked in at most (maxListDepth+1) times 2 reads of
// each node of the file, not in the paths through its aliases. What a mapping
// gives with its whole merge chain is worked out once for the call
// (mergeChain), however many mappings name it, so the keys read are those of
// each mapping met once, and the absorbs are one for each merge edge. The
// object flag is an argument of the visit and never part of that memo. The
// memos live one call, so concurrent builds share nothing.
func (g *buildGuard) noteKinds(n *kyaml.Node, depth int, object bool, w *kindWalk) string {
	n = resolve(n)
	if n == nil || n.Kind != kyaml.MappingNode {
		return ""
	}
	visit := kindVisit{n, depth, object}
	if w.seen[visit] {
		return ""
	}
	w.seen[visit] = true
	g.kindVisits++
	c := w.mergeChain(g, n)
	m := c.meta
	if c.ambiguous {
		return "which holds a mapping whose merge cycle leaves its apiVersion, kind or items ambiguous, so kure cannot tell the scope of its kind"
	}
	if c.cyclic && (object || m.hasMetadata) {
		return "which holds an object whose merge keys lead back to themselves, which kure does not follow to check the scope of its kind"
	}
	object = object || m.hasMetadata
	if strings.HasSuffix(m.kind, "List") && m.items != nil {
		if items := resolve(m.items); items != nil && items.Kind == kyaml.SequenceNode {
			if depth >= maxListDepth {
				if object || g.holdsObject(items, w) {
					return "which holds Lists nested deeper than kure follows to check the scope of their kinds"
				}
				return g.note(m.apiVersion, m.kind, false)
			}
			// The Lists that hold one sequence step over it once.
			if step := (kindVisit{items, depth, object}); !w.seen[step] {
				w.seen[step] = true
				for _, item := range items.Content {
					g.kindItems++
					if why := g.noteKinds(item, depth+1, object, w); why != "" {
						return why
					}
				}
			}
		}
	}
	return g.note(m.apiVersion, m.kind, object)
}

// holdsObject says whether a node of the sequence or mapping n, or an item of
// the Lists below it, is an object: it has a metadata key, of its own or from
// its merge chain at any depth (mergeChain). It is asked at the depth cap,
// where the Lists are not walked, so that what they hold is not dropped when
// it may be an object. w.held is the set of nodes read for the call (noteKinds
// does not read it), so each is read once and the cost is that of the nodes of
// the file. A true ends the call in a refusal, so a node read without finding
// an object is the only answer the set needs to keep, and Lists that hold
// themselves end the read without a cycle walk: a node met again was, or is
// being, read.
func (g *buildGuard) holdsObject(n *kyaml.Node, w *kindWalk) bool {
	n = resolve(n)
	if n == nil || w.held[n] {
		return false
	}
	w.held[n] = true
	g.kindScans++
	switch n.Kind {
	case kyaml.SequenceNode:
		for _, item := range n.Content {
			if g.holdsObject(item, w) {
				return true
			}
		}
	case kyaml.MappingNode:
		c := w.mergeChain(g, n)
		if c.ambiguous || c.meta.hasMetadata {
			return true
		}
		m := c.meta
		if strings.HasSuffix(m.kind, "List") {
			return g.holdsObject(m.items, w)
		}
	default:
		// A scalar holds no object.
	}
	return false
}

// readKeys is m.read(n, merges), counting the keys of n it reads.
func (g *buildGuard) readKeys(m *objectMeta, n *kyaml.Node, merges *[]*kyaml.Node) {
	g.kindKeys += len(n.Content) / 2
	m.read(n, merges)
}

// note records the kind once, in the order met. An apiVersion or kind longer
// than maxKindBytes names no kind and is not recorded, but on an object it is a
// refusal: kustomize reads the object and kure cannot ask about its kind.
func (g *buildGuard) note(apiVersion, kind string, object bool) string {
	if len(apiVersion) > maxKindBytes || len(kind) > maxKindBytes {
		if object {
			return "which holds an object whose apiVersion or kind is longer than kure records to check the scope of its kind"
		}
		return ""
	}
	if apiVersion == "" || kind == "" {
		return ""
	}
	if key := (kindKey{apiVersion, kind}); !g.kindSeen[key] {
		g.kindSeen[key] = true
		g.kinds = append(g.kinds, key)
	}
	return ""
}

// objectMeta is what loadedKinds reads of a mapping: the first value found of
// each key, one the mapping sets itself before any a merge key brings. A key
// present with an empty or non-scalar value is found: it hides the same key of
// a merge source.
type objectMeta struct {
	apiVersion, kind string
	items            *kyaml.Node
	hasAPIVersion    bool
	hasKind          bool
	hasMetadata      bool
}

// read takes from the mapping n what m has not got yet, and appends the values
// of its merge keys to merges.
func (m *objectMeta) read(n *kyaml.Node, merges *[]*kyaml.Node) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := resolve(n.Content[i]), n.Content[i+1]
		if k == nil || k.Kind != kyaml.ScalarNode {
			continue
		}
		if k.Tag == "!!merge" {
			*merges = append(*merges, v)
			continue
		}
		switch k.Value {
		case "apiVersion":
			if !m.hasAPIVersion {
				m.hasAPIVersion, m.apiVersion = true, scalarValue(v)
			}
		case "kind":
			if !m.hasKind {
				m.hasKind, m.kind = true, scalarValue(v)
			}
		case "items":
			if m.items == nil {
				m.items = v
			}
		case "metadata":
			m.hasMetadata = true
		}
	}
}

// absorb takes from s, the fields of a mapping a merge key names, what m has
// not got yet: the same as m.read of that mapping.
func (m *objectMeta) absorb(s objectMeta) {
	if !m.hasAPIVersion && s.hasAPIVersion {
		m.hasAPIVersion, m.apiVersion = true, s.apiVersion
	}
	if !m.hasKind && s.hasKind {
		m.hasKind, m.kind = true, s.kind
	}
	if m.items == nil {
		m.items = s.items
	}
	m.hasMetadata = m.hasMetadata || s.hasMetadata
}

// mergeSources is the mappings the value v of a merge key names: v itself, or
// each element of v, read through one alias hop.
func mergeSources(v *kyaml.Node) []*kyaml.Node {
	v = resolve(v)
	if v == nil {
		return nil
	}
	if v.Kind == kyaml.MappingNode {
		return []*kyaml.Node{v}
	}
	var out []*kyaml.Node
	if v.Kind == kyaml.SequenceNode {
		for _, e := range v.Content {
			if e = resolve(e); e != nil && e.Kind == kyaml.MappingNode {
				out = append(out, e)
			}
		}
	}
	return out
}

// resolve is n, or the node an alias n names, one hop.
func resolve(n *kyaml.Node) *kyaml.Node {
	if n != nil && n.Kind == kyaml.AliasNode {
		return n.Alias
	}
	return n
}

// scalarValue is the value of the scalar node n, read through one alias hop,
// or "" for any other node.
func scalarValue(n *kyaml.Node) string {
	if n = resolve(n); n == nil || n.Kind != kyaml.ScalarNode {
		return ""
	}
	return n.Value
}
