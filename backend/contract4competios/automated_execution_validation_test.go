package contract4competios

import (
	"encoding/json"
	"errors"
	"testing"
)

func mustExecutionEvent(t *testing.T, payload ExecutionEventPayload) ExecutionEvent {
	t.Helper()
	event, err := NewExecutionEvent(payload)
	if err != nil {
		t.Fatalf("NewExecutionEvent() error = %v", err)
	}
	return event
}

func eventGrantFixtureForContractTest(t *testing.T, event ExecutionEvent, purpose GrantPurpose, scope GrantScope) (OperationGrant, OperationRouteBinding) {
	t.Helper()
	_, grant, _ := launchGrantFixture(t)
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	grant.Purpose, grant.Scope = purpose, scope
	grant.CompetitionID, grant.ContestID, grant.RequestID = event.CompetitionID, event.ContestID, event.RequestID
	grant.ProviderID, grant.AdapterID, grant.ProviderInstanceID = event.ProviderID, event.AdapterID, event.ProviderInstanceID
	grant.CommandID, grant.TypedPayloadDigest = event.CommandID, event.TypedPayloadDigest
	grant.RawTransportDigest = DigestRawTransportBody(grant.TransportContentType, body)
	grant.Resource = "/competios/events"
	return grant, routeBindingForGrant(grant)
}

func failedEventPayloadFixture(evidence *FailureEvidence) ExecutionEventPayload {
	payload := completedEventPayloadFixture()
	payload.ID = "failed-event"
	payload.Kind = LifecycleEventFailed
	payload.Result = nil
	payload.Failure = &ExecutionFailure{Code: "provider-stopped", AdjudicationCode: "runtime-fault", Evidence: evidence}
	return payload
}

func TestEventAcknowledgementAndGrantPurposeAreClosed(t *testing.T) {
	for _, status := range []EventAcknowledgementStatus{EventAcknowledgementAccepted, EventAcknowledgementReplayed} {
		if err := ValidateEventAcknowledgement(EventAcknowledgement{Status: status}); err != nil {
			t.Fatalf("defined acknowledgement %q rejected: %v", status, err)
		}
	}
	if err := ValidateEventAcknowledgement(EventAcknowledgement{Status: "stored"}); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("unknown acknowledgement error = %v, want ErrInvalidExecution", err)
	}

	startedPayload := completedEventPayloadFixture()
	startedPayload.ID, startedPayload.Kind, startedPayload.Result = "started-event", LifecycleEventStarted, nil
	started := mustExecutionEvent(t, startedPayload)
	completed := mustExecutionEvent(t, completedEventPayloadFixture())
	failed := mustExecutionEvent(t, failedEventPayloadFixture(nil))

	tests := []struct {
		name    string
		event   ExecutionEvent
		purpose GrantPurpose
		scope   GrantScope
	}{
		{name: "started", event: started, purpose: GrantPurposeContestStarted, scope: GrantScopeContestStarted},
		{name: "completed", event: completed, purpose: GrantPurposeContestResultSubmit, scope: GrantScopeContestResultSubmit},
		{name: "failed", event: failed, purpose: GrantPurposeContestResultSubmit, scope: GrantScopeContestResultSubmit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			grant, route := eventGrantFixtureForContractTest(t, test.event, test.purpose, test.scope)
			if err := ValidateEventGrantForEvent(VerifiedOperationGrant{Claims: grant}, route, test.event); err != nil {
				t.Fatalf("valid event grant rejected: %v", err)
			}

			wrongRoute := route
			wrongRoute.RawTransportDigest = testPayloadDigest("9")
			if err := ValidateEventGrantForEvent(VerifiedOperationGrant{Claims: grant}, wrongRoute, test.event); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("wrong raw event body error = %v, want ErrInvalidGrant", err)
			}

			foreignPurpose, foreignScope := GrantPurposeContestStarted, GrantScopeContestStarted
			if test.event.Kind == LifecycleEventStarted {
				foreignPurpose, foreignScope = GrantPurposeContestResultSubmit, GrantScopeContestResultSubmit
			}
			foreign, foreignRoute := eventGrantFixtureForContractTest(t, test.event, foreignPurpose, foreignScope)
			if err := ValidateOperationGrant(foreign); err != nil {
				t.Fatalf("foreign-purpose probe must remain structurally valid: %v", err)
			}
			if err := ValidateEventGrantForEvent(VerifiedOperationGrant{Claims: foreign}, foreignRoute, test.event); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("foreign-purpose event grant error = %v, want ErrInvalidGrant", err)
			}
		})
	}
}

