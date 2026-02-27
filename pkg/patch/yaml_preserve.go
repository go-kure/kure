package patch

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// YAMLDocument represents a single YAML document with preserved structure
type YAMLDocument struct {
	Node     *yaml.Node
	Resource *unstructured.Unstructured
	Original string // Original YAML content with comments
	Order    int    // Original position in file
}

// YAMLDocumentSet holds multiple YAML documents with preserved order and comments
type YAMLDocumentSet struct {
	Documents []*YAMLDocument
	Separator string // Document separator (usually "---")
}

// LoadResourcesWithStructure loads YAML resources while preserving comments and order
func LoadResourcesWithStructure(r io.Reader) (*YAMLDocumentSet, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML content: %w", err)
	}

	// Split into documents while preserving separators and content
	documents, err := parseYAMLDocuments(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML documents: %w", err)
	}

	set := &YAMLDocumentSet{
		Documents: make([]*YAMLDocument, 0, len(documents)),
		Separator: "---",
	}

	for i, docContent := range documents {
		if strings.TrimSpace(docContent) == "" {
			continue // Skip empty documents
		}

		// Parse with yaml.v3 to preserve structure
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(docContent), &node); err != nil {
			return nil, fmt.Errorf("failed to parse document %d: %w", i, err)
		}

		// Also parse into unstructured for patching
		var raw map[string]interface{}
		if err := yaml.Unmarshal([]byte(docContent), &raw); err != nil {
			return nil, fmt.Errorf("failed to parse document %d into unstructured: %w", i, err)
		}

		if len(raw) == 0 {
			continue // Skip empty documents
		}

		// Apply type conversion to base YAML values to fix string ports, etc.
		// First convert the raw data for the unstructured object
		convertedRaw := convertBaseYAMLTypes(raw)
		var resource *unstructured.Unstructured
		if convertedMap, ok := convertedRaw.(map[string]interface{}); ok {
			resource = &unstructured.Unstructured{Object: convertedMap}
		} else {
			// Fallback to original if conversion failed
			resource = &unstructured.Unstructured{Object: raw}
		}

		// Also apply type conversion to the YAML node to preserve formatting
		if err := convertYAMLNodeTypes(&node); err != nil {
			if Debug {
				fmt.Printf("Warning: failed to convert YAML node types: %v\n", err)
			}
		}

		doc := &YAMLDocument{
			Node:     &node,
			Resource: resource,
			Original: docContent,
			Order:    i,
		}

		set.Documents = append(set.Documents, doc)

		if Debug {
			fmt.Printf("Loaded document %d: kind=%s name=%s\n", i, resource.GetKind(), resource.GetName())
		}
	}

	return set, nil
}

// parseYAMLDocuments splits multi-document YAML while preserving content structure
func parseYAMLDocuments(content string) ([]string, error) {
	var documents []string
	var currentDoc strings.Builder

	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := scanner.Text()

		// Check for document separator
		if strings.TrimSpace(line) == "---" {
			// Save current document if it has content
			if currentDoc.Len() > 0 {
				documents = append(documents, currentDoc.String())
				currentDoc.Reset()
			}
			continue
		}

		// Add line to current document
		if currentDoc.Len() > 0 {
			currentDoc.WriteString("\n")
		}
		currentDoc.WriteString(line)
	}

	// Add final document
	if currentDoc.Len() > 0 {
		documents = append(documents, currentDoc.String())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading YAML content: %w", err)
	}

	return documents, nil
}

