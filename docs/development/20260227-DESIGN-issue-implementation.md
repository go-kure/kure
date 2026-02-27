# Kure — Implementation Design

> **Generated**: 2026-02-27
> **Source**: Issue specifications + live codebase exploration
> **Issues designed**: 15

## Design Overview

The 15 issues span three categories: lint alignment (Issues 1-6), documentation (Issues 7-10, 13-15), and structural refactoring (Issues 11-12). A critical finding from codebase exploration is that **Issues 1-6 are substantially complete** on the existing `chore/align-golangci-lint` branch (commit `de76b37`). That branch already enables errcheck, ineffassign, unused, gosimple, bodyclose, durationcheck, errorlint, exhaustive, misspell, nilerr, unconvert, and whitespace -- touching 66 files with +364/-563 lines of changes. The one gap is **gosec** (Issue 4), which is not enabled even on that branch. The design for Issues 1-6 below accounts for this: they primarily document what was done and note the remaining gosec gap.

**Recommended implementation order**: Issues 1-5 (batch as one PR from existing branch), then Issue 4 (gosec separately), then Issue 6 (alignment verification), then Issues 7, 10, 14, 15 (low-effort docs), then Issues 8, 9 (medium-effort docs), then Issues 12, 13 (low-effort Phase 5), and finally Issue 11 (high-effort split). This order minimizes conflicts and builds on completed work.

---

## Issue 1: refactor: enable errcheck linter and fix violations

### Current State

**Already done on branch `chore/align-golangci-lint`.** The current `.golangci.yml` on HEAD (commit `de76b37`) enables `errcheck` in the linter list (line 11) and relaxes it only for test files (lines 46-49). The old `main` branch had `errcheck` in the `disable` list with the comment "Too many unchecked errors in existing code."

The branch diff shows errcheck fixes across 66 files. Key patterns fixed:
- `pkg/launcher/cli.go`: error returns from `json.MarshalIndent`, `yaml.Marshal`, file operations now checked (+69/-69 lines)
- `pkg/cmd/kure/generate/*.go`: error returns from `cobra.MarkFlagRequired` now checked
- `pkg/patch/loader.go`: error returns from `yaml` decode operations now checked (-70 lines of removed dead code)
- `pkg/io/printer.go`, `pkg/io/runtime.go`, `pkg/io/table.go`: print function error returns handled
- `cmd/demo/main.go`: wrapped unchecked errors in `logError()` helper
- Test files: excluded via `.golangci.yml` rule `path: _test\.go` for errcheck

### Implementation Approach

1. **File**: `.golangci.yml` — Already changed. `errcheck` is in the `enable` list. Test exclusion rule is at lines 46-49.
2. **Pattern followed**: Errors from `cobra.MarkFlagRequired()` are now handled with `if err := ...; err != nil` pattern. Print functions use `_ = ` discard pattern or check returns. File close errors use `defer func() { _ = file.Close() }()`.
3. **Remaining work**: Merge the `chore/align-golangci-lint` branch to `main`. No additional code changes needed.

### Refined Acceptance Criteria

- [x] `errcheck` enabled in `.golangci.yml` (already done on branch)
- [x] Zero lint violations from `errcheck` across the codebase
- [ ] PR merged to main
- [ ] CI passes: `make verify`

### Effort Estimate

- **Files touched**: 66 (already done)
- **Lines of code**: +364/-563 net (already done)
- **Risk**: low — work is complete, just needs merge
- **Estimated time**: XS (PR review + merge only)

---

## Issue 2: refactor: enable ineffassign linter and fix violations

### Current State

**Already done on branch `chore/align-golangci-lint`.** The `ineffassign` linter is enabled at line 14 of the current `.golangci.yml`. The old `main` had it in the `disable` list with the comment "Some assignments are intentional for readability."

The branch diff shows very few ineffassign-specific fixes, confirming the issue spec's prediction of "low violation count." The linter was enabled alongside other linters in the single commit `de76b37`.

### Implementation Approach

1. **File**: `.golangci.yml` — Already changed. `ineffassign` is in the `enable` list at line 14.
2. **Remaining work**: None beyond merging the branch.

### Refined Acceptance Criteria

- [x] `ineffassign` enabled in `.golangci.yml`
- [x] Zero lint violations from `ineffassign`
- [ ] PR merged to main
- [ ] CI passes: `make verify`

### Effort Estimate

- **Files touched**: 0 additional (part of Issue 1 branch)
- **Lines of code**: minimal (included in Issue 1 totals)
- **Risk**: low
- **Estimated time**: XS (included in Issue 1 merge)

---

## Issue 3: refactor: enable unused linter and remove dead code

### Current State

**Already done on branch `chore/align-golangci-lint`.** The `unused` linter is enabled at line 16 of the current `.golangci.yml`. The old `main` had it in the `disable` list with the comment "Many utility functions are kept for future use."

Key dead code removals visible in the branch diff:
- `pkg/launcher/patch_processor.go`: -24 lines (removed unused `evaluateConditionSimple` or similar)
- `pkg/patch/loader.go`: -70 lines (substantial dead code removal)
- `internal/fluxcd/source.go`: -1 line (unused import or function)
- `pkg/stack/generators/fluxhelm/v1alpha1_test.go`: -9 lines

The old `.golangci.yml` had `unused.go: "1.24"` setting which is no longer needed since `unused` now runs with default settings.

### Implementation Approach

1. **File**: `.golangci.yml` — Already changed. `unused` is in the `enable` list.
2. **Dead code removed**: Confirmed in diff. No `//nolint:unused` annotations were needed, indicating all flagged code was genuinely dead.
3. **Remaining work**: None beyond merging the branch.

### Refined Acceptance Criteria

- [x] `unused` enabled in `.golangci.yml`
- [x] Dead functions removed
- [x] No `//nolint:unused` annotations needed
- [ ] PR merged to main
- [ ] CI passes: `make verify`

### Effort Estimate

- **Files touched**: ~5 (part of Issue 1 branch)
- **Lines of code**: ~-100 net (included in Issue 1 totals)
- **Risk**: low
- **Estimated time**: XS (included in Issue 1 merge)

---

## Issue 4: security: enable gosec linter for automated security scanning

### Current State