func TestFailureEvidenceIsPhaseAppropriateAndFailClosed(t *testing.T) {
	valid := map[string]*FailureEvidence{
		"runtime only": {RuntimeDigest: testArtifactDigest("4")},
		"full available replay": {
			RuntimeDigest: testArtifactDigest("4"), EventLogDigest: testArtifactDigest("5"),
			ExecutionPayloadDigest: testPayloadDigest("6"),
			Replay:                 &TerminalReplay{State: ReplayAvailable, Reference: "replay:failure-1"},
		},
		"processing replay":  {Replay: &TerminalReplay{State: ReplayProcessing}},
		"unavailable replay": {Replay: &TerminalReplay{State: ReplayUnavailable}},
	}
	for name, evidence := range valid {
		t.Run("valid "+name, func(t *testing.T) {
			if _, err := NewExecutionEvent(failedEventPayloadFixture(evidence)); err != nil {
				t.Fatalf("phase-appropriate failure evidence rejected: %v", err)
			}
		})
	}

	invalid := map[string]*FailureEvidence{
		"empty evidence":        {},
		"malformed digest":      {RuntimeDigest: "sha256:a"},
		"available without ref": {Replay: &TerminalReplay{State: ReplayAvailable}},
		"processing with ref":   {Replay: &TerminalReplay{State: ReplayProcessing, Reference: "replay:too-early"}},
		"unavailable with ref":  {Replay: &TerminalReplay{State: ReplayUnavailable, Reference: "replay:fabricated"}},
		"unknown replay state":  {Replay: &TerminalReplay{State: "archived"}},
	}
	for name, evidence := range invalid {
		t.Run("invalid "+name, func(t *testing.T) {
			if _, err := NewExecutionEvent(failedEventPayloadFixture(evidence)); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("NewExecutionEvent() error = %v, want ErrInvalidExecution", err)
			}
		})
	}

	request := mustExecutionRequest(t, providerRequestPayloadFixture())
	receipt := ExecutionReceipt{
		RequestID: request.ID, CommandID: request.CommandID, ProviderID: request.ProviderID,
		AdapterID: request.AdapterID, ProviderInstanceID: "instance-1", Status: ReceiptAccepted,
	}
	bound := mustExecutionEvent(t, failedEventPayloadFixture(&FailureEvidence{RuntimeDigest: testArtifactDigest("4")}))
	if err := ValidateExecutionEventForExecution(bound, request, receipt); err != nil {
		t.Fatalf("request-bound failed event rejected: %v", err)
	}
}

func TestExecutionAndCompletionDiscriminatorsRejectCanonicalMutations(t *testing.T) {
	requestCases := map[string]func() ExecutionRequestPayload{
		"missing identity": func() ExecutionRequestPayload {
			payload := providerRequestPayloadFixture()
			payload.ID = ""
			return payload
		},
		"duplicate public artifact": func() ExecutionRequestPayload {
			payload := providerRequestPayloadFixture()
			payload.RequestedPublicArtifacts = []PublicArtifactKind{PublicArtifactTerminalReplay, PublicArtifactTerminalReplay}
			return payload
		},
		"deadline before not-before": func() ExecutionRequestPayload {
			payload := providerRequestPayloadFixture()
			payload.Profile.ProviderExecuted.Deadline = payload.Profile.ProviderExecuted.NotBefore.Add(-1)
			return payload
		},
		"scheduled empty slot": func() ExecutionRequestPayload {
			payload := scheduledRequestPayloadFixture()
			payload.Profile.ParticipantScheduled.Slots[0].Participants = nil
			return payload
		},
		"scheduled duplicate participant": func() ExecutionRequestPayload {
			payload := scheduledRequestPayloadFixture()
			payload.Profile.ParticipantScheduled.Slots[1].Participants[0] = "person-a"
			return payload
		},
		"unknown profile": func() ExecutionRequestPayload {
			payload := providerRequestPayloadFixture()
			payload.Profile = ExecutionProfile{Kind: "delegated"}
			return payload
		},
	}
	for name, fixture := range requestCases {
		t.Run("request "+name, func(t *testing.T) {
			if _, err := NewExecutionRequest(fixture()); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("NewExecutionRequest() error = %v, want ErrInvalidExecution", err)
			}
		})
	}

	for _, state := range []ReplayPublicationState{ReplayProcessing, ReplayUnavailable} {
		payload := completedEventPayloadFixture()
		payload.Result.Evidence.Replay = TerminalReplay{State: state}
		if _, err := NewExecutionEvent(payload); err != nil {
			t.Fatalf("reference-free %q replay state rejected: %v", state, err)
		}
	}

	completionCases := map[string]func(*ExecutionEventPayload){
		"malformed participant artifact": func(payload *ExecutionEventPayload) {
			payload.Result.Evidence.ProviderExecuted.ParticipantArtifactDigests[0] = "sha256:a"
		},
		"scheduled evidence with provider provenance": func(payload *ExecutionEventPayload) {
			payload.Result.Evidence = CompletionEvidence{
				ProfileKind: ExecutionProfileParticipantScheduled, Replay: TerminalReplay{State: ReplayUnavailable},
				ProviderExecuted: &RecordedProvenance{}, ParticipantScheduled: &ParticipantScheduledCompletionEvidence{},
			}
		},
		"unknown profile evidence": func(payload *ExecutionEventPayload) {
			payload.Result.Evidence = CompletionEvidence{ProfileKind: "delegated", Replay: TerminalReplay{State: ReplayUnavailable}}
		},
		"processing replay with reference": func(payload *ExecutionEventPayload) {
			payload.Result.Evidence.Replay = TerminalReplay{State: ReplayProcessing, Reference: "replay:not-ready"}
		},
		"unknown replay state": func(payload *ExecutionEventPayload) {
			payload.Result.Evidence.Replay = TerminalReplay{State: "archived"}
		},
	}
	for name, mutate := range completionCases {
		t.Run("completion "+name, func(t *testing.T) {
			payload := completedEventPayloadFixture()
			mutate(&payload)
			if _, err := NewExecutionEvent(payload); !errors.Is(err, ErrInvalidExecution) {
				t.Fatalf("NewExecutionEvent() error = %v, want ErrInvalidExecution", err)
			}
		})
	}
}