// ApplyPatchesToDocument applies patches to a YAML document while preserving structure
func (doc *YAMLDocument) ApplyPatchesToDocument(patches []PatchOp) error {
	// Apply patches to the unstructured resource
	for _, patch := range patches {
		if err := applyPatchOp(doc.Resource.Object, patch); err != nil {
			return fmt.Errorf("failed to apply patch %v: %w", patch, err)
		}
	}

	// Update the YAML node with patched data
	// First, marshal the patched resource to YAML
	patchedYAML, err := yaml.Marshal(doc.Resource.Object)
	if err != nil {
		return fmt.Errorf("failed to marshal patched resource: %w", err)
	}

	// Parse the patched YAML back into a node structure
	var patchedNode yaml.Node
	if err := yaml.Unmarshal(patchedYAML, &patchedNode); err != nil {
		return fmt.Errorf("failed to parse patched YAML: %w", err)
	}

	// Try to preserve comments by merging structures
	if err := mergeYAMLNodes(doc.Node, &patchedNode); err != nil {
		// If merging fails, fall back to using the patched node
		doc.Node = &patchedNode
	}

	return nil
}

// ApplyStrategicPatchesToDocument applies strategic merge patches to a YAML
// document while preserving structure. The resource is mutated in place via
// ApplyStrategicMergePatch, then the YAML node is updated to reflect the
// new state while preserving comments.
func (doc *YAMLDocument) ApplyStrategicPatchesToDocument(patches []StrategicPatch, lookup KindLookup) error {
	for _, smp := range patches {
		if err := ApplyStrategicMergePatch(doc.Resource, smp.Patch, lookup); err != nil {
			return fmt.Errorf("failed to apply strategic patch: %w", err)
		}
	}

	// Re-marshal the patched resource and merge into the existing YAML node
	patchedYAML, err := yaml.Marshal(doc.Resource.Object)
	if err != nil {
		return fmt.Errorf("failed to marshal patched resource: %w", err)
	}

	var patchedNode yaml.Node
	if err := yaml.Unmarshal(patchedYAML, &patchedNode); err != nil {
		return fmt.Errorf("failed to parse patched YAML: %w", err)
	}

	if err := mergeYAMLNodes(doc.Node, &patchedNode); err != nil {
		doc.Node = &patchedNode
	}

	return nil
}

// mergeYAMLNodes attempts to merge patched values into original node structure
// This is a best-effort attempt to preserve comments and formatting
func mergeYAMLNodes(original, patched *yaml.Node) error {
	if original.Kind != patched.Kind {
		return fmt.Errorf("node kinds don't match: %v vs %v", original.Kind, patched.Kind)
	}

	switch original.Kind {
	case yaml.DocumentNode:
		if len(original.Content) > 0 && len(patched.Content) > 0 {
			return mergeYAMLNodes(original.Content[0], patched.Content[0])
		}
	case yaml.MappingNode:
		return mergeMappingNodes(original, patched)
	case yaml.SequenceNode:
		return mergeSequenceNodes(original, patched)
	case yaml.ScalarNode:
		// Update scalar value while preserving style and comments
		original.Value = patched.Value
		// Keep original style and comments
	case yaml.AliasNode:
		// Alias nodes reference other nodes; no merge needed
	}

	return nil
}

// mergeMappingNodes merges mapping (object) nodes
func mergeMappingNodes(original, patched *yaml.Node) error {
	if len(patched.Content)%2 != 0 {
		return fmt.Errorf("invalid mapping node: odd number of content items")
	}

	// Create a map of keys in the patched node for efficient lookup
	patchedMap := make(map[string]*yaml.Node)
	for i := 0; i < len(patched.Content); i += 2 {
		key := patched.Content[i].Value
		value := patched.Content[i+1]
		patchedMap[key] = value
	}

	// Update existing keys in original
	for i := 0; i < len(original.Content); i += 2 {
		key := original.Content[i].Value
		if patchedValue, exists := patchedMap[key]; exists {
			// Recursively merge if both are objects/arrays, otherwise replace
			if original.Content[i+1].Kind == patchedValue.Kind &&
				(patchedValue.Kind == yaml.MappingNode || patchedValue.Kind == yaml.SequenceNode) {
				if err := mergeYAMLNodes(original.Content[i+1], patchedValue); err != nil {
					return err
				}
			} else {
				// Replace the value node but keep the key node (preserves comments on key)
				original.Content[i+1] = patchedValue
			}
			delete(patchedMap, key) // Mark as processed
		}
	}

	// Add new keys from patched
	for key, value := range patchedMap {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: key,
		}
		original.Content = append(original.Content, keyNode, value)
	}

	return nil
}