**NOT done.** The `gosec` linter is **not present** in the current `.golangci.yml` on either `main` or the `chore/align-golangci-lint` branch. Neither crane's `.golangci.yml` (`/home/serge/src/autops/wharf/crane/.golangci.yml`) enables `gosec` either — it is not in crane's linter list. The issue spec states this should be enabled for kure because it "generates RBAC rules, certificates, security contexts, and network policies."

The codebase does not have any `//nolint:gosec` annotations currently (confirmed by grep returning zero results in Go files).

Key areas likely to produce gosec findings:
- `pkg/launcher/options.go` line 30: `CacheDir: "/tmp/kurel-cache"` — G101 (hardcoded credentials path) or G304 (file path from variable)
- `pkg/launcher/builder.go`: file write operations using os.Create
- `pkg/launcher/loader.go`: file read operations using os.Open, os.Stat
- `internal/certmanager/acme.go`: constructs TLS-related objects (likely false positives)
- `cmd/demo/main.go` line 154: `os.MkdirAll(filepath.Dir(outputPath), 0755)` — G301 (directory permissions)
- `pkg/patch/yaml_preserve.go`: YAML parsing operations
- `pkg/launcher/cli.go`: file I/O operations

### Implementation Approach

1. **File**: `.golangci.yml` — Add `gosec` to the `enable` list under `# Additional linters`:
   ```yaml
   linters:
     enable:
       # ...existing...
       - gosec
       - gofmt
       # ...
   ```

2. **Run `golangci-lint run` to identify all violations**. Expected categories:
   - **G301** (directory permissions): `os.MkdirAll(..., 0755)` — These are intentional and safe for CLI tools. Annotate with `//nolint:gosec // G301: directory permissions appropriate for CLI output`.
   - **G304** (file path injection): `os.Open(path)` where path comes from user input — Verify each path is sanitized via `filepath.Clean`. Most uses in `pkg/launcher/loader.go` and `pkg/launcher/builder.go` already use `filepath.Join` which sanitizes.
   - **G101** (hardcoded credentials): `/tmp/kurel-cache` in `options.go` — False positive. Annotate with `//nolint:gosec // G101: not a credential, this is a cache directory path`.
   - **G110** (potential DoS from decompression): `pkg/stack/layout/tar.go` — Check if tar extraction has size limits.
   - **G107** (HTTP request with variable URL): unlikely in this codebase (no HTTP client code).

3. **File**: `pkg/launcher/options.go` — Likely needs `//nolint:gosec` annotation on line 30.
4. **File**: `cmd/demo/main.go` — Likely needs annotations on `os.MkdirAll` and `os.Create` calls.
5. **File**: `pkg/stack/layout/tar.go` — Review tar operations for G110 (zip slip / decompression bomb).
6. **File**: `pkg/launcher/loader.go` — Review file operations for G304 annotations.

7. **Pattern to follow**: Use the `//nolint:gosec // GXXX: justification` format. Every nolint must include the rule ID and a justification string.

### Refined Acceptance Criteria

- [ ] `gosec` added to `.golangci.yml` linter enable list
- [ ] All genuine security issues fixed (file path sanitization, permission issues)
- [ ] False positives annotated with `//nolint:gosec // GXXX: [justification]`
- [ ] `make lint` passes with zero gosec violations
- [ ] `make verify` passes
- [ ] No regressions in existing tests

### Effort Estimate

- **Files touched**: ~8-12 (config + annotation files)
- **Lines of code**: ~+20-40 net (mostly annotations and minor fixes)
- **Risk**: low — kure is a library with no HTTP server; most findings will be false positives on file I/O
- **Estimated time**: S

---

## Issue 5: refactor: enable gosimple linter and simplify flagged code

### Current State

**Already done on branch `chore/align-golangci-lint`.** The `gosimple` linter is enabled at line 12 of the current `.golangci.yml`. The old `main` had it in the `disable` list with "Some simplifications reduce readability."

The branch diff does not show extensive gosimple-specific changes, suggesting the existing code was already mostly idiomatic Go. The linter was enabled as part of the batch alignment.

### Implementation Approach

1. **File**: `.golangci.yml` — Already changed. `gosimple` is in the `enable` list.
2. **Remaining work**: None beyond merging the branch.

### Refined Acceptance Criteria

- [x] `gosimple` enabled in `.golangci.yml`
- [x] Zero lint violations from `gosimple`
- [ ] PR merged to main
- [ ] CI passes: `make verify`

### Effort Estimate

- **Files touched**: 0 additional
- **Lines of code**: minimal
- **Risk**: low
- **Estimated time**: XS (included in Issue 1 merge)

---

## Issue 6: refactor: align golangci-lint config with crane linter set

### Current State

**Substantially done on branch `chore/align-golangci-lint`.** The current kure `.golangci.yml` is now **identical** to crane's `/home/serge/src/autops/wharf/crane/.golangci.yml` except for:
- `goimports.local-prefixes`: kure uses `github.com/go-kure/kure`, crane uses `gitlab.com/autops/wharf/crane` (correct difference)

Both configs now enable the same 16 linters:
```
errcheck, gosimple, govet, ineffassign, staticcheck, unused,
bodyclose, durationcheck, errorlint, exhaustive, gofmt, goimports,
misspell, nilerr, unconvert, whitespace
```

The one gap versus the issue spec is `gosec`, which is not in crane's config either. The issue spec's acceptance criteria lists `bodyclose`, `durationcheck`, `errorlint`, `exhaustive`, `misspell`, `nilerr`, `whitespace` as the linters to add -- all are already present.

### Implementation Approach

1. **File**: `.golangci.yml` — Already aligned with crane. No changes needed.
2. **File**: `DEVELOPMENT.md` — Add a section documenting the active linter set. Insert after the "Contributing Workflow" section (around line 50):
   ```markdown
   ### Linting

   Kure uses golangci-lint with the following enabled linters (aligned with crane):

   **Default linters**: errcheck, gosimple, govet, ineffassign, staticcheck, unused
   **Additional linters**: bodyclose, durationcheck, errorlint, exhaustive, gofmt, goimports, misspell, nilerr, unconvert, whitespace

   Configuration: `.golangci.yml`
   ```
3. **Remaining work**: Document the linter set in `DEVELOPMENT.md`, then merge the branch.

