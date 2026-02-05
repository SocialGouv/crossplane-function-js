package logger

import (
	"testing"

	fntypes "github.com/crossplane/function-sdk-go/proto/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestParseAPIVersion(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name       string
		apiVersion string
		wantGroup  string
		wantVer    string
	}{
		{name: "grouped", apiVersion: "test.crossplane.io/v1beta1", wantGroup: "test.crossplane.io", wantVer: "v1beta1"},
		{name: "core", apiVersion: "v1", wantGroup: "", wantVer: "v1"},
		{name: "empty", apiVersion: "", wantGroup: "", wantVer: ""},
		{name: "trim", apiVersion: "  acme.io/v1 ", wantGroup: "acme.io", wantVer: "v1"},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, v := parseAPIVersion(tc.apiVersion)
			if g != tc.wantGroup || v != tc.wantVer {
				t.Fatalf("parseAPIVersion(%q) = (%q,%q), want (%q,%q)", tc.apiVersion, g, v, tc.wantGroup, tc.wantVer)
			}
		})
	}
}

func TestExtractResourceInfoFromProto_ObservedComposite(t *testing.T) {
	t.Parallel()

	// Minimal RunFunctionRequest with observed.composite.resource = {apiVersion, kind, metadata{name,namespace}}
	res, err := structpb.NewStruct(map[string]interface{}{
		"apiVersion": "test.crossplane.io/v1beta1",
		"kind":       "SimpleConfigMap",
		"metadata": map[string]interface{}{
			"name":      "sample-configmap",
			"namespace": "test-xfuncjs",
		},
	})
	if err != nil {
		t.Fatalf("failed to build struct: %v", err)
	}

	req := &fntypes.RunFunctionRequest{
		Observed: &fntypes.State{
			Composite: &fntypes.Resource{Resource: res},
		},
	}

	ri := extractResourceInfoFromProto(req)
	if ri == nil {
		t.Fatalf("expected resource info, got nil")
	}

	// Required XR fields
	if ri.XRAPIVersion != "test.crossplane.io/v1beta1" {
		t.Fatalf("XRAPIVersion = %q", ri.XRAPIVersion)
	}
	if ri.XRGroup != "test.crossplane.io" {
		t.Fatalf("XRGroup = %q", ri.XRGroup)
	}
	if ri.XRVersion != "v1beta1" {
		t.Fatalf("XRVersion = %q", ri.XRVersion)
	}
	if ri.XRKind != "SimpleConfigMap" {
		t.Fatalf("XRKind = %q", ri.XRKind)
	}
	if ri.XRName != "sample-configmap" {
		t.Fatalf("XRName = %q", ri.XRName)
	}
	if ri.XRNamespace != "test-xfuncjs" {
		t.Fatalf("XRNamespace = %q", ri.XRNamespace)
	}
}

