package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

func TestNewIOStreams(t *testing.T) {
	streams := NewIOStreams()

	if streams.In != os.Stdin {
		t.Error("expected In to be os.Stdin")
	}

	if streams.Out != os.Stdout {
		t.Error("expected Out to be os.Stdout")
	}

	if streams.ErrOut != os.Stderr {
		t.Error("expected ErrOut to be os.Stderr")
	}
}

func TestNewFactory(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := NewFactory(globalOpts)

	if factory == nil {
		t.Fatal("expected non-nil factory")
	}

	// Test interface implementation
	var _ Factory = factory
}

func TestFactoryGlobalOptions(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	globalOpts.Output = "json"

	factory := NewFactory(globalOpts)

	retrievedOpts := factory.GlobalOptions()
	if retrievedOpts != globalOpts {
		t.Error("expected same global options instance")
	}

	if !retrievedOpts.Verbose {
		t.Error("expected verbose to be true")
	}

	if retrievedOpts.Output != "json" {
		t.Errorf("expected output 'json', got %s", retrievedOpts.Output)
	}
}

func TestFactoryIOStreams(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := NewFactory(globalOpts)

	streams := factory.IOStreams()

	if streams.In != os.Stdin {
		t.Error("expected In to be os.Stdin")
	}

	if streams.Out != os.Stdout {
		t.Error("expected Out to be os.Stdout")
	}

	if streams.ErrOut != os.Stderr {
		t.Error("expected ErrOut to be os.Stderr")
	}
}

func TestFactoryValidate(t *testing.T) {
	tests := []struct {
		name      string
		setupOpts func() *options.GlobalOptions
		wantErr   bool
	}{
		{
			name: "valid options",
			setupOpts: func() *options.GlobalOptions {
				opts := options.NewGlobalOptions()
				opts.Output = "yaml"
				return opts
			},
			wantErr: false,
		},
		{
			name: "invalid output format",
			setupOpts: func() *options.GlobalOptions {
				opts := options.NewGlobalOptions()
				opts.Output = "invalid"
				return opts
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalOpts := tt.setupOpts()
			factory := NewFactory(globalOpts)

			err := factory.Validate()

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestFactoryImpl(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := NewFactory(globalOpts).(*factoryImpl)

	if factory.globalOpts != globalOpts {
		t.Error("expected same global options instance")
	}

	// Test that IOStreams are initialized
	streams := factory.ioStreams
	if streams.In == nil || streams.Out == nil || streams.ErrOut == nil {
		t.Error("expected all IOStreams to be initialized")
	}
}

func TestIOStreamsUsage(t *testing.T) {
	// Test that we can actually use the streams
	globalOpts := options.NewGlobalOptions()
	factory := NewFactory(globalOpts)
	streams := factory.IOStreams()

	// Verify streams are non-nil (they are already the correct types: io.Writer, io.Reader)
	if streams.Out == nil {
		t.Error("Out stream is nil")
	}
	if streams.In == nil {
		t.Error("In stream is nil")
	}
	if streams.ErrOut == nil {
		t.Error("ErrOut stream is nil")
	}
}

func TestFactoryIntegration(t *testing.T) {
	// Test a complete workflow with factory
	globalOpts := options.NewGlobalOptions()
	globalOpts.Output = "json"
	globalOpts.Verbose = true

	factory := NewFactory(globalOpts)

	// Validate the factory
	if err := factory.Validate(); err != nil {
		t.Fatalf("factory validation failed: %v", err)
	}

	// Get options and verify they're correct
	opts := factory.GlobalOptions()
	if opts.Output != "json" {
		t.Errorf("expected output 'json', got %s", opts.Output)
	}

	if !opts.Verbose {
		t.Error("expected verbose to be true")
	}

	// Get streams and verify they work
	streams := factory.IOStreams()
	if streams.Out == nil {
		t.Error("expected non-nil Out stream")
	}
}

// TestCustomIOStreams tests creating factory with custom IO streams
func TestCustomIOStreams(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := NewFactory(globalOpts).(*factoryImpl)

	// Create custom streams for testing
	inBuf := strings.NewReader("test input")
	outBuf := &strings.Builder{}
	errBuf := &strings.Builder{}

	// Replace the streams
	factory.ioStreams = IOStreams{
		In:     inBuf,
		Out:    outBuf,
		ErrOut: errBuf,
	}

	streams := factory.IOStreams()
	if streams.In != inBuf {
		t.Error("expected custom In stream")
	}

	if streams.Out != outBuf {
		t.Error("expected custom Out stream")
	}

	if streams.ErrOut != errBuf {
		t.Error("expected custom ErrOut stream")
	}
}
