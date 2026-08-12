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
	if ack, err := prematureSink.SubmitExecutionEvent(ctx, eventGrantFixture(start, "post-rejection-start-token", "key-a"), start); err != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		violations = append(violations, fmt.Errorf("valid start after rejected premature result = %+v: %v", ack, err))
	}
	if ack, err := prematureSink.SubmitExecutionEvent(ctx, eventGrantFixture(result, "post-rejection-result-token", "key-a"), result); err != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		violations = append(violations, fmt.Errorf("valid result after rejected premature result = %+v: %v", ack, err))
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
		mutationSink := factory(request, receipt)
		baselineAck, baselineErr := mutationSink.SubmitExecutionEvent(ctx, eventGrantFixture(start, "start-mutation-baseline-"+name, "key-a"), start)
		if baselineErr != nil || baselineAck.Status != contract4competios.EventAcknowledgementAccepted {
			violations = append(violations, fmt.Errorf("start %s baseline = %+v: %v", name, baselineAck, baselineErr))
			continue
		}
		payload := copyEventPayload(start)
		mutate(&payload)
		changed, buildErr := contract4competios.NewExecutionEvent(payload)
		if buildErr != nil {
			violations = append(violations, fmt.Errorf("start %s mutation invalid: %v", name, buildErr))
			continue
		}
		_, submitErr := mutationSink.SubmitExecutionEvent(ctx, eventGrantFixture(changed, "changed-start-"+name, "key-a"), changed)
		want := contract4competios.ErrCommandConflict
		if eventIdentityMutation(name) {
			want = contract4competios.ErrInvalidGrant
		}
		if !errors.Is(submitErr, want) {
			violations = append(violations, fmt.Errorf("changed start %s error = %v, want %v", name, submitErr, want))
		}
		replayed, replayErr := mutationSink.SubmitExecutionEvent(ctx, eventGrantFixture(start, "start-mutation-replay-"+name, "key-rotated"), start)
		if replayErr != nil || replayed.Status != contract4competios.EventAcknowledgementReplayed {
			violations = append(violations, fmt.Errorf("start %s exact replay after rejection = %+v: %v", name, replayed, replayErr))
		}
		continued, continueErr := mutationSink.SubmitExecutionEvent(ctx, eventGrantFixture(result, "start-mutation-result-"+name, "key-a"), result)
		if continueErr != nil || continued.Status != contract4competios.EventAcknowledgementAccepted {
			violations = append(violations, fmt.Errorf("start %s journey after rejection = %+v: %v", name, continued, continueErr))
		}
	}

	for name, mutate := range invalidEventGrantMutations() {
		bad := eventGrantFixture(start, "bad-event-"+name, "key-a")
		mutate(&bad.Claims)
		rejectionSink := factory(request, receipt)
		if _, submitErr := rejectionSink.SubmitExecutionEvent(ctx, bad, start); submitErr == nil {
			violations = append(violations, fmt.Errorf("%s event grant was accepted", name))
		}
		if ack, submitErr := rejectionSink.SubmitExecutionEvent(ctx, eventGrantFixture(start, "valid-after-bad-"+name, "key-a"), start); submitErr != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
			violations = append(violations, fmt.Errorf("valid start after rejected %s grant = %+v: %v", name, ack, submitErr))
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

	for name, mutate := range resultEventPayloadMutations(request) {
		mutationSink := factory(request, receipt)
		if baselineAck, baselineErr := mutationSink.SubmitExecutionEvent(ctx, eventGrantFixture(start, "result-mutation-start-"+name, "key-a"), start); baselineErr != nil || baselineAck.Status != contract4competios.EventAcknowledgementAccepted {
			violations = append(violations, fmt.Errorf("result %s start baseline = %+v: %v", name, baselineAck, baselineErr))
			continue
		}
		baselineAck, baselineErr := mutationSink.SubmitExecutionEvent(ctx, eventGrantFixture(result, "result-mutation-baseline-"+name, "key-a"), result)
		if baselineErr != nil || baselineAck.Status != contract4competios.EventAcknowledgementAccepted {
			violations = append(violations, fmt.Errorf("result %s baseline = %+v: %v", name, baselineAck, baselineErr))
			continue
		}
		payload := copyEventPayload(result)
		mutate(&payload)
		changed, buildErr := contract4competios.NewExecutionEvent(payload)
		if buildErr != nil {
			violations = append(violations, fmt.Errorf("result %s mutation invalid: %v", name, buildErr))
			continue
		}
		_, submitErr := mutationSink.SubmitExecutionEvent(ctx, eventGrantFixture(changed, "changed-result-"+name, "key-a"), changed)
		want := contract4competios.ErrCommandConflict
		if eventIdentityMutation(name) || resultRequestBindingMutation(name) {
			want = contract4competios.ErrInvalidGrant
		}
		if !errors.Is(submitErr, want) {
			violations = append(violations, fmt.Errorf("changed result %s error = %v, want %v", name, submitErr, want))
		}
		replayed, replayErr := mutationSink.SubmitExecutionEvent(ctx, eventGrantFixture(result, "result-mutation-replay-"+name, "key-rotated"), result)
		if replayErr != nil || replayed.Status != contract4competios.EventAcknowledgementReplayed {
			violations = append(violations, fmt.Errorf("result %s exact replay after rejection = %+v: %v", name, replayed, replayErr))
		}
	}

	boundMutations := map[string]func(*contract4competios.ExecutionEventPayload){
		"unknown frozen entry": func(value *contract4competios.ExecutionEventPayload) {
			value.Result.Placements[0].EntryID = "unknown-entry"
		},
	}
	if request.Profile.Kind == contract4competios.ExecutionProfileProviderExecuted {
		boundMutations["wrong frozen artifact"] = func(value *contract4competios.ExecutionEventPayload) {
			value.Result.Evidence.ProviderExecuted.ParticipantArtifactDigests[0] = artifactDigest("9")
		}
	} else {
		boundMutations["fabricated provider provenance"] = func(value *contract4competios.ExecutionEventPayload) {
			providerResult := resultFixtureForRequest(executionFixture(), value.ProviderInstanceID, []uint16{1, 1})
			value.Result.Evidence = providerResult.Result.Evidence
		}
	}
	for name, mutate := range boundMutations {
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
		if ack, submitErr := freshSink.SubmitExecutionEvent(ctx, eventGrantFixture(result, "valid-result-after-"+name, "key-a"), result); submitErr != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
			violations = append(violations, fmt.Errorf("valid result after rejected %s = %+v: %v", name, ack, submitErr))
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

func resultEventPayloadMutations(request contract4competios.ExecutionRequest) map[string]func(*contract4competios.ExecutionEventPayload) {
	mutations := map[string]func(*contract4competios.ExecutionEventPayload){
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
	}
	if request.Profile.Kind == contract4competios.ExecutionProfileProviderExecuted {
		mutations["participant artifacts"] = func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.ProviderExecuted.ParticipantArtifactDigests[0] = artifactDigest("9")
		}
		mutations["configuration digest"] = func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.ProviderExecuted.ProviderConfigurationDigest = artifactDigest("9")
		}
		mutations["runtime digest"] = func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.ProviderExecuted.RuntimeDigest = artifactDigest("9")
		}
		mutations["rules digest"] = func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.ProviderExecuted.RulesDigest = artifactDigest("9")
		}
		mutations["limit digest"] = func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.ProviderExecuted.LimitProfileDigest = artifactDigest("9")
		}
		mutations["seed digest"] = func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.ProviderExecuted.SeedDigest = artifactDigest("9")
		}
		mutations["event log digest"] = func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.ProviderExecuted.EventLogDigest = artifactDigest("9")
		}
		mutations["execution digest"] = func(v *contract4competios.ExecutionEventPayload) {
			v.Result.Evidence.ProviderExecuted.ExecutionPayloadDigest = payloadDigest("9")
		}
	} else {
		mutations["profile evidence"] = func(v *contract4competios.ExecutionEventPayload) {
			providerResult := resultFixtureForRequest(executionFixture(), v.ProviderInstanceID, []uint16{1, 1})
			v.Result.Evidence = providerResult.Result.Evidence
		}
	}
	return mutations
}

func eventIdentityMutation(name string) bool {
	switch name {
	case "competition", "contest", "request", "provider", "adapter", "instance":
		return true
	default:
		return false
	}
}

func resultRequestBindingMutation(name string) bool {
	switch name {
	case "placement slot", "placement entry", "participant artifacts", "configuration digest", "profile evidence":
		return true
	default:
		return false
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
