package kure

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/patch"
)

// PatchOptions contains options for the patch command
type PatchOptions struct {
	// Input options
	BaseFile   string
	PatchFiles []string
	PatchDir   string

	// Output options
	OutputFile string
	OutputDir  string

	// Patch options
	ValidateOnly bool
	Interactive  bool
	Combined     bool
	Diff         bool
	GroupBy      string

	// Dependencies
	Factory   cli.Factory
	IOStreams cli.IOStreams
}

// NewPatchCommand creates the top-level patch command
func NewPatchCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	// Create factory for dependency injection
	factory := cli.NewFactory(globalOpts)
	o := &PatchOptions{
		Factory:   factory,
		IOStreams: factory.IOStreams(),
	}

	cmd := &cobra.Command{
		Use:   "patch [flags] BASE_FILE PATCH_FILE...",
		Short: "Apply patches to Kubernetes manifests",
		Long: `Apply patches to existing Kubernetes manifests using Kure's patch system.

This command applies declarative patches to base YAML files containing
Kubernetes resources, supporting JSONPath-based modifications.

Examples:
  # Apply single patch to base file
  kure patch base.yaml patch.yaml

  # Apply multiple patches
  kure patch base.yaml patch1.yaml patch2.yaml patch3.yaml

  # Apply all patches from directory
  kure patch --patch-dir ./patches base.yaml

  # Validate patches without applying
  kure patch --validate-only base.yaml patch.yaml

  # Apply patches interactively
  kure patch --interactive base.yaml

  # Preview changes without applying (diff mode)
  kure patch --diff base.yaml patch.yaml`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.BaseFile = args[0]
			if len(args) > 1 {
				o.PatchFiles = args[1:]
			}

			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run()
		},
	}

	// Add flags
	o.AddFlags(cmd.Flags())

	return cmd
}

// AddFlags adds flags to the command
func (o *PatchOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.PatchDir, "patch-dir", "p", "", "directory containing patch files")
	flags.StringVar(&o.OutputFile, "output-file", "", "output file for patched resources (stdout if not specified)")
	flags.StringVarP(&o.OutputDir, "output-dir", "d", "out/patches", "output directory for patched resources")
	flags.BoolVar(&o.ValidateOnly, "validate-only", false, "validate patches without applying them")
	flags.BoolVar(&o.Interactive, "interactive", false, "interactive patch mode")
	flags.BoolVar(&o.Combined, "combined", false, "apply all patches and write a single combined output")
	flags.BoolVar(&o.Diff, "diff", false, "show unified diff of changes without applying")
	flags.StringVar(&o.GroupBy, "group-by", "none", "group resources in combined output (none|file|kind)")
}

// Complete completes the options
func (o *PatchOptions) Complete() error {
	globalOpts := o.Factory.GlobalOptions()

	// Use global output file if specified
	if globalOpts.OutputFile != "" {
		o.OutputFile = globalOpts.OutputFile
	}

	// Scan patch directory if specified
	if o.PatchDir != "" {
		patchFiles, err := o.scanPatchDirectory()
		if err != nil {
			return fmt.Errorf("failed to scan patch directory: %w", err)
		}
		o.PatchFiles = append(o.PatchFiles, patchFiles...)
	}

	// Apply dry-run logic
	if globalOpts.DryRun && o.OutputFile == "" {
		o.OutputFile = "/dev/stdout"
	}

	return nil
}

// Validate validates the options
func (o *PatchOptions) Validate() error {
	// Validate base file exists
	if _, err := os.Stat(o.BaseFile); os.IsNotExist(err) {
		return errors.NewFileError("read", o.BaseFile, "file does not exist", errors.ErrFileNotFound)
	}

	// For interactive mode, we don't need patch files
	if o.Interactive {
		return nil
	}

	// Validate we have patch files
	if len(o.PatchFiles) == 0 {
		return errors.NewValidationError("patches", "", "PatchOptions", []string{"patch-file", "patch-dir"})
	}

	// Validate all patch files exist
	for _, file := range o.PatchFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return errors.NewFileError("read", file, "patch file does not exist", errors.ErrFileNotFound)
		}
	}

	return nil
}

