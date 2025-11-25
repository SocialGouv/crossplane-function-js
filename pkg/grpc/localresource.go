package grpc

import (
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/socialgouv/xfuncjs-server/pkg/conditions"
)

// JSResponse represents the response from a JavaScript function
type JSResponse struct {
	// Composite represents the desired composite resource itself
	Composite *JSResource `json:"composite,omitempty"`
	// Resources is a map of resource name to resource object
	Resources map[string]JSResource `json:"resources"`
	// Events is a list of events to create
	Events []CreateEvent `json:"events,omitempty"`
	// Conditions is a list of conditions to create
	Conditions []conditions.ConditionResource `json:"conditions,omitempty"`
	// Context is a map of context data to add to the response
	Context map[string]interface{} `json:"context,omitempty"`
	// ExtraResourceRequirements is a map of resource name to resource requirements
	ExtraResourceRequirements map[string]ExtraResourceRequirement `json:"extraResourceRequirements,omitempty"`
}

// ExtraResourceRequirement defines a requirement for extra resources
type ExtraResourceRequirement struct {
	// APIVersion of the resource
	APIVersion string `json:"apiVersion"`
	// Kind of the resource
	Kind string `json:"kind"`
	// MatchLabels defines the labels to match the resource
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// MatchName defines the name to match the resource
	MatchName string `json:"matchName,omitempty"`
	// Namespace optionally constrains the selector to a single namespace.
	//
	// When omitted, Crossplane will:
	//   - match cluster-scoped resources, or
	//   - match namespaced resources by labels across all namespaces
	//     (per the v1beta1 ResourceSelector semantics).
	Namespace string `json:"namespace,omitempty"`
}

// ToResourceSelector converts the ExtraResourceRequirement to a fnv1.ResourceSelector
func (e *ExtraResourceRequirement) ToResourceSelector() *fnv1.ResourceSelector {
	out := &fnv1.ResourceSelector{
		ApiVersion: e.APIVersion,
		Kind:       e.Kind,
	}

	if e.MatchName == "" && len(e.MatchLabels) > 0 {
		out.Match = &fnv1.ResourceSelector_MatchLabels{
			MatchLabels: &fnv1.MatchLabels{Labels: e.MatchLabels},
		}
	} else if e.MatchName != "" {
		out.Match = &fnv1.ResourceSelector_MatchName{
			MatchName: e.MatchName,
		}
	}

	// Note: the v1 ResourceSelector type from function-sdk-go currently has no
	// explicit Namespace field in this environment, so we can't forward
	// ExtraResourceRequirement.Namespace yet. Leaving it here keeps the JSON
	// contract ready for future SDKs without breaking compilation today.

	return out
}

// JSResource represents a resource in the JavaScript function response
type JSResource struct {
	// Resource is the Kubernetes resource
	Resource json.RawMessage `json:"resource"`
	// Ready indicates if the resource is ready
	Ready *bool `json:"ready,omitempty"`
	// ConnectionDetails contains connection details for the resource
	ConnectionDetails map[string]string `json:"connectionDetails,omitempty"`
}

// CreateEvent will create an event for the target(s).
type CreateEvent struct {
	// The target(s) to create an event for. Can be Composite or
	// CompositeAndClaim. Defaults to Composite
	Target *string `json:"target"`

	// Event to create.
	Event Event `json:"event"`
}

// Event allows you to specify the fields of an event to create.
type Event struct {
	// Type of the event. Optional. Should be either Normal or Warning.
	Type *string `json:"type"`
	// Reason of the event. Optional.
	Reason *string `json:"reason"`
	// Message of the event. Required.
	Message string `json:"message"`
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
			resourceMap := map[string]interface{}{
				"resource": resource.Resource.UnstructuredContent(),
			}

			// Add connection details if present
			if len(resource.ConnectionDetails) > 0 {
				connectionDetails := make(map[string]string)
				for k, v := range resource.ConnectionDetails {
					connectionDetails[k] = string(v)
				}
				resourceMap["connectionDetails"] = connectionDetails
			}

			resources[string(name)] = resourceMap
		}
	}
	return resources
}

// ProcessResources processes the resources from the JavaScript function response
func ProcessResources(rsp *fnv1.RunFunctionResponse, dxr *resource.Composite, desired map[resource.Name]*resource.DesiredComposed, jsResponse *JSResponse) error {
	// First, process the desired composite resource if provided
	if jsResponse.Composite != nil {
		var compositeMap map[string]interface{}
		if err := json.Unmarshal(jsResponse.Composite.Resource, &compositeMap); err != nil {
			return errors.Wrapf(err, "error unmarshaling composite resource")
		}

		// Remove the namespace from the resource metadata if it exists
		// This prevents Crossplane from trying to add it to resourceRefs
		if metadata, ok := compositeMap["metadata"].(map[string]interface{}); ok {
			if _, ok := metadata["namespace"].(string); ok {
				delete(metadata, "namespace")
			}
		}

		// Set the desired composite resource object (spec, status, metadata, etc.)
		dxr.Resource.Object = compositeMap

		// Apply connection details for the composite if provided
		if len(jsResponse.Composite.ConnectionDetails) > 0 {
			if dxr.ConnectionDetails == nil {
				dxr.ConnectionDetails = make(map[string][]byte)
			}
			for k, v := range jsResponse.Composite.ConnectionDetails {
				dxr.ConnectionDetails[k] = []byte(v)
			}
		}
	}

	// Then process composed resources as before
	for name, res := range jsResponse.Resources {
		// Parse the resource
		var resourceMap map[string]interface{}
		if err := json.Unmarshal(res.Resource, &resourceMap); err != nil {
			return errors.Wrapf(err, "error unmarshaling resource %s", name)
		}

		// Remove the namespace from the resource metadata if it exists
		// This prevents Crossplane from trying to add it to resourceRefs
		if metadata, ok := resourceMap["metadata"].(map[string]interface{}); ok {
			if _, ok := metadata["namespace"].(string); ok {
				// Remove the namespace from the resource metadata
				delete(metadata, "namespace")
			}
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

		// Process connection details if provided
		if len(res.ConnectionDetails) > 0 {
			// For composed resources, we don't set connection details directly
			// They will be collected by Crossplane from the actual resources
			// But for the composite resource (XR), we can set connection details
			if string(name) == "composite" {
				if dxr.ConnectionDetails == nil {
					dxr.ConnectionDetails = make(map[string][]byte)
				}
				for k, v := range res.ConnectionDetails {
					dxr.ConnectionDetails[k] = []byte(v)
				}
			}
		}

		// Add the resource to the desired resources
		desired[resource.Name(name)] = cd
	}

	return nil
}
