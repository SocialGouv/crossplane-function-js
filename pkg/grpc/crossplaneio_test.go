package grpc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactInPlace(t *testing.T) {
	in := map[string]interface{}{
		"credentials": map[string]interface{}{"foo": "bar"},
		"observed": map[string]interface{}{
			"composite": map[string]interface{}{
				"connectionDetails": map[string]interface{}{"password": "s3cr3t"},
			},
		},
		"spec": map[string]interface{}{
			"token": "abcd",
			"nested": map[string]interface{}{
				"privateKey": "pem",
			},
		},
	}

	var v interface{} = in
	redactInPlace(&v)

	outBytes, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(outBytes)

	for _, wantAbsent := range []string{"s3cr3t", "abcd", "pem", "bar"} {
		if strings.Contains(out, wantAbsent) {
			t.Fatalf("expected %q to be redacted, but found in output: %s", wantAbsent, out)
		}
	}

	for _, wantPresent := range []string{"REDACTED", "credentials", "connectionDetails"} {
		if !strings.Contains(out, wantPresent) {
			t.Fatalf("expected %q to be present in output: %s", wantPresent, out)
		}
	}
}