// Run executes the patch command
func (o *PatchOptions) Run() error {
	globalOpts := o.Factory.GlobalOptions()

	if o.Interactive {
		return o.runInteractive()
	}

	if o.ValidateOnly {
		return o.runValidation()
	}

	if o.Diff {
		return o.runDiff()
	}

	if o.Combined {
		return o.runCombined()
	}

	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Applying %d patches to %s\n", len(o.PatchFiles), o.BaseFile)
	}

	// Load base resources
	documentSet, err := o.loadBaseResources()
	if err != nil {
		return fmt.Errorf("failed to load base resources: %w", err)
	}

	// Apply patches
	patchedSet, err := o.applyPatches(documentSet)
	if err != nil {
		return fmt.Errorf("failed to apply patches: %w", err)
	}

	// Write output
	if err := o.writeOutput(patchedSet); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Successfully applied patches to %d resources\n", len(documentSet.Documents))
	}

	return nil
}

// runCombined loads the base, applies all patches sequentially, and writes a single combined output.
func (o *PatchOptions) runCombined() error {
	globalOpts := o.Factory.GlobalOptions()

	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Combined mode: applying %d patches to %s\n", len(o.PatchFiles), o.BaseFile)
	}

	// Validate group-by value
	switch o.GroupBy {
	case "none", "file", "kind":
		// valid
	default:
		return errors.NewValidationError("group-by", o.GroupBy, "PatchOptions", []string{"none", "file", "kind"})
	}

	// Load base resources
	documentSet, err := o.loadBaseResources()
	if err != nil {
		return fmt.Errorf("failed to load base resources: %w", err)
	}

	// Apply each patch file sequentially to the same document set
	for _, patchFile := range o.PatchFiles {
		if globalOpts.Verbose {
			fmt.Fprintf(o.IOStreams.ErrOut, "Applying patch: %s\n", patchFile)
		}

		file, err := os.Open(patchFile)
		if err != nil {
			return fmt.Errorf("failed to open patch file %s: %w", patchFile, err)
		}

		patchSpecs, err := patch.LoadPatchFile(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("failed to load patch file %s: %w", patchFile, err)
		}

		// Convert PatchSpec slice to PatchOp slice
		var ops []patch.PatchOp
		for _, spec := range patchSpecs {
			ops = append(ops, spec.Patch)
		}

		// Apply patches to each document
		for _, doc := range documentSet.Documents {
			if err := doc.ApplyPatchesToDocument(ops); err != nil {
				// Skip documents that don't match the patch target
				continue
			}
		}
	}

	// Sort documents within groups if requested
	if o.GroupBy == "kind" {
		o.sortDocumentsByKind(documentSet)
	}

	// Write combined output
	if o.OutputFile != "" {
		return o.writeCombinedToFile(documentSet)
	}

	return documentSet.WriteToFile("/dev/stdout")
}

// sortDocumentsByKind sorts documents deterministically by kind then name.
func (o *PatchOptions) sortDocumentsByKind(docSet *patch.YAMLDocumentSet) {
	docs := docSet.Documents
	sort.SliceStable(docs, func(i, j int) bool {
		ki := docs[i].Resource.GetKind()
		kj := docs[j].Resource.GetKind()
		if ki != kj {
			return ki < kj
		}
		return docs[i].Resource.GetName() < docs[j].Resource.GetName()
	})
}

// writeCombinedToFile writes the combined document set to a file.
func (o *PatchOptions) writeCombinedToFile(docSet *patch.YAMLDocumentSet) error {
	if o.OutputFile == "/dev/stdout" {
		return docSet.WriteToFile("/dev/stdout")
	}

	dir := filepath.Dir(o.OutputFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return docSet.WriteToFile(o.OutputFile)
}

// runDiff shows a unified diff of changes without applying them.
func (o *PatchOptions) runDiff() error {
	globalOpts := o.Factory.GlobalOptions()

	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Diff mode: comparing %d patches against %s\n", len(o.PatchFiles), o.BaseFile)
	}

	// Load base resources
	originalSet, err := o.loadBaseResources()
	if err != nil {
		return fmt.Errorf("failed to load base resources: %w", err)
	}

	// Create a deep copy to apply patches to
	patchedSet, err := originalSet.Copy()
	if err != nil {
		return fmt.Errorf("failed to copy document set: %w", err)
	}

	// Apply each patch file sequentially to the copied set
	for _, patchFile := range o.PatchFiles {
		if globalOpts.Verbose {
			fmt.Fprintf(o.IOStreams.ErrOut, "Applying patch: %s\n", patchFile)
		}

		file, err := os.Open(patchFile)
		if err != nil {
			return fmt.Errorf("failed to open patch file %s: %w", patchFile, err)
		}

		patchSpecs, err := patch.LoadPatchFile(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("failed to load patch file %s: %w", patchFile, err)
		}

		// Convert PatchSpec slice to PatchOp slice
		var ops []patch.PatchOp
		for _, spec := range patchSpecs {
			ops = append(ops, spec.Patch)
		}

		// Apply patches to each document
		for _, doc := range patchedSet.Documents {
			if err := doc.ApplyPatchesToDocument(ops); err != nil {
				// Skip documents that don't match the patch target
				continue
			}
		}
	}

	// Generate original YAML content
	var originalBuf bytes.Buffer
	for i, doc := range originalSet.Documents {
		if i > 0 {
			originalBuf.WriteString("---\n")
		}
		originalBuf.WriteString(doc.Original)
		if !strings.HasSuffix(doc.Original, "\n") {
			originalBuf.WriteString("\n")
		}
	}

	// Generate patched YAML content
	var patchedBuf bytes.Buffer
	if err := o.writeDocumentSetToBuffer(patchedSet, &patchedBuf); err != nil {
		return fmt.Errorf("failed to generate patched output: %w", err)
	}

	// Generate unified diff
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(originalBuf.String()),
		B:        difflib.SplitLines(patchedBuf.String()),
		FromFile: o.BaseFile,
		ToFile:   o.BaseFile + " (patched)",
		Context:  3,
	}

	diffText, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return fmt.Errorf("failed to generate diff: %w", err)
	}

	if diffText == "" {
		fmt.Fprintf(o.IOStreams.Out, "No changes detected.\n")
		return nil
	}

	// Output diff with optional coloring
	o.printColoredDiff(diffText)

	return nil
}

