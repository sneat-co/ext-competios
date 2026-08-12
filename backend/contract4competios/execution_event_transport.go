package contract4competios

import "context"

// ExecutionEventEnvelope is the exact authenticated callback transport an
// event producer issues for one durable execution event. The opaque token and
// raw transport facts are transient authority: consumers forward them
// byte-for-byte and do not decode, reserialize, or replace them with verified
// claims.
type ExecutionEventEnvelope struct {
	AccessToken EncodedAccessToken `json:"-"`
	Method      string             `json:"-"`
	Resource    string             `json:"-"`
	ContentType string             `json:"-"`
	RawBody     []byte             `json:"-"`
}

// CloneExecutionEventEnvelope copies the mutable raw body so the issuing event
// producer and receiving consumer cannot accidentally share authority-bearing
// bytes.
func CloneExecutionEventEnvelope(value ExecutionEventEnvelope) ExecutionEventEnvelope {
	value.RawBody = append([]byte(nil), value.RawBody...)
	return value
}

// ValidateExecutionEventEnvelope checks only the transient transport shape.
// It deliberately cannot validate the opaque token, decode the event, or
// impose a deployment-specific raw-body limit after allocation.
func ValidateExecutionEventEnvelope(value ExecutionEventEnvelope) error {
	if value.AccessToken == "" {
		return ErrInvalidGrant
	}
	return validateOpaqueTransportShape(value.Method, value.Resource, value.ContentType, value.RawBody)
}

// ExecutionEventEnvelopeSink is the production callback receiver port. It
// accepts only opaque, exact transport facts; verified claims remain private
// to the receiving service's authentication ingress.
type ExecutionEventEnvelopeSink interface {
	SubmitExecutionEventEnvelope(context.Context, ExecutionEventEnvelope) (EventAcknowledgement, error)
}

// NewExecutionEventGrantRequest constructs the one operation-grant request
// that can authorize an event's exact typed identity and exact raw transport.
// Started and terminal purpose selection is contract-owned so a game cannot
// accidentally issue a result token for a start (or the reverse).
func NewExecutionEventGrantRequest(event ExecutionEvent, method, resource, contentType string, rawBody []byte) (OperationGrantRequest, error) {
	if ValidateExecutionEvent(event) != nil || validateOpaqueTransportShape(method, resource, contentType, rawBody) != nil {
		return OperationGrantRequest{}, ErrInvalidGrant
	}
	purpose, scope := eventGrantPurpose(event.Kind)
	if purpose == "" || scope == "" {
		return OperationGrantRequest{}, ErrInvalidGrant
	}
	request := OperationGrantRequest{
		Purpose: purpose, ProviderID: event.ProviderID, AdapterID: event.AdapterID,
		CompetitionID: event.CompetitionID, ContestID: event.ContestID,
		RequestID: event.RequestID, ProviderInstanceID: event.ProviderInstanceID,
		CommandID: event.CommandID, TypedPayloadDigest: event.TypedPayloadDigest,
		TransportContentType: contentType,
		RawTransportDigest:   DigestRawTransportBody(contentType, rawBody),
		Method:               method, Resource: resource,
	}
	if ValidateOperationGrantRequest(request) != nil || GrantScopeForPurpose(request.Purpose) != scope {
		return OperationGrantRequest{}, ErrInvalidGrant
	}
	return request, nil
}
