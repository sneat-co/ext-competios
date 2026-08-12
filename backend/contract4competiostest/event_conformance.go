package contract4competiostest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type ExecutionEventSinkFactory func(contract4competios.ExecutionRequest, contract4competios.ExecutionReceipt) contract4competios.ExecutionEventSink

func CheckExecutionEventSink(factory ExecutionEventSinkFactory) []error {
	request := executionFixture()
	receipt := executionReceiptFixture(request, "instance")
	return CheckExecutionEventSinkWithEvents(factory, request, receipt, startFixture("instance"), resultFixture("instance"))
}

func CheckExecutionEventSinkWithEvents(factory ExecutionEventSinkFactory, request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt, start, result contract4competios.ExecutionEvent) []error {
	ctx := context.Background()
	var violations []error
	if err := contract4competios.ValidateExecutionEventForExecution(start, request, receipt); err != nil {
		violations = append(violations, fmt.Errorf("start fixture is not request-bound: %v", err))
	}
	if err := contract4competios.ValidateExecutionEventForExecution(result, request, receipt); err != nil {
		violations = append(violations, fmt.Errorf("result fixture is not request-bound: %v", err))
	}

	prematureSink := factory(request, receipt)
	if _, err := prematureSink.SubmitExecutionEvent(ctx, eventGrantFixture(result, "premature-result-token", "key-a"), result); !errors.Is(err, contract4competios.ErrInvalidTransition) {
		violations = append(violations, fmt.Errorf("result before start error = %v", err))
	}

	sink := factory(request, receipt)
	startGrant := eventGrantFixture(start, "start-token", "key-a")
	ack, err := sink.SubmitExecutionEvent(ctx, startGrant, start)
	if err != nil || contract4competios.ValidateEventAcknowledgement(ack) != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		return append(violations, fmt.Errorf("first start = %+v: %v", ack, err))
	}
	ack, err = sink.SubmitExecutionEvent(ctx, startGrant, start)
	if err != nil || ack.Status != contract4competios.EventAcknowledgementReplayed {
		violations = append(violations, fmt.Errorf("same-token start replay = %+v: %v", ack, err))
	}
	freshStartGrant := eventGrantFixture(start, "start-token-fresh", "key-rotated")
	ack, err = sink.SubmitExecutionEvent(ctx, freshStartGrant, start)
	if err != nil || ack.Status != contract4competios.EventAcknowledgementReplayed {
		violations = append(violations, fmt.Errorf("fresh-token start replay = %+v: %v", ack, err))
	}

	for name, mutate := range startEventPayloadMutations() {
		payload := copyEventPayload(start)
		mutate(&payload)
		changed, buildErr := contract4competios.NewExecutionEvent(payload)
		if buildErr != nil {
			violations = append(violations, fmt.Errorf("start %s mutation invalid: %v", name, buildErr))
			continue
		}
		if _, submitErr := sink.SubmitExecutionEvent(ctx, eventGrantFixture(changed, "changed-start-"+name, "key-a"), changed); !errors.Is(submitErr, contract4competios.ErrCommandConflict) {
			violations = append(violations, fmt.Errorf("changed start %s error = %v", name, submitErr))
		}
	}

	for name, mutate := range invalidEventGrantMutations() {
		bad := eventGrantFixture(start, "bad-event-"+name, "key-a")
		mutate(&bad.Claims)
		if _, submitErr := factory(request, receipt).SubmitExecutionEvent(ctx, bad, start); submitErr == nil {
			violations = append(violations, fmt.Errorf("%s event grant was accepted", name))
		}
	}

	resultGrant := eventGrantFixture(result, "result-token", "key-a")
	ack, err = sink.SubmitExecutionEvent(ctx, resultGrant, result)
	if err != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		violations = append(violations, fmt.Errorf("first result = %+v: %v", ack, err))
	}
	freshResultGrant := eventGrantFixture(result, "result-token-fresh", "key-rotated")
	ack, err = sink.SubmitExecutionEvent(ctx, freshResultGrant, result)
	if err != nil || ack.Status != contract4competios.EventAcknowledgementReplayed {
		violations = append(violations, fmt.Errorf("fresh-token result replay = %+v: %v", ack, err))
	}

	for name, mutate := range resultEventPayloadMutations() {
		payload := copyEventPayload(result)
		mutate(&payload)
		changed, buildErr := contract4competios.NewExecutionEvent(payload)
		if buildErr != nil {
			violations = append(violations, fmt.Errorf("result %s mutation invalid: %v", name, buildErr))
			continue
		}
		if _, submitErr := sink.SubmitExecutionEvent(ctx, eventGrantFixture(changed, "changed-result-"+name, "key-a"), changed); !errors.Is(submitErr, contract4competios.ErrCommandConflict) {
			violations = append(violations, fmt.Errorf("changed result %s error = %v", name, submitErr))
		}
	}

	for name, mutate := range map[string]func(*contract4competios.ExecutionEventPayload){
		"unknown frozen entry": func(value *contract4competios.ExecutionEventPayload) {
			value.Result.Placements[0].EntryID = "unknown-entry"
		},
		"wrong frozen artifact": func(value *contract4competios.ExecutionEventPayload) {
			value.Result.Evidence.RecordedProvenance.ParticipantArtifactDigests[0] = artifactDigest("9")
		},
	} {
		freshSink := factory(request, receipt)
		if _, submitErr := freshSink.SubmitExecutionEvent(ctx, eventGrantFixture(start, "bound-start-"+name, "key-a"), start); submitErr != nil {
			violations = append(violations, fmt.Errorf("%s setup start: %v", name, submitErr))
			continue
		}
		payload := copyEventPayload(result)
		payload.ID, payload.CommandID = contract4competios.EventID("bound-"+name), contract4competios.CommandID("bound-command-"+name)
		mutate(&payload)
		changed, buildErr := contract4competios.NewExecutionEvent(payload)
		if buildErr != nil {
			violations = append(violations, fmt.Errorf("%s fixture: %v", name, buildErr))
			continue
		}
		if _, submitErr := freshSink.SubmitExecutionEvent(ctx, eventGrantFixture(changed, "bound-token-"+name, "key-a"), changed); submitErr == nil {
			violations = append(violations, fmt.Errorf("%s was accepted", name))
		}
	}

	lateStartPayload := copyEventPayload(start)
	lateStartPayload.ID, lateStartPayload.CommandID = "late-start", "late-start-command"
	lateStart, buildErr := contract4competios.NewExecutionEvent(lateStartPayload)
	if buildErr != nil {
		violations = append(violations, buildErr)
	}
	for _, late := range []contract4competios.ExecutionEvent{
		failureFixture(result.ProviderInstanceID, contract4competios.LifecycleEventFailed, "late-failure"),
		failureFixture(result.ProviderInstanceID, contract4competios.LifecycleEventCancelled, "late-cancellation"),
		lateStart,
	} {
		if _, submitErr := sink.SubmitExecutionEvent(ctx, eventGrantFixture(late, "late-token-"+string(late.Kind), "key-a"), late); !errors.Is(submitErr, contract4competios.ErrInvalidTransition) {
			violations = append(violations, fmt.Errorf("post-terminal %s error = %v", late.Kind, submitErr))
		}
	}

	cancelSink := factory(request, receipt)
	cancel := failureFixture(start.ProviderInstanceID, contract4competios.LifecycleEventCancelled, "cancel-command")
	ack, err = cancelSink.SubmitExecutionEvent(ctx, eventGrantFixture(cancel, "cancel-token", "key-a"), cancel)
	if err != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		violations = append(violations, fmt.Errorf("pre-start cancellation = %+v: %v", ack, err))
	}
	if cancel.Failure == nil || cancel.Failure.Evidence != nil {
		violations = append(violations, errors.New("pre-start cancellation fabricated runtime evidence"))
	}
	if _, submitErr := cancelSink.SubmitExecutionEvent(ctx, eventGrantFixture(start, "after-cancel-token", "key-a"), start); !errors.Is(submitErr, contract4competios.ErrInvalidTransition) {
		violations = append(violations, fmt.Errorf("start after cancellation error = %v", submitErr))
	}

	failureSink := factory(request, receipt)
	failure := failureFixture(start.ProviderInstanceID, contract4competios.LifecycleEventFailed, "failure-command")
	ack, err = failureSink.SubmitExecutionEvent(ctx, eventGrantFixture(failure, "failure-token", "key-a"), failure)
	if err != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		violations = append(violations, fmt.Errorf("pre-start failure = %+v: %v", ack, err))
	}
	return violations
}

