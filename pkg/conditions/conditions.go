package conditions

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/socialgouv/xfuncjs-server/pkg/logger"
	"k8s.io/utils/ptr"
)

// ConditionStatus represents the status of a condition
type ConditionStatus string

const (
	// ConditionTrue means the condition is true
	ConditionTrue ConditionStatus = "True"

	// ConditionFalse means the condition is false
	ConditionFalse ConditionStatus = "False"

	// ConditionUnknown means the condition is unknown
	ConditionUnknown ConditionStatus = "Unknown"
)

// Target determines which objects to set the condition on
type Target string

const (
	// TargetComposite targets only the composite resource
	TargetComposite Target = "Composite"

	// TargetCompositeAndClaim targets both the composite and the claim
	TargetCompositeAndClaim Target = "CompositeAndClaim"
)

// ConditionResources is a list of condition resources
type ConditionResources []ConditionResource

// ConditionResource will set a condition on the target
type ConditionResource struct {
	// The target(s) to receive the condition. Can be Composite or
	// CompositeAndClaim. Defaults to Composite
	Target *Target `json:"target"`
	// If true, the condition will override a condition of the same Type. Defaults
	// to false.
	Force *bool `json:"force"`
	// Condition to set.
	Condition Condition `json:"condition"`
}

// Condition allows you to specify fields to set on a composite resource and claim
type Condition struct {
	// Type of the condition. Required.
	Type string `json:"type"`
	// Status of the condition. Required.
	Status ConditionStatus `json:"status"`
	// Reason of the condition. Required.
	Reason string `json:"reason"`
	// Message of the condition. Optional.
	Message *string `json:"message"`
}

// transformCondition converts a ConditionResource into an fnv1.Condition
func transformCondition(cs ConditionResource) *fnv1.Condition {
	c := &fnv1.Condition{
		Type:   cs.Condition.Type,
		Reason: cs.Condition.Reason,
		Target: transformTarget(cs.Target),
	}

	switch cs.Condition.Status {
	case ConditionTrue:
		c.Status = fnv1.Status_STATUS_CONDITION_TRUE
	case ConditionFalse:
		c.Status = fnv1.Status_STATUS_CONDITION_FALSE
	case ConditionUnknown:
		fallthrough
	default:
		c.Status = fnv1.Status_STATUS_CONDITION_UNKNOWN
	}

	c.Message = cs.Condition.Message

	return c
}

// transformTarget converts a Target to an fnv1.Target
func transformTarget(t *Target) *fnv1.Target {
	target := ptr.Deref(t, TargetComposite)
	if target == TargetCompositeAndClaim {
		return fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum()
	}
	return fnv1.Target_TARGET_COMPOSITE.Enum()
}

// SetConditions updates the RunFunctionResponse with specified conditions
func SetConditions(rsp *fnv1.RunFunctionResponse, cr ConditionResources, log logger.Logger) error {
	conditionsSet := map[string]bool{}

	// Set the desired conditions
	for _, cs := range cr {
		// Check if this is a system condition type
		if isSystemConditionType(cs.Condition.Type) {
			response.Fatal(rsp, errors.Errorf("cannot set condition type: %s is a reserved Crossplane Condition", cs.Condition.Type))
			return errors.New("error updating response")
		}

		if conditionsSet[cs.Condition.Type] && (cs.Force == nil || !*cs.Force) {
			// The condition is already set and this setter is not forceful
			log.Debug("skipping because condition is already set and setCondition is not forceful")
			continue
		}

		log.Debug("setting condition", "type", cs.Condition.Type, "status", cs.Condition.Status)

		c := transformCondition(cs)
		rsp.Conditions = append(rsp.Conditions, c)
		conditionsSet[cs.Condition.Type] = true
	}

	return nil
}

// isSystemConditionType checks if the condition type is a system condition
func isSystemConditionType(conditionType string) bool {
	systemTypes := map[string]bool{
		"Ready":   true,
		"Synced":  true,
		"Healthy": true,
	}

	return systemTypes[conditionType]
}
