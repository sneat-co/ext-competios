// Package contract4competiostest provides reusable provider conformance checks.
package contract4competiostest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type ExecutionProviderFactory func() contract4competios.ExecutionProvider
type ExecutionEventSinkFactory func() contract4competios.ExecutionEventSink
type OperationGrantVerifierFactory func() contract4competios.OperationGrantVerifier

func CheckExecutionProvider(factory ExecutionProviderFactory) []error {
	return CheckExecutionProviderWithRequest(factory, executionFixture())
}

func CheckExecutionProviderWithRequest(factory ExecutionProviderFactory, request contract4competios.ExecutionRequest) []error {
	ctx, grant := context.Background(), launchGrantFixture()
	provider := factory()
	first, err := provider.LaunchExecution(ctx, grant, request)
	if err != nil {
		return []error{fmt.Errorf("first launch: %w", err)}
	}
	if first.Status != contract4competios.ReceiptAccepted || first.ProviderInstanceID == "" {
		return []error{fmt.Errorf("first receipt = %+v", first)}
	}
	var violations []error
	replay, err := provider.LaunchExecution(ctx, grant, request)
	if err != nil || replay.Status != contract4competios.ReceiptReplayed || replay.ProviderInstanceID != first.ProviderInstanceID {
		violations = append(violations, fmt.Errorf("same-command replay = %+v, %v", replay, err))
	}
	changed := request
	changed.Configuration.Data = []byte("other")
	changed.TypedPayloadDigest, _ = contract4competios.DigestTypedPayload(struct {
		ID   contract4competios.ExecutionRequestID `json:"id"`
		Data string                                `json:"data"`
	}{changed.ID, string(changed.Configuration.Data)})
	if _, err := provider.LaunchExecution(ctx, grant, changed); !errors.Is(err, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf("changed command body error = %v", err))
	}
	wrong := grant
	wrong.Claims.Purpose = contract4competios.GrantPurposeParticipantVersionValidate
	if _, err := provider.LaunchExecution(ctx, wrong, request); err == nil {
		violations = append(violations, errors.New("wrong grant purpose was accepted"))
	}
	wrong = grant
	wrong.Claims.ContestID = "other"
	if _, err := provider.LaunchExecution(ctx, wrong, request); err == nil {
		violations = append(violations, errors.New("wrong grant contest was accepted"))
	}
	wrong = grant
	wrong.Claims.Audience = "other"
	if _, err := provider.LaunchExecution(ctx, wrong, request); err == nil {
		violations = append(violations, errors.New("wrong grant audience was accepted"))
	}
	wrong = grant
	wrong.Claims.RawTransportDigest = "sha256:other"
	if _, err := provider.LaunchExecution(ctx, wrong, request); err == nil {
		violations = append(violations, errors.New("wrong grant transport digest was accepted"))
	}
	return violations
}

func CheckExecutionEventSink(factory ExecutionEventSinkFactory) []error {
	return CheckExecutionEventSinkWithEvents(factory, startFixture("instance"), resultFixture("instance"))
}

