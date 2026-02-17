package patch

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/errors"
)

// PatchOp represents a single patch operation to apply to an object.
type PatchOp struct {
	Op         string      `json:"op"`
	Path       string      `json:"path"`
	ParsedPath []PathPart  `json:"patsedpath,omitempty"`
	Selector   string      `json:"selector,omitempty"`
	Value      interface{} `json:"value"`
}

// ResourceWithPatches ties a base object with the patches that should be applied to it.
type ResourceWithPatches struct {
	Name             string
	Base             *unstructured.Unstructured
	Patches          []PatchOp
	StrategicPatches []StrategicPatch // applied before field-level patches
	KindLookup       KindLookup       // may be nil; used for strategic merge
}

// Apply executes all patches on the base object. Strategic merge patches are
// applied first (setting broad document shape), then field-level patches make
// precise tweaks on top.
func (r *ResourceWithPatches) Apply() error {
	for _, smp := range r.StrategicPatches {
		if err := ApplyStrategicMergePatch(r.Base, smp.Patch, r.KindLookup); err != nil {
			return errors.NewPatchError(
				"strategic-merge",
				"",
				r.Name,
				"strategic merge patch failed",
				err,
			)
		}
	}
	for _, patch := range r.Patches {
		if err := applyPatchOp(r.Base.Object, patch); err != nil {
			return errors.NewPatchError(
				patch.Op,
				patch.Path,
				r.Name,
				"patch application failed",
				err,
			)
		}
	}
	return nil
}

func applyPatchOp(obj map[string]interface{}, op PatchOp) error {
	switch op.Op {
	case "replace":
		// Handle array selector patches
		if op.Selector != "" {
			return applyArrayReplace(obj, op)
		}
		convertedValue := convertValueForUnstructured(op.Value)
		if err := unstructured.SetNestedField(obj, convertedValue, parsePath(op.Path)...); err != nil {
			return errors.NewPatchError(op.Op, op.Path, "", "failed to set field", err)
		}
		return nil
	case "delete":
		if op.Selector == "" {
			_, found, err := unstructured.NestedFieldNoCopy(obj, parsePath(op.Path)...)
			if err != nil {
				return errors.NewPatchError(op.Op, op.Path, "", "failed to access field", err)
			}
			if !found {
				return errors.NewPatchError(op.Op, op.Path, "", "path not found", nil)
			}
			unstructured.RemoveNestedField(obj, parsePath(op.Path)...)
			return nil
		}
		path := parsePath(op.Path)
		lst, found, err := unstructured.NestedSlice(obj, path...)
		if err != nil {
			return errors.NewPatchError(op.Op, op.Path, "", "failed to access list", err)
		}
		if !found {
			return errors.NewPatchError(op.Op, op.Path, "", "list not found", nil)
		}
		idx, err := resolveListIndex(lst, op.Selector)
		if err != nil {
			return errors.NewPatchError(op.Op, op.Path, "", "failed to resolve list index", err)
		}
		if idx < 0 || idx >= len(lst) {
			return errors.NewPatchError(op.Op, op.Path, "", fmt.Sprintf("index %d out of bounds for list of length %d", idx, len(lst)), nil)
		}
		lst = append(lst[:idx], lst[idx+1:]...)
		return unstructured.SetNestedSlice(obj, lst, path...)
	case "append":
		lst, found, err := unstructured.NestedSlice(obj, parsePath(op.Path)...)
		if err != nil {
			return errors.NewPatchError(op.Op, op.Path, "", "failed to access list", err)
		}
		if !found {
			return errors.NewPatchError(op.Op, op.Path, "", "list not found", nil)
		}
		convertedValue := convertValueForUnstructured(op.Value)
		lst = append(lst, convertedValue)
		if err := unstructured.SetNestedSlice(obj, lst, parsePath(op.Path)...); err != nil {
			return errors.NewPatchError(op.Op, op.Path, "", "failed to update list", err)
		}
		return nil
	case "insertBefore", "insertAfter":
		return applyListPatch(obj, op)
	default:
		return errors.NewValidationError("operation", op.Op, "patch", []string{"replace", "delete", "append", "insertBefore", "insertAfter"})
	}
}