### Refined Acceptance Criteria

- [x] All crane linters enabled in kure (bodyclose, durationcheck, errorlint, exhaustive, misspell, nilerr, whitespace)
- [x] Zero violations across all linters
- [ ] Active linter set documented in `DEVELOPMENT.md`
- [ ] PR merged to main
- [ ] CI passes: `make verify`

### Effort Estimate

- **Files touched**: 1 (DEVELOPMENT.md)
- **Lines of code**: ~+15
- **Risk**: low
- **Estimated time**: XS

---

## Issue 7: chore: document k8s.io replace directives in go.mod

### Current State

The `go.mod` file (`/home/serge/src/autops/wharf/kure/go.mod`) contains four replace directives at lines 5-10:

```go
replace (
    k8s.io/api => k8s.io/api v0.33.2
    k8s.io/apimachinery => k8s.io/apimachinery v0.33.2
    k8s.io/cli-runtime => k8s.io/cli-runtime v0.33.2
    k8s.io/client-go => k8s.io/client-go v0.33.2
)
```

These pin all four `k8s.io` modules to `v0.33.2`. The `require` section also declares them at `v0.33.2` (lines 35-38) and `k8s.io/client-go v0.33.2` appears as indirect (line 109). The CI has a K8s compatibility matrix testing against v0.34 and v0.35 (`.github/workflows/ci.yml` lines 419-423), which temporarily overrides these pins.

The replace directives exist because the `k8s.io` modules use a non-standard versioning scheme (v0.X.Y maps to Kubernetes 1.X) and different k8s.io modules at different versions can have incompatible transitive dependencies. Without the pins, `go mod tidy` might resolve to mismatched versions across the four modules.

### Implementation Approach

1. **File**: `go.mod` — Add inline comments above each replace directive explaining:
   ```go
   replace (
       // Pin k8s.io modules to a single consistent version (v0.33.2 = Kubernetes 1.33).
       // The k8s.io ecosystem modules must be at the same minor version to avoid
       // incompatible transitive dependencies between api, apimachinery, and client-go.
       // Remove these pins when upgrading to a new Kubernetes minor version.
       // See: https://github.com/kubernetes/client-go/blob/master/INSTALL.md
       // Related: #253 (expand K8s target range), #129 (K8s upgrade tracking)
       k8s.io/api => k8s.io/api v0.33.2
       k8s.io/apimachinery => k8s.io/apimachinery v0.33.2
       k8s.io/cli-runtime => k8s.io/cli-runtime v0.33.2
       k8s.io/client-go => k8s.io/client-go v0.33.2
   )
   ```

