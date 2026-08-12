package contract4competiostest

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

const (
	fixtureIssuer      = "https://issuer.example"
	fixtureSubject     = "svc:competios"
	fixtureAudience    = "game/execution"
	fixtureContentType = "application/vnd.competios.operation+json;version=1"
	fixtureMethod      = "POST"
	fixtureResource    = "/game/executions"
)

var fixtureTime = time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)

func artifactDigest(char string) contract4competios.ArtifactDigest {
	return contract4competios.ArtifactDigest("sha256:" + strings.Repeat(char, 64))
}

func payloadDigest(char string) contract4competios.PayloadDigest {
	return contract4competios.PayloadDigest("sha256:" + strings.Repeat(char, 64))
}

func executionFixture() contract4competios.ExecutionRequest {
	return executionFixtureFor("chess-raiders", "chess-provider-v1", []byte(`{"board":"standard"}`), 2)
}

func executionFixtureFor(game contract4competios.GameID, configVersion string, config []byte, slotCount int) contract4competios.ExecutionRequest {
	slots := make([]contract4competios.ExecutionSlot, slotCount)
	for index := range slots {
		letter := string(rune('a' + index))
		slots[index] = contract4competios.ExecutionSlot{
			Ordinal: uint16(index), EntryID: contract4competios.EntryID("entry-" + letter),
			Participant: contract4competios.ParticipantVersionRef{
				ParticipantID:        contract4competios.ParticipantID("participant-" + letter),
				ParticipantVersionID: contract4competios.ParticipantVersionID("version-" + letter),
				ArtifactDigest:       artifactDigest(letter),
			},
		}
	}
	request, err := contract4competios.NewExecutionRequest(contract4competios.ExecutionRequestPayload{
		ID: "request", ProviderID: "provider", AdapterID: "adapter",
		CompetitionID: "cup", ContestID: "contest", CommandID: "launch-command",
		GameID: game, RulesetVersion: "rules",
		Profile: contract4competios.ExecutionProfile{
			Kind: contract4competios.ExecutionProfileProviderExecuted,
			ProviderExecuted: &contract4competios.ProviderExecutedProfile{
				Slots: slots, Configuration: contract4competios.ProviderConfiguration{Version: configVersion, Data: config},
				NotBefore: fixtureTime.Add(time.Minute), Deadline: fixtureTime.Add(time.Hour),
			},
		},
		RequestedPublicArtifacts: []contract4competios.PublicArtifactKind{contract4competios.PublicArtifactTerminalReplay},
		Callback:                 contract4competios.CallbackResource{Resource: "/competios/events"},
	})
	if err != nil {
		panic(err)
	}
	return request
}

func scheduledExecutionFixture() contract4competios.ExecutionRequest {
	request, err := contract4competios.NewExecutionRequest(contract4competios.ExecutionRequestPayload{
		ID: "request", ProviderID: "provider", AdapterID: "adapter",
		CompetitionID: "cup", ContestID: "contest", CommandID: "launch-command",
		GameID: "scheduled-table-game", RulesetVersion: "rules",
		Profile: contract4competios.ExecutionProfile{
			Kind: contract4competios.ExecutionProfileParticipantScheduled,
			ParticipantScheduled: &contract4competios.ParticipantScheduledProfile{
				StartsAt: fixtureTime.Add(time.Hour),
				Slots: []contract4competios.ParticipantScheduledSlot{
					{Ordinal: 0, EntryID: "entry-a", Participants: []contract4competios.ParticipantID{"person-a", "person-b"}},
					{Ordinal: 1, EntryID: "entry-b", Participants: []contract4competios.ParticipantID{"person-c"}},
				},
			},
		},
		RequestedPublicArtifacts: []contract4competios.PublicArtifactKind{contract4competios.PublicArtifactTerminalReplay},
		Callback:                 contract4competios.CallbackResource{Resource: "/competios/events"},
	})
	if err != nil {
		panic(err)
	}
	return request
}

func executionReceiptFixture(request contract4competios.ExecutionRequest, instance contract4competios.ProviderInstanceID) contract4competios.ExecutionReceipt {
	return contract4competios.ExecutionReceipt{
		RequestID: request.ID, CommandID: request.CommandID,
		ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		ProviderInstanceID: instance, Status: contract4competios.ReceiptAccepted,
	}
}

