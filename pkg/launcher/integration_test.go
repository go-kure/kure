package launcher_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/launcher"
	"github.com/go-kure/kure/pkg/logger"
)

// Integration tests for the complete launcher workflow
func TestIntegration_SimplePackage(t *testing.T) {
	ctx := context.Background()
	log := logger.Noop()

	// Create all components
	loader := launcher.NewPackageLoader(log)
	resolver := launcher.NewResolver(log)
	processor := launcher.NewPatchProcessor(log, resolver)
	validator := launcher.NewValidator(log)
	builder := launcher.NewBuilder(log)

	// Load simple package
	packagePath := "testdata/packages/simple"

	// Check if file exists
	kurelPath := filepath.Join(packagePath, "kurel.yaml")
	if _, err := os.Stat(kurelPath); os.IsNotExist(err) {
		t.Fatalf("Package file does not exist: %s", kurelPath)
	}

	opts := launcher.DefaultOptions()
	opts.Logger = logger.Default() // Use real logger for debug

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		t.Fatalf("Failed to load simple package: %v", err)
	}

	// Debug: Print loaded package
	t.Logf("Loaded package: name=%s, version=%s, resources=%d",
		def.Metadata.Name, def.Metadata.Version, len(def.Resources))

	// Validate package
	result, err := validator.ValidatePackage(ctx, def)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !result.IsValid() {
		t.Errorf("Simple package should be valid, got %d errors", len(result.Errors))
		for _, e := range result.Errors {
			t.Logf("  Error: %v", e)
		}
	}

	// Resolve patches
	enabledPatches, err := processor.ResolveDependencies(ctx, def.Patches, def.Parameters)
	if err != nil {
		t.Fatalf("Failed to resolve patches: %v", err)
	}

	// Create instance
	instance := &launcher.PackageInstance{
		Definition:     def,
		UserValues:     make(launcher.ParameterMap),
		EnabledPatches: enabledPatches,
	}

	// Build manifests
	buildOpts := launcher.BuildOptions{
		Output:     launcher.OutputStdout,
		Format:     launcher.FormatYAML,
		OutputPath: "", // Will write to buffer
	}

	// Use a buffer to capture output
	var buf bytes.Buffer
	builder.SetOutputWriter(&buf)

	err = builder.Build(ctx, instance, buildOpts, opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Get output from buffer
	output := buf.String()

	// Verify output contains expected resources
	if !strings.Contains(output, "kind: Deployment") {
		t.Error("Output should contain Deployment")
	}
	if !strings.Contains(output, "kind: Service") {
		t.Error("Output should contain Service")
	}
}

func TestIntegration_ComplexPackage(t *testing.T) {
	ctx := context.Background()
	log := logger.Noop()

	// Create all components
	loader := launcher.NewPackageLoader(log)
	resolver := launcher.NewResolver(log)
	processor := launcher.NewPatchProcessor(log, resolver)
	validator := launcher.NewValidator(log)

	// Load complex package
	packagePath := "testdata/packages/complex"
	opts := launcher.DefaultOptions()
	opts.Logger = log

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		t.Fatalf("Failed to load complex package: %v", err)
	}

	// Test with production parameters
	def.Parameters["environment"] = "production"

	// Validate package
	result, err := validator.ValidatePackage(ctx, def)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !result.IsValid() {
		t.Errorf("Complex package should be valid, got %d errors", len(result.Errors))
		for _, e := range result.Errors {
			t.Logf("  Error: %v", e)
		}
	}

	// Verify parameters loaded correctly
	if def.Parameters["environment"] != "production" {
		t.Errorf("Environment should be production, got %v", def.Parameters["environment"])
	}

	// Resolve patches - production patches should be enabled
	enabledPatches, err := processor.ResolveDependencies(ctx, def.Patches, def.Parameters)
	if err != nil {
		t.Fatalf("Failed to resolve patches: %v", err)
	}

	// For now, just check that patch resolution works
	t.Logf("Found %d patches, %d enabled", len(def.Patches), len(enabledPatches))
}

