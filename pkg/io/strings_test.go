package io

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// controlCharacters are the characters a YAML 1.1 reader folds (NEL) or
// refuses (DEL, the other C1 controls, U+FFFE, U+FFFF) when they reach it
// unescaped.
var controlCharacters = []struct {
	name string
	c    string
}{
	{"NEL", "\u0085"},
	{"C1 U+0086", "\u0086"},
	{"C1 U+009F", "\u009f"},
	{"DEL", "\u007f"},
	{"U+FFFE", "\ufffe"},
	{"U+FFFF", "\uffff"},
}

// controlCharacterWriters are every YAML writer of the package, each given
// the same object.
var controlCharacterWriters = []struct {
	name  string
	write func(u *unstructured.Unstructured) ([]byte, error)
}{
	{"EncodeObjectsToYAML", func(u *unstructured.Unstructured) ([]byte, error) {
		var obj client.Object = u
		return EncodeObjectsToYAML([]*client.Object{&obj})
	}},
	{"EncodeObjectsToYAMLWithOptions/KubernetesFieldOrder", func(u *unstructured.Unstructured) ([]byte, error) {
		var obj client.Object = u
		return EncodeObjectsToYAMLWithOptions([]*client.Object{&obj}, EncodeOptions{KubernetesFieldOrder: true})
	}},
	{"EncodeObjectsTo/yaml", func(u *unstructured.Unstructured) ([]byte, error) {
		var obj client.Object = u
		out, err := EncodeObjectsTo([]*client.Object{&obj}, true)
		return bytes.TrimSuffix(out, []byte("---\n")), err
	}},
	{"Marshal", func(u *unstructured.Unstructured) ([]byte, error) {
		var buf bytes.Buffer
		err := Marshal(&buf, u.Object)
		return buf.Bytes(), err
	}},
	{"Buffer.Marshal", func(u *unstructured.Unstructured) ([]byte, error) {
		var buf Buffer
		err := buf.Marshal(u.Object)
		return buf.Bytes(), err
	}},
}

// controlCharacterReaders read a written document back as kure and kubectl
// do (sigs.k8s.io/yaml) and as kustomize and Flux do (kyaml).
var controlCharacterReaders = []struct {
	name string
	read func(out []byte) (map[string]any, error)
}{
	{"sigs.k8s.io/yaml", func(out []byte) (map[string]any, error) {
		var m map[string]any
		err := sigsyaml.Unmarshal(out, &m)
		return m, err
	}},
	{"kyaml", func(out []byte) (map[string]any, error) {
		n, err := kyaml.Parse(string(out))
		if err != nil {
			return nil, err
		}
		return n.Map()
	}},
}

// TestYAMLWritersKeepControlCharacters pins that every YAML writer keeps a
// string holding a character a YAML 1.1 reader would fold or refuse: in a
// data value, an annotation value and a map key, the write succeeds, the
// character is written as an escape, and both readers give back the same
// strings.
func TestYAMLWritersKeepControlCharacters(t *testing.T) {
	for _, cc := range controlCharacters {
		value := "a" + cc.c + "b"
		key := "k" + cc.c
		for _, w := range controlCharacterWriters {
			t.Run(cc.name+"/"+w.name, func(t *testing.T) {
				u := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
						"name":        "control",
						"annotations": map[string]any{"note": value},
					},
					"data": map[string]any{"value": value, key: "x"},
				}}

				out, err := w.write(u)
				if err != nil {
					t.Fatalf("write: %v", err)
				}
				if bytes.Contains(out, []byte(cc.c)) {
					t.Errorf("output holds the character unescaped:\n%q", out)
				}

				for _, r := range controlCharacterReaders {
					got, err := r.read(out)
					if err != nil {
						t.Errorf("%s: read back: %v\n%q", r.name, err, out)
						continue
					}
					if v, _, _ := unstructured.NestedString(got, "data", "value"); v != value {
						t.Errorf("%s: data value read back as %q, want %q", r.name, v, value)
					}
					if v, _, _ := unstructured.NestedString(got, "metadata", "annotations", "note"); v != value {
						t.Errorf("%s: annotation read back as %q, want %q", r.name, v, value)
					}
					if v, _, _ := unstructured.NestedString(got, "data", key); v != "x" {
						t.Errorf("%s: key %q not read back, data is %v", r.name, key, got["data"])
					}
				}
			})
		}
	}
}

// rawJSONObject is an object whose JSON is the given text, as a type with its
// own MarshalJSON writes it.
type rawJSONObject struct {
	*unstructured.Unstructured
	raw string
}

func (o rawJSONObject) MarshalJSON() ([]byte, error) { return []byte(o.raw), nil }

