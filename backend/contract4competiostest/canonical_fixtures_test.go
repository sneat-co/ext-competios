package contract4competiostest

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type executionLifecycleFixture struct {
	Request   contract4competios.ExecutionRequest `json:"request"`
	Grant     contract4competios.OperationGrant   `json:"grant"`
	Receipt   contract4competios.ExecutionReceipt `json:"receipt"`
	Started   contract4competios.ExecutionEvent   `json:"started"`
	Completed contract4competios.ExecutionEvent   `json:"completed"`
}

type sourceArtifactLifecycleFixture struct {
	ManifestRequest    contract4competios.ManifestClosurePlanRequest            `json:"manifestRequest"`
	PlanReceipt        contract4competios.ClosurePlanReceipt                    `json:"planReceipt"`
	CandidateRequest   contract4competios.CandidateClosureRetentionRequest      `json:"candidateRequest"`
	RetentionReceipt   contract4competios.ArtifactRetentionReceipt              `json:"retentionReceipt"`
	DisclosureRequest  contract4competios.ArtifactDisclosureVerificationRequest `json:"disclosureRequest"`
	DisclosureReceipt  contract4competios.ArtifactDisclosureVerificationReceipt `json:"disclosureReceipt"`
	PublicationRequest contract4competios.ArtifactPublicationRequest            `json:"publicationRequest"`
	PublicationReceipt contract4competios.ArtifactPublicationReceipt            `json:"publicationReceipt"`
}

func TestCanonicalExecutionLifecycleFixture(t *testing.T) {
	var fixture executionLifecycleFixture
	readFixture(t, "execution-lifecycle.json", &fixture)
	assertExecutionFixtureDigests(t, fixture)
	if err := contract4competios.ValidateExecutionRequest(fixture.Request); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateOperationGrant(fixture.Grant); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateLaunchGrantForRequest(contract4competios.VerifiedOperationGrant{Claims: fixture.Grant}, routeFromExecutionFixture(fixture), fixture.Request); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateExecutionReceiptForRequest(fixture.Receipt, fixture.Request); err != nil {
		t.Fatal(err)
	}
	if fixture.Started.ProviderInstanceID != fixture.Receipt.ProviderInstanceID || fixture.Completed.ProviderInstanceID != fixture.Receipt.ProviderInstanceID {
		t.Fatal("lifecycle fixture does not preserve the receipt provider instance")
	}
	if err := contract4competios.ValidateExecutionEvent(fixture.Started); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateExecutionEvent(fixture.Completed); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateExecutionEventForExecution(fixture.Started, fixture.Request, fixture.Receipt); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateExecutionEventForExecution(fixture.Completed, fixture.Request, fixture.Receipt); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateLifecycleTransition(contract4competios.ExecutionStateAccepted, fixture.Started.Kind); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateLifecycleTransition(contract4competios.ExecutionStateStarted, fixture.Completed.Kind); err != nil {
		t.Fatal(err)
	}
	assertCanonicalRoundTrip(t, fixture)
}

func assertExecutionFixtureDigests(t *testing.T, fixture executionLifecycleFixture) {
	t.Helper()
	checks := []struct {
		name string
		got  contract4competios.PayloadDigest
		want func() (contract4competios.PayloadDigest, error)
	}{
		{"execution request", fixture.Request.TypedPayloadDigest, func() (contract4competios.PayloadDigest, error) {
			return contract4competios.DigestExecutionRequestPayload(fixture.Request.Payload())
		}},
		{"started event", fixture.Started.TypedPayloadDigest, func() (contract4competios.PayloadDigest, error) {
			return contract4competios.DigestExecutionEventPayload(fixture.Started.Payload())
		}},
		{"completed event", fixture.Completed.TypedPayloadDigest, func() (contract4competios.PayloadDigest, error) {
			return contract4competios.DigestExecutionEventPayload(fixture.Completed.Payload())
		}},
	}
	for _, check := range checks {
		want, err := check.want()
		if err != nil {
			t.Fatal(err)
		}
		if check.got != want {
			t.Fatalf("%s digest = %q, want %q", check.name, check.got, want)
		}
	}
	if fixture.Request.Profile.ProviderExecuted != nil && fixture.Completed.Result != nil && fixture.Completed.Result.Evidence.ProviderExecuted != nil {
		want := contract4competios.DigestProviderConfiguration(fixture.Request.Profile.ProviderExecuted.Configuration)
		if got := fixture.Completed.Result.Evidence.ProviderExecuted.ProviderConfigurationDigest; got != want {
			t.Fatalf("provider configuration digest = %q, want %q", got, want)
		}
	}
}

