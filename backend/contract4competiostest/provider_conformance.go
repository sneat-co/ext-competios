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
	for name, mutate := range invalidLaunchGrantMutations() {
		bad := launchGrantFixture(request, "populated-launch-bad-"+name, "key-a")
		mutate(&bad.Claims)
		if badReceipt, launchErr := provider.LaunchExecution(ctx, bad, request); !errors.Is(launchErr, contract4competios.ErrInvalidGrant) || !emptyExecutionReceipt(badReceipt) {
			violations = append(violations, fmt.Errorf("populated launch %s grant = %+v: %v, want empty receipt and ErrInvalidGrant", name, badReceipt, launchErr))
		}
		replayed, replayErr := provider.LaunchExecution(ctx, launchGrantFixture(request, "populated-launch-retry-"+name, "key-rotated"), request)
		if replayErr != nil || replayed.Status != contract4competios.ReceiptReplayed || !sameReceiptEvidence(first, replayed) {
			violations = append(violations, fmt.Errorf("populated launch %s valid retry = %+v: %v", name, replayed, replayErr))
		}
	}

	for name, mutate := range requestPayloadMutations(request) {
		mutationProvider := factory()
		baseline, baselineErr := mutationProvider.LaunchExecution(ctx, launchGrantFixture(request, "mutation-baseline-"+name, "key-a"), request)
		if baselineErr != nil || baseline.Status != contract4competios.ReceiptAccepted {
			violations = append(violations, fmt.Errorf("%s mutation baseline = %+v: %v", name, baseline, baselineErr))
			continue
		}
		payload := copyRequestPayload(request)
		mutate(&payload)
		changed, buildErr := contract4competios.NewExecutionRequest(payload)
		if buildErr != nil {
			violations = append(violations, fmt.Errorf("%s mutation did not produce a valid request: %v", name, buildErr))
			continue
		}
		changedGrant := launchGrantFixture(changed, "changed-"+name, "key-a")
		_, launchErr := mutationProvider.LaunchExecution(ctx, changedGrant, changed)
		want := contract4competios.ErrCommandConflict
		if name == "provider" || name == "adapter" {
			want = contract4competios.ErrInvalidGrant
		}
		if !errors.Is(launchErr, want) {
			violations = append(violations, fmt.Errorf("%s same-command mutation error = %v, want %v", name, launchErr, want))
		}
		replayed, replayErr := mutationProvider.LaunchExecution(ctx, launchGrantFixture(request, "mutation-replay-"+name, "key-rotated"), request)
		if replayErr != nil || replayed.Status != contract4competios.ReceiptReplayed || replayed.ProviderInstanceID != baseline.ProviderInstanceID || !sameReceiptEvidence(baseline, replayed) {
			violations = append(violations, fmt.Errorf("%s exact replay after rejection = %+v: %v", name, replayed, replayErr))
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
		rejectionProvider := factory()
		bad := launchGrantFixture(request, "bad-"+name, "key-a")
		mutate(&bad.Claims)
		if _, launchErr := rejectionProvider.LaunchExecution(ctx, bad, request); launchErr == nil {
			violations = append(violations, fmt.Errorf("%s grant was accepted", name))
		}
		accepted, launchErr := rejectionProvider.LaunchExecution(ctx, launchGrantFixture(request, "valid-after-bad-"+name, "key-a"), request)
		if launchErr != nil || accepted.Status != contract4competios.ReceiptAccepted {
			violations = append(violations, fmt.Errorf("valid launch after rejected %s grant = %+v: %v", name, accepted, launchErr))
		}
	}
	return violations
}