func applyListPatch(obj map[string]interface{}, op PatchOp) error {
	path := parsePath(op.Path)
	lst, found, err := unstructured.NestedSlice(obj, path...)
	if err != nil {
		return errors.NewPatchError(op.Op, op.Path, "", "failed to access list", err)
	}
	if !found {
		return errors.NewPatchError(op.Op, op.Path, "", "list not found", nil)
	}

	idx, err := resolveListIndex(lst, op.Selector)
	if err != nil {
		return errors.NewPatchError(op.Op, op.Path, "", "failed to resolve list index", err)
	}

	convertedValue := convertValueForUnstructured(op.Value)
	switch op.Op {
	case "insertBefore":
		lst = append(lst[:idx], append([]interface{}{convertedValue}, lst[idx:]...)...)
	case "insertAfter":
		lst = append(lst[:idx+1], append([]interface{}{convertedValue}, lst[idx+1:]...)...)
	}

	if err := unstructured.SetNestedSlice(obj, lst, path...); err != nil {
		return errors.NewPatchError(op.Op, op.Path, "", "failed to update list", err)
	}
	return nil
}

func resolveListIndex(list []interface{}, selector string) (int, error) {
	if strings.Contains(selector, "=") {
		parts := strings.SplitN(selector, "=", 2)
		key, val := parts[0], parts[1]
		for i, item := range list {
			m, ok := item.(map[string]interface{})
			if ok && fmt.Sprintf("%v", m[key]) == val {
				return i, nil
			}
		}
		return -1, errors.NewPatchError("resolve", "", "", fmt.Sprintf("key-value match '%s=%s' not found in list", key, val), nil)
	}
	i, err := strconv.Atoi(selector)
	if err != nil {
		return -1, errors.NewValidationError("selector", selector, "patch", []string{"integer index", "key=value pair"})
	}
	if i < 0 {
		i = len(list) + i
	}
	if i < 0 || i > len(list) {
		return -1, errors.NewPatchError("resolve", "", "", fmt.Sprintf("index %d out of bounds for list of length %d", i, len(list)), nil)
	}
	return i, nil
}

// applyArrayReplace handles replace operations on array elements using selectors
func applyArrayReplace(obj map[string]interface{}, op PatchOp) error {
	path := parsePath(op.Path)
	lst, found, err := unstructured.NestedSlice(obj, path...)
	if err != nil {
		return errors.NewPatchError(op.Op, op.Path, "", "failed to access array", err)
	}
	if !found {
		return errors.NewPatchError(op.Op, op.Path, "", "array not found", nil)
	}

	idx, err := resolveListIndex(lst, op.Selector)
	if err != nil {
		return errors.Wrap(err, "failed to resolve array index")
	}

	if idx < 0 || idx >= len(lst) {
		return errors.NewPatchError(op.Op, op.Path, "", fmt.Sprintf("array index %d out of bounds for array of length %d", idx, len(lst)), nil)
	}

	// Check if this is a nested field patch (value is a map with remaining path)
	if valueMap, ok := op.Value.(map[string]interface{}); ok && len(valueMap) == 1 {
		// This is a nested patch - get the array item and patch it
		item, ok := lst[idx].(map[string]interface{})
		if !ok {
			return errors.NewPatchError(op.Op, op.Path, "", fmt.Sprintf("array item at index %d is not an object", idx), nil)
		}

		// Apply the nested patch to the array item
		for remainingPath, newValue := range valueMap {
			// Convert value to appropriate type for unstructured
			convertedValue := convertValueForUnstructured(newValue)
			if err := unstructured.SetNestedField(item, convertedValue, parsePath(remainingPath)...); err != nil {
				return errors.NewPatchError(op.Op, op.Path+"."+remainingPath, "", "failed to set nested field", err)
			}
		}

		// Update the array with the modified item
		lst[idx] = item
	} else {
		// Direct replacement of the array item
		convertedValue := convertValueForUnstructured(op.Value)
		lst[idx] = convertedValue
	}

	if err := unstructured.SetNestedSlice(obj, lst, path...); err != nil {
		return errors.NewPatchError(op.Op, op.Path, "", "failed to update array", err)
	}
	return nil
}

func parsePath(path string) []string {
	clean := strings.Trim(path, ".")
	if clean == "" {
		return []string{}
	}
	return strings.Split(clean, ".")
}

