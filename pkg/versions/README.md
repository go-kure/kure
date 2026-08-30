# Versions - Supported-Version Metadata

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/versions.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/versions)

Stable Go access to kure's supported-version metadata.

`versions.yaml` at the repository root records, per infrastructure dependency, the range of
deployed tool versions kure generates YAML for. This package exposes the machine-readable part of
that file so consumers do not have to read it off disk and decode key paths by hand.

## Usage

```go
import "github.com/go-kure/kure/pkg/versions"

k8s, ok := versions.Get("kubernetes")   // ok == false if the key was renamed or removed
fmt.Println(k8s.SupportedRange, k8s.Min, k8s.Max) // "1.33 - 1.37" "1.33" "1.37"

for _, d := range versions.All() {
    fmt.Printf("%s: %s (%s)\n", d.Name, d.SupportedRange, d.GoModule)
}
```

`GoVersion` holds `versions.yaml`'s `go.current`.

## What is exported, and what is not

Exported: `Name`, `GoModule`, `SupportedRange`, `Min`, `Max`, `VersionBasis`.

Not exported: `notes`, `related_packages`, `upstream_repo`, `upstream_release`,
`upstream_release_commit`, the go.mod build version. The build version moves on every
dependency bump; the others can change too (reviewer prose edits, a release-pinned
dependency getting re-pinned) but not on every routine bump — an in-range patch bump
needs no `versions.yaml` change at all. Excluding all of them either way keeps this
API's content stable rather than tied to prose or pin churn.

## Regenerating

Never edit `versions_gen.go` by hand:

```bash
mise run versions:generate    # ./scripts/sync-versions.sh generate
mise run versions:check       # fails if the committed file has drifted (also run in CI)
```

Removing an `infrastructure` key an external consumer looks up is a breaking change for them even
though no Go signature moved — see the organization API stability contract before doing it.
