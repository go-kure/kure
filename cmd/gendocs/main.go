// cmd/gendocs generates CLI reference documentation from cobra command trees.
//
// Output is written to site/.generated/cli-reference/ with Hugo front matter
// included so the generated markdown can be mounted directly by Hugo.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra/doc"

	"github.com/go-kure/kure/pkg/cmd/kure"
)

func main() {
	outDir := "site/.generated/cli-reference"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	if err := os.MkdirAll(filepath.Clean(outDir), 0o755); err != nil { //nolint:gosec // G703: CLI tool, output dir from args
		fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Custom file prepender that adds Hugo front matter.
	weight := 10
	filePrepender := func(filename string) string {
		name := filepath.Base(filename)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		name = strings.ReplaceAll(name, "_", " ")

		w := weight
		weight += 10

		return fmt.Sprintf(`---
title: "%s"
weight: %d
---

`, name, w)
	}

	// Custom link handler that strips the .md extension for Hugo.
	linkHandler := func(name string) string {
		base := strings.TrimSuffix(name, filepath.Ext(name))
		return "../" + strings.ToLower(base) + "/"
	}

	// Generate kure CLI docs
	kureCmd := kure.NewKureCommand()
	kureCmd.DisableAutoGenTag = true
	if err := doc.GenMarkdownTreeCustom(kureCmd, outDir, filePrepender, linkHandler); err != nil {
		fmt.Fprintf(os.Stderr, "error generating kure docs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("CLI reference generated in %s\n", outDir)
}
