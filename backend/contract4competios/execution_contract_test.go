package contract4competios

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

var contractTestTime = time.Date(2026, time.August, 12, 9, 10, 11, 12000000, time.UTC)

func testArtifactDigest(char string) ArtifactDigest {
	return ArtifactDigest("sha256:" + strings.Repeat(char, 64))
}

func testPayloadDigest(char string) PayloadDigest {
	return PayloadDigest("sha256:" + strings.Repeat(char, 64))
}

func providerRequestPayloadFixture() ExecutionRequestPayload {
	return ExecutionRequestPayload{
		ID:             "request-1",
		ProviderID:     "provider-1",
		AdapterID:      "adapter-1",
		CompetitionID:  "competition-1",
		ContestID:      "contest-1",
		CommandID:      "command-1",
		GameID:         "generic-board-game",
		RulesetVersion: "rules-2026-08",
		Profile: ExecutionProfile{
			Kind: ExecutionProfileProviderExecuted,
			ProviderExecuted: &ProviderExecutedProfile{
				Slots: []ExecutionSlot{
					{Ordinal: 0, EntryID: "entry-a", Participant: ParticipantVersionRef{ParticipantID: "participant-a", ParticipantVersionID: "version-a", ArtifactDigest: testArtifactDigest("a")}},
					{Ordinal: 1, EntryID: "entry-b", Participant: ParticipantVersionRef{ParticipantID: "participant-b", ParticipantVersionID: "version-b", ArtifactDigest: testArtifactDigest("b")}},
				},
				Configuration: ProviderConfiguration{Version: "config-1", Data: []byte(`{"turnLimit":9}`)},
				NotBefore:     contractTestTime.Add(time.Minute),
				Deadline:      contractTestTime.Add(time.Hour),
			},
		},
		RequestedPublicArtifacts: []PublicArtifactKind{PublicArtifactTerminalReplay},
		Callback:                 CallbackResource{Resource: "/competitions/competition-1/contests/contest-1/events"},
	}
}

func scheduledRequestPayloadFixture() ExecutionRequestPayload {
	payload := providerRequestPayloadFixture()
	payload.Profile = ExecutionProfile{
		Kind: ExecutionProfileParticipantScheduled,
		ParticipantScheduled: &ParticipantScheduledProfile{
			StartsAt: contractTestTime.Add(2 * time.Hour),
			Slots: []ParticipantScheduledSlot{
				{Ordinal: 0, EntryID: "team-a", Participants: []ParticipantID{"person-a", "person-b"}},
				{Ordinal: 1, EntryID: "team-b", Participants: []ParticipantID{"person-c"}},
			},
		},
	}
	payload.RequestedPublicArtifacts = nil
	return payload
}

func mustExecutionRequest(t *testing.T, payload ExecutionRequestPayload) ExecutionRequest {
	t.Helper()
	request, err := NewExecutionRequest(payload)
	if err != nil {
		t.Fatalf("NewExecutionRequest() error = %v", err)
	}
	return request
}

func TestExecutionProfileDiscriminator(t *testing.T) {
	for _, payload := range []ExecutionRequestPayload{providerRequestPayloadFixture(), scheduledRequestPayloadFixture()} {
		if _, err := NewExecutionRequest(payload); err != nil {
			t.Fatalf("valid %q profile rejected: %v", payload.Profile.Kind, err)
		}
	}

	wrong := scheduledRequestPayloadFixture()
	wrong.Profile.ProviderExecuted = providerRequestPayloadFixture().Profile.ProviderExecuted
	if _, err := NewExecutionRequest(wrong); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("two profile bodies error = %v, want ErrInvalidExecution", err)
	}

	wrong = scheduledRequestPayloadFixture()
	wrong.Profile.Kind = ExecutionProfileProviderExecuted
	if _, err := NewExecutionRequest(wrong); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("mismatched discriminator error = %v, want ErrInvalidExecution", err)
	}
}