// mergeSequenceNodes replaces the original sequence with the patched sequence
// while preserving comments from matched items. This is NOT a true merge:
// items present in the original but absent in the patched set are dropped.
// Callers are expected to provide a patched set that already contains all
// surviving items (e.g. the output of a strategic merge patch).
// When items can be matched by a well-known merge key (e.g. "name"), comments
// and formatting are preserved on existing items. Items without a match key
// fall back to full replacement.
func mergeSequenceNodes(original, patched *yaml.Node) error {
	mergeKey := detectMergeKey(original)
	if mergeKey == "" {
		// No recognizable merge key — replace the entire sequence
		original.Content = patched.Content
		return nil
	}

	// Build index of original items by merge key value
	origIndex := buildMergeKeyIndex(original, mergeKey)

	// Process patched items: merge into existing or append
	var merged []*yaml.Node
	seen := make(map[string]bool)

	for _, patchedItem := range patched.Content {
		keyVal := extractMergeKeyValue(patchedItem, mergeKey)
		if keyVal != "" {
			seen[keyVal] = true
			if origItem, exists := origIndex[keyVal]; exists {
				// Use patched item as the base (correct field set),
				// but preserve comments from the original item.
				preserveComments(origItem, patchedItem)
				merged = append(merged, patchedItem)
				continue
			}
		}
		merged = append(merged, patchedItem)
	}

	original.Content = merged
	return nil
}

// commonMergeKeys are the well-known keys used by Kubernetes strategic merge
// patch to identify list items.
var commonMergeKeys = []string{
	"name",
	"containerPort",
	"port",
	"mountPath",
	"key",
	"ip",
}

// detectMergeKey examines the first mapping item in a sequence to find a
// recognized merge key. Returns "" if none found.
func detectMergeKey(seq *yaml.Node) string {
	if len(seq.Content) == 0 {
		return ""
	}
	first := seq.Content[0]
	if first.Kind != yaml.MappingNode {
		return ""
	}
	keys := make(map[string]bool)
	for i := 0; i < len(first.Content); i += 2 {
		if first.Content[i].Kind == yaml.ScalarNode {
			keys[first.Content[i].Value] = true
		}
	}
	for _, mk := range commonMergeKeys {
		if keys[mk] {
			return mk
		}
	}
	return ""
}

// buildMergeKeyIndex builds a map from merge-key value to the original node.
func buildMergeKeyIndex(seq *yaml.Node, mergeKey string) map[string]*yaml.Node {
	index := make(map[string]*yaml.Node)
	for _, item := range seq.Content {
		if val := extractMergeKeyValue(item, mergeKey); val != "" {
			index[val] = item
		}
	}
	return index
}

// extractMergeKeyValue extracts the value of the merge key from a mapping node.
func extractMergeKeyValue(node *yaml.Node, mergeKey string) string {
	if node.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Kind == yaml.ScalarNode && node.Content[i].Value == mergeKey {
			if node.Content[i+1].Kind == yaml.ScalarNode {
				return node.Content[i+1].Value
			}
		}
	}
	return ""
}

