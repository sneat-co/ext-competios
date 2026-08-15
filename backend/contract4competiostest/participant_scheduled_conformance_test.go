package contract4competiostest

import (
	"context"
	"strings"
	"testing"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

func TestGameBoardLiveParticipantScheduledPolicyConformance(t *testing.T) {
	request := gameBoardLiveScheduledFixture()
	policy := ParticipantScheduledProviderPolicy{
		ProviderID: "gameboard-live", AdapterID: "gameboard-live", GameID: "basketball",
		RulesetVersion: "gameboard-live-basketball-2026-08", SlotCount: 2,
	}
	if violations := CheckParticipantScheduledProviderPolicy(func() contract4competios.ExecutionProvider {
		return &policyProvider{policy: policy}
	}, request, policy); len(violations) != 0 {
		t.Fatalf("GameBoard.live participant-scheduled policy violations: %v", violations)
	}
}

func TestParticipantScheduledConformanceRejectsDeliberatelyBadProviderAndSink(t *testing.T) {
	request := gameBoardLiveScheduledFixture()
	policy := ParticipantScheduledProviderPolicy{
		ProviderID: "gameboard-live", AdapterID: "gameboard-live", GameID: "basketball",
		RulesetVersion: "gameboard-live-basketball-2026-08", SlotCount: 2,
	}
	violations := CheckParticipantScheduledProviderPolicy(func() contract4competios.ExecutionProvider {
		return permissiveProvider{}
	}, request, policy)
	for _, want := range []string{"wrong game", "wrong adapter", "wrong slot count", "wrong grant purpose"} {
		if !containsViolation(violations, want) {
			t.Fatalf("deliberately bad provider violations = %v, missing %q", violations, want)
		}
	}

	receipt := executionReceiptFixture(request, "gameboard-instance")
	start := startFixture(receipt.ProviderInstanceID)
	startPayload := start.Payload()
	startPayload.CompetitionID, startPayload.ContestID, startPayload.RequestID = request.CompetitionID, request.ContestID, request.ID
	startPayload.ProviderID, startPayload.AdapterID = request.ProviderID, request.AdapterID
	start = mustEvent(startPayload)
	result := resultFixtureForRequest(request, receipt.ProviderInstanceID, []uint16{1, 2})
	violations = CheckExecutionEventSinkWithEvents(func(contract4competios.ExecutionRequest, contract4competios.ExecutionReceipt) contract4competios.ExecutionEventSink {
		return permissiveEventSink{}
	}, request, receipt, start, result)
	if !containsViolation(violations, "placement slot") {
		t.Fatalf("deliberately bad event sink violations = %v, missing placement-order rejection", violations)
	}
}

type policyProvider struct {
	policy   ParticipantScheduledProviderPolicy
	launches map[contract4competios.CommandID]contract4competios.ExecutionReceipt
}

func (p *policyProvider) LaunchExecution(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	if contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateExecutionRequest(request) != nil || contract4competios.ValidateLaunchGrantForRequest(grant, launchRouteFixture(request), request) != nil {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	if request.ProviderID != p.policy.ProviderID || request.AdapterID != p.policy.AdapterID || request.GameID != p.policy.GameID || request.RulesetVersion != p.policy.RulesetVersion || request.Profile.Kind != contract4competios.ExecutionProfileParticipantScheduled || request.Profile.ParticipantScheduled == nil || len(request.Profile.ParticipantScheduled.Slots) != p.policy.SlotCount {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidExecution
	}
	if p.launches == nil {
		p.launches = map[contract4competios.CommandID]contract4competios.ExecutionReceipt{}
	}
	if prior, exists := p.launches[request.CommandID]; exists {
		prior.Status = contract4competios.ReceiptReplayed
		return prior, nil
	}
	receipt := executionReceiptFixture(request, contract4competios.ProviderInstanceID("gameboard-"+request.ID))
	p.launches[request.CommandID] = receipt
	return receipt, nil
}

type permissiveProvider struct{}

func (permissiveProvider) LaunchExecution(_ context.Context, _ contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	return executionReceiptFixture(request, "permissive-instance"), nil
}

type permissiveEventSink struct{}

func (permissiveEventSink) SubmitExecutionEvent(_ context.Context, _ contract4competios.VerifiedOperationGrant, _ contract4competios.ExecutionEvent) (contract4competios.EventAcknowledgement, error) {
	return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementAccepted}, nil
}

func gameBoardLiveScheduledFixture() contract4competios.ExecutionRequest {
	payload := scheduledExecutionFixture().Payload()
	payload.ProviderID, payload.AdapterID = "gameboard-live", "gameboard-live"
	payload.GameID, payload.RulesetVersion = "basketball", "gameboard-live-basketball-2026-08"
	payload.Profile.ParticipantScheduled.Slots[0].DisplayName = "River City Ravens"
	payload.Profile.ParticipantScheduled.Slots[1].DisplayName = "Northside Foxes"
	request, err := contract4competios.NewExecutionRequest(payload)
	if err != nil {
		panic(err)
	}
	return request
}

func mustEvent(payload contract4competios.ExecutionEventPayload) contract4competios.ExecutionEvent {
	event, err := contract4competios.NewExecutionEvent(payload)
	if err != nil {
		panic(err)
	}
	return event
}

func containsViolation(violations []error, want string) bool {
	for _, violation := range violations {
		if strings.Contains(violation.Error(), want) {
			return true
		}
	}
	return false
}
