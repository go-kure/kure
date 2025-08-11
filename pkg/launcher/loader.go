package launcher

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/logger"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// packageLoader implements the PackageLoader interface
type packageLoader struct {
	logger logger.Logger
	mu     sync.RWMutex
}

// NewPackageLoader creates a new package loader
func NewPackageLoader(log logger.Logger) PackageLoader {
	if log == nil {
		log = logger.Default()
	}
	return &packageLoader{
		logger: log,
	}
}

// LoadDefinition loads a package definition from the specified path
func (l *packageLoader) LoadDefinition(ctx context.Context, path string, opts *LauncherOptions) (*PackageDefinition, error) {
	if opts == nil {
		opts = DefaultOptions()
	}
	if opts.Logger == nil {
		opts.Logger = logger.Default()
	}

	l.logger.Debug("Loading package definition from %s", path)

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.NewFileError("stat", path, "path not found", err)
	}

	// Determine package root
	var pkgRoot string
	if info.IsDir() {
		pkgRoot = path
	} else {
		pkgRoot = filepath.Dir(path)
	}

	// Create partial definition
	def := &PackageDefinition{
		Path:       pkgRoot,
		Parameters: make(ParameterMap),
		Resources:  []Resource{},
		Patches:    []Patch{},
	}

	// Collect errors for hybrid error handling
	loadErrors := NewLoadErrors(def, nil)

	// Load kurel.yaml metadata
	if err := l.loadMetadata(ctx, def, pkgRoot, opts); err != nil {
		if !IsCriticalError(err) {
			loadErrors.Issues = append(loadErrors.Issues, err)
		} else {
			return nil, err
		}
	}

	// Load parameters.yaml (non-critical - package can work without parameters)
	if err := l.loadParameters(ctx, def, pkgRoot, opts); err != nil {
		loadErrors.Issues = append(loadErrors.Issues, err)
		// Continue even if parameters fail to load
	}

	// Load resources
	resources, err := l.LoadResources(ctx, pkgRoot, opts)
	if err != nil {
		if !IsCriticalError(err) {
			loadErrors.Issues = append(loadErrors.Issues, err)
		} else {
			return nil, err
		}
	}
	def.Resources = resources

	// Load patches
	patches, err := l.LoadPatches(ctx, pkgRoot, opts)
	if err != nil {
		if !IsCriticalError(err) {
			loadErrors.Issues = append(loadErrors.Issues, err)
		} else {
			return nil, err
		}
	}
	def.Patches = patches

	// Enforce patch name uniqueness
	if err := l.validatePatchUniqueness(patches); err != nil {
		loadErrors.Issues = append(loadErrors.Issues, err)
	}

	// Return with any non-critical errors
	if len(loadErrors.Issues) > 0 {
		l.logger.Warn("Package loaded with %d issues", len(loadErrors.Issues))
		return def, loadErrors
	}

	l.logger.Info("Successfully loaded package %s", def.Metadata.Name)
	return def, nil
}

// loadMetadata loads the kurel.yaml file
func (l *packageLoader) loadMetadata(ctx context.Context, def *PackageDefinition, pkgRoot string, opts *LauncherOptions) error {
	metaPath := filepath.Join(pkgRoot, "kurel.yaml")

	// Check if file exists
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		// Metadata is optional, use defaults
		def.Metadata = KurelMetadata{
			Name:    filepath.Base(pkgRoot),
			Version: "0.0.0",
		}
		l.logger.Debug("No kurel.yaml found, using defaults")
		return nil
	}

	// Load the metadata file
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return errors.NewFileError("read", metaPath, "failed to read metadata", err)
	}

	// Parse YAML
	var meta KurelMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return errors.NewParseError(metaPath, "invalid YAML syntax", 0, 0, err)
	}

	def.Metadata = meta
	return nil
}

// loadParameters loads the parameters.yaml file
func (l *packageLoader) loadParameters(ctx context.Context, def *PackageDefinition, pkgRoot string, opts *LauncherOptions) error {
	paramPath := filepath.Join(pkgRoot, "parameters.yaml")

	// Check if file exists
	if _, err := os.Stat(paramPath); os.IsNotExist(err) {
		// Parameters are optional
		l.logger.Debug("No parameters.yaml found")
		return nil
	}

	// Load the parameters file
	data, err := os.ReadFile(paramPath)
	if err != nil {
		return errors.NewFileError("read", paramPath, "failed to read parameters", err)
	}

	// Parse YAML into ParameterMap
	var params ParameterMap
	if err := yaml.Unmarshal(data, &params); err != nil {
		return errors.NewParseError(paramPath, "invalid YAML syntax", 0, 0, err)
	}

	def.Parameters = params
	return nil
}

