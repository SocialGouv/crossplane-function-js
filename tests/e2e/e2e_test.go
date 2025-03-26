package e2e

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCrossplaneXFuncJS(t *testing.T) {
	// Check if we're running in a Kubernetes environment
	// We'll check for the KUBERNETES_SERVICE_HOST environment variable
	// which is set in Kubernetes pods
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		t.Skip("Skipping test: not running in a Kubernetes environment")
	}

	// Load kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		t.Fatalf("Error building kubeconfig: %v", err)
	}

	// Create dynamic client
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatalf("Error creating client: %v", err)
	}

	// Define GVR for SimpleConfigMap
	xsimpleConfigMapGVR := schema.GroupVersionResource{
		Group:    "test.crossplane.io",
		Version:  "v1beta1",
		Resource: "xsimpleconfigmaps",
	}

	// Define GVR for ConfigMap
	configMapGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	// Create test namespace
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": "test-xfuncjs",
			},
		},
	}
	_, err = client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}).Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Error creating namespace: %v", err)
	}

	// Create XSimpleConfigMap
	xsimpleConfigMap := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test.crossplane.io/v1beta1",
			"kind":       "XSimpleConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-simple-configmap",
				"namespace": "test-xfuncjs",
			},
			"spec": map[string]interface{}{
				"data": map[string]interface{}{
					"name":  "John Doe",
					"email": "john.doe@example.com",
					"role":  "developer",
				},
			},
		},
	}

	_, err = client.Resource(xsimpleConfigMapGVR).Namespace("test-xfuncjs").Create(context.TODO(), xsimpleConfigMap, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating XSimpleConfigMap: %v", err)
	}

	// Wait for ConfigMap to be created
	var configMap *unstructured.Unstructured
	for i := 0; i < 30; i++ {
		configMap, err = client.Resource(configMapGVR).Namespace("test-xfuncjs").Get(context.TODO(), "generated-configmap", metav1.GetOptions{})
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		t.Fatalf("Error getting ConfigMap: %v", err)
	}

	// Verify ConfigMap data
	data, found, err := unstructured.NestedMap(configMap.Object, "data")
	if err != nil || !found {
		t.Fatalf("ConfigMap data not found: %v", err)
	}

	// Check that the data was transformed correctly (uppercase)
	expectedKeys := []string{"NAME", "EMAIL", "ROLE"}
	expectedValues := []string{"JOHN DOE", "JOHN.DOE@EXAMPLE.COM", "DEVELOPER"}

	for i, key := range expectedKeys {
		value, found := data[key]
		if !found {
			t.Errorf("Key %s not found in ConfigMap data", key)
			continue
		}
		if value != expectedValues[i] {
			t.Errorf("Expected %s=%s, got %s=%s", key, expectedValues[i], key, value)
		}
	}
}
