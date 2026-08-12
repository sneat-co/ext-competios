// Package contract4competiostest provides reusable fail-closed contract checks.
package contract4competiostest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type ExecutionProviderFactory func() contract4competios.ExecutionProvider

func CheckExecutionProvider(factory ExecutionProviderFactory) []error {
	return CheckExecutionProviderWithRequest(factory, executionFixture())
}

func CheckExecutionProviderWithRequest(factory ExecutionProviderFactory, request contract4competios.ExecutionRequest) []error {
	ctx := context.Background()
	provider := factory()
	grant := launchGrantFixture(request, "launch-token", "key-a")
	first, err := provider.LaunchExecution(ctx, grant, request)
	if err != nil {
		return []error{fmt.Errorf("first launch: %w", err)}
	}
	var violations []error
	if err := contract4competios.ValidateExecutionReceiptForRequest(first, request); err != nil || first.Status != contract4competios.ReceiptAccepted {
		violations = append(violations, fmt.Errorf("first receipt = %+v: %v", first, err))
	}

	replay, err := provider.LaunchExecution(ctx, grant, request)
	if err != nil || replay.Status != contract4competios.ReceiptReplayed || replay.ProviderInstanceID != first.ProviderInstanceID || !sameReceiptEvidence(first, replay) {
		violations = append(violations, fmt.Errorf("same-token replay = %+v: %v", replay, err))
	}
	freshGrant := launchGrantFixture(request, "launch-token-fresh", "key-rotated")
	replay, err = provider.LaunchExecution(ctx, freshGrant, request)
	if err != nil || replay.Status != contract4competios.ReceiptReplayed || replay.ProviderInstanceID != first.ProviderInstanceID || !sameReceiptEvidence(first, replay) {
		violations = append(violations, fmt.Errorf("fresh-token replay = %+v: %v", replay, err))
	}

	for name, mutate := range requestPayloadMutations() {
		payload := copyRequestPayload(request)
		mutate(&payload)
		changed, buildErr := contract4competios.NewExecutionRequest(payload)
		if buildErr != nil {
			violations = append(violations, fmt.Errorf("%s mutation did not produce a valid request: %v", name, buildErr))
			continue
		}
		changedGrant := launchGrantFixture(changed, "changed-"+name, "key-a")
		if _, launchErr := provider.LaunchExecution(ctx, changedGrant, changed); !errors.Is(launchErr, contract4competios.ErrCommandConflict) {
			violations = append(violations, fmt.Errorf("%s same-command mutation error = %v", name, launchErr))
		}
	}

	newCommandPayload := copyRequestPayload(request)
	newCommandPayload.CommandID = "independent-command"
	newCommandPayload.ID = "independent-request"
	newCommand, buildErr := contract4competios.NewExecutionRequest(newCommandPayload)
	if buildErr != nil {
		violations = append(violations, buildErr)
	} else {
		second, launchErr := provider.LaunchExecution(ctx, launchGrantFixture(newCommand, "independent-token", "key-a"), newCommand)
		if launchErr != nil || second.Status != contract4competios.ReceiptAccepted || second.ProviderInstanceID == first.ProviderInstanceID {
			violations = append(violations, fmt.Errorf("independent command = %+v: %v", second, launchErr))
		}
	}

	for name, mutate := range invalidLaunchGrantMutations() {
		bad := launchGrantFixture(request, "bad-"+name, "key-a")
		mutate(&bad.Claims)
		if _, launchErr := provider.LaunchExecution(ctx, bad, request); launchErr == nil {
			violations = append(violations, fmt.Errorf("%s grant was accepted", name))
		}
	}
	return violations
}

func sameReceiptEvidence(first, replay contract4competios.ExecutionReceipt) bool {
	if first.RequestID != replay.RequestID || first.CommandID != replay.CommandID || first.ProviderID != replay.ProviderID || first.AdapterID != replay.AdapterID || first.ProviderInstanceID != replay.ProviderInstanceID || len(first.SafeReferences) != len(replay.SafeReferences) {
		return false
	}
	for index := range first.SafeReferences {
		if first.SafeReferences[index] != replay.SafeReferences[index] {
			return false
		}
	}
	return true
}