// TestEncodeObjectsToYAMLWritesJSONAsSigsYAML pins every writer to its
// sigs.k8s.io/yaml output on JSON that json.Marshal of a map would not write:
// number forms an object's own MarshalJSON can give, numbers no float64 holds,
// and the null of a typed nil object.
func TestEncodeObjectsToYAMLWritesJSONAsSigsYAML(t *testing.T) {
	numbers := []string{
		"1e6", "1E+2", "1.0", "1000000.0", "-0", "-0.0", "2.5e-3", "1e21", "123456789012345678901234567890",
		"1e309", "-1e309", "1" + strings.Repeat("0", 400),
	}
	for _, n := range numbers {
		raw := `{"apiVersion":"example.io/v1","kind":"Widget","metadata":{"name":"w"},"spec":{"n":` + n + `}}`
		t.Run(n[:min(len(n), 32)], func(t *testing.T) {
			var obj client.Object = rawJSONObject{Unstructured: &unstructured.Unstructured{}, raw: raw}

			// The default path reads the cleaned map, which refuses a number no
			// float64 holds, as it did before.
			got, err := EncodeObjectsToYAML([]*client.Object{&obj})
			if m, derr := decodeResourceMap([]byte(raw)); derr != nil {
				if err == nil {
					t.Errorf("EncodeObjectsToYAML: wrote %q, want the error %v", got, derr)
				}
			} else {
				want, err2 := sigsyaml.Marshal(m)
				if err2 != nil {
					t.Fatal(err2)
				}
				if err != nil || !bytes.Equal(got, want) {
					t.Errorf("EncodeObjectsToYAML: got %q (err %v), want %q", got, err, want)
				}
			}

			// Marshal, Buffer.Marshal and EncodeObjectsTo read the object's JSON
			// text itself.
			want, err := sigsyaml.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := Marshal(&buf, obj); err != nil || !bytes.Equal(buf.Bytes(), want) {
				t.Errorf("Marshal: got %q (err %v), want %q", buf.Bytes(), err, want)
			}
			var b Buffer
			if err := b.Marshal(obj); err != nil || !bytes.Equal(b.Bytes(), want) {
				t.Errorf("Buffer.Marshal: got %q (err %v), want %q", b.Bytes(), err, want)
			}
			serializer := kjson.NewSerializerWithOptions(kjson.DefaultMetaFactory, kubernetes.Scheme, kubernetes.Scheme,
				kjson.SerializerOptions{Yaml: true})
			var wantBuf bytes.Buffer
			if err := serializer.Encode(obj, &wantBuf); err != nil {
				t.Fatal(err)
			}
			wantBuf.WriteString("---\n")
			got, err = EncodeObjectsTo([]*client.Object{&obj}, true)
			if err != nil || !bytes.Equal(got, wantBuf.Bytes()) {
				t.Errorf("EncodeObjectsTo: got %q (err %v), want %q", got, err, wantBuf.Bytes())
			}
		})
	}

	t.Run("typed nil", func(t *testing.T) {
		var obj client.Object = (*unstructured.Unstructured)(nil)
		got, err := EncodeObjectsToYAML([]*client.Object{&obj})
		if err != nil {
			t.Fatalf("EncodeObjectsToYAML: %v", err)
		}
		if string(got) != "null\n" {
			t.Errorf("EncodeObjectsToYAML: got %q, want %q", got, "null\n")
		}
		var buf bytes.Buffer
		if err := Marshal(&buf, obj); err != nil || buf.String() != "null\n" {
			t.Errorf("Marshal: got %q (err %v), want %q", buf.Bytes(), err, "null\n")
		}
	})
}

// TestYAMLWritersReadJSONEscapes pins that the writers reading an object's
// JSON text keep a string spelled with a JSON escape YAML 1.1 lacks (\/, a
// surrogate pair) or holding a literal line separator, which
// sigs.k8s.io/yaml.JSONToYAML refused or changed.
func TestYAMLWritersReadJSONEscapes(t *testing.T) {
	raw := `{"apiVersion":"example.io/v1","kind":"Widget","metadata":{"name":"w"},"spec":{` +
		`"slash":"a\/b","emoji":"\ud83d\ude00","ls":"a ` + "\u2028" + ` b","ps":"a ` + "\u2029" + ` b"}}`
	want := map[string]string{"slash": "a/b", "emoji": "\U0001F600", "ls": "a \u2028 b", "ps": "a \u2029 b"}

	var obj client.Object = rawJSONObject{Unstructured: &unstructured.Unstructured{}, raw: raw}
	writers := []struct {
		name  string
		write func() ([]byte, error)
	}{
		{"EncodeObjectsToYAML", func() ([]byte, error) { return EncodeObjectsToYAML([]*client.Object{&obj}) }},
		{"EncodeObjectsTo/yaml", func() ([]byte, error) {
			out, err := EncodeObjectsTo([]*client.Object{&obj}, true)
			return bytes.TrimSuffix(out, []byte("---\n")), err
		}},
		{"Marshal", func() ([]byte, error) {
			var buf bytes.Buffer
			err := Marshal(&buf, obj)
			return buf.Bytes(), err
		}},
		{"Buffer.Marshal", func() ([]byte, error) {
			var buf Buffer
			err := buf.Marshal(obj)
			return buf.Bytes(), err
		}},
		{"jsonToYAML", func() ([]byte, error) { return jsonToYAML([]byte(raw)) }},
	}
	for _, w := range writers {
		t.Run(w.name, func(t *testing.T) {
			out, err := w.write()
			if err != nil {
				t.Fatalf("write: %v", err)
			}
			for _, r := range controlCharacterReaders {
				got, err := r.read(out)
				if err != nil {
					t.Errorf("%s: read back: %v\n%q", r.name, err, out)
					continue
				}
				for field, v := range want {
					if s, _, _ := unstructured.NestedString(got, "spec", field); s != v {
						t.Errorf("%s: spec.%s read back as %q, want %q\n%q", r.name, field, s, v, out)
					}
				}
			}
		})
	}
}

