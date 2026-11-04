package fluxcd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"google.golang.org/protobuf/types/known/anypb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/kubernetes/cilium"
)

// envoyConfig builds a Cilium Envoy config on kure's own builders, with res as
// its one resource, namespaced or cluster-wide.
func envoyConfig(clusterwide bool, res ciliumv2.XDSResource) client.Object {
	spec := ciliumv2.CiliumEnvoyConfigSpec{Resources: []ciliumv2.XDSResource{res}}
	if clusterwide {
		o := cilium.CreateCiliumClusterwideEnvoyConfig("cluster-proxy")
		o.Spec = spec
		return o
	}
	o := cilium.CreateCiliumEnvoyConfig("my-proxy", "default")
	o.Spec = spec
	return o
}

// refusals runs every entry point on r (runHostile) and returns the answers
// that are no layout error of any resource: the "integrated" placement is not
// one GenerateFromLayout supports, and a layout built by hand has no node.
func refusals(r func() client.Object) []string {
	var out []string
	for _, a := range runHostile(r, false) {
		if strings.Contains(a, "does not support FluxPlacement") || strings.Contains(a, `missing node "root"`) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func listenerResource(t *testing.T) ciliumv2.XDSResource {
	t.Helper()
	res, err := anypb.New(&listenerv3.Listener{Name: "listener"})
	if err != nil {
		t.Fatalf("wrapping the listener: %v", err)
	}
	return ciliumv2.XDSResource{Any: res}
}

// A Cilium Envoy config is a resource kure builds itself: its XDSResource
// marshals itself with protojson and calls no caller code, so the entry points
// read it as they did before the caller-object check.
func TestCallerObjects_EnvoyConfigsAreNotRefused(t *testing.T) {
	for _, clusterwide := range []bool{false, true} {
		name := "CiliumEnvoyConfig"
		if clusterwide {
			name = "CiliumClusterwideEnvoyConfig"
		}
		t.Run(name, func(t *testing.T) {
			res := listenerResource(t)
			if answers := refusals(func() client.Object { return envoyConfig(clusterwide, res) }); len(answers) > 0 {
				t.Errorf("an entry point refused the %s:\n%s", name, strings.Join(answers, "\n"))
			}
		})
	}
}

// An XDSResource with no Any, and one whose Any names a type nobody
// registered, must give an error or a refusal and never a panic.
func TestCallerObjects_EnvoyResourceWithoutOrUnresolvableAny(t *testing.T) {
	cases := map[string]ciliumv2.XDSResource{
		"nil Any":              {},
		"unresolvable TypeUrl": {Any: &anypb.Any{TypeUrl: "type.googleapis.com/no.such.Type", Value: []byte{1}}},
	}
	for name, res := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked: %v", r)
				}
			}()
			// Every answer is an error; reaching here without a panic is the point.
			_ = runHostile(func() client.Object { return envoyConfig(false, res) }, false)
			var obj client.Object = envoyConfig(false, res)
			_, err := kureio.EncodeObjectsToYAML([]*client.Object{&obj})
			if name == "unresolvable TypeUrl" && err == nil {
				t.Error("writing a resource whose Any cannot be resolved gave no error")
			}
		})
	}
}

// XDSResource is trusted by its exact type, not by its package: a type of
// another package that has its name and marshals itself stays refused.
type XDSResource struct{ N int }

func (XDSResource) MarshalJSON() ([]byte, error) { return []byte(`{}`), nil }

type lookalikeHolder struct {
	corev1.ConfigMap
	Resources []XDSResource `json:"resources"`
}

func (h *lookalikeHolder) DeepCopyObject() runtime.Object { c := *h; return &c }

func TestCallerObjects_TypeNamedLikeATrustedTypeIsRefused(t *testing.T) {
	r := func() client.Object {
		h := &lookalikeHolder{Resources: []XDSResource{{N: 1}}}
		h.APIVersion, h.Kind, h.Name, h.Namespace = "v1", "ConfigMap", "cm", "default"
		return h
	}
	answers := refusals(r)
	if len(answers) == 0 {
		t.Fatal("no entry point refused a type named like a trusted one")
	}
	for _, a := range answers {
		if !strings.Contains(a, "marshals itself") {
			t.Errorf("not refused for its marshaler: %s", a)
		}
	}
}