func requestPayloadMutations() map[string]func(*contract4competios.ExecutionRequestPayload) {
	return map[string]func(*contract4competios.ExecutionRequestPayload){
		"id":          func(v *contract4competios.ExecutionRequestPayload) { v.ID = "changed-request" },
		"provider":    func(v *contract4competios.ExecutionRequestPayload) { v.ProviderID = "changed-provider" },
		"adapter":     func(v *contract4competios.ExecutionRequestPayload) { v.AdapterID = "changed-adapter" },
		"competition": func(v *contract4competios.ExecutionRequestPayload) { v.CompetitionID = "changed-cup" },
		"contest":     func(v *contract4competios.ExecutionRequestPayload) { v.ContestID = "changed-contest" },
		"game":        func(v *contract4competios.ExecutionRequestPayload) { v.GameID = "changed-game" },
		"ruleset":     func(v *contract4competios.ExecutionRequestPayload) { v.RulesetVersion = "changed-rules" },
		"profile kind": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile = contract4competios.ExecutionProfile{
				Kind: contract4competios.ExecutionProfileParticipantScheduled,
				ParticipantScheduled: &contract4competios.ParticipantScheduledProfile{
					StartsAt: fixtureTime.Add(2 * time.Hour),
					Slots: []contract4competios.ParticipantScheduledSlot{
						{Ordinal: 0, EntryID: "entry-a", Participants: []contract4competios.ParticipantID{"human-a"}},
						{Ordinal: 1, EntryID: "entry-b", Participants: []contract4competios.ParticipantID{"human-b"}},
					},
				},
			}
		},
		"slot order": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].EntryID, v.Profile.ProviderExecuted.Slots[1].EntryID = v.Profile.ProviderExecuted.Slots[1].EntryID, v.Profile.ProviderExecuted.Slots[0].EntryID
			v.Profile.ProviderExecuted.Slots[0].Participant, v.Profile.ProviderExecuted.Slots[1].Participant = v.Profile.ProviderExecuted.Slots[1].Participant, v.Profile.ProviderExecuted.Slots[0].Participant
		},
		"slot entry": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].EntryID = "changed-entry"
		},
		"participant": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].Participant.ParticipantID = "changed-participant"
		},
		"participant version": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].Participant.ParticipantVersionID = "changed-version"
		},
		"artifact digest": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].Participant.ArtifactDigest = artifactDigest("9")
		},
		"slot count": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots = append(v.Profile.ProviderExecuted.Slots, contract4competios.ExecutionSlot{
				Ordinal: 2, EntryID: "entry-c",
				Participant: contract4competios.ParticipantVersionRef{ParticipantID: "participant-c", ParticipantVersionID: "version-c", ArtifactDigest: artifactDigest("c")},
			})
		},
		"configuration version": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Configuration.Version = "changed-config"
		},
		"configuration body": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Configuration.Data = []byte("changed")
		},
		"not before": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.NotBefore = v.Profile.ProviderExecuted.NotBefore.Add(time.Second)
		},
		"deadline": func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Deadline = v.Profile.ProviderExecuted.Deadline.Add(time.Second)
		},
		"public artifacts": func(v *contract4competios.ExecutionRequestPayload) { v.RequestedPublicArtifacts = nil },
		"callback":         func(v *contract4competios.ExecutionRequestPayload) { v.Callback.Resource = "/changed/events" },
	}
}

func invalidLaunchGrantMutations() map[string]func(*contract4competios.OperationGrant) {
	return map[string]func(*contract4competios.OperationGrant){
		"issuer":       func(v *contract4competios.OperationGrant) { v.Issuer = "other" },
		"subject":      func(v *contract4competios.OperationGrant) { v.Subject = "other" },
		"audience":     func(v *contract4competios.OperationGrant) { v.Audience = "other" },
		"token type":   func(v *contract4competios.OperationGrant) { v.TokenType = "other" },
		"scope":        func(v *contract4competios.OperationGrant) { v.Scope = "other" },
		"purpose":      func(v *contract4competios.OperationGrant) { v.Purpose = "other" },
		"token ID":     func(v *contract4competios.OperationGrant) { v.TokenID = "" },
		"issued time":  func(v *contract4competios.OperationGrant) { v.IssuedAt = v.NotBefore.Add(time.Second) },
		"expiry":       func(v *contract4competios.OperationGrant) { v.ExpiresAt = v.NotBefore },
		"provider":     func(v *contract4competios.OperationGrant) { v.ProviderID = "other" },
		"adapter":      func(v *contract4competios.OperationGrant) { v.AdapterID = "other" },
		"competition":  func(v *contract4competios.OperationGrant) { v.CompetitionID = "other" },
		"contest":      func(v *contract4competios.OperationGrant) { v.ContestID = "other" },
		"request":      func(v *contract4competios.OperationGrant) { v.RequestID = "other" },
		"instance":     func(v *contract4competios.OperationGrant) { v.ProviderInstanceID = "forbidden" },
		"command":      func(v *contract4competios.OperationGrant) { v.CommandID = "other" },
		"typed digest": func(v *contract4competios.OperationGrant) { v.TypedPayloadDigest = payloadDigest("9") },
		"content type": func(v *contract4competios.OperationGrant) { v.TransportContentType = "application/json" },
		"raw digest":   func(v *contract4competios.OperationGrant) { v.RawTransportDigest = payloadDigest("8") },
		"method":       func(v *contract4competios.OperationGrant) { v.Method = "PUT" },
		"resource":     func(v *contract4competios.OperationGrant) { v.Resource = "/other" },
		"source field": func(v *contract4competios.OperationGrant) { v.ParticipantID = "forbidden" },
	}
}

func providerViolation(label string, err error) error {
	return fmt.Errorf("provider conformance %s: %w", label, err)
}
