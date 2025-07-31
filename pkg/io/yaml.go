package io

import (
	"bytes"
	"io"
	"os"

	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/k8s"
)

// Buffer is a simple in-memory buffer that implements io.Reader and io.Writer
// and can marshal and unmarshal YAML representations of objects.
type Buffer struct {
	bytes.Buffer
}

// Marshal writes the YAML representation of obj to the buffer.
func (b *Buffer) Marshal(obj interface{}) error {
	b.Reset()
	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = b.Write(data)
	return err
}

// Unmarshal parses the buffer contents as YAML into obj.
func (b *Buffer) Unmarshal(obj interface{}) error {
	return yaml.Unmarshal(b.Bytes(), obj)
}

// Marshal writes the YAML representation of obj to w.
func Marshal(w io.Writer, obj interface{}) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// Unmarshal reads YAML from r into obj.
func Unmarshal(r io.Reader, obj interface{}) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, obj)
}

// SaveFile marshals obj as YAML and writes it to the given file path.
func SaveFile(path string, obj interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
		}
	}(f)
	return Marshal(f, obj)
}

// LoadFile reads YAML from the given path and unmarshals it into obj.
func LoadFile(path string, obj interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)
	return Unmarshal(f, obj)
}

func EncodeObjectsTo(objects []*client.Object, yaml bool) ([]byte, error) {
	serializer := kjson.NewSerializerWithOptions(
		kjson.DefaultMetaFactory, k8s.Scheme, k8s.Scheme,
		kjson.SerializerOptions{Yaml: yaml, Pretty: false, Strict: false},
	)

	var buf bytes.Buffer
	for _, obj := range objects {
		if err := serializer.Encode(*obj, &buf); err != nil {
			return nil, err
		}
		// Add YAML document separator
		buf.WriteString("---\n")
	}
	return buf.Bytes(), nil
}

func EncodeObjectsToYAML(objects []*client.Object) ([]byte, error) {
	return EncodeObjectsTo(objects, true)
}
func EncodeObjectsToJSON(objects []*client.Object) ([]byte, error) {
	return EncodeObjectsTo(objects, false)
}
