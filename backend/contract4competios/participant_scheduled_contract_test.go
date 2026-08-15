package contract4competios

import (
	"errors"
	"strings"
	"testing"
)

func TestParticipantScheduledDisplayNamesAndTerminalEvidenceAreOptionalAndBound(t *testing.T) {
	legacy := mustExecutionRequest(t, scheduledRequestPayloadFixture())
	if got := legacy.Profile.ParticipantScheduled.Slots[0].DisplayName; got != "" {
		t.Fatalf("legacy scheduled display name = %q, want empty", got)
	}
	legacyEvent := participantScheduledCompletedEvent(t, legacy)
	if legacyEvent.Result.Evidence.ParticipantScheduled.TerminalStateDigest != "" || legacyEvent.Result.Evidence.ParticipantScheduled.EventLogDigest != "" {
		t.Fatalf("legacy scheduled evidence = %#v, want no optional terminal evidence", legacyEvent.Result.Evidence.ParticipantScheduled)
	}

	payload := scheduledRequestPayloadFixture()
	payload.Profile.ParticipantScheduled.Slots[0].DisplayName = "River City Ravens"
	payload.Profile.ParticipantScheduled.Slots[1].DisplayName = "Northside Foxes"
	request := mustExecutionRequest(t, payload)
	event := participantScheduledCompletedEvent(t, request)
	eventPayload := event.Payload()
	eventPayload.Result.Evidence.ParticipantScheduled.TerminalStateDigest = testArtifactDigest("8")
	eventPayload.Result.Evidence.ParticipantScheduled.EventLogDigest = testArtifactDigest("9")
	event = mustExecutionEvent(t, eventPayload)
	receipt := ExecutionReceipt{RequestID: request.ID, CommandID: request.CommandID, ProviderID: request.ProviderID, AdapterID: request.AdapterID, ProviderInstanceID: "scheduled-instance", Status: ReceiptAccepted}
	if err := ValidateExecutionEventForExecution(event, request, receipt); err != nil {
		t.Fatalf("display names and terminal evidence are not request-bound: %v", err)
	}

	request.Profile.ParticipantScheduled.Slots[0].DisplayName = "substituted label"
	if err := ValidateExecutionRequest(request); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("mutated display name error = %v, want ErrInvalidExecution", err)
	}
	event.Result.Evidence.ParticipantScheduled.EventLogDigest = testArtifactDigest("7")
	if err := ValidateExecutionEvent(event); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("mutated terminal evidence error = %v, want ErrInvalidExecution", err)
	}
}

func TestParticipantScheduledPresentationAndTerminalEvidenceFailClosed(t *testing.T) {
	requestCases := map[string]func(*ExecutionRequestPayload){
		"display name exceeds byte bound": func(payload *ExecutionRequestPayload) {
			payload.Profile.ParticipantScheduled.Slots[0].DisplayName = strings.Repeat("a", MaxParticipantScheduledSlotDisplayNameBytes+1)
		},
		"display name is invalid UTF-8": func(payload *ExecutionRequestPayload) {
			payload.Profile.ParticipantScheduled.Slots[0].DisplayName = string([]byte{0xff})
		},
	}
	for name, mutate := range requestCases {
		t.Run(name, func(t *testing.T) {
			payload := scheduledRequestPayloadFixture()
			mutate(&payload)
			if _, err := NewExecutionRequest(payload); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("NewExecutionRequest() error = %v, want ErrInvalidExecution", err)
			}
		})
	}

	request := mustExecutionRequest(t, scheduledRequestPayloadFixture())
	for name, mutate := range map[string]func(*ExecutionEventPayload){
		"bad terminal-state digest": func(payload *ExecutionEventPayload) {
			payload.Result.Evidence.ParticipantScheduled.TerminalStateDigest = "sha256:bad"
		},
		"bad event-log digest": func(payload *ExecutionEventPayload) {
			payload.Result.Evidence.ParticipantScheduled.EventLogDigest = "sha256:bad"
		},
	} {
		t.Run(name, func(t *testing.T) {
			payload := participantScheduledCompletedEvent(t, request).Payload()
			mutate(&payload)
			if _, err := NewExecutionEvent(payload); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("NewExecutionEvent() error = %v, want ErrInvalidExecution", err)
			}
		})
	}
}

func participantScheduledCompletedEvent(t *testing.T, request ExecutionRequest) ExecutionEvent {
	t.Helper()
	payload := completedEventPayloadFixture()
	payload.Result.Evidence = CompletionEvidence{
		ProfileKind:          ExecutionProfileParticipantScheduled,
		Replay:               TerminalReplay{State: ReplayAvailable, Reference: "replay:scheduled-terminal"},
		ParticipantScheduled: &ParticipantScheduledCompletionEvidence{},
	}
	payload.Result.Placements = []Placement{
		{SlotOrdinal: 0, EntryID: request.Profile.ParticipantScheduled.Slots[0].EntryID, Rank: 1, Status: PlacementStatusFinished},
		{SlotOrdinal: 1, EntryID: request.Profile.ParticipantScheduled.Slots[1].EntryID, Rank: 2, Status: PlacementStatusFinished},
	}
	payload.CompetitionID, payload.ContestID, payload.RequestID = request.CompetitionID, request.ContestID, request.ID
	payload.ProviderID, payload.AdapterID, payload.ProviderInstanceID = request.ProviderID, request.AdapterID, "scheduled-instance"
	return mustExecutionEvent(t, payload)
}
