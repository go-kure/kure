package appsets

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

var Debug = os.Getenv("KURE_DEBUG") == "1"

func LoadPatchFile(r io.Reader) ([]PatchOp, error) {
    dec := yaml.NewDecoder(r)

    var firstToken yaml.Node
    if err := dec.Decode(&firstToken); err != nil {
        return nil, fmt.Errorf("failed to read patch input: %w", err)
    }

    var patches []PatchOp

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
            patches = append(patches, op)
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
                patches = append(patches, op)
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
        u := &unstructured.Unstructured{}
        err := dec.Decode(u)
        if err != nil {
            if err == io.EOF {
                break
            }
            return nil, fmt.Errorf("failed to decode resource document: %w", err)
        }
        if len(u.Object) > 0 {
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

    var wrapped []struct {
        Target string
        Patch  PatchOp
    }

    for _, p := range patches {
        if err := p.NormalizePath(); err != nil {
            return nil, fmt.Errorf("invalid patch path syntax: %s: %w", p.Path, err)
        }

        target := resolvePatchTarget(resources, p.Path)
        if target == "" {
            return nil, fmt.Errorf("could not determine target resource for patch path: %s", p.Path)
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

func resolvePatchTarget(resources []*unstructured.Unstructured, path string) string {
    pathParts := parsePath(path)
    if len(pathParts) == 0 {
        return ""
    }
    for _, r := range resources {
        name := r.GetName()
        if strings.EqualFold(name, pathParts[0]) {
            return name
        }
        kind := strings.ToLower(r.GetKind())
        composite := fmt.Sprintf("%s.%s", kind, name)
        if strings.EqualFold(composite, pathParts[0]) {
            return name
        }
    }
    return ""
}
