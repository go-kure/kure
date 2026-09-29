package admission

import (
	"os"
	"strings"
	"testing"
)

// gapsGolden lists the generated bodies the grammar alone admits and a check
// before it refuses: the bodies the grammar does not yet read as the older
// checks do. It is grammarGaps for the generated bodies.
const gapsGolden = "testdata/gaps.golden"

// TestClassify_GeneratedGaps compares the generated bodies the grammar alone
// admits and a check before it refuses with gapsGolden, by name. A body
// marked unstable in differentialGolden is left out: whether a check before
// the grammar refuses it depends on the order of a map. The golden holds no
// reason, because that order also picks which of two older checks refuses a
// body with two nested pointer nil-inits; both are older checks, so the body
// is a gap either way.
func TestClassify_GeneratedGaps(t *testing.T) {
	if testing.Short() {
		t.Skip("classifies a generated package of some thousand helpers")
	}
	bodies := wellTyped(t, generate())
	dir := writeGenerated(t, genSource(bodies))
	verdicts := classifyDir(t, dir)
	own := grammarAlone(t, dir, nil)
	accepted, _ := readDifferentialGolden(t)

	var b strings.Builder
	b.WriteString("# The generated bodies the grammar alone admits and a check before it\n")
	b.WriteString("# refuses, one name per line. Written by TestClassify_GeneratedGaps with\n")
	b.WriteString("# " + goldenEnv + "=1; review the diff.\n")
	got := map[string]bool{}
	for _, body := range bodies {
		f, ok := verdicts[body.name]
		if !ok {
			t.Errorf("%s: not classified", body.name)
			continue
		}
		if len(accepted[body.name]) > 1 || own[body.name] != "" || !olderRefusal(f) {
			continue
		}
		b.WriteString(body.name + "\n")
		got[body.name] = true
	}
	if os.Getenv(goldenEnv) != "" {
		writeGolden(t, gapsGolden, b.String())
		t.Logf("wrote %s: %d bodies", gapsGolden, len(got))
		return
	}

	data, err := os.ReadFile(gapsGolden)
	if err != nil {
		t.Fatalf("%v; write it with %s=1", err, goldenEnv)
	}
	want := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			want[line] = true
		}
	}
	for _, body := range bodies {
		switch name := body.name; {
		case got[name] && !want[name]:
			t.Errorf("%s: a gap, not in %s (%s)", name, gapsGolden, verdicts[name].Reason)
		case want[name] && !got[name]:
			t.Errorf("%s: in %s, no longer a gap; the grammar alone gives %q", name, gapsGolden, own[name])
		}
		delete(want, body.name)
	}
	for name := range want {
		t.Errorf("%s: in %s but not generated", name, gapsGolden)
	}
}