func routeFromExecutionFixture(fixture executionLifecycleFixture) contract4competios.OperationRouteBinding {
	body, _ := json.Marshal(fixture.Request)
	return contract4competios.OperationRouteBinding{
		Issuer: fixture.Grant.Issuer, Subject: fixture.Grant.Subject, Audience: fixture.Grant.Audience,
		TokenType: fixture.Grant.TokenType, Scope: fixture.Grant.Scope, Purpose: fixture.Grant.Purpose,
		ProviderID: fixture.Request.ProviderID, AdapterID: fixture.Request.AdapterID,
		TransportContentType: fixture.Grant.TransportContentType,
		RawTransportDigest:   contract4competios.DigestRawTransportBody(fixture.Grant.TransportContentType, body),
		Method:               fixture.Grant.Method, Resource: fixture.Grant.Resource,
	}
}

func TestCanonicalSourceArtifactLifecycleFixture(t *testing.T) {
	var fixture sourceArtifactLifecycleFixture
	readFixture(t, "source-artifact-lifecycle.json", &fixture)
	assertSourceFixtureDigests(t, fixture)
	if err := contract4competios.ValidateManifestClosurePlanRequest(fixture.ManifestRequest); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateClosurePlanReceiptForRequest(fixture.PlanReceipt, fixture.ManifestRequest); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateCandidateClosureRetentionRequest(fixture.CandidateRequest); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateCandidateClosureInput(fixture.CandidateRequest, fixture.PlanReceipt.Plan, sourceCandidateTransferFixture()); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateArtifactRetentionReceiptForRequest(fixture.RetentionReceipt, fixture.CandidateRequest); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateArtifactDisclosureInput(fixture.DisclosureRequest, fixture.PlanReceipt.Plan, sourceCandidateTransferFixture()); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateArtifactDisclosureVerificationReceiptForRequest(fixture.DisclosureReceipt, fixture.DisclosureRequest); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateArtifactPublicationPrerequisites(fixture.PublicationRequest, fixture.RetentionReceipt, fixture.DisclosureRequest, fixture.DisclosureReceipt); err != nil {
		t.Fatal(err)
	}
	if err := contract4competios.ValidateArtifactPublicationReceiptForRequest(fixture.PublicationReceipt, fixture.PublicationRequest); err != nil {
		t.Fatal(err)
	}
	assertCanonicalRoundTrip(t, fixture)
}

func assertSourceFixtureDigests(t *testing.T, fixture sourceArtifactLifecycleFixture) {
	t.Helper()
	checks := []struct {
		name string
		got  contract4competios.PayloadDigest
		want func() (contract4competios.PayloadDigest, error)
	}{
		{"manifest request", fixture.ManifestRequest.TypedPayloadDigest, func() (contract4competios.PayloadDigest, error) {
			return contract4competios.DigestManifestClosurePlanRequestPayload(fixture.ManifestRequest.Payload())
		}},
		{"closure plan", fixture.PlanReceipt.Plan.ClosurePlanDigest, func() (contract4competios.PayloadDigest, error) {
			return contract4competios.DigestClosurePlanPayload(fixture.PlanReceipt.Plan.Payload())
		}},
		{"candidate request", fixture.CandidateRequest.TypedPayloadDigest, func() (contract4competios.PayloadDigest, error) {
			return contract4competios.DigestCandidateClosureRetentionRequestPayload(fixture.CandidateRequest.Payload())
		}},
		{"disclosure request", fixture.DisclosureRequest.TypedPayloadDigest, func() (contract4competios.PayloadDigest, error) {
			return contract4competios.DigestArtifactDisclosureVerificationRequestPayload(fixture.DisclosureRequest.Payload())
		}},
		{"publication request", fixture.PublicationRequest.TypedPayloadDigest, func() (contract4competios.PayloadDigest, error) {
			return contract4competios.DigestArtifactPublicationRequestPayload(fixture.PublicationRequest.Payload())
		}},
	}
	for _, check := range checks {
		want, err := check.want()
		if err != nil {
			t.Fatal(err)
		}
		if check.got != want {
			t.Fatalf("%s digest = %q, want %q", check.name, check.got, want)
		}
	}
}

func readFixture(t *testing.T, name string, target any) {
	t.Helper()
	bytes, err := os.ReadFile("testdata/automated-execution/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bytes, target); err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(bytes))
	for _, forbidden := range []string{"bearer", "accesstoken", "githubinstallation", "credential", "privatekey"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("%s leaks %q", name, forbidden)
		}
	}
}

func assertCanonicalRoundTrip(t *testing.T, value any) {
	t.Helper()
	first, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	copy := reflect.New(reflect.TypeOf(value)).Interface()
	if err := json.Unmarshal(first, copy); err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(reflect.ValueOf(copy).Elem().Interface())
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("canonical JSON changed after round-trip")
	}
}