func TestExecutionRequestCanonicalDigestVector(t *testing.T) {
	payload := providerRequestPayloadFixture()
	digest, err := DigestExecutionRequestPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	const want = "sha256:63e22a393af5ba610c91becd8653a929a2a917aab63a44e7929187c773fd8e53"
	if digest != want {
		t.Fatalf("digest = %q, want %q", digest, want)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "typedPayloadDigest") {
		t.Fatalf("canonical payload unexpectedly contains its digest: %s", encoded)
	}
}

func TestExecutionRequestRejectsMutatedCanonicalFields(t *testing.T) {
	tests := map[string]func(*ExecutionRequest){
		"id":           func(v *ExecutionRequest) { v.ID = "other" },
		"provider":     func(v *ExecutionRequest) { v.ProviderID = "other" },
		"adapter":      func(v *ExecutionRequest) { v.AdapterID = "other" },
		"competition":  func(v *ExecutionRequest) { v.CompetitionID = "other" },
		"contest":      func(v *ExecutionRequest) { v.ContestID = "other" },
		"command":      func(v *ExecutionRequest) { v.CommandID = "other" },
		"game":         func(v *ExecutionRequest) { v.GameID = "other" },
		"ruleset":      func(v *ExecutionRequest) { v.RulesetVersion = "other" },
		"profile kind": func(v *ExecutionRequest) { v.Profile.Kind = ExecutionProfileParticipantScheduled },
		"slot ordinal": func(v *ExecutionRequest) { v.Profile.ProviderExecuted.Slots[0].Ordinal = 1 },
		"slot entry":   func(v *ExecutionRequest) { v.Profile.ProviderExecuted.Slots[0].EntryID = "other" },
		"participant":  func(v *ExecutionRequest) { v.Profile.ProviderExecuted.Slots[0].Participant.ParticipantID = "other" },
		"participant version": func(v *ExecutionRequest) {
			v.Profile.ProviderExecuted.Slots[0].Participant.ParticipantVersionID = "other"
		},
		"participant artifact":  func(v *ExecutionRequest) { v.Profile.ProviderExecuted.Slots[0].Participant.ArtifactDigest = "other" },
		"configuration version": func(v *ExecutionRequest) { v.Profile.ProviderExecuted.Configuration.Version = "other" },
		"configuration bytes":   func(v *ExecutionRequest) { v.Profile.ProviderExecuted.Configuration.Data = []byte("other") },
		"not before": func(v *ExecutionRequest) {
			v.Profile.ProviderExecuted.NotBefore = v.Profile.ProviderExecuted.NotBefore.Add(time.Second)
		},
		"deadline": func(v *ExecutionRequest) {
			v.Profile.ProviderExecuted.Deadline = v.Profile.ProviderExecuted.Deadline.Add(time.Second)
		},
		"requested public artifacts": func(v *ExecutionRequest) { v.RequestedPublicArtifacts = nil },
		"callback":                   func(v *ExecutionRequest) { v.Callback.Resource = "/other" },
		"typed digest":               func(v *ExecutionRequest) { v.TypedPayloadDigest = testPayloadDigest("f") },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			request := mustExecutionRequest(t, providerRequestPayloadFixture())
			mutate(&request)
			if err := ValidateExecutionRequest(request); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("ValidateExecutionRequest() error = %v, want ErrInvalidExecution", err)
			}
		})
	}
}

func TestRawTransportDigestVectorAndBoundaries(t *testing.T) {
	const contentType = "application/vnd.competios.execution+json;version=1"
	body := []byte(`{"request":"same bytes"}`)
	const want = "sha256:cba5d4ae2e4b29c89158500797160e956e0f80d85ca35bcfb2a5b953f9d44263"
	if got := DigestRawTransportBody(contentType, body); got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
	baseline := DigestRawTransportBody(contentType, body)
	if got := DigestRawTransportBody(contentType+";charset=utf-8", body); got == baseline {
		t.Fatal("content-type mutation did not change raw digest")
	}
	if got := DigestRawTransportBody(contentType, append(body, ' ')); got == baseline {
		t.Fatal("body mutation did not change raw digest")
	}
}

