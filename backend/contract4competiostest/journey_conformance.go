package contract4competiostest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

const privateDiagnosticCanary = "private-repo-canary-9"

type DiagnosticObserver interface {
	ObservedDiagnostics() []string
}

// ExecutionJourneyProvider is a provider-side conformance adapter used to
// prove one launch receipt flows into the exact started and terminal events.
// It is deliberately outside the production port: transports may deliver
// events asynchronously while still running this same journey suite.
type ExecutionJourneyProvider interface {
	contract4competios.ExecutionProvider
	LifecycleEvents(context.Context, contract4competios.ExecutionRequest, contract4competios.ExecutionReceipt) (contract4competios.ExecutionEvent, contract4competios.ExecutionEvent, error)
}

type ExecutionJourneyProviderFactory func() ExecutionJourneyProvider

// CheckExecutionJourney proves launch -> started -> completed as one composed
// flow through the provider and consumer conformance ports.
func CheckExecutionJourney(factory ExecutionJourneyProviderFactory, request contract4competios.ExecutionRequest, inspectTerminal func(contract4competios.ExecutionEvent) error) []error {
	ctx := context.Background()
	provider := factory()
	receipt, err := provider.LaunchExecution(ctx, launchGrantFixture(request, "journey-launch-token", "key-a"), request)
	if err != nil {
		return []error{fmt.Errorf("journey launch: %w", err)}
	}
	var violations []error
	if validationErr := contract4competios.ValidateExecutionReceiptForRequest(receipt, request); validationErr != nil || receipt.Status != contract4competios.ReceiptAccepted {
		return append(violations, fmt.Errorf("journey receipt = %+v: %v", receipt, validationErr))
	}
	violations = append(violations, privacyViolations("request", request)...)
	violations = append(violations, privacyViolations("receipt", receipt)...)
	started, terminal, err := provider.LifecycleEvents(ctx, request, receipt)
	if err != nil {
		return append(violations, fmt.Errorf("journey lifecycle: %w", err))
	}
	if started.Kind != contract4competios.LifecycleEventStarted || terminal.Kind != contract4competios.LifecycleEventCompleted {
		violations = append(violations, fmt.Errorf("journey event kinds = %q -> %q", started.Kind, terminal.Kind))
	}
	if validationErr := contract4competios.ValidateExecutionEventForExecution(started, request, receipt); validationErr != nil {
		violations = append(violations, fmt.Errorf("journey start binding: %v", validationErr))
	}
	if validationErr := contract4competios.ValidateExecutionEventForExecution(terminal, request, receipt); validationErr != nil {
		violations = append(violations, fmt.Errorf("journey terminal binding: %v", validationErr))
	}
	if inspectTerminal != nil {
		if inspectionErr := inspectTerminal(terminal); inspectionErr != nil {
			violations = append(violations, inspectionErr)
		}
	}
	violations = append(violations, privacyViolations("started event", started)...)
	violations = append(violations, privacyViolations("terminal event", terminal)...)

	sink := newJourneyEventSink(request, receipt)
	ack, submitErr := sink.SubmitExecutionEvent(ctx, eventGrantFixture(started, "journey-start-token", "key-a"), started)
	if submitErr != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		violations = append(violations, fmt.Errorf("journey start acknowledgement = %+v: %v", ack, submitErr))
		return violations
	}
	ack, submitErr = sink.SubmitExecutionEvent(ctx, eventGrantFixture(terminal, "journey-terminal-token", "key-rotated"), terminal)
	if submitErr != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		violations = append(violations, fmt.Errorf("journey terminal acknowledgement = %+v: %v", ack, submitErr))
	}

	badGrant := launchGrantFixture(request, "journey-rejection-token", "key-a")
	badGrant.Claims.RawTransportDigest = payloadDigest("9")
	_, rejectionErr := provider.LaunchExecution(ctx, badGrant, request)
	if rejectionErr == nil {
		violations = append(violations, fmt.Errorf("journey provider accepted an invalid rejection probe"))
	} else if strings.Contains(rejectionErr.Error(), privateDiagnosticCanary) {
		violations = append(violations, fmt.Errorf("journey rejection error leaked private diagnostic data"))
	}
	if observer, ok := provider.(DiagnosticObserver); ok {
		for _, diagnostic := range observer.ObservedDiagnostics() {
			if strings.Contains(diagnostic, privateDiagnosticCanary) {
				violations = append(violations, fmt.Errorf("journey diagnostic log leaked private data"))
			}
		}
	}
	return violations
}

type journeyStoredEvent struct {
	digest contract4competios.PayloadDigest
}

type journeyEventSink struct {
	state   contract4competios.ExecutionState
	request contract4competios.ExecutionRequest
	receipt contract4competios.ExecutionReceipt
	events  map[contract4competios.CommandID]journeyStoredEvent
}

func newJourneyEventSink(request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) *journeyEventSink {
	return &journeyEventSink{
		state: contract4competios.ExecutionStateAccepted, request: request, receipt: receipt,
		events: map[contract4competios.CommandID]journeyStoredEvent{},
	}
}

func (s *journeyEventSink) SubmitExecutionEvent(_ context.Context, grant contract4competios.VerifiedOperationGrant, event contract4competios.ExecutionEvent) (contract4competios.EventAcknowledgement, error) {
	if contract4competios.ValidateEventGrantForEvent(grant, eventRouteFixture(event), event) != nil || contract4competios.ValidateExecutionEventForExecution(event, s.request, s.receipt) != nil {
		return contract4competios.EventAcknowledgement{}, contract4competios.ErrInvalidGrant
	}
	if prior, exists := s.events[event.CommandID]; exists {
		if prior.digest != event.TypedPayloadDigest {
			return contract4competios.EventAcknowledgement{}, contract4competios.ErrCommandConflict
		}
		return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementReplayed}, nil
	}
	if err := contract4competios.ValidateLifecycleTransition(s.state, event.Kind); err != nil {
		return contract4competios.EventAcknowledgement{}, err
	}
	s.events[event.CommandID] = journeyStoredEvent{digest: event.TypedPayloadDigest}
	s.state = journeyStateForEvent(event.Kind)
	return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementAccepted}, nil
}

func journeyStateForEvent(kind contract4competios.LifecycleEventKind) contract4competios.ExecutionState {
	switch kind {
	case contract4competios.LifecycleEventStarted:
		return contract4competios.ExecutionStateStarted
	case contract4competios.LifecycleEventCompleted:
		return contract4competios.ExecutionStateCompleted
	case contract4competios.LifecycleEventFailed:
		return contract4competios.ExecutionStateFailed
	case contract4competios.LifecycleEventCancelled:
		return contract4competios.ExecutionStateCancelled
	default:
		return ""
	}
}

func privacyViolations(label string, value any) []error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []error{fmt.Errorf("%s privacy serialization: %v", label, err)}
	}
	if strings.Contains(string(encoded), privateDiagnosticCanary) {
		return []error{fmt.Errorf("%s leaked private canary data", label)}
	}
	return nil
}