// The sweep: every typed object kure's own builders produce, as the golden
// fixtures hold them, passes the caller-object check through every entry
// point. A type refused here is a refusal the check must not make; report it,
// do not allowlist it unread.
func TestCallerObjects_GoldenFixturesOfTheBuildersAreNotRefused(t *testing.T) {
	if err := kubernetes.RegisterSchemes(); err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, pattern := range []string{
		"../../kubernetes/testdata/*.yaml",
		"../../kubernetes/*/testdata/*.yaml",
		"../../kubernetes/internal/*/testdata/*.yaml",
	} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) < 40 {
		t.Fatalf("found %d golden fixtures, want the whole set (56 at the time of writing)", len(files))
	}
	objects := 0
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // a test fixture
		if err != nil {
			t.Fatal(err)
		}
		for i, doc := range bytes.Split(data, []byte("\n---\n")) {
			if len(bytes.TrimSpace(doc)) == 0 {
				continue
			}
			obj, _, err := kubernetes.Codecs.UniversalDeserializer().Decode(doc, nil, nil)
			if err != nil {
				t.Errorf("%s document %d does not decode into a typed object: %v", f, i, err)
				continue
			}
			co, ok := obj.(client.Object)
			if !ok {
				continue
			}
			objects++
			name := filepath.Base(filepath.Dir(filepath.Dir(f))) + "/" + filepath.Base(f)
			if answers := refusals(func() client.Object { return co.DeepCopyObject().(client.Object) }); len(answers) > 0 {
				t.Errorf("%s (%T) was refused:\n%s", name, co, strings.Join(answers, "\n"))
			}
		}
	}
	if objects < 40 {
		t.Fatalf("swept %d objects, want at least 40", objects)
	}
}

// rawSelfList holds one item as a RawExtension.
type rawSelfList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []runtime.RawExtension `json:"items"`
}

func (l *rawSelfList) DeepCopyObject() runtime.Object { c := *l; return &c }

const rawConfigMapJSON = `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"raw","namespace":"default"}}`

func rawSelfListOf(raw []byte) client.Object {
	l := &rawSelfList{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
	l.Name, l.Namespace = "holder", "default"
	l.Items = []runtime.RawExtension{{Raw: raw}}
	l.Items[0].Object = l
	return l
}

// A RawExtension with raw JSON is written from the JSON, and its Object is
// never looked at, so an Object that is the List itself is no cycle: every
// entry point, in every placement, runs to its end and the List is written.
func TestCallerObjects_RawExtensionWithRawJSONIgnoresItsObject(t *testing.T) {
	if answers := refusals(func() client.Object { return rawSelfListOf([]byte(rawConfigMapJSON)) }); len(answers) > 0 {
		t.Errorf("an entry point refused a RawExtension whose raw JSON is set:\n%s", strings.Join(answers, "\n"))
	}
	obj := rawSelfListOf([]byte(rawConfigMapJSON))
	data, err := kureio.EncodeObjectsToYAML([]*client.Object{&obj})
	if err != nil {
		t.Fatalf("writing the List: %v", err)
	}
	if !strings.Contains(string(data), "name: raw") {
		t.Errorf("the List is not written from the raw JSON:\n%s", data)
	}
}

// Without raw JSON the Object is what is written, so an Object that is the List
// itself has no end and stays refused.
func TestCallerObjects_RawExtensionWithoutRawJSONHoldingItsListIsRefused(t *testing.T) {
	answers := refusals(func() client.Object { return rawSelfListOf(nil) })
	if len(answers) == 0 {
		t.Fatal("no entry point refused a List that holds itself through a RawExtension")
	}
	for _, a := range answers {
		if !strings.Contains(a, "holds itself") {
			t.Errorf("refused for another reason: %s", a)
		}
	}
}
