package patch

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/logger"
)

// patchLogger is a package-level logger for debug messages.
var patchLogger = logger.New(logger.Options{
	Output: os.Stderr,
	Level:  logger.LevelDebug,
	Prefix: "patch",
})

// debugLog logs a debug message when the Debug flag is set.
func debugLog(format string, args ...interface{}) {
	if Debug {
		patchLogger.Debug(format, args...)
	}
}

type RawPatchMap map[string]interface{}

type TargetedPatch struct {
	Target string                 `yaml:"target"`
	Type   string                 `yaml:"type,omitempty"` // "" (field-level) or "strategic"
	Patch  map[string]interface{} `yaml:"patch"`
}

// PatchSpec ties a parsed PatchOp to an optional explicit target.
// For strategic merge patches, Strategic is non-nil and Patch is zero-value.
type PatchSpec struct {
	Target    string
	Patch     PatchOp         // field-level patch (zero value when Strategic is set)
	Strategic *StrategicPatch // non-nil for strategic merge patches
}

var Debug = os.Getenv("KURE_DEBUG") == "1"

func LoadPatchFile(r io.Reader) ([]PatchSpec, error) {
	return LoadPatchFileWithVariables(r, nil)
}

func LoadPatchFileWithVariables(r io.Reader, varCtx *VariableContext) ([]PatchSpec, error) {
	// Read all content to detect format
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read patch content: %w", err)
	}

	contentStr := string(content)

	// Detect format and delegate to appropriate parser
	if IsTOMLFormat(contentStr) {
		return LoadTOMLPatchFile(strings.NewReader(contentStr), varCtx)
	} else {
		return LoadYAMLPatchFile(strings.NewReader(contentStr), varCtx)
	}
}

func LoadYAMLPatchFile(r io.Reader, varCtx *VariableContext) ([]PatchSpec, error) {
	dec := yaml.NewDecoder(r)

	var firstToken yaml.Node
	if err := dec.Decode(&firstToken); err != nil {
		return nil, fmt.Errorf("failed to read patch input: %w", err)
	}
	if firstToken.Kind == yaml.DocumentNode && len(firstToken.Content) > 0 {
		firstToken = *firstToken.Content[0]
	}

	var patches []PatchSpec

	if firstToken.Kind == yaml.MappingNode {
		var raw RawPatchMap
		if err := firstToken.Decode(&raw); err != nil {
			return nil, fmt.Errorf("invalid simple patch map: %w", err)
		}
		for k, v := range raw {
			// Apply variable substitution
			substitutedValue, err := SubstituteVariables(fmt.Sprintf("%v", v), varCtx)
			if err != nil {
				return nil, fmt.Errorf("variable substitution failed for key '%s': %w", k, err)
			}

			// Apply type inference to convert strings to appropriate types (int, bool, etc.)
			if valueStr, ok := substitutedValue.(string); ok {
				substitutedValue = inferValueType(k, valueStr)
			}

			op, err := ParsePatchLine(k, substitutedValue)
			if err != nil {
				return nil, fmt.Errorf("invalid patch line '%s': %w", k, err)
			}
			patches = append(patches, PatchSpec{Patch: op})
		}
	} else if firstToken.Kind == yaml.SequenceNode {
		var list []TargetedPatch
		if err := firstToken.Decode(&list); err != nil {
			return nil, fmt.Errorf("invalid patch list: %w", err)
		}
		for _, entry := range list {
			if entry.Type != "" && entry.Type != "strategic" {
				return nil, fmt.Errorf("unknown patch type %q for target %q: must be \"\" or \"strategic\"", entry.Type, entry.Target)
			}
			if entry.Type == "strategic" {
				if len(entry.Patch) == 0 {
					return nil, fmt.Errorf("strategic patch targeting '%s' has no patch payload", entry.Target)
				}
				substitutedPatch, err := substituteVariablesInMap(entry.Patch, varCtx)
				if err != nil {
					return nil, fmt.Errorf("variable substitution failed for strategic patch targeting '%s': %w", entry.Target, err)
				}
				inferTypesInMap(substitutedPatch)
				debugLog("Strategic patch loaded: target=%s", entry.Target)
				patches = append(patches, PatchSpec{
					Target:    entry.Target,
					Strategic: &StrategicPatch{Patch: substitutedPatch},
				})
				continue
			}

			for k, v := range entry.Patch {
				// Apply variable substitution
				substitutedValue, err := SubstituteVariables(fmt.Sprintf("%v", v), varCtx)
				if err != nil {
					return nil, fmt.Errorf("variable substitution failed for key '%s': %w", k, err)
				}

				// Apply type inference to convert strings to appropriate types (int, bool, etc.)
				if valueStr, ok := substitutedValue.(string); ok {
					substitutedValue = inferValueType(k, valueStr)
				}

				op, err := ParsePatchLine(k, substitutedValue)
				if err != nil {
					return nil, fmt.Errorf("invalid patch line '%s': %w", k, err)
				}
				if err := op.NormalizePath(); err != nil {
					return nil, fmt.Errorf("invalid patch path syntax: %s: %w", op.Path, err)
				}
				debugLog("Targeted patch loaded: target=%s op=%s path=%s value=%v", entry.Target, op.Op, op.Path, substitutedValue)
				patches = append(patches, PatchSpec{Target: entry.Target, Patch: op})
			}
		}
	} else {
		return nil, fmt.Errorf("unrecognized patch format")
	}

	return patches, nil
}

