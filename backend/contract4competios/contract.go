// Package contract4competios defines the dependency-free boundary between
// Competios and an automated game provider. It deliberately contains no JWT,
// OAuth, HTTP, storage, or game-rule implementation.
package contract4competios

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ExtensionID is Competios' stable global Firestore extension namespace.
// Hosts and adapters use it without coupling to an implementation package.
const ExtensionID = "competios"

var (
	ErrCommandConflict  = errors.New("competios contract: command ID reused with a different payload")
	ErrInvalidExecution = errors.New("competios contract: invalid execution")
	ErrInvalidGrant     = errors.New("competios contract: invalid operation grant")
)

type CompetitionID string
type ContestID string
type EntryID string
type ProviderID string
type AdapterID string
type ExecutionRequestID string
type ProviderInstanceID string
type ParticipantVersionID string
type ArtifactDigest string
type PayloadDigest string
type EventID string
type ReplayReference string

// ParticipantVersionRef is an immutable provider-owned participant version.
// ArtifactDigest identifies an already-retained executable artifact; it never
// carries source bytes, repository credentials, or an implementation language.
type ParticipantVersionRef struct {
	ParticipantID        ParticipantID        `json:"participantID"`
	ParticipantVersionID ParticipantVersionID `json:"participantVersionID"`
	ArtifactDigest       ArtifactDigest       `json:"artifactDigest"`
}

// ExecutionSlot has a stable generic ordinal. A provider may assign colours,
// bids, seats or other game semantics after accepting the request.
type ExecutionSlot struct {
	Ordinal     uint16                `json:"ordinal"`
	EntryID     EntryID               `json:"entryID"`
	Participant ParticipantVersionRef `json:"participant"`
}

// ProviderConfiguration is versioned opaque provider data. It is intentionally
// bytes rather than a map, so its exact representation participates in digest
// vectors and cannot acquire a game-specific public schema here.
type ProviderConfiguration struct {
	Version string `json:"version"`
	Data    []byte `json:"data"`
}

// CallbackResource is the provider's typed destination for service events.
// It is a resource name, never a user-supplied URL.
type CallbackResource struct {
	Resource string `json:"resource"`
}

// ExecutionRequest is one immutable ordered N-slot provider operation.
// TypedPayloadDigest is over the deterministic typed payload encoding. The
// separately named transport digest is calculated over exact content type and
// raw HTTP bytes by DigestRawTransportBody; neither is interchangeable.
type ExecutionRequest struct {
	ID                 ExecutionRequestID    `json:"id"`
	ProviderID         ProviderID            `json:"providerID"`
	AdapterID          AdapterID             `json:"adapterID"`
	CompetitionID      CompetitionID         `json:"competitionID"`
	ContestID          ContestID             `json:"contestID"`
	CommandID          CommandID             `json:"commandID"`
	GameID             GameID                `json:"gameID"`
	RulesetVersion     RulesetVersion        `json:"rulesetVersion"`
	Slots              []ExecutionSlot       `json:"slots"`
	Configuration      ProviderConfiguration `json:"configuration"`
	NotBefore          time.Time             `json:"notBefore,omitempty"`
	Deadline           time.Time             `json:"deadline,omitempty"`
	Callback           CallbackResource      `json:"callback"`
	TypedPayloadDigest PayloadDigest         `json:"typedPayloadDigest"`
}

type ReceiptStatus string

const (
	ReceiptAccepted ReceiptStatus = "accepted"
	ReceiptReplayed ReceiptStatus = "replayed"
)

// ExecutionReceipt is the sole immutable response to a successfully accepted
// execution command. SafeReferences may refer only to provider-public artifacts.
type ExecutionReceipt struct {
	RequestID          ExecutionRequestID `json:"requestID"`
	CommandID          CommandID          `json:"commandID"`
	ProviderID         ProviderID         `json:"providerID"`
	AdapterID          AdapterID          `json:"adapterID"`
	ProviderInstanceID ProviderInstanceID `json:"providerInstanceID"`
	Status             ReceiptStatus      `json:"status"`
	SafeReferences     []string           `json:"safeReferences,omitempty"`
}

