package cli

import (
	"io"
	"os"

	"github.com/go-kure/kure/pkg/cmd/options"
)

// Factory provides access to common dependencies and configuration
type Factory interface {
	// Configuration
	GlobalOptions() *options.GlobalOptions
	
	// IO
	IOStreams() IOStreams
	
	// Validation
	Validate() error
}

// IOStreams represents the standard input, output, and error streams
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// NewIOStreams creates IOStreams with standard streams
func NewIOStreams() IOStreams {
	return IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
}

// factoryImpl implements the Factory interface
type factoryImpl struct {
	globalOpts *options.GlobalOptions
	ioStreams  IOStreams
}

// NewFactory creates a new Factory implementation
func NewFactory(globalOpts *options.GlobalOptions) Factory {
	return &factoryImpl{
		globalOpts: globalOpts,
		ioStreams:  NewIOStreams(),
	}
}

// GlobalOptions returns the global options
func (f *factoryImpl) GlobalOptions() *options.GlobalOptions {
	return f.globalOpts
}

// IOStreams returns the IO streams
func (f *factoryImpl) IOStreams() IOStreams {
	return f.ioStreams
}

// Validate validates the factory configuration
func (f *factoryImpl) Validate() error {
	return f.globalOpts.Validate()
}