func LoadTOMLPatchFile(r io.Reader, varCtx *VariableContext) ([]PatchSpec, error) {
	scanner := bufio.NewScanner(r)
	var patches []PatchSpec
	var currentHeader *TOMLHeader

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for TOML header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			header, err := ParseTOMLHeader(line)
			if err != nil {
				return nil, fmt.Errorf("invalid TOML header at line %d: %w", lineNum, err)
			}
			currentHeader = header
			continue
		}

		// Parse key-value pair
		if currentHeader == nil {
			return nil, fmt.Errorf("patch value without header at line %d: %s", lineNum, line)
		}

		// Split key: value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid patch line format at line %d: %s", lineNum, line)
		}

		key := strings.TrimSpace(parts[0])
		valueStr := strings.TrimSpace(parts[1])

		// Apply variable substitution to value
		value, err := SubstituteVariables(valueStr, varCtx)
		if err != nil {
			return nil, fmt.Errorf("variable substitution failed at line %d: %w", lineNum, err)
		}

		// Apply type inference to convert strings to appropriate types (int, bool, etc.)
		if valueStr, ok := value.(string); ok {
			value = inferValueType(key, valueStr)
		}

		// Convert TOML header to resource target and field path
		resourceTarget, fieldPath, err := currentHeader.ResolveTOMLPath()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve TOML path for header %s: %w", currentHeader.String(), err)
		}

		// Combine field path with key if we have a field path
		var finalPath string
		if fieldPath != "" {
			finalPath = fieldPath + "." + key
		} else {
			finalPath = key
		}

		// Create patch operation
		op, err := ParsePatchLine(finalPath, value)
		if err != nil {
			return nil, fmt.Errorf("invalid patch line '%s' at line %d: %w", finalPath, lineNum, err)
		}

		if err := op.NormalizePath(); err != nil {
			return nil, fmt.Errorf("invalid patch path syntax: %s: %w", op.Path, err)
		}

		debugLog("TOML patch loaded: header=%s target=%s op=%s path=%s value=%v",
			currentHeader.String(), resourceTarget, op.Op, op.Path, value)

		patches = append(patches, PatchSpec{Target: resourceTarget, Patch: op})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading patch file: %w", err)
	}

	return patches, nil
}