// LoadResources loads Kubernetes resources from the package
func (l *packageLoader) LoadResources(ctx context.Context, path string, opts *LauncherOptions) ([]Resource, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	l.logger.Debug("Loading resources from %s", path)

	// Determine resource directory
	resourceDir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		resourceDir = filepath.Dir(path)
	}

	// Look for resources in standard locations
	var resources []Resource
	seenResources := make(map[string]bool) // Track processed files to avoid duplicates
	locations := []string{
		filepath.Join(resourceDir, "resources"),
		filepath.Join(resourceDir, "manifests"),
		resourceDir, // Also check root directory
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err != nil {
			continue
		}

		// Find all YAML files
		err := filepath.Walk(loc, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip files with errors
			}

			// Skip directories and non-YAML files
			if info.IsDir() || !isYAMLFile(path) {
				return nil
			}

			// Skip patch files
			if strings.Contains(path, "patches/") || strings.HasSuffix(path, ".kpatch") {
				return nil
			}

			// Skip package metadata and parameter files
			filename := filepath.Base(path)
			if filename == "kurel.yaml" || filename == "parameters.yaml" {
				return nil
			}

			// Skip files we've already processed
			if seenResources[path] {
				return nil
			}
			seenResources[path] = true

			// Read raw file content first (may contain templates)
			rawData, err := os.ReadFile(path)
			if err != nil {
				l.logger.Warn("Failed to read %s: %v", path, err)
				return nil
			}

			// Try to parse as Kubernetes objects
			// If it contains variables, this will fail and we'll store as template
			objs, parseErr := io.ParseYAML(rawData)
			if parseErr != nil {
				// File likely contains template variables, store as template data
				l.logger.Debug("File %s contains templates, deferring parsing: %v", path, parseErr)

				// Create a placeholder resource with template data
				// We'll need basic metadata to identify it later
				if err := l.loadTemplateResource(path, rawData, &resources); err != nil {
					l.logger.Warn("Failed to load template resource %s: %v", path, err)
				}
				return nil
			}

			// Successfully parsed - convert to launcher Resources
			for _, obj := range objs {
				res, err := l.clientObjectToResource(obj)
				if err != nil {
					l.logger.Warn("Failed to convert object from %s: %v", path, err)
					continue
				}
				// Also store the raw template data for later processing
				res.TemplateData = rawData
				resources = append(resources, res)
			}

			return nil
		})

		if err != nil {
			l.logger.Warn("Error walking directory %s: %v", loc, err)
		}
	}

	l.logger.Info("Loaded %d resources", len(resources))
	return resources, nil
}

