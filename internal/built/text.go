package built

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/go-kure/kure/pkg/errors"
	kio "github.com/go-kure/kure/pkg/io"
)

// Text returns obj's labels and annotations as written (Labels, Annotations)
// as a kustomize build reads them from what the writers write. kustomize takes
// such a value as the text of its YAML node (kyaml v0.21.2 RNode.GetLabels
// and GetAnnotations through getMapFromMeta, yaml/rnode.go:519-570), and
// selects a patch's targets by those texts (RNode.MatchesLabelSelector and
// MatchesAnnotationSelector, yaml/rnode.go:1191-1206). So a boolean reads as
// "true" or "false", a number as the writers write it ("3", "1.5", and a
// whole float64 as an integer, "2"), null as "null", and a map or a list,
// which is no scalar, as "". A string reads as itself, unless it is no valid
// UTF-8: the writers' JSON replaces each invalid byte with U+FFFD. And what
// the writers leave out is not read at all: they drop the
// kubectl.kubernetes.io/last-applied-configuration annotation.
//
// The writers marshal an object to JSON and clean its metadata before they
// write it as YAML (kio.EncodeObjectsToYAML), so Text does not restate their
// rules: it writes both maps through them, as the metadata of an object of
// their own, and reads them back as kustomize does.
func Text(obj client.Object) (objLabels, objAnnotations map[string]string, err error) {
	metadata := map[string]any{"name": "text"}
	if m := Labels(obj); m != nil {
		metadata["labels"] = m
	}
	if m := Annotations(obj); m != nil {
		metadata["annotations"] = m
	}
	var written client.Object = &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   metadata,
	}}
	data, err := kio.EncodeObjectsToYAML([]*client.Object{&written})
	if err != nil {
		return nil, nil, errors.Wrap(err, "write the labels and annotations")
	}
	node, err := kyaml.Parse(string(data))
	if err != nil {
		return nil, nil, errors.Wrap(err, "read the written labels and annotations")
	}
	return node.GetLabels(), node.GetAnnotations(), nil
}
