package generate

import (
	"testing"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

func TestNewGenerateCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewGenerateCommand(globalOpts)
	
	if cmd == nil {
		t.Fatal("expected non-nil generate command")
	}
	
	if cmd.Use != "generate" {
		t.Errorf("expected command name 'generate', got %s", cmd.Use)
	}
	
	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}
	
	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}
	
	// Check aliases
	foundGen := false
	for _, alias := range cmd.Aliases {
		if alias == "gen" {
			foundGen = true
			break
		}
	}
	if !foundGen {
		t.Error("expected 'gen' alias not found")
	}
}

func TestGenerateCommandSubcommands(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewGenerateCommand(globalOpts)
	
	expectedSubcommands := []string{"cluster", "app", "bootstrap"}
	
	commands := cmd.Commands()
	if len(commands) < len(expectedSubcommands) {
		t.Errorf("expected at least %d subcommands, got %d", len(expectedSubcommands), len(commands))
	}
	
	// Check that expected subcommands exist
	commandMap := make(map[string]bool)
	for _, subCmd := range commands {
		commandMap[subCmd.Use] = true
	}
	
	for _, expectedCmd := range expectedSubcommands {
		// Extract command name (remove any args specification)
		cmdName := extractCommandName(expectedCmd)
		found := false
		for cmdUse := range commandMap {
			if extractCommandName(cmdUse) == cmdName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %s not found", expectedCmd)
		}
	}
}

func TestGenerateCommandFactory(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	
	// Test that factory is properly passed to subcommands
	cmd := NewGenerateCommand(globalOpts)
	
	// Verify that subcommands are created (this tests the factory integration)
	commands := cmd.Commands()
	if len(commands) == 0 {
		t.Error("expected subcommands to be created")
	}
	
	// Test each subcommand creation
	clusterCmd := NewClusterCommand(factory)
	if clusterCmd == nil {
		t.Error("cluster command creation failed")
	}
	
	appCmd := NewAppCommand(factory)
	if appCmd == nil {
		t.Error("app command creation failed")
	}
	
	bootstrapCmd := NewBootstrapCommand(factory)
	if bootstrapCmd == nil {
		t.Error("bootstrap command creation failed")
	}
}

func TestGenerateCommandIntegration(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	
	cmd := NewGenerateCommand(globalOpts)
	
	// Test that the command structure is properly set up
	if cmd.Use != "generate" {
		t.Errorf("expected Use to be 'generate', got %s", cmd.Use)
	}
	
	// Test that we can access subcommands
	subcommands := cmd.Commands()
	if len(subcommands) == 0 {
		t.Error("expected at least one subcommand")
	}
	
	// Test that each subcommand has proper structure
	for _, subcmd := range subcommands {
		if subcmd.Use == "" {
			t.Error("subcommand has empty Use field")
		}
		if subcmd.Short == "" {
			t.Error("subcommand has empty Short field")
		}
	}
}

// Helper function to extract command name from Use string
func extractCommandName(use string) string {
	for i, char := range use {
		if char == ' ' || char == '[' {
			return use[:i]
		}
	}
	return use
}

func TestGenerateCommandStructure(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewGenerateCommand(globalOpts)
	
	// Test command properties
	if cmd.Use == "" {
		t.Error("expected non-empty Use field")
	}
	
	if cmd.Short == "" {
		t.Error("expected non-empty Short field")
	}
	
	if cmd.Long == "" {
		t.Error("expected non-empty Long field")
	}
	
	// Test that the command has the expected structure for a parent command
	if cmd.RunE != nil {
		t.Error("parent command should not have RunE set")
	}
	
	// Test that subcommands are properly registered
	hasCluster := false
	hasApp := false
	hasBootstrap := false
	
	for _, subcmd := range cmd.Commands() {
		cmdName := extractCommandName(subcmd.Use)
		switch cmdName {
		case "cluster":
			hasCluster = true
		case "app":
			hasApp = true  
		case "bootstrap":
			hasBootstrap = true
		}
	}
	
	if !hasCluster {
		t.Error("cluster subcommand not found")
	}
	if !hasApp {
		t.Error("app subcommand not found")
	}
	if !hasBootstrap {
		t.Error("bootstrap subcommand not found")
	}
}

func TestNewGenerateCommandWithDifferentOptions(t *testing.T) {
	// Test with different global option configurations
	tests := []struct {
		name        string
		setupOpts   func() *options.GlobalOptions
		expectValid bool
	}{
		{
			name: "default options",
			setupOpts: func() *options.GlobalOptions {
				return options.NewGlobalOptions()
			},
			expectValid: true,
		},
		{
			name: "verbose options",
			setupOpts: func() *options.GlobalOptions {
				opts := options.NewGlobalOptions()
				opts.Verbose = true
				return opts
			},
			expectValid: true,
		},
		{
			name: "debug options", 
			setupOpts: func() *options.GlobalOptions {
				opts := options.NewGlobalOptions()
				opts.Debug = true
				return opts
			},
			expectValid: true,
		},
		{
			name: "json output",
			setupOpts: func() *options.GlobalOptions {
				opts := options.NewGlobalOptions()
				opts.Output = "json"
				return opts
			},
			expectValid: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalOpts := tt.setupOpts()
			cmd := NewGenerateCommand(globalOpts)
			
			if tt.expectValid && cmd == nil {
				t.Error("expected valid command but got nil")
			}
			
			if tt.expectValid {
				// Verify command structure is intact
				if cmd.Use != "generate" {
					t.Errorf("expected Use to be 'generate', got %s", cmd.Use)
				}
				
				if len(cmd.Commands()) == 0 {
					t.Error("expected subcommands to be present")
				}
			}
		})
	}
}