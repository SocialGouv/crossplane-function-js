package grpc

import (
	"context"
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/socialgouv/xfuncjs-server/pkg/context/fncontext"
	"github.com/socialgouv/xfuncjs-server/pkg/events"
	"github.com/socialgouv/xfuncjs-server/pkg/logger"
	"github.com/socialgouv/xfuncjs-server/pkg/types"
)

// RunFunction implements the RunFunction method of the FunctionRunnerService interface
func (f *Function) RunFunction(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	log := f.logger.WithValues("tag", req.GetMeta().GetTag())
	log.Info("Running Function")

	// Create a response with default TTL
	rsp := response.To(req, response.DefaultTTL)

	// Parse and validate input
	xfuncjsInput, err := parseInput(req)
	if err != nil {
		response.Fatal(rsp, err)
		return rsp, nil
	}

	// Prepare resources
	resources, err := prepareResources(req)
	if err != nil {
		response.Fatal(rsp, err)
		return rsp, nil
	}

	// Create enhanced input for JavaScript function
	enhancedInput, err := createEnhancedInput(xfuncjsInput, resources)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "failed to create enhanced input"))
		return rsp, nil
	}

	// Execute function
	result, err := f.executeFunction(ctx, xfuncjsInput, enhancedInput)
	if err != nil {
		response.Fatal(rsp, err)
		return rsp, nil
	}

	// Process result
	jsResponse, err := processResult(result)
	if err != nil {
		response.Fatal(rsp, err)
		return rsp, nil
	}

	// Build response
	if err := buildResponse(rsp, jsResponse, resources, f.logger); err != nil {
		response.Fatal(rsp, err)
		return rsp, nil
	}

	f.logger.Info("Successfully processed JavaScript function resources")
	return rsp, nil
}

// parseInput parses and validates the input from the request
func parseInput(req *fnv1.RunFunctionRequest) (*types.XFuncJSInput, error) {
	xfuncjsInput := &types.XFuncJSInput{}
	if err := request.GetInput(req, xfuncjsInput); err != nil {
		return nil, errors.Wrapf(err, "cannot get Function input from %T", req)
	}

	// Validate the input
	if err := xfuncjsInput.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid function input")
	}

	return xfuncjsInput, nil
}

// prepareResources prepares the resources from the request
type resourceBundle struct {
	oxr      *resource.Composite
	dxr      *resource.Composite
	observed map[resource.Name]resource.ObservedComposed
	desired  map[resource.Name]*resource.DesiredComposed
}

func prepareResources(req *fnv1.RunFunctionRequest) (*resourceBundle, error) {
	// Get the observed composite resource
	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get observed composite resource")
	}

	// Get the desired composite resource
	dxr, err := request.GetDesiredCompositeResource(req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get desired composite resource")
	}

	// Set API version and kind from observed to desired
	dxr.Resource.SetAPIVersion(oxr.Resource.GetAPIVersion())
	dxr.Resource.SetKind(oxr.Resource.GetKind())

	// Get the desired composed resources
	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get desired composed resources from %T", req)
	}

	// Get the observed composed resources
	observed, err := request.GetObservedComposedResources(req)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get observed composed resources from %T", req)
	}

	// Get extra resources
	_, err = request.GetExtraResources(req)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get extra resources from %T", req)
	}

	return &resourceBundle{
		oxr:      oxr,
		dxr:      dxr,
		observed: observed,
		desired:  desired,
	}, nil
}

// createEnhancedInput creates an enhanced input for the JavaScript function
func createEnhancedInput(xfuncjsInput *types.XFuncJSInput, resources *resourceBundle) (string, error) {
	// Create a structured input for the JavaScript function
	enhancedInput := map[string]interface{}{
		"input": map[string]interface{}{
			"apiVersion": xfuncjsInput.APIVersion,
			"kind":       xfuncjsInput.Kind,
			"spec":       xfuncjsInput.Spec,
		},
		"observed": map[string]interface{}{
			"composite": map[string]interface{}{
				"resource": resources.oxr.Resource.UnstructuredContent(),
			},
			"resources": ObservedToMap(resources.observed),
		},
	}

	// Convert the enhanced input to JSON
	enhancedInputJSON, err := json.Marshal(enhancedInput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal enhanced input to JSON")
	}

	return string(enhancedInputJSON), nil
}

