package contract4competios

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func executionEventEnvelopeFixture() ExecutionEventEnvelope {
	return ExecutionEventEnvelope{
		AccessToken: "opaque-event-token",
		Method:      "POST",
		Resource:    "/v1/execution-events",
		ContentType: "application/vnd.competios.execution-event+json",
		RawBody:     []byte(`{"event":"one"}`),
	}
}

func TestExecutionEventEnvelopeIsTransientCloneableTransport(t *testing.T) {
	original := executionEventEnvelopeFixture()
	clone := CloneExecutionEventEnvelope(original)
	clone.RawBody[0] = '['
	if bytes.Equal(original.RawBody, clone.RawBody) {
		t.Fatal("clone aliases the authority-bearing raw event body")
	}
	encoded, err := json.Marshal(original)
	if err != nil || string(encoded) != "{}" {
		t.Fatalf("transient event envelope JSON = %s, %v", encoded, err)
	}
}

func TestValidateExecutionEventEnvelopeChecksShapeWithoutPolicyOrBodyCap(t *testing.T) {
	valid := executionEventEnvelopeFixture()
	if err := ValidateExecutionEventEnvelope(valid); err != nil {
		t.Fatalf("valid envelope: %v", err)
	}
	for name, mutate := range map[string]func(*ExecutionEventEnvelope){
		"empty token":       func(v *ExecutionEventEnvelope) { v.AccessToken = "" },
		"method whitespace": func(v *ExecutionEventEnvelope) { v.Method = "CUSTOM METHOD" },
		"method newline":    func(v *ExecutionEventEnvelope) { v.Method += "\n" },
		"route newline":     func(v *ExecutionEventEnvelope) { v.Resource += "\nsecret" },
		"content newline":   func(v *ExecutionEventEnvelope) { v.ContentType += "\rsecret" },
		"empty body":        func(v *ExecutionEventEnvelope) { v.RawBody = nil },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := CloneExecutionEventEnvelope(valid)
			mutate(&candidate)
			if err := ValidateExecutionEventEnvelope(candidate); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("got %v, want invalid grant", err)
			}
		})
	}
	extension := CloneExecutionEventEnvelope(valid)
	extension.Method, extension.Resource = "PUBLISH-EVENT", "urn:competios:event"
	extension.RawBody = make([]byte, 300*1024)
	if err := ValidateExecutionEventEnvelope(extension); err != nil {
		t.Fatalf("transport-neutral event envelope rejected extension/cap: %v", err)
	}
	extension.AccessToken, extension.RawBody = "not-a-token-the-contract-can-parse", []byte("not json")
	if err := ValidateExecutionEventEnvelope(extension); err != nil {
		t.Fatalf("contract parsed opaque token/body: %v", err)
	}
}

func executionEventForGrantRequestTest(t *testing.T, kind LifecycleEventKind) ExecutionEvent {
	t.Helper()
	payload := completedEventPayloadFixture()
	payload.Kind = kind
	switch kind {
	case LifecycleEventStarted:
		payload.Result = nil
	case LifecycleEventFailed, LifecycleEventCancelled:
		payload.Result = nil
		payload.Failure = &ExecutionFailure{Code: "provider-stopped"}
	}
	event, err := NewExecutionEvent(payload)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestNewExecutionEventGrantRequestBindsStartedAndEveryTerminalKind(t *testing.T) {
	for _, kind := range []LifecycleEventKind{LifecycleEventStarted, LifecycleEventCompleted, LifecycleEventFailed, LifecycleEventCancelled} {
		t.Run(string(kind), func(t *testing.T) {
			event := executionEventForGrantRequestTest(t, kind)
			body, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			request, err := NewExecutionEventGrantRequest(event, "POST", "/v1/execution-events", "application/vnd.competios.execution-event+json", body)
			if err != nil {
				t.Fatal(err)
			}
			wantPurpose := GrantPurposeContestResultSubmit
			if kind == LifecycleEventStarted {
				wantPurpose = GrantPurposeContestStarted
			}
			want := OperationGrantRequest{
				Purpose: wantPurpose, ProviderID: event.ProviderID, AdapterID: event.AdapterID,
				CompetitionID: event.CompetitionID, ContestID: event.ContestID,
				RequestID: event.RequestID, ProviderInstanceID: event.ProviderInstanceID,
				CommandID: event.CommandID, TypedPayloadDigest: event.TypedPayloadDigest,
				TransportContentType: "application/vnd.competios.execution-event+json",
				RawTransportDigest:   DigestRawTransportBody("application/vnd.competios.execution-event+json", body),
				Method:               "POST", Resource: "/v1/execution-events",
			}
			if !reflect.DeepEqual(request, want) || ValidateOperationGrantRequest(request) != nil {
				t.Fatalf("grant request = %+v, want %+v", request, want)
			}

			changedBody := append(append([]byte(nil), body...), ' ')
			bodyRequest, err := NewExecutionEventGrantRequest(event, "POST", "/v1/execution-events", "application/vnd.competios.execution-event+json", changedBody)
			if err != nil || bodyRequest.RawTransportDigest == request.RawTransportDigest || bodyRequest.TypedPayloadDigest != request.TypedPayloadDigest {
				t.Fatalf("body mutation binding = %+v, %v", bodyRequest, err)
			}
			contentRequest, err := NewExecutionEventGrantRequest(event, "POST", "/v1/execution-events", "application/json", body)
			if err != nil || contentRequest.TransportContentType == request.TransportContentType || contentRequest.RawTransportDigest == request.RawTransportDigest {
				t.Fatalf("content mutation binding = %+v, %v", contentRequest, err)
			}
			routeRequest, err := NewExecutionEventGrantRequest(event, "SEND-EVENT", "urn:competios:event", "application/vnd.competios.execution-event+json", body)
			if err != nil || routeRequest.Method != "SEND-EVENT" || routeRequest.Resource != "urn:competios:event" || routeRequest.RawTransportDigest != request.RawTransportDigest {
				t.Fatalf("route mutation binding = %+v, %v", routeRequest, err)
			}
		})
	}
}

func TestNewExecutionEventGrantRequestRejectsInvalidEventAndTransportShape(t *testing.T) {
	event := executionEventForGrantRequestTest(t, LifecycleEventStarted)
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	invalidEvent := event
	invalidEvent.TypedPayloadDigest = testPayloadDigest("9")
	for name, invoke := range map[string]func() error{
		"invalid event": func() error {
			_, callErr := NewExecutionEventGrantRequest(invalidEvent, "POST", "/events", "application/json", body)
			return callErr
		},
		"empty method": func() error {
			_, callErr := NewExecutionEventGrantRequest(event, "", "/events", "application/json", body)
			return callErr
		},
		"invalid method": func() error {
			_, callErr := NewExecutionEventGrantRequest(event, "POST GET", "/events", "application/json", body)
			return callErr
		},
		"invalid resource": func() error {
			_, callErr := NewExecutionEventGrantRequest(event, "POST", "/events\nother", "application/json", body)
			return callErr
		},
		"invalid content type": func() error {
			_, callErr := NewExecutionEventGrantRequest(event, "POST", "/events", "application/json\r", body)
			return callErr
		},
		"empty body": func() error {
			_, callErr := NewExecutionEventGrantRequest(event, "POST", "/events", "application/json", nil)
			return callErr
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := invoke(); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("got %v, want invalid grant", err)
			}
		})
	}
}
