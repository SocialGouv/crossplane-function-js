package types

// ResourceInfo holds information about a Crossplane resource
type ResourceInfo struct {
	// XR-specific fields.
	XRAPIVersion string
	XRGroup      string
	XRVersion    string
	XRKind       string
	XRName       string
	XRNamespace  string
}

// ToFields converts ResourceInfo to a map of logger fields
func (r *ResourceInfo) ToFields() map[string]interface{} {
	fields := make(map[string]interface{})

	// XR fields (explicitly required for XR-related logs).
	if r.XRAPIVersion != "" {
		fields["xr.apiVersion"] = r.XRAPIVersion
	}
	if r.XRGroup != "" {
		fields["xr.group"] = r.XRGroup
	}
	if r.XRVersion != "" {
		fields["xr.version"] = r.XRVersion
	}
	if r.XRKind != "" {
		fields["xr.kind"] = r.XRKind
	}
	if r.XRName != "" {
		fields["xr.name"] = r.XRName
	}
	if r.XRNamespace != "" {
		fields["xr.namespace"] = r.XRNamespace
	}

	return fields
}
