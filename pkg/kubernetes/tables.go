package kubernetes

import "strings"

// KindInfo describes one object kind registered in [Scheme]: its group,
// version and kind, the Go type that implements it, and the pinned module that
// type came from.
//
// The values live in zz_generated_tables.go, derived from the registered
// scheme and the upstream +kubebuilder:resource markers. They are plain
// strings and bools on purpose: the generator writes into this package, so a
// table that referenced upstream types could not be regenerated after an API
// bump removed one of them.
type KindInfo struct {
	Group         string
	Version       string
	Kind          string
	GoType        string // Go type name, without package qualifier
	ImportPath    string // package declaring GoType
	Module        string // module providing ImportPath
	ModuleVersion string // the version this build pins
	Namespaced    bool
	// ScopeSource names what said so: "marker" for the kind's own
	// +kubebuilder:resource marker, "crd" for the CustomResourceDefinition its
	// module ships, "builtin" for the kinds whose scope the API server rather
	// than a marker defines. A wrong scope is traceable to the thing that
	// claimed it.
	ScopeSource string
}

// GroupKind returns the "group/Kind" key the scope lookups use. The core group
// is empty, so a core kind reads as "/Pod".
func (k KindInfo) GroupKind() string { return k.Group + "/" + k.Kind }

// APIVersion returns the apiVersion string for the kind: "v1" for the core
// group, "group/version" otherwise.
func (k KindInfo) APIVersion() string {
	if k.Group == "" {
		return k.Version
	}
	return k.Group + "/" + k.Version
}

// Maturity is what the pinned upstream sources say about a field's stability.
type Maturity string

const (
	// MaturityStable is the absence of any maturity signal.
	MaturityStable Maturity = ""
	// MaturityAlpha is declared by the field's own documentation.
	MaturityAlpha Maturity = "alpha"
	// MaturityBeta is declared by the field's own documentation.
	MaturityBeta Maturity = "beta"
	// MaturityDeprecated is declared by a "Deprecated:" doc comment.
	MaturityDeprecated Maturity = "deprecated"
)

// FieldMaturity is one field, of one type reachable from a registered kind,
// that upstream gates or documents as less than stable.
//
// kure neither warns nor filters on this. It reports, and a consumer with
// cluster knowledge decides. The table exists because the failure it describes
// is silent: for built-in types the API server does not reject a field whose
// feature gate is disabled, it clears the field and admits the object, so the
// manifest reads as applied and is not.
//
// Only construction-side fields appear. Status types are excluded: their
// fields are reported by the cluster, never set by a caller.
type FieldMaturity struct {
	ImportPath    string   // package declaring TypeName
	TypeName      string   // Go type declaring the field
	Field         string   // the field's JSON name, or its Go name when untagged
	GoField       string   // the Go field name
	Gates         []string // upstream feature gates that must be enabled
	Stability     Maturity
	Module        string
	ModuleVersion string
}

// Kinds returns every object kind registered in the scheme, with the pinned
// module its Go type came from and the scope derived from that module.
//
// The returned slice is a copy. The table it comes from is process-global and is
// read on every lookup, including the ones [github.com/go-kure/kure/pkg/manifest]
// answers scope questions with, so a consumer that edited a row in place — to
// relabel a kind for its own output, say — would change what every later caller
// in the process is told, at a distance and with nothing to trace it to.
func Kinds() []KindInfo {
	out := make([]KindInfo, len(kinds))
	copy(out, kinds)
	return out
}

// FieldMaturities returns every construction-side field of a registered kind
// that the pinned sources gate or document as less than stable. Status types
// are not walked; see the package README for why.
//
// A copy, for the reason [Kinds] gives, and a deep one: each row's Gates slice
// is copied too, since a shallow copy would hand out rows that still share their
// gate list with the table.
func FieldMaturities() []FieldMaturity {
	out := make([]FieldMaturity, 0, len(fieldMaturities))
	for _, m := range fieldMaturities {
		out = append(out, copyMaturity(m))
	}
	return out
}

// copyMaturity returns m with its own Gates slice.
func copyMaturity(m FieldMaturity) FieldMaturity {
	if m.Gates != nil {
		gates := make([]string, len(m.Gates))
		copy(gates, m.Gates)
		m.Gates = gates
	}
	return m
}

// KindByGroupKind returns the registered kind for a "group/Kind" key.
func KindByGroupKind(groupKind string) (KindInfo, bool) {
	for _, k := range kinds {
		if k.GroupKind() == groupKind {
			return k, true
		}
	}
	return KindInfo{}, false
}

// KindFor returns the registered kind for an apiVersion and kind, matching the
// version as well as the group.
//
// The version is part of the match because most of what a KindInfo carries is
// version-specific: GoType, ImportPath and ModuleVersion describe one version's
// Go type. Answering an "autoscaling/v1" question with the registered
// "autoscaling/v2" row would name a type that does not implement the version
// asked about. A group/kind kure registers at a version other than the one
// asked for reads as unregistered, which is what it is.
func KindFor(apiVersion, kind string) (KindInfo, bool) {
	group, version := groupVersion(apiVersion)
	for _, k := range kinds {
		if k.Group == group && k.Version == version && k.Kind == kind {
			return k, true
		}
	}
	return KindInfo{}, false
}

// IsNamespaced reports whether a registered kind is namespaced, and whether it
// is registered at all. A caller must distinguish the two: an unregistered
// kind's scope is unknown, not namespaced.
//
// Unlike [KindFor] this ignores the version, and deliberately: scope is a
// property of the resource, uniform across the versions of one group/kind — an
// apiVersion the API server serves at all serves it at the same scope. A
// manifest written against an older or newer version of a kind kure registers
// must still be answered, since the alternative is "unknown" for a scope that
// is not in doubt.
func IsNamespaced(apiVersion, kind string) (namespaced, known bool) {
	group, _ := groupVersion(apiVersion)
	k, ok := KindByGroupKind(group + "/" + kind)
	if !ok {
		return false, false
	}
	return k.Namespaced, true
}

// groupVersion splits an apiVersion into its group and version. The core group
// is written without one ("v1"), so a value with no slash is all version.
func groupVersion(apiVersion string) (group, version string) {
	if g, v, ok := strings.Cut(apiVersion, "/"); ok {
		return g, v
	}
	return "", apiVersion
}

// MaturityForType returns the maturity entries declared by one Go type. The
// rows are copies, Gates included.
func MaturityForType(importPath, typeName string) []FieldMaturity {
	var out []FieldMaturity
	for _, m := range fieldMaturities {
		if m.ImportPath == importPath && m.TypeName == typeName {
			out = append(out, copyMaturity(m))
		}
	}
	return out
}

// GatedFields returns every field that requires an upstream feature gate. The
// rows are copies, Gates included.
func GatedFields() []FieldMaturity {
	var out []FieldMaturity
	for _, m := range fieldMaturities {
		if len(m.Gates) > 0 {
			out = append(out, copyMaturity(m))
		}
	}
	return out
}
