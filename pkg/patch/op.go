package patch

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	Name    string
	Base    *unstructured.Unstructured
	Patches []PatchOp
}

// Apply executes all patches on the base object.
func (r *ResourceWithPatches) Apply() error {
	for _, patch := range r.Patches {
		if err := applyPatchOp(r.Base.Object, patch); err != nil {
			return fmt.Errorf("failed to apply patch %v: %w", patch, err)
		}
	}
	return nil
}

func applyPatchOp(obj map[string]interface{}, op PatchOp) error {
	switch op.Op {
	case "replace":
		return unstructured.SetNestedField(obj, op.Value, parsePath(op.Path)...)
	case "delete":
		if op.Selector == "" {
			_, found, err := unstructured.NestedFieldNoCopy(obj, parsePath(op.Path)...)
			if err != nil || !found {
				return fmt.Errorf("path not found: %s", op.Path)
			}
			unstructured.RemoveNestedField(obj, parsePath(op.Path)...)
			return nil
		}
		path := parsePath(op.Path)
		lst, found, err := unstructured.NestedSlice(obj, path...)
		if err != nil || !found {
			return fmt.Errorf("path not found for list delete: %s", op.Path)
		}
		idx, err := resolveListIndex(lst, op.Selector)
		if err != nil {
			return err
		}
		if idx < 0 || idx >= len(lst) {
			return fmt.Errorf("index out of bounds: %d", idx)
		}
		lst = append(lst[:idx], lst[idx+1:]...)
		return unstructured.SetNestedSlice(obj, lst, path...)
	case "append":
		lst, found, err := unstructured.NestedSlice(obj, parsePath(op.Path)...)
		if err != nil || !found {
			return fmt.Errorf("path not found: %s", op.Path)
		}
		lst = append(lst, op.Value)
		return unstructured.SetNestedSlice(obj, lst, parsePath(op.Path)...)
	case "insertBefore", "insertAfter":
		return applyListPatch(obj, op)
	default:
		return fmt.Errorf("unsupported op: %s", op.Op)
	}
}

func applyListPatch(obj map[string]interface{}, op PatchOp) error {
	path := parsePath(op.Path)
	lst, found, err := unstructured.NestedSlice(obj, path...)
	if err != nil || !found {
		return fmt.Errorf("path not found for list insert: %s", op.Path)
	}

	idx, err := resolveListIndex(lst, op.Selector)
	if err != nil {
		return err
	}

	switch op.Op {
	case "insertBefore":
		lst = append(lst[:idx], append([]interface{}{op.Value}, lst[idx:]...)...)
	case "insertAfter":
		lst = append(lst[:idx+1], append([]interface{}{op.Value}, lst[idx+1:]...)...)
	}

	return unstructured.SetNestedSlice(obj, lst, path...)
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
		return -1, errors.New("key match not found")
	}
	i, err := strconv.Atoi(selector)
	if err != nil {
		return -1, fmt.Errorf("invalid index: %s", selector)
	}
	if i < 0 {
		i = len(list) + i
	}
	if i < 0 || i > len(list) {
		return -1, fmt.Errorf("index out of bounds: %d", i)
	}
	return i, nil
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

	re := regexp.MustCompile(`(.*)\[(.*?)]$`)
	matches := re.FindStringSubmatch(key)
	if len(matches) == 3 {
		path, sel := matches[1], matches[2]
		switch {
		case strings.HasPrefix(sel, "-="):
			op.Op = "insertBefore"
			op.Selector = strings.TrimPrefix(sel, "-=")
		case strings.HasPrefix(sel, "+="):
			op.Op = "insertAfter"
			op.Selector = strings.TrimPrefix(sel, "+=")
		default:
			op.Op = "replace"
			op.Selector = sel
		}
		op.Path = path
		op.Value = value
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
	if strings.Contains(path, "[+=]") || strings.Contains(path, "[+=name=") {
		return "insertafter"
	}
	if strings.Contains(path, "[-=") {
		return "insertbefore"
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