func completedEventPayloadFixture() ExecutionEventPayload {
	return ExecutionEventPayload{
		ID:                 "event-1",
		Kind:               LifecycleEventCompleted,
		CompetitionID:      "competition-1",
		ContestID:          "contest-1",
		RequestID:          "request-1",
		ProviderID:         "provider-1",
		AdapterID:          "adapter-1",
		ProviderInstanceID: "instance-1",
		CommandID:          "event-command-1",
		OccurredAt:         contractTestTime.Add(30 * time.Minute),
		Result: &ExecutionResult{
			Placements: []Placement{
				{SlotOrdinal: 0, EntryID: "entry-a", Rank: 1, Status: PlacementStatusFinished},
				{SlotOrdinal: 1, EntryID: "entry-b", Rank: 1, Status: PlacementStatusFinished},
				{SlotOrdinal: 2, EntryID: "entry-c", Rank: 3, Status: PlacementStatusFinished},
			},
			Evidence: CompletionEvidence{
				Replay: TerminalReplay{State: ReplayAvailable, Reference: "replay-1"},
				RecordedProvenance: RecordedProvenance{
					ParticipantArtifactDigests:  []ArtifactDigest{testArtifactDigest("a"), testArtifactDigest("b"), testArtifactDigest("c")},
					ProviderConfigurationDigest: testArtifactDigest("d"),
					RuntimeDigest:               testArtifactDigest("e"),
					RulesDigest:                 testArtifactDigest("f"),
					LimitProfileDigest:          testArtifactDigest("0"),
					SeedDigest:                  testArtifactDigest("1"),
					EventLogDigest:              testArtifactDigest("2"),
					ExecutionPayloadDigest:      testPayloadDigest("3"),
				},
			},
		},
	}
}

func TestTerminalEventsCarryPhaseAppropriateEvidence(t *testing.T) {
	if _, err := NewExecutionEvent(completedEventPayloadFixture()); err != nil {
		t.Fatalf("completed event rejected: %v", err)
	}
	for _, kind := range []LifecycleEventKind{LifecycleEventFailed, LifecycleEventCancelled} {
		payload := completedEventPayloadFixture()
		payload.Kind = kind
		payload.Result = nil
		payload.Failure = &ExecutionFailure{Code: "provider-unavailable"}
		event, err := NewExecutionEvent(payload)
		if err != nil {
			t.Fatalf("%s without fabricated evidence rejected: %v", kind, err)
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		for _, privateOrFabricated := range []string{"placements", "runtimeDigest", "seedDigest", "eventLogDigest", "replay"} {
			if strings.Contains(string(encoded), privateOrFabricated) {
				t.Fatalf("%s event fabricated %q: %s", kind, privateOrFabricated, encoded)
			}
		}
	}
}

func TestPlacementCompetitionRanks(t *testing.T) {
	valid := completedEventPayloadFixture()
	if _, err := NewExecutionEvent(valid); err != nil {
		t.Fatalf("1,1,3 ranking rejected: %v", err)
	}

	for name, ranks := range map[string][]uint16{
		"dense after tie": {1, 1, 2},
		"gap without tie": {1, 3, 3},
	} {
		t.Run(name, func(t *testing.T) {
			payload := completedEventPayloadFixture()
			for index, rank := range ranks {
				payload.Result.Placements[index].Rank = rank
			}
			if _, err := NewExecutionEvent(payload); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("NewExecutionEvent() error = %v, want ErrInvalidExecution", err)
			}
		})
	}

	wrongTieOrder := completedEventPayloadFixture()
	wrongTieOrder.Result.Placements[0].SlotOrdinal = 1
	wrongTieOrder.Result.Placements[1].SlotOrdinal = 0
	if _, err := NewExecutionEvent(wrongTieOrder); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("tie slot order error = %v, want ErrInvalidExecution", err)
	}

	outOfRange := completedEventPayloadFixture()
	outOfRange.Result.Placements[2].SlotOrdinal = 9
	if _, err := NewExecutionEvent(outOfRange); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("out-of-range slot error = %v, want ErrInvalidExecution", err)
	}
}

