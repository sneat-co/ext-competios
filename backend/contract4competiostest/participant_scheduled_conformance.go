package contract4competiostest

import (
	"context"
	"fmt"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

// ParticipantScheduledProviderPolicy is an adapter-owned acceptance boundary
// for a participant-scheduled provider. It deliberately does not alter the
// generic N-slot wire contract: an adapter supplies its own exact game,
// provider, adapter and supported slot-count policy to the conformance suite.
type ParticipantScheduledProviderPolicy struct {
	ProviderID     contract4competios.ProviderID
	AdapterID      contract4competios.AdapterID
	GameID         contract4competios.GameID
	RulesetVersion contract4competios.RulesetVersion
	SlotCount      int
}

// CheckParticipantScheduledProviderPolicy proves an adapter rejects a fresh
// command outside its declared participant-scheduled acceptance boundary
// before accepting the valid command. It complements CheckExecutionProvider,
// which remains the generic N-slot/idempotency conformance suite.
func CheckParticipantScheduledProviderPolicy(factory ExecutionProviderFactory, request contract4competios.ExecutionRequest, policy ParticipantScheduledProviderPolicy) []error {
	if err := validateParticipantScheduledPolicyFixture(request, policy); err != nil {
		return []error{err}
	}
	ctx := context.Background()
	mutations := map[string]func(*contract4competios.ExecutionRequestPayload){
		"wrong game": func(payload *contract4competios.ExecutionRequestPayload) { payload.GameID = "wrong-game" },
		"wrong adapter": func(payload *contract4competios.ExecutionRequestPayload) {
			payload.AdapterID = "wrong-adapter"
		},
		"wrong slot count": func(payload *contract4competios.ExecutionRequestPayload) {
			payload.Profile.ParticipantScheduled.Slots = append(payload.Profile.ParticipantScheduled.Slots, contract4competios.ParticipantScheduledSlot{
				Ordinal: uint16(len(payload.Profile.ParticipantScheduled.Slots)), EntryID: "wrong-extra-entry", Participants: []contract4competios.ParticipantID{"wrong-extra-participant"},
			})
		},
	}
	var violations []error
	for name, mutate := range mutations {
		provider := factory()
		payload := copyRequestPayload(request)
		payload.ID, payload.CommandID = contract4competios.ExecutionRequestID("policy-"+name), contract4competios.CommandID("policy-"+name)
		mutate(&payload)
		changed, err := contract4competios.NewExecutionRequest(payload)
		if err != nil {
			violations = append(violations, fmt.Errorf("%s fixture is not a valid generic request: %w", name, err))
			continue
		}
		if receipt, launchErr := provider.LaunchExecution(ctx, launchGrantFixture(changed, "policy-"+name, "key-a"), changed); launchErr == nil || !emptyExecutionReceipt(receipt) {
			violations = append(violations, fmt.Errorf("%s accepted by participant-scheduled provider: %+v, %v", name, receipt, launchErr))
		}
		if receipt, launchErr := provider.LaunchExecution(ctx, launchGrantFixture(request, "policy-valid-after-"+name, "key-a"), request); launchErr != nil || receipt.Status != contract4competios.ReceiptAccepted {
			violations = append(violations, fmt.Errorf("valid request after rejected %s = %+v, %v", name, receipt, launchErr))
		}
	}

	provider := factory()
	wrongPurpose := launchGrantFixture(request, "policy-wrong-purpose", "key-a")
	wrongPurpose.Claims.Purpose, wrongPurpose.Claims.Scope = contract4competios.GrantPurposeContestStarted, contract4competios.GrantScopeContestStarted
	if receipt, launchErr := provider.LaunchExecution(ctx, wrongPurpose, request); launchErr == nil || !emptyExecutionReceipt(receipt) {
		violations = append(violations, fmt.Errorf("wrong grant purpose accepted by participant-scheduled provider: %+v, %v", receipt, launchErr))
	}
	if receipt, launchErr := provider.LaunchExecution(ctx, launchGrantFixture(request, "policy-valid-after-purpose", "key-a"), request); launchErr != nil || receipt.Status != contract4competios.ReceiptAccepted {
		violations = append(violations, fmt.Errorf("valid request after rejected wrong grant purpose = %+v, %v", receipt, launchErr))
	}
	return violations
}

func validateParticipantScheduledPolicyFixture(request contract4competios.ExecutionRequest, policy ParticipantScheduledProviderPolicy) error {
	if policy.ProviderID == "" || policy.AdapterID == "" || policy.GameID == "" || policy.RulesetVersion == "" || policy.SlotCount < 1 ||
		request.ProviderID != policy.ProviderID || request.AdapterID != policy.AdapterID || request.GameID != policy.GameID || request.RulesetVersion != policy.RulesetVersion ||
		request.Profile.Kind != contract4competios.ExecutionProfileParticipantScheduled || request.Profile.ParticipantScheduled == nil || request.Profile.ProviderExecuted != nil || len(request.Profile.ParticipantScheduled.Slots) != policy.SlotCount {
		return fmt.Errorf("participant-scheduled policy fixture does not match declared provider boundary")
	}
	return nil
}
