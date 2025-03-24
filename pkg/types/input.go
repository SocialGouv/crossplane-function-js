package types

import "errors"

// SkyhookInput represents the input for the Skyhook function
type SkyhookInput struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		Source struct {
			Inline       string            `json:"inline,omitempty"`
			Dependencies map[string]string `json:"dependencies,omitempty"`
			YarnLock     string            `json:"yarnLock,omitempty"`
		} `json:"source"`
		Params map[string]interface{} `json:"params,omitempty"`
		Target string                 `json:"target,omitempty"`
	} `json:"spec"`
}

// Validate validates the input
func (i *SkyhookInput) Validate() error {
	if i.Spec.Source.Inline == "" {
		return errors.New("source.inline is required")
	}
	return nil
}
