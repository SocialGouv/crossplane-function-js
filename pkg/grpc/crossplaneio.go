package grpc

import (
	"encoding/json"
	"strings"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/socialgouv/xfuncjs-server/pkg/logger"
)

// NOTE: This file logs Crossplane RunFunction request/response payloads.
// It must always redact secrets to avoid leaking credentials in logs.

var sensitiveKeyFragments = []string{
	"password",
	"passwd",
	"token",
	"secret",
	"private",
	"key",
	"certificate",
}

// logCrossplaneRequest logs a redacted RunFunctionRequest at DEBUG level.
func logCrossplaneRequest(log logger.Logger, req *fnv1.RunFunctionRequest) {
	if req == nil {
		return
	}
	logCrossplaneProto(log, "crossplane.request", req)
}

// logCrossplaneResponse logs a redacted RunFunctionResponse at DEBUG level.
func logCrossplaneResponse(log logger.Logger, rsp *fnv1.RunFunctionResponse) {
	if rsp == nil {
		return
	}
	logCrossplaneProto(log, "crossplane.response", rsp)
}

func logCrossplaneProto(log logger.Logger, field string, msg proto.Message) {
	// Marshal with proto field names to keep stability across versions.
	b, err := protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal(msg)
	if err != nil {
		log.WithFields(map[string]interface{}{
			logger.FieldError: err.Error(),
		}).Debug("Failed to marshal Crossplane payload")
		return
	}

	var m interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		log.WithFields(map[string]interface{}{
			logger.FieldError: err.Error(),
		}).Debug("Failed to unmarshal Crossplane payload for redaction")
		return
	}

	redactInPlace(&m)

	out, err := json.Marshal(m)
	if err != nil {
		log.WithFields(map[string]interface{}{
			logger.FieldError: err.Error(),
		}).Debug("Failed to marshal redacted Crossplane payload")
		return
	}

	log.WithFields(map[string]interface{}{
		field: string(out),
	}).Debug("Crossplane I/O")
}

// redactInPlace recursively redacts common secret-bearing fields.
//
// The Crossplane RunFunctionRequest includes a credentials map. We redact it
// entirely. Connection details may appear in observed composite state; we redact
// any field named connectionDetails.
func redactInPlace(v *interface{}) {
	switch t := (*v).(type) {
	case map[string]interface{}:
		for k, child := range t {
			lk := strings.ToLower(k)

			// Crossplane explicit sensitive fields.
			if lk == "credentials" {
				t[k] = "REDACTED"
				continue
			}
			if lk == "connectiondetails" {
				t[k] = "REDACTED"
				continue
			}

			// Best-effort redaction for arbitrary payloads.
			if looksSensitiveKey(lk) {
				t[k] = "REDACTED"
				continue
			}

			// Recurse.
			cv := child
			redactInPlace(&cv)
			t[k] = cv
		}
	case []interface{}:
		for i := range t {
			cv := t[i]
			redactInPlace(&cv)
			t[i] = cv
		}
	default:
		// scalar; nothing to redact unless keyed above.
	}
}

func looksSensitiveKey(lowerKey string) bool {
	for _, frag := range sensitiveKeyFragments {
		if strings.Contains(lowerKey, frag) {
			return true
		}
	}
	return false
}