var yamlDocumentSeparator = regexp.MustCompile(`(?m)^---[ \t]*$`)

// TestYAMLWritersMatchSigsYAML pins that the writers emit, byte for byte, what
// sigs.k8s.io/yaml.Marshal (and, for EncodeObjectsTo, apimachinery's YAML
// serializer, which calls it) emits for every value it writes unchanged. The
// inputs are every YAML mapping document in the repository and a set of number
// edge cases. Each comparison runs on the same JSON the writer reads.
func TestYAMLWritersMatchSigsYAML(t *testing.T) {
	root := filepath.Join("..", "..")
	compared := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(p); ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, doc := range yamlDocumentSeparator.Split(string(data), -1) {
			j, err := sigsyaml.YAMLToJSON([]byte(doc))
			if err != nil || !bytes.HasPrefix(bytes.TrimSpace(j), []byte("{")) {
				continue // a template, an empty document or not a mapping
			}
			compareWithSigsYAML(t, p+"#"+strconv.Itoa(i), j)
			compared++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	if compared < 100 {
		t.Fatalf("compared %d documents, want at least 100: the walk did not reach the repository's YAML", compared)
	}
	t.Logf("compared %d YAML documents", compared)

	numbers := []any{
		1.5, 0.1, 1e-7, 1e-6, 123.456, 1e20, 1e21, 1e22, 1.2345e30, -1e21,
		math.Pow(2, 62), math.Pow(2, 63), math.Pow(2, 64), -math.Pow(2, 63),
		math.MaxFloat64, math.SmallestNonzeroFloat64,
		int64(math.MaxInt64), int64(math.MinInt64), uint64(math.MaxUint64), int64(9007199254740993),
	}
	for _, n := range numbers {
		j, err := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "n": n})
		if err != nil {
			t.Fatal(err)
		}
		compareWithSigsYAML(t, string(j), j)
	}
}

// compareWithSigsYAML compares each writer's output for the JSON mapping j
// with sigs.k8s.io/yaml's output for the same input.
func compareWithSigsYAML(t *testing.T, name string, j []byte) {
	t.Helper()

	// The default path of EncodeObjectsToYAML emits the cleaned map.
	raw, err := decodeResourceMap(j)
	if err != nil {
		t.Errorf("%s: decode: %v", name, err)
		return
	}
	want, wantErr := sigsyaml.Marshal(raw)
	got, err := marshalYAML(raw)
	if wantErr == nil && (err != nil || !bytes.Equal(got, want)) {
		t.Errorf("%s: cleaned map differs from sigs.k8s.io/yaml (err %v):\n got %q\nwant %q", name, err, got, want)
	}

	// Marshal and Buffer.Marshal take any value. Numbers are kept as
	// json.Number, so both see each number as the JSON spells it.
	var v map[string]any
	dec := json.NewDecoder(bytes.NewReader(j))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Errorf("%s: unmarshal: %v", name, err)
		return
	}
	want, wantErr = sigsyaml.Marshal(v)
	var buf bytes.Buffer
	err = Marshal(&buf, v)
	if wantErr == nil && (err != nil || !bytes.Equal(buf.Bytes(), want)) {
		t.Errorf("%s: Marshal differs from sigs.k8s.io/yaml (err %v):\n got %q\nwant %q", name, err, buf.Bytes(), want)
	}
	var b Buffer
	err = b.Marshal(v)
	if wantErr == nil && (err != nil || !bytes.Equal(b.Bytes(), want)) {
		t.Errorf("%s: Buffer.Marshal differs from sigs.k8s.io/yaml (err %v):\n got %q\nwant %q", name, err, b.Bytes(), want)
	}

	// EncodeObjectsTo encodes an object, which needs a kind and apiVersion.
	if _, ok := v["kind"].(string); !ok {
		return
	}
	if _, ok := v["apiVersion"].(string); !ok {
		return
	}
	var obj client.Object = &unstructured.Unstructured{Object: v}
	serializer := kjson.NewSerializerWithOptions(kjson.DefaultMetaFactory, kubernetes.Scheme, kubernetes.Scheme,
		kjson.SerializerOptions{Yaml: true})
	var wantBuf bytes.Buffer
	if err := serializer.Encode(obj, &wantBuf); err != nil {
		return
	}
	wantBuf.WriteString("---\n")
	got, err = EncodeObjectsTo([]*client.Object{&obj}, true)
	if err != nil || !bytes.Equal(got, wantBuf.Bytes()) {
		t.Errorf("%s: EncodeObjectsTo differs from the YAML serializer (err %v):\n got %q\nwant %q", name, err, got, wantBuf.Bytes())
	}
}
