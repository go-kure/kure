package io

import (
	"bytes"
	"io"
	"os"

	"sigs.k8s.io/yaml"
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
	defer f.Close()
	return Marshal(f, obj)
}

// LoadFile reads YAML from the given path and unmarshals it into obj.
func LoadFile(path string, obj interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return Unmarshal(f, obj)
}