func TestSourceGrantPurposesRejectCrossPhaseClaimShapes(t *testing.T) {
	tests := map[string]func(*OperationGrant){
		"manifest with closure plan": func(grant *OperationGrant) {
			grant.ClosurePlanID = "foreign-plan"
		},
		"candidate with manifest path": func(grant *OperationGrant) {
			grant.ManifestPath = "bots/manifest.json"
		},
		"publication with repository": func(grant *OperationGrant) {
			grant.RepositoryNodeID = "foreign-repository"
		},
		"disclosure with candidate digest": func(grant *OperationGrant) {
			grant.CandidateTransferredBytesDigest = testArtifactDigest("8")
		},
	}
	fixtures := map[string]func(*testing.T) OperationGrant{
		"manifest with closure plan":       func(t *testing.T) OperationGrant { _, grant, _ := manifestGrantFixture(t); return grant },
		"candidate with manifest path":     func(t *testing.T) OperationGrant { _, grant, _ := candidateGrantFixture(t); return grant },
		"publication with repository":      func(t *testing.T) OperationGrant { _, grant, _ := publicationGrantFixture(t); return grant },
		"disclosure with candidate digest": func(t *testing.T) OperationGrant { _, grant, _ := disclosureGrantFixture(t); return grant },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			grant := fixtures[name](t)
			if err := ValidateOperationGrant(grant); err != nil {
				t.Fatalf("fixture grant rejected before mutation: %v", err)
			}
			mutate(&grant)
			if err := ValidateOperationGrant(grant); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("cross-phase claim error = %v, want ErrInvalidGrant", err)
			}
		})
	}
}

