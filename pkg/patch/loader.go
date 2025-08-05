package patch

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type RawPatchMap map[string]interface{}

type TargetedPatch struct {
	Target string                 `yaml:"target"`
	Patch  map[string]interface{} `yaml:"patch"`
}

// PatchSpec ties a parsed PatchOp to an optional explicit target.
type PatchSpec struct {
	Target string
	Patch  PatchOp
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
				if Debug {
					log.Printf("Targeted patch loaded: target=%s op=%s path=%s value=%v", entry.Target, op.Op, op.Path, substitutedValue)
				}
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

		if Debug {
			log.Printf("TOML patch loaded: header=%s target=%s op=%s path=%s value=%v",
				currentHeader.String(), resourceTarget, op.Op, op.Path, value)
		}

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
			if Debug {
				log.Printf("Loaded resource: kind=%s name=%s", u.GetKind(), u.GetName())
			}
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

// NewPatchableAppSet constructs a PatchableAppSet from already loaded resources
// and parsed patch specifications.
func NewPatchableAppSet(resources []*unstructured.Unstructured, patches []PatchSpec) (*PatchableAppSet, error) {
	var wrapped []struct {
		Target string
		Patch  PatchOp
	}

	for _, spec := range patches {
		p := spec.Patch
		if err := p.NormalizePath(); err != nil {
			return nil, fmt.Errorf("invalid patch path syntax: %s: %w", p.Path, err)
		}

		var target string
		var trimmed string
		if spec.Target != "" {
			if !resourceExists(resources, spec.Target) {
				return nil, fmt.Errorf("explicit target not found: %s", spec.Target)
			}
			// Extract actual resource name from kind.name format if needed
			target = extractResourceName(resources, spec.Target)
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

		if Debug {
			log.Printf("Patch resolved: target=%s op=%s path=%s value=%v", target, p.Op, p.Path, p.Value)
		}
		wrapped = append(wrapped, struct {
			Target string
			Patch  PatchOp
		}{Target: target, Patch: p})
	}

	return &PatchableAppSet{
		Resources: resources,
		Patches:   wrapped,
	}, nil
}

// NewPatchableAppSetWithStructure constructs a PatchableAppSet with YAML structure preservation
func NewPatchableAppSetWithStructure(documentSet *YAMLDocumentSet, patches []PatchSpec) (*PatchableAppSet, error) {
	resources := documentSet.GetResources()

	var wrapped []struct {
		Target string
		Patch  PatchOp
	}

	for _, spec := range patches {
		p := spec.Patch
		if err := p.NormalizePath(); err != nil {
			return nil, fmt.Errorf("invalid patch path syntax: %s: %w", p.Path, err)
		}

		var target string
		var trimmed string
		if spec.Target != "" {
			if !resourceExists(resources, spec.Target) {
				return nil, fmt.Errorf("explicit target not found: %s", spec.Target)
			}
			// Preserve kind.name format when needed for disambiguation
			target = preserveTargetForDisambiguation(resources, spec.Target)
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

		if Debug {
			log.Printf("Patch resolved: target=%s op=%s path=%s value=%v", target, p.Op, p.Path, p.Value)
		}
		wrapped = append(wrapped, struct {
			Target string
			Patch  PatchOp
		}{Target: target, Patch: p})
	}

	return &PatchableAppSet{
		Resources:   resources,
		DocumentSet: documentSet,
		Patches:     wrapped,
	}, nil
}
