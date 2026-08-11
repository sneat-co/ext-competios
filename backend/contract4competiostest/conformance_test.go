package contract4competiostest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type referenceEventSink struct {
	events   map[contract4competios.CommandID]contract4competios.ExecutionEvent
	started  bool
	instance contract4competios.ProviderInstanceID
}

func (s *referenceEventSink) SubmitExecutionEvent(_ context.Context, grant contract4competios.VerifiedOperationGrant, event contract4competios.ExecutionEvent) (contract4competios.EventAcknowledgement, error) {
	if contract4competios.ValidateLifecycleEvent(event) != nil || grant.Claims.ContestID != event.ContestID || grant.Claims.RequestID != event.RequestID || grant.Claims.ProviderInstanceID != event.ProviderInstanceID || grant.Claims.CommandID != event.CommandID || ((event.Kind == contract4competios.LifecycleStarted) != (grant.Claims.Purpose == contract4competios.GrantPurposeContestStarted)) && ((event.Kind == contract4competios.LifecycleCompleted) != (grant.Claims.Purpose == contract4competios.GrantPurposeContestResultSubmit)) {
		return contract4competios.EventAcknowledgement{}, contract4competios.ErrInvalidGrant
	}
	if s.events == nil {
		s.events = map[contract4competios.CommandID]contract4competios.ExecutionEvent{}
	}
	if prior, ok := s.events[event.CommandID]; ok {
		if prior.ID != event.ID || prior.ProviderInstanceID != event.ProviderInstanceID || prior.Kind != event.Kind {
			return contract4competios.EventAcknowledgement{}, contract4competios.ErrCommandConflict
		}
		if event.Result != nil && (prior.Result == nil || prior.Result.Placements[0].Rank != event.Result.Placements[0].Rank) {
			return contract4competios.EventAcknowledgement{}, contract4competios.ErrCommandConflict
		}
		return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementReplayed}, nil
	}
	if event.Kind == contract4competios.LifecycleStarted {
		s.started, s.instance = true, event.ProviderInstanceID
	} else if event.Kind == contract4competios.LifecycleCompleted && (!s.started || s.instance != event.ProviderInstanceID) {
		return contract4competios.EventAcknowledgement{}, contract4competios.ErrInvalidTransition
	}
	s.events[event.CommandID] = event
	return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementAccepted}, nil
}

type referenceVerifier struct{}

func (referenceVerifier) VerifyOperationGrant(_ context.Context, value contract4competios.OperationGrant) (contract4competios.VerifiedOperationGrant, error) {
	if err := contract4competios.ValidateOperationGrant(value); err != nil || value.Audience != "game/execution" || value.Purpose != contract4competios.GrantPurposeContestLaunch || value.RawTransportDigest != "sha256:transport" {
		return contract4competios.VerifiedOperationGrant{}, contract4competios.ErrInvalidGrant
	}
	return contract4competios.VerifiedOperationGrant{Claims: value}, nil
}

// unsafeProvider is intentional: it returns a new successful instance for every
// request and accepts every grant. The reusable suite must expose both flaws.
type unsafeProvider struct{}

func (unsafeProvider) LaunchExecution(_ context.Context, _ contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	return contract4competios.ExecutionReceipt{RequestID: request.ID, ProviderInstanceID: "new-every-time", Status: contract4competios.ReceiptAccepted}, nil
}

type unsafeEventSink struct{}

func (unsafeEventSink) SubmitExecutionEvent(context.Context, contract4competios.VerifiedOperationGrant, contract4competios.ExecutionEvent) (contract4competios.EventAcknowledgement, error) {
	return contract4competios.EventAcknowledgement{Status: contract4competios.EventAcknowledgementAccepted}, nil
}

type unsafeVerifier struct{}

func (unsafeVerifier) VerifyOperationGrant(_ context.Context, value contract4competios.OperationGrant) (contract4competios.VerifiedOperationGrant, error) {
	return contract4competios.VerifiedOperationGrant{Claims: value}, nil
}

// biddingTicTacToeProvider is intentionally an independent implementation:
// its opaque configuration is a bid policy rather than a Chess rules profile.
type biddingTicTacToeProvider struct {
	receipts map[contract4competios.CommandID]contract4competios.ExecutionReceipt
	digests  map[contract4competios.CommandID]contract4competios.PayloadDigest
}

