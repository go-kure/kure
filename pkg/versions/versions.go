// Package versions exposes kure's supported-version metadata as a stable Go
// API, so consumers do not have to parse versions.yaml key paths by hand.
//
// The data in versions_gen.go is generated from versions.yaml by
// scripts/sync-versions.sh; `./scripts/sync-versions.sh check` (run in CI)
// fails if the generated file has drifted from versions.yaml.
//
// Only machine-readable fields are exported. Reviewer prose (notes,
// related_packages), the release-pinning fields, and the go.mod build
// version are omitted: the build version moves on every dependency bump, and
// the others can change too (prose edits, a release-pinned dependency being
// re-pinned) even though not every routine bump touches them. Excluding all
// of them keeps this API's content stable rather than tied to prose or pin
// churn.
package versions

// Dependency is one infrastructure entry's machine-readable metadata.
type Dependency struct {
	// Name is the versions.yaml key, e.g. "kubernetes".
	Name string
	// GoModule is the Go module path kure imports for this dependency.
	GoModule string
	// SupportedRange is the raw supported_range, e.g. "1.33 - 1.36" -- the
	// range of deployed tool versions kure targets, not the build version.
	SupportedRange string
	// Min is the low bound of SupportedRange, major.minor.
	Min string
	// Max is the high bound of SupportedRange; equals Min for a
	// single-value range such as "0.5".
	Max string
	// VersionBasis is "semver" (default) or "kubernetes" -- the latter for a
	// module versioned v0.N.x whose range is expressed in cluster terms (1.N).
	VersionBasis string
}

// All returns every infrastructure dependency, in versions.yaml document
// order. The returned slice is a copy; callers may modify it freely.
func All() []Dependency {
	out := make([]Dependency, len(infrastructure))
	copy(out, infrastructure)
	return out
}

// Get returns the dependency stored under the given versions.yaml key. ok is
// false when no such entry exists.
func Get(name string) (Dependency, bool) {
	for _, d := range infrastructure {
		if d.Name == name {
			return d, true
		}
	}
	return Dependency{}, false
}