// preserveComments copies comments from src (original) to dst (patched) by
// walking both mapping nodes in parallel and matching keys. Only comments on
// keys that exist in dst are transferred.
func preserveComments(src, dst *yaml.Node) {
	if src == nil || dst == nil {
		return
	}

	// Copy top-level comments
	if dst.HeadComment == "" {
		dst.HeadComment = src.HeadComment
	}
	if dst.LineComment == "" {
		dst.LineComment = src.LineComment
	}
	if dst.FootComment == "" {
		dst.FootComment = src.FootComment
	}

	if src.Kind != yaml.MappingNode || dst.Kind != yaml.MappingNode {
		return
	}

	// Build index of src keys -> (keyNode, valueNode)
	type kvPair struct {
		key   *yaml.Node
		value *yaml.Node
	}
	srcMap := make(map[string]kvPair)
	for i := 0; i < len(src.Content)-1; i += 2 {
		if src.Content[i].Kind == yaml.ScalarNode {
			srcMap[src.Content[i].Value] = kvPair{src.Content[i], src.Content[i+1]}
		}
	}

	for i := 0; i < len(dst.Content)-1; i += 2 {
		dstKey := dst.Content[i]
		dstVal := dst.Content[i+1]
		if dstKey.Kind != yaml.ScalarNode {
			continue
		}
		if srcKV, ok := srcMap[dstKey.Value]; ok {
			// Copy comments from source key and value nodes
			if dstKey.HeadComment == "" {
				dstKey.HeadComment = srcKV.key.HeadComment
			}
			if dstKey.LineComment == "" {
				dstKey.LineComment = srcKV.key.LineComment
			}
			if dstKey.FootComment == "" {
				dstKey.FootComment = srcKV.key.FootComment
			}
			if dstVal.HeadComment == "" {
				dstVal.HeadComment = srcKV.value.HeadComment
			}
			if dstVal.LineComment == "" {
				dstVal.LineComment = srcKV.value.LineComment
			}
			if dstVal.FootComment == "" {
				dstVal.FootComment = srcKV.value.FootComment
			}
			// Recurse into nested structures
			preserveComments(srcKV.value, dstVal)
		}
	}
}

// WriteToFile writes the document set to a file with preserved structure
func (set *YAMLDocumentSet) WriteToFile(filename string) error {
	var buf bytes.Buffer

	for i, doc := range set.Documents {
		if i > 0 {
			buf.WriteString(set.Separator + "\n")
		}

		// Marshal the updated node back to YAML
		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)

		if err := encoder.Encode(doc.Node); err != nil {
			return fmt.Errorf("failed to encode document %d: %w", i, err)
		}

		_ = encoder.Close()
	}

	// Write to file
	content := buf.String()
	// Clean up extra newlines at the end
	content = strings.TrimSuffix(content, "\n") + "\n"

	if err := writeFile(filename, []byte(content)); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filename, err)
	}

	return nil
}

// writeFile is a helper to write content to a file
func writeFile(filename string, content []byte) error {
	return os.WriteFile(filename, content, 0644)
}

// GetResources returns the unstructured resources in order
func (set *YAMLDocumentSet) GetResources() []*unstructured.Unstructured {
	resources := make([]*unstructured.Unstructured, len(set.Documents))
	for i, doc := range set.Documents {
		resources[i] = doc.Resource
	}
	return resources
}

// FindDocumentByName finds a document by resource name
func (set *YAMLDocumentSet) FindDocumentByName(name string) *YAMLDocument {
	for _, doc := range set.Documents {
		if doc.Resource.GetName() == name {
			return doc
		}
	}
	return nil
}

// FindDocumentByKindAndName finds a document by resource kind and name
func (set *YAMLDocumentSet) FindDocumentByKindAndName(kind, name string) *YAMLDocument {
	for _, doc := range set.Documents {
		if strings.EqualFold(doc.Resource.GetKind(), kind) &&
			doc.Resource.GetName() == name {
			return doc
		}
	}
	return nil
}

// FindDocumentByKindNameAndNamespace finds a document by kind, name, and namespace.
// All three must match for a result.
func (set *YAMLDocumentSet) FindDocumentByKindNameAndNamespace(kind, name, namespace string) *YAMLDocument {
	for _, doc := range set.Documents {
		if strings.EqualFold(doc.Resource.GetKind(), kind) &&
			doc.Resource.GetName() == name &&
			doc.Resource.GetNamespace() == namespace {
			return doc
		}
	}
	return nil
}

