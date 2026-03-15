package remediation

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"encoding/json"
	"strings"
)

// Remediator re-applies manifests to fix drift
type Remediator struct {
	client  dynamic.Interface
	dryRun  bool
}

// NewRemediator creates a new Remediator
func NewRemediator(kubeconfigPath string, dryRun bool) (*Remediator, error) {
	var config *rest.Config
	var err error

	if kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Remediator{client: client, dryRun: dryRun}, nil
}

// RemediateManifest re-applies a single manifest to the cluster
func (r *Remediator) RemediateManifest(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read manifest %s: %w", path, err)
	}

	// Parse YAML into unstructured object
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse manifest %s: %w", path, err)
	}

	obj := &unstructured.Unstructured{Object: raw}
	if obj.GetKind() == "" || obj.GetName() == "" {
		return nil
	}

	gvr := gvrFromUnstructured(obj)
	namespace := obj.GetNamespace()
	name := obj.GetName()

	if r.dryRun {
		fmt.Printf("[DRY RUN] Would remediate %s/%s in namespace %s\n", obj.GetKind(), name, namespace)
		return nil
	}

	// Convert to JSON for server-side apply
	jsonData, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Use server-side apply (patch) to re-apply the manifest
	if namespace != "" {
		_, err = r.client.Resource(gvr).Namespace(namespace).Patch(
			ctx, name, types.ApplyPatchType, jsonData,
			metav1.PatchOptions{FieldManager: "driftguard", Force: func() *bool { b := true; return &b }()},
		)
	} else {
		_, err = r.client.Resource(gvr).Patch(
			ctx, name, types.ApplyPatchType, jsonData,
			metav1.PatchOptions{FieldManager: "driftguard", Force: func() *bool { b := true; return &b }()},
		)
	}

	if err != nil {
		return fmt.Errorf("failed to apply manifest %s: %w", path, err)
	}

	fmt.Printf("✓ Remediated %s/%s in namespace %s\n", obj.GetKind(), name, namespace)
	return nil
}

// gvrFromUnstructured maps an unstructured object to a GroupVersionResource
func gvrFromUnstructured(obj *unstructured.Unstructured) schema.GroupVersionResource {
	kindToResource := map[string]string{
		"Deployment":               "deployments",
		"Service":                  "services",
		"ConfigMap":                "configmaps",
		"ServiceAccount":           "serviceaccounts",
		"ClusterRole":              "clusterroles",
		"ClusterRoleBinding":       "clusterrolebindings",
		"Namespace":                "namespaces",
		"StatefulSet":              "statefulsets",
		"DaemonSet":                "daemonsets",
		"Ingress":                  "ingresses",
		"PersistentVolumeClaim":    "persistentvolumeclaims",
		"CustomResourceDefinition": "customresourcedefinitions",
	}

	resource, ok := kindToResource[obj.GetKind()]
	if !ok {
		resource = strings.ToLower(obj.GetKind()) + "s"
	}

	gv := obj.GroupVersionKind()
	return schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}
}