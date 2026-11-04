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
// data file. A value given as an alias,
// and a mapping given as an alias, are read through one hop, as are the
// mappings a merge key (<<) names; an object kustomize reads through more, or a
// kind that ends in List (kustomize reads one as its items) holding Lists more
// than maxListDepth deep, is not read, but only when it looks like an object
// (it has a metadata key, or sits in a List that has one, or holds below the
// depth cap an item that has one), so a data file with a deep merge chain or
// deep Lists still builds. An apiVersion or kind over maxKindBytes on an
// object is not read either, and a data file with one is skipped.
// The false refusal left is a List that deep, or an object with a merge chain
// that deep, in a file that is no resource.
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
// so concurrent builds share nothing: the visits made, what each mapping a
// merge key names contributes, and what the mappings a merge value names
// contribute together.
type kindWalk struct {
	seen         map[kindVisit]bool
	contributing map[*kyaml.Node]mergeContribution
	aggregates   map[*kyaml.Node]mergeContribution
	// held is the set of nodes holdsObject has read, apart from seen: what lies
	// below the depth cap is asked about without being walked, so a node met
	// there is still walked when an alias brings it to a shallower depth.
	held map[*kyaml.Node]bool
	// chained is the set of mappings holdsObject has followed a merge chain
	// through, for the same reason.
	chained map[*kyaml.Node]bool
	// chainDone is the set of merge values chainHoldsMetadata has been asked
	// about, so a sequence many mappings name is walked once.
	chainDone map[*kyaml.Node]bool
}

func newKindWalk() *kindWalk {
	return &kindWalk{
		seen:         map[kindVisit]bool{},
		contributing: map[*kyaml.Node]mergeContribution{},
		aggregates:   map[*kyaml.Node]mergeContribution{},
		held:         map[*kyaml.Node]bool{},
		chained:      map[*kyaml.Node]bool{},
		chainDone:    map[*kyaml.Node]bool{},
	}
}

// mergeContribution is what a mapping a merge key names gives the mapping that
// names it: the fields of its own keys, read as a mapping on its own, and
// whether it has a merge key itself (which is not followed). It depends on the
// mapping alone, never on the one that names it.
type mergeContribution struct {
	meta   objectMeta
	deeper bool
}

// aggregate is what the mappings the merge value v names give together, worked
// out once for every call however many mappings name v: the first value found
// of each key, and a merge key or a metadata key in any of them. It is the same
// as absorbing the mappings in order into the mapping that names v, because
// absorb takes the first value of a key and ORs the flags, so the mappings a
// merge value names cost one absorb each for the call, and each mapping that
// names it one.
func (w *kindWalk) aggregate(g *buildGuard, v *kyaml.Node) mergeContribution {
	v = resolve(v)
	if v == nil {
		return mergeContribution{}
	}
	if c, ok := w.aggregates[v]; ok {
		return c
	}
	var c mergeContribution
	for _, src := range mergeSources(v) {
		s := w.contribution(g, src)
		c.meta.absorb(s.meta)
		g.kindAbsorbs++
		c.deeper = c.deeper || s.deeper
	}
	w.aggregates[v] = c
	return c
}

// contribution reads the keys of the mapping src once for every call.
func (w *kindWalk) contribution(g *buildGuard, src *kyaml.Node) mergeContribution {
	if c, ok := w.contributing[src]; ok {
		return c
	}
	var c mergeContribution
	var merges []*kyaml.Node
	g.readKeys(&c.meta, src, &merges)
	c.deeper = len(merges) > 0
	w.contributing[src] = c
	return c
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
// each node of the file, not in the paths through its aliases. A mapping a
// merge key names is read once for the call, however many mappings name it, so
// the keys read are those of each mapping visited plus those of each mapping
// named once. What the mappings a merge value names give together is worked
// out once as well (aggregate), so the absorbs are one for each mapping a merge
// value names plus one for each merge key that uses it, not their product.
// The memos live one call, so concurrent builds share nothing.
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
	m := objectMeta{}
	var merges []*kyaml.Node
	g.readKeys(&m, n, &merges)
	for _, v := range merges {
		c := w.aggregate(g, v)
		m.absorb(c.meta)
		g.kindAbsorbs++
		m.deepMerge = m.deepMerge || c.deeper
	}
	if m.deepMerge && (object || m.hasMetadata) {
		return "which holds an object whose apiVersion or kind may come through merge keys more than one level deep, which kure does not follow to check the scope of its kind"
	}
	object = object || m.hasMetadata
	if strings.HasSuffix(m.kind, "List") && m.items != nil {
		if items := resolve(m.items); items != nil && items.Kind == kyaml.SequenceNode {
			if depth >= maxListDepth {
				if object || g.holdsObject(items, w) {
					return "which holds Lists nested deeper than kure follows to check the scope of their kinds"
				}
				return ""
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
// the Lists below it, is an object: it has a metadata key, of its own or from a
// merge key, through a chain of any depth (chainHoldsMetadata). It is asked at
// the depth cap, where the Lists are not walked, so that what they hold is not
// dropped when it may be an object. w.held is the
// set of nodes read for the call (noteKinds does not read it), so each is read
// once and the cost is that of the nodes of the file. A true ends the call in a
// refusal, so a node read without finding an object is the only answer the set
// needs to keep, and Lists that hold themselves end the read without a cycle
// walk: a node met again was, or is being, read.
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
		m := objectMeta{}
		var merges []*kyaml.Node
		g.readKeys(&m, n, &merges)
		deeper := false
		for _, v := range merges {
			c := w.aggregate(g, v)
			m.absorb(c.meta)
			g.kindAbsorbs++
			deeper = deeper || c.deeper
		}
		if m.hasMetadata {
			return true
		}
		if deeper {
			for _, v := range merges {
				if g.chainHoldsMetadata(v, w) {
					return true
				}
			}
		}
		if strings.HasSuffix(m.kind, "List") {
			return g.holdsObject(m.items, w)
		}
	default:
		// A scalar holds no object.
	}
	return false
}

// chainHoldsMetadata says whether a metadata key comes through the merge value
// v at any depth: noteKinds follows a merge key one level, and a mapping below
// the depth cap whose metadata lies deeper would otherwise be taken for data.
// A merge value is asked about once for the call (w.chainDone), and a mapping
// is read once (w.chained), which also ends a chain that leads back to itself.
func (g *buildGuard) chainHoldsMetadata(v *kyaml.Node, w *kindWalk) bool {
	v = resolve(v)
	if v == nil || w.chainDone[v] {
		return false
	}
	w.chainDone[v] = true
	for _, src := range mergeSources(v) {
		g.kindScans++
		if w.chained[src] {
			continue
		}
		w.chained[src] = true
		m := objectMeta{}
		var merges []*kyaml.Node
		g.readKeys(&m, src, &merges)
		if m.hasMetadata {
			return true
		}
		for _, next := range merges {
			if g.chainHoldsMetadata(next, w) {
				return true
			}
		}
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
	deepMerge        bool // a mapping a merge key names has a merge key itself
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
