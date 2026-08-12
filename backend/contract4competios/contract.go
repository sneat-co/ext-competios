// Package contract4competios defines the dependency-free boundary between
// Competios and a game provider. It contains no JWT, OAuth, HTTP, storage, or
// game-rule implementation.
package contract4competios

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ExtensionID is Competios' stable global extension namespace.
const ExtensionID = "competios"

var (
	ErrCommandConflict     = errors.New("competios contract: command ID reused with a different payload")
	ErrTokenReplayConflict = errors.New("competios contract: token ID reused with different claims")
	ErrInvalidExecution    = errors.New("competios contract: invalid execution")
	ErrInvalidGrant        = errors.New("competios contract: invalid operation grant")
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

type ExecutionProfileKind string

const (
	ExecutionProfileProviderExecuted     ExecutionProfileKind = "provider-executed"
	ExecutionProfileParticipantScheduled ExecutionProfileKind = "participant-scheduled"
)

type ParticipantVersionRef struct {
	ParticipantID        ParticipantID        `json:"participantID"`
	ParticipantVersionID ParticipantVersionID `json:"participantVersionID"`
	ArtifactDigest       ArtifactDigest       `json:"artifactDigest"`
}

// ExecutionSlot is an immutable provider-owned participant version. Its
// ordinal is generic; the provider assigns colours, seats, bids, or sides.
type ExecutionSlot struct {
	Ordinal     uint16                `json:"ordinal"`
	EntryID     EntryID               `json:"entryID"`
	Participant ParticipantVersionRef `json:"participant"`
}

type ProviderConfiguration struct {
	Version string `json:"version"`
	Data    []byte `json:"data"`
}

type ProviderExecutedProfile struct {
	Slots         []ExecutionSlot       `json:"slots"`
	Configuration ProviderConfiguration `json:"configuration"`
	NotBefore     time.Time             `json:"notBefore,omitempty"`
	Deadline      time.Time             `json:"deadline,omitempty"`
}

// ParticipantScheduledSlot represents a frozen human/team lineup without
// reintroducing user IDs or game-specific side vocabulary.
type ParticipantScheduledSlot struct {
	Ordinal      uint16          `json:"ordinal"`
	EntryID      EntryID         `json:"entryID"`
	Participants []ParticipantID `json:"participants"`
}

type ParticipantScheduledProfile struct {
	StartsAt time.Time                  `json:"startsAt"`
	Slots    []ParticipantScheduledSlot `json:"slots"`
}

// ExecutionProfile is a closed discriminator. Exactly one matching payload is
// present; there is no legacy lineup/side variant.
type ExecutionProfile struct {
	Kind                 ExecutionProfileKind         `json:"kind"`
	ProviderExecuted     *ProviderExecutedProfile     `json:"providerExecuted,omitempty"`
	ParticipantScheduled *ParticipantScheduledProfile `json:"participantScheduled,omitempty"`
}

type CallbackResource struct {
	Resource string `json:"resource"`
}

type PublicArtifactKind string

const PublicArtifactTerminalReplay PublicArtifactKind = "terminal-replay"

// ExecutionRequestPayload is the complete canonical digest input. It excludes
// only TypedPayloadDigest, preventing a self-referential hash.
type ExecutionRequestPayload struct {
	ID                       ExecutionRequestID   `json:"id"`
	ProviderID               ProviderID           `json:"providerID"`
	AdapterID                AdapterID            `json:"adapterID"`
	CompetitionID            CompetitionID        `json:"competitionID"`
	ContestID                ContestID            `json:"contestID"`
	CommandID                CommandID            `json:"commandID"`
	GameID                   GameID               `json:"gameID"`
	RulesetVersion           RulesetVersion       `json:"rulesetVersion"`
	Profile                  ExecutionProfile     `json:"profile"`
	RequestedPublicArtifacts []PublicArtifactKind `json:"requestedPublicArtifacts,omitempty"`
	Callback                 CallbackResource     `json:"callback"`
}

type ExecutionRequest struct {
	ID                       ExecutionRequestID   `json:"id"`
	ProviderID               ProviderID           `json:"providerID"`
	AdapterID                AdapterID            `json:"adapterID"`
	CompetitionID            CompetitionID        `json:"competitionID"`
	ContestID                ContestID            `json:"contestID"`
	CommandID                CommandID            `json:"commandID"`
	GameID                   GameID               `json:"gameID"`
	RulesetVersion           RulesetVersion       `json:"rulesetVersion"`
	Profile                  ExecutionProfile     `json:"profile"`
	RequestedPublicArtifacts []PublicArtifactKind `json:"requestedPublicArtifacts,omitempty"`
	Callback                 CallbackResource     `json:"callback"`
	TypedPayloadDigest       PayloadDigest        `json:"typedPayloadDigest"`
}

func NewExecutionRequest(payload ExecutionRequestPayload) (ExecutionRequest, error) {
	digest, err := DigestExecutionRequestPayload(payload)
	if err != nil {
		return ExecutionRequest{}, err
	}
	request := ExecutionRequest{
		ID: payload.ID, ProviderID: payload.ProviderID, AdapterID: payload.AdapterID,
		CompetitionID: payload.CompetitionID, ContestID: payload.ContestID,
		CommandID: payload.CommandID, GameID: payload.GameID,
		RulesetVersion: payload.RulesetVersion, Profile: payload.Profile,
		RequestedPublicArtifacts: append([]PublicArtifactKind(nil), payload.RequestedPublicArtifacts...),
		Callback:                 payload.Callback, TypedPayloadDigest: digest,
	}
	if err := ValidateExecutionRequest(request); err != nil {
		return ExecutionRequest{}, err
	}
	return request, nil
}

func (r ExecutionRequest) Payload() ExecutionRequestPayload {
	return ExecutionRequestPayload{
		ID: r.ID, ProviderID: r.ProviderID, AdapterID: r.AdapterID,
		CompetitionID: r.CompetitionID, ContestID: r.ContestID,
		CommandID: r.CommandID, GameID: r.GameID,
		RulesetVersion: r.RulesetVersion, Profile: r.Profile,
		RequestedPublicArtifacts: append([]PublicArtifactKind(nil), r.RequestedPublicArtifacts...), Callback: r.Callback,
	}
}

const executionRequestDigestDomain = "competios.execution-request-payload.v1"
const executionEventDigestDomain = "competios.execution-event-payload.v1"
const rawTransportDigestDomain = "competios.raw-transport-body.v1"

func DigestExecutionRequestPayload(payload ExecutionRequestPayload) (PayloadDigest, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return digestParts(executionRequestDigestDomain, encoded), nil
}

// DigestRawTransportBody binds the exact versioned content type and exact raw
// body bytes. Length prefixes make component boundaries unambiguous.
func DigestRawTransportBody(contentType string, body []byte) PayloadDigest {
	return digestParts(rawTransportDigestDomain, []byte(contentType), body)
}

func digestParts(domain string, parts ...[]byte) PayloadDigest {
	h := sha256.New()
	writeDigestPart(h, []byte(domain))
	for _, part := range parts {
		writeDigestPart(h, part)
	}
	return PayloadDigest("sha256:" + hex.EncodeToString(h.Sum(nil)))
}

type digestWriter interface{ Write([]byte) (int, error) }

func writeDigestPart(w digestWriter, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = w.Write(size[:])
	_, _ = w.Write(value)
}

func validSHA256Digest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[len("sha256:"):] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

type ReceiptStatus string

const (
	ReceiptAccepted ReceiptStatus = "accepted"
	ReceiptReplayed ReceiptStatus = "replayed"
)

type ExecutionReceipt struct {
	RequestID          ExecutionRequestID `json:"requestID"`
	CommandID          CommandID          `json:"commandID"`
	ProviderID         ProviderID         `json:"providerID"`
	AdapterID          AdapterID          `json:"adapterID"`
	ProviderInstanceID ProviderInstanceID `json:"providerInstanceID"`
	Status             ReceiptStatus      `json:"status"`
	SafeReferences     []string           `json:"safeReferences,omitempty"`
}

type ExecutionProvider interface {
	LaunchExecution(context.Context, VerifiedOperationGrant, ExecutionRequest) (ExecutionReceipt, error)
}

func ValidateExecutionReceiptForRequest(receipt ExecutionReceipt, request ExecutionRequest) error {
	if ValidateExecutionRequest(request) != nil || receipt.RequestID != request.ID || receipt.CommandID != request.CommandID || receipt.ProviderID != request.ProviderID || receipt.AdapterID != request.AdapterID || receipt.ProviderInstanceID == "" {
		return ErrInvalidExecution
	}
	switch receipt.Status {
	case ReceiptAccepted, ReceiptReplayed:
	default:
		return ErrInvalidExecution
	}
	seen := map[string]bool{}
	for _, reference := range receipt.SafeReferences {
		if reference == "" || seen[reference] {
			return ErrInvalidExecution
		}
		seen[reference] = true
	}
	return nil
}

// LifecycleEventKind contains only facts submitted after the launch receipt.
// The accepted launch receipt itself is the canonical queued fact.
type LifecycleEventKind string

const (
	LifecycleEventStarted   LifecycleEventKind = "started"
	LifecycleEventCompleted LifecycleEventKind = "completed"
	LifecycleEventFailed    LifecycleEventKind = "failed"
	LifecycleEventCancelled LifecycleEventKind = "cancelled"
)

// ExecutionState is the consumer's persisted lifecycle state. Accepted is
// entered only from an accepted/replayed launch receipt, never from an event.
type ExecutionState string

const (
	ExecutionStateAccepted  ExecutionState = "accepted"
	ExecutionStateStarted   ExecutionState = "started"
	ExecutionStateCompleted ExecutionState = "completed"
	ExecutionStateFailed    ExecutionState = "failed"
	ExecutionStateCancelled ExecutionState = "cancelled"
)

type PlacementStatus string

const (
	PlacementStatusFinished     PlacementStatus = "finished"
	PlacementStatusForfeited    PlacementStatus = "forfeited"
	PlacementStatusDisqualified PlacementStatus = "disqualified"
	PlacementStatusDidNotFinish PlacementStatus = "did-not-finish"
)

type Placement struct {
	SlotOrdinal uint16          `json:"slotOrdinal"`
	EntryID     EntryID         `json:"entryID"`
	Rank        uint16          `json:"rank"`
	Status      PlacementStatus `json:"status"`
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

type RecordedProvenance struct {
	ParticipantArtifactDigests  []ArtifactDigest `json:"participantArtifactDigests"`
	ProviderConfigurationDigest ArtifactDigest   `json:"providerConfigurationDigest"`
	RuntimeDigest               ArtifactDigest   `json:"runtimeDigest"`
	RulesDigest                 ArtifactDigest   `json:"rulesDigest"`
	LimitProfileDigest          ArtifactDigest   `json:"limitProfileDigest"`
	SeedDigest                  ArtifactDigest   `json:"seedDigest"`
	EventLogDigest              ArtifactDigest   `json:"eventLogDigest"`
	ExecutionPayloadDigest      PayloadDigest    `json:"executionPayloadDigest"`
}

type CompletionEvidence struct {
	Replay             TerminalReplay     `json:"replay"`
	RecordedProvenance RecordedProvenance `json:"recordedProvenance"`
}

type ExecutionResult struct {
	Placements []Placement        `json:"placements"`
	Evidence   CompletionEvidence `json:"evidence"`
}

// FailureEvidence contains only facts that actually exist at the failed phase.
// A pre-start cancellation legitimately has nil Evidence.
type FailureEvidence struct {
	RuntimeDigest          ArtifactDigest  `json:"runtimeDigest,omitempty"`
	EventLogDigest         ArtifactDigest  `json:"eventLogDigest,omitempty"`
	ExecutionPayloadDigest PayloadDigest   `json:"executionPayloadDigest,omitempty"`
	Replay                 *TerminalReplay `json:"replay,omitempty"`
}

// ExecutionFailure models failed/cancelled states without fabricated results,
// placements, runtime facts, seeds, event logs, or replays.
type ExecutionFailure struct {
	Code             string           `json:"code"`
	AdjudicationCode string           `json:"adjudicationCode,omitempty"`
	Evidence         *FailureEvidence `json:"evidence,omitempty"`
}

// ExecutionEventPayload is the complete event digest input. Completed carries
// Result; failed/cancelled carry Failure; started carries neither.
type ExecutionEventPayload struct {
	ID                 EventID            `json:"id"`
	Kind               LifecycleEventKind `json:"kind"`
	CompetitionID      CompetitionID      `json:"competitionID"`
	ContestID          ContestID          `json:"contestID"`
	RequestID          ExecutionRequestID `json:"requestID"`
	ProviderID         ProviderID         `json:"providerID"`
	AdapterID          AdapterID          `json:"adapterID"`
	ProviderInstanceID ProviderInstanceID `json:"providerInstanceID"`
	CommandID          CommandID          `json:"commandID"`
	OccurredAt         time.Time          `json:"occurredAt"`
	Result             *ExecutionResult   `json:"result,omitempty"`
	Failure            *ExecutionFailure  `json:"failure,omitempty"`
}

type ExecutionEvent struct {
	ID                 EventID            `json:"id"`
	Kind               LifecycleEventKind `json:"kind"`
	CompetitionID      CompetitionID      `json:"competitionID"`
	ContestID          ContestID          `json:"contestID"`
	RequestID          ExecutionRequestID `json:"requestID"`
	ProviderID         ProviderID         `json:"providerID"`
	AdapterID          AdapterID          `json:"adapterID"`
	ProviderInstanceID ProviderInstanceID `json:"providerInstanceID"`
	CommandID          CommandID          `json:"commandID"`
	OccurredAt         time.Time          `json:"occurredAt"`
	Result             *ExecutionResult   `json:"result,omitempty"`
	Failure            *ExecutionFailure  `json:"failure,omitempty"`
	TypedPayloadDigest PayloadDigest      `json:"typedPayloadDigest"`
}

func NewExecutionEvent(payload ExecutionEventPayload) (ExecutionEvent, error) {
	digest, err := DigestExecutionEventPayload(payload)
	if err != nil {
		return ExecutionEvent{}, err
	}
	event := ExecutionEvent{
		ID: payload.ID, Kind: payload.Kind, CompetitionID: payload.CompetitionID,
		ContestID: payload.ContestID, RequestID: payload.RequestID,
		ProviderID: payload.ProviderID, AdapterID: payload.AdapterID,
		ProviderInstanceID: payload.ProviderInstanceID, CommandID: payload.CommandID,
		OccurredAt: payload.OccurredAt, Result: payload.Result, Failure: payload.Failure,
		TypedPayloadDigest: digest,
	}
	if err := ValidateExecutionEvent(event); err != nil {
		return ExecutionEvent{}, err
	}
	return event, nil
}

func (e ExecutionEvent) Payload() ExecutionEventPayload {
	return ExecutionEventPayload{
		ID: e.ID, Kind: e.Kind, CompetitionID: e.CompetitionID, ContestID: e.ContestID,
		RequestID: e.RequestID, ProviderID: e.ProviderID, AdapterID: e.AdapterID,
		ProviderInstanceID: e.ProviderInstanceID, CommandID: e.CommandID,
		OccurredAt: e.OccurredAt, Result: e.Result, Failure: e.Failure,
	}
}

func DigestExecutionEventPayload(payload ExecutionEventPayload) (PayloadDigest, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return digestParts(executionEventDigestDomain, encoded), nil
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

func ValidateEventAcknowledgement(value EventAcknowledgement) error {
	switch value.Status {
	case EventAcknowledgementAccepted, EventAcknowledgementReplayed:
		return nil
	default:
		return ErrInvalidExecution
	}
}

func ValidateExecutionRequest(value ExecutionRequest) error {
	if value.ID == "" || value.ProviderID == "" || value.AdapterID == "" || value.CompetitionID == "" || value.ContestID == "" || value.CommandID == "" || value.GameID == "" || value.RulesetVersion == "" || value.Callback.Resource == "" || !validSHA256Digest(string(value.TypedPayloadDigest)) {
		return ErrInvalidExecution
	}
	seenArtifacts := map[PublicArtifactKind]bool{}
	for _, artifact := range value.RequestedPublicArtifacts {
		if artifact != PublicArtifactTerminalReplay || seenArtifacts[artifact] {
			return ErrInvalidExecution
		}
		seenArtifacts[artifact] = true
	}
	if err := validateExecutionProfile(value.Profile); err != nil {
		return err
	}
	digest, err := DigestExecutionRequestPayload(value.Payload())
	if err != nil || digest != value.TypedPayloadDigest {
		return ErrInvalidExecution
	}
	return nil
}

func validateExecutionProfile(profile ExecutionProfile) error {
	switch profile.Kind {
	case ExecutionProfileProviderExecuted:
		if profile.ProviderExecuted == nil || profile.ParticipantScheduled != nil {
			return ErrInvalidExecution
		}
		value := profile.ProviderExecuted
		if value.Configuration.Version == "" || len(value.Slots) == 0 || !value.Deadline.IsZero() && !value.NotBefore.IsZero() && value.Deadline.Before(value.NotBefore) {
			return ErrInvalidExecution
		}
		seenOrdinal, seenEntry, seenVersion := map[uint16]bool{}, map[EntryID]bool{}, map[ParticipantVersionID]bool{}
		for index, slot := range value.Slots {
			if slot.Ordinal != uint16(index) || slot.EntryID == "" || slot.Participant.ParticipantID == "" || slot.Participant.ParticipantVersionID == "" || !validSHA256Digest(string(slot.Participant.ArtifactDigest)) || seenOrdinal[slot.Ordinal] || seenEntry[slot.EntryID] || seenVersion[slot.Participant.ParticipantVersionID] {
				return ErrInvalidExecution
			}
			seenOrdinal[slot.Ordinal], seenEntry[slot.EntryID], seenVersion[slot.Participant.ParticipantVersionID] = true, true, true
		}
	case ExecutionProfileParticipantScheduled:
		if profile.ParticipantScheduled == nil || profile.ProviderExecuted != nil || profile.ParticipantScheduled.StartsAt.IsZero() || len(profile.ParticipantScheduled.Slots) == 0 {
			return ErrInvalidExecution
		}
		seenEntry, seenParticipant := map[EntryID]bool{}, map[ParticipantID]bool{}
		for index, slot := range profile.ParticipantScheduled.Slots {
			if slot.Ordinal != uint16(index) || slot.EntryID == "" || len(slot.Participants) == 0 || seenEntry[slot.EntryID] {
				return ErrInvalidExecution
			}
			seenEntry[slot.EntryID] = true
			for _, participant := range slot.Participants {
				if participant == "" || seenParticipant[participant] {
					return ErrInvalidExecution
				}
				seenParticipant[participant] = true
			}
		}
	default:
		return ErrInvalidExecution
	}
	return nil
}

func ValidateExecutionEvent(value ExecutionEvent) error {
	if value.ID == "" || value.CompetitionID == "" || value.ContestID == "" || value.RequestID == "" || value.ProviderID == "" || value.AdapterID == "" || value.ProviderInstanceID == "" || value.CommandID == "" || value.OccurredAt.IsZero() || !validSHA256Digest(string(value.TypedPayloadDigest)) {
		return ErrInvalidExecution
	}
	switch value.Kind {
	case LifecycleEventStarted:
		if value.Result != nil || value.Failure != nil {
			return ErrInvalidExecution
		}
	case LifecycleEventCompleted:
		if value.Result == nil || value.Failure != nil || len(value.Result.Placements) == 0 || len(value.Result.Evidence.RecordedProvenance.ParticipantArtifactDigests) != len(value.Result.Placements) || validateCompletionEvidence(value.Result.Evidence) != nil || validatePlacements(value.Result.Placements) != nil {
			return ErrInvalidExecution
		}
	case LifecycleEventFailed, LifecycleEventCancelled:
		if value.Result != nil || value.Failure == nil || value.Failure.Code == "" || validateFailureEvidence(value.Failure.Evidence) != nil {
			return ErrInvalidExecution
		}
	default:
		return ErrInvalidExecution
	}
	digest, err := DigestExecutionEventPayload(value.Payload())
	if err != nil || digest != value.TypedPayloadDigest {
		return ErrInvalidExecution
	}
	return nil
}

func validatePlacements(placements []Placement) error {
	seenEntries, seenSlots := map[EntryID]bool{}, map[uint16]bool{}
	var previous uint16
	var previousSlot uint16
	for index, placement := range placements {
		if placement.EntryID == "" || int(placement.SlotOrdinal) >= len(placements) || placement.Rank == 0 || seenEntries[placement.EntryID] || seenSlots[placement.SlotOrdinal] || placement.Rank < previous || index == 0 && placement.Rank != 1 || index > 0 && placement.Rank > previous && placement.Rank != uint16(index+1) || index > 0 && placement.Rank == previous && placement.SlotOrdinal <= previousSlot {
			return ErrInvalidExecution
		}
		switch placement.Status {
		case PlacementStatusFinished, PlacementStatusForfeited, PlacementStatusDisqualified, PlacementStatusDidNotFinish:
		default:
			return ErrInvalidExecution
		}
		seenEntries[placement.EntryID], seenSlots[placement.SlotOrdinal], previous, previousSlot = true, true, placement.Rank, placement.SlotOrdinal
	}
	return nil
}

// ValidateExecutionEventForExecution binds an otherwise valid lifecycle event
// to the immutable request and provider receipt that created its instance. For
// a completed event it also proves that every frozen slot appears exactly once
// and, for provider-executed contests, that the provenance artifact identities
// are the exact request artifacts in slot order.
func ValidateExecutionEventForExecution(event ExecutionEvent, request ExecutionRequest, receipt ExecutionReceipt) error {
	if ValidateExecutionEvent(event) != nil || ValidateExecutionReceiptForRequest(receipt, request) != nil || event.CompetitionID != request.CompetitionID || event.ContestID != request.ContestID || event.RequestID != request.ID || event.ProviderID != request.ProviderID || event.AdapterID != request.AdapterID || event.ProviderInstanceID != receipt.ProviderInstanceID {
		return ErrInvalidExecution
	}
	if event.Kind != LifecycleEventCompleted {
		return nil
	}

	var entries []EntryID
	var artifactDigests []ArtifactDigest
	switch request.Profile.Kind {
	case ExecutionProfileProviderExecuted:
		entries = make([]EntryID, len(request.Profile.ProviderExecuted.Slots))
		artifactDigests = make([]ArtifactDigest, len(request.Profile.ProviderExecuted.Slots))
		for index, slot := range request.Profile.ProviderExecuted.Slots {
			entries[index] = slot.EntryID
			artifactDigests[index] = slot.Participant.ArtifactDigest
		}
	case ExecutionProfileParticipantScheduled:
		entries = make([]EntryID, len(request.Profile.ParticipantScheduled.Slots))
		for index, slot := range request.Profile.ParticipantScheduled.Slots {
			entries[index] = slot.EntryID
		}
	default:
		return ErrInvalidExecution
	}
	if event.Result == nil || len(event.Result.Placements) != len(entries) {
		return ErrInvalidExecution
	}
	for _, placement := range event.Result.Placements {
		if int(placement.SlotOrdinal) >= len(entries) || placement.EntryID != entries[placement.SlotOrdinal] {
			return ErrInvalidExecution
		}
	}
	if artifactDigests != nil {
		actual := event.Result.Evidence.RecordedProvenance.ParticipantArtifactDigests
		if len(actual) != len(artifactDigests) {
			return ErrInvalidExecution
		}
		for index := range artifactDigests {
			if actual[index] != artifactDigests[index] {
				return ErrInvalidExecution
			}
		}
	}
	return nil
}

func validateCompletionEvidence(value CompletionEvidence) error {
	if len(value.RecordedProvenance.ParticipantArtifactDigests) == 0 || !validSHA256Digest(string(value.RecordedProvenance.ProviderConfigurationDigest)) || !validSHA256Digest(string(value.RecordedProvenance.RuntimeDigest)) || !validSHA256Digest(string(value.RecordedProvenance.RulesDigest)) || !validSHA256Digest(string(value.RecordedProvenance.LimitProfileDigest)) || !validSHA256Digest(string(value.RecordedProvenance.SeedDigest)) || !validSHA256Digest(string(value.RecordedProvenance.EventLogDigest)) || !validSHA256Digest(string(value.RecordedProvenance.ExecutionPayloadDigest)) {
		return ErrInvalidExecution
	}
	for _, digest := range value.RecordedProvenance.ParticipantArtifactDigests {
		if !validSHA256Digest(string(digest)) {
			return ErrInvalidExecution
		}
	}
	switch value.Replay.State {
	case ReplayAvailable:
		if value.Replay.Reference == "" {
			return ErrInvalidExecution
		}
	case ReplayProcessing, ReplayUnavailable:
		if value.Replay.Reference != "" {
			return ErrInvalidExecution
		}
	default:
		return ErrInvalidExecution
	}
	return nil
}

func validateFailureEvidence(value *FailureEvidence) error {
	if value == nil {
		return nil
	}
	if value.RuntimeDigest == "" && value.EventLogDigest == "" && value.ExecutionPayloadDigest == "" && value.Replay == nil {
		return ErrInvalidExecution
	}
	if value.RuntimeDigest != "" && !validSHA256Digest(string(value.RuntimeDigest)) || value.EventLogDigest != "" && !validSHA256Digest(string(value.EventLogDigest)) || value.ExecutionPayloadDigest != "" && !validSHA256Digest(string(value.ExecutionPayloadDigest)) {
		return ErrInvalidExecution
	}
	if value.Replay == nil {
		return nil
	}
	switch value.Replay.State {
	case ReplayAvailable:
		if value.Replay.Reference == "" {
			return ErrInvalidExecution
		}
	case ReplayProcessing, ReplayUnavailable:
		if value.Replay.Reference != "" {
			return ErrInvalidExecution
		}
	default:
		return ErrInvalidExecution
	}
	return nil
}

func ValidateLifecycleTransition(previous ExecutionState, next LifecycleEventKind) error {
	valid := map[ExecutionState]map[LifecycleEventKind]bool{
		ExecutionStateAccepted: {
			LifecycleEventStarted:   true,
			LifecycleEventFailed:    true,
			LifecycleEventCancelled: true,
		},
		ExecutionStateStarted: {
			LifecycleEventCompleted: true,
			LifecycleEventFailed:    true,
			LifecycleEventCancelled: true,
		},
	}
	if !valid[previous][next] {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, previous, next)
	}
	return nil
}
