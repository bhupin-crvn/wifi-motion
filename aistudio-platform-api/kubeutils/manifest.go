package kubeutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
)

func ApplyManifest(filePath, namespace string, dynClient dynamic.Interface, mapper meta.RESTMapper) error {
	if !strings.HasSuffix(filePath, ".yaml") && !strings.HasSuffix(filePath, ".yml") {
		return nil
	}
	fmt.Printf("Applying: %s\n", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	docs := bytes.Split(data, []byte("---"))
	for i, doc := range docs {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(doc), 4096)
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			return fmt.Errorf("failed to decode YAML document %d in %s: %w", i, filePath, err)
		}

		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to parse object in document %d of %s: %w", i, filePath, err)
		}
		unstructuredObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("failed to convert to unstructured object in document %d of %s", i, filePath)
		}

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("failed to get REST mapping for %s in document %d of %s: %w", gvk.String(), i, filePath, err)
		}

		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			if unstructuredObj.GetNamespace() == "" {
				unstructuredObj.SetNamespace(namespace)
			}
			dr = dynClient.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
		} else {
			dr = dynClient.Resource(mapping.Resource)
		}

		_, err = dr.Create(context.Background(), unstructuredObj, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			_, err = dr.Update(context.Background(), unstructuredObj, metav1.UpdateOptions{})
		}
		if err != nil {
			return fmt.Errorf("failed to apply document %d in %s: %w", i, filePath, err)
		}
		fmt.Printf(" Applied document %d in %s\n", i, filePath)
	}

	return nil
}
