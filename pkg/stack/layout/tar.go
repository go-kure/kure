package layout

import (
	"archive/tar"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kio "github.com/go-kure/kure/pkg/io"
)

// WriteToTar writes the ManifestLayout to a tar archive, mirroring the
// directory structure that WriteToDisk would produce. File paths use
// forward slashes and output is deterministic (sorted file names). It refuses
// a tree in which two layouts resolve to the same directory (see
// checkLayoutTree) before writing any entry.
func (ml *ManifestLayout) WriteToTar(w io.Writer) error {
	plan := tarPlan("")
	if err := checkLayoutTree(ml, plan); err != nil {
		return err
	}
	tw := tar.NewWriter(w)
	defer func() { _ = tw.Close() }()
	return ml.writeToTarRecursive(tw, plan, true)
}

// writeToTarRecursive writes ml and its children as writeToDisk does; root
// is false for a child, and an AppFileSingle child adds no
// kustomization.yaml entry (a later entry for its parent's path would shadow
// the parent's, go-kure/kure#860).
func (ml *ManifestLayout) writeToTarRecursive(tw *tar.Writer, plan writerPlan, root bool) error {
	fullPath, _ := plan.outDir(ml)
	sortedFileNames, fileGroups := plan.files(ml)
	if err := checkExtraFiles(ml, plan.outDir, sortedFileNames); err != nil {
		return err
	}
	// Added only after the check: the archive is a stream, so an entry for a
	// refused layout could not be taken back.
	if err := writeTarDir(tw, fullPath); err != nil {
		return err
	}

	// Write resource files
	for _, fileName := range sortedFileNames {
		objs := fileGroups[fileName]
		var objPtrs []*client.Object
		for _, obj := range objs {
			objPtr := &obj
			objPtrs = append(objPtrs, objPtr)
		}

		data, err := kio.EncodeObjectsToYAML(objPtrs)
		if err != nil {
			return err
		}

		if err := writeTarFile(tw, path.Join(fullPath, fileName), data); err != nil {
			return err
		}
	}

	if err := writeExtraFilesToTar(tw, fullPath, ml.ExtraFiles); err != nil {
		return err
	}

	// Write kustomization.yaml
	kMode := plan.kustomizationMode(ml)

	if plan.writesKustomization(ml, root) {
		var kustomBuf strings.Builder
		kustomBuf.WriteString("apiVersion: kustomize.config.k8s.io/v1beta1\n")
		kustomBuf.WriteString("kind: Kustomization\n")
		// The header is written with the first entry: a kustomization that
		// lists nothing says so with "resources: []", since kustomize refuses
		// a bare "resources:" with nothing else as an empty kustomization.
		listed := false
		entry := func(s string) {
			if !listed {
				kustomBuf.WriteString("resources:\n")
				listed = true
			}
			kustomBuf.WriteString(s)
		}

		// Every resource file in explicit mode or for a leaf; see
		// listedResourceFiles for recursive mode.
		for _, file := range listedResourceFiles(ml, kMode, sortedFileNames, fileGroups) {
			entry(fmt.Sprintf("  - %s\n", file))
		}

		for _, child := range ml.Children {
			if child.UmbrellaChild {
				// Umbrella children are not referenced from the parent
				// kustomization.yaml's Children loop:
				//   - FluxIntegratedPerLayout: the child's Kustomization CR is
				//     already in ml.Resources (placed there by the
				//     LayoutIntegrator), so the Resources loop above
				//     emits the filename exactly once.
				//   - FluxSeparate: the child is applied by its own CR
				//     under flux-system/ with spec.path pointing directly
				//     at the child subdir, so the parent must not
				//     reference it at all.
				// The sub-layout is still walked below to write its
				// workloads + own kustomization.yaml.
				continue
			}
			if child.rendersBundle() {
				// The child renders bundles, so it is a reconciliation unit:
				// its own Flux Kustomization (or ArgoCD Application) applies
				// it, and only that one. Listing it here too would apply its
				// objects twice, under two owners, and put them in reach of
				// this directory's patches.
				continue
			}
			if child.ApplicationFileMode == AppFileSingle {
				if child.writesSingleFile() {
					entry(fmt.Sprintf("  - %s.yaml\n", child.Name))
				}
			} else if ml.FluxPlacement == FluxIntegratedPerLayout {
				// FluxIntegratedPerLayout: the child is applied by the Flux
				// Kustomization the integrator placed in ml.Resources, which
				// the resource list above already names. Nothing is guessed
				// from the child's name.
				continue
			} else {
				if ml.PackageRef != nil && child.PackageRef != nil && ml.PackageRef != child.PackageRef {
					continue
				}
				entry(fmt.Sprintf("  - %s\n", child.Name))
			}
		}
		if !listed {
			kustomBuf.WriteString("resources: []\n")
		}

		kustomBuf.WriteString(renderConfigMapGeneratorBlock(ml.ConfigMapGenerators))

		if err := writeTarFile(tw, path.Join(fullPath, "kustomization.yaml"), []byte(kustomBuf.String())); err != nil {
			return err
		}
	}

	// Recurse into children
	for _, child := range ml.Children {
		if err := child.writeToTarRecursive(tw, plan, false); err != nil {
			return err
		}
	}

	return nil
}

// writeTarDir adds a directory entry to the tar archive.
func writeTarDir(tw *tar.Writer, dirPath string) error {
	hdr := &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     dirPath + "/",
		Mode:     0755,
		ModTime:  time.Time{},
	}
	return tw.WriteHeader(hdr)
}

// writeTarFile adds a file entry to the tar archive.
func writeTarFile(tw *tar.Writer, filePath string, data []byte) error {
	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     filePath,
		Size:     int64(len(data)),
		Mode:     0644,
		ModTime:  time.Time{},
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
