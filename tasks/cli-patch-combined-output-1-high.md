# Task: Add Combined-Output Mode to kure patch

**Priority:** 1 - High (Short Term: 2-3 days)
**Category:** cli
**Status:** Not Started
**Dependencies:** None
**Blocked By:** None

---

## Overview

Add a combined-output mode to `kure patch` that applies many patches to produce a single output file, clarifying the current per-patch output behavior.

## Current Behavior

`kure patch` currently outputs patched resources separately:

```bash
kure patch base.yaml patch1.kpatch patch2.kpatch
# Creates: base-patched-1.yaml, base-patched-2.yaml
```

This is useful for debugging but not for production workflows where you want a single output file with all patches applied.

## Desired Behavior

```bash
# Current (separate outputs)
kure patch base.yaml patch1.kpatch patch2.kpatch

# NEW: Combined output
kure patch base.yaml patch1.kpatch patch2.kpatch --combined -o output.yaml

# NEW: Pipe to kubectl
kure patch base.yaml *.kpatch --combined | kubectl apply -f -
```

## Implementation Tasks

### 1. Add --combined Flag

**File:** `pkg/cmd/kure/patch.go`

```go
func newPatchCommand() *cobra.Command {
    var (
        outputFile string
        combined   bool  // NEW FLAG
    )

    cmd := &cobra.Command{
        Use:   "patch <base> <patch-files...>",
        Short: "Apply patches to Kubernetes manifests",
        RunE: func(cmd *cobra.Command, args []string) error {
            if len(args) < 2 {
                return fmt.Errorf("requires base file and at least one patch file")
            }

            baseFile := args[0]
            patchFiles := args[1:]

            if combined {
                return applyPatchesCombined(baseFile, patchFiles, outputFile)
            } else {
                return applyPatchesSeparate(baseFile, patchFiles, outputFile)
            }
        },
    }

    cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (stdout if not specified)")
    cmd.Flags().BoolVar(&combined, "combined", false, "Apply all patches and produce single output")  // NEW

    return cmd
}
```

### 2. Implement applyPatchesCombined

```go
func applyPatchesCombined(baseFile string, patchFiles []string, outputFile string) error {
    // Load base resources
    resources, err := loadYAMLFile(baseFile)
    if err != nil {
        return fmt.Errorf("failed to load base file: %w", err)
    }

    // Apply each patch sequentially
    for _, patchFile := range patchFiles {
        patches, err := loadPatchFile(patchFile)
        if err != nil {
            return fmt.Errorf("failed to load patch %s: %w", patchFile, err)
        }

        for _, p := range patches {
            resources, err = patch.Apply(resources, p)
            if err != nil {
                return fmt.Errorf("failed to apply patch from %s: %w", patchFile, err)
            }
        }
    }

    // Write output
    return writeOutput(resources, outputFile)
}
```

### 3. Refactor Existing Code (applyPatchesSeparate)

```go
func applyPatchesSeparate(baseFile string, patchFiles []string, outputFile string) error {
    // Existing logic - keep current behavior
    // Apply each patch and write separate output
    for i, patchFile := range patchFiles {
        // ... existing code
        out := fmt.Sprintf("%s-patched-%d.yaml", baseFile, i+1)
        if err := writeOutput(patched, out); err != nil {
            return err
        }
    }
    return nil
}
```

### 4. Update Help Text

```go
cmd := &cobra.Command{
    Use:   "patch <base> <patch-files...>",
    Short: "Apply patches to Kubernetes manifests",
    Long: `Apply patches to Kubernetes YAML manifests.

By default, each patch file is applied separately and creates its own output.
Use --combined to apply all patches sequentially to produce a single output.

Examples:
  # Apply patches separately (default)
  kure patch deployment.yaml patch1.kpatch patch2.kpatch

  # Apply all patches and produce single output
  kure patch deployment.yaml *.kpatch --combined -o final.yaml

  # Pipe to kubectl
  kure patch base.yaml patches/*.kpatch --combined | kubectl apply -f -
`,
}
```

## Testing Requirements

### 1. Unit Tests

```go
func TestApplyPatchesCombined(t *testing.T) {
    // Create test files
    baseFile := createTestYAML(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 1
`)

    patch1 := createTestPatch(t, `
[deployment.test]
spec.replicas: 3
`)

    patch2 := createTestPatch(t, `
[deployment.test]
spec.template.metadata.labels.version: "v2"
`)

    // Apply combined
    output := filepath.Join(t.TempDir(), "output.yaml")
    err := applyPatchesCombined(baseFile, []string{patch1, patch2}, output)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Verify output
    data, _ := os.ReadFile(output)
    if !strings.Contains(string(data), "replicas: 3") {
        t.Error("patch1 not applied")
    }
    if !strings.Contains(string(data), "version: v2") {
        t.Error("patch2 not applied")
    }
}
```

### 2. Integration Test

```bash
# Create test files
cat > base.yaml <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:1.14

## UX Clarifications

### Document Separators
- Preserve YAML document separators (`---`) between resources
- Ensure multi-document YAML is valid for `kubectl apply -f -`

### Grouping Options
Add `--group-by` flag:
- `none` (default): Apply order as encountered
- `file`: Group by source file
- `kind`: Group by Kubernetes kind (Namespace, Deployment, Service, etc.)

### Deterministic Order
- Sort resources within groups alphabetically by namespace/name
- Ensures reproducible output for git diffs

### Implementation
```go
cmd.Flags().StringVar(&groupBy, "group-by", "none", "Group resources (none|file|kind)")
```

## Definition of Done

### Code Artifacts
- [ ] `--combined` flag implemented in `pkg/cmd/kure/patch.go`
- [ ] `applyPatchesCombined()` applies patches sequentially
- [ ] `applyPatchesSeparate()` maintains backward compatibility
- [ ] Document separators (`---`) preserved in output
- [ ] `--group-by` flag for resource ordering
- [ ] Deterministic output (sorted within groups)

### Testing
- [ ] Unit test: Combined mode applies all patches correctly
- [ ] Unit test: Separate mode unchanged (backward compat)
- [ ] Unit test: Document separators preserved
- [ ] Integration test: Output pipes to `kubectl apply --dry-run=client`
- [ ] Integration test: --group-by produces expected order

### Documentation
- [ ] Help text updated with --combined examples
- [ ] examples/patches/README.md includes combined mode usage
- [ ] Exit codes documented (0 = success, 1 = patch error)

### Out of Scope
- Conflict detection between patches (apply sequentially, fail fast)
- Interactive patch selection
- Diff preview (tracked in cli-patch-diff-option-2-medium.md)

### Acceptance Test
```bash
# Create test files
cat > deployment.yaml <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:1.14