func TestIntegration_InvalidPackage(t *testing.T) {
	ctx := context.Background()
	log := logger.Noop()

	// Create components
	loader := launcher.NewPackageLoader(log)
	validator := launcher.NewValidator(log)

	// Load invalid package
	packagePath := "testdata/packages/invalid"
	opts := launcher.DefaultOptions()
	opts.Logger = log
	opts.StrictMode = false // Don't fail on warnings

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		// Some errors might prevent loading entirely
		t.Logf("Expected load error: %v", err)
		return
	}

	// Validate package - should have errors
	result, err := validator.ValidatePackage(ctx, def)
	if err == nil && result.IsValid() {
		t.Error("Invalid package should have validation errors")
	}

	// Check for expected errors
	if len(result.Errors) == 0 {
		t.Error("Should have validation errors")
	} else {
		t.Logf("Found %d validation errors (expected)", len(result.Errors))
		for _, e := range result.Errors {
			t.Logf("  Error: %v", e)
		}
	}
}

func TestIntegration_WithExtensions(t *testing.T) {
	ctx := context.Background()
	log := logger.Noop()

	// Create all components
	loader := launcher.NewPackageLoader(log)
	extension := launcher.NewExtensionLoader(log)
	validator := launcher.NewValidator(log)

	// Load package with extensions
	packagePath := "testdata/packages/extensions"
	opts := launcher.DefaultOptions()
	opts.Logger = log

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	// Apply extensions
	def, err = extension.LoadWithExtensions(ctx, def, packagePath, opts)
	if err != nil {
		t.Fatalf("Failed to apply extensions: %v", err)
	}

	// Check that extension modified parameters
	if def.Parameters["environment"] != "production" {
		t.Error("Extension should have set environment to production")
	}
	if def.Parameters["replicas"] != 5 {
		t.Errorf("Extension should have set replicas to 5, got %v", def.Parameters["replicas"])
	}

	// Check that extension applied (simplified check)
	t.Logf("Package has %d patches after extension", len(def.Patches))

	// Validate extended package
	result, err := validator.ValidatePackage(ctx, def)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !result.IsValid() {
		t.Errorf("Extended package should be valid, got %d errors", len(result.Errors))
	}
}

func TestIntegration_WithUserValues(t *testing.T) {
	ctx := context.Background()
	log := logger.Noop()

	// Create components
	loader := launcher.NewPackageLoader(log)

	// Load simple package
	packagePath := "testdata/packages/simple"
	opts := launcher.DefaultOptions()
	opts.Logger = log

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	// Load user values
	valuesPath := "testdata/values/test-values.yaml"
	valuesData, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatalf("Failed to read values file: %v", err)
	}

	var userValues launcher.ParameterMap
	if err := yaml.Unmarshal(valuesData, &userValues); err != nil {
		t.Fatalf("Failed to parse values: %v", err)
	}

	// Create a copy of parameters and merge with user values
	mergedParams := make(launcher.ParameterMap)
	for k, v := range def.Parameters {
		mergedParams[k] = v
	}
	for k, v := range userValues {
		mergedParams[k] = v
	}

	// Check that user values were applied
	if mergedParams["replicas"] != 4 {
		t.Errorf("User value for replicas should be 4, got %v", mergedParams["replicas"])
	}

	// Check nested values - need to handle the type correctly
	if imageVal, ok := mergedParams["image"]; ok {
		if imageMap, ok := imageVal.(map[string]interface{}); ok {
			if imageMap["tag"] != "v2.0.0" {
				t.Errorf("User value for image tag should be v2.0.0, got %v", imageMap["tag"])
			}
		} else if imageMap, ok := imageVal.(launcher.ParameterMap); ok {
			if imageMap["tag"] != "v2.0.0" {
				t.Errorf("User value for image tag should be v2.0.0, got %v", imageMap["tag"])
			}
		} else {
			t.Errorf("Image should be a map, got %T", imageVal)
		}
	}
}