// executeFunction executes the JavaScript function
func (f *Function) executeFunction(ctx context.Context, xfuncjsInput *types.XFuncJSInput, enhancedInput string) (string, error) {
	// Execute the function using the process manager with the enhanced input
	result, err := f.processManager.ExecuteFunction(ctx, xfuncjsInput, enhancedInput)
	if err != nil {
		return "", errors.Wrap(err, "error executing function")
	}

	return result, nil
}

// processResult processes the result from the JavaScript function
func processResult(result string) (*JSResponse, error) {
	// Parse the result
	var nodeResp struct {
		Result json.RawMessage `json:"result,omitempty"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Stack   string `json:"stack,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal([]byte(result), &nodeResp); err != nil {
		return nil, errors.Wrapf(err, "failed to parse Node.js response")
	}

	if nodeResp.Error != nil {
		return nil, errors.New(nodeResp.Error.Message)
	}

	// Process the JavaScript function's response
	jsResponse, err := ParseJSResponse(nodeResp.Result)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse JavaScript function response")
	}

	return jsResponse, nil
}

// buildResponse builds the final response
func buildResponse(rsp *fnv1.RunFunctionResponse, jsResponse *JSResponse, resources *resourceBundle, log logger.Logger) error {
	// Process resources
	if err := ProcessResources(rsp, resources.dxr, resources.desired, jsResponse); err != nil {
		return errors.Wrapf(err, "failed to process resources")
	}

	// Process events if present
	if len(jsResponse.Events) > 0 {
		// Convert our events to the events package format
		eventsToSet := make(events.JSEvents, len(jsResponse.Events))
		for i, e := range jsResponse.Events {
			var eventType *events.EventType
			if e.Event.Type != nil {
				t := events.EventType(*e.Event.Type)
				eventType = &t
			}
			eventsToSet[i] = events.CreateEvent{
				Target: e.Target,
				Event: events.Event{
					Type:    eventType,
					Reason:  e.Event.Reason,
					Message: e.Event.Message,
				},
			}
		}
		if err := events.SetEvents(rsp, eventsToSet); err != nil {
			return errors.Wrapf(err, "failed to process events")
		}
	}

	// Process context data if present
	if err := processContext(rsp, jsResponse, log); err != nil {
		return err
	}

	// Set desired composite resource
	if err := response.SetDesiredCompositeResource(rsp, resources.dxr); err != nil {
		return errors.Wrapf(err, "cannot set desired composite resource in %T", rsp)
	}

	// Set desired composed resources
	if err := response.SetDesiredComposedResources(rsp, resources.desired); err != nil {
		return errors.Wrapf(err, "cannot set desired composed resources in %T", rsp)
	}

	return nil
}

// processContext processes the context data from the JavaScript function response
func processContext(rsp *fnv1.RunFunctionResponse, jsResponse *JSResponse, log logger.Logger) error {
	if len(jsResponse.Context) == 0 {
		return nil
	}

	// Create a new RequestMeta from the ResponseMeta
	reqMeta := &fnv1.RequestMeta{
		Tag: rsp.Meta.GetTag(),
	}

	req := &fnv1.RunFunctionRequest{
		Meta:    reqMeta,
		Context: rsp.Context,
	}

	mergedCtx, err := fncontext.MergeContext(req, jsResponse.Context)
	if err != nil {
		return errors.Wrapf(err, "cannot merge Context")
	}

	for key, v := range mergedCtx {
		vv, err := structpb.NewValue(v)
		if err != nil {
			return errors.Wrap(err, "cannot convert value to structpb.Value")
		}
		log.Debug("Updating Composition environment", "key", key, "data", v)
		response.SetContextKey(rsp, key, vv)
	}

	return nil
}
