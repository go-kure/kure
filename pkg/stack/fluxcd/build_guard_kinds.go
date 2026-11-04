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
	// every kind recorded is hashed and asked about.
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
// (it has a metadata key, or sits in a List that has one), so a data file with
// a deep merge chain still builds.
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
// so concurrent builds share nothing: the visits made, the mappings each
// merge key names, and what each mapping a merge key names contributes.
type kindWalk struct {
	seen         map[kindVisit]bool
	sources      map[*kyaml.Node][]*kyaml.Node
	contributing map[*kyaml.Node]mergeContribution
}

func newKindWalk() *kindWalk {
	return &kindWalk{
		seen:         map[kindVisit]bool{},
		sources:      map[*kyaml.Node][]*kyaml.Node{},
		contributing: map[*kyaml.Node]mergeContribution{},
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

// mergeSources is mergeSources(v), once for each node v names.
func (w *kindWalk) mergeSources(v *kyaml.Node) []*kyaml.Node {
	v = resolve(v)
	if v == nil {
		return nil
	}
	if srcs, ok := w.sources[v]; ok {
		return srcs
	}
	srcs := mergeSources(v)
	w.sources[v] = srcs
	return srcs
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
// named once. The memos live one call, so concurrent builds share nothing.
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
		for _, src := range w.mergeSources(v) {
			c := w.contribution(g, src)
			m.absorb(c.meta)
			m.deepMerge = m.deepMerge || c.deeper
		}
	}
	if m.deepMerge && (object || m.hasMetadata) {
		return "which holds an object whose apiVersion or kind may come through merge keys more than one level deep, which kure does not follow to check the scope of its kind"
	}
	object = object || m.hasMetadata
	if strings.HasSuffix(m.kind, "List") && m.items != nil {
		if items := resolve(m.items); items != nil && items.Kind == kyaml.SequenceNode {
			if depth >= maxListDepth {
				if object {
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
	g.note(m.apiVersion, m.kind)
	return ""
}

// readKeys is m.read(n, merges), counting the keys of n it reads.
func (g *buildGuard) readKeys(m *objectMeta, n *kyaml.Node, merges *[]*kyaml.Node) {
	g.kindKeys += len(n.Content) / 2
	m.read(n, merges)
}

// note records the kind once, in the order met.
func (g *buildGuard) note(apiVersion, kind string) {
	if apiVersion == "" || kind == "" || len(apiVersion) > maxKindBytes || len(kind) > maxKindBytes {
		return
	}
	if key := (kindKey{apiVersion, kind}); !g.kindSeen[key] {
		g.kindSeen[key] = true
		g.kinds = append(g.kinds, key)
	}
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
