package appsets

import "testing"

func TestParsePatchLine(t *testing.T) {
	op, err := ParsePatchLine("spec/template/spec/containers[+=name=main]", map[string]interface{}{"image": "nginx"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if op.Op != "insertAfter" || op.Selector != "name=main" {
		t.Fatalf("unexpected op %+v", op)
	}
	if op.Path != "spec/template/spec/containers" {
		t.Fatalf("unexpected path %s", op.Path)
	}
}

func TestParsePatchPath(t *testing.T) {
	cases := []struct {
		in   string
		want []PathPart
	}{
		{
			in: "spec/template/spec/containers[0]/image",
			want: []PathPart{
				{Field: "spec"},
				{Field: "template"},
				{Field: "spec"},
				{Field: "containers", MatchType: "index", MatchValue: "0"},
				{Field: "image"},
			},
		},
		{
			in: "spec/containers[name=main]/image",
			want: []PathPart{
				{Field: "spec"},
				{Field: "containers", MatchType: "key", MatchValue: "name=main"},
				{Field: "image"},
			},
		},
	}

	for _, tc := range cases {
		got, err := ParsePatchPath(tc.in)
		if err != nil {
			t.Fatalf("ParsePatchPath error: %v", err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("segments len mismatch for %s: got %d want %d", tc.in, len(got), len(tc.want))
		}
		for i, p := range got {
			if p != tc.want[i] {
				t.Fatalf("segment %d mismatch for %s: got %+v want %+v", i, tc.in, p, tc.want[i])
			}
		}
	}
}
