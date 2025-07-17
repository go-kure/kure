package bootstrap

import (
    "os"
    "sigs.k8s.io/yaml"
    sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
)

func PatchOCIRepositoryFromFile(path string, patchFn func(*sourcev1.OCIRepository) error) error {
    raw, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    var repo sourcev1.OCIRepository
    if err := yaml.Unmarshal(raw, &repo); err != nil {
        return err
    }

    if err := patchFn(&repo); err != nil {
        return err
    }

    newData, err := yaml.Marshal(repo)
    if err != nil {
        return err
    }

    return os.WriteFile(path, newData, 0644)
}