func TestIntegration_SchemaGeneration(t *testing.T) {
	ctx := context.Background()
	log := logger.Noop()

	// Create schema generator
	schemaGen := launcher.NewSchemaGenerator(log)

	// Generate schema
	schema, err := schemaGen.GeneratePackageSchema(ctx)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	// Export schema
	schemaJSON, err := schemaGen.ExportSchema(schema)
	if err != nil {
		t.Fatalf("Failed to export schema: %v", err)
	}

	// Parse and validate schema structure
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
		t.Fatalf("Failed to parse schema JSON: %v", err)
	}

	// Just check that we got a valid JSON schema structure
	if schemaMap["type"] == nil {
		t.Error("Schema should have type field")
	}

	t.Logf("Generated schema has keys: %v", getMapKeys(schemaMap))
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestIntegration_OutputFormats(t *testing.T) {
	ctx := context.Background()
	log := logger.Noop()

	// Create components
	loader := launcher.NewPackageLoader(log)
	builder := launcher.NewBuilder(log)

	// Load simple package
	packagePath := "testdata/packages/simple"
	opts := launcher.DefaultOptions()
	opts.Logger = log

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	instance := &launcher.PackageInstance{
		Definition:     def,
		UserValues:     make(launcher.ParameterMap),
		EnabledPatches: []launcher.Patch{},
	}

	// Test YAML output
	t.Run("YAML Output", func(t *testing.T) {
		tempDir := t.TempDir()
		outputPath := filepath.Join(tempDir, "output.yaml")

		buildOpts := launcher.BuildOptions{
			Output:     launcher.OutputFile,
			OutputPath: outputPath,
			Format:     launcher.FormatYAML,
		}

		if err := builder.Build(ctx, instance, buildOpts, opts); err != nil {
			t.Fatalf("Build failed: %v", err)
		}

		// Read and verify output
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("Failed to read output: %v", err)
		}

		// Should be valid YAML
		var docs []interface{}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		for {
			var doc interface{}
			if err := decoder.Decode(&doc); err != nil {
				break
			}
			docs = append(docs, doc)
		}

		if len(docs) < 2 {
			t.Error("Should have at least 2 documents (Deployment and Service)")
		}
	})

	// Test JSON output
	t.Run("JSON Output", func(t *testing.T) {
		tempDir := t.TempDir()
		outputPath := filepath.Join(tempDir, "output.json")

		buildOpts := launcher.BuildOptions{
			Output:      launcher.OutputFile,
			OutputPath:  outputPath,
			Format:      launcher.FormatJSON,
			PrettyPrint: true,
		}

		if err := builder.Build(ctx, instance, buildOpts, opts); err != nil {
			t.Fatalf("Build failed: %v", err)
		}

		// Read and verify output
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("Failed to read output: %v", err)
		}

		// Should be valid JSON
		var jsonData interface{}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			t.Errorf("Output should be valid JSON: %v", err)
		}
	})

	// Test directory output
	t.Run("Directory Output", func(t *testing.T) {
		tempDir := t.TempDir()

		buildOpts := launcher.BuildOptions{
			Output:        launcher.OutputDirectory,
			OutputPath:    tempDir,
			Format:        launcher.FormatYAML,
			SeparateFiles: true,
			IncludeIndex:  true,
		}

		if err := builder.Build(ctx, instance, buildOpts, opts); err != nil {
			t.Fatalf("Build failed: %v", err)
		}

		// Check files were created
		files, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("Failed to read directory: %v", err)
		}

		if len(files) < 2 {
			t.Errorf("Should have at least 2 files, got %d", len(files))
		}

		// Check file naming with index
		foundDeployment := false
		foundService := false
		for _, file := range files {
			name := file.Name()
			if strings.Contains(name, "deployment") {
				foundDeployment = true
			}
			if strings.Contains(name, "service") {
				foundService = true
			}
		}

		if !foundDeployment {
			t.Error("Should have deployment file")
		}
		if !foundService {
			t.Error("Should have service file")
		}
	})
}

func TestIntegration_Filtering(t *testing.T) {
	ctx := context.Background()
	log := logger.Noop()

	// Create components
	loader := launcher.NewPackageLoader(log)
	builder := launcher.NewBuilder(log)

	// Load simple package
	packagePath := "testdata/packages/simple"
	opts := launcher.DefaultOptions()
	opts.Logger = log

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	instance := &launcher.PackageInstance{
		Definition:     def,
		UserValues:     make(launcher.ParameterMap),
		EnabledPatches: []launcher.Patch{},
	}

	// Test kind filter
	t.Run("Filter by Kind", func(t *testing.T) {
		tempDir := t.TempDir()
		outputPath := filepath.Join(tempDir, "deployment.yaml")

		buildOpts := launcher.BuildOptions{
			Output:     launcher.OutputFile,
			OutputPath: outputPath,
			Format:     launcher.FormatYAML,
			FilterKind: "Deployment",
		}

		if err := builder.Build(ctx, instance, buildOpts, opts); err != nil {
			t.Fatalf("Build failed: %v", err)
		}

		// Read and verify output
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("Failed to read output: %v", err)
		}

		output := string(data)
		if !strings.Contains(output, "kind: Deployment") {
			t.Error("Output should contain Deployment")
		}
		if strings.Contains(output, "kind: Service") {
			t.Error("Output should not contain Service")
		}
	})

	// Test name filter
	t.Run("Filter by Name", func(t *testing.T) {
		tempDir := t.TempDir()
		outputPath := filepath.Join(tempDir, "filtered.yaml")

		buildOpts := launcher.BuildOptions{
			Output:     launcher.OutputFile,
			OutputPath: outputPath,
			Format:     launcher.FormatYAML,
			FilterName: "simple-app",
		}

		if err := builder.Build(ctx, instance, buildOpts, opts); err != nil {
			t.Fatalf("Build failed: %v", err)
		}

		// Read and verify output
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("Failed to read output: %v", err)
		}

		output := string(data)
		if !strings.Contains(output, "name: simple-app") {
			t.Error("Output should contain resources named simple-app")
		}
	})
}

