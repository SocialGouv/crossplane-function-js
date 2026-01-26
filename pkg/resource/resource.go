package resource

import (
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"

	"github.com/socialgouv/xfuncjs-server/pkg/events"
)

// JSResponse represents the response from a JavaScript function
type JSResponse struct {
	// Resources is a map of resource name to resource object
	Resources map[string]JSResource `json:"resources"`
	// Events is a list of events to create
	Events events.JSEvents `json:"events,omitempty"`
	// Context is a map of context data to add to the response
	Context map[string]interface{} `json:"context,omitempty"`
}

// JSResource represents a resource in the JavaScript function response
type JSResource struct {
	// Resource is the Kubernetes resource
	Resource json.RawMessage `json:"resource"`
	// Ready indicates if the resource is ready
	Ready *bool `json:"ready,omitempty"`
	// ConnectionDetails is a list of connection details to extract
	ConnectionDetails []string `json:"connectionDetails,omitempty"`
}

// ParseJSResponse parses the JavaScript function response
func ParseJSResponse(result json.RawMessage) (*JSResponse, error) {
	var jsResponse JSResponse
	if err := json.Unmarshal(result, &jsResponse); err != nil {
		return nil, errors.Wrapf(err, "failed to parse JavaScript function response")
	}
	return &jsResponse, nil
}

// ObservedToMap converts observed resources to a map for JavaScript input
func ObservedToMap(observed map[resource.Name]resource.ObservedComposed) map[string]interface{} {
	resources := make(map[string]interface{})
	for name, resource := range observed {
		if resource.Resource != nil {
			resources[string(name)] = map[string]interface{}{
				"resource": resource.Resource.UnstructuredContent(),
			}
		}
	}
	return resources
}

// ProcessResources processes the resources from the JavaScript function response
func ProcessResources(rsp *fnv1.RunFunctionResponse, dxr *resource.Composite, desired map[resource.Name]*resource.DesiredComposed, jsResponse *JSResponse) error {
	// Process resources
	for name, res := range jsResponse.Resources {
		// Parse the resource
		var resourceMap map[string]interface{}
		if err := json.Unmarshal(res.Resource, &resourceMap); err != nil {
			return errors.Wrapf(err, "error unmarshaling resource %s", name)
		}

		// Create a new desired composed resource
		cd := resource.NewDesiredComposed()
		cd.Resource.Object = resourceMap

		// Set ready status if provided
		if res.Ready != nil {
			if *res.Ready {
				cd.Ready = resource.ReadyTrue
			} else {
				cd.Ready = resource.ReadyFalse
			}
		}

		// Connection details are not supported in this version of function-sdk-go

		// Add the resource to the desired resources
		desired[resource.Name(name)] = cd
	}

	return nil
}