func CheckExecutionEventSinkWithEvents(factory ExecutionEventSinkFactory, start, result contract4competios.ExecutionEvent) []error {
	ctx, sink, request, launchGrant := context.Background(), factory(), executionFixture(), launchGrantFixture()
	receipt, err := (&referenceProvider{}).LaunchExecution(ctx, launchGrant, request)
	if err != nil {
		return []error{err}
	}
	start.ProviderInstanceID = receipt.ProviderInstanceID
	grant := eventGrantFixture(start, contract4competios.GrantPurposeContestStarted)
	ack, err := sink.SubmitExecutionEvent(ctx, grant, start)
	if err != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		return []error{fmt.Errorf("first start = %+v, %v", ack, err)}
	}
	var violations []error
	ack, err = sink.SubmitExecutionEvent(ctx, grant, start)
	if err != nil || ack.Status != contract4competios.EventAcknowledgementReplayed {
		violations = append(violations, fmt.Errorf("start replay = %+v, %v", ack, err))
	}
	result.ProviderInstanceID = receipt.ProviderInstanceID
	resultGrant := eventGrantFixture(result, contract4competios.GrantPurposeContestResultSubmit)
	ack, err = sink.SubmitExecutionEvent(ctx, resultGrant, result)
	if err != nil || ack.Status != contract4competios.EventAcknowledgementAccepted {
		violations = append(violations, fmt.Errorf("result = %+v, %v", ack, err))
	}
	changed := result
	changed.Result = &contract4competios.ExecutionResult{ID: result.Result.ID, CompletedAt: result.Result.CompletedAt, Placements: append([]contract4competios.Placement(nil), result.Result.Placements...), Replay: result.Result.Replay, RecordedProvenance: result.Result.RecordedProvenance}
	changed.Result.Placements[0].Rank = 2
	if _, err := sink.SubmitExecutionEvent(ctx, resultGrant, changed); !errors.Is(err, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf("changed result error = %v", err))
	}
	wrongInstance := result
	wrongInstance.ProviderInstanceID = "other"
	if _, err := sink.SubmitExecutionEvent(ctx, resultGrant, wrongInstance); err == nil {
		violations = append(violations, errors.New("wrong provider instance was accepted"))
	}
	return violations
}

func CheckOperationGrantVerifier(factory OperationGrantVerifierFactory) []error {
	ctx, verifier, good := context.Background(), factory(), launchGrantFixture().Claims
	verified, err := verifier.VerifyOperationGrant(ctx, good)
	if err != nil || verified.Claims.TokenID != good.TokenID {
		return []error{fmt.Errorf("good grant = %+v, %v", verified, err)}
	}
	var violations []error
	for name, change := range map[string]func(*contract4competios.OperationGrant){
		"purpose":   func(v *contract4competios.OperationGrant) { v.Purpose = "other" },
		"audience":  func(v *contract4competios.OperationGrant) { v.Audience = "other" },
		"digest":    func(v *contract4competios.OperationGrant) { v.RawTransportDigest = "sha256:other" },
		"cancelled": func(v *contract4competios.OperationGrant) { v.ExpiresAt = v.NotBefore },
	} {
		bad := good
		change(&bad)
		if _, err := verifier.VerifyOperationGrant(ctx, bad); err == nil {
			violations = append(violations, fmt.Errorf("%s grant was accepted", name))
		}
	}
	return violations
}

func executionFixture() contract4competios.ExecutionRequest {
	request := contract4competios.ExecutionRequest{ID: "request", ProviderID: "provider", AdapterID: "adapter", CompetitionID: "cup", ContestID: "contest", CommandID: "launch-command", GameID: "generic-game", RulesetVersion: "rules", Configuration: contract4competios.ProviderConfiguration{Version: "provider-config", Data: []byte("opaque")}, Callback: contract4competios.CallbackResource{Resource: "competios/events"}, Slots: []contract4competios.ExecutionSlot{{Ordinal: 0, EntryID: "entry-a", Participant: contract4competios.ParticipantVersionRef{ParticipantID: "participant-a", ParticipantVersionID: "version-a", ArtifactDigest: "sha256:a"}}, {Ordinal: 1, EntryID: "entry-b", Participant: contract4competios.ParticipantVersionRef{ParticipantID: "participant-b", ParticipantVersionID: "version-b", ArtifactDigest: "sha256:b"}}}}
	digest, _ := contract4competios.DigestTypedPayload(struct {
		ID contract4competios.ExecutionRequestID `json:"id"`
	}{request.ID})
	request.TypedPayloadDigest = digest
	return request
}

func launchGrantFixture() contract4competios.VerifiedOperationGrant {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	return contract4competios.VerifiedOperationGrant{Claims: contract4competios.OperationGrant{Issuer: "game", Subject: "svc:competios", Audience: "game/execution", Purpose: contract4competios.GrantPurposeContestLaunch, KeyID: "key", TokenID: "token", IssuedAt: now, NotBefore: now, ExpiresAt: now.Add(time.Minute), ProviderID: "provider", AdapterID: "adapter", CompetitionID: "cup", ContestID: "contest", RequestID: "request", CommandID: "launch-command", TransportContentType: "application/json;v=1", RawTransportDigest: "sha256:transport", Method: "POST", Resource: "game/executions"}}
}