func LoadResourcesFromMultiYAML(r io.Reader) ([]*unstructured.Unstructured, error) {
	dec := yaml.NewDecoder(r)
	var resources []*unstructured.Unstructured
	for {
		var raw map[string]interface{}
		err := dec.Decode(&raw)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode resource document: %w", err)
		}
		if len(raw) > 0 {
			u := &unstructured.Unstructured{Object: raw}
			debugLog("Loaded resource: kind=%s name=%s", u.GetKind(), u.GetName())
			resources = append(resources, u)
		}
	}
	return resources, nil
}

func LoadPatchableAppSet(resourceReaders []io.Reader, patchReader io.Reader) (*PatchableAppSet, error) {
	var resources []*unstructured.Unstructured
	for _, r := range resourceReaders {
		rs, err := LoadResourcesFromMultiYAML(r)
		if err != nil {
			return nil, err
		}
		resources = append(resources, rs...)
	}

	patches, err := LoadPatchFile(patchReader)
	if err != nil {
		return nil, err
	}

	return NewPatchableAppSet(resources, patches)
}

func resolvePatchTarget(resources []*unstructured.Unstructured, path string) (string, string) {
	pathParts := parsePath(path)
	if len(pathParts) == 0 {
		return "", ""
	}
	first := strings.ToLower(pathParts[0])
	for _, r := range resources {
		name := strings.ToLower(r.GetName())
		kind := strings.ToLower(r.GetKind())
		if first == name || first == fmt.Sprintf("%s.%s", kind, name) {
			trimmed := strings.Join(pathParts[1:], ".")
			return r.GetName(), trimmed
		}
	}
	return "", ""
}

func resourceExists(resources []*unstructured.Unstructured, name string) bool {
	for _, r := range resources {
		// Check direct name match
		if r.GetName() == name {
			return true
		}
		// Check kind.name format match
		kindName := fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), r.GetName())
		if strings.ToLower(name) == kindName {
			return true
		}
	}
	return false
}

func extractResourceName(resources []*unstructured.Unstructured, target string) string {
	for _, r := range resources {
		// Check direct name match
		if r.GetName() == target {
			return target
		}
		// Check kind.name format match
		kindName := fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), r.GetName())
		if strings.ToLower(target) == kindName {
			return r.GetName()
		}
	}
	return target // fallback to original target if no match
}

// CanonicalResourceKey returns the unique key for a resource.
// For namespaced resources: "namespace/kind.name"
// For cluster-scoped resources: "kind.name"
func CanonicalResourceKey(r *unstructured.Unstructured) string {
	kindName := fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), r.GetName())
	if ns := r.GetNamespace(); ns != "" {
		return fmt.Sprintf("%s/%s", ns, kindName)
	}
	return kindName
}

// ResolveTargetKey resolves a patch target to its canonical resource key.
// Accepts short names ("my-app"), kind-qualified names ("deployment.my-app"),
// and namespace-qualified names ("staging/deployment.my-app").
// Returns an error if the target matches no resource or is ambiguous.
func ResolveTargetKey(resources []*unstructured.Unstructured, target string) (string, error) {
	// Check for namespace/kind.name format
	if idx := strings.Index(target, "/"); idx > 0 {
		ns := target[:idx]
		rest := target[idx+1:]
		lowRest := strings.ToLower(rest)
		for _, r := range resources {
			kindName := fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), r.GetName())
			if lowRest == kindName && r.GetNamespace() == ns {
				return CanonicalResourceKey(r), nil
			}
		}
		return "", fmt.Errorf("target %q not found in base resources", target)
	}

	// Try kind.name match first (case-insensitive)
	lowTarget := strings.ToLower(target)
	var kindNameMatches []*unstructured.Unstructured
	for _, r := range resources {
		kindName := fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), r.GetName())
		if lowTarget == kindName {
			kindNameMatches = append(kindNameMatches, r)
		}
	}
	if len(kindNameMatches) == 1 {
		return CanonicalResourceKey(kindNameMatches[0]), nil
	}
	if len(kindNameMatches) > 1 {
		return "", fmt.Errorf("target %q is ambiguous, matches %d resources; use namespace/kind.name to disambiguate",
			target, len(kindNameMatches))
	}

	// Try short name match
	var shortMatches []*unstructured.Unstructured
	for _, r := range resources {
		if r.GetName() == target {
			shortMatches = append(shortMatches, r)
		}
	}

	switch len(shortMatches) {
	case 0:
		return "", fmt.Errorf("target %q not found in base resources", target)
	case 1:
		return CanonicalResourceKey(shortMatches[0]), nil
	default:
		names := make([]string, len(shortMatches))
		for i, r := range shortMatches {
			names[i] = fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), r.GetName())
		}
		return "", fmt.Errorf("target %q is ambiguous, matches: %s; use kind.name format",
			target, strings.Join(names, ", "))
	}
}

