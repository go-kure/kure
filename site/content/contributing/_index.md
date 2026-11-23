+++
title = "Contributing"
weight = 70
+++

Kure is a Go library that describes a GitOps repository as a model and writes it as plain YAML. This section
is for people who change it. Pick the route that matches your task.

## Where to start

The [Development Guide](/contributing/guide/) covers setup, the build, the tests and code quality.
[Releasing](/contributing/releasing/) describes how a release is cut, and
[Dependency Updates](/contributing/dependency-updates/) how pinned dependencies are bumped. The CI jobs are in
[GitHub Workflows](/contributing/github-workflows/) and [Shared Workflows](/contributing/shared-workflows/).

## Find your way around the code

Read the [architecture map](https://github.com/go-kure/kure/blob/main/ARCHITECTURE.md) first. It lists every
package with what it owns, shows how data moves from a cluster model to files on disk, names the rules the code
keeps and where each is enforced, and says where to start reading.

For the detail, use the [Architecture](/concepts/architecture/) reference, the
[domain model](/concepts/domain-model/) and the [builder contract](/concepts/builder-contract/). The site's
API reference covers the packages, except `pkg/gvk`, `pkg/stack/argocd` and `pkg/stack/helm`: read their
`README.md` in the repository.

## Extend Kure

| You want to add | Start here |
|---|---|
| Support for a new Kubernetes kind | [Adding support for a new kind](/concepts/architecture/#adding-support-for-a-new-kind) |
| A change to the domain model | [Extending the domain model](/concepts/architecture/#extending-domain-model) |
| A new GitOps workflow | [Implementing new GitOps workflows](/concepts/architecture/#implementing-new-gitops-workflows) |

## Keep the documents in step

A change to a package updates that package's `README.md` and any page that describes it, in the same pull
request. A change that adds or moves a package also updates the Map in `ARCHITECTURE.md`.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/go-kure/kure.git
cd kure

# Install tools
make tools

# Run checks
make check

# Run full pre-commit validation
make precommit
```

## Branch Workflow

`main` is protected. Create a feature branch:

```bash
git checkout -b feat/my-feature main
# make changes
git push -u origin feat/my-feature
gh pr create
```

Required CI checks: `lint`, `test`, `build`, `pr-review / AI Code Review`.
