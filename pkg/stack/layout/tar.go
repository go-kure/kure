package layout

import (
	"archive/tar"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kio "github.com/go-kure/kure/pkg/io"
)

// WriteToTar writes the ManifestLayout to a tar archive, mirroring the
// directory structure that WriteToDisk would produce. File paths use
// forward slashes and output is deterministic (sorted file names).
func (ml *ManifestLayout) WriteToTar(w io.Writer) error {
	tw := tar.NewWriter(w)
	defer func() { _ = tw.Close() }()
	return ml.writeToTarRecursive(tw, "")
}

func (ml *ManifestLayout) writeToTarRecursive(tw *tar.Writer, basePath string) error {
	fileMode := ml.FilePer
	if fileMode == FilePerUnset {
		fileMode = FilePerResource
	}
	appMode := ml.ApplicationFileMode
	if appMode == AppFileUnset {
		appMode = AppFilePerResource
	}

	var fullPath string
	if appMode == AppFileSingle {
		fullPath = path.Join(basePath, ml.Namespace)
	} else {
		fullPath = path.Join(basePath, ml.FullRepoPath())
	}

	// Add directory entry
	if err := writeTarDir(tw, fullPath); err != nil {
		return err
	}

	nameFn := ml.resolveManifestFileName()

	// Group resources into files
	fileGroups := map[string][]client.Object{}
	for _, obj := range ml.Resources {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = "cluster"
		}

		kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
		name := obj.GetName()

		var fileName string
		if appMode == AppFileSingle {
			fileName = fmt.Sprintf("%s.yaml", ml.Name)
		} else {
			fileName = nameFn(ns, kind, name, fileMode)
		}

		fileGroups[fileName] = append(fileGroups[fileName], obj)
	}

	// Sort file names for deterministic output
	sortedFileNames := make([]string, 0, len(fileGroups))
	for fileName := range fileGroups {
		sortedFileNames = append(sortedFileNames, fileName)
	}
	sort.Strings(sortedFileNames)

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
	kMode := ml.Mode
	if kMode == KustomizationUnset {
		kMode = KustomizationExplicit
	}

	if len(fileGroups) > 0 || len(ml.Children) > 0 {
		var kustomBuf strings.Builder
		kustomBuf.WriteString("apiVersion: kustomize.config.k8s.io/v1beta1\n")
		kustomBuf.WriteString("kind: Kustomization\n")
		kustomBuf.WriteString("resources:\n")

		if kMode == KustomizationExplicit || len(ml.Children) == 0 {
			for _, file := range sortedFileNames {
				kustomBuf.WriteString(fmt.Sprintf("  - %s\n", file))
			}
		}

		for _, child := range ml.Children {
			if child.UmbrellaChild {
				// Umbrella children are not referenced from the parent
				// kustomization.yaml's Children loop:
				//   - FluxIntegrated: the child's Kustomization CR is
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
			if child.ApplicationFileMode == AppFileSingle {
				kustomBuf.WriteString(fmt.Sprintf("  - %s.yaml\n", child.Name))
			} else if ml.FluxPlacement == FluxIntegrated {
				// FluxIntegrated: reference Flux Kustomization YAML files.
				// Always use FilePerResource here — each child must have a
				// unique filename; FilePerKind would drop child.Name.
				fluxKustName := nameFn("flux-system", "kustomization", child.Name, FilePerResource)
				kustomBuf.WriteString(fmt.Sprintf("  - %s\n", fluxKustName))
			} else {
				if ml.PackageRef != nil && child.PackageRef != nil && ml.PackageRef != child.PackageRef {
					continue
				}
				kustomBuf.WriteString(fmt.Sprintf("  - %s\n", child.Name))
			}
		}

		kustomBuf.WriteString(renderConfigMapGeneratorBlock(ml.ConfigMapGenerators))

		if err := writeTarFile(tw, path.Join(fullPath, "kustomization.yaml"), []byte(kustomBuf.String())); err != nil {
			return err
		}
	}

	// Recurse into children
	for _, child := range ml.Children {
		if err := child.writeToTarRecursive(tw, basePath); err != nil {
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