func startFixture(instance contract4competios.ProviderInstanceID) contract4competios.ExecutionEvent {
	return contract4competios.ExecutionEvent{ID: "start", Kind: contract4competios.LifecycleStarted, CompetitionID: "cup", ContestID: "contest", RequestID: "request", ProviderID: "provider", AdapterID: "adapter", ProviderInstanceID: instance, CommandID: "start-command", OccurredAt: time.Date(2026, 8, 11, 12, 1, 0, 0, time.UTC)}
}
func resultFixture(instance contract4competios.ProviderInstanceID) contract4competios.ExecutionEvent {
	return contract4competios.ExecutionEvent{ID: "result", Kind: contract4competios.LifecycleCompleted, CompetitionID: "cup", ContestID: "contest", RequestID: "request", ProviderID: "provider", AdapterID: "adapter", ProviderInstanceID: instance, CommandID: "result-command", OccurredAt: time.Date(2026, 8, 11, 12, 2, 0, 0, time.UTC), Result: &contract4competios.ExecutionResult{ID: "result", CompletedAt: time.Date(2026, 8, 11, 12, 2, 0, 0, time.UTC), Placements: []contract4competios.Placement{{EntryID: "entry-a", Rank: 1, Status: contract4competios.PlacementStatusFinished}, {EntryID: "entry-b", Rank: 1, Status: contract4competios.PlacementStatusFinished}}, Replay: contract4competios.TerminalReplay{State: contract4competios.ReplayAvailable, Reference: "replay:1"}, RecordedProvenance: contract4competios.RecordedProvenance{ParticipantArtifactDigests: []contract4competios.ArtifactDigest{"sha256:a", "sha256:b"}, ProviderConfigurationDigest: "sha256:config", RuntimeDigest: "sha256:runtime", RulesDigest: "sha256:rules", LimitProfileDigest: "sha256:limits", SeedDigest: "sha256:seed", EventLogDigest: "sha256:events", ExecutionPayloadDigest: "sha256:execution", ResultPayloadDigest: "sha256:result"}}}
}
func eventGrantFixture(event contract4competios.ExecutionEvent, purpose contract4competios.GrantPurpose) contract4competios.VerifiedOperationGrant {
	grant := launchGrantFixture().Claims
	grant.Purpose, grant.ProviderInstanceID, grant.CommandID, grant.RawTransportDigest, grant.Resource = purpose, event.ProviderInstanceID, event.CommandID, "sha256:event", "competios/events"
	return contract4competios.VerifiedOperationGrant{Claims: grant}
}

type referenceProvider struct {
	receipt     *contract4competios.ExecutionReceipt
	fingerprint contract4competios.PayloadDigest
}

func (p *referenceProvider) LaunchExecution(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	if grant.Claims.Purpose != contract4competios.GrantPurposeContestLaunch || grant.Claims.Audience != "game/execution" || grant.Claims.RawTransportDigest != "sha256:transport" || grant.Claims.ContestID != request.ContestID || grant.Claims.RequestID != request.ID || grant.Claims.CommandID != request.CommandID || contract4competios.ValidateExecutionRequest(request) != nil {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	if p.receipt != nil {
		if p.fingerprint != request.TypedPayloadDigest {
			return contract4competios.ExecutionReceipt{}, contract4competios.ErrCommandConflict
		}
		replay := *p.receipt
		replay.Status = contract4competios.ReceiptReplayed
		return replay, nil
	}
	receipt := contract4competios.ExecutionReceipt{RequestID: request.ID, CommandID: request.CommandID, ProviderID: request.ProviderID, AdapterID: request.AdapterID, ProviderInstanceID: "instance", Status: contract4competios.ReceiptAccepted}
	p.receipt, p.fingerprint = &receipt, request.TypedPayloadDigest
	return receipt, nil
}