// writeDocumentSetToBuffer writes a YAMLDocumentSet to a buffer.
func (o *PatchOptions) writeDocumentSetToBuffer(docSet *patch.YAMLDocumentSet, buf *bytes.Buffer) error {
	for i, doc := range docSet.Documents {
		if i > 0 {
			buf.WriteString("---\n")
		}

		// Marshal the YAML node to get the patched content
		docBuf := new(bytes.Buffer)
		encoder := yaml.NewEncoder(docBuf)
		encoder.SetIndent(2)
		if err := encoder.Encode(doc.Node); err != nil {
			return fmt.Errorf("failed to encode document %d: %w", i, err)
		}
		encoder.Close()

		content := docBuf.String()
		content = strings.TrimSuffix(content, "\n")
		buf.WriteString(content)
		buf.WriteString("\n")
	}
	return nil
}

// printColoredDiff prints the diff with ANSI colors if output is a terminal.
func (o *PatchOptions) printColoredDiff(diffText string) {
	// Check if we should use colors (simple heuristic: non-empty TERM env var)
	useColor := os.Getenv("TERM") != "" && os.Getenv("NO_COLOR") == ""

	if !useColor {
		fmt.Fprint(o.IOStreams.Out, diffText)
		return
	}

	// ANSI color codes
	const (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorGreen  = "\033[32m"
		colorCyan   = "\033[36m"
		colorYellow = "\033[33m"
	)

	lines := strings.Split(diffText, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			fmt.Fprintf(o.IOStreams.Out, "%s%s%s\n", colorYellow, line, colorReset)
		case strings.HasPrefix(line, "@@"):
			fmt.Fprintf(o.IOStreams.Out, "%s%s%s\n", colorCyan, line, colorReset)
		case strings.HasPrefix(line, "-"):
			fmt.Fprintf(o.IOStreams.Out, "%s%s%s\n", colorRed, line, colorReset)
		case strings.HasPrefix(line, "+"):
			fmt.Fprintf(o.IOStreams.Out, "%s%s%s\n", colorGreen, line, colorReset)
		default:
			fmt.Fprintf(o.IOStreams.Out, "%s\n", line)
		}
	}
}

// scanPatchDirectory scans directory for patch files
func (o *PatchOptions) scanPatchDirectory() ([]string, error) {
	var patchFiles []string

	entries, err := os.ReadDir(o.PatchDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".kpatch") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			patchFiles = append(patchFiles, filepath.Join(o.PatchDir, name))
		}
	}

	return patchFiles, nil
}

// runValidation validates patches without applying them
func (o *PatchOptions) runValidation() error {
	globalOpts := o.Factory.GlobalOptions()

	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Validating %d patch files\n", len(o.PatchFiles))
	}

	for _, patchFile := range o.PatchFiles {
		if err := o.validatePatchFile(patchFile); err != nil {
			return fmt.Errorf("validation failed for %s: %w", patchFile, err)
		}

		if globalOpts.Verbose {
			fmt.Fprintf(o.IOStreams.ErrOut, "✓ %s\n", patchFile)
		}
	}

	fmt.Fprintf(o.IOStreams.Out, "All patch files are valid\n")
	return nil
}

