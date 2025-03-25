package node

import (
	"github.com/socialgouv/crossplane-skyhook/pkg/types"
)

// extractResourceInfo extracts resource information from the input data
func extractResourceInfo(data map[string]interface{}) *types.ResourceInfo {
	resourceInfo := &types.ResourceInfo{}

	// Check for Crossplane composite resource in the input
	if input, ok := data["input"].(map[string]interface{}); ok {
		// Try to extract from apiVersion and kind
		if apiVersion, ok := input["apiVersion"].(string); ok {
			resourceInfo.Version = apiVersion
		}

		if kind, ok := input["kind"].(string); ok {
			resourceInfo.Kind = kind
		}

		// Try to extract from metadata
		if metadata, ok := input["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				resourceInfo.Name = name
			}

			if namespace, ok := metadata["namespace"].(string); ok {
				resourceInfo.Namespace = namespace
			}
		}
	}

	// Check for observed resources
	if observed, ok := data["observed"].(map[string]interface{}); ok {
		if composite, ok := observed["composite"].(map[string]interface{}); ok {
			if resource, ok := composite["resource"].(map[string]interface{}); ok {
				// Try to extract from apiVersion and kind
				if apiVersion, ok := resource["apiVersion"].(string); ok && resourceInfo.Version == "" {
					resourceInfo.Version = apiVersion
				}

				if kind, ok := resource["kind"].(string); ok && resourceInfo.Kind == "" {
					resourceInfo.Kind = kind
				}

				// Try to extract from metadata
				if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
					if name, ok := metadata["name"].(string); ok && resourceInfo.Name == "" {
						resourceInfo.Name = name
					}

					if namespace, ok := metadata["namespace"].(string); ok && resourceInfo.Namespace == "" {
						resourceInfo.Namespace = namespace
					}
				}
			}
		}
	}

	// If we couldn't extract any resource information, return nil
	if resourceInfo.Version == "" && resourceInfo.Kind == "" && resourceInfo.Name == "" && resourceInfo.Namespace == "" {
		return nil
	}

	return resourceInfo
}
