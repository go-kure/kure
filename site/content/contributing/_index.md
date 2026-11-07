+++
title = "Contributing"
weight = 70
+++

# Contributing

Resources for contributing to Kure.

- [Development Guide](/contributing/guide/) - Setup, testing, code quality, and CI/CD workflows
- [Releasing](/contributing/releasing/) - How a release is cut
- [GitHub Workflows](/contributing/github-workflows/) - CI/CD pipeline documentation
- [Shared Workflows](/contributing/shared-workflows/) - PR review, merge queue, shared CI checks and docs deploy
- [Dependency Updates](/contributing/dependency-updates/) - How pinned dependencies are bumped

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