// validatePatchFile validates a single patch file
func (o *PatchOptions) validatePatchFile(patchFile string) error {
	file, err := os.Open(patchFile)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = patch.LoadPatchFile(file)
	return err
}

// runInteractive runs interactive patch mode
func (o *PatchOptions) runInteractive() error {
	// Load base resources
	file, err := os.Open(o.BaseFile)
	if err != nil {
		return fmt.Errorf("failed to open base file: %w", err)
	}
	defer file.Close()

	resources, err := patch.LoadResourcesFromMultiYAML(file)
	if err != nil {
		return fmt.Errorf("failed to load base resources: %w", err)
	}

	fmt.Fprintf(o.IOStreams.Out, "=== Interactive Patch Mode ===\n")
	fmt.Fprintf(o.IOStreams.Out, "Loaded %d resources from %s\n", len(resources), o.BaseFile)
	fmt.Fprintf(o.IOStreams.Out, "Type 'help' for available commands\n\n")

	// This would implement the interactive loop similar to the existing main.go
	// For now, returning a placeholder
	return errors.ErrInteractiveMode
}

// loadBaseResources loads and parses the base YAML file
func (o *PatchOptions) loadBaseResources() (*patch.YAMLDocumentSet, error) {
	file, err := os.Open(o.BaseFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return patch.LoadResourcesWithStructure(file)
}

// applyPatches applies all patch files to the document set
func (o *PatchOptions) applyPatches(documentSet *patch.YAMLDocumentSet) (*patch.PatchableAppSet, error) {
	// Create patchable set
	patchableSet := &patch.PatchableAppSet{
		Resources:   documentSet.GetResources(),
		DocumentSet: documentSet,
		Patches: make([]struct {
			Target string
			Patch  patch.PatchOp
		}, 0),
	}

	// Apply each patch file
	for _, patchFile := range o.PatchFiles {
		if err := o.applyPatchFile(patchableSet, patchFile); err != nil {
			return nil, fmt.Errorf("failed to apply patch file %s: %w", patchFile, err)
		}
	}

	return patchableSet, nil
}

// applyPatchFile applies a single patch file to the patchable set
func (o *PatchOptions) applyPatchFile(patchableSet *patch.PatchableAppSet, patchFile string) error {
	globalOpts := o.Factory.GlobalOptions()

	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Applying patch: %s\n", patchFile)
	}

	// For now, use the WritePatchedFiles method from the existing patch system
	outputDir := filepath.Join(o.OutputDir, "temp")
	return patchableSet.WritePatchedFiles(o.BaseFile, []string{patchFile}, outputDir)
}

// writeOutput writes the patched resources to output
func (o *PatchOptions) writeOutput(patchableSet *patch.PatchableAppSet) error {
	globalOpts := o.Factory.GlobalOptions()

	if o.OutputFile != "" {
		return o.writeToFile(patchableSet)
	}

	if globalOpts.DryRun {
		return o.writeToStdout(patchableSet)
	}

	// Write to directory
	return o.writeToDirectory(patchableSet)
}

// writeToFile writes output to a single file
func (o *PatchOptions) writeToFile(patchableSet *patch.PatchableAppSet) error {
	if o.OutputFile == "/dev/stdout" {
		return o.writeToStdout(patchableSet)
	}

	// Create directory if needed
	dir := filepath.Dir(o.OutputFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Use document set to write with preserved structure
	return patchableSet.DocumentSet.WriteToFile(o.OutputFile)
}

// writeToStdout writes output to stdout
func (o *PatchOptions) writeToStdout(patchableSet *patch.PatchableAppSet) error {
	return patchableSet.DocumentSet.WriteToFile("/dev/stdout")
}

// writeToDirectory writes output to organized directory structure
func (o *PatchOptions) writeToDirectory(patchableSet *patch.PatchableAppSet) error {
	// Clean output directory
	if err := os.RemoveAll(o.OutputDir); err != nil {
		return err
	}

	// Create base filename from input
	baseName := strings.TrimSuffix(filepath.Base(o.BaseFile), filepath.Ext(o.BaseFile))
	outputFile := filepath.Join(o.OutputDir, baseName+"-patched.yaml")

	// Create directory
	if err := os.MkdirAll(o.OutputDir, 0755); err != nil {
		return err
	}

	// Write patched resources
	return patchableSet.DocumentSet.WriteToFile(outputFile)
}
