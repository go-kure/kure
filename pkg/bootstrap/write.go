package bootstrap

import (
    "os"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/yaml"
)

func WriteYAMLResource(path string, obj client.Object) error {
    data, err := yaml.Marshal(obj)
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}
