package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	kio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/k8s"
	"github.com/go-kure/kure/pkg/layout"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	baseDir := flag.String("base", "examples/cluster", "path to cluster example")
	flag.Parse()

	cl, err := loadCluster(*baseDir)
	if err != nil {
		log.Fatalf("load cluster: %v", err)
	}

	// walk cluster to build manifest layout
	rules := layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat}
	ml, err := layout.WalkCluster(cl, rules)
	if err != nil {
		log.Fatalf("walk cluster: %v", err)
	}

	// write manifests to a temporary directory
	tempDir, err := os.MkdirTemp("", "stack-demo-")
	if err != nil {
		log.Fatalf("create temp dir: %v", err)
	}
	cfg := layout.DefaultLayoutConfig()
	if err := layout.WriteManifest(tempDir, cfg, ml); err != nil {
		log.Fatalf("write manifests: %v", err)
	}
	log.Printf("manifests written to %s", tempDir)

	// build Flux kustomizations from the cluster
	wf := fluxstack.NewWorkflow()
	fluxObjs, err := wf.Cluster(cl)
	if err != nil {
		log.Fatalf("flux workflow: %v", err)
	}
	var ptrs []*client.Object
	for _, obj := range fluxObjs {
		o := obj
		ptrs = append(ptrs, &o)
	}
	out, err := kio.EncodeObjectsToYAML(ptrs)
	if err != nil {
		log.Fatalf("encode flux objects: %v", err)
	}
	os.Stdout.Write(out)
}

func loadCluster(baseDir string) (*stack.Cluster, error) {
	file, err := os.Open(filepath.Join(baseDir, "cluster.yaml"))
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) { _ = f.Close() }(file)

	dec := yaml.NewDecoder(file)
	var cl stack.Cluster
	if err := dec.Decode(&cl); err != nil {
		return nil, err
	}
	if cl.Node == nil {
		return &cl, nil
	}

	for _, child := range cl.Node.Children {
		child.Parent = cl.Node
		dir := filepath.Join(baseDir, child.Name)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := filepath.Ext(entry.Name())
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			fp := filepath.Join(dir, entry.Name())
			f, err := os.Open(fp)
			if err != nil {
				return nil, err
			}
			dec := yaml.NewDecoder(f)
			for {
				var cfg k8s.AppWorkloadConfig
				if err := dec.Decode(&cfg); err != nil {
					if err == io.EOF {
						break
					}
					_ = f.Close()
					return nil, err
				}
				app := stack.NewApplication(cfg.Name, cfg.Namespace, &cfg)
				bundle, err := stack.NewBundle(cfg.Name, []*stack.Application{app}, nil)
				if err != nil {
					_ = f.Close()
					return nil, err
				}
				if err := bundle.Validate(); err != nil {
					_ = f.Close()
					return nil, err
				}
				node := &stack.Node{Name: cfg.Name, Parent: child, Bundle: bundle}
				child.Children = append(child.Children, node)
			}
			_ = f.Close()
		}
	}

	return &cl, nil
}