// UpdateDocumentFromResource updates a document's YAML node from its resource
func (doc *YAMLDocument) UpdateDocumentFromResource() error {
	// Marshal the updated resource to YAML
	updatedYAML, err := yaml.Marshal(doc.Resource.Object)
	if err != nil {
		return fmt.Errorf("failed to marshal updated resource: %w", err)
	}

	// Parse the updated YAML back into a node structure
	var updatedNode yaml.Node
	if err := yaml.Unmarshal(updatedYAML, &updatedNode); err != nil {
		return fmt.Errorf("failed to parse updated YAML: %w", err)
	}

	// For now, use the updated node directly to ensure correctness
	// Comment preservation will be improved in a future iteration
	doc.Node = &updatedNode

	return nil
}

// GenerateOutputFilename creates the output filename based on the pattern
// <outputDir>/<originalname>-patch-<patchname>.yaml
func GenerateOutputFilename(originalPath, patchPath, outputDir string) string {
	// Extract base names without extensions
	originalBase := extractBaseName(originalPath)
	patchBase := extractBaseName(patchPath)

	if outputDir == "" {
		outputDir = "." // Default to current directory
	}

	return fmt.Sprintf("%s/%s-patch-%s.yaml", outputDir, originalBase, patchBase)
}

// Copy creates a deep copy of the YAMLDocumentSet
func (set *YAMLDocumentSet) Copy() (*YAMLDocumentSet, error) {
	copiedSet := &YAMLDocumentSet{
		Documents: make([]*YAMLDocument, len(set.Documents)),
		Separator: set.Separator,
	}

	for i, doc := range set.Documents {
		// Deep copy the YAML node
		copiedNode, err := copyYAMLNode(doc.Node)
		if err != nil {
			return nil, fmt.Errorf("failed to copy YAML node for document %d: %w", i, err)
		}

		// Deep copy the unstructured resource
		copiedResource := doc.Resource.DeepCopy()

		copiedSet.Documents[i] = &YAMLDocument{
			Node:     copiedNode,
			Resource: copiedResource,
			Original: doc.Original, // String is immutable, safe to share
			Order:    doc.Order,
		}
	}

	return copiedSet, nil
}

// copyYAMLNode creates a deep copy of a yaml.Node
func copyYAMLNode(node *yaml.Node) (*yaml.Node, error) {
	if node == nil {
		return nil, nil
	}

	copied := &yaml.Node{
		Kind:        node.Kind,
		Style:       node.Style,
		Tag:         node.Tag,
		Value:       node.Value,
		Anchor:      node.Anchor,
		Alias:       node.Alias,
		Content:     make([]*yaml.Node, len(node.Content)),
		HeadComment: node.HeadComment,
		LineComment: node.LineComment,
		FootComment: node.FootComment,
		Line:        node.Line,
		Column:      node.Column,
	}

	// Recursively copy content nodes
	for i, child := range node.Content {
		copiedChild, err := copyYAMLNode(child)
		if err != nil {
			return nil, fmt.Errorf("failed to copy child node %d: %w", i, err)
		}
		copied.Content[i] = copiedChild
	}

	return copied, nil
}

// extractBaseName extracts the base name without extension from a file path
func extractBaseName(filePath string) string {
	// Find the last slash to get filename
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash == -1 {
		lastSlash = strings.LastIndex(filePath, "\\") // Windows path
	}

	filename := filePath
	if lastSlash >= 0 {
		filename = filePath[lastSlash+1:]
	}

	// Remove extension
	if dotIndex := strings.LastIndex(filename, "."); dotIndex > 0 {
		filename = filename[:dotIndex]
	}

	// Clean up filename for use in output (remove invalid characters)
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	return re.ReplaceAllString(filename, "-")
}

