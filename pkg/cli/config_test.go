package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestNewDefaultConfig(t *testing.T) {
	config := NewDefaultConfig()
	
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	
	// Test defaults
	if config.Defaults.Output != "yaml" {
		t.Errorf("expected default output to be 'yaml', got %s", config.Defaults.Output)
	}
	
	if config.Defaults.Namespace != "" {
		t.Errorf("expected default namespace to be empty, got %s", config.Defaults.Namespace)
	}
	
	if config.Defaults.Verbose != false {
		t.Errorf("expected default verbose to be false, got %t", config.Defaults.Verbose)
	}
	
	if config.Defaults.Debug != false {
		t.Errorf("expected default debug to be false, got %t", config.Defaults.Debug)
	}
	
	// Test layout defaults
	if config.Layout.ManifestsDir != "manifests" {
		t.Errorf("expected default manifestsDir to be 'manifests', got %s", config.Layout.ManifestsDir)
	}
	
	if config.Layout.BundleGrouping != "flat" {
		t.Errorf("expected default bundleGrouping to be 'flat', got %s", config.Layout.BundleGrouping)
	}
	
	if config.Layout.ApplicationGrouping != "flat" {
		t.Errorf("expected default applicationGrouping to be 'flat', got %s", config.Layout.ApplicationGrouping)
	}
	
	if config.Layout.FluxPlacement != "integrated" {
		t.Errorf("expected default fluxPlacement to be 'integrated', got %s", config.Layout.FluxPlacement)
	}
	
	// Test GitOps defaults
	if config.GitOps.Type != "flux" {
		t.Errorf("expected default gitops type to be 'flux', got %s", config.GitOps.Type)
	}
	
	if config.GitOps.Branch != "main" {
		t.Errorf("expected default gitops branch to be 'main', got %s", config.GitOps.Branch)
	}
	
	if config.GitOps.Path != "clusters" {
		t.Errorf("expected default gitops path to be 'clusters', got %s", config.GitOps.Path)
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected *Config
		wantErr  bool
	}{
		{
			name:     "empty config file path",
			config:   "",
			expected: NewDefaultConfig(),
			wantErr:  false,
		},
		{
			name:   "valid config",
			config: createTempConfig(t, validConfigYAML),
			expected: &Config{
				Defaults: struct {
					Output    string `yaml:"output"`
					Namespace string `yaml:"namespace"`
					Verbose   bool   `yaml:"verbose"`
					Debug     bool   `yaml:"debug"`
				}{
					Output:    "json",
					Namespace: "test",
					Verbose:   true,
					Debug:     false,
				},
				Layout: struct {
					ManifestsDir        string `yaml:"manifestsDir"`
					BundleGrouping      string `yaml:"bundleGrouping"`
					ApplicationGrouping string `yaml:"applicationGrouping"`
					FluxPlacement       string `yaml:"fluxPlacement"`
				}{
					ManifestsDir:        "custom-manifests",
					BundleGrouping:      "hierarchical",
					ApplicationGrouping: "hierarchical",
					FluxPlacement:       "separate",
				},
				GitOps: struct {
					Type       string `yaml:"type"`
					Repository string `yaml:"repository"`
					Branch     string `yaml:"branch"`
					Path       string `yaml:"path"`
				}{
					Type:       "argocd",
					Repository: "https://github.com/example/repo.git",
					Branch:     "develop",
					Path:       "apps",
				},
			},
			wantErr: false,
		},
		{
			name:     "non-existent file",
			config:   "/non/existent/file.yaml",
			expected: NewDefaultConfig(),
			wantErr:  false, // Should return default config
		},
		{
			name:    "invalid yaml",
			config:  createTempConfig(t, "invalid: yaml: content: ["),
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadConfig(tt.config)
			
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
				return
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if !tt.wantErr {
				if config.Defaults.Output != tt.expected.Defaults.Output {
					t.Errorf("expected output %s, got %s", tt.expected.Defaults.Output, config.Defaults.Output)
				}
				if config.GitOps.Type != tt.expected.GitOps.Type {
					t.Errorf("expected gitops type %s, got %s", tt.expected.GitOps.Type, config.GitOps.Type)
				}
			}
		})
	}
}

func TestSaveConfig(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	
	config := NewDefaultConfig()
	config.Defaults.Output = "json"
	config.GitOps.Type = "argocd"
	
	err := SaveConfig(config, configFile)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	
	// Verify file was created
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
	
	// Load and verify content
	loadedConfig, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	
	if loadedConfig.Defaults.Output != "json" {
		t.Errorf("expected output 'json', got %s", loadedConfig.Defaults.Output)
	}
	
	if loadedConfig.GitOps.Type != "argocd" {
		t.Errorf("expected gitops type 'argocd', got %s", loadedConfig.GitOps.Type)
	}
}

func TestSaveConfigCreateDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "nested", "dir", "config.yaml")
	
	config := NewDefaultConfig()
	err := SaveConfig(config, configFile)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	
	// Verify file was created
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config file was not created in nested directory")
	}
}

func TestGetConfigPath(t *testing.T) {
	// Save original viper state
	originalConfigFile := viper.ConfigFileUsed()
	defer func() {
		viper.SetConfigFile(originalConfigFile)
	}()
	
	// Test with viper config file set
	testConfigFile := "/test/config.yaml"
	viper.SetConfigFile(testConfigFile)
	path := GetConfigPath()
	if path != testConfigFile {
		t.Errorf("expected path %s, got %s", testConfigFile, path)
	}
	
	// Clear viper config file
	viper.SetConfigFile("")
	
	// Test with home directory
	path = GetConfigPath()
	if path == "" {
		t.Error("expected non-empty config path")
	}
	
	if !filepath.IsAbs(path) && path != ".kure.yaml" {
		t.Errorf("expected absolute path or '.kure.yaml', got %s", path)
	}
}

func TestEnsureConfigDir(t *testing.T) {
	// This test is tricky because it affects the real home directory
	// We'll just verify it doesn't error when the directory is accessible
	err := EnsureConfigDir()
	// Only fail if it's not a permission error (which is expected in some test environments)
	if err != nil && !os.IsPermission(err) {
		t.Errorf("EnsureConfigDir failed: %v", err)
	}
}

const validConfigYAML = `
defaults:
  output: json
  namespace: test
  verbose: true
  debug: false

layout:
  manifestsDir: custom-manifests
  bundleGrouping: hierarchical
  applicationGrouping: hierarchical
  fluxPlacement: separate

gitops:
  type: argocd
  repository: https://github.com/example/repo.git
  branch: develop
  path: apps
`

func createTempConfig(t *testing.T, content string) string {
	t.Helper()
	
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	
	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}
	
	return configFile
}