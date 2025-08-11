# Plugin Architecture Design for Kure

## Overview

This document outlines the design for implementing a plugin architecture that allows external generators to extend Kure's capabilities. The plugin system would enable users to create custom generators for specific needs (Terraform, Pulumi, custom CRDs, etc.) while maintaining type safety and security.

## Current Generator Architecture

Kure currently uses a static registration system where generators implement the `ApplicationConfig` interface:

```go
type ApplicationConfig interface {
    Generate(*Application) ([]*client.Object, error)
}
```

Built-in generators like `AppWorkload` and `FluxHelm` register themselves via `init()` functions using the GVK (Group, Version, Kind) system:

```go
func init() {
    gvkObj := gvk.GVK{
        Group:   "generators.gokure.dev",
        Version: "v1alpha1", 
        Kind:    "AppWorkload",
    }
    
    factory := func() stack.ApplicationConfig {
        return &ConfigV1Alpha1{}
    }
    
    stack.RegisterApplicationConfig(gvkObj, factory)
}
```

## Plugin Architecture Requirements

### Core Components

1. **Plugin Interface**: Standard contract for all plugins
2. **Plugin Loader**: Dynamic loading from shared libraries
3. **Plugin Manager**: Lifecycle management and registry
4. **Security Layer**: Digital signature verification
5. **Registry Integration**: Seamless integration with existing GVK system
6. **Development SDK**: Helper library for plugin authors
7. **CLI Integration**: Commands for plugin management

### Key Features

- **Dynamic Loading**: Runtime loading from .so/.dylib files
- **Type Safety**: Strong typing through Go interfaces and GVK system
- **Security**: Digital signature verification for authenticity
- **Extensibility**: Support for generators, validators, and documentation providers
- **Isolation**: Clear interface boundaries between plugins and core
- **Discovery**: CLI tools for plugin management
- **SDK Support**: Simplified development experience

## Detailed Implementation Plan

### 1. Plugin Interface Definition

```go
// pkg/stack/plugins/interface.go
package plugins

import (
    "context"
    "github.com/go-kure/kure/internal/gvk"
    "github.com/go-kure/kure/pkg/stack"
)

// Plugin represents a loadable generator plugin
type Plugin interface {
    // Metadata
    Name() string
    Version() string
    Description() string
    Author() string
    
    // Registration
    SupportedGVKs() []gvk.GVK
    Register(registry Registry) error
    
    // Lifecycle
    Initialize(ctx context.Context) error
    Shutdown(ctx context.Context) error
    
    // Health
    HealthCheck(ctx context.Context) error
}

// Registry allows plugins to register their generators
type Registry interface {
    RegisterGenerator(gvk gvk.GVK, factory stack.ApplicationConfigFactory) error
    RegisterValidator(gvk gvk.GVK, validator Validator) error
    RegisterDocumentationProvider(gvk gvk.GVK, provider DocumentationProvider) error
}

// Validator provides validation for plugin-specific configurations
type Validator interface {
    Validate(config stack.ApplicationConfig) error
    GetSchema() ([]byte, error) // JSON Schema
}

// DocumentationProvider provides documentation and examples
type DocumentationProvider interface {
    GetDocumentation() string
    GetExamples() []Example
    GetUsageGuide() string
}

type Example struct {
    Name        string
    Description string
    Config      string // YAML example
}
```

### 2. Plugin Loader and Manager

```go
// pkg/stack/plugins/loader.go
package plugins

import (
    "context"
    "fmt"
    "plugin"
    "sync"
    "time"
)

// LoaderConfig configures plugin loading behavior
type LoaderConfig struct {
    PluginDirs     []string      // Directories to search for plugins
    AllowUnsigned  bool          // Allow unsigned plugins (development only)
    TrustedSigners []string      // Trusted plugin signer keys
    Timeout        time.Duration
}

// Manager manages the lifecycle of plugins
type Manager struct {
    config    LoaderConfig
    plugins   map[string]Plugin
    registry  *PluginRegistry
    mu        sync.RWMutex
}

func NewManager(config LoaderConfig) *Manager {
    return &Manager{
        config:   config,
        plugins:  make(map[string]Plugin),
        registry: NewPluginRegistry(),
    }
}

// LoadPlugins discovers and loads all plugins from configured directories
func (m *Manager) LoadPlugins(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for _, dir := range m.config.PluginDirs {
        if err := m.loadFromDirectory(ctx, dir); err != nil {
            return fmt.Errorf("failed to load plugins from %s: %w", dir, err)
        }
    }
    
    return nil
}

func (m *Manager) loadPlugin(ctx context.Context, path string) error {
    // Verify plugin signature if security is enabled
    if !m.config.AllowUnsigned {
        if err := m.verifyPluginSignature(path); err != nil {
            return fmt.Errorf("plugin signature verification failed: %w", err)
        }
    }
    
    // Load the plugin
    p, err := plugin.Open(path)
    if err != nil {
        return fmt.Errorf("failed to open plugin: %w", err)
    }
    
    // Look for the plugin entry point
    symbol, err := p.Lookup("NewPlugin")
    if err != nil {
        return fmt.Errorf("plugin missing NewPlugin function: %w", err)
    }
    
    // Cast to plugin factory function
    factory, ok := symbol.(func() Plugin)
    if !ok {
        return fmt.Errorf("NewPlugin has wrong signature")
    }
    
    // Create and initialize plugin instance
    pluginInstance := factory()
    
    if err := pluginInstance.Initialize(ctx); err != nil {
        return fmt.Errorf("plugin initialization failed: %w", err)
    }
    
    // Register plugin generators
    if err := pluginInstance.Register(m.registry); err != nil {
        return fmt.Errorf("plugin registration failed: %w", err)
    }
    
    // Store plugin
    m.plugins[pluginInstance.Name()] = pluginInstance
    
    return nil
}

// Additional methods: GetPlugin, ListPlugins, Shutdown...
```

### 3. Plugin Registry Integration

```go
// pkg/stack/plugins/registry.go
package plugins

import (
    "fmt"
    "sync"
    
    "github.com/go-kure/kure/internal/gvk"
    "github.com/go-kure/kure/pkg/stack"
)

// PluginRegistry manages plugin-registered generators
type PluginRegistry struct {
    generators    map[gvk.GVK]stack.ApplicationConfigFactory
    validators    map[gvk.GVK]Validator
    docProviders  map[gvk.GVK]DocumentationProvider
    plugins       map[gvk.GVK]string // Maps GVK to plugin name
    mu           sync.RWMutex
}

func NewPluginRegistry() *PluginRegistry {
    return &PluginRegistry{
        generators:   make(map[gvk.GVK]stack.ApplicationConfigFactory),
        validators:   make(map[gvk.GVK]Validator),
        docProviders: make(map[gvk.GVK]DocumentationProvider),
        plugins:      make(map[gvk.GVK]string),
    }
}

func (r *PluginRegistry) RegisterGenerator(gvkObj gvk.GVK, factory stack.ApplicationConfigFactory) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if _, exists := r.generators[gvkObj]; exists {
        return fmt.Errorf("generator for GVK %s already registered", gvkObj)
    }
    
    r.generators[gvkObj] = factory
    
    // Also register with the global stack registry
    stack.RegisterApplicationConfig(gvkObj, factory)
    
    return nil
}

// Additional methods for validators, documentation providers, queries...
```

### 4. Security and Verification

```go
// pkg/stack/plugins/security.go
package plugins

import (
    "crypto"
    "crypto/rsa"
    "crypto/sha256"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "os"
)

// verifyPluginSignature verifies a plugin's digital signature
func (m *Manager) verifyPluginSignature(pluginPath string) error {
    // Look for signature file
    sigPath := pluginPath + ".sig"
    sigData, err := os.ReadFile(sigPath)
    if err != nil {
        return fmt.Errorf("signature file not found: %w", err)
    }
    
    // Read plugin binary
    pluginData, err := os.ReadFile(pluginPath)
    if err != nil {
        return fmt.Errorf("failed to read plugin: %w", err)
    }
    
    // Hash the plugin
    hash := sha256.Sum256(pluginData)
    
    // Verify signature against trusted signers
    for _, signerKey := range m.config.TrustedSigners {
        if err := m.verifySignature(hash[:], sigData, signerKey); err == nil {
            return nil // Valid signature found
        }
    }
    
    return fmt.Errorf("no valid signature found")
}

func (m *Manager) verifySignature(hash, signature []byte, pubKeyPath string) error {
    // Read and parse public key
    keyData, err := os.ReadFile(pubKeyPath)
    if err != nil {
        return fmt.Errorf("failed to read public key: %w", err)
    }
    
    // Parse PEM format
    block, _ := pem.Decode(keyData)
    if block == nil {
        return fmt.Errorf("invalid PEM format")
    }
    
    // Parse public key
    pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
    if err != nil {
        return fmt.Errorf("failed to parse public key: %w", err)
    }
    
    rsaPubKey, ok := pubKey.(*rsa.PublicKey)
    if !ok {
        return fmt.Errorf("only RSA keys supported")
    }
    
    // Verify signature
    return rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, hash, signature)
}
```

### 5. Plugin Development SDK

```go
// pkg/stack/plugins/sdk/base.go
package sdk

import (
    "context"
    
    "github.com/go-kure/kure/internal/gvk"
    "github.com/go-kure/kure/pkg/stack/plugins"
)

// BasePlugin provides a default implementation of common plugin functionality
type BasePlugin struct {
    name        string
    version     string
    description string
    author      string
    supportedGVKs []gvk.GVK
}

func NewBasePlugin(name, version, description, author string) *BasePlugin {
    return &BasePlugin{
        name:        name,
        version:     version,
        description: description,
        author:      author,
    }
}

func (p *BasePlugin) Name() string        { return p.name }
func (p *BasePlugin) Version() string     { return p.version }
func (p *BasePlugin) Description() string { return p.description }
func (p *BasePlugin) Author() string      { return p.author }

func (p *BasePlugin) SupportedGVKs() []gvk.GVK {
    return p.supportedGVKs
}

func (p *BasePlugin) AddSupportedGVK(gvkObj gvk.GVK) {
    p.supportedGVKs = append(p.supportedGVKs, gvkObj)
}

// Default implementations that can be overridden
func (p *BasePlugin) Initialize(ctx context.Context) error {
    return nil
}

func (p *BasePlugin) Shutdown(ctx context.Context) error {
    return nil
}

func (p *BasePlugin) HealthCheck(ctx context.Context) error {
    return nil
}

// Register must be implemented by concrete plugins
func (p *BasePlugin) Register(registry plugins.Registry) error {
    panic("Register method must be implemented by concrete plugin")
}
```

### 6. CLI Integration

```go
// pkg/cmd/kure/plugins.go
package main

import (
    "context"
    "fmt"
    "os"
    "text/tabwriter"
    "time"
    
    "github.com/spf13/cobra"
    "github.com/go-kure/kure/pkg/stack/plugins"
)

func newPluginsCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "plugins",
        Short: "Manage generator plugins",
    }
    
    cmd.AddCommand(newPluginsListCmd())
    cmd.AddCommand(newPluginsInstallCmd())
    cmd.AddCommand(newPluginsUninstallCmd())
    cmd.AddCommand(newPluginsInfoCmd())
    
    return cmd
}

func newPluginsListCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "list",
        Short: "List installed plugins",
        RunE: func(cmd *cobra.Command, args []string) error {
            manager := getPluginManager()
            
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()
            
            if err := manager.LoadPlugins(ctx); err != nil {
                return fmt.Errorf("failed to load plugins: %w", err)
            }
            
            plugins := manager.ListPlugins()
            if len(plugins) == 0 {
                fmt.Println("No plugins installed.")
                return nil
            }
            
            w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
            fmt.Fprintln(w, "NAME\tVERSION\tAUTHOR\tDESCRIPTION")
            
            for _, plugin := range plugins {
                fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
                    plugin.Name(),
                    plugin.Version(),
                    plugin.Author(),
                    plugin.Description())
            }
            
            return w.Flush()
        },
    }
}

func getPluginManager() *plugins.Manager {
    config := plugins.LoaderConfig{
        PluginDirs:     []string{"/usr/local/lib/kure/plugins", "~/.kure/plugins", "./plugins"},
        AllowUnsigned:  false, // Should be configurable
        TrustedSigners: []string{"/etc/kure/trusted-signers.pem"},
        Timeout:        30 * time.Second,
    }
    
    return plugins.NewManager(config)
}
```

## Example Plugin Implementation

### Terraform Generator Plugin

