package shared

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information that gets injected during build
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// NewVersionCommand creates a version command for any CLI
func NewVersionCommand(appName string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  fmt.Sprintf("Print the version number of %s", appName),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s version %s\n", appName, Version)
			fmt.Printf("  Git commit: %s\n", GitCommit)
			fmt.Printf("  Build date: %s\n", BuildDate)
			fmt.Printf("  Go version: %s\n", runtime.Version())
			fmt.Printf("  OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}