func TestCompletedEventBindsEveryFrozenRequestSlotAndArtifact(t *testing.T) {
	request := mustExecutionRequest(t, providerRequestPayloadFixture())
	receipt := ExecutionReceipt{
		RequestID: request.ID, CommandID: request.CommandID,
		ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		ProviderInstanceID: "instance-1", Status: ReceiptAccepted,
	}
	payload := completedEventPayloadFixture()
	payload.Result.Placements = payload.Result.Placements[:2]
	payload.Result.Evidence.RecordedProvenance.ParticipantArtifactDigests = payload.Result.Evidence.RecordedProvenance.ParticipantArtifactDigests[:2]
	event, err := NewExecutionEvent(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateExecutionEventForExecution(event, request, receipt); err != nil {
		t.Fatalf("bound completed event rejected: %v", err)
	}

	for name, mutate := range map[string]func(*ExecutionEventPayload){
		"unknown entry": func(value *ExecutionEventPayload) {
			value.Result.Placements[0].EntryID = "unknown-entry"
		},
		"wrong slot": func(value *ExecutionEventPayload) {
			value.Result.Placements[0].SlotOrdinal, value.Result.Placements[1].SlotOrdinal = 1, 0
			value.Result.Placements[0], value.Result.Placements[1] = value.Result.Placements[1], value.Result.Placements[0]
		},
		"wrong artifact": func(value *ExecutionEventPayload) {
			value.Result.Evidence.RecordedProvenance.ParticipantArtifactDigests[0] = testArtifactDigest("9")
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := payload
			encoded, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if unmarshalErr := json.Unmarshal(encoded, &changed); unmarshalErr != nil {
				t.Fatal(unmarshalErr)
			}
			mutate(&changed)
			changedEvent, buildErr := NewExecutionEvent(changed)
			if buildErr != nil {
				t.Fatalf("mutation must remain a structurally valid event: %v", buildErr)
			}
			if err := ValidateExecutionEventForExecution(changedEvent, request, receipt); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("error = %v, want ErrInvalidExecution", err)
			}
		})
	}
}

func TestLifecycleReceiptIsQueuedFact(t *testing.T) {
	if err := ValidateLifecycleTransition(ExecutionStateAccepted, LifecycleEventStarted); err != nil {
		t.Fatalf("accepted to started rejected: %v", err)
	}
	if err := ValidateLifecycleTransition(ExecutionStateAccepted, LifecycleEventCompleted); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("result before start error = %v, want ErrInvalidTransition", err)
	}
	if err := ValidateLifecycleTransition(ExecutionStateAccepted, LifecycleEventCancelled); err != nil {
		t.Fatalf("pre-start cancellation rejected: %v", err)
	}
	if err := ValidateLifecycleTransition(ExecutionStateCompleted, LifecycleEventCancelled); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("post-terminal transition error = %v, want ErrInvalidTransition", err)
	}
}

func TestSHA256IdentityFormatIsStrict(t *testing.T) {
	for _, malformed := range []string{
		"",
		"sha256:a",
		"SHA256:" + strings.Repeat("a", 64),
		"sha256:" + strings.Repeat("A", 64),
		"sha256:" + strings.Repeat("g", 64),
		"sha256:" + strings.Repeat("a", 63),
		"sha256:" + strings.Repeat("a", 65),
	} {
		t.Run(malformed, func(t *testing.T) {
			requestPayload := providerRequestPayloadFixture()
			requestPayload.Profile.ProviderExecuted.Slots[0].Participant.ArtifactDigest = ArtifactDigest(malformed)
			if _, err := NewExecutionRequest(requestPayload); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("malformed request artifact error = %v, want ErrInvalidExecution", err)
			}

			eventPayload := completedEventPayloadFixture()
			eventPayload.Result.Evidence.RecordedProvenance.RuntimeDigest = ArtifactDigest(malformed)
			if _, err := NewExecutionEvent(eventPayload); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("malformed provenance error = %v, want ErrInvalidExecution", err)
			}
		})
	}
}