// ParsePatchLine converts a YAML patch line of form "path[selector]" into a PatchOp.
func ParsePatchLine(key string, value interface{}) (PatchOp, error) {
	var op PatchOp
	if strings.HasSuffix(key, "[-]") {
		op.Op = "append"
		op.Path = strings.TrimSuffix(key, "[-]")
		op.Value = value
		return op, nil
	}

	// handle delete syntax: path[delete] or path[delete=selector]
	delRe := regexp.MustCompile(`^(.*)\[delete(?:=(.*))?]$`)
	if m := delRe.FindStringSubmatch(key); len(m) == 3 {
		op.Op = "delete"
		op.Path = m[1]
		op.Selector = m[2]
		op.Value = nil
		return op, nil
	}

	// First check for selectors at the end (existing behavior)
	re := regexp.MustCompile(`(.*)\[(.*?)]$`)
	matches := re.FindStringSubmatch(key)
	if len(matches) == 3 {
		path, sel := matches[1], matches[2]
		switch {
		case strings.HasPrefix(sel, "-") && !isNumeric(strings.TrimPrefix(sel, "-")):
			// Insert before matching item: [-name=value]
			op.Op = "insertBefore"
			op.Selector = strings.TrimPrefix(sel, "-")
		case strings.HasPrefix(sel, "+") && !isNumeric(strings.TrimPrefix(sel, "+")):
			// Insert after matching item: [+name=value]
			op.Op = "insertAfter"
			op.Selector = strings.TrimPrefix(sel, "+")
		case strings.HasPrefix(sel, "-") && isNumeric(strings.TrimPrefix(sel, "-")):
			// Insert before index: [-3] means insert before index 3
			op.Op = "insertBefore"
			op.Selector = strings.TrimPrefix(sel, "-")
		case strings.HasPrefix(sel, "+") && isNumeric(strings.TrimPrefix(sel, "+")):
			// Insert after index: [+2] means insert after index 2
			op.Op = "insertAfter"
			op.Selector = strings.TrimPrefix(sel, "+")
		default:
			op.Op = "replace"
			op.Selector = sel
		}
		op.Path = path
		op.Value = value
		return op, nil
	}

	// Check for selectors in the middle of the path
	midSelectorRe := regexp.MustCompile(`^(.+)\[([^\]]+)\]\.(.+)$`)
	midMatches := midSelectorRe.FindStringSubmatch(key)
	if len(midMatches) == 4 {
		basePath, sel, remainingPath := midMatches[1], midMatches[2], midMatches[3]
		switch {
		case strings.HasPrefix(sel, "-") && !isNumeric(strings.TrimPrefix(sel, "-")):
			// Insert before matching item in mid-path
			op.Op = "insertBefore"
			op.Selector = strings.TrimPrefix(sel, "-")
		case strings.HasPrefix(sel, "+") && !isNumeric(strings.TrimPrefix(sel, "+")):
			// Insert after matching item in mid-path
			op.Op = "insertAfter"
			op.Selector = strings.TrimPrefix(sel, "+")
		case strings.HasPrefix(sel, "-") && isNumeric(strings.TrimPrefix(sel, "-")):
			// Insert before index in mid-path
			op.Op = "insertBefore"
			op.Selector = strings.TrimPrefix(sel, "-")
		case strings.HasPrefix(sel, "+") && isNumeric(strings.TrimPrefix(sel, "+")):
			// Insert after index in mid-path
			op.Op = "insertAfter"
			op.Selector = strings.TrimPrefix(sel, "+")
		default:
			op.Op = "replace"
			op.Selector = sel
		}
		op.Path = basePath
		// Store the remaining path after the selector as part of the operation
		// We'll need to modify the patch application logic to handle this
		op.Value = map[string]interface{}{remainingPath: value}
		return op, nil
	}

	op.Op = "replace"
	op.Path = key
	op.Value = value
	return op, nil
}