func launchGrantFixture(request contract4competios.ExecutionRequest, tokenID, keyID string) contract4competios.VerifiedOperationGrant {
	body, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}
	claims := contract4competios.OperationGrant{
		Issuer: fixtureIssuer, Subject: fixtureSubject, Audience: fixtureAudience,
		TokenType: contract4competios.GrantTokenTypeAccessJWT,
		Scope:     contract4competios.GrantScopeContestLaunch, Purpose: contract4competios.GrantPurposeContestLaunch,
		KeyID: keyID, TokenID: tokenID, IssuedAt: fixtureTime, NotBefore: fixtureTime,
		ExpiresAt: fixtureTime.Add(5 * time.Minute), ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		CompetitionID: request.CompetitionID, ContestID: request.ContestID, RequestID: request.ID,
		CommandID: request.CommandID, TypedPayloadDigest: request.TypedPayloadDigest,
		TransportContentType: fixtureContentType,
		RawTransportDigest:   contract4competios.DigestRawTransportBody(fixtureContentType, body),
		Method:               fixtureMethod, Resource: fixtureResource,
	}
	return contract4competios.VerifiedOperationGrant{Claims: claims}
}

func launchRouteFixture(request contract4competios.ExecutionRequest) contract4competios.OperationRouteBinding {
	grant := launchGrantFixture(request, "route-token", "route-key").Claims
	return contract4competios.OperationRouteBinding{
		Issuer: grant.Issuer, Subject: grant.Subject, Audience: grant.Audience,
		TokenType: grant.TokenType, Scope: grant.Scope, Purpose: grant.Purpose,
		ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		TransportContentType: grant.TransportContentType, RawTransportDigest: grant.RawTransportDigest,
		Method: grant.Method, Resource: grant.Resource,
	}
}

func startFixture(instance contract4competios.ProviderInstanceID) contract4competios.ExecutionEvent {
	event, err := contract4competios.NewExecutionEvent(contract4competios.ExecutionEventPayload{
		ID: "start", Kind: contract4competios.LifecycleEventStarted,
		CompetitionID: "cup", ContestID: "contest", RequestID: "request",
		ProviderID: "provider", AdapterID: "adapter", ProviderInstanceID: instance,
		CommandID: "start-command", OccurredAt: fixtureTime.Add(time.Minute),
	})
	if err != nil {
		panic(err)
	}
	return event
}

func resultFixture(instance contract4competios.ProviderInstanceID) contract4competios.ExecutionEvent {
	return resultFixtureForRequest(executionFixture(), instance, []uint16{1, 1})
}

func resultFixtureForRequest(request contract4competios.ExecutionRequest, instance contract4competios.ProviderInstanceID, ranks []uint16) contract4competios.ExecutionEvent {
	var entries []contract4competios.EntryID
	var evidence contract4competios.CompletionEvidence
	switch request.Profile.Kind {
	case contract4competios.ExecutionProfileProviderExecuted:
		profile := request.Profile.ProviderExecuted
		entries = make([]contract4competios.EntryID, len(profile.Slots))
		artifacts := make([]contract4competios.ArtifactDigest, len(profile.Slots))
		for index, slot := range profile.Slots {
			entries[index], artifacts[index] = slot.EntryID, slot.Participant.ArtifactDigest
		}
		evidence = contract4competios.CompletionEvidence{
			ProfileKind: contract4competios.ExecutionProfileProviderExecuted,
			Replay:      contract4competios.TerminalReplay{State: contract4competios.ReplayAvailable, Reference: "replay:1"},
			ProviderExecuted: &contract4competios.RecordedProvenance{
				ParticipantArtifactDigests:  artifacts,
				ProviderConfigurationDigest: contract4competios.DigestProviderConfiguration(profile.Configuration),
				RuntimeDigest:               artifactDigest("d"), RulesDigest: artifactDigest("e"),
				LimitProfileDigest: artifactDigest("f"), SeedDigest: artifactDigest("0"),
				EventLogDigest: artifactDigest("1"), ExecutionPayloadDigest: payloadDigest("2"),
			},
		}
	case contract4competios.ExecutionProfileParticipantScheduled:
		profile := request.Profile.ParticipantScheduled
		entries = make([]contract4competios.EntryID, len(profile.Slots))
		for index, slot := range profile.Slots {
			entries[index] = slot.EntryID
		}
		evidence = contract4competios.CompletionEvidence{
			ProfileKind:          contract4competios.ExecutionProfileParticipantScheduled,
			Replay:               contract4competios.TerminalReplay{State: contract4competios.ReplayAvailable, Reference: "replay:scheduled-1"},
			ParticipantScheduled: &contract4competios.ParticipantScheduledCompletionEvidence{},
		}
	default:
		panic("unsupported execution profile")
	}
	if len(ranks) != len(entries) {
		panic("rank count does not match slots")
	}
	placements := make([]contract4competios.Placement, len(entries))
	for index, entry := range entries {
		placements[index] = contract4competios.Placement{SlotOrdinal: uint16(index), EntryID: entry, Rank: ranks[index], Status: contract4competios.PlacementStatusFinished}
	}
	event, err := contract4competios.NewExecutionEvent(contract4competios.ExecutionEventPayload{
		ID: "result", Kind: contract4competios.LifecycleEventCompleted,
		CompetitionID: request.CompetitionID, ContestID: request.ContestID, RequestID: request.ID,
		ProviderID: request.ProviderID, AdapterID: request.AdapterID, ProviderInstanceID: instance,
		CommandID: "result-command", OccurredAt: fixtureTime.Add(2 * time.Minute),
		Result: &contract4competios.ExecutionResult{
			Placements: placements,
			Evidence:   evidence,
		},
	})
	if err != nil {
		panic(err)
	}
	return event
}