func startEventPayloadMutations() map[string]func(*contract4competios.ExecutionEventPayload) {
	return map[string]func(*contract4competios.ExecutionEventPayload){
		"id":          func(v *contract4competios.ExecutionEventPayload) { v.ID = "other-start" },
		"competition": func(v *contract4competios.ExecutionEventPayload) { v.CompetitionID = "other-cup" },
		"contest":     func(v *contract4competios.ExecutionEventPayload) { v.ContestID = "other-contest" },
		"request":     func(v *contract4competios.ExecutionEventPayload) { v.RequestID = "other-request" },
		"provider":    func(v *contract4competios.ExecutionEventPayload) { v.ProviderID = "other-provider" },
		"adapter":     func(v *contract4competios.ExecutionEventPayload) { v.AdapterID = "other-adapter" },
		"instance":    func(v *contract4competios.ExecutionEventPayload) { v.ProviderInstanceID = "other-instance" },
		"occurred at": func(v *contract4competios.ExecutionEventPayload) { v.OccurredAt = v.OccurredAt.Add(time.Second) },
		"kind and failure": func(v *contract4competios.ExecutionEventPayload) {
			v.Kind = contract4competios.LifecycleEventFailed
			v.Failure = &contract4competios.ExecutionFailure{Code: "failed"}
		},
	}
}