// ExecutionProvider must persist durable command plus exact typed payload
// identity. A fresh transport grant can therefore retry an unknown operation;
// changed content under the same command must return ErrCommandConflict.
type ExecutionProvider interface {
	LaunchExecution(context.Context, VerifiedOperationGrant, ExecutionRequest) (ExecutionReceipt, error)
}

type LifecycleKind string

const (
	LifecycleQueued    LifecycleKind = "queued"
	LifecycleStarted   LifecycleKind = "started"
	LifecycleCompleted LifecycleKind = "completed"
	LifecycleFailed    LifecycleKind = "failed"
	LifecycleCancelled LifecycleKind = "cancelled"
)

type PlacementStatus string

const (
	PlacementStatusFinished     PlacementStatus = "finished"
	PlacementStatusForfeited    PlacementStatus = "forfeited"
	PlacementStatusDisqualified PlacementStatus = "disqualified"
	PlacementStatusDidNotFinish PlacementStatus = "did-not-finish"
)

// Placement order is deterministic. Equal Rank values are valid ties.
type Placement struct {
	EntryID EntryID         `json:"entryID"`
	Rank    uint16          `json:"rank"`
	Status  PlacementStatus `json:"status"`
}

type ReplayPublicationState string

const (
	ReplayAvailable   ReplayPublicationState = "available"
	ReplayProcessing  ReplayPublicationState = "processing"
	ReplayUnavailable ReplayPublicationState = "unavailable"
)

type TerminalReplay struct {
	State     ReplayPublicationState `json:"state"`
	Reference ReplayReference        `json:"reference,omitempty"`
}

// RecordedProvenance contains digest facts only. It is recorded provenance,
// not a detached signature and never carries source bytes or bearer material.
type RecordedProvenance struct {
	ParticipantArtifactDigests  []ArtifactDigest `json:"participantArtifactDigests"`
	ProviderConfigurationDigest ArtifactDigest   `json:"providerConfigurationDigest"`
	RuntimeDigest               ArtifactDigest   `json:"runtimeDigest"`
	RulesDigest                 ArtifactDigest   `json:"rulesDigest"`
	LimitProfileDigest          ArtifactDigest   `json:"limitProfileDigest"`
	SeedDigest                  ArtifactDigest   `json:"seedDigest"`
	EventLogDigest              ArtifactDigest   `json:"eventLogDigest"`
	ExecutionPayloadDigest      PayloadDigest    `json:"executionPayloadDigest"`
	ResultPayloadDigest         PayloadDigest    `json:"resultPayloadDigest"`
}

type ExecutionResult struct {
	ID                 EventID            `json:"id"`
	Placements         []Placement        `json:"placements"`
	CompletedAt        time.Time          `json:"completedAt"`
	FailureCode        string             `json:"failureCode,omitempty"`
	AdjudicationCode   string             `json:"adjudicationCode,omitempty"`
	Replay             TerminalReplay     `json:"replay"`
	RecordedProvenance RecordedProvenance `json:"recordedProvenance"`
}

// ExecutionEvent expresses queued/start/terminal provider evidence. A result
// is present only for completed, failed, or cancelled terminal events.
type ExecutionEvent struct {
	ID                 EventID            `json:"id"`
	Kind               LifecycleKind      `json:"kind"`
	CompetitionID      CompetitionID      `json:"competitionID"`
	ContestID          ContestID          `json:"contestID"`
	RequestID          ExecutionRequestID `json:"requestID"`
	ProviderID         ProviderID         `json:"providerID"`
	AdapterID          AdapterID          `json:"adapterID"`
	ProviderInstanceID ProviderInstanceID `json:"providerInstanceID"`
	CommandID          CommandID          `json:"commandID"`
	OccurredAt         time.Time          `json:"occurredAt"`
	Result             *ExecutionResult   `json:"result,omitempty"`
}

type EventAcknowledgementStatus string

const (
	EventAcknowledgementAccepted EventAcknowledgementStatus = "accepted"
	EventAcknowledgementReplayed EventAcknowledgementStatus = "replayed"
)

type EventAcknowledgement struct {
	Status EventAcknowledgementStatus `json:"status"`
}