func TestIntegration_Labels(t *testing.T) {
	ctx := context.Background()
	log := logger.Noop()

	// Create components
	loader := launcher.NewPackageLoader(log)
	builder := launcher.NewBuilder(log)

	// Load simple package
	packagePath := "testdata/packages/simple"
	opts := launcher.DefaultOptions()
	opts.Logger = log

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	instance := &launcher.PackageInstance{
		Definition:     def,
		UserValues:     make(launcher.ParameterMap),
		EnabledPatches: []launcher.Patch{},
	}

	// Test adding labels
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "labeled.yaml")

	buildOpts := launcher.BuildOptions{
		Output:     launcher.OutputFile,
		OutputPath: outputPath,
		Format:     launcher.FormatYAML,
		AddLabels: map[string]string{
			"managed-by": "kurel",
			"team":       "platform",
		},
	}

	if err := builder.Build(ctx, instance, buildOpts, opts); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Read and verify output
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "managed-by: kurel") {
		t.Error("Output should contain added label managed-by")
	}
	if !strings.Contains(output, "team: platform") {
		t.Error("Output should contain added label team")
	}
}

// Benchmark tests
func BenchmarkIntegration_SimplePackage(b *testing.B) {
	ctx := context.Background()
	log := logger.Noop()

	// Create all components
	loader := launcher.NewPackageLoader(log)
	resolver := launcher.NewResolver(log)
	processor := launcher.NewPatchProcessor(log, resolver)
	builder := launcher.NewBuilder(log)

	// Load package once
	packagePath := "testdata/packages/simple"
	opts := launcher.DefaultOptions()
	opts.Logger = log

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		b.Fatalf("Failed to load package: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Resolve patches
		enabledPatches, _ := processor.ResolveDependencies(ctx, def.Patches, def.Parameters)

		// Create instance
		instance := &launcher.PackageInstance{
			Definition:     def,
			UserValues:     make(launcher.ParameterMap),
			EnabledPatches: enabledPatches,
		}

		// Build to discard writer
		buildOpts := launcher.BuildOptions{
			Output: launcher.OutputStdout,
			Format: launcher.FormatYAML,
		}

		_ = builder.Build(ctx, instance, buildOpts, opts)
	}
}

func BenchmarkIntegration_ComplexPackage(b *testing.B) {
	ctx := context.Background()
	log := logger.Noop()

	// Create all components
	loader := launcher.NewPackageLoader(log)
	resolver := launcher.NewResolver(log)
	processor := launcher.NewPatchProcessor(log, resolver)
	builder := launcher.NewBuilder(log)

	// Load package once
	packagePath := "testdata/packages/complex"
	opts := launcher.DefaultOptions()
	opts.Logger = log

	def, err := loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		b.Fatalf("Failed to load package: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Set production environment
		def.Parameters["environment"] = "production"

		// Resolve patches
		enabledPatches, _ := processor.ResolveDependencies(ctx, def.Patches, def.Parameters)

		// Create instance
		instance := &launcher.PackageInstance{
			Definition:     def,
			UserValues:     make(launcher.ParameterMap),
			EnabledPatches: enabledPatches,
		}

		// Build to discard writer
		buildOpts := launcher.BuildOptions{
			Output: launcher.OutputStdout,
			Format: launcher.FormatYAML,
		}

		_ = builder.Build(ctx, instance, buildOpts, opts)
	}
}