func resultEventPayloadMutations() map[string]func(*contract4competios.ExecutionEventPayload) {
	return map[string]func(*contract4competios.ExecutionEventPayload){
		"id":          func(v *contract4competios.ExecutionEventPayload) { v.ID = "other-result" },
		"competition": func(v *contract4competios.ExecutionEventPayload) { v.CompetitionID = "other-cup" },
		"contest":     func(v *contract4competios.ExecutionEventPayload) { v.ContestID = "other-contest" },
		"request":     func(v *contract4competios.ExecutionEventPayload) { v.RequestID = "other-request" },
		"provider":    func(v *contract4competios.ExecutionEventPayload) { v.ProviderID = "other-provider" },
		"adapter":     func(v *contract4competios.ExecutionEventPayload) { v.AdapterID = "other-adapter" },
		"instance":    func(v *contract4competios.ExecutionEventPayload) { v.ProviderInstanceID = "other-instance" },
		"occurred at": func(v *contract4competios.ExecutionEventPayload) { v.OccurredAt = v.OccurredAt.Add(time.Second) },
		"kind and failure": func(v *contract4competios.ExecutionEventPayload) {
			v.Kind, v.Result = contract4competios.LifecycleEventFailed, nil
			v.Failure = &contract4competios.ExecutionFailure{Code: "changed-failure"}
		},
		"placement slot": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Placements[0].SlotOrdinal, v.Result.Placements[1].SlotOrdinal = 1, 0
			v.Result.Placements[0], v.Result.Placements[1] = v.Result.Placements[1], v.Result.Placements[0]
		},
		"placement entry": func(v *contract4competios.ExecutionEventPayload) { v.Result.Placements[0].EntryID = "changed-entry" },
		"placement rank":  func(v *contract4competios.ExecutionEventPayload) { v.Result.Placements[1].Rank = 2 },
		"placement status": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Placements[0].Status = contract4competios.PlacementStatusForfeited
		},
		"replay": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.Replay = contract4competios.TerminalReplay{State: contract4competios.ReplayProcessing}
		},
		"participant artifacts": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.RecordedProvenance.ParticipantArtifactDigests[0] = artifactDigest("9")
		},
		"configuration digest": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.RecordedProvenance.ProviderConfigurationDigest = artifactDigest("9")
		},
		"runtime digest": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.RecordedProvenance.RuntimeDigest = artifactDigest("9")
		},
		"rules digest": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.RecordedProvenance.RulesDigest = artifactDigest("9")
		},
		"limit digest": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.RecordedProvenance.LimitProfileDigest = artifactDigest("9")
		},
		"seed digest": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.RecordedProvenance.SeedDigest = artifactDigest("9")
		},
		"event log digest": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.RecordedProvenance.EventLogDigest = artifactDigest("9")
		},
		"execution digest": func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.RecordedProvenance.ExecutionPayloadDigest = payloadDigest("9")
		},
	}
}

func invalidEventGrantMutations() map[string]func(*contract4competios.OperationGrant) {
	return map[string]func(*contract4competios.OperationGrant){
		"issuer":       func(v *contract4competios.OperationGrant) { v.Issuer = "other" },
		"subject":      func(v *contract4competios.OperationGrant) { v.Subject = "other" },
		"audience":     func(v *contract4competios.OperationGrant) { v.Audience = "other" },
		"token type":   func(v *contract4competios.OperationGrant) { v.TokenType = "other" },
		"scope":        func(v *contract4competios.OperationGrant) { v.Scope = "other" },
		"purpose":      func(v *contract4competios.OperationGrant) { v.Purpose = "other" },
		"provider":     func(v *contract4competios.OperationGrant) { v.ProviderID = "other" },
		"adapter":      func(v *contract4competios.OperationGrant) { v.AdapterID = "other" },
		"competition":  func(v *contract4competios.OperationGrant) { v.CompetitionID = "other" },
		"contest":      func(v *contract4competios.OperationGrant) { v.ContestID = "other" },
		"request":      func(v *contract4competios.OperationGrant) { v.RequestID = "other" },
		"instance":     func(v *contract4competios.OperationGrant) { v.ProviderInstanceID = "other" },
		"command":      func(v *contract4competios.OperationGrant) { v.CommandID = "other" },
		"typed digest": func(v *contract4competios.OperationGrant) { v.TypedPayloadDigest = payloadDigest("9") },
		"content type": func(v *contract4competios.OperationGrant) { v.TransportContentType = "application/json" },
		"raw digest":   func(v *contract4competios.OperationGrant) { v.RawTransportDigest = payloadDigest("8") },
		"method":       func(v *contract4competios.OperationGrant) { v.Method = "PUT" },
		"resource":     func(v *contract4competios.OperationGrant) { v.Resource = "/other" },
		"source field": func(v *contract4competios.OperationGrant) { v.ParticipantID = "forbidden" },
	}
}
