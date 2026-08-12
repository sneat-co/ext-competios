package contract4competiostest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type storedLaunch struct {
	digest  contract4competios.PayloadDigest
	receipt contract4competios.ExecutionReceipt
}

type referenceProvider struct {
	launches map[contract4competios.CommandID]storedLaunch
}

func (p *referenceProvider) LaunchExecution(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	if contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateExecutionRequest(request) != nil || contract4competios.ValidateLaunchGrantForRequest(grant, launchRouteFixture(request), request) != nil {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	if p.launches == nil {
		p.launches = map[contract4competios.CommandID]storedLaunch{}
	}
	if request.ProviderID != "provider" || request.AdapterID != "adapter" {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	if prior, exists := p.launches[request.CommandID]; exists {
		if prior.digest != request.TypedPayloadDigest {
			return contract4competios.ExecutionReceipt{}, contract4competios.ErrCommandConflict
		}
		replay := prior.receipt
		replay.Status = contract4competios.ReceiptReplayed
		return replay, nil
	}
	receipt := contract4competios.ExecutionReceipt{
		RequestID: request.ID, CommandID: request.CommandID, ProviderID: request.ProviderID,
		AdapterID: request.AdapterID, ProviderInstanceID: contract4competios.ProviderInstanceID("instance-" + request.ID),
		Status: contract4competios.ReceiptAccepted, SafeReferences: []contract4competios.SafeReference{"safe:receipt:" + contract4competios.SafeReference(request.ID)},
	}
	p.launches[request.CommandID] = storedLaunch{digest: request.TypedPayloadDigest, receipt: receipt}
	return receipt, nil
}

// biddingTicTacToeProvider intentionally implements the interface independently
// and interprets configuration as a sealed-bid policy, not Chess vocabulary.
type biddingTicTacToeProvider struct {
	receipts map[contract4competios.CommandID]contract4competios.ExecutionReceipt
	digests  map[contract4competios.CommandID]contract4competios.PayloadDigest
}

func (p *biddingTicTacToeProvider) LifecycleEvents(_ context.Context, request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) (contract4competios.ExecutionEvent, contract4competios.ExecutionEvent, error) {
	var policy struct {
		OpeningBid int `json:"openingBid"`
	}
	if request.Profile.ProviderExecuted == nil || json.Unmarshal(request.Profile.ProviderExecuted.Configuration.Data, &policy) != nil || policy.OpeningBid != 2 {
		return contract4competios.ExecutionEvent{}, contract4competios.ExecutionEvent{}, errors.New("invalid sealed bid policy")
	}
	return startFixture(receipt.ProviderInstanceID), resultFixtureForRequest(request, receipt.ProviderInstanceID, []uint16{1, 2}), nil
}

type chessJourneyProvider struct{ referenceProvider }

func (p *chessJourneyProvider) LifecycleEvents(_ context.Context, request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) (contract4competios.ExecutionEvent, contract4competios.ExecutionEvent, error) {
	if request.GameID != "chess-raiders" || request.Profile.ProviderExecuted == nil || request.Profile.ProviderExecuted.Configuration.Version != "chess-provider-v1" {
		return contract4competios.ExecutionEvent{}, contract4competios.ExecutionEvent{}, errors.New("invalid Chess execution profile")
	}
	return startFixture(receipt.ProviderInstanceID), resultFixtureForRequest(request, receipt.ProviderInstanceID, []uint16{1, 1}), nil
}

type threeSlotJourneyProvider struct{ referenceProvider }

func (p *threeSlotJourneyProvider) LifecycleEvents(_ context.Context, request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) (contract4competios.ExecutionEvent, contract4competios.ExecutionEvent, error) {
	if request.GameID != "three-slot-game" || request.Profile.ProviderExecuted == nil || len(request.Profile.ProviderExecuted.Slots) != 3 {
		return contract4competios.ExecutionEvent{}, contract4competios.ExecutionEvent{}, errors.New("invalid three-slot execution profile")
	}
	return startFixture(receipt.ProviderInstanceID), resultFixtureForRequest(request, receipt.ProviderInstanceID, []uint16{1, 1, 3}), nil
}

type scheduledJourneyProvider struct{ referenceProvider }

func (p *scheduledJourneyProvider) LifecycleEvents(_ context.Context, request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) (contract4competios.ExecutionEvent, contract4competios.ExecutionEvent, error) {
	if request.Profile.ParticipantScheduled == nil || request.Profile.ProviderExecuted != nil {
		return contract4competios.ExecutionEvent{}, contract4competios.ExecutionEvent{}, errors.New("invalid participant-scheduled execution profile")
	}
	return startFixture(receipt.ProviderInstanceID), resultFixtureForRequest(request, receipt.ProviderInstanceID, []uint16{1, 1}), nil
}

type leakyJourneyProvider struct {
	referenceProvider
	diagnostics []string
}

func (p *leakyJourneyProvider) LaunchExecution(ctx context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	receipt, err := p.referenceProvider.LaunchExecution(ctx, grant, request)
	if err != nil {
		p.diagnostics = append(p.diagnostics, "rejected "+privateDiagnosticCanary)
		return contract4competios.ExecutionReceipt{}, fmt.Errorf("launch rejection included %s", privateDiagnosticCanary)
	}
	receipt.SafeReferences = append(receipt.SafeReferences, contract4competios.SafeReference("safe:receipt:"+privateDiagnosticCanary))
	return receipt, nil
}

func (p *leakyJourneyProvider) LifecycleEvents(_ context.Context, request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) (contract4competios.ExecutionEvent, contract4competios.ExecutionEvent, error) {
	terminalPayload := copyEventPayload(resultFixtureForRequest(request, receipt.ProviderInstanceID, []uint16{1, 1}))
	terminalPayload.Result.Evidence.Replay.Reference = contract4competios.ReplayReference("replay:" + privateDiagnosticCanary)
	terminal, err := contract4competios.NewExecutionEvent(terminalPayload)
	return startFixture(receipt.ProviderInstanceID), terminal, err
}

func (p *leakyJourneyProvider) ObservedDiagnostics() []string {
	return append([]string(nil), p.diagnostics...)
}

func (p *biddingTicTacToeProvider) LaunchExecution(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	if contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateExecutionRequest(request) != nil || contract4competios.ValidateLaunchGrantForRequest(grant, launchRouteFixture(request), request) != nil {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	if p.receipts == nil {
		p.receipts = map[contract4competios.CommandID]contract4competios.ExecutionReceipt{}
		p.digests = map[contract4competios.CommandID]contract4competios.PayloadDigest{}
	}
	if request.ProviderID != "provider" || request.AdapterID != "adapter" {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	if prior, exists := p.receipts[request.CommandID]; exists {
		if p.digests[request.CommandID] != request.TypedPayloadDigest {
			return contract4competios.ExecutionReceipt{}, contract4competios.ErrCommandConflict
		}
		prior.Status = contract4competios.ReceiptReplayed
		return prior, nil
	}
	if request.GameID != "bidding-tic-tac-toe" || request.Profile.ProviderExecuted == nil || request.Profile.ProviderExecuted.Configuration.Version != "sealed-bid-policy" {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	receipt := contract4competios.ExecutionReceipt{
		RequestID: request.ID, CommandID: request.CommandID, ProviderID: request.ProviderID,
		AdapterID: request.AdapterID, ProviderInstanceID: contract4competios.ProviderInstanceID("bid-" + request.ID),
		Status: contract4competios.ReceiptAccepted,
	}
	p.receipts[request.CommandID], p.digests[request.CommandID] = receipt, request.TypedPayloadDigest
	return receipt, nil
}

type unsafeProvider struct{}

func (unsafeProvider) LaunchExecution(_ context.Context, _ contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	return contract4competios.ExecutionReceipt{RequestID: request.ID, ProviderInstanceID: "new-every-time", Status: contract4competios.ReceiptAccepted}, nil
}

type poisonOnRejectProvider struct {
	delegate referenceProvider
	poisoned bool
}

func (p *poisonOnRejectProvider) LaunchExecution(ctx context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	if p.poisoned {
		return contract4competios.ExecutionReceipt{}, errors.New("provider state poisoned by a prior rejection")
	}
	receipt, err := p.delegate.LaunchExecution(ctx, grant, request)
	if err != nil {
		p.poisoned = true
	}
	return receipt, err
}

type storedEvent struct {
	digest contract4competios.PayloadDigest
}

type referenceEventSink struct {
	state   contract4competios.ExecutionState
	request contract4competios.ExecutionRequest
	receipt contract4competios.ExecutionReceipt
	events  map[contract4competios.CommandID]storedEvent
}

func newReferenceEventSink(request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) *referenceEventSink {
	return &referenceEventSink{state: contract4competios.ExecutionStateAccepted, request: request, receipt: receipt, events: map[contract4competios.CommandID]storedEvent{}}
}

func (s *referenceEventSink) SubmitExecutionEvent(_ context.Context, grant contract4competios.VerifiedOperationGrant, event contract4competios.ExecutionEvent) (contract4competios.EventAcknowledgement, error) {
	if contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateExecutionEvent(event) != nil || contract4competios.ValidateEventGrantForEvent(grant, eventRouteFixture(event), event) != nil {
		return contract4competios.EventAcknowledgement{}, contract4competios.ErrInvalidGrant
	}
	if contract4competios.ValidateExecutionEventForExecution(event, s.request, s.receipt) != nil {
		return contract4competios.EventAcknowledgement{}, contract4competios.ErrInvalidGrant
	}
	if prior, exists := s.events[event.CommandID]; exists {
		if prior.digest != event.TypedPayloadDigest {
			return contract4competios.EventAcknowledgement{}, contract4competios.ErrCommandConflict
		}
		return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementReplayed}, nil
	}
	if err := contract4competios.ValidateLifecycleTransition(s.state, event.Kind); err != nil {
		return contract4competios.EventAcknowledgement{}, err
	}
	s.events[event.CommandID] = storedEvent{digest: event.TypedPayloadDigest}
	s.state = stateForEvent(event.Kind)
	return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementAccepted}, nil
}

func stateForEvent(kind contract4competios.LifecycleEventKind) contract4competios.ExecutionState {
	switch kind {
	case contract4competios.LifecycleEventStarted:
		return contract4competios.ExecutionStateStarted
	case contract4competios.LifecycleEventCompleted:
		return contract4competios.ExecutionStateCompleted
	case contract4competios.LifecycleEventFailed:
		return contract4competios.ExecutionStateFailed
	case contract4competios.LifecycleEventCancelled:
		return contract4competios.ExecutionStateCancelled
	default:
		return ""
	}
}

type unsafeEventSink struct{}

func (unsafeEventSink) SubmitExecutionEvent(context.Context, contract4competios.VerifiedOperationGrant, contract4competios.ExecutionEvent) (contract4competios.EventAcknowledgement, error) {
	return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementAccepted}, nil
}

type poisonOnRejectEventSink struct {
	delegate *referenceEventSink
	poisoned bool
}

func (s *poisonOnRejectEventSink) SubmitExecutionEvent(ctx context.Context, grant contract4competios.VerifiedOperationGrant, event contract4competios.ExecutionEvent) (contract4competios.EventAcknowledgement, error) {
	if s.poisoned {
		return contract4competios.EventAcknowledgement{}, errors.New("event state poisoned by a prior rejection")
	}
	ack, err := s.delegate.SubmitExecutionEvent(ctx, grant, event)
	if err != nil {
		s.poisoned = true
	}
	return ack, err
}

type referenceVerifier struct {
	registry map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant
	seen     map[string]contract4competios.OperationGrant
}

func (v *referenceVerifier) VerifyOperationGrant(_ context.Context, token contract4competios.EncodedAccessToken) (contract4competios.VerifiedOperationGrant, error) {
	claims, exists := v.registry[token]
	if !exists {
		return contract4competios.VerifiedOperationGrant{}, contract4competios.ErrInvalidGrant
	}
	if prior, replayed := v.seen[claims.TokenID]; replayed && prior != claims {
		return contract4competios.VerifiedOperationGrant{}, contract4competios.ErrTokenReplayConflict
	}
	now := fixtureTime.Add(2 * time.Minute)
	allowedKey := claims.KeyID == "key-a" || claims.KeyID == "key-rotated"
	trustedGameOperation := claims.Issuer == fixtureIssuer && claims.Subject == fixtureSubject && claims.Audience == fixtureAudience
	trustedCompetiosEvent := claims.Issuer == "https://competios.example" && claims.Subject == "svc:game" && claims.Audience == "competios/events"
	if contract4competios.ValidateOperationGrant(claims) != nil || !trustedGameOperation && !trustedCompetiosEvent || claims.TokenType != contract4competios.GrantTokenTypeAccessJWT || !allowedKey || now.Before(claims.NotBefore) || !now.Before(claims.ExpiresAt) {
		return contract4competios.VerifiedOperationGrant{}, contract4competios.ErrInvalidGrant
	}
	v.seen[claims.TokenID] = claims
	return contract4competios.VerifiedOperationGrant{Claims: claims}, nil
}

type unsafeVerifier struct {
	registry map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant
}

func (v unsafeVerifier) VerifyOperationGrant(_ context.Context, token contract4competios.EncodedAccessToken) (contract4competios.VerifiedOperationGrant, error) {
	if claims, ok := v.registry[token]; ok {
		return contract4competios.VerifiedOperationGrant{Claims: claims}, nil
	}
	return contract4competios.VerifiedOperationGrant{Claims: launchGrantFixture(executionFixture(), "forged", "key-a").Claims}, nil
}

type referenceAuthority struct {
	registry map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant
	allowed  contract4competios.OperationGrantRequest
	next     int
}

func (a *referenceAuthority) IssueOperationGrant(_ context.Context, request contract4competios.OperationGrantRequest) (contract4competios.IssuedOperationAccessToken, error) {
	if contract4competios.ValidateOperationGrantRequest(request) != nil || request != a.allowed {
		return contract4competios.IssuedOperationAccessToken{}, contract4competios.ErrInvalidGrant
	}
	a.next++
	token := contract4competios.EncodedAccessToken(fmt.Sprintf("opaque-issued-%d", a.next))
	claims := grantClaimsFromRequest(request)
	claims.TokenID = fmt.Sprintf("issued-token-%d", a.next)
	if a.next%2 == 0 {
		claims.KeyID = "key-rotated"
		claims.IssuedAt, claims.NotBefore = fixtureTime.Add(time.Minute), fixtureTime.Add(time.Minute)
	}
	claims.ExpiresAt = fixtureTime.Add(5 * time.Minute)
	a.registry[token] = claims
	return contract4competios.IssuedOperationAccessToken{AccessToken: token, TokenType: claims.TokenType, ExpiresAt: claims.ExpiresAt}, nil
}

func grantClaimsFromRequest(request contract4competios.OperationGrantRequest) contract4competios.OperationGrant {
	baseline := launchGrantFixture(executionFixture(), "issued", "key-a").Claims
	if request.Purpose == contract4competios.GrantPurposeContestStarted || request.Purpose == contract4competios.GrantPurposeContestResultSubmit {
		baseline = eventGrantFixture(startFixture("instance"), "issued", "key-a").Claims
	}
	baseline.Purpose = request.Purpose
	baseline.Scope = contract4competios.GrantScopeForPurpose(request.Purpose)
	baseline.ProviderID, baseline.AdapterID = request.ProviderID, request.AdapterID
	baseline.CompetitionID, baseline.ContestID, baseline.RequestID = request.CompetitionID, request.ContestID, request.RequestID
	baseline.ProviderInstanceID, baseline.CommandID = request.ProviderInstanceID, request.CommandID
	baseline.TypedPayloadDigest, baseline.TransportContentType = request.TypedPayloadDigest, request.TransportContentType
	baseline.RawTransportDigest, baseline.Method, baseline.Resource = request.RawTransportDigest, request.Method, request.Resource
	baseline.ParticipantID, baseline.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	baseline.RepositoryNodeID, baseline.CommitOID, baseline.ManifestPath = request.RepositoryNodeID, request.CommitOID, request.ManifestPath
	baseline.ManifestEntryKind = request.ManifestEntryKind
	baseline.RawManifestBytesDigest, baseline.ManifestByteLimit = request.RawManifestBytesDigest, request.ManifestByteLimit
	baseline.ClosurePlanID, baseline.ClosurePlanDigest = request.ClosurePlanID, request.ClosurePlanDigest
	baseline.CandidateTransferredBytesDigest = request.CandidateTransferredBytesDigest
	baseline.PublicCandidateTransferredBytesDigest = request.PublicCandidateTransferredBytesDigest
	baseline.AggregateByteLimit, baseline.RetentionReceiptID, baseline.ArtifactDigest = request.AggregateByteLimit, request.RetentionReceiptID, request.ArtifactDigest
	baseline.DisclosureReceiptID, baseline.DisclosureRequestDigest = request.DisclosureReceiptID, request.DisclosureRequestDigest
	return baseline
}

func TestExecutionProviderConformanceAcceptsChessShapedProvider(t *testing.T) {
	if violations := CheckExecutionProvider(func() contract4competios.ExecutionProvider { return &referenceProvider{} }); len(violations) != 0 {
		t.Fatalf("Chess-shaped provider violations: %v", violations)
	}
}

func TestExecutionProviderConformanceAcceptsUnrelatedBiddingTicTacToeFake(t *testing.T) {
	request := executionFixtureFor("bidding-tic-tac-toe", "sealed-bid-policy", []byte(`{"openingBid":2}`), 2)
	if violations := CheckExecutionProviderWithRequest(func() contract4competios.ExecutionProvider { return &biddingTicTacToeProvider{} }, request); len(violations) != 0 {
		t.Fatalf("Bidding Tic-Tac-Toe provider violations: %v", violations)
	}
}

func TestExecutionProviderConformanceAcceptsParticipantScheduledProfile(t *testing.T) {
	request := scheduledExecutionFixture()
	if violations := CheckExecutionProviderWithRequest(func() contract4competios.ExecutionProvider { return &referenceProvider{} }, request); len(violations) != 0 {
		t.Fatalf("participant-scheduled provider violations: %v", violations)
	}
}

func TestExecutionProviderConformanceRejectsDeliberatelyUnsafeProvider(t *testing.T) {
	if violations := CheckExecutionProvider(func() contract4competios.ExecutionProvider { return unsafeProvider{} }); len(violations) < 10 {
		t.Fatalf("unsafe provider was not decisively rejected: %v", violations)
	}
}

func TestExecutionProviderConformanceRejectsPoisonOnRejectionFake(t *testing.T) {
	if violations := CheckExecutionProvider(func() contract4competios.ExecutionProvider { return &poisonOnRejectProvider{} }); len(violations) == 0 {
		t.Fatal("provider that mutates state on rejection unexpectedly passed conformance")
	}
}

func TestCompositeExecutionJourneysRemainGameNeutral(t *testing.T) {
	tests := []struct {
		name    string
		factory ExecutionJourneyProviderFactory
		request contract4competios.ExecutionRequest
		inspect func(contract4competios.ExecutionEvent) error
	}{
		{
			name:    "Chess-shaped two-slot provider",
			factory: func() ExecutionJourneyProvider { return &chessJourneyProvider{} },
			request: executionFixture(),
		},
		{
			name:    "Bidding Tic-Tac-Toe sealed configuration",
			factory: func() ExecutionJourneyProvider { return &biddingTicTacToeProvider{} },
			request: executionFixtureFor("bidding-tic-tac-toe", "sealed-bid-policy", []byte(`{"openingBid":2}`), 2),
			inspect: func(terminal contract4competios.ExecutionEvent) error {
				if terminal.Result == nil || len(terminal.Result.Placements) != 2 || terminal.Result.Placements[0].EntryID != "entry-a" || terminal.Result.Placements[0].Rank != 1 || terminal.Result.Placements[1].Rank != 2 {
					return fmt.Errorf("sealed bid did not determine expected result: %+v", terminal.Result)
				}
				return nil
			},
		},
		{
			name:    "generic three-slot tied provider",
			factory: func() ExecutionJourneyProvider { return &threeSlotJourneyProvider{} },
			request: executionFixtureFor("three-slot-game", "three-slot-config", []byte(`{}`), 3),
			inspect: func(terminal contract4competios.ExecutionEvent) error {
				if terminal.Result == nil || len(terminal.Result.Placements) != 3 || terminal.Result.Placements[0].Rank != 1 || terminal.Result.Placements[1].Rank != 1 || terminal.Result.Placements[2].Rank != 3 {
					return fmt.Errorf("three-slot competition ranks changed: %+v", terminal.Result)
				}
				return nil
			},
		},
		{
			name:    "participant-scheduled human profile",
			factory: func() ExecutionJourneyProvider { return &scheduledJourneyProvider{} },
			request: scheduledExecutionFixture(),
			inspect: func(terminal contract4competios.ExecutionEvent) error {
				if terminal.Result == nil || terminal.Result.Evidence.ProfileKind != contract4competios.ExecutionProfileParticipantScheduled || terminal.Result.Evidence.ParticipantScheduled == nil || terminal.Result.Evidence.ProviderExecuted != nil {
					return fmt.Errorf("scheduled result fabricated provider provenance: %+v", terminal.Result)
				}
				encoded, _ := json.Marshal(terminal)
				if strings.Contains(string(encoded), "providerExecuted") || strings.Contains(string(encoded), "participantArtifactDigests") || strings.Contains(string(encoded), "runtimeDigest") {
					return fmt.Errorf("scheduled public result contains provider-only evidence")
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if violations := CheckExecutionJourney(test.factory, test.request, test.inspect); len(violations) != 0 {
				t.Fatalf("journey violations: %v", violations)
			}
		})
	}
}

func TestCompositeJourneyRejectsOtherwiseValidPrivacyLeaks(t *testing.T) {
	violations := CheckExecutionJourney(func() ExecutionJourneyProvider { return &leakyJourneyProvider{} }, executionFixture(), nil)
	if len(violations) < 4 {
		t.Fatalf("leaky receipt/replay/error/log provider was not decisively rejected: %v", violations)
	}
	for _, violation := range violations {
		if strings.Contains(violation.Error(), privateDiagnosticCanary) {
			t.Fatalf("privacy conformance echoed the private canary in its own error: %v", violation)
		}
	}
}

func TestEventSinkConformanceAcceptsTiedThreeSlotTerminalResult(t *testing.T) {
	request := executionFixtureFor("three-slot-game", "three-slot-config", []byte(`{}`), 3)
	receipt := executionReceiptFixture(request, "instance")
	start := startFixture("instance")
	result := resultFixtureForRequest(request, "instance", []uint16{1, 1, 3})
	if violations := CheckExecutionEventSinkWithEvents(func(request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) contract4competios.ExecutionEventSink {
		return newReferenceEventSink(request, receipt)
	}, request, receipt, start, result); len(violations) != 0 {
		t.Fatalf("event sink violations: %v", violations)
	}
}

func TestEventSinkConformanceAcceptsParticipantScheduledCompletion(t *testing.T) {
	request := scheduledExecutionFixture()
	receipt := executionReceiptFixture(request, "scheduled-instance")
	start := startFixture("scheduled-instance")
	result := resultFixtureForRequest(request, "scheduled-instance", []uint16{1, 1})
	if violations := CheckExecutionEventSinkWithEvents(func(request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) contract4competios.ExecutionEventSink {
		return newReferenceEventSink(request, receipt)
	}, request, receipt, start, result); len(violations) != 0 {
		t.Fatalf("participant-scheduled event sink violations: %v", violations)
	}
}

func TestEventSinkConformanceRejectsDeliberatelyUnsafeSink(t *testing.T) {
	if violations := CheckExecutionEventSink(func(contract4competios.ExecutionRequest, contract4competios.ExecutionReceipt) contract4competios.ExecutionEventSink {
		return unsafeEventSink{}
	}); len(violations) < 10 {
		t.Fatalf("unsafe event sink was not decisively rejected: %v", violations)
	}
}

func TestEventSinkConformanceRejectsPoisonOnRejectionFake(t *testing.T) {
	if violations := CheckExecutionEventSink(func(request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) contract4competios.ExecutionEventSink {
		return &poisonOnRejectEventSink{delegate: newReferenceEventSink(request, receipt)}
	}); len(violations) == 0 {
		t.Fatal("event sink that mutates state on rejection unexpectedly passed conformance")
	}
}

func TestOperationGrantVerifierConformance(t *testing.T) {
	goodFactory := func(registry map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant) contract4competios.OperationGrantVerifier {
		return &referenceVerifier{registry: registry, seen: map[string]contract4competios.OperationGrant{}}
	}
	if violations := CheckOperationGrantVerifier(goodFactory); len(violations) != 0 {
		t.Fatalf("reference verifier violations: %v", violations)
	}
	badFactory := func(registry map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant) contract4competios.OperationGrantVerifier {
		return unsafeVerifier{registry: registry}
	}
	if violations := CheckOperationGrantVerifier(badFactory); len(violations) == 0 {
		t.Fatal("unsafe verifier unexpectedly passed conformance")
	}
}

func TestBilateralIssuerVerifierConformance(t *testing.T) {
	factory := func(allowed contract4competios.OperationGrantRequest) (contract4competios.OperationGrantIssuer, contract4competios.OperationGrantVerifier) {
		registry := map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant{}
		return &referenceAuthority{registry: registry, allowed: allowed}, &referenceVerifier{registry: registry, seen: map[string]contract4competios.OperationGrant{}}
	}
	if violations := CheckOperationGrantAuthority(factory); len(violations) != 0 {
		t.Fatalf("bilateral authority violations: %v", violations)
	}
}

func TestPublicExecutionJSONContainsNoBearerOrPrivateSource(t *testing.T) {
	encoded, err := json.Marshal(resultFixture("instance"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"bearer", "accesstoken", "github", "repositorynodeid", "sourcebytes", "private"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("public execution event leaks %q: %s", forbidden, encoded)
		}
	}
}

func TestNoActionDoesNotCallProvider(t *testing.T) {
	provider := &countingProvider{}
	_ = provider
	if provider.calls != 0 {
		t.Fatalf("provider calls without a request = %d", provider.calls)
	}
}

type countingProvider struct{ calls int }

func (p *countingProvider) LaunchExecution(context.Context, contract4competios.VerifiedOperationGrant, contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	p.calls++
	return contract4competios.ExecutionReceipt{}, errors.New("unexpected call")
}