// convertBaseYAMLTypes recursively converts string values in base YAML to appropriate types
// This fixes issues where Kubernetes YAML files have numeric fields as strings
func convertBaseYAMLTypes(obj interface{}) interface{} {
	switch v := obj.(type) {
	case map[string]interface{}:
		converted := make(map[string]interface{})
		for key, value := range v {
			// Apply type inference based on field name and value
			if strValue, ok := value.(string); ok {
				if shouldConvertToInteger(key, strValue) {
					if intVal, err := strconv.Atoi(strValue); err == nil {
						converted[key] = int64(intVal) // Use int64 for unstructured compatibility
						continue
					}
				}
				if shouldConvertToBoolean(strValue) {
					if boolVal, err := strconv.ParseBool(strValue); err == nil {
						converted[key] = boolVal
						continue
					}
				}
			}
			// Recursively convert nested objects
			converted[key] = convertBaseYAMLTypes(value)
		}
		return converted
	case []interface{}:
		converted := make([]interface{}, len(v))
		for i, item := range v {
			converted[i] = convertBaseYAMLTypes(item)
		}
		return converted
	default:
		return obj
	}
}

// shouldConvertToInteger determines if a string field should be converted to integer
func shouldConvertToInteger(key, value string) bool {
	// Skip if not a valid integer
	if _, err := strconv.Atoi(value); err != nil {
		return false
	}

	// Convert common Kubernetes integer fields
	integerFields := []string{
		"port", "targetport", "nodeport", "containerport",
		"replicas", "maxunavailable", "maxsurge",
		"initialdelayseconds", "timeoutseconds", "periodseconds",
		"successthreshold", "failurethreshold",
		"terminationgraceperiodseconds", "activedeadlineseconds",
		"runasuser", "runasgroup", "fsgroup",
		"weight", "priority", "number",
	}

	keyLower := strings.ToLower(key)
	for _, field := range integerFields {
		if strings.Contains(keyLower, field) {
			return true
		}
	}

	return false
}

// shouldConvertToBoolean determines if a string should be converted to boolean
func shouldConvertToBoolean(value string) bool {
	lowerValue := strings.ToLower(value)
	return lowerValue == "true" || lowerValue == "false"
}

// convertYAMLNodeTypes recursively converts string values in YAML nodes to appropriate types
// while preserving the original YAML structure and formatting
func convertYAMLNodeTypes(node *yaml.Node) error {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		// Process document content
		for _, child := range node.Content {
			if err := convertYAMLNodeTypes(child); err != nil {
				return err
			}
		}

	case yaml.MappingNode:
		// Process mapping nodes (key-value pairs)
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			// Convert the value node based on the key name
			if keyNode.Kind == yaml.ScalarNode && valueNode.Kind == yaml.ScalarNode {
				convertScalarNodeType(keyNode.Value, valueNode)
			}

			// Recursively process nested nodes
			if err := convertYAMLNodeTypes(valueNode); err != nil {
				return err
			}
		}

	case yaml.SequenceNode:
		// Process sequence nodes (arrays)
		for _, child := range node.Content {
			if err := convertYAMLNodeTypes(child); err != nil {
				return err
			}
		}

	case yaml.ScalarNode:
		// Scalar nodes are handled by their parent mapping node
		// No additional processing needed here

	case yaml.AliasNode:
		// Alias nodes reference other nodes; no conversion needed
	}

	return nil
}

// convertScalarNodeType converts a scalar YAML node value based on the field name
func convertScalarNodeType(fieldName string, valueNode *yaml.Node) {
	if valueNode.Kind != yaml.ScalarNode {
		return
	}

	originalValue := valueNode.Value

	// Try integer conversion
	if shouldConvertToInteger(fieldName, originalValue) {
		if _, err := strconv.Atoi(originalValue); err == nil {
			// Keep the same value but remove quotes by changing the style
			valueNode.Style = 0 // Plain style (no quotes)
			// The value stays the same, but YAML will interpret it as an integer
			return
		}
	}

	// Try boolean conversion
	if shouldConvertToBoolean(originalValue) {
		lowerValue := strings.ToLower(originalValue)
		if lowerValue == "true" || lowerValue == "false" {
			valueNode.Value = lowerValue
			valueNode.Style = 0 // Plain style
			return
		}
	}
}