func failureFixture(instance contract4competios.ProviderInstanceID, kind contract4competios.LifecycleEventKind, command string) contract4competios.ExecutionEvent {
	event, err := contract4competios.NewExecutionEvent(contract4competios.ExecutionEventPayload{
		ID: contract4competios.EventID(command + "-event"), Kind: kind,
		CompetitionID: "cup", ContestID: "contest", RequestID: "request",
		ProviderID: "provider", AdapterID: "adapter", ProviderInstanceID: instance,
		CommandID: contract4competios.CommandID(command), OccurredAt: fixtureTime.Add(90 * time.Second),
		Failure: &contract4competios.ExecutionFailure{Code: "provider-stopped"},
	})
	if err != nil {
		panic(err)
	}
	return event
}

func eventGrantFixture(event contract4competios.ExecutionEvent, tokenID, keyID string) contract4competios.VerifiedOperationGrant {
	body, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}
	purpose, scope := contract4competios.GrantPurposeContestResultSubmit, contract4competios.GrantScopeContestResultSubmit
	if event.Kind == contract4competios.LifecycleEventStarted {
		purpose, scope = contract4competios.GrantPurposeContestStarted, contract4competios.GrantScopeContestStarted
	}
	claims := contract4competios.OperationGrant{
		Issuer: "https://competios.example", Subject: "svc:game", Audience: "competios/events",
		TokenType: contract4competios.GrantTokenTypeAccessJWT, Scope: scope, Purpose: purpose,
		KeyID: keyID, TokenID: tokenID, IssuedAt: fixtureTime, NotBefore: fixtureTime,
		ExpiresAt: fixtureTime.Add(5 * time.Minute), ProviderID: event.ProviderID, AdapterID: event.AdapterID,
		CompetitionID: event.CompetitionID, ContestID: event.ContestID, RequestID: event.RequestID,
		ProviderInstanceID: event.ProviderInstanceID, CommandID: event.CommandID,
		TypedPayloadDigest: event.TypedPayloadDigest, TransportContentType: fixtureContentType,
		RawTransportDigest: contract4competios.DigestRawTransportBody(fixtureContentType, body),
		Method:             fixtureMethod, Resource: "/competios/events",
	}
	return contract4competios.VerifiedOperationGrant{Claims: claims}
}

func eventRouteFixture(event contract4competios.ExecutionEvent) contract4competios.OperationRouteBinding {
	grant := eventGrantFixture(event, "route-event-token", "route-key").Claims
	return contract4competios.OperationRouteBinding{
		Issuer: grant.Issuer, Subject: grant.Subject, Audience: grant.Audience,
		TokenType: grant.TokenType, Scope: grant.Scope, Purpose: grant.Purpose,
		ProviderID: event.ProviderID, AdapterID: event.AdapterID,
		TransportContentType: grant.TransportContentType, RawTransportDigest: grant.RawTransportDigest,
		Method: grant.Method, Resource: grant.Resource,
	}
}

func copyRequestPayload(request contract4competios.ExecutionRequest) contract4competios.ExecutionRequestPayload {
	bytes, _ := json.Marshal(request.Payload())
	var payload contract4competios.ExecutionRequestPayload
	_ = json.Unmarshal(bytes, &payload)
	return payload
}

func copyEventPayload(event contract4competios.ExecutionEvent) contract4competios.ExecutionEventPayload {
	bytes, _ := json.Marshal(event.Payload())
	var payload contract4competios.ExecutionEventPayload
	_ = json.Unmarshal(bytes, &payload)
	return payload
}