```go
// examples/plugins/terraform-generator/main.go
package main

import (
    "context"
    
    "sigs.k8s.io/controller-runtime/pkg/client"
    
    "github.com/go-kure/kure/internal/gvk"
    "github.com/go-kure/kure/pkg/stack"
    "github.com/go-kure/kure/pkg/stack/plugins"
    "github.com/go-kure/kure/pkg/stack/plugins/sdk"
)

// TerraformPlugin generates Kubernetes resources that deploy Terraform configurations
type TerraformPlugin struct {
    *sdk.BasePlugin
}

func NewPlugin() plugins.Plugin {
    plugin := &TerraformPlugin{
        BasePlugin: sdk.NewBasePlugin(
            "terraform-generator",
            "v1.0.0",
            "Generates Kubernetes Jobs that run Terraform configurations",
            "Example Corp <plugins@example.com>",
        ),
    }
    
    plugin.AddSupportedGVK(gvk.GVK{
        Group:   "generators.example.com",
        Version: "v1alpha1",
        Kind:    "TerraformConfig",
    })
    
    return plugin
}

func (p *TerraformPlugin) Register(registry plugins.Registry) error {
    gvkObj := gvk.GVK{
        Group:   "generators.example.com",
        Version: "v1alpha1",
        Kind:    "TerraformConfig",
    }
    
    factory := func() stack.ApplicationConfig {
        return &TerraformConfig{}
    }
    
    return registry.RegisterGenerator(gvkObj, factory)
}

// TerraformConfig represents a Terraform configuration to be deployed
type TerraformConfig struct {
    APIVersion string `yaml:"apiVersion" json:"apiVersion"`
    Kind       string `yaml:"kind" json:"kind"`
    
    // Terraform-specific configuration
    Module          string            `yaml:"module" json:"module"`
    Variables       map[string]string `yaml:"variables,omitempty" json:"variables,omitempty"`
    BackendConfig   BackendConfig     `yaml:"backend" json:"backend"`
    RequiredVersion string            `yaml:"requiredVersion,omitempty" json:"requiredVersion,omitempty"`
}

type BackendConfig struct {
    Type   string            `yaml:"type" json:"type"`
    Config map[string]string `yaml:"config" json:"config"`
}

func (c *TerraformConfig) Generate(app *stack.Application) ([]*client.Object, error) {
    // Implementation would create:
    // - Kubernetes Job to run terraform apply
    // - ConfigMap with terraform files
    // - Secret for backend credentials  
    // - ServiceAccount with appropriate RBAC
    
    return []*client.Object{
        // Job, ConfigMap, Secret, ServiceAccount, etc.
    }, nil
}

func (c *TerraformConfig) GetAPIVersion() string { return c.APIVersion }
func (c *TerraformConfig) GetKind() string       { return c.Kind }

// Required for Go plugins
func main() {} // Empty main required for buildmode=plugin
```

## Plugin Build and Distribution

### Building Plugins

```bash
# Makefile for building plugins
.PHONY: plugin
plugin:
	go build -buildmode=plugin -o terraform-generator.so ./examples/plugins/terraform-generator/

# Plugin signing
sign-plugin:
	openssl dgst -sha256 -sign private-key.pem -out terraform-generator.so.sig terraform-generator.so

# Installation script
install-plugin:
	sudo cp terraform-generator.so /usr/local/lib/kure/plugins/
	sudo cp terraform-generator.so.sig /usr/local/lib/kure/plugins/
```

### Usage Example

```yaml
# Using a plugin-provided generator
apiVersion: generators.example.com/v1alpha1
kind: TerraformConfig
metadata:
  name: infrastructure
spec:
  module: "./terraform/modules/vpc"
  variables:
    region: "us-west-2"
    environment: "production"
  backend:
    type: "s3"
    config:
      bucket: "terraform-state-bucket"
      key: "infrastructure/terraform.tfstate"
      region: "us-west-2"
```

```bash
# CLI plugin management
kure plugins list
kure plugins install terraform-generator.so
kure plugins info terraform-generator
```

## Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Plugin interface definition
- [ ] Basic plugin loader without security
- [ ] Plugin registry integration
- [ ] Simple CLI commands

### Phase 2: Security and Validation  
- [ ] Digital signature verification
- [ ] Plugin validation framework
- [ ] Configuration schema support
- [ ] Security documentation

### Phase 3: Developer Experience
- [ ] Plugin development SDK
- [ ] Example plugins
- [ ] Documentation and tutorials
- [ ] Testing utilities

### Phase 4: Advanced Features
- [ ] Plugin dependency management
- [ ] Hot-reloading support
- [ ] Plugin marketplace integration
- [ ] Monitoring and metrics

## Security Considerations

1. **Code Execution**: Plugins run in the same process space
2. **Digital Signatures**: Required for production use
3. **Sandboxing**: Consider future isolation mechanisms
4. **Resource Limits**: Memory and CPU constraints
5. **Network Access**: Plugin network restrictions
6. **File System**: Limited file system access

## Benefits

1. **Extensibility**: Users can create domain-specific generators
2. **Community**: Ecosystem of third-party plugins
3. **Maintenance**: Reduces core maintenance burden
4. **Innovation**: Faster iteration on new generator types
5. **Adoption**: Easier integration with existing toolchains

## Risks and Mitigation

1. **Security**: Mitigated by signature verification and sandboxing
2. **Stability**: Mitigated by plugin isolation and error handling
3. **Performance**: Mitigated by lazy loading and resource limits
4. **Compatibility**: Mitigated by versioned interfaces and testing

## Status

This task is currently **postponed** pending completion of higher-priority features like:
- KurelPackage generator completion
- ArgoCD bootstrap implementation  
- OpenAPI schema generation

The design is ready for implementation when development resources become available.