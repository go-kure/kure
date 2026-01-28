// Package cli provides shared utilities and abstractions for building
// command-line interfaces in the Kure and kurel tools.
//
// # Overview
//
// The cli package provides the foundational components for CLI commands:
//
//   - [Factory]: Dependency injection container for commands
//   - [IOStreams]: Abstraction for stdin/stdout/stderr
//   - [Printer]: Output formatting for various output modes
//   - [Config]: Configuration file handling
//
// # Factory Pattern
//
// The [Factory] interface provides dependency injection for CLI commands,
// making them easier to test and configure:
//
//	factory := cli.NewFactory(globalOpts)
//
//	cmd := &cobra.Command{
//	    RunE: func(cmd *cobra.Command, args []string) error {
//	        streams := factory.IOStreams()
//	        opts := factory.GlobalOptions()
//
//	        fmt.Fprintf(streams.Out, "Running with verbose=%v\n", opts.Verbose)
//	        return nil
//	    },
//	}
//
// # IOStreams
//
// [IOStreams] abstracts the standard I/O streams, enabling testable commands:
//
//	// Production usage
//	streams := cli.NewIOStreams()  // Uses os.Stdin/Stdout/Stderr
//
//	// Test usage
//	var buf bytes.Buffer
//	streams := cli.IOStreams{
//	    In:     strings.NewReader("input"),
//	    Out:    &buf,
//	    ErrOut: &buf,
//	}
//
// # Output Formatting
//
// The [Printer] type supports multiple output formats compatible with
// kubectl conventions:
//
//	printer := cli.NewPrinter(cli.PrintOptions{
//	    Format:     cli.OutputFormatYAML,
//	    NoHeaders:  false,
//	})
//	err := printer.Print(obj, streams.Out)
//
// Supported formats:
//   - YAML (default)
//   - JSON
//   - Table (wide and narrow)
//   - Name (resource name only)
//
// # Configuration
//
// The [Config] type handles configuration file loading and merging:
//
//	cfg, err := cli.LoadConfig("~/.kure/config.yaml")
//	if err != nil {
//	    // handle error
//	}
//
// Configuration is merged from multiple sources:
//  1. Default values
//  2. Configuration file
//  3. Environment variables (KURE_*)
//  4. Command-line flags
package cli