// ValidateAgainst checks that the patch operation is valid for the given object.
func (p *PatchOp) ValidateAgainst(obj *unstructured.Unstructured) error {
	path := parsePath(p.Path)
	switch p.Op {
	case "replace":
		_, found, err := unstructured.NestedFieldNoCopy(obj.Object, path...)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("path not found for replace: %s", p.Path)
		}
	case "delete":
		if p.Selector == "" {
			_, found, err := unstructured.NestedFieldNoCopy(obj.Object, path...)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("path not found for delete: %s", p.Path)
			}
			return nil
		}
		lst, found, err := unstructured.NestedSlice(obj.Object, path...)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("path not found for list delete: %s", p.Path)
		}
		if _, err := resolveListIndex(lst, p.Selector); err != nil {
			return err
		}
	case "insertBefore", "insertAfter", "append":
		_, found, err := unstructured.NestedSlice(obj.Object, path...)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("path not found for list op: %s", p.Path)
		}
	}
	return nil
}

// PathPart represents one segment of a parsed patch path.
type PathPart struct {
	Field      string
	MatchType  string // "", "index", or "key"
	MatchValue string
}

// NormalizePath parses the Path field and stores the result in ParsedPath.
func (p *PatchOp) NormalizePath() error {
	parsed, err := ParsePatchPath(p.Path)
	if err != nil {
		return fmt.Errorf("NormalizePath failed for %s: %w", p.Path, err)
	}
	p.ParsedPath = parsed
	return nil
}

// InferPatchOp infers a patch operation based on the path syntax.
func InferPatchOp(path string) string {
	// Check for insertion patterns
	if re := regexp.MustCompile(`\[([+-])[^0-9]`); re.MatchString(path) {
		matches := re.FindStringSubmatch(path)
		if len(matches) > 1 {
			if matches[1] == "+" {
				return "insertafter"
			} else if matches[1] == "-" {
				return "insertbefore"
			}
		}
	}
	// Check for pure index-based insertion
	if re := regexp.MustCompile(`\[([+-])\d+\]`); re.MatchString(path) {
		matches := re.FindStringSubmatch(path)
		if len(matches) > 1 {
			if matches[1] == "+" {
				return "insertafter"
			} else if matches[1] == "-" {
				return "insertbefore"
			}
		}
	}
	if strings.HasSuffix(path, "[-]") {
		return "append"
	}
	return "replace"
}

// ParsePatchPath parses a patch path with selectors into structured parts.
func ParsePatchPath(path string) ([]PathPart, error) {
	clean := strings.Trim(path, ".")
	if clean == "" {
		return nil, fmt.Errorf("empty path")
	}

	segments := strings.Split(clean, ".")
	parts := make([]PathPart, 0, len(segments))

	for _, seg := range segments {
		if seg == "" {
			return nil, fmt.Errorf("invalid empty segment in %q", path)
		}

		var part PathPart
		idx := strings.IndexRune(seg, '[')
		if idx == -1 {
			part.Field = seg
			parts = append(parts, part)
			continue
		}
		if !strings.HasSuffix(seg, "]") || idx == 0 {
			return nil, fmt.Errorf("malformed selector in segment %q", seg)
		}

		part.Field = seg[:idx]
		sel := seg[idx+1 : len(seg)-1]
		if sel == "" {
			return nil, fmt.Errorf("empty selector in segment %q", seg)
		}

		if strings.Contains(sel, "=") {
			part.MatchType = "key"
			part.MatchValue = sel
		} else {
			if _, err := strconv.Atoi(sel); err != nil {
				return nil, fmt.Errorf("invalid index %q in segment %q", sel, seg)
			}
			part.MatchType = "index"
			part.MatchValue = sel
		}
		parts = append(parts, part)
	}

	return parts, nil
}

// isNumeric checks if a string represents a valid integer
func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// convertValueForUnstructured converts values to types compatible with unstructured.SetNestedField
// The unstructured package expects specific types and doesn't handle raw int types well
func convertValueForUnstructured(value interface{}) interface{} {
	switch v := value.(type) {
	case int:
		return int64(v) // Convert int to int64 for unstructured compatibility
	case int32:
		return int64(v)
	case int64:
		return v
	case float32:
		return float64(v)
	case float64:
		return v
	case bool:
		return v
	case string:
		return v
	case map[string]interface{}:
		// Recursively convert map values
		converted := make(map[string]interface{})
		for k, val := range v {
			converted[k] = convertValueForUnstructured(val)
		}
		return converted
	case []interface{}:
		// Recursively convert slice values
		converted := make([]interface{}, len(v))
		for i, val := range v {
			converted[i] = convertValueForUnstructured(val)
		}
		return converted
	default:
		// Return as-is for other types
		return value
	}
}
