package grants

import (
	"context"
	"errors"
	"testing"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

func TestRouteBindingRefusesPurposeOutsideDirection(t *testing.T) {
	direction := chessDirection(t, testHMACKey(t, "kid"), NewMemoryReplayStore(nil))
	_, err := RouteBinding(direction, contract4competios.GrantPurposeContestStarted, "provider", "adapter", "application/json", []byte("{}"), "POST", "/resource")
	if !errors.Is(err, ErrPurposeNotPermitted) {
		t.Fatalf("err = %v, want ErrPurposeNotPermitted", err)
	}
}

// TestRouteBindingIntegratesWithContractEventValidation exercises the exact
// caller-side flow Decision 0007 leaves outside the issuer/verifier: issue an
// event grant -> verify it -> build the RouteBinding from the SAME Direction
// -> feed both into contract4competios.ValidateEventGrantForEvent, the
// contract's own purpose-specific route+payload-digest comparison. A
// verified grant that does not exactly match the request actually received
// must be rejected by that contract function, not re-derived here.
func TestRouteBindingIntegratesWithContractEventValidation(t *testing.T) {
	event, err := contract4competios.NewExecutionEvent(contract4competios.ExecutionEventPayload{
		ID: "evt-1", Kind: contract4competios.LifecycleEventStarted,
		CompetitionID: "competition", ContestID: "contest", RequestID: "request",
		ProviderID: "provider", AdapterID: "adapter", ProviderInstanceID: "instance",
		CommandID: "command", OccurredAt: testFixtureTime,
	})
	if err != nil {
		t.Fatalf("NewExecutionEvent: %v", err)
	}
	rawBody := []byte(`{"kind":"started"}`)
	const contentType = "application/vnd.competios.operation+json;version=1"
	const method, resource = "POST", "/competios/events"
	request, err := contract4competios.NewExecutionEventGrantRequest(event, method, resource, contentType, rawBody)
	if err != nil {
		t.Fatalf("NewExecutionEventGrantRequest: %v", err)
	}

	direction := eventDirection(t, testHMACKey(t, "kid-event"), NewMemoryReplayStore(nil))
	issuer, err := NewIssuer(direction)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	verifier, err := NewVerifier(direction)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	ctx := context.Background()
	issued, err := issuer.IssueOperationGrant(ctx, request)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	verified, err := verifier.VerifyOperationGrant(ctx, issued.AccessToken)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	route, err := RouteBinding(direction, contract4competios.GrantPurposeContestStarted, "provider", "adapter", contentType, rawBody, method, resource)
	if err != nil {
		t.Fatalf("RouteBinding: %v", err)
	}
	if err := contract4competios.ValidateEventGrantForEvent(verified, route, event); err != nil {
		t.Fatalf("ValidateEventGrantForEvent rejected a genuine, matching grant: %v", err)
	}

	// A route built for the WRONG resource must be rejected -- this is the
	// "compare route binding + payload digests" step actually catching a
	// mismatched request.
	wrongRoute, err := RouteBinding(direction, contract4competios.GrantPurposeContestStarted, "provider", "adapter", contentType, rawBody, method, "/some/other/resource")
	if err != nil {
		t.Fatalf("RouteBinding: %v", err)
	}
	if err := contract4competios.ValidateEventGrantForEvent(verified, wrongRoute, event); err == nil {
		t.Fatalf("ValidateEventGrantForEvent accepted a grant against a mismatched resource")
	}
}