func emptyExecutionReceipt(value contract4competios.ExecutionReceipt) bool {
	return value.RequestID == "" && value.CommandID == "" && value.ProviderID == "" && value.AdapterID == "" && value.ProviderInstanceID == "" && value.Status == "" && len(value.SafeReferences) == 0
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

func requestPayloadMutations(request contract4competios.ExecutionRequest) map[string]func(*contract4competios.ExecutionRequestPayload) {
	mutations := map[string]func(*contract4competios.ExecutionRequestPayload){
		"id":               func(v *contract4competios.ExecutionRequestPayload) { v.ID = "changed-request" },
		"provider":         func(v *contract4competios.ExecutionRequestPayload) { v.ProviderID = "changed-provider" },
		"adapter":          func(v *contract4competios.ExecutionRequestPayload) { v.AdapterID = "changed-adapter" },
		"competition":      func(v *contract4competios.ExecutionRequestPayload) { v.CompetitionID = "changed-cup" },
		"contest":          func(v *contract4competios.ExecutionRequestPayload) { v.ContestID = "changed-contest" },
		"game":             func(v *contract4competios.ExecutionRequestPayload) { v.GameID = "changed-game" },
		"ruleset":          func(v *contract4competios.ExecutionRequestPayload) { v.RulesetVersion = "changed-rules" },
		"public artifacts": func(v *contract4competios.ExecutionRequestPayload) { v.RequestedPublicArtifacts = nil },
		"callback":         func(v *contract4competios.ExecutionRequestPayload) { v.Callback.Resource = "/changed/events" },
	}
	if request.Profile.Kind == contract4competios.ExecutionProfileProviderExecuted {
		mutations["profile kind"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile = scheduledExecutionFixture().Profile
		}
		mutations["slot order"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].EntryID, v.Profile.ProviderExecuted.Slots[1].EntryID = v.Profile.ProviderExecuted.Slots[1].EntryID, v.Profile.ProviderExecuted.Slots[0].EntryID
			v.Profile.ProviderExecuted.Slots[0].Participant, v.Profile.ProviderExecuted.Slots[1].Participant = v.Profile.ProviderExecuted.Slots[1].Participant, v.Profile.ProviderExecuted.Slots[0].Participant
		}
		mutations["slot entry"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].EntryID = "changed-entry"
		}
		mutations["participant"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].Participant.ParticipantID = "changed-participant"
		}
		mutations["participant version"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].Participant.ParticipantVersionID = "changed-version"
		}
		mutations["artifact digest"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots[0].Participant.ArtifactDigest = artifactDigest("9")
		}
		mutations["slot count"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Slots = append(v.Profile.ProviderExecuted.Slots, contract4competios.ExecutionSlot{
				Ordinal: 2, EntryID: "entry-c",
				Participant: contract4competios.ParticipantVersionRef{ParticipantID: "participant-c", ParticipantVersionID: "version-c", ArtifactDigest: artifactDigest("c")},
			})
		}
		mutations["configuration version"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Configuration.Version = "changed-config"
		}
		mutations["configuration body"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Configuration.Data = []byte("changed")
		}
		mutations["not before"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.NotBefore = v.Profile.ProviderExecuted.NotBefore.Add(time.Second)
		}
		mutations["deadline"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ProviderExecuted.Deadline = v.Profile.ProviderExecuted.Deadline.Add(time.Second)
		}
	} else {
		mutations["profile kind"] = func(v *contract4competios.ExecutionRequestPayload) { v.Profile = executionFixture().Profile }
		mutations["slot order"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ParticipantScheduled.Slots[0].EntryID, v.Profile.ParticipantScheduled.Slots[1].EntryID = v.Profile.ParticipantScheduled.Slots[1].EntryID, v.Profile.ParticipantScheduled.Slots[0].EntryID
			v.Profile.ParticipantScheduled.Slots[0].Participants, v.Profile.ParticipantScheduled.Slots[1].Participants = v.Profile.ParticipantScheduled.Slots[1].Participants, v.Profile.ParticipantScheduled.Slots[0].Participants
		}
		mutations["slot entry"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ParticipantScheduled.Slots[0].EntryID = "changed-entry"
		}
		mutations["participant"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ParticipantScheduled.Slots[0].Participants[0] = "changed-person"
		}
		mutations["slot count"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ParticipantScheduled.Slots = append(v.Profile.ParticipantScheduled.Slots, contract4competios.ParticipantScheduledSlot{Ordinal: 2, EntryID: "entry-c", Participants: []contract4competios.ParticipantID{"person-d"}})
		}
		mutations["starts at"] = func(v *contract4competios.ExecutionRequestPayload) {
			v.Profile.ParticipantScheduled.StartsAt = v.Profile.ParticipantScheduled.StartsAt.Add(time.Second)
		}
	}
	return mutations
}

func invalidLaunchGrantMutations() map[string]func(*contract4competios.OperationGrant) {
	return map[string]func(*contract4competios.OperationGrant){
		"issuer":       func(v *contract4competios.OperationGrant) { v.Issuer = "other" },
		"subject":      func(v *contract4competios.OperationGrant) { v.Subject = "other" },
		"audience":     func(v *contract4competios.OperationGrant) { v.Audience = "other" },
		"token type":   func(v *contract4competios.OperationGrant) { v.TokenType = "other" },
		"scope":        func(v *contract4competios.OperationGrant) { v.Scope = "other" },
		"purpose":      func(v *contract4competios.OperationGrant) { v.Purpose = "other" },
		"key":          func(v *contract4competios.OperationGrant) { v.KeyID = "" },
		"token ID":     func(v *contract4competios.OperationGrant) { v.TokenID = "" },
		"issued time":  func(v *contract4competios.OperationGrant) { v.IssuedAt = v.NotBefore.Add(time.Second) },
		"not before":   func(v *contract4competios.OperationGrant) { v.NotBefore = v.IssuedAt.Add(time.Hour) },
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