type ExecutionEventSink interface {
	SubmitExecutionEvent(context.Context, VerifiedOperationGrant, ExecutionEvent) (EventAcknowledgement, error)
}

// DigestTypedPayload deterministically hashes a typed public contract payload.
// Callers must avoid maps in interoperable payloads; opaque configuration is
// bytes specifically so the contract does not claim a JSON canonicalizer.
func DigestTypedPayload(value any) (PayloadDigest, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return PayloadDigest("sha256:" + digest(encoded)), nil
}

// DigestRawTransportBody is distinct from DigestTypedPayload. It commits to
// the exact versioned content type and raw transport bytes before parsing.
func DigestRawTransportBody(contentType string, body []byte) PayloadDigest {
	return PayloadDigest("sha256:" + digest(append(append([]byte(contentType), '\n'), body...)))
}

func digest(value []byte) string { sum := sha256.Sum256(value); return hex.EncodeToString(sum[:]) }

func ValidateExecutionRequest(value ExecutionRequest) error {
	if value.ID == "" || value.ProviderID == "" || value.AdapterID == "" || value.CompetitionID == "" || value.ContestID == "" || value.CommandID == "" || value.GameID == "" || value.RulesetVersion == "" || value.Configuration.Version == "" || value.Callback.Resource == "" || value.TypedPayloadDigest == "" || len(value.Slots) == 0 {
		return ErrInvalidExecution
	}
	if !value.Deadline.IsZero() && !value.NotBefore.IsZero() && value.Deadline.Before(value.NotBefore) {
		return ErrInvalidExecution
	}
	seenOrdinal, seenEntry, seenVersion := map[uint16]bool{}, map[EntryID]bool{}, map[ParticipantVersionID]bool{}
	for index, slot := range value.Slots {
		if slot.Ordinal != uint16(index) || slot.EntryID == "" || slot.Participant.ParticipantID == "" || slot.Participant.ParticipantVersionID == "" || slot.Participant.ArtifactDigest == "" || seenOrdinal[slot.Ordinal] || seenEntry[slot.EntryID] || seenVersion[slot.Participant.ParticipantVersionID] {
			return ErrInvalidExecution
		}
		seenOrdinal[slot.Ordinal], seenEntry[slot.EntryID], seenVersion[slot.Participant.ParticipantVersionID] = true, true, true
	}
	return nil
}

func ValidateLifecycleEvent(value ExecutionEvent) error {
	if value.ID == "" || value.Kind == "" || value.CompetitionID == "" || value.ContestID == "" || value.RequestID == "" || value.ProviderID == "" || value.AdapterID == "" || value.ProviderInstanceID == "" || value.CommandID == "" || value.OccurredAt.IsZero() {
		return ErrInvalidExecution
	}
	terminal := value.Kind == LifecycleCompleted || value.Kind == LifecycleFailed || value.Kind == LifecycleCancelled
	if terminal != (value.Result != nil) {
		return ErrInvalidExecution
	}
	if !terminal {
		return nil
	}
	result := value.Result
	if result.ID == "" || result.CompletedAt.IsZero() || len(result.Placements) == 0 || result.Replay.State == "" {
		return ErrInvalidExecution
	}
	if result.Replay.State == ReplayAvailable && result.Replay.Reference == "" || result.Replay.State != ReplayAvailable && result.Replay.Reference != "" {
		return ErrInvalidExecution
	}
	seen := map[EntryID]bool{}
	for _, placement := range result.Placements {
		if placement.EntryID == "" || placement.Rank == 0 || placement.Status == "" || seen[placement.EntryID] {
			return ErrInvalidExecution
		}
		seen[placement.EntryID] = true
	}
	return nil
}

func ValidateLifecycleTransition(previous, next LifecycleKind) error {
	valid := map[LifecycleKind]map[LifecycleKind]bool{
		LifecycleQueued:  {LifecycleStarted: true, LifecycleCancelled: true, LifecycleFailed: true},
		LifecycleStarted: {LifecycleCompleted: true, LifecycleCancelled: true, LifecycleFailed: true},
	}
	if !valid[previous][next] {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, previous, next)
	}
	return nil
}
