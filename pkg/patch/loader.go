package patch

import (
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
			op, err := ParsePatchLine(k, v)
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
				op, err := ParsePatchLine(k, v)
				if err != nil {
					return nil, fmt.Errorf("invalid patch line '%s': %w", k, err)
				}
				if err := op.NormalizePath(); err != nil {
					return nil, fmt.Errorf("invalid patch path syntax: %s: %w", op.Path, err)
				}
				if Debug {
					log.Printf("Targeted patch loaded: target=%s op=%s path=%s value=%v", entry.Target, op.Op, op.Path, op.Value)
				}
				patches = append(patches, PatchSpec{Target: entry.Target, Patch: op})
			}
		}
	} else {
		return nil, fmt.Errorf("unrecognized patch format")
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
		if r.GetName() == name {
			return true
		}
	}
	return false
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
			target = spec.Target
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