2. **Evaluation**: Check if any pins can be removed now. The CI already tests against v0.34 and v0.35 -- if those tests pass, the v0.33.2 pins are still needed because v0.33.2 is the current *minimum* supported version, not a workaround for a bug. The pins ensure reproducible builds at the declared minimum. These should remain until the K8s version range is updated (#253).

3. **File**: No changes to the actual replace versions. Only comments added.

### Refined Acceptance Criteria

- [ ] Each `k8s.io/*` replace directive has a comment block explaining the reason and removal condition
- [ ] Comment references relevant issues (#253, #129)
- [ ] `go mod tidy` still produces the same `go.sum` (no functional change)
- [ ] `make verify` passes

### Effort Estimate

- **Files touched**: 1 (go.mod)
- **Lines of code**: ~+8 (comments only)
- **Risk**: low — comments only, no functional change
- **Estimated time**: XS

---

## Issue 8: docs: add doc.go examples for CRD builder packages

### Current State

The three CRD builder packages have minimal `doc.go` files:

- `/home/serge/src/autops/wharf/kure/internal/certmanager/doc.go` (3 lines): `// Package certmanager provides helpers for building cert-manager resources`
- `/home/serge/src/autops/wharf/kure/internal/externalsecrets/doc.go` (2 lines): `// Package externalsecrets contains helpers for constructing resources used by the External Secrets Operator.`
- `/home/serge/src/autops/wharf/kure/internal/metallb/doc.go` (2 lines): `// Package metallb provides constructors for MetalLB custom resources`

None of these packages have `example_test.go` files. The existing builder functions are:

**certmanager**:
- `CreateClusterIssuer(name string, spec certv1.IssuerSpec)` in `clusterissuer.go`
- `CreateCertificate(name, namespace string, spec certv1.CertificateSpec)` in `certificate.go`
- `CreateACMEIssuer(server, email string, key cmmeta.SecretKeySelector)` in `acme.go`
- `CreateIssuer(name, namespace string, spec certv1.IssuerSpec)` in `issuer.go`
- Helpers: `SetClusterIssuerACME`, `SetCertificateIssuerRef`, `AddCertificateDNSName`, `CreateACMEDNS01SolverCloudflare`, etc.

**externalsecrets**:
- `CreateSecretStore(name, namespace string, spec esv1.SecretStoreSpec)` in `secretstore.go`
- `CreateExternalSecret(name, namespace string, spec esv1.ExternalSecretSpec)` in `externalsecret.go`
- `CreateClusterSecretStore(name string, spec esv1.SecretStoreSpec)` in `clustersecretstore.go`
- Helpers: `SetSecretStoreProvider`, `SetExternalSecretSecretStoreRef`, `AddExternalSecretData`

**metallb**:
- `CreateBGPPeer(name, namespace string, spec metallbv1beta1.BGPPeerSpec)` in `bgppeer.go`
- `CreateBGPAdvertisement(name, namespace string, spec metallbv1beta1.BGPAdvertisementSpec)` in `bgpadvertisement.go`
- `CreateIPAddressPool(name, namespace string, spec metallbv1beta1.IPAddressPoolSpec)` in `ipaddresspool.go`
- `CreateL2Advertisement(name, namespace string, spec metallbv1beta1.L2AdvertisementSpec)` in `l2advertisement.go`
- `CreateBFDProfile(name, namespace string, spec metallbv1beta1.BFDProfileSpec)` in `bfdprofile.go`

### Implementation Approach

1. **File**: `internal/certmanager/example_test.go` — New file. Create testable example:
   ```go
   package certmanager_test

   import (
       "fmt"

       cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
       certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
       cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

       "github.com/go-kure/kure/internal/certmanager"
   )

   func Example_acmeClusterIssuerWithCertificate() {
       // Create an ACME issuer (Let's Encrypt)
       acmeIssuer := certmanager.CreateACMEIssuer(
           "https://acme-v02.api.letsencrypt.org/directory",
           "admin@example.com",
           cmmeta.SecretKeySelector{
               LocalObjectReference: cmmeta.LocalObjectReference{Name: "letsencrypt-key"},
               Key:                  "tls.key",
           },
       )
       // Add a DNS01 solver
       solver := certmanager.CreateACMEDNS01SolverCloudflare(
           "admin@example.com",
           cmmeta.SecretKeySelector{
               LocalObjectReference: cmmeta.LocalObjectReference{Name: "cloudflare-token"},
               Key:                  "api-token",
           },
       )
       certmanager.AddACMEIssuerSolver(acmeIssuer, solver)

       // Create ClusterIssuer with ACME config
       issuer := certmanager.CreateClusterIssuer("letsencrypt-prod", certv1.IssuerSpec{})
       _ = certmanager.SetClusterIssuerACME(issuer, acmeIssuer)

       // Create a Certificate referencing the issuer
       cert := certmanager.CreateCertificate("app-tls", "default", certv1.CertificateSpec{
           SecretName: "app-tls-secret",
       })
       _ = certmanager.SetCertificateIssuerRef(cert, cmmeta.ObjectReference{
           Name:  "letsencrypt-prod",
           Kind:  "ClusterIssuer",
           Group: "cert-manager.io",
       })
       _ = certmanager.AddCertificateDNSName(cert, "app.example.com")

       fmt.Println(issuer.Name)
       fmt.Println(cert.Name)
       // Output:
       // letsencrypt-prod
       // app-tls
   }
   ```

2. **File**: `internal/externalsecrets/example_test.go` — New file. Create testable example showing SecretStore + ExternalSecret composition with a Vault backend pattern.

3. **File**: `internal/metallb/example_test.go` — New file. Create testable example showing BGPPeer + IPAddressPool + BGPAdvertisement composition.

4. **Pattern to follow**: Use `Example_descriptiveName()` function naming convention. Use `// Output:` directive for compilation verification. Follow the pattern in existing test files in `internal/certmanager/certificate_test.go`.

### Refined Acceptance Criteria

- [ ] `internal/certmanager/example_test.go` with ACME ClusterIssuer + Certificate example
- [ ] `internal/externalsecrets/example_test.go` with SecretStore + ExternalSecret example
- [ ] `internal/metallb/example_test.go` with BGP setup example (Peer + Pool + Advertisement)
- [ ] All examples compile: `go test ./internal/certmanager/... ./internal/externalsecrets/... ./internal/metallb/...`
- [ ] `make verify` passes

### Effort Estimate

- **Files touched**: 3 (new example_test.go files)
- **Lines of code**: ~+200 (3 example files, ~60-70 lines each)
- **Risk**: low — additive, test-only files
- **Estimated time**: S

---

## Issue 9: docs: add integration example (Cluster-to-Disk pipeline)

### Current State

The `examples/` directory exists at `/home/serge/src/autops/wharf/kure/examples/` with 8 subdirectories: `app-workloads`, `bootstrap`, `clusters`, `generators`, `kurel`, `multi-oci`, `patches`, `validation`. The `clusters/basic/` directory contains YAML config files (`cluster.yaml`, `cluster-flux-operator.yaml`, `cluster-argocd.yaml`) but these are declarative YAML consumed by `cmd/demo/main.go` -- not a standalone Go program.

The `cmd/demo/main.go` (`/home/serge/src/autops/wharf/kure/cmd/demo/main.go`, 529 lines) provides the closest existing reference for a full pipeline. Its `runClusterExample()` function (lines 193-273) demonstrates: Cluster decode -> Bundle creation -> Node setup -> FluxWorkflow -> LayoutWithResources -> WriteManifest. However, it reads YAML from disk rather than constructing the domain model programmatically using the fluent builder API.

Key pipeline components:
- `stack.NewClusterBuilder(name)` — fluent builder in `pkg/stack/builders.go` (line 183)
- `stack.NewWorkflow("flux")` — factory in `pkg/stack/workflow.go` (line 32), requires blank import of `pkg/stack/fluxcd`
- `layout.WalkCluster(cluster, rules)` — layout walker in `pkg/stack/layout/walker.go`
- `layout.WriteManifest(dir, cfg, ml)` — disk writer in `pkg/stack/layout/write.go`
- Generator registration via blank imports: `_ "github.com/go-kure/kure/pkg/stack/generators/appworkload"`, `_ "github.com/go-kure/kure/pkg/stack/generators/fluxhelm"`

### Implementation Approach

1. **Directory**: `examples/getting-started/` — New directory.

2. **File**: `examples/getting-started/main.go` — New standalone program demonstrating the full pipeline:
   ```go
   package main

   import (
       "fmt"
       "os"

       "sigs.k8s.io/controller-runtime/pkg/client"

       "github.com/go-kure/kure/pkg/stack"
       "github.com/go-kure/kure/pkg/stack/layout"

       // Register workflow and generator implementations
       _ "github.com/go-kure/kure/pkg/stack/fluxcd"
       _ "github.com/go-kure/kure/pkg/stack/generators/appworkload"
       _ "github.com/go-kure/kure/pkg/stack/generators/fluxhelm"
   )

   func main() {
       // Step 1: Build a Cluster using the fluent builder
       cluster, err := stack.NewClusterBuilder("production").
           WithGitOps(&stack.GitOpsConfig{Type: "flux"}).
           WithNode("root").
               WithChild("infrastructure").
                   WithBundle("cert-manager").
                       WithApplication("cert-manager", &helmApp{...}).
                   End().
               End().
               WithChild("apps").
                   WithBundle("web-app").
                       WithApplication("frontend", &workloadApp{...}).
                   End().
               End().
           End().
           Build()

       // Step 2: Create Flux workflow
       wf, err := stack.NewWorkflow("flux")

       // Step 3: Generate layout with Flux resources
       rules := layout.DefaultLayoutRules()
       result, err := wf.CreateLayoutWithResources(cluster, rules)

       // Step 4: Write to disk
       ml := result.(*layout.ManifestLayout)
       cfg := layout.Config{ManifestsDir: "clusters"}
       err = layout.WriteManifest("output", cfg, ml)
   }
   ```

   The example must include concrete `ApplicationConfig` implementations (simple structs that implement `Generate(*Application) ([]*client.Object, error)`) to make the pipeline runnable.

3. **File**: `examples/getting-started/README.md` — New file explaining each step with comments linking to the relevant packages.

4. **Pattern to follow**: Mirror the structure of `cmd/demo/main.go`'s `runClusterExample()` but use the fluent builder API instead of YAML decoding. Reference the builder pattern documented in `AGENTS.md` lines 326-337.

5. **Build verification**: Add the example to the build system. The `Makefile` should be able to compile it: `go build ./examples/getting-started/`.

### Refined Acceptance Criteria

- [ ] `examples/getting-started/main.go` with complete Cluster-to-Disk pipeline
- [ ] Uses `NewClusterBuilder` fluent API (not YAML decode)
- [ ] Includes ApplicationConfig implementations
- [ ] Compiles: `go build ./examples/getting-started/`
- [ ] Runs: `go run ./examples/getting-started/` produces manifest directory
- [ ] `README.md` explains each step
- [ ] `make verify` passes

### Effort Estimate

- **Files touched**: 2-3 (main.go, README.md, possibly app configs in separate file)
- **Lines of code**: ~+200-300
- **Risk**: medium — requires working ApplicationConfig implementations that produce valid objects; the example must actually generate valid YAML
- **Estimated time**: M

---

## Issue 10: docs: clarify AGENTS.md fmt.Errorf guidance

### Current State

`AGENTS.md` (`/home/serge/src/autops/wharf/kure/AGENTS.md`) contains the following at line 186:

```
**Never use `fmt.Errorf` directly** - always use the errors package.
```

The `pkg/errors/errors.go` (`/home/serge/src/autops/wharf/kure/pkg/errors/errors.go`) implementation at lines 10-28 shows that the errors package itself wraps `fmt.Errorf`:

```go
func Wrap(err error, message string) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf("%s: %w", message, err)
}

func Wrapf(err error, format string, args ...interface{}) error {
    // ...
    return fmt.Errorf(format+": %w", append(args, err)...)
}

func Errorf(format string, args ...interface{}) error {
    return fmt.Errorf(format, args...)
}
```

The contradiction is clear: the guidance says "never use `fmt.Errorf` directly" but the errors package itself is a thin wrapper around `fmt.Errorf`. The intent is that *application code* should use `errors.Wrap`/`errors.Wrapf`/`errors.Errorf` rather than calling `fmt.Errorf` directly, for consistency and potential future stack trace support.

Additionally, the `pkg/stack/builders.go` file uses `fmt.Errorf` directly at lines 194, 227, 249, 254, 278, 282, 308, 337, 339, 346, 369, 394 -- these are in the fluent builder which accumulates errors rather than returning them immediately. This pattern is intentional (errors are collected, not wrapped).

### Implementation Approach

1. **File**: `AGENTS.md` — Update lines 174-186 in the "Error Handling" section:
   ```markdown
   ### Error Handling

   Always use the kure/errors package in application code:

   ```go
   import "github.com/go-kure/kure/pkg/errors"

   // Wrapping errors with context
   return errors.Wrap(err, "context about what failed")

   // Creating new formatted errors
   return errors.Errorf("description: %s", detail)

   // Creating simple errors
   return errors.New("description of error")
   ```

   **Use `pkg/errors` functions instead of `fmt.Errorf` directly** in application code.
   The `pkg/errors` package itself uses `fmt.Errorf` internally -- this is correct.
   Exception: fluent builders that accumulate `[]error` may use `fmt.Errorf` for
   lightweight error construction (see `pkg/stack/builders.go`).

   Preferred:
   ```go
   return errors.Wrap(err, "failed to load config")
   ```

   Discouraged in application code:
   ```go
   return fmt.Errorf("failed to load config: %w", err)
   ```
   ```

2. **No code changes** — documentation only.

### Refined Acceptance Criteria

- [ ] AGENTS.md error handling section updated with precise wording
- [ ] Clarifies that `pkg/errors` internals using `fmt.Errorf` is correct
- [ ] Notes the fluent builder exception
- [ ] Shows preferred vs discouraged patterns with examples
- [ ] `make verify` passes (no code changes)

### Effort Estimate

- **Files touched**: 1 (AGENTS.md)
- **Lines of code**: ~+15 net (replaced section)
- **Risk**: low — documentation only
- **Estimated time**: XS

---

## Issue 11: refactor: split pkg/launcher into sub-packages

### Current State

`pkg/launcher/` contains 14 source files totaling 6,497 non-test lines:

| File | Lines | Responsibility |
|------|-------|---------------|
| `schema.go` | 1,144 | SchemaGenerator impl: JSON schema generation, field tracing |
| `validator.go` | 860 | Validator impl: package/resource/patch validation |
| `patch_processor.go` | 734 | PatchProcessor impl: dependency resolution, patch application |
| `cli.go` | 655 | CLI struct: cobra commands, top-level orchestration |
| `extensions.go` | 586 | ExtensionLoader impl: .local.kurel file handling |
| `resolver.go` | 504 | Resolver impl: variable substitution, cycle detection |
| `loader.go` | 496 | PackageLoader impl: definition/resource/patch loading |
| `builder.go` | 492 | Builder impl: output generation, file writing |
| `errors.go` | 311 | LoadErrors type, error formatting |
| `types.go` | 207 | Data types: Resource, Patch, PackageDefinition, etc. |
| `options.go` | 183 | LauncherOptions, BuildOptions, enum types |
| `interfaces.go` | 137 | 9 interfaces: PackageLoader, Resolver, PatchProcessor, etc. |
| `deepcopy.go` | 97 | Deep copy helper functions |
| `doc.go` | 91 | Package documentation |

`interfaces.go` defines 9 distinct interfaces at lines 16-137:
- `DefinitionLoader`, `ResourceLoader`, `PatchLoader`, `PackageLoader` (composite) — loading concerns
- `Resolver` — variable resolution
- `PatchProcessor` — patch handling
- `SchemaGenerator` — schema generation
- `Validator` — validation
- `Builder` — output building
- `ExtensionLoader` — extension handling
- `ProgressReporter`, `FileWriter`, `OutputWriter` — I/O abstractions

**Crane does NOT import `pkg/launcher`** (confirmed by grep returning zero results in `/home/serge/src/autops/wharf/crane`). The issue spec was wrong about this being a breaking change for crane. However, `pkg/cmd/kure/generate/`, `pkg/cmd/kure/patch.go`, `pkg/cmd/kure/initialize/init.go`, and `pkg/cmd/kurel/cmd.go` import launcher internally.

### Implementation Approach

The split should follow the interface boundaries identified in `interfaces.go`. Natural sub-packages based on cohesion analysis:

**Proposed structure:**
```
pkg/launcher/
    doc.go          — package doc (re-export convenience types)
    interfaces.go   — keep all interfaces in root for import convenience
    types.go        — shared types (Resource, Patch, PackageDefinition, etc.)
    options.go      — LauncherOptions, BuildOptions, enums
    errors.go       — LoadErrors and error types
    deepcopy.go     — deep copy helpers for shared types
    launcher.go     — NEW: re-exports / facade (optional)

    loader/
        loader.go       — packageLoader implementation (from loader.go)
        loader_test.go
        extensions.go   — extensionLoader implementation (from extensions.go)
        extensions_test.go

    resolver/
        resolver.go     — variableResolver implementation (from resolver.go)
        resolver_test.go

    patcher/
        processor.go    — patchProcessor implementation (from patch_processor.go)
        processor_test.go

    schema/
        generator.go    — schemaGenerator implementation (from schema.go)
        generator_test.go

    validate/
        validator.go    — validator implementation (from validator.go)
        validator_test.go

    builder/
        builder.go      — outputBuilder implementation (from builder.go)
        builder_test.go

    cli/
        cli.go          — CLI struct and commands (from cli.go)
        cli_test.go
```

**Implementation steps:**

1. **Phase 1: Create sub-package directories** and move implementation files. Keep `interfaces.go`, `types.go`, `options.go`, `errors.go`, `deepcopy.go` in the root `pkg/launcher/` package.

2. **Phase 2: Update imports**. Each sub-package will import `pkg/launcher` for types/interfaces. Internal consumers (`pkg/cmd/kure/generate/*.go`, `pkg/cmd/kure/patch.go`, `pkg/cmd/kurel/cmd.go`) currently use:
   - `launcher.NewPackageLoader(log)` -> `loader.NewPackageLoader(log)`
   - `launcher.NewResolver(log)` -> `resolver.NewResolver(log)`
   - `launcher.NewValidator(log)` -> `validate.NewValidator(log)`
   - `launcher.NewBuilder(log)` -> `builder.NewBuilder(log)`
   - `launcher.NewCLI(log)` -> `cli.NewCLI(log)`

3. **Phase 3: Add re-export aliases** in root `pkg/launcher/` for backward compatibility (optional, since crane doesn't import it):
   ```go
   // Deprecated: Use loader.NewPackageLoader instead.
   var NewPackageLoader = loader.NewPackageLoader
   ```

4. **Phase 4: Move tests** to their new package locations. Tests import the implementation struct directly, so they must move with their implementation.

5. **Phase 5: Update integration test** (`integration_test.go`) and benchmark test (`benchmark_test.go`) to import from the new sub-packages.

**Key decisions:**
- **Interfaces stay in root**: All 9 interfaces remain in `pkg/launcher/interfaces.go` to avoid circular imports and to provide a single import for consumers who need only the interfaces.
- **Types stay in root**: `Resource`, `Patch`, `PackageDefinition`, `ParameterMap` stay in root because all sub-packages depend on them.
- **CLI is the top-level orchestrator**: `cli.go` imports all sub-packages and wires them together. It should be the last to move.

### Refined Acceptance Criteria

- [ ] `pkg/launcher` split into 6 sub-packages: `loader`, `resolver`, `patcher`, `schema`, `validate`, `builder`
- [ ] Interfaces and shared types remain in `pkg/launcher/` root
- [ ] CLI orchestration in `pkg/launcher/cli/` (or remains in root if circular import issues arise)
- [ ] All tests pass in their new locations
- [ ] Internal consumers (`pkg/cmd/`) updated to use new import paths
- [ ] No exported API removed from `pkg/launcher/` root (backward-compatible re-exports)
- [ ] `make verify` passes
- [ ] No crane import breakage (confirmed: crane does not import `pkg/launcher`)

### Effort Estimate

- **Files touched**: ~30 (14 source + 14 test files moved + 2-4 updated consumers)
- **Lines of code**: ~+200 net (new package declarations, import updates, re-exports)
- **Risk**: high — large structural change affecting import paths across internal consumers; must ensure no circular imports between sub-packages and root types
- **Estimated time**: L

---

## Issue 12: refactor: simplify Bundle.Generate() label propagation

### Current State

The label propagation code is in `pkg/stack/bundle.go` (`/home/serge/src/autops/wharf/kure/pkg/stack/bundle.go`) at lines 89-118:

```go
func (a *Bundle) Generate() ([]*client.Object, error) {
    var resources []*client.Object
    for _, app := range a.Applications {
        addresources, err := app.Generate()
        if err != nil {
            return nil, err
        }
        resources = append(resources, addresources...)
    }

    // Propagate bundle labels to all generated resources.
    // Application-specific labels take precedence.
    if len(a.Labels) > 0 {
        for _, r := range resources {
            obj := *r          // <-- misleading: looks like a value copy
            labels := obj.GetLabels()
            if labels == nil {
                labels = make(map[string]string, len(a.Labels))
            }
            for k, v := range a.Labels {
                if _, exists := labels[k]; !exists {
                    labels[k] = v
                }
            }
            obj.SetLabels(labels)  // <-- mutates original via interface pointer
        }
    }

    return resources, nil
}
```

The issue is at line 103: `obj := *r` dereferences the `*client.Object` (a pointer to an interface). Since `client.Object` is an interface backed by a pointer to a concrete struct (e.g., `*appsv1.Deployment`), `obj` receives a copy of the interface value but the underlying concrete pointer is shared. Calling `obj.SetLabels(labels)` at line 113 modifies the original object.

The test file is `pkg/stack/bundle_test.go` (`/home/serge/src/autops/wharf/kure/pkg/stack/bundle_test.go`). The existing tests verify label propagation behavior and will confirm the refactor has no behavioral change.

### Implementation Approach

1. **File**: `pkg/stack/bundle.go` — Replace lines 101-114 with direct mutation:
   ```go
   // Propagate bundle labels to all generated resources.
   // Application-specific labels take precedence.
   // Note: We mutate *r directly because client.Object is an interface backed
   // by a pointer to the concrete K8s resource. The intermediate `obj := *r`
   // pattern was misleading -- it appeared to create a value copy but actually
   // operated on the same underlying object via the shared interface pointer.
   if len(a.Labels) > 0 {
       for _, r := range resources {
           labels := (*r).GetLabels()
           if labels == nil {
               labels = make(map[string]string, len(a.Labels))
           }
           for k, v := range a.Labels {
               if _, exists := labels[k]; !exists {
                   labels[k] = v
               }
           }
           (*r).SetLabels(labels)
       }
   }
   ```

2. **No test changes** — existing tests in `bundle_test.go` should pass unchanged, confirming behavioral equivalence.

3. **Pattern note**: The `resources` slice holds `[]*client.Object` (pointer to interface). Each `*r` is a `client.Object` (interface). Calling `(*r).GetLabels()` and `(*r).SetLabels()` is equivalent to the old code but makes the mutation explicit.

### Refined Acceptance Criteria

- [ ] `obj := *r` intermediate variable removed from `Bundle.Generate()`
- [ ] Label propagation operates directly on `(*r)` or equivalent
- [ ] Comment explains why direct mutation is correct (interface pointer semantics)
- [ ] All existing tests in `bundle_test.go` pass unchanged
- [ ] `make verify` passes

### Effort Estimate

- **Files touched**: 1 (bundle.go)
- **Lines of code**: ~+5/-4 net (replace 4 lines, add 5-line comment)
- **Risk**: low — behavioral no-op confirmed by existing tests
- **Estimated time**: XS

---

## Issue 13: chore: plan v1alpha1 API graduation path

### Current State

The `v1alpha1` package is at `/home/serge/src/autops/wharf/kure/pkg/stack/v1alpha1/` with 4,122 total lines across 10 files (5 source + 5 test):

| File | Lines | Content |
|------|-------|---------|
| `converters.go` | 409 | Type conversion functions between v1alpha1 CRDs and internal domain model |
| `converters_test.go` | 1,075 | Converter tests |
| `cluster.go` | 160 | `ClusterConfig` CRD type with `stack.gokure.dev/v1alpha1` API version |
| `cluster_test.go` | 428 | Cluster config tests |
| `bundle.go` | 326 | `BundleConfig` CRD type |
| `bundle_test.go` | 710 | Bundle config tests |
| `node.go` | 191 | `NodeConfig` CRD type |
| `node_test.go` | 292 | Node config tests |
| `registry.go` | 193 | GVK registry for v1alpha1 types |
| `registry_test.go` | 338 | Registry tests |

The `ClusterConfig` type (`cluster.go` line 12) uses `APIVersion: "stack.gokure.dev/v1alpha1"` and `Kind: "Cluster"`. It has `ConvertTo(version)` and `ConvertFrom(from)` methods (lines 130-148) that only support `v1alpha1` to `v1alpha1` conversion currently.

The package depends on `internal/gvk` for `gvk.BaseMetadata` type and uses `pkg/errors` for error handling. Crane does not appear to directly import `v1alpha1` types (grep confirms no direct import), but crane may consume them indirectly through `pkg/stack` converters.

### Implementation Approach

1. **File**: `docs/development/api-graduation-plan.md` — New planning document:

   ```markdown
   # v1alpha1 API Graduation Plan

   ## Current State
   - API version: `stack.gokure.dev/v1alpha1`
   - Types: ClusterConfig, NodeConfig, BundleConfig
   - Converters: v1alpha1 <-> internal domain model
   - Consumer: crane (indirect via pkg/stack converters)

   ## Graduation Criteria for v1beta1
   1. **Stability**: 6+ months without breaking changes to the v1alpha1 API
   2. **Coverage**: >80% test coverage on v1alpha1 types and converters
   3. **Consumers**: 2+ consumers using the API (crane + at least one external)
   4. **Completeness**: All planned builder promotions (#241) complete
   5. **Documentation**: Full API reference on gokure.dev

   ## Breaking Changes Inventory
   - Type renames: None planned
   - Package move: `pkg/stack/v1alpha1` -> `pkg/stack/v1beta1` (new package)
   - Deprecated APIs: `ConvertTo`/`ConvertFrom` signatures may change
   - New required fields: TBD after builder promotions

   ## Timeline
   - Phase 5 (current): Complete builder promotions, stabilize API
   - v1beta1 target: Q4 2026 (6 months after v1alpha1 stability milestone)
   - v1 target: Q2 2027 (6 months after v1beta1)

   ## Migration Strategy
   - v1alpha1 and v1beta1 will coexist during migration
   - Converters between versions will be provided
   - Crane migration PR will be coordinated
   ```

2. **No code changes** — planning document only.

### Refined Acceptance Criteria

- [ ] Graduation criteria documented with measurable thresholds
- [ ] Breaking changes inventory drafted
- [ ] Timeline proposed with concrete milestones
- [ ] Migration strategy outlined (coexistence, converters, coordination)
- [ ] `make verify` passes (no code changes)

### Effort Estimate

- **Files touched**: 1 (new docs file)
- **Lines of code**: ~+60
- **Risk**: low — planning document only
- **Estimated time**: S

---

## Issue 14: docs: document deepCopyBundle shallow copy behavior

### Current State

The `deepCopyBundle` function is in `pkg/stack/builders.go` (`/home/serge/src/autops/wharf/kure/pkg/stack/builders.go`) at lines 112-138:

```go
func deepCopyBundle(b *Bundle) *Bundle {
    if b == nil {
        return nil
    }
    newBundle := &Bundle{
        Name:          b.Name,
        ParentPath:    b.ParentPath,
        SourceRef:     b.SourceRef,
        Interval:      b.Interval,
        Labels:        b.Labels,
        Annotations:   b.Annotations,
        Description:   b.Description,
        Prune:         b.Prune,
        Wait:          b.Wait,
        Timeout:       b.Timeout,
        RetryInterval: b.RetryInterval,
    }
    if b.Applications != nil {
        newBundle.Applications = make([]*Application, len(b.Applications))
        copy(newBundle.Applications, b.Applications)  // <-- shallow copy of pointers
    }
    if b.DependsOn != nil {
        newBundle.DependsOn = make([]*Bundle, len(b.DependsOn))
        copy(newBundle.DependsOn, b.DependsOn)        // <-- shallow copy of pointers
    }
    return newBundle
}
```

Line 131: `copy(newBundle.Applications, b.Applications)` creates a new slice but copies the `*Application` pointers, not the `Application` objects. The `Application` struct (from `pkg/stack/application.go`) has fields `Name string`, `Namespace string`, `Config ApplicationConfig`. After the copy, both the original and the copy point to the same `Application` objects.

This is used by the copy-on-write builder pattern in `ensureOwned()` methods (lines 154-178). The builder creates deep copies when branching (`WithChild`, `WithBundle`, etc.) but shares `Application` objects across branches.

Similarly, `Labels` and `Annotations` maps at lines 121-122 are assigned by reference (not deep copied). This means label/annotation mutations on the copy affect the original. However, the builder pattern creates new bundles with empty maps, so this is only an issue for bundles cloned from existing ones.

### Implementation Approach

1. **File**: `pkg/stack/builders.go` — Add comment block above `deepCopyBundle`:
   ```go
   // deepCopyBundle creates a deep copy of a Bundle's structural fields but performs
   // a SHALLOW copy of the Applications and DependsOn slices. This means:
   //
   //   - The returned Bundle has its own slice headers (append-safe)
   //   - The *Application and *Bundle pointers are SHARED with the original
   //   - Mutating an existing Application object after branching affects both copies
   //
   // This is safe for the copy-on-write builder pattern (NewClusterBuilder) because:
   //   1. Builders only APPEND new applications to the slice (never mutate existing ones)
   //   2. Application.Config is set once at construction and not modified afterward
   //   3. The builder creates a new Bundle for each WithBundle() call
   //
   // Invariant: callers must not mutate existing *Application objects after branching.
   // If deep Application copying is ever needed, implement Application.DeepCopy().
   //
   // Note: Labels and Annotations maps are also assigned by reference. The builder
   // pattern creates new bundles with fresh maps, so this is safe in practice.
   func deepCopyBundle(b *Bundle) *Bundle {
   ```

2. **No code changes** beyond the comment.

### Refined Acceptance Criteria

- [ ] Comment on `deepCopyBundle` explaining shallow copy of `*Application` pointers
- [ ] Comment explains append-only safety invariant
- [ ] Comment notes the Labels/Annotations map reference sharing
- [ ] `make verify` passes (no code changes)

### Effort Estimate

- **Files touched**: 1 (builders.go)
- **Lines of code**: ~+14 (comment block)
- **Risk**: low — comment only
- **Estimated time**: XS

---

## Issue 15: docs: document Cluster getter/setter duality

### Current State

The `Cluster` type in `pkg/stack/cluster.go` (`/home/serge/src/autops/wharf/kure/pkg/stack/cluster.go`) has exported fields and matching getter/setter methods at lines 9-79:

```go
type Cluster struct {
    Name   string        `yaml:"name"`
    Node   *Node         `yaml:"node,omitempty"`
    GitOps *GitOpsConfig `yaml:"gitops,omitempty"`
}

// Getters
func (c *Cluster) GetName() string          { return c.Name }
func (c *Cluster) GetNode() *Node           { return c.Node }
func (c *Cluster) GetGitOps() *GitOpsConfig { return c.GitOps }

// Setters
func (c *Cluster) SetName(n string)          { c.Name = n }
func (c *Cluster) SetNode(t *Node)           { c.Node = t }
func (c *Cluster) SetGitOps(g *GitOpsConfig) { c.GitOps = g }
```

The same pattern exists for `Node` at lines 81-93:
```go
func (n *Node) GetName() string                         { return n.Name }
func (n *Node) GetChildren() []*Node                    { return n.Children }
func (n *Node) GetPackageRef() *schema.GroupVersionKind { return n.PackageRef }
func (n *Node) GetBundle() *Bundle                      { return n.Bundle }
// ... and matching setters
```

The getters/setters add no validation -- they are pure pass-through. The `Bundle` type in `bundle.go` does NOT have this duality (only `GetParent`/`SetParent`/`GetPath`/`GetParentPath` which do have logic beyond simple field access).

The `doc.go` for the stack package is at `/home/serge/src/autops/wharf/kure/pkg/stack/doc.go`.

### Implementation Approach

1. **File**: `pkg/stack/cluster.go` — Add a comment block above the getter/setter section:
   ```go
   // Getters and setters provide encapsulated access to Cluster fields.
   //
   // The Cluster type exports its fields directly for two reasons:
   //   1. YAML serialization requires exported fields with struct tags
   //   2. Direct field access is convenient in tests and internal code
   //
   // The getter/setter methods exist for library consumers who prefer
   // encapsulated access and may benefit from future validation additions.
   //
   // Guidelines for new code:
   //   - Public API and library consumers: prefer setters (e.g., c.SetName("prod"))
   //   - Tests and internal code: direct field access is acceptable (e.g., c.Name = "prod")
   //   - New fields: always add both exported field and getter/setter pair for consistency

   // GetName Helper getters.
   func (c *Cluster) GetName() string          { return c.Name }
   ```

2. **File**: `pkg/stack/cluster.go` — Add similar comment above Node getter/setter section:
   ```go
   // Node getters provide read access to node fields.
   // See Cluster type documentation for the getter/setter rationale.
   ```

3. **No code changes** — documentation only.

### Refined Acceptance Criteria

- [ ] Comment on `Cluster` getter/setter section explaining the dual access pattern
- [ ] Comment provides guidance for new code (setters for public API, fields for tests)
- [ ] Matching comment on `Node` getter/setter section
- [ ] `make verify` passes (no code changes)

### Effort Estimate

- **Files touched**: 1 (cluster.go)
- **Lines of code**: ~+15 (comments)
- **Risk**: low — comments only
- **Estimated time**: XS