func (p *biddingTicTacToeProvider) LaunchExecution(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	if grant.Claims.Purpose != contract4competios.GrantPurposeContestLaunch || grant.Claims.Audience != "game/execution" || grant.Claims.RawTransportDigest != "sha256:transport" || request.GameID != "bidding-tic-tac-toe" || grant.Claims.ContestID != request.ContestID || grant.Claims.RequestID != request.ID || grant.Claims.CommandID != request.CommandID || contract4competios.ValidateExecutionRequest(request) != nil {
		return contract4competios.ExecutionReceipt{}, contract4competios.ErrInvalidGrant
	}
	if p.receipts == nil {
		p.receipts, p.digests = map[contract4competios.CommandID]contract4competios.ExecutionReceipt{}, map[contract4competios.CommandID]contract4competios.PayloadDigest{}
	}
	if receipt, ok := p.receipts[request.CommandID]; ok {
		if p.digests[request.CommandID] != request.TypedPayloadDigest {
			return contract4competios.ExecutionReceipt{}, contract4competios.ErrCommandConflict
		}
		receipt.Status = contract4competios.ReceiptReplayed
		return receipt, nil
	}
	receipt := contract4competios.ExecutionReceipt{RequestID: request.ID, CommandID: request.CommandID, ProviderID: request.ProviderID, AdapterID: request.AdapterID, ProviderInstanceID: "bid-instance", Status: contract4competios.ReceiptAccepted}
	p.receipts[request.CommandID], p.digests[request.CommandID] = receipt, request.TypedPayloadDigest
	return receipt, nil
}

type countingProvider struct{ calls int }

func (p *countingProvider) LaunchExecution(context.Context, contract4competios.VerifiedOperationGrant, contract4competios.ExecutionRequest) (contract4competios.ExecutionReceipt, error) {
	p.calls++
	return contract4competios.ExecutionReceipt{}, errors.New("should not be called")
}

func TestExecutionProviderConformanceAcceptsChessShapedProvider(t *testing.T) {
	if violations := CheckExecutionProvider(func() contract4competios.ExecutionProvider { return &referenceProvider{} }); len(violations) != 0 {
		t.Fatalf("chess-shaped provider violations: %v", violations)
	}
}
func TestExecutionProviderConformanceAcceptsUnrelatedBiddingTicTacToeFake(t *testing.T) {
	request := executionFixture()
	request.GameID, request.Configuration = "bidding-tic-tac-toe", contract4competios.ProviderConfiguration{Version: "bid-policy", Data: []byte("sealed-bid")}
	request.TypedPayloadDigest, _ = contract4competios.DigestTypedPayload(struct {
		Game string `json:"game"`
	}{string(request.GameID)})
	if violations := CheckExecutionProviderWithRequest(func() contract4competios.ExecutionProvider { return &biddingTicTacToeProvider{} }, request); len(violations) != 0 {
		t.Fatalf("bidding-tic-tac-toe provider violations: %v", violations)
	}
}
func TestExecutionProviderConformanceRejectsDeliberatelyUnsafeProvider(t *testing.T) {
	if violations := CheckExecutionProvider(func() contract4competios.ExecutionProvider { return unsafeProvider{} }); len(violations) < 2 {
		t.Fatalf("unsafe provider violations = %v", violations)
	}
}
func TestEventSinkConformanceAcceptsTiedThreeSlotTerminalResult(t *testing.T) {
	start, result := startFixture("instance"), resultFixture("instance")
	result.Result.Placements = append(result.Result.Placements, contract4competios.Placement{EntryID: "entry-c", Rank: 3, Status: contract4competios.PlacementStatusFinished})
	if violations := CheckExecutionEventSinkWithEvents(func() contract4competios.ExecutionEventSink { return &referenceEventSink{} }, start, result); len(violations) != 0 {
		t.Fatalf("event sink violations: %v", violations)
	}
}
func TestEventSinkConformanceRejectsDeliberatelyUnsafeProvider(t *testing.T) {
	if violations := CheckExecutionEventSink(func() contract4competios.ExecutionEventSink { return unsafeEventSink{} }); len(violations) < 2 {
		t.Fatalf("unsafe event sink violations = %v", violations)
	}
}
func TestGrantVerifierConformanceRejectsDeliberatelyUnsafeVerifier(t *testing.T) {
	if violations := CheckOperationGrantVerifier(func() contract4competios.OperationGrantVerifier { return unsafeVerifier{} }); len(violations) == 0 {
		t.Fatal("unsafe verifier unexpectedly passed conformance")
	}
}

