package fncontext

import (
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
)

// MergeContext merges existing Context with new values provided
func MergeContext(req *fnv1.RunFunctionRequest, val map[string]interface{}) (map[string]interface{}, error) {
	mergedContext := req.GetContext().AsMap()
	if len(val) == 0 {
		return mergedContext, nil
	}

	// Merge the new values into the existing context
	for k, v := range val {
		mergedContext[k] = v
	}

	return mergedContext, nil
}