func TestSourceCanonicalDigestsPathsAndReceiptStatesFailClosed(t *testing.T) {
	for _, unsafePath := range []string{"", ".", "/bots/manifest.json", "../bots/manifest.json", "bots/../manifest.json", `bots\manifest.json`, "bots/manifest\n.json"} {
		t.Run("manifest path "+unsafePath, func(t *testing.T) {
			payload := manifestRequestFixture(t).Payload()
			payload.ManifestPath = unsafePath
			if _, err := NewManifestClosurePlanRequest(payload); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("unsafe manifest path %q error = %v, want ErrInvalidGrant", unsafePath, err)
			}
		})
	}
	if _, err := DigestCandidateTransferredFiles(nil); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("empty candidate transfer error = %v, want ErrInvalidGrant", err)
	}
	if _, err := DigestCandidateTransferredFiles([]CandidateSourceFile{{CanonicalPath: "bots/bot.star", EntryKind: "fifo", Bytes: []byte("bot")}}); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("unknown candidate entry kind error = %v, want ErrInvalidGrant", err)
	}

	t.Run("canonical digest tampering", func(t *testing.T) {
		manifest := manifestRequestFixture(t)
		manifest.ManifestByteLimit++
		if err := ValidateManifestClosurePlanRequest(manifest); !errors.Is(err, ErrInvalidGrant) {
			t.Fatalf("manifest digest tampering error = %v, want ErrInvalidGrant", err)
		}
		plan := closurePlanFixture(t)
		plan.AggregateByteLimit++
		if err := ValidateClosurePlan(plan); !errors.Is(err, ErrInvalidExecution) {
			t.Fatalf("plan digest tampering error = %v, want ErrInvalidExecution", err)
		}
		candidate := candidateRequestFixture(t)
		candidate.AggregateByteLimit++
		if err := ValidateCandidateClosureRetentionRequest(candidate); !errors.Is(err, ErrInvalidGrant) {
			t.Fatalf("candidate digest tampering error = %v, want ErrInvalidGrant", err)
		}
		disclosure := disclosureRequestFixture(t)
		disclosure.AggregateByteLimit++
		if err := ValidateArtifactDisclosureVerificationRequest(disclosure); !errors.Is(err, ErrInvalidGrant) {
			t.Fatalf("disclosure digest tampering error = %v, want ErrInvalidGrant", err)
		}
		publication := publicationRequestFixture(t)
		publication.ParticipantID = "other-participant"
		if err := ValidateArtifactPublicationRequest(publication); !errors.Is(err, ErrInvalidGrant) {
			t.Fatalf("publication digest tampering error = %v, want ErrInvalidGrant", err)
		}
	})

	manifest := manifestRequestFixture(t)
	plan := closurePlanFixture(t)
	planReceipt := ClosurePlanReceipt{
		ProviderID: plan.ProviderID, AdapterID: plan.AdapterID, CommandID: manifest.CommandID,
		ParticipantID: manifest.ParticipantID, ParticipantVersionID: manifest.ParticipantVersionID,
		RequestPayloadDigest: manifest.TypedPayloadDigest, Plan: plan, Status: "stored",
	}
	if err := ValidateClosurePlanReceiptForRequest(planReceipt, manifest); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("unknown closure-plan receipt status error = %v, want ErrInvalidExecution", err)
	}

	candidate := candidateRequestFixture(t)
	retention := ArtifactRetentionReceipt{
		ReceiptID: "retention-1", ProviderID: candidate.ProviderID, AdapterID: candidate.AdapterID,
		CommandID: candidate.CommandID, ParticipantID: candidate.ParticipantID, ParticipantVersionID: candidate.ParticipantVersionID,
		ClosurePlanID: candidate.ClosurePlanID, ClosurePlanDigest: candidate.ClosurePlanDigest,
		CandidateRequestDigest: candidate.TypedPayloadDigest, ArtifactDigest: testArtifactDigest("9"), Status: "stored",
	}
	if err := ValidateArtifactRetentionReceiptForRequest(retention, candidate); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("unknown retention receipt status error = %v, want ErrInvalidExecution", err)
	}

	disclosure := disclosureRequestFixture(t)
	disclosureReceipt := ArtifactDisclosureVerificationReceipt{
		ReceiptID: "disclosure-1", ProviderID: disclosure.ProviderID, AdapterID: disclosure.AdapterID,
		CommandID: disclosure.CommandID, ParticipantID: disclosure.ParticipantID, ParticipantVersionID: disclosure.ParticipantVersionID,
		RetentionReceiptID: disclosure.RetentionReceiptID, ArtifactDigest: disclosure.ArtifactDigest,
		VerificationRequestDigest: disclosure.TypedPayloadDigest, Verdict: "stored", VerifiedAt: contractTestTime,
	}
	if err := ValidateArtifactDisclosureVerificationReceiptForRequest(disclosureReceipt, disclosure); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("unknown disclosure receipt verdict error = %v, want ErrInvalidExecution", err)
	}

	publication := publicationRequestFixture(t)
	publicationReceipt := ArtifactPublicationReceipt{
		ReceiptID: "publication-1", ProviderID: publication.ProviderID, AdapterID: publication.AdapterID,
		CommandID: publication.CommandID, ParticipantID: publication.ParticipantID, ParticipantVersionID: publication.ParticipantVersionID,
		RetentionReceiptID: publication.RetentionReceiptID, DisclosureReceiptID: publication.DisclosureReceiptID,
		DisclosureRequestDigest: publication.DisclosureRequestDigest, PublicationRequestDigest: publication.TypedPayloadDigest,
		ArtifactDigest: publication.ArtifactDigest, PublishedAt: contractTestTime,
		PublicReference: "https://game.example/public/artifact-1", Status: "stored",
	}
	if err := ValidateArtifactPublicationReceiptForRequest(publicationReceipt, publication); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("unknown publication receipt status error = %v, want ErrInvalidExecution", err)
	}

	validDisclosureReceipt := disclosureReceipt
	validDisclosureReceipt.Verdict = ArtifactDisclosureMatched
	retention.Status = "stored"
	if err := ValidateArtifactPublicationPrerequisites(publication, retention, disclosure, validDisclosureReceipt); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("publication with unknown retention state error = %v, want ErrInvalidExecution", err)
	}
}
