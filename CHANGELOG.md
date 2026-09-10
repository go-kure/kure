# Changelog

All notable changes to this project will be documented in this file.

> **Builder contract, release 1.** The builder contract (ADR-038, "thin core plus admissible
> sugar") removes several hundred `Create*`/`Set*`/`Add*` helpers and every value the constructors
> used to inject. Every removal is listed once, with the expression that replaces it, in
> [Builder contract: release 1 migration notes](https://www.gokure.dev/kure/concepts/builder-contract-release-1/):
> the deleted helpers and sub-type constructors, and the constructor and workflow defaults that no
> longer apply. Read it before upgrading across the entries below. The contract itself is
> [Kubernetes Builders](https://www.gokure.dev/kure/api-reference/kubernetes-builders/).
## [0.2.0-beta.11] - 2026-09-10

### Breaking

- Delete bare field forwarders and sub-type constructors
- Fold per-kind pod-template helpers onto PodSpec
- Drop error returns from resource and Helm values setters
- Stop sugar helpers writing fields the caller did not name
- Inline the ConfigMap helpers, drop the bulk-map ones
- Remove the per-kind label and annotation helpers
- Declare every injected default as an exported name

### Added

- Track go-kure/.github ref: via customManager + vendor-guard
- Pin external-secrets to upstream releases, not main HEAD
- Detect the govulncheck env pin and keep golangci-lint pins in sync
- Add LayoutIntentAugmenter companion interface for layout placement intent
- Add pkg/versions, a stable Go API over versions.yaml metadata
- Guard pkg/versions/versions_gen.go against versions.yaml drift
- Mechanically enforce barman-cloud's MVS-floor claim
- Gate go-kure/.github pin bumps on their real impact
- Land the builder contract core (ADR-038)
- Derive kind scope from the pinned upstream markers
- Walk the pinned API types for gated and deprecated fields
- Publish the derived kinds, scope and maturity tables

### Build

- Update github.com/cloudnative-pg/barman-cloud digest to 5aa56cd
- Update toolchain
- Update kubernetes to v0.36.4
- Update go-kure/.github digest to 3cfa256
- Track external-secrets by release, not raw gomod
- Update module github.com/cilium/cilium to v1.20.1
- Update go-kure/.github digest to d22e3cf
- Update go-kure/.github digest to 9260c00
- Update go-kure/.github digest to 0c93c6f
- Update go-kure/.github digest to d96e444
- Update module golang.org/x/crypto to v0.55.0 [security]
- Update dependency external-secrets/external-secrets to v2.10.0
- Update toolchain
- Update kubernetes to v0.37.0
- Update go-kure/.github digest to b3513ac
- Update dependency git-cliff to v2.14.1
- Update module github.com/cloudnative-pg/machinery to v0.6.0
- Update go-kure/.github digest to 48d8390
- Update module google.golang.org/grpc to v1.83.1 [security]
- Update dependency go to v1.26.8
- Update go-kure/.github digest to d392f39
- Update module golang.org/x/crypto to v0.56.0 [security]
- Update go-kure/.github digest to abcba39
- Update go-kure/.github digest to 022fde2
- Update module golang.org/x/mod to v0.40.0 [security]
- Update go-kure/.github digest to 7d126c7
- Update module github.com/cloudnative-pg/plugin-barman-cloud to v0.15.0
- Update sigs.k8s.io
- Update go-kure/.github digest to 3f72c03
- Update go-kure/.github digest to f76dc32
- Update go-kure/.github digest to 636e9f2
- Update go-kure/.github digest to d904eef
- Update go-kure/.github digest to 5e5c8e1
- Update module golang.org/x/vuln to v1.8.0
- Update module golang.org/x/tools to v0.50.0
- Update dependency hugo to v0.166.0
- Update go-kure/.github digest to 992e9ee
- Update go-kure/.github digest to 6990dea

### CI

- Only save Go build cache from the default branch
- Syntax-check version-sync scripts; make dep-updates example version-neutral
- Drop the sensitive-file grep, which warned on every run and on cache keys

### Changed

- Delete the hand-kept scope tables, derive every scope
- Hand out copies of the generated tables, not the tables

### Dependencies

- Widen prometheus-operator supported range to 0.94

### Documentation

- Add AI agent gates (A1-A7) section
- Correct PATH-ordering troubleshooting note for pinned golangci-lint
- Fix CNPG Monitoring example to match the real Cluster/ClusterOptions API
- Strengthen the GO-2026-5377 govulncheck waiver justification
- Document the release-pinned dependency pattern
- List the four new tool-version-parity go: filter entries
- Fix golangci-lint recovery instructions to cover all four synced files
- Document pkg/versions and its regeneration
- Add external-secrets example to pkg/versions README
- Sync pkg/versions README example with 1.37 supported_range
- Note VolumeMount comparability as a public-API breaking change
- Correct controller-runtime note for the k8s v0.37.0 MVS bump
- Explain strip-ack/rerun gotcha on pin-impact-ack
- Note the strip-ack rerun gotcha in the pin-impact-ack section
- Scope pin-impact-ack rerun gotcha to same-repo PRs
- Say the prune rewrites class-shaped error-returning helpers
- Document direct field assignment in generators
- Make every removal in the ledger findable and replaceable
- Correct the CreateIngressRule replacement note
- Fix six defects the post-undraft review found
- Describe widen as YAML-only, not two-file regeneration
- Document where scope and maturity come from
- Point at the compatibility matrix instead of restating a pin
- Document that SupportedRange/Min/Max move on every widen
- Fail CI when a page names a builder function that no longer exists
- Describe a domain model and layout engine on a thin Kubernetes foundation
- Record the builder-contract design and point the changelog at the ledger
- Require a reason on every suppressing doc-api-refs marker
- Resolve documented builders per package, and scan examples/
- Name the Prune default among ARCHITECTURE's non-mechanical migrations
- Correct the cnpg config-builder error model
- Say which release-1 migrations are silent at the call site
- Close three more ways a stale builder reference resolves
- Link the contract by absolute URL from the unpublished design record
- Check Go doc comments and the ledger's replacement column
- Scan block-form GoDoc, and route a new family's import path
- Check root Markdown, and fix three stale contract claims
- Correct the sugar-class rationale and the scope-failure remedy
- Correct the new-kind recipe for cluster scope and the doc gate
- Finish the sugar-class correction across Go doc and the glossary
- Qualify the follow-up issue reference in the design record
- Record a fourth checker residual, the nested-comment marker
- Test the whole object in the new-kind recipe
- State the metadata exemption, and align the agent guidance
- Name the examples exclusion on the per-package coverage floor
- Scope the migration claim, and fix two recipe defects
- Separate the doc gate's bar from the repository's rule
- Record the unscanned agent-guidance file as a seventh residual
- Correct two overstatements on the concepts page
- Correct two claims about what this release changes
- State the xargs tolerance's limit at the site
- The config builders did change what they emit
- Replace worked examples that call a nonexistent API
- Flag spec.selector as required, not cosmetic
- Add the CronJob rows to the required-value call-out
- Scope the CronJob restart-policy loss to who it hits
- Split the call-out into rejected vs silently different
- Stop citing the one row the section declines to judge
- Correct the workflow tables that claim CI runs make targets
- Record ci.yml's real PR trigger types and its base-branch filter
- Resolve two self-contradictions this guide introduced
- Narrow the removal rationale to what was observed
- Fix four accuracy defects in the removal rationale
- Scope the removed step's uninformative-output claim to its annotation
- State what the removed step did, not what it could never do
- Cut the deleted step's autopsy down to what the page needs
- Fix the non-provider-patterns claim and the match-scope claim
- A supported pattern earns an alert, not necessarily a block
- Document the bootstrap namespace in the mapped guide
- Scope the DefaultNamespace claim to gotk mode
- Say where a widening's compatibility assessment lives

### Fixed

- Align golangci-lint pins and add a version assertion to lint
- Use the canonical golangci-lint install.sh URL
- Enforce pinned golangci-lint version instead of only printing it
- Don't mask a failed golangci-lint install behind a piped shell
- Stop tracking .claude/settings.local.json
- Preserve go.mod perms and fail loudly on missing replace-block anchor
- Guard get_gomod_pin_comment against pipefail abort
- Fix cliff.toml allow-term pragma adjacency for the platform component label
- Require k8s.io/api replace directive for pin comment, not require fallback
- Harden ref: extraction against future ambiguity
- Peel annotated tags to their commit in sync-eso-pin.sh
- Verify upstream_release_commit live, document ESO breaking change
- Distinguish unreachable-server from tag-not-found in resolve_tag_commit
- Guard sync-eso-pin.sh's ls-remote fallback under set -euo pipefail
- Gate CI on sync-eso-pin.sh changes, bound its network calls
- Correct link-check and syntax-check bugs from PR review
- Strip a possible v-prefix from the mise golangci-lint pin
- Widen syncer's sed patterns to match the checker's tolerance
- Add check-tool-versions to make precommit
- Keep govulncheck version docs in sync with ci.yml
- Anchor govulncheck doc-sync extraction, sync stale task descriptions
- Harden govulncheck doc checker against case and duplicate-pin risk
- Doc-sync gaps and CI_VAL whitespace trim for govulncheck checks
- Sed -i -E portability + CI-vs-precommit doc-sync gap
- Tolerate trailing whitespace on the ci.yml pin line
- Scope the govulncheck-docs rule to its own customManager
- Strip trailing whitespace from Makefile golangci-lint pin, document sync-tool-versions.sh
- Anchor barman-cloud to its MVS floor, range-check it
- Syntax-check the two new sync scripts; reject duplicate mise pins
- Sh-vs-bash syntax-check dispatch; tolerate mise.toml = spacing
- Tolerate TOML single-quote pins; align syncer's mise parser with the checker
- Scope mise.toml pin matching to [tools]; guard syncer against duplicate pins; fix stale doc line
- Tolerate whitespace around the [tools] table header
- Recognize TOML quoted-key [tools] table headers
- Retain the generated pkg/versions Go API on bot branches
- Don't leave a truncated versions_gen.go on generate_go_api's guard failure
- Don't clobber validate_go_api_drift's RETURN trap in generate_go_api
- Keep generated versions_gen.go world-readable (mktemp default is 0600)
- Close cross-device mv and multiline-value gaps in generate_go_api
- Guard go mod edit|yq pipe assignment under set -e
- Distinguish tool-pipe failure from confirmed missing requirement
- Apply GOWORK=off to sync-versions.sh's step-4 go mod edit call
- Bound and root-anchor the MVS-floor guard's go list probe
- Harden sync-versions.sh/vendor-guard.sh against missing timeout/mktemp
- Strengthen case 37's mktemp-guard oracle and correct case 09 doc wording
- Harden mktemp helper and correct harness docs (round 3 review)
- Fail closed on action.yml steps outside the scripts/*.sh pattern
- Stop the pin-impact gate silently swallowing its own exit code
- Guard --old/--new argument parsing and non-ahead compares
- Add pin-impact-ack override, harden multi-step/dot-segment gaps
- Recognize uses:/run: written as the step's first YAML key
- Rerun pin-impact on label change, detect compound-line source
- Scan .yaml workflows too, reject mixed deps in one run block
- Bind pin-impact-ack to the reviewed head SHA
- Skip label mutation on fork PRs, fix token auth, use immutable base SHA
- Fail closed on ack-strip failure, cover reopened PRs, tighten perms
- Re-sync external-secrets pin and widen supported_range to 2.10
- Sync CI/docs yq pins with mise.toml's 4.53.6 bump
- Sync mise.toml's own yq comment with the 4.53.6 bump
- Widen k8s supported_range to 1.37, fix VolumeMount comparability
- Wire go.mod's go directive into the mise-sync postUpgradeTasks
- Sync versions.yaml's go.current too, and reorder before generate
- Syntax-check sync-go-version.sh in CI, document its path filter, anchor its go.mod sed
- Drop dead self-substitution sed, harden go.mod directive sync
- Widen go-version postUpgradeTasks fileFilters to every workflow file
- Chain sync-versions.sh generate into make sync-go-version
- Mark GoVersion as doc-gate:trivial; trim README's hardcoded examples
- Scope the rerun-gotcha warning to synchronize/reopened reruns
- Close admission gaps, catch orphaned generated files, sync package docs
- Align the admission classifier with ADR-038 and harden the generator
- Tie class (a) to a field and trace nil through literals and initialised locals
- Root admission field writes in a parameter and order local write-backs
- Track admission provenance by object and position, not by name
- Reject result-bearing helpers in the admission test
- Build generator errors with pkg/errors
- Let generate rewrite a rendered file that lost its header
- Reject a default written next to an admitted operation
- Require caller-supplied values and reject silent nil returns
- Close two admission classifier evasions
- Drop the orphaned PVC options struct, restore a nil guard
- Make bootstrap sourceRef name the source it emits
- Stop bootstrap panicking on a nil root node
- Stop range-checking MVS-floor dependencies, add widen subcommand
- Address Codex review findings on the widen subcommand
- Guard widen against a bare --note flag and unchecked yq writes
- Address Codex third-pass review on the widen test cases
- Reject a raw commit SHA in widen's --note before writing
- Replace mm_key's collision-prone fixed-multiplier key with mm_cmp
- Compare version components as digit strings, not bash arithmetic
- Make the marker-coverage check able to fail, and record what it found
- Resolve an unmarked kind's scope from the CRD its module ships
- Refuse a half-read CRD manifest, and skip json:"-" fields
- Match the version in KindFor, and cover the table lookups
- Decide the docs root on the resolved path, not the spelling
- Read a field's stability from its own words, not its neighbours'
- Regenerate the API tables on an external-secrets re-pin
- Claim stability only where the prose claims it, and keep IsNamespacedBuiltinKind about built-ins
- Let a supplied CRD govern the scope of a custom resource
- Widen cnpg-barman-cloud-plugin and controller-runtime ranges
- Close six ways the builder-reference check could pass wrongly
- Fail the builder-reference check when its page list is incomplete
- Scope the ledger exemption to the names it removed
- License the ledger's names from a validated list, not its cells
- Widen the page set, reject nested fences, drop a hand-kept count
- Reject repeated fence markers; correct two ARCHITECTURE claims
- Stop a no-match xargs batch from killing the symbol scan
- Group breaking commits under their own section
- Match git-cliff's parsed breaking field, not the subject
- Size the test budget for the slowest package, not a typical one
- Honour DefaultNamespace in the gotk components

### Maintenance

- Drop stale ready_for_review from pr-review workflow
- Read yq and lychee versions from mise.toml at run time
- Bump go-kure/.github pin to post-#136 main
- Regenerate builder tables for plugin-barman-cloud v0.15.0
- Regenerate builder tables for sigs.k8s.io bump

### Testing

- Drop TestGetKubernetes's Min!=Max assertion, redundant and brittle
- Add hermetic failure-path harness for sync-versions.sh guards
- Make the sync-versions guard case 23 able to fail
- Fix docs-drift masking in mvs-floor/range/notes guard cases, scope case-32's sed, add mise yq pin, sync ci docs
- Clear ambient GOWORK in harness runner, make fixture edits BSD-sed portable
- Harden guard-test harness against mktemp failure and missing seq; wire into make precommit
- Fix full-scope confirm-pass findings (mktemp in _run, seq in cases 21/22, ci coverage, case-09 flakiness)
- Give the self-reference test a deadline it can fail on
- Fail rather than panic when a table round trip loses rows
- Assert the gate copy-out on names, not on a count that cannot move

## [0.2.0-beta.10] - 2026-08-23

### Added

- Add release-identity RenderOption to RenderChart (the downstream operator#378)

### Build

- Bump Go to 1.26.6 for stdlib CVE fixes
- Migrate dependency updates from Dependabot to Renovate

### CI

- Re-trigger CI when a draft PR is marked ready for review
- Group the fluxcd ecosystem and fail check on stale compatibility docs
- Run the version guards for version-metadata-only PRs
- Add merge_group trigger to pr-review.yml
- Run checks + AI review on draft PRs (GitLab mr-review parity)
- Bump govulncheck to v1.7.0
- Reword the govulncheck pin comment to drop downstream names
- Remove the dead GitHub->GitLab mirror job

### Changed

- Keep Helm's ReleaseOptions out of the public RenderOption type

### Dependencies

- Group the cloudnative-pg modules
- Bump helm.sh/helm/v4 from 4.2.3 to 4.2.4
- Bump github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring

### Documentation

- Track cilium in versions.yaml with a 1.18 floor
- Document the doc-sync composite actions in docs-build/docs-check
- Qualify draft-review claim on rollout dependency (codex confirm-round, wording)
- Correct the documented coverage thresholds
- Correct the go.work ignore rationale
- Regenerate compatibility.md for prometheus-operator 0.93.1

### Fixed

- Consume canonical doc-sync scripts from go-kure/.github
- Fix build.sh's own dangling reference to the vendored script
- Block CI on reachable vulnerabilities
- Pin the govulncheck-gate ref; refresh the stale check-action-pins pin
- Address govulncheck-gate review findings on the security job
- Reword the GitLab-parity note to avoid the downstream repo name
- Keep ready_for_review in pr-review caller (A6 finding)
- Make CI safety checks signal correctly
- Reject invalid coverage profiles

### Maintenance

- Pin third-party actions to commit SHAs
- Pin first-party composite actions to a commit SHA
- Bump tj-actions/changed-files in the actions group
- Git-ignore go.work and go.work.sum
- Enforce downstream-reference policy

### Testing

- Temporary reproducer proving the vulnerability gate blocks

## [0.2.0-beta.9] - 2026-08-03

### Breaking

- Bump prometheus-operator monitoring API to v0.93.0

### Dependencies

- Bump cilium 1.20.0, plugin-barman-cloud 0.14.0, cert-manager 1.21.1

### Fixed

- Align unmapped-package message with the documented convention
- Drop the lychee invocation change, keep the Requires note
- Correct ResourceSet assertions so the package compiles

### Maintenance

- Bump actions/setup-go from 6 to 7 in the actions group

## [0.2.0-beta.8] - 2026-08-03

### Dependencies

- Bump oras.land/oras-go/v2 from 2.6.1 to 2.6.2
- Bump google.golang.org/grpc from 1.81.1 to 1.82.1
- Bump Flux ecosystem, cilium, gateway-api and k8s.io to v0.36.3

## [0.2.0-beta.7] - 2026-07-13

### Added

- Add prerelease bump scope

### Dependencies

- Bump github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring
- Bump helm.sh/helm/v4 from 4.2.2 to 4.2.3
- Bump fluxcd ecosystem to 2.9.1 / controllers 1.9.2
- Bump cert-manager 1.21, gateway-api 1.6, cnpg 1.30

### Fixed

- Run tag-collision checks in preview + guard next dev version
- Resolve section-nav links under the version slot
- Make README cross-package links root-absolute

### Maintenance

- Bump Go to 1.26.5

### Performance

- Source-aware Go build cache, split by job purpose

## [0.2.0-beta.6] - 2026-07-01

### Added

- Single-source docs map with blocking sync enforcement
- Enforce link integrity + docs-with-code gate; agentic cascade
- Adopt Flux 2.9 API set (#607)

### CI

- Cap govulncheck memory with GOMEMLIMIT to avoid runner OOM

### Dependencies

- Bump go.universe.tf/metallb from 0.16.0 to 0.16.1
- Sync metallb 0.16.1 in versions.yaml and compatibility docs
- Bump github.com/backube/volsync from 0.15.0 to 0.16.0
- Bump github.com/cloudnative-pg/plugin-barman-cloud
- Widen cnpg-barman-cloud supported_range to 0.13
- Bump github.com/cert-manager/cert-manager
- Bump flux-operator to v0.53.0 + re-vendor install bundle
- Bump prometheus-operator to 0.92 + widen supported_range
- Bump github.com/cilium/cilium from 1.19.4 to 1.19.5

### Documentation

- Remove stale references to removed generators package
- Correct backend to claude-max-proxy:3456
- Drop hardcoded versions from the illustrative example

### Fixed

- Harden check-doc-sync; adopt canonical validator
- Derive build versions from go.mod, guard supported_range (#593)

### Maintenance

- Bump codecov/codecov-action from 6 to 7 in the actions group
- Bump the actions group across 1 directory with 2 updates

## [0.2.0-beta.5] - 2026-06-03

### Added

- Add shared CRD/scope classifier

### CI

- Add merge_group trigger and harden change detection for merge queue

### Dependencies

- Bump github.com/cloudnative-pg/machinery

### Maintenance

- Remove rebase-check job and auto-rebase workflow

## [0.2.0-beta.4] - 2026-06-02

### Fixed

- Collapse cluster-name double-nesting when ClusterName == root node name

### Maintenance

- Set dependabot rebase-strategy to auto

## [0.2.0-beta.3] - 2026-05-28

### Breaking

- Add FluxIntegratedPerBundle placement mode

### Fixed

- Gate umbrella-child augmenter CRs by per-bundle mode

## [0.2.0-beta.2] - 2026-05-28

### CI

- Redesign release workflows — Create/Promote/Bump/Publish
- Raise coverage threshold to 90%, document targets in AGENTS.md
- Enforce per-package 90% coverage gate; fix smoke-test assertion

### Documentation

- Remove deprecated kurel/CLI content from docs site
- Rewrite homepage with improved UX and structure
- Remove stale generator registry references

### Fixed

- Emit CRs for augmenter-added sub-layouts of umbrella children
- Correct broken and wrong API examples across package docs
- Add missing SetKustomizationPrune to quickstart example
- Improve error messages and docs accuracy in release tooling

### Maintenance

- Remove pkg/stack/generators and ApplicationWrapper
- Tidy go.mod after removing pkg/stack/generators

### Testing

- Cover WorkflowEngine delegation methods, EngineWithConfig, init factory, GetName/GetVersion
- Cover RenewBefore setter, Issuer/ClusterIssuer annotations, sealed interface markers
- Cover init factory invocation and CreateLayoutWithResources branches
- Cover generateValues and processExtension via GeneratePackageFiles
- Table-driven validation tests covering validateResourceSource, Dependency, ValuesConfig, Extension, Patch
- Filesystem-backed tests for gatherResourcesFromSource and generateKurelYAML
- Raise coverage from 83.1% to 90.2%
- Cover renderOCI error path, renderHTTP error paths, corrupt archive
- Cover MarshalKustomization and SetPodSpec
- Cover gap functions to reach 95% total coverage

## [0.2.0-beta.1] - 2026-05-27

### Breaking

- Rules.FluxPlacement is the sole controller of placement

## [0.2.0-beta.0] - 2026-05-27

### Added

- Add DependsOn field to ManifestLayout
- Add createKustomizationForLayout to ResourceGenerator
- Emit Flux CRs for augmenter-added child layouts in FluxIntegrated mode
- Add validateSourceRefsForFluxIntegrated
- Call validateSourceRefsForFluxIntegrated in CreateLayoutWithResources

### Dependencies

- Bump Flux 2.8.8 and MetalLB 0.16.0 (coordinated batch)

### Documentation

- Document augmenter child layout Flux CR generation, DependsOn, and naming constraint
- Document early SourceRef validation in FluxIntegrated mode

### Fixed

- Skip duplicate kustomization.yaml entries for FluxIntegrated children
- Recurse into grandchildren even when direct child already has a CR

### Testing

- End-to-end tests for augmenter child layout CR generation
- Add SourceRef to existing FluxIntegrated fixtures
- White-box unit tests for validateSourceRefsForFluxIntegrated
- Integration tests for SourceRef validation wiring in CreateLayoutWithResources

## [0.2.0-alpha.10] - 2026-05-26

### Added

- App-scoped augmentation in nodeOnly and cluster-name paths
- App-scoped augmentation in WalkClusterByPackage nodeOnly path

## [0.2.0-alpha.9] - 2026-05-24

### Fixed

- Raise default reconciliation interval to 60m

### Testing

- Update default interval assertions to 60m

## [0.2.0-alpha.8] - 2026-05-21

### Fixed

- Skip NOTES.txt in assembleManifests

## [0.2.0-alpha.7] - 2026-05-21

### Fixed

- Cliff.toml skips release-bot commits; group chore as Maintenance; add alpha.6 CHANGELOG section

## [0.2.0-alpha.6] - 2026-05-21

### Maintenance

- Add goreleaser config for library-only release

## [0.2.0-alpha.5] - 2026-05-21

### Added

- Add GitRepository setter gaps
- Add Kustomization setter gaps
- Add HelmRelease Install/Upgrade flag setters
- Add ExternalArtifact builder
- Add ArtifactGenerator builder (source-watcher)
- Add NamedDependsOn to Bundle for name-based kustomization chaining
- Add SplitByHookWeight utility for helm hook-phase ordering
- Add SetHelmReleaseValuesFromMap; add HelmRelease golden test
- Extend RenderChart to support HTTP Helm repositories

### Changed

- Remove CLI layer (cmd/, pkg/cmd/, pkg/cli/)

### Dependencies

- Upgrade flux-operator to v0.48.0
- Bump sigs.k8s.io/controller-runtime
- Bump github.com/cilium/cilium from 1.19.3 to 1.19.4
- Bump github.com/google/cel-go from 0.28.0 to 0.28.1

### Documentation

- Update README for new CRDs and setter coverage
- Add Organization Resources section referencing go-kure/.github
- Replace oci-layout.md with redirect to go-kure/.github
- Mark generators deprecated and pkg/patch moved to launcher
- Remove stale pkg/patch references from ARCHITECTURE.md
- Document HTTP RenderChart as public-only; explain Helm v4.2.0 bump

### Fixed

- Address PR review findings — stale site content and doc cleanup
- Bump cnpg from 1.29.0 to 1.29.1 (CVE-2026-44477)
- Bump prometheus-operator from 0.90.1 to 0.91.0
- Upgrade FluxCD ecosystem to v2.8.7 (CVE-2026-45022)
- Remove SetHelmReleaseValuesFromMap from README (not yet added); fix sourcev1 import in guide
- Pre-release review — deepCopyBundle missing fields, renderHTTP guard, coverage gaps, docs

### Maintenance

- Tidy go.sum
- Update controller-runtime to 0.24.1 in versions.yaml
- Regenerate compatibility.md for controller-runtime 0.24.1

### Testing

- Add HelmRepository golden tests; rewrite pkg/kubernetes/fluxcd README for current API

## [0.2.0-alpha.4] - 2026-05-11

### Breaking

- Nil-receiver setters panic instead of returning error
- Migrate CRD subpackage constructors to CreateX pattern

### Added

- Add actions:read permission to release-create workflow

### CI

- Route cache through in-cluster server, rename runner label
- Explicitly set ACTIONS_CACHE_URL in workflow env to override job dispatch
- Drop go build cache, use gomod- key prefix, bump test timeout
- Restore build cache, fix Hugo cache, fix govulncheck, cache tools
- Fix govulncheck blocking on findings, fix codecov file input
- Increase test job timeout from 10 min to 20 min
- Remove ineffective ACTIONS_CACHE_URL env vars, document cache routing
- Make artifact upload/download resilient on ARC runners
- Set ACTIONS_RESULTS_URL at workflow level for step env routing
- Stable gobuild cache key and expand workflow path filter

### Documentation

- Bump Go version to 1.26.3 in github-workflows.md

### Fixed

- Remove deprecated SetServiceLoadBalancerIP
- Propagate SetNestedField errors
- Complete migration by removing all stale internal package imports

### Maintenance

- Bump Go 1.26.2 → 1.26.3 for stdlib security fixes

### Testing

- Add missing tests for CRD subpackage setters
- Suppress staticcheck SA1019 on intentional deprecated field usage

## [0.2.0-alpha.3] - 2026-05-06

### Added

- Promote ConfigMap builder to public API
- Add granular setters for RefreshInterval, Target, DataFrom
- Expose GenerateFluxInstance as public method
- Add Pooler builder
- Add CreateVolumeClaimTemplate helper for StatefulSet embedding
- Add public Namespace builder with PSA label support
- Add Type field to HelmRepositoryConfig for OCI registries
- Extend HelmReleaseConfig with chartRef, valuesFrom, and remediation fields closes #496
- Extend builders for infra component use cases
- Add CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy, and CiliumCIDRGroup builders
- Add builders for remaining stable Cilium CRDs (#514)

### Changed

- Wrap PgBouncerSpec in domain PgBouncerOptions

### Documentation

- Add the downstream operator state analysis review (2026-05-06)
- Note that GenerateFluxInstance does not check config.Enabled
- Add Namespace builder section to README
- Add OCI HelmRepository example to README
- Fix variable name collision in README OCI example
- Update README with 10 new CRD builders

### Fixed

- Remove dead variable and fix README ptr usage in HelmRelease tests
- Address review findings on #499 builders
- Address review findings on #501 builders
- Add EnableDefaultDeny setters for CNP and CCNP

### Testing

- Add default label/annotation assertions in TestCreateConfigMap

## [0.2.0-alpha.2] - 2026-05-05

### Added

- Add domain config types for Cluster, Database, and ObjectStore

### Fixed

- Use kure errors package instead of fmt.Errorf

### Maintenance

- Fix gofmt alignment in cnpg types

## [0.2.0-alpha.1] - 2026-05-04

### Added

- Add configMapGenerator support to ManifestLayout
- Add ReplicationSource and ReplicationDestination builders
- Add LayoutRules.FlattenSingleTier opt-in flag

### CI

- Fix build-binaries timeout and remove make dependency
- Remove make dependency from docs-build job
- Skip apt-get in test job when build-essential already installed

### Dependencies

- Bump FluxCD ecosystem to v2.8.6 and cnpg/barman-cloud to 0.5.1
- Bump sigs.k8s.io/controller-runtime from 0.23.3 to 0.24.0
- Pin k8s.io to v0.36.0 to match controller-runtime v0.24.0
- Update versions.yaml for controller-runtime 0.24.0 and k8s 0.36.0

### Documentation

- Fix stale references to deleted stack/v1alpha1 and generator aliases
- Fix remaining stale references after stack/v1alpha1 removal

### Fixed

- Use kustomize.config.k8s.io domain in generated kustomization.yaml

### Maintenance

- Bump dorny/paths-filter from v3 to v4
- Refactor builders to sealed-interface idiom
- Remove pkg/stack/v1alpha1 and deprecated generator aliases

## [0.2.0-alpha.0] - 2026-05-01

### Added

- Move kure docs site to /kure/ subpath

### Build

- Remove kurel and extracted packages from build and docs

### CI

- Trigger Claude Code action automatically on PRs
- Replace shared workflows with callers to go-kure/.github
- Optimize pipeline — parallel jobs, single test run, Hugo cache, path filtering
- Cancel in-progress runs when new push arrives on same branch

### Fixed

- Handle large PR diffs in review workflow, fix formatting, update docs
- Set skipped output before early exits in pr-review workflow
- Add test to build gate needs to catch skipped cascade on failure
- Guard against empty Hugo version parse from mise.toml
- Add validate to build gate needs so lint failure blocks merge
- Sync hugo.toml mounts and check-mounts after package extraction
- Guard against empty Hugo version read
- Repair 5 broken links on deployed docs site
- Support unnamed root node — merge resources at cluster root

### Maintenance

- Extract pkg/launcher, pkg/patch, cmd/kurel, pkg/cmd/kurel to go-kure/launcher

### Testing

- Add missing tests for Patches/PostBuild and stable helm output
- Add OCI layout pattern tests (Namespace:"."/layer naming/3-layer structure)
- Assert spec.path and sourceRef in Layer 2 Kustomization tests

## [0.1.0-rc.11] - 2026-04-20

### Added

- Add public pkg/kubernetes/cnpg wrapper

### Dependencies

- Bump github.com/moby/spdystream from 0.5.0 to 0.5.1
- Bump github.com/cloudnative-pg/machinery
- Bump github.com/cloudnative-pg/plugin-barman-cloud

### Documentation

- Add internal design note for launcher extraction
- Correct patch.go treatment in launcher extraction design
- Document OCI folder layout and split strategy design
- Rename infra to platform in OCI layout design doc
- Register oci-layout.md in docs site scripts

### Maintenance

- Update issue template labels to post-rename names
- Update versions.yaml for plugin-barman-cloud 0.12.0

## [0.1.0-rc.10] - 2026-04-15

### Added

- Add builders for Role, RoleBinding, ClusterRole, ClusterRoleBinding

## [0.1.0-rc.9] - 2026-04-15

### Added

- Add Patches and PostBuild fields to Bundle
- Add Force and Suspend fields to Bundle

### Dependencies

- Bump github.com/cert-manager/cert-manager

### Maintenance

- Update versions.yaml for cert-manager v1.20.2

## [0.1.0-rc.8] - 2026-04-14

### Added

- Add RenderChart for client-side OCI chart rendering

### Dependencies

- Bump github.com/google/cel-go from 0.27.0 to 0.28.0

### Documentation

- Document undocumented features across package READMEs and guides
- Add prometheus-builders API reference page and fix broken links

### Fixed

- Sort manifest keys for stable output

## [0.1.0-rc.7] - 2026-04-12

### Fixed

- Honor FileNaming in WriteToDisk, WriteManifest, and package walker

## [0.1.0-rc.6] - 2026-04-12

### Added

- Propagate FileNaming to ManifestLayout and WriteToTar

### Fixed

- Force FilePerResource for FluxIntegrated kustomization refs

## [0.1.0-rc.5] - 2026-04-11

### Added

- Emit full Flux Operator install bundle in flux-operator mode

## [0.1.0-rc.4] - 2026-04-10

### Added

- Add Bundle.Children + shared cluster validator
- Wire ValidateCluster into all entry points
- Umbrella Kustomization spec generation
- Umbrella layout walker + integrated placement + v1alpha1 parity
- Add umbrella cluster demo and fix writer CR duplication

### Dependencies

- Bump github.com/fluxcd/flux2/v2 from 2.8.2 to 2.8.3
- Bump github.com/cert-manager/cert-manager
- Bump github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring
- Bump github.com/cloudnative-pg/cloudnative-pg
- Bump github.com/fluxcd/flux2/v2 from 2.8.2 to 2.8.3

### Fixed

- Bump Go to 1.26.2 for stdlib security fixes
- Switch to claude-max-proxy
- Nest child nodes under root node layout when ClusterName is set

### Maintenance

- Bump codecov/codecov-action from 5 to 6 in the actions group
- Add mise tasks for versions:check and versions:generate
- Update versions.yaml for fluxcd 2.8.5

### Testing

- Move gotk network tests to integration, use flux-operator in workflow test

## [0.1.0-rc.3] - 2026-03-22

### Added

- Upgrade cert-manager v1.19.4 → v1.20.0
- Add PSAViolationError with field paths

### Dependencies

- Bundle dependency updates

### Documentation

- Update lint job timeout from 10 to 15 minutes

### Fixed

- Map dependency-updates.md to docs site
- Increase lint job timeout to 15 minutes
- Resolve broken links on versioned doc subsites
- Disable setup-go built-in cache to prevent double-caching
- Resolve broken dependency-updates link on contributing guide page

## [0.1.0-rc.2] - 2026-03-20

### Added

- Dynamic version notice on homepage

### CI

- Run all GitHub Actions on self-hosted runner

### Changed

- Reorganize examples/ with demo/ grouping and READMEs

### Documentation

- Update CI docs and changelog for isDeepEmpty fix

### Fixed

- Patch pipeline bugs and update demo examples to AppWorkload format
- Correct containerport typo to containerPort in example YAMLs
- Pin yq version and cache Hugo modules in CI
- Add fallback for COMMIT_SHA in gen-versions-toml.sh
- Install missing tools on self-hosted runner in CI workflows
- Run apt-get update before installing make in CI workflows
- Install gcc and enable CGO for race tests on self-hosted runner
- Remove gotestfmt from unit test step to fix self-hosted runner
- Install build-essential to provide C headers for CGO
- Strip zero-value primitives in isDeepEmpty
- Add nil and []any handling to isDeepEmpty
- Install goimports before formatting check
- Increase lint timeout and scope cache to modules only
- Gate release on pre-release tests and fix CGO_ENABLED

### Maintenance

- Revert unrelated settings.json changes from ci/selfhosted-runners

## [0.1.0-rc.0] - 2026-03-09

### Changed

- Simplify Bundle.Generate() label propagation
- Standardize CRD builders to void returns
- Standardize ConfigMap and Secret builders to void returns
- Standardize validation strategy with void returns

### Dependencies

- Bump github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring
- Bump sigs.k8s.io/gateway-api from 1.4.0 to 1.5.0

### Fixed

- Simplify SetSecretImmutable and remove unnecessary Immutable pre-allocation

### Maintenance

- Sync versions.yaml for prometheus-operator v0.89.0
- Sync versions.yaml for gateway-api v1.5.0

### Testing

- Add golden file tests for InitContainer builders

## [0.1.0-beta.7] - 2026-03-08

### Added

- Add public facade package
- Add public facade package
- Add public facade package
- Support flat root output with NodeGrouping=GroupFlat (#240)
- Add configurable layout presets (#263)
- Add NetworkPolicy and HTTPRoute builders (#242)
- Add PSA security context helpers (#243)
- Add Prometheus operator builders (#354)
- Add ResourceRequirements builder (#244)
- Add {kind}-{name}.yaml file naming pattern (#266)
- Set FileNamingKindName in centralized preset LayoutRules
- Add SourceKind field to BootstrapConfig (#254)
- Implement regex pattern validation in schema
- Add configurable kustomization mode per FluxPlacement (#265)
- Add Flux 2.8 remediation and wait strategy builders (#255)
- Promote flux-operator to primary bootstrap mode (#256)
- Add remediation config to ReleaseConfig (#236)

### Documentation

- Add package README
- Add metallb to AGENTS.md reverse mapping table
- Fix version mismatches and outdated references across documentation
- Add godoc comments to namespace builder functions
- Document Provider/Alert v1beta3 blocked state (#250)
- Document k8s.io replace directives in go.mod (#291)

### Fixed

- Add missing types.go, doc.go, tests and SetClusterIssuerCA
- Skip empty ACME solvers, make ACME and CA mutually exclusive
- Append GOPATH/bin to PATH instead of prepending in Makefile
- Use predefined nil error constants in metallb builders
- Use pkg/errors instead of fmt.Errorf in patch package
- Check FluxPlacement in WriteToDisk and WriteToTar (#264)
- Centralize error sentinels and add documentation
- Avoid slice mutation and validate ephemeral containers in PSA
- Use centralized sentinel errors and add missing Prometheus helpers
- Remove trailing blank line in bootstrap generator test

### Maintenance

- Explicitly enable unused linter (#288)
- Document gosimple inclusion via staticcheck (#290)
- Align golangci-lint config with the downstream operator linter set (#293)

### Testing

- Add comprehensive tests for public facade
- Add tests and docs for externalsecrets facade

## [0.1.0-beta.6] - 2026-03-06

### Added

- Add strategic merge patch support with namespace-aware target resolution
- Add HelmRelease targetNamespace and releaseName fields
- Expose valuesFrom in FluxHelm ReleaseConfig
- Add PostRenderer Kustomize builder helpers
- Add CRD lifecycle policy fields to ReleaseConfig
- Add chartRef support to FluxHelm generator (#267)
- Allow SourceRefName override in FluxHelm generator
- Support prune protection annotation on generated resources
- Add DriftDetection builder helpers for HelmRelease
- Add HealthChecks field to Bundle
- Add mise release task with dry-run trigger wrapper
- Promote internal/gvk to pkg/gvk
- Promote 7 internal K8s builders to public API
- Add CNPG Database CR builder
- Add CNPG ObjectStore CR builder
- Add CNPG ScheduledBackup plugin method and advanced knobs
- Enable gosec linter with fixes for all violations
- Add CNPG managed roles builder

### Build

- Add gen-versions-toml.sh for Hugo config overlay
- Use versioned config overlay in docs-build
- Rework deploy-docs for multi-version deployment
- Trigger versioned docs deployment on stable release
- Add manage-docs admin workflow
- Migrate golangci-lint from v1 to v2
- Upgrade to Go 1.26.0
- Remove accidentally committed temp file
- Upgrade k8s.io dependencies to v0.35.1 (Kubernetes 1.35)
- Update versions.yaml for k8s 1.35 upgrade
- Remove broken k8s-compat CI job
- Upgrade cert-manager to v1.19.4 and metallb to v0.15.3
- Upgrade external-secrets to v1.3.2 (module path migration)

### Changed

- Replace any with typed parameters in Workflow interface

### Dependencies

- Upgrade FluxCD ecosystem to 2.8
- Bump sigs.k8s.io/controller-runtime

### Documentation

- Update branch protection docs to reflect ruleset migration
- Add version-aware banner and header display
- Add pkg.go.dev reference links to package READMEs
- Document versioned docs system in github-workflows.md
- Add 2026-02-26 deep code review
- Add action plan, issue specs, and implementation design
- Clarify AGENTS.md fmt.Errorf guidance
- Add example_test.go for CRD builder packages
- Document deepCopyBundle shallow copy behavior
- Document Cluster getter/setter duality
- Add getting-started example for Cluster-to-Disk pipeline

### Fixed

- Add release notes extraction to release workflow
- Use tab-indented code block for YAML example in doc comment
- Guard unstructured fallback from list decode panics
- Release read lock before invoking converter callback
- Fix CI lint baseline and resolve pre-existing lint issues
- Resolve 30+ broken links on gokure.dev
- Use json.Marshal for HelmRelease values encoding
- Exclude docs/development/ from unmapped docs check
- Replace gh CLI with curl in pr-review workflow
- Address AI review findings on pr-review workflow
- Enforce ChartRef validation and mutual exclusivity
- Check remote tags before releasing
- Use pkg/errors and pkg/logger in getting-started example
- Use ToClientObject helper instead of manual pointer-to-interface
- Isolate TestEnsureConfigDir from host environment
- Harden release-trigger.sh remote detection and hint output
- Remove curl|sh auto-install from lint-fast target
- Strip v prefix from pseudo-version in versions.yaml
- Add deprecation markers and update DESIGN.md references
- Add timeline notification comments for PR review updates
- Replace gh CLI with curl for timeline notice comments
- Add Content-Type header to timeline notice API calls
- Replace sticky comments with regular PR comments
- Sync controller-runtime version to 0.23.3 in versions.yaml
- Align pr-review workflow with GitLab mr-review template
- Correct yq pipe precedence in max_dependabot filter
- Migrate cosign signing to v3 bundle format
- Bump Go to 1.26.1 (security patch)
- Remove output flag and use sigstore.json extension for cosign v3
- Add release-notes.md to .gitignore

### Maintenance

- Bump the actions group with 2 updates
- Align golangci-lint config with the downstream operator standard
- Replace generic code review with two-pass PR review workflow
- Bump the actions group with 3 updates
- Add CNPG to versions.yaml and dependency governance
- Update versions.yaml for completed dependency upgrades
- Update PR review model to gpt-5.4

### Performance

- Add lint-fast Makefile target

### Testing

- Improve test coverage from 78% to 88%

### Pr-review

- Fix broken pipe and stale assessment comment

## [0.1.0-beta.0] - 2026-02-17

### Added

- Expose HPA helpers in pkg/kubernetes
- Expose PDB helpers in pkg/kubernetes
- Add deterministic YAML serialization option
- Expose Deployment, Service, Ingress helpers in pkg/kubernetes
- Expose CronJob helpers in pkg/kubernetes
- Add optional Validator interface for ApplicationConfig
- Add unstructured fallback for unknown GVKs
- Implement Generate() for stack pipeline integration
- Add comprehensive server-set field stripping (#196)
- Add kure init scaffolding command (#136)
- Rewrite fluent builders with immutable copy semantics (#139)
- Migrate release automation from semver.sh to CI-driven release.sh

### Changed

- Consolidate generator registries into pkg/stack (#179)

### Dependencies

- Bump sigs.k8s.io/kustomize/api in the k8s-ecosystem group

### Documentation

- Add implementation workflow checklist
- Document ApplicationConfig breaking change (#178)

## [0.1.0-alpha.3] - 2026-02-12

### Documentation

- Add changelog entry for v0.1.0-alpha.3

### Fixed

- Install syft in release workflow for SBOM generation

## [0.1.0-alpha.2] - 2026-02-12

### Added

- Deterministic kustomization.yaml ordering
- Add missing Bundle fields (Prune, Wait, Timeout, etc.)
- Clean YAML output in EncodeObjectsToYAML by default
- Implement createSource() for OCIRepository/GitRepository
- Add WriteToTar(io.Writer) for in-memory layout generation
- Propagate Bundle.Labels to all generated resources
- Rename CI job names to match branch protection check names
- Add Hugo documentation site with CI/CD and mise tasks
- Add auto-rebase workflow and rebase-check job

### Build

- Improve release workflow security and reproducibility

### CI

- Add GitLab mirror push after all checks pass
- Add divergence detection and tag sync to GitLab mirror

### Dependencies

- Bump github.com/google/cel-go from 0.26.1 to 0.27.0

### Documentation

- Archive completed PLAN.md to docs/history/
- Streamline README as landing page with badges
- Use shields.io badge for Go Report Card
- Restructure site around user needs with code-synced READMEs

### Fixed

- Use git-cliff for changelog generation in release script
- Bump Go 1.24.12 → 1.24.13, add govulncheck summary to CI
- Use path-based matching in findLayoutNode()
- Anchor GO_VERSION patterns to avoid matching HUGO_VERSION
- Add rollup build gate job to satisfy branch protection check

### Maintenance

- Use individual usernames in CODEOWNERS

## [0.1.0-alpha.1] - 2026-01-30

### Fixed

- Run tests directly in release workflow instead of checking CI status

## [0.1.0-alpha.0] - 2026-01-30

### Added

- Add storageclass helpers
- Add kustomize helpers
- Add flux source helpers
- Add helpers
- Add fluxcd builders package
- Add layout grouping and app file mode
- Support file- and dir-per-application layouts
- Implement OCI artifact separation in layout system
- Implement GitOps bootstrap and refactor demo system to data-driven architecture
- Implement comprehensive Kubernetes printer wrappers in io module
- Modernize error handling with custom error types and standardization
- Implement professional Cobra CLI with comprehensive command structure
- Implement comprehensive structured error handling system
- Add shorthand flags for common CLI options across all commands
- Complete kurel package system design documentation
- Implement package loader with hybrid error handling
- Implement variable resolver with cycle detection
- Implement patch processor with dependency resolution
- Implement schema generation and validation for launcher
- Complete Phase 4 - schema generation and validation
- Complete Phase 5 - output builder and local extensions
- Implement Phase 6 - CLI command integration
- Implement Phase 7 - comprehensive integration tests
- Implement GVK-based ApplicationConfig generator system
- Implement GVK-based versioning for stack module structs
- Implement GVK-based versioning for stack module structs
- Add comprehensive Makefile and CI/CD pipeline
- Complete KurelPackage generator implementation
- Enable Kubernetes schema inclusion in kurel CLI
- Implement fluent builder pattern Phase 1
- Implement comprehensive interval validation for GitOps configurations
- Add Go version management tools
- Add fast precommit target for git hooks
- Add PodDisruptionBudget builder
- Add HorizontalPodAutoscaler builder
- Add combined-output mode to kure patch
- Add --diff option to kure patch

### Build

- Update Go to 1.24.12 to fix govulncheck vulnerabilities
- Automate changelog generation with git-cliff

### CI

- Add GitHub Action to refresh Go proxy on main branch commits
- Enforce Go version consistency in PR checks
- Remove Qodana workflow due to licensing issues
- Fix security scan action to use official gosec action
- Remove gosec security scan (CodeQL provides coverage)

### Changed

- Loop over YAML prints
- Split appsets module
- Export ApplyPatch
- Register k8s schemes on demand
- Move pkg/layout to pkg/stack/layout for better organization
- Move pkg/fluxcd to pkg/k8s/fluxcd for better organization
- Yaml dir naming and proper marshalling
- Modernize errors package to follow Go best practices
- Modernize patch module with clean syntax and comprehensive tooling
- Rename cmd/patch to cmd/kure for better CLI naming
- Promote patch command from subcommand to top-level command
- Rename .patch files to .kpatch to avoid conflicts with diff patches
- Eliminate circular references in Node and Bundle structures
- Centralize validation logic across Kubernetes builders
- Standardize error handling to use KureError consistently
- Standardize function naming conventions across codebase
- Multi-CLI architecture and package naming standardization
- Implement clean workflow interface architecture
- Implement launcher base types with shared libraries
- Implement shared internal/gvk infrastructure
- Apply go fmt formatting to codebase
- Simplify Claude settings with symlink and expanded permissions
- Reorganize task files with numbered prefixes
- Migrate to GoReleaser v2 workflow
- Consolidate Makefile targets and enhance dev workflow
- Standardize validation patterns across packages
- Consolidate 4 GitHub workflows into 2 (ci.yml + release.yml)
- Consolidate 4 GitHub workflows into 2 (ci.yml + release.yml)
- Improve pkg/kubernetes testability and coverage

### Dependencies

- Align k8s.io/cli-runtime to v0.33.2 to match replace directive
- Bump tj-actions/changed-files
- Bump github.com/external-secrets/external-secrets
- Bump sigs.k8s.io/kustomize/api from 0.20.0 to 0.21.0
- Bump sigs.k8s.io/yaml from 1.5.0 to 1.6.0
- Implement centralized dependency version management
- Document blocked dependency updates for Go 1.25
- Bump github.com/spf13/cobra in the go-safe group
- Bump github.com/cert-manager/cert-manager
- Update versions.yaml for cert-manager 1.16.5

### Documentation

- Add project README
- Mention base resources and expose constructor
- Expand kio package documentation
- Expand kio documentation
- Expand fluxcd package overview
- Correct Flux auto-generated kustomization details
- Update README to reflect current repository state
- Add comprehensive architectural documentation
- Add comprehensive architectural documentation for generators
- Add comprehensive UX design document and recommendations
- Update project status and document remaining features
- Add comprehensive plugin architecture design
- Update CLAUDE.md with current project priorities and status
- Update CLAUDE.md with current project status and accurate metrics
- Update user documentation with current project state
- Add detailed explanation of CEL Validation Enhancement task
- Add comprehensive repository review and task management system
- Update task statuses after upstream rebase
- Add comprehensive puzl-cloud/kubesdk review with kure comparison
- Add task #1 for CEL validation enhancement
- Add workflow guidelines to tasks.md
- Remove references to non-existent demo-internals make target
- Add HPA and PDB builder tasks for the downstream operator OAM support
- Add the downstream operator integration documentation
- Add tasks README and update task 03 status
- Add quickstart guide
- Expand README with end-to-end examples
- Mark high-priority tasks 1-5, 23, 24 as completed
- Mark task #8 as completed
- Add comprehensive GoDoc documentation
- Mark task #10 as completed
- Mark task #6 as completed
- Mark tasks #7, #9, #11, #12 as completed

### Fixed

- Separate helper group comments
- Add missing unstructured import to patch CLI
- Correct type usage in generators package tests
- Resolve all layout module test failures
- Ensure all manifest directories have kustomization.yaml for GitOps compliance
- Resolve test failures in launcher module
- Resolve CLI test output capture issues
- Resolve all failing tests and improve TOML patch support
- Correct appworkload test to match ServiceConfig structure
- Update demo and kure commands to use new GVK-based ApplicationWrapper
- Resolve intermittent test failures in cmd/demo package
- Resolve stdout capture synchronization in demo tests
- Configure golangci-lint compatibility and resolve linting issues
- Correct YAML structure in CI workflow
- Add goimports to make fmt for CI/local parity
- Add GOPATH/bin to PATH in lint and fmt targets
- Upgrade Go to 1.24.11 to resolve security vulnerabilities
- Make max_depth_exceeded test deterministic
- Fix CVE in mapstructure and add workflow permissions
- Resolve repo issues across docs, CI, validation, and caching
- Propagate --strict flag to validator in kurel validate
- Update K8s compatibility matrix to test supported versions
- Remove K8s 0.33 from CI compatibility matrix
- Align mise.toml Go version with CI workflows
- Lower coverage threshold to 70% to match current main coverage
- Improve dependabot wildcard pattern matching in validation
- Block FluxCD major version updates in dependabot

### Maintenance

- Log errors via log package
- Update Claude settings - always save to .claude/settings.json
- Go fmt
- Standardize Go version and improve workflow organization
- Align repo with the downstream operator scaffold and the downstream operator standards
- Enhance dependabot configuration
- Migrate tasks to GitHub issues
- Bump the actions group across 1 directory with 7 updates

### Testing

- Check errors
- Add runCluster coverage
- Add comprehensive test coverage for all packages
- Add comprehensive test coverage for FluxHelm internal package
- Skip demo integration tests in short mode
- Skip demo tests when examples directory is missing
- Fix data race in TestMainFunction
- Skip max_depth_exceeded test due to resolver bugs
- Add integration tests for stack generation workflows
- Add fuzz tests for patch parser
- Add Kubernetes version matrix to CI
- Add tests to improve coverage and fix Go version
- Add Phase 1 coverage for simple getters/setters
- Add Phase 2 parsing tests, reach 70.5% coverage
- Add Phase 3 validation tests, reach 100% validation coverage
- Add Phase 4 stack domain model tests
- Add Phase 5 layout integrator tests
- Add wrapper function tests, reach 94.8% gvk coverage
- Add setter function tests for internal packages
- Add comprehensive IO table and printer tests
- Add comprehensive appworkload internal tests