// preserveTargetForDisambiguation returns the target string, preserving kind.name format
// when there are multiple resources with the same name but different kinds
func preserveTargetForDisambiguation(resources []*unstructured.Unstructured, target string) string {
	// If it's already a kind.name format and exists, keep it as-is
	if strings.Contains(target, ".") && resourceExists(resources, target) {
		return target
	}

	// If it's just a name, check if there are multiple resources with this name
	nameCount := 0
	for _, r := range resources {
		if r.GetName() == target {
			nameCount++
		}
	}

	// If there's only one resource with this name, we can use the short name
	if nameCount <= 1 {
		return target
	}

	// Multiple resources with same name - we need to keep the kind.name format
	// Try to find the original kind.name format that matches this target
	for _, r := range resources {
		kindName := fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), r.GetName())
		if strings.ToLower(target) == strings.ToLower(r.GetName()) {
			// This could be ambiguous, but we need to keep the original target if possible
			continue
		}
		if strings.ToLower(target) == kindName {
			return kindName
		}
	}

	return target
}

// smartTarget attempts to match a patch to a resource based on field presence.
func smartTarget(resources []*unstructured.Unstructured, p PatchOp) []string {
	var matches []string
	for _, r := range resources {
		if err := p.ValidateAgainst(r); err == nil {
			matches = append(matches, r.GetName())
		}
	}
	return matches
}

// resolvedPatch is the internal type used by resolvePatches.
type resolvedPatch struct {
	Patch     PatchOp
	Strategic *StrategicPatch
	Target    string
}

// resolvePatches is the shared resolution logic for NewPatchableAppSet and
// NewPatchableAppSetWithStructure. It normalizes paths, resolves targets,
// and returns wrapped patches ready for inclusion in a PatchableAppSet.
func resolvePatches(resources []*unstructured.Unstructured, patches []PatchSpec) ([]resolvedPatch, error) {
	var wrapped []resolvedPatch

	for _, spec := range patches {
		// Handle strategic merge patches
		if spec.Strategic != nil {
			target := spec.Target
			if target == "" {
				return nil, fmt.Errorf("strategic merge patch requires an explicit target")
			}
			resolved, err := ResolveTargetKey(resources, target)
			if err != nil {
				return nil, fmt.Errorf("strategic merge patch: %w", err)
			}
			target = resolved

			debugLog("Strategic patch resolved: target=%s", target)
			wrapped = append(wrapped, resolvedPatch{Target: target, Strategic: spec.Strategic})
			continue
		}

		p := spec.Patch
		if err := p.NormalizePath(); err != nil {
			return nil, fmt.Errorf("invalid patch path syntax: %s: %w", p.Path, err)
		}

		var target string
		var trimmed string
		if spec.Target != "" {
			resolved, err := ResolveTargetKey(resources, spec.Target)
			if err != nil {
				if strings.Contains(err.Error(), "not found in base resources") {
					return nil, fmt.Errorf("explicit target not found: %s", spec.Target)
				}
				return nil, fmt.Errorf("explicit target %q: %w", spec.Target, err)
			}
			target = resolved
		} else {
			target, trimmed = resolvePatchTarget(resources, p.Path)
			if target == "" {
				cands := smartTarget(resources, p)
				if len(cands) == 1 {
					target = cands[0]
				}
			}
		}

		if target == "" {
			return nil, fmt.Errorf("could not determine target resource for patch path: %s", p.Path)
		}

		if trimmed != "" {
			p.Path = trimmed
		}

		debugLog("Patch resolved: target=%s op=%s path=%s value=%v", target, p.Op, p.Path, p.Value)
		wrapped = append(wrapped, resolvedPatch{Target: target, Patch: p})
	}

	return wrapped, nil
}