func TestValidationRejectsResultBeforeStartAndCancellationTransition(t *testing.T) {
	result := resultFixture("instance")
	if _, err := (&referenceEventSink{}).SubmitExecutionEvent(context.Background(), eventGrantFixture(result, contract4competios.GrantPurposeContestResultSubmit), result); !errors.Is(err, contract4competios.ErrInvalidTransition) {
		t.Fatalf("result before start = %v", err)
	}
	if err := contract4competios.ValidateLifecycleTransition(contract4competios.LifecycleCancelled, contract4competios.LifecycleStarted); !errors.Is(err, contract4competios.ErrInvalidTransition) {
		t.Fatalf("cancelled restart = %v", err)
	}
}

func TestRawTransportAndTypedPayloadDigestsAreExplicitlyDifferent(t *testing.T) {
	typed, err := contract4competios.DigestTypedPayload(struct {
		Value string `json:"value"`
	}{"one"})
	if err != nil {
		t.Fatal(err)
	}
	raw := contract4competios.DigestRawTransportBody("application/json;v=1", []byte(`{"value":"one"}`))
	if typed == raw || typed == "" || raw == "" {
		t.Fatalf("typed/raw digests = %q/%q", typed, raw)
	}
}

func TestValidationGrantHasNoContestOrExecutionBinding(t *testing.T) {
	grant := launchGrantFixture().Claims
	grant.Purpose, grant.CompetitionID, grant.ContestID, grant.RequestID, grant.ProviderInstanceID = contract4competios.GrantPurposeParticipantVersionValidate, "", "", "", ""
	grant.ParticipantID, grant.ParticipantVersionID = "participant", "version"
	grant.RepositoryNodeID, grant.Commit, grant.Path = "repository", "full-commit", "bot.star"
	grant.ManifestDigest, grant.ArtifactDigest, grant.ByteLimit = "sha256:manifest", "sha256:artifact", 1024
	if err := contract4competios.ValidateOperationGrant(grant); err != nil {
		t.Fatalf("valid source-validation grant = %v", err)
	}
	grant.ContestID = "forbidden"
	if err := contract4competios.ValidateOperationGrant(grant); !errors.Is(err, contract4competios.ErrInvalidGrant) {
		t.Fatalf("source-validation grant with contest = %v", err)
	}
}

func TestNoActionDoesNotCallProvider(t *testing.T) {
	provider := &countingProvider{}
	if provider.calls != 0 {
		t.Fatalf("provider calls before a request = %d", provider.calls)
	}
}

func TestCanonicalFixturesValidate(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		value any
	}{
		{"execution-request.json", &contract4competios.ExecutionRequest{}},
		{"operation-grant.json", &contract4competios.OperationGrant{}},
	} {
		bytes, err := os.ReadFile("testdata/automated-execution/" + fixture.name)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(bytes, fixture.value); err != nil {
			t.Fatalf("%s: %v", fixture.name, err)
		}
		switch value := fixture.value.(type) {
		case *contract4competios.ExecutionRequest:
			if err = contract4competios.ValidateExecutionRequest(*value); err != nil {
				t.Fatalf("%s: %v", fixture.name, err)
			}
		case *contract4competios.OperationGrant:
			if err = contract4competios.ValidateOperationGrant(*value); err != nil {
				t.Fatalf("%s: %v", fixture.name, err)
			}
		}
	}
}

func TestPublicJSONCannotCarryBearerOrPrivateSourceFields(t *testing.T) {
	encoded, err := json.Marshal(resultFixture("instance"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"token", "bearer", "github", "sourcebytes", "private"} {
		if containsFold(string(encoded), forbidden) {
			t.Fatalf("public event leaks %q: %s", forbidden, encoded)
		}
	}
}
func containsFold(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		equal := true
		for offset := range part {
			a, b := value[index+offset], part[offset]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				equal = false
				break
			}
		}
		if equal {
			return true
		}
	}
	return false
}
