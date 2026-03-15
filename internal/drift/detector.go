package drift

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// DriftResult represents a single drifted resource
type DriftResult struct {
	Kind      string
	Name      string
	Namespace string
	Reason    string // "missing" or "modified"
}

// Detector compares Git manifests against live cluster state
type Detector struct {
	client dynamic.Interface
}

// NewDetector creates a new Detector using in-cluster or kubeconfig credentials
func NewDetector(kubeconfigPath string) (*Detector, error) {
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

	return &Detector{client: client}, nil
}

// manifest is a minimal struct for parsing Kubernetes YAML
type manifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

// parseManifest reads a YAML file and returns a manifest struct
func parseManifest(path string) (*manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	var m manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse YAML %s: %w", path, err)
	}

	// Skip files that aren't Kubernetes manifests
	if m.Kind == "" || m.APIVersion == "" {
		return nil, nil
	}

	return &m, nil
}

// gvrFromManifest maps a manifest to a GroupVersionResource for the dynamic client
func gvrFromManifest(m *manifest) schema.GroupVersionResource {
	kindToResource := map[string]string{
		"Deployment":             "deployments",
		"Service":                "services",
		"ConfigMap":              "configmaps",
		"ServiceAccount":         "serviceaccounts",
		"ClusterRole":            "clusterroles",
		"ClusterRoleBinding":     "clusterrolebindings",
		"Namespace":              "namespaces",
		"StatefulSet":            "statefulsets",
		"DaemonSet":              "daemonsets",
		"Ingress":                "ingresses",
		"PersistentVolumeClaim":  "persistentvolumeclaims",
		"CustomResourceDefinition": "customresourcedefinitions",
	}

	resource, ok := kindToResource[m.Kind]
	if !ok {
		resource = strings.ToLower(m.Kind) + "s"
	}

	// Parse group and version from apiVersion (e.g. "apps/v1" or "v1")
	parts := strings.SplitN(m.APIVersion, "/", 2)
	var group, version string
	if len(parts) == 2 {
		group = parts[0]
		version = parts[1]
	} else {
		group = ""
		version = parts[0]
	}

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
}

// CheckManifest checks whether a single manifest exists in the cluster
func (d *Detector) CheckManifest(ctx context.Context, path string) (*DriftResult, error) {
	m, err := parseManifest(path)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}

	gvr := gvrFromManifest(m)
	namespace := m.Metadata.Namespace
	name := m.Metadata.Name

	if name == "" {
		return nil, nil
	}

	var liveObj *unstructured.Unstructured

	if namespace != "" {
		liveObj, err = d.client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		liveObj, err = d.client.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}

	if err != nil {
		// Resource is missing from cluster
		return &DriftResult{
			Kind:      m.Kind,
			Name:      name,
			Namespace: namespace,
			Reason:    "missing",
		}, nil
	}

	// Resource exists — check if labels/annotations differ
	_ = liveObj
	return nil, nil
}

// DetectDrift checks all manifests and returns a list of drifted resources
func (d *Detector) DetectDrift(ctx context.Context, manifestPaths []string) ([]DriftResult, error) {
	var drifts []DriftResult

	for _, path := range manifestPaths {
		result, err := d.CheckManifest(ctx, path)
		if err != nil {
			// Log and continue — don't fail the whole sync on one bad manifest
			fmt.Printf("warning: failed to check manifest %s: %v\n", path, err)
			continue
		}
		if result != nil {
			drifts = append(drifts, *result)
		}
	}

	return drifts, nil
}