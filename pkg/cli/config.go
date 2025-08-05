package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents the kure configuration structure
type Config struct {
	// Default settings
	Defaults struct {
		Output    string `yaml:"output"`
		Namespace string `yaml:"namespace"`
		Verbose   bool   `yaml:"verbose"`
		Debug     bool   `yaml:"debug"`
	} `yaml:"defaults"`

	// Layout settings
	Layout struct {
		ManifestsDir        string `yaml:"manifestsDir"`
		BundleGrouping      string `yaml:"bundleGrouping"`
		ApplicationGrouping string `yaml:"applicationGrouping"`
		FluxPlacement       string `yaml:"fluxPlacement"`
	} `yaml:"layout"`

	// GitOps settings
	GitOps struct {
		Type       string `yaml:"type"`
		Repository string `yaml:"repository"`
		Branch     string `yaml:"branch"`
		Path       string `yaml:"path"`
	} `yaml:"gitops"`
}

// NewDefaultConfig returns a config with default values
func NewDefaultConfig() *Config {
	config := &Config{}

	// Set defaults
	config.Defaults.Output = "yaml"
	config.Defaults.Namespace = ""
	config.Defaults.Verbose = false
	config.Defaults.Debug = false

	// Layout defaults
	config.Layout.ManifestsDir = "manifests"
	config.Layout.BundleGrouping = "flat"
	config.Layout.ApplicationGrouping = "flat"
	config.Layout.FluxPlacement = "integrated"

	// GitOps defaults
	config.GitOps.Type = "flux"
	config.GitOps.Branch = "main"
	config.GitOps.Path = "clusters"

	return config
}

// LoadConfig loads configuration from file
func LoadConfig(configFile string) (*Config, error) {
	config := NewDefaultConfig()

	if configFile == "" {
		return config, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config, configFile string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the default config file path
func GetConfigPath() string {
	if configFile := viper.ConfigFileUsed(); configFile != "" {
		return configFile
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ".kure.yaml"
	}

	return filepath.Join(home, ".kure.yaml")
}

// EnsureConfigDir ensures the config directory exists
func EnsureConfigDir() error {
	configPath := GetConfigPath()
	dir := filepath.Dir(configPath)
	return os.MkdirAll(dir, 0755)
}