// toAppSetPatches converts resolvedPatch slice to the PatchableAppSet.Patches type.
func toAppSetPatches(resolved []resolvedPatch) []struct {
	Target    string
	Patch     PatchOp
	Strategic *StrategicPatch
} {
	out := make([]struct {
		Target    string
		Patch     PatchOp
		Strategic *StrategicPatch
	}, len(resolved))
	for i, r := range resolved {
		out[i] = struct {
			Target    string
			Patch     PatchOp
			Strategic *StrategicPatch
		}{Target: r.Target, Patch: r.Patch, Strategic: r.Strategic}
	}
	return out
}

// NewPatchableAppSet constructs a PatchableAppSet from already loaded resources
// and parsed patch specifications.
func NewPatchableAppSet(resources []*unstructured.Unstructured, patches []PatchSpec) (*PatchableAppSet, error) {
	resolved, err := resolvePatches(resources, patches)
	if err != nil {
		return nil, err
	}
	return &PatchableAppSet{
		Resources: resources,
		Patches:   toAppSetPatches(resolved),
	}, nil
}

// NewPatchableAppSetWithStructure constructs a PatchableAppSet with YAML structure preservation.
func NewPatchableAppSetWithStructure(documentSet *YAMLDocumentSet, patches []PatchSpec) (*PatchableAppSet, error) {
	resources := documentSet.GetResources()
	resolved, err := resolvePatches(resources, patches)
	if err != nil {
		return nil, err
	}
	return &PatchableAppSet{
		Resources:   resources,
		DocumentSet: documentSet,
		Patches:     toAppSetPatches(resolved),
	}, nil
}

// substituteVariablesInMap recursively applies variable substitution to all
// string leaf values in a map[string]interface{}.
func substituteVariablesInMap(m map[string]interface{}, ctx *VariableContext) (map[string]interface{}, error) {
	if ctx == nil {
		return m, nil
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		substituted, err := substituteVariablesInValue(v, ctx)
		if err != nil {
			return nil, err
		}
		result[k] = substituted
	}
	return result, nil
}

func substituteVariablesInValue(v interface{}, ctx *VariableContext) (interface{}, error) {
	switch val := v.(type) {
	case string:
		return SubstituteVariables(val, ctx)
	case map[string]interface{}:
		return substituteVariablesInMap(val, ctx)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			s, err := substituteVariablesInValue(item, ctx)
			if err != nil {
				return nil, err
			}
			result[i] = s
		}
		return result, nil
	default:
		return v, nil
	}
}

// inferTypesInMap recursively applies type inference to string leaf values
// in a map, converting numeric and boolean strings to their typed equivalents.
// This matches the type inference applied to field-level patch values.
func inferTypesInMap(m map[string]interface{}) {
	for k, v := range m {
		m[k] = inferTypesInValue(k, v)
	}
}

func inferTypesInValue(key string, v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		return inferValueType(key, val)
	case map[string]interface{}:
		inferTypesInMap(val)
		return val
	case []interface{}:
		for i, item := range val {
			val[i] = inferTypesInValue(key, item)
		}
		return val
	default:
		return v
	}
}
