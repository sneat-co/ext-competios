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
	if prior, exists := p.launches[request.CommandID]; exists {
		if prior.digest != request.TypedPayloadDigest {
			return contract4competios.ExecutionReceipt{}, contract4competios.ErrCommandConflict
		}
		replay := prior.receipt
		replay.Status = contract4competios.ReceiptReplayed
		return replay, nil
	}
	if request.ProviderID != "provider" || request.AdapterID != "adapter" {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	receipt := contract4competios.ExecutionReceipt{
		RequestID: request.ID, CommandID: request.CommandID, ProviderID: request.ProviderID,
		AdapterID: request.AdapterID, ProviderInstanceID: contract4competios.ProviderInstanceID("instance-" + request.ID),
		Status: contract4competios.ReceiptAccepted, SafeReferences: []string{"receipt:" + string(request.ID)},
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

func (p *biddingTicTacToeProvider) LaunchExecution(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	if contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateExecutionRequest(request) != nil || contract4competios.ValidateLaunchGrantForRequest(grant, launchRouteFixture(request), request) != nil {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	if p.receipts == nil {
		p.receipts = map[contract4competios.CommandID]contract4competios.ExecutionReceipt{}
		p.digests = map[contract4competios.CommandID]contract4competios.PayloadDigest{}
	}
	if prior, exists := p.receipts[request.CommandID]; exists {
		if p.digests[request.CommandID] != request.TypedPayloadDigest {
			return contract4competios.ExecutionReceipt{}, contract4competios.ErrCommandConflict
		}
		prior.Status = contract4competios.ReceiptReplayed
		return prior, nil
	}
	if request.ProviderID != "provider" || request.AdapterID != "adapter" || request.GameID != "bidding-tic-tac-toe" || request.Profile.ProviderExecuted == nil || request.Profile.ProviderExecuted.Configuration.Version != "sealed-bid-policy" {
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
	if prior, exists := s.events[event.CommandID]; exists {
		if prior.digest != event.TypedPayloadDigest {
			return contract4competios.EventAcknowledgement{}, contract4competios.ErrCommandConflict
		}
		return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementReplayed}, nil
	}
	if contract4competios.ValidateExecutionEventForExecution(event, s.request, s.receipt) != nil {
		return contract4competios.EventAcknowledgement{}, contract4competios.ErrInvalidGrant
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
	if contract4competios.ValidateOperationGrant(claims) != nil || claims.Issuer != fixtureIssuer || claims.Subject != fixtureSubject || claims.Audience != fixtureAudience || claims.TokenType != contract4competios.GrantTokenTypeAccessJWT || !allowedKey || now.Before(claims.NotBefore) || !now.Before(claims.ExpiresAt) {
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
	registry       map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant
	allowedPurpose contract4competios.GrantPurpose
	next           int
}

func (a *referenceAuthority) IssueOperationGrant(_ context.Context, request contract4competios.OperationGrantRequest) (contract4competios.IssuedOperationAccessToken, error) {
	if contract4competios.ValidateOperationGrantRequest(request) != nil || request.Purpose != a.allowedPurpose {
		return contract4competios.IssuedOperationAccessToken{}, contract4competios.ErrInvalidGrant
	}
	a.next++
	token := contract4competios.EncodedAccessToken(fmt.Sprintf("opaque-issued-%d", a.next))
	claims := grantClaimsFromRequest(request)
	claims.TokenID = fmt.Sprintf("issued-token-%d", a.next)
	a.registry[token] = claims
	return contract4competios.IssuedOperationAccessToken{AccessToken: token, TokenType: claims.TokenType, ExpiresAt: claims.ExpiresAt}, nil
}

func grantClaimsFromRequest(request contract4competios.OperationGrantRequest) contract4competios.OperationGrant {
	baseline := launchGrantFixture(executionFixture(), "issued", "key-a").Claims
	baseline.Purpose = request.Purpose
	baseline.Scope = scopeForPurpose(request.Purpose)
	baseline.ProviderID, baseline.AdapterID = request.ProviderID, request.AdapterID
	baseline.CompetitionID, baseline.ContestID, baseline.RequestID = request.CompetitionID, request.ContestID, request.RequestID
	baseline.ProviderInstanceID, baseline.CommandID = request.ProviderInstanceID, request.CommandID
	baseline.TypedPayloadDigest, baseline.TransportContentType = request.TypedPayloadDigest, request.TransportContentType
	baseline.RawTransportDigest, baseline.Method, baseline.Resource = request.RawTransportDigest, request.Method, request.Resource
	baseline.ParticipantID, baseline.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	baseline.RepositoryNodeID, baseline.Commit, baseline.ManifestPath = request.RepositoryNodeID, request.Commit, request.ManifestPath
	baseline.ManifestEntryKind = request.ManifestEntryKind
	baseline.RawManifestBytesDigest, baseline.ManifestByteLimit = request.RawManifestBytesDigest, request.ManifestByteLimit
	baseline.ClosurePlanID, baseline.ClosurePlanDigest = request.ClosurePlanID, request.ClosurePlanDigest
	baseline.CandidateTransferredBytesDigest = request.CandidateTransferredBytesDigest
	baseline.PublicCandidateTransferredBytesDigest = request.PublicCandidateTransferredBytesDigest
	baseline.AggregateByteLimit, baseline.RetentionReceiptID, baseline.ArtifactDigest = request.AggregateByteLimit, request.RetentionReceiptID, request.ArtifactDigest
	return baseline
}

func scopeForPurpose(purpose contract4competios.GrantPurpose) contract4competios.GrantScope {
	switch purpose {
	case contract4competios.GrantPurposeContestLaunch:
		return contract4competios.GrantScopeContestLaunch
	case contract4competios.GrantPurposeContestStarted:
		return contract4competios.GrantScopeContestStarted
	case contract4competios.GrantPurposeContestResultSubmit:
		return contract4competios.GrantScopeContestResultSubmit
	case contract4competios.GrantPurposeManifestClosurePlan:
		return contract4competios.GrantScopeManifestClosurePlan
	case contract4competios.GrantPurposeCandidateValidateRetain:
		return contract4competios.GrantScopeCandidateValidateRetain
	case contract4competios.GrantPurposeArtifactPublish:
		return contract4competios.GrantScopeArtifactPublish
	case contract4competios.GrantPurposeArtifactDisclosureVerify:
		return contract4competios.GrantScopeArtifactDisclosureVerify
	default:
		return ""
	}
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

func TestExecutionProviderConformanceRejectsDeliberatelyUnsafeProvider(t *testing.T) {
	if violations := CheckExecutionProvider(func() contract4competios.ExecutionProvider { return unsafeProvider{} }); len(violations) < 10 {
		t.Fatalf("unsafe provider was not decisively rejected: %v", violations)
	}
}

func TestEventSinkConformanceAcceptsTiedThreeSlotTerminalResult(t *testing.T) {
	request := executionFixtureFor("three-slot-game", "three-slot-config", []byte(`{}`), 3)
	receipt := executionReceiptFixture(request, "instance")
	start := startFixture("instance")
	payload := copyEventPayload(resultFixture("instance"))
	payload.Result.Placements = append(payload.Result.Placements, contract4competios.Placement{SlotOrdinal: 2, EntryID: "entry-c", Rank: 3, Status: contract4competios.PlacementStatusFinished})
	payload.Result.Evidence.RecordedProvenance.ParticipantArtifactDigests = append(payload.Result.Evidence.RecordedProvenance.ParticipantArtifactDigests, artifactDigest("c"))
	result, err := contract4competios.NewExecutionEvent(payload)
	if err != nil {
		t.Fatal(err)
	}
	if violations := CheckExecutionEventSinkWithEvents(func(request contract4competios.ExecutionRequest, receipt contract4competios.ExecutionReceipt) contract4competios.ExecutionEventSink {
		return newReferenceEventSink(request, receipt)
	}, request, receipt, start, result); len(violations) != 0 {
		t.Fatalf("event sink violations: %v", violations)
	}
}

func TestEventSinkConformanceRejectsDeliberatelyUnsafeSink(t *testing.T) {
	if violations := CheckExecutionEventSink(func(contract4competios.ExecutionRequest, contract4competios.ExecutionReceipt) contract4competios.ExecutionEventSink {
		return unsafeEventSink{}
	}); len(violations) < 10 {
		t.Fatalf("unsafe event sink was not decisively rejected: %v", violations)
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
	factory := func() (contract4competios.OperationGrantIssuer, contract4competios.OperationGrantVerifier) {
		registry := map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant{}
		return &referenceAuthority{registry: registry, allowedPurpose: contract4competios.GrantPurposeContestLaunch}, &referenceVerifier{registry: registry, seen: map[string]contract4competios.OperationGrant{}}
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
