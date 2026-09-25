package logger_test

import (
	"os"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/logger"
)

// The README's Go block is generated from this function
// (scripts/gen-doc-examples.sh); go test checks the lines it logs.

func ExampleNew() {
	// logger.Default() writes to stderr with a timestamp and drops Debug; these
	// options log every level to stdout without one.
	log := logger.New(logger.Options{Output: os.Stdout, Level: logger.LevelDebug})
	log.Info("loading package: %s", "/path/to/package")
	log.Error("failed to parse %s: %v", "config.yaml", errors.New("unexpected end of stream"))
	log.Debug("parsed %d resources", 3)

	// No-op logger for quiet mode
	log = logger.Noop()
	log.Info("discarded")
	// Output:
	// [INFO] loading package: /path/to/package
	// [ERROR] failed to parse config.yaml: unexpected end of stream
	// [DEBUG] parsed 3 resources
}
