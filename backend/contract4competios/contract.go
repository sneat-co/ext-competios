package contract4competios

import (
	"context"
	"errors"
	"time"
)

// ExtensionID is Competios' stable global Firestore extension namespace.
// Hosts and adapters use it to place Competios-owned data below
// /ext/competios/... without coupling to an implementation package.
const ExtensionID = "competios"

var ErrCommandConflict = errors.New("competios contract: command ID reused with a different payload")

type CompetitionID string
type ContestID string
type EntryID string
type ResultID string
type ContestStartID string

type LineupMember struct {
	UserID string `json:"userID"`
}

type SideAssignment struct {
	EntryID EntryID        `json:"entryID"`
	Side    string         `json:"side"`
	Lineup  []LineupMember `json:"lineup"`
}

type LaunchRequest struct {
	CompetitionID  CompetitionID    `json:"competitionID"`
	ContestID      ContestID        `json:"contestID"`
	CommandID      string           `json:"commandID"`
	GameID         string           `json:"gameID"`
	RulesetVersion string           `json:"rulesetVersion"`
	Assignments    []SideAssignment `json:"assignments"`
	StartsAt       time.Time        `json:"startsAt"`
}

type LaunchOutcome struct {
	GameInstanceID string `json:"gameInstanceID"`
	Replayed       bool   `json:"replayed"`
}

type GameLauncher interface {
	LaunchContest(ctx context.Context, request LaunchRequest) (LaunchOutcome, error)
}

// ContestStart is the immutable game-side attestation that a scheduled contest
// actually entered play. GameAdapterID and GameInstanceID identify the trusted
// configured game adapter and the exact game instance bound at launch.
type ContestStart struct {
	ID             ContestStartID `json:"id"`
	CompetitionID  CompetitionID  `json:"competitionID"`
	ContestID      ContestID      `json:"contestID"`
	GameAdapterID  string         `json:"gameAdapterID"`
	GameInstanceID string         `json:"gameInstanceID"`
	StartedAt      time.Time      `json:"startedAt"`
}

type ContestStartAcknowledgementStatus string

const (
	ContestStartAcknowledgementAccepted ContestStartAcknowledgementStatus = "accepted"
	ContestStartAcknowledgementReplayed ContestStartAcknowledgementStatus = "replayed"
)

type ContestStartAcknowledgement struct {
	Status ContestStartAcknowledgementStatus `json:"status"`
}

// ContestStartSink accepts durable, idempotent actual-play attestations from
// a configured game adapter. commandID is a durable delivery command and must
// reject reuse with a payload different from start.
type ContestStartSink interface {
	SubmitContestStart(
		ctx context.Context,
		commandID string,
		start ContestStart,
	) (ContestStartAcknowledgement, error)
}

type PlacementStatus string

const (
	PlacementStatusFinished     PlacementStatus = "finished"
	PlacementStatusForfeited    PlacementStatus = "forfeited"
	PlacementStatusDisqualified PlacementStatus = "disqualified"
	PlacementStatusDidNotFinish PlacementStatus = "did-not-finish"
)

type Placement struct {
	EntryID EntryID         `json:"entryID"`
	Rank    int             `json:"rank"`
	Status  PlacementStatus `json:"status"`
}

type OrderedResult struct {
	ID                 ResultID      `json:"id"`
	CompetitionID      CompetitionID `json:"competitionID"`
	ContestID          ContestID     `json:"contestID"`
	RulesetVersion     string        `json:"rulesetVersion"`
	GameAdapterID      string        `json:"gameAdapterID"`
	GameInstanceID     string        `json:"gameInstanceID"`
	SupersedesResultID ResultID      `json:"supersedesResultID,omitempty"`
	Placements         []Placement   `json:"placements"`
	CompletedAt        time.Time     `json:"completedAt"`
}

type ResultAcknowledgementStatus string

const (
	ResultAcknowledgementAccepted ResultAcknowledgementStatus = "accepted"
	ResultAcknowledgementReplayed ResultAcknowledgementStatus = "replayed"
)

type ResultAcknowledgement struct {
	Status ResultAcknowledgementStatus `json:"status"`
}

type ResultSink interface {
	SubmitResult(
		ctx context.Context,
		commandID string,
		result OrderedResult,
	) (ResultAcknowledgement, error)
}
