package contract4competios

import (
	"bytes"
	"errors"
	"testing"
)

func validExecutionLaunchEnvelope() ExecutionLaunchEnvelope {
	return ExecutionLaunchEnvelope{
		AccessToken: "opaque-token",
		Method:      "POST",
		Resource:    "/v1/executions",
		ContentType: "application/json",
		RawBody:     []byte(`{"request":"one"}`),
	}
}

func TestExecutionLaunchEnvelopeClonesRawBody(t *testing.T) {
	original := validExecutionLaunchEnvelope()
	clone := CloneExecutionLaunchEnvelope(original)
	clone.RawBody[0] = '['
	if bytes.Equal(original.RawBody, clone.RawBody) {
		t.Fatal("clone aliases the authority-bearing raw body")
	}
}

func TestValidateExecutionLaunchEnvelope(t *testing.T) {
	valid := validExecutionLaunchEnvelope()
	if err := ValidateExecutionLaunchEnvelope(valid); err != nil {
		t.Fatalf("valid envelope: %v", err)
	}

	tests := map[string]func(*ExecutionLaunchEnvelope){
		"empty token":       func(v *ExecutionLaunchEnvelope) { v.AccessToken = "" },
		"method whitespace": func(v *ExecutionLaunchEnvelope) { v.Method = "CUSTOM METHOD" },
		"method newline":    func(v *ExecutionLaunchEnvelope) { v.Method = "POST\n" },
		"route newline":     func(v *ExecutionLaunchEnvelope) { v.Resource += "\nsecret" },
		"content newline":   func(v *ExecutionLaunchEnvelope) { v.ContentType += "\rsecret" },
		"empty body":        func(v *ExecutionLaunchEnvelope) { v.RawBody = nil },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := CloneExecutionLaunchEnvelope(valid)
			mutate(&candidate)
			if err := ValidateExecutionLaunchEnvelope(candidate); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("got %v, want invalid grant", err)
			}
		})
	}

	extension := CloneExecutionLaunchEnvelope(valid)
	extension.Method = "LAUNCH-BOT"
	extension.Resource = "urn:competios:launch"
	if err := ValidateExecutionLaunchEnvelope(extension); err != nil {
		t.Fatalf("opaque extension method/resource rejected: %v", err)
	}

	large := CloneExecutionLaunchEnvelope(valid)
	large.RawBody = make([]byte, 300*1024)
	if err := ValidateExecutionLaunchEnvelope(large); err != nil {
		t.Fatalf("contract envelope imposed a deployment body cap: %v", err)
	}

	opaque := CloneExecutionLaunchEnvelope(valid)
	opaque.AccessToken = "not-a-token-the-contract-can-parse"
	opaque.RawBody = []byte("not json")
	if err := ValidateExecutionLaunchEnvelope(opaque); err != nil {
		t.Fatalf("contract envelope decoded opaque token or body: %v", err)
	}
}

func TestExecutionLaunchEnvelopeDigestBindsExactBodyAndContentType(t *testing.T) {
	value := validExecutionLaunchEnvelope()
	baseline := DigestRawTransportBody(value.ContentType, value.RawBody)

	changedBody := CloneExecutionLaunchEnvelope(value)
	changedBody.RawBody[len(changedBody.RawBody)-2] = '2'
	if got := DigestRawTransportBody(changedBody.ContentType, changedBody.RawBody); got == baseline {
		t.Fatal("one-byte body mutation retained raw transport digest")
	}
	if got := DigestRawTransportBody("application/problem+json", value.RawBody); got == baseline {
		t.Fatal("content-type mutation retained raw transport digest")
	}
}
