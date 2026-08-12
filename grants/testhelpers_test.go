package grants

import (
	"strings"
	"testing"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

const (
	testChessIssuer   = "https://issuer.example"
	testChessSubject  = "svc:competios"
	testChessAudience = "game/execution"

	testEventIssuer   = "https://competios.example"
	testEventSubject  = "svc:game"
	testEventAudience = "competios/events"
)

var testFixtureTime = time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)

func testClock() time.Time { return testFixtureTime }

func testDigest(char string) contract4competios.PayloadDigest {
	return contract4competios.PayloadDigest("sha256:" + strings.Repeat(char, 64))
}

func testArtifactDigest(char string) contract4competios.ArtifactDigest {
	return contract4competios.ArtifactDigest("sha256:" + strings.Repeat(char, 64))
}

func testCommitOID() contract4competios.SourceObjectID {
	return contract4competios.SourceObjectID("sha1:" + strings.Repeat("a", 40))
}

func testHMACKey(t *testing.T, keyID string) HMACKey {
	t.Helper()
	key, err := NewHMACKey(keyID, []byte(strings.Repeat("k", MinimumHMACSecretLength)))
	if err != nil {
		t.Fatalf("NewHMACKey(%q): %v", keyID, err)
	}
	return key
}

func chessPurposes() []contract4competios.GrantPurpose {
	return []contract4competios.GrantPurpose{
		contract4competios.GrantPurposeManifestClosurePlan,
		contract4competios.GrantPurposeCandidateValidateRetain,
		contract4competios.GrantPurposeArtifactDisclosureVerify,
		contract4competios.GrantPurposeArtifactPublish,
		contract4competios.GrantPurposeContestLaunch,
	}
}

func eventPurposes() []contract4competios.GrantPurpose {
	return []contract4competios.GrantPurpose{
		contract4competios.GrantPurposeContestStarted,
		contract4competios.GrantPurposeContestResultSubmit,
	}
}

func allPurposes() []contract4competios.GrantPurpose {
	all := append([]contract4competios.GrantPurpose{}, chessPurposes()...)
	return append(all, eventPurposes()...)
}

// chessDirection returns a Direction covering exactly the five Chess-issued
// source/launch purposes, using the same fixed issuer/subject/audience as
// contract4competiostest's own launch/source fixtures so tests can, if
// needed, cross-check against contract4competios' purpose-specific
// ValidateXGrantForRequest helpers.
func chessDirection(t *testing.T, key KeyMaterial, replay ReplayStore) Direction {
	t.Helper()
	return Direction{
		Name: "chess-issued", Issuer: testChessIssuer, Subject: testChessSubject, Audience: testChessAudience,
		Purposes: chessPurposes(), Key: key, Replay: replay, Now: testClock,
	}
}

// eventDirection returns a Direction covering exactly the two
// Competios-issued event purposes.
func eventDirection(t *testing.T, key KeyMaterial, replay ReplayStore) Direction {
	t.Helper()
	return Direction{
		Name: "competios-issued", Issuer: testEventIssuer, Subject: testEventSubject, Audience: testEventAudience,
		Purposes: eventPurposes(), Key: key, Replay: replay, Now: testClock,
	}
}

// requestForPurpose builds the smallest OperationGrantRequest that satisfies
// contract4competios.ValidateOperationGrantRequest's per-purpose field rules
// for purpose. It exists so tests can drive every purpose without
// hand-duplicating operation_grants.go's field matrix at each call site.
func requestForPurpose(purpose contract4competios.GrantPurpose) contract4competios.OperationGrantRequest {
	request := contract4competios.OperationGrantRequest{
		Purpose: purpose, ProviderID: "provider", AdapterID: "adapter", CommandID: "command",
		TypedPayloadDigest: testDigest("1"), TransportContentType: "application/json",
		RawTransportDigest: testDigest("2"), Method: "POST", Resource: "/resource",
	}
	switch purpose {
	case contract4competios.GrantPurposeManifestClosurePlan:
		request.ParticipantID, request.ParticipantVersionID = "participant", "version"
		request.RepositoryNodeID, request.CommitOID = "repo", testCommitOID()
		request.ManifestPath, request.ManifestEntryKind = "manifest.json", contract4competios.SourceEntryRegular
		request.RawManifestBytesDigest, request.ManifestByteLimit = testArtifactDigest("3"), 1024
	case contract4competios.GrantPurposeCandidateValidateRetain:
		request.ParticipantID, request.ParticipantVersionID = "participant", "version"
		request.RepositoryNodeID, request.CommitOID = "repo", testCommitOID()
		request.ClosurePlanID, request.ClosurePlanDigest = "plan", testDigest("4")
		request.CandidateTransferredBytesDigest, request.AggregateByteLimit = testArtifactDigest("5"), 2048
	case contract4competios.GrantPurposeArtifactPublish:
		request.ParticipantID, request.ParticipantVersionID = "participant", "version"
		request.RetentionReceiptID, request.ArtifactDigest = "retention", testArtifactDigest("6")
		request.DisclosureReceiptID, request.DisclosureRequestDigest = "disclosure", testDigest("7")
	case contract4competios.GrantPurposeArtifactDisclosureVerify:
		request.ParticipantID, request.ParticipantVersionID = "participant", "version"
		request.RepositoryNodeID, request.CommitOID = "repo", testCommitOID()
		request.ClosurePlanID, request.ClosurePlanDigest = "plan", testDigest("4")
		request.PublicCandidateTransferredBytesDigest, request.AggregateByteLimit = testArtifactDigest("8"), 2048
		request.RetentionReceiptID, request.ArtifactDigest = "retention", testArtifactDigest("6")
	case contract4competios.GrantPurposeContestLaunch:
		request.CompetitionID, request.ContestID, request.RequestID = "competition", "contest", "request"
	case contract4competios.GrantPurposeContestStarted, contract4competios.GrantPurposeContestResultSubmit:
		request.CompetitionID, request.ContestID, request.RequestID = "competition", "contest", "request"
		request.ProviderInstanceID = "instance"
	}
	return request
}
