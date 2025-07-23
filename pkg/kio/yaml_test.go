package kio

import (
	"path/filepath"
	"reflect"
	"testing"
)

type demo struct {
	Name string `yaml:"name"`
	Age  int    `yaml:"age"`
}

func TestBufferMarshalUnmarshal(t *testing.T) {
	b := &Buffer{}
	in := demo{Name: "test", Age: 5}
	if err := b.Marshal(in); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out demo
	if err := b.Unmarshal(&out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip mismatch: %#v != %#v", in, out)
	}
}

func TestSaveLoadFile(t *testing.T) {
	d := demo{Name: "file", Age: 8}
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.yaml")
	if err := SaveFile(path, d); err != nil {
		t.Fatalf("save: %v", err)
	}
	var out demo
	if err := LoadFile(path, &out); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(d, out) {
		t.Fatalf("file round trip mismatch: %#v != %#v", d, out)
	}
}
