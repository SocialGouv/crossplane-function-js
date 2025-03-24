package types

// ResourceInfo holds information about a Crossplane resource
type ResourceInfo struct {
	Version   string
	Kind      string
	Name      string
	Namespace string
}

// ToFields converts ResourceInfo to a map of logger fields
func (r *ResourceInfo) ToFields() map[string]interface{} {
	fields := make(map[string]interface{})

	if r.Version != "" {
		fields["resource.version"] = r.Version
	}

	if r.Kind != "" {
		fields["resource.kind"] = r.Kind
	}

	if r.Name != "" {
		fields["resource.name"] = r.Name
	}

	if r.Namespace != "" {
		fields["resource.namespace"] = r.Namespace
	}

	return fields
}