// loadTemplateResource creates a placeholder resource for template files
func (l *packageLoader) loadTemplateResource(path string, rawData []byte, resources *[]Resource) error {
	// Extract basic resource info from the template by looking for YAML structure
	// This is a heuristic approach - we'll look for apiVersion, kind, metadata patterns
	content := string(rawData)

	// Try to extract apiVersion and kind even with variable substitution
	var apiVersion, kind, name, namespace string

	// Look for apiVersion (may contain variables)
	if matches := regexp.MustCompile(`apiVersion:\s*(.+)`).FindStringSubmatch(content); len(matches) > 1 {
		apiVersion = strings.TrimSpace(matches[1])
	}

	// Look for kind (may contain variables)
	if matches := regexp.MustCompile(`kind:\s*(.+)`).FindStringSubmatch(content); len(matches) > 1 {
		kind = strings.TrimSpace(matches[1])
	}

	// Look for metadata.name (may contain variables)
	if matches := regexp.MustCompile(`name:\s*(.+)`).FindStringSubmatch(content); len(matches) > 1 {
		name = strings.TrimSpace(matches[1])
	}

	// Look for metadata.namespace (may contain variables)
	if matches := regexp.MustCompile(`namespace:\s*(.+)`).FindStringSubmatch(content); len(matches) > 1 {
		namespace = strings.TrimSpace(matches[1])
	}

	// Create a placeholder resource
	resource := Resource{
		APIVersion:   apiVersion,
		Kind:         kind,
		TemplateData: rawData,
		Metadata: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	*resources = append(*resources, resource)
	l.logger.Debug("Loaded template resource: %s/%s from %s", kind, name, path)
	return nil
}

// LoadPatches loads patch files from the package
func (l *packageLoader) LoadPatches(ctx context.Context, path string, opts *LauncherOptions) ([]Patch, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	l.logger.Debug("Loading patches from %s", path)

	// Determine patch directory
	patchDir := filepath.Join(path, "patches")
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		patchDir = filepath.Join(filepath.Dir(path), "patches")
	}

	// Check if patches directory exists
	if _, err := os.Stat(patchDir); os.IsNotExist(err) {
		l.logger.Debug("No patches directory found")
		return nil, nil
	}

	var patches []Patch

	// Find all patch files
	err := filepath.Walk(patchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Skip directories and non-patch files
		if info.IsDir() || !isPatchFile(path) {
			return nil
		}

		// Load patch file
		patch, err := l.loadPatchFile(path)
		if err != nil {
			l.logger.Warn("Failed to load patch %s: %v", path, err)
			return nil // Continue with other patches
		}

		patches = append(patches, patch)
		return nil
	})

	if err != nil {
		return nil, errors.NewFileError("walk", patchDir, "failed to scan patches", err)
	}

	l.logger.Info("Loaded %d patches", len(patches))
	return patches, nil
}

// loadPatchFile loads a single patch file
func (l *packageLoader) loadPatchFile(path string) (Patch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Patch{}, errors.NewFileError("read", path, "failed to read patch", err)
	}

	// Extract patch name from filename
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	patch := Patch{
		Name:    name,
		Path:    path,
		Content: string(data),
	}

	// Check for metadata header in the patch file
	if strings.HasPrefix(string(data), "# kurel:") {
		patch.Metadata = l.parsePatchMetadata(string(data))
	}

	return patch, nil
}

// parsePatchMetadata extracts metadata from patch file comments
func (l *packageLoader) parsePatchMetadata(content string) *PatchMetadata {
	lines := strings.Split(content, "\n")
	meta := &PatchMetadata{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "# kurel:") {
			break // Stop at first non-metadata line
		}

		// Parse metadata directives
		directive := strings.TrimPrefix(line, "# kurel:")
		parts := strings.SplitN(directive, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "enabled":
			meta.Enabled = value
		case "description":
			meta.Description = value
		case "requires":
			meta.Requires = strings.Fields(value)
		case "conflicts":
			meta.Conflicts = strings.Fields(value)
		}
	}

	return meta
}

// clientObjectToResource converts a client.Object to a launcher Resource
func (l *packageLoader) clientObjectToResource(obj client.Object) (Resource, error) {
	// Convert to unstructured
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Try to convert through runtime
		data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return Resource{}, errors.Wrap(err, "failed to convert to unstructured")
		}
		u = &unstructured.Unstructured{Object: data}
	}

	gvk := u.GetObjectKind().GroupVersionKind()

	// Extract metadata
	meta := metav1.ObjectMeta{
		Name:        u.GetName(),
		Namespace:   u.GetNamespace(),
		Labels:      u.GetLabels(),
		Annotations: u.GetAnnotations(),
	}

	return Resource{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Metadata:   meta,
		Raw:        u.DeepCopy(),
	}, nil
}

// validatePatchUniqueness ensures all patch names are unique
func (l *packageLoader) validatePatchUniqueness(patches []Patch) error {
	seen := make(map[string]bool)
	var duplicates []string

	for _, patch := range patches {
		if seen[patch.Name] {
			duplicates = append(duplicates, patch.Name)
		}
		seen[patch.Name] = true
	}

	if len(duplicates) > 0 {
		return NewDependencyError("conflict", "patches", strings.Join(duplicates, ", "), nil)
	}

	return nil
}

// isYAMLFile checks if a file is a YAML file
func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// isPatchFile checks if a file is a patch file
func isPatchFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".kpatch" || ext == ".patch" || ext == ".toml" ||
		(isYAMLFile(path) && strings.Contains(path, "patches/"))
}
