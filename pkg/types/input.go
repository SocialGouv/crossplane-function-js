package types

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// XFuncJSInput represents the input for the XFuncJS function
type XFuncJSInput struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		Source struct {
			Inline       string            `json:"inline,omitempty"`
			Dependencies map[string]string `json:"dependencies,omitempty"`
			YarnLock     string            `json:"yarnLock,omitempty"`
			TsConfig     string            `json:"tsConfig,omitempty"`
		} `json:"source"`
		Params map[string]interface{} `json:"params,omitempty"`
		Target string                 `json:"target,omitempty"`
	} `json:"spec"`

	// TypeMeta is required for runtime.Object implementation
	metav1.TypeMeta `json:",inline"`
}

// GetObjectKind implements the runtime.Object interface
func (i *XFuncJSInput) GetObjectKind() schema.ObjectKind {
	return &i.TypeMeta
}

// DeepCopyObject implements the runtime.Object interface
func (i *XFuncJSInput) DeepCopyObject() runtime.Object {
	if i == nil {
		return nil
	}
	copy := &XFuncJSInput{
		APIVersion: i.APIVersion,
		Kind:       i.Kind,
	}
	copy.Spec.Source.Inline = i.Spec.Source.Inline
	copy.Spec.Source.YarnLock = i.Spec.Source.YarnLock
	copy.Spec.Source.TsConfig = i.Spec.Source.TsConfig
	copy.Spec.Target = i.Spec.Target

	// Copy dependencies
	if i.Spec.Source.Dependencies != nil {
		copy.Spec.Source.Dependencies = make(map[string]string, len(i.Spec.Source.Dependencies))
		for k, v := range i.Spec.Source.Dependencies {
			copy.Spec.Source.Dependencies[k] = v
		}
	}

	// Copy params
	if i.Spec.Params != nil {
		copy.Spec.Params = make(map[string]interface{}, len(i.Spec.Params))
		for k, v := range i.Spec.Params {
			copy.Spec.Params[k] = v
		}
	}

	return copy
}

// Validate validates the input
func (i *XFuncJSInput) Validate() error {
	if i.Spec.Source.Inline == "" {
		return errors.New("source.inline is required")
	}
	return nil
}
