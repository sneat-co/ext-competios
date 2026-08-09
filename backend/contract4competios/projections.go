package contract4competios

import "time"

// ProjectionPurpose is the caller's requested redaction purpose. It is never
// authority or role evidence; the application independently authorises the
// principal before choosing which fields and actions to return.
type ProjectionPurpose string

const (
	ProjectionPurposePublic      ProjectionPurpose = "public"
	ProjectionPurposeParticipant ProjectionPurpose = "participant"
	ProjectionPurposeTeamManager ProjectionPurpose = "team-manager"
	ProjectionPurposeOrganiser   ProjectionPurpose = "organiser"
)

type ParticipantReference struct {
	Kind              ParticipantKind `json:"kind"`
	UserID            UserID          `json:"userID,omitempty"`
	TeamSpaceID       SpaceID         `json:"teamSpaceID,omitempty"`
	SelectedMemberIDs []UserID        `json:"selectedMemberIDs,omitempty"`
}

type EntryState string

const (
	EntryInvited                   EntryState = "invited"
	EntryAwaitingTeamAcceptance    EntryState = "awaiting-team-acceptance"
	EntryAwaitingOrganiserApproval EntryState = "awaiting-organiser-approval"
	EntryEnrolled                  EntryState = "enrolled"
	EntryWaitlisted                EntryState = "waitlisted"
	EntryRejected                  EntryState = "rejected"
	EntryWithdrawn                 EntryState = "withdrawn"
)

type EntryTransitionAuditProjection struct {
	CommandID  CommandID  `json:"commandID"`
	FromState  EntryState `json:"fromState,omitempty"`
	ToState    EntryState `json:"toState"`
	OccurredAt time.Time  `json:"occurredAt"`
}

type EntryCommandBase struct {
	Caller        CallerContext   `json:"caller"`
	Metadata      CommandMetadata `json:"metadata"`
	CompetitionID CompetitionID   `json:"competitionID"`
	EntryID       EntryID         `json:"entryID,omitempty"`
}

type InviteEntryCommand struct {
	EntryCommandBase
	Participant ParticipantReference `json:"participant"`
}

type RequestEntryCommand struct {
	EntryCommandBase
	Participant ParticipantReference `json:"participant"`
}

type AcceptEntryCommand struct {
	EntryCommandBase
}

type ApproveEntryCommand struct {
	EntryCommandBase
}

type RejectEntryCommand struct {
	EntryCommandBase
}

type RevokeInvitationCommand struct {
	EntryCommandBase
}

type WithdrawEntryCommand struct {
	EntryCommandBase
}

type LockEnrolmentCommand struct {
	Caller        CallerContext   `json:"caller"`
	Metadata      CommandMetadata `json:"metadata"`
	CompetitionID CompetitionID   `json:"competitionID"`
}

type SeedAssignment struct {
	EntryID EntryID `json:"entryID"`
	Seed    uint16  `json:"seed"`
}

type SeedCompetitionCommand struct {
	Caller        CallerContext    `json:"caller"`
	Metadata      CommandMetadata  `json:"metadata"`
	CompetitionID CompetitionID    `json:"competitionID"`
	Assignments   []SeedAssignment `json:"assignments"`
}

type EntryParticipantProjection struct {
	ParticipantID ParticipantID   `json:"participantID,omitempty"`
	Kind          ParticipantKind `json:"kind"`
	DisplayName   string          `json:"displayName,omitempty"`
}

type EntryProjection struct {
	EntryID          EntryID                          `json:"entryID"`
	AggregateVersion AggregateVersion                 `json:"aggregateVersion"`
	ContentRevision  ContentRevision                  `json:"contentRevision"`
	State            EntryState                       `json:"state"`
	Participant      EntryParticipantProjection       `json:"participant"`
	InvitationID     InvitationID                     `json:"invitationID,omitempty"`
	WaitlistPosition uint16                           `json:"waitlistPosition,omitempty"`
	AvailableActions []string                         `json:"availableActions,omitempty"`
	History          []EntryTransitionAuditProjection `json:"history,omitempty"`
}

type WaitlistEntryProjection struct {
	EntryID          EntryID                    `json:"entryID"`
	AggregateVersion AggregateVersion           `json:"aggregateVersion"`
	ContentRevision  ContentRevision            `json:"contentRevision"`
	Position         uint16                     `json:"position"`
	Participant      EntryParticipantProjection `json:"participant"`
}

type WaitlistProjection struct {
	AggregateVersion AggregateVersion          `json:"aggregateVersion"`
	ContentRevision  ContentRevision           `json:"contentRevision"`
	Entries          []WaitlistEntryProjection `json:"entries"`
}

type ContestSlotProjection struct {
	Slot        uint8                      `json:"slot"`
	EntryID     EntryID                    `json:"entryID,omitempty"`
	Participant EntryParticipantProjection `json:"participant,omitempty"`
}

type ContestProjection struct {
	ContestID        ContestID               `json:"contestID"`
	AggregateVersion AggregateVersion        `json:"aggregateVersion"`
	ContentRevision  ContentRevision         `json:"contentRevision"`
	StageID          StageID                 `json:"stageID"`
	ContestIndex     uint16                  `json:"contestIndex"`
	ParticipantCount uint8                   `json:"participantCount"`
	State            string                  `json:"state"`
	StartsAt         *time.Time              `json:"startsAt,omitempty"`
	Slots            []ContestSlotProjection `json:"slots"`
}

type BracketStageProjection struct {
	StageID  StageID             `json:"stageID"`
	Contests []ContestProjection `json:"contests"`
}

type BracketProjection struct {
	AggregateVersion AggregateVersion         `json:"aggregateVersion"`
	ContentRevision  ContentRevision          `json:"contentRevision"`
	Stages           []BracketStageProjection `json:"stages"`
}

type SchedulingState string

const (
	SchedulingPending   SchedulingState = "pending"
	SchedulingScheduled SchedulingState = "scheduled"
	SchedulingStarted   SchedulingState = "started"
	SchedulingComplete  SchedulingState = "complete"
)

type SchedulingProjection struct {
	State    SchedulingState `json:"state"`
	StartsAt *time.Time      `json:"startsAt,omitempty"`
}

type CompetitionState string

const (
	CompetitionEnrolling  CompetitionState = "enrolling"
	CompetitionLocked     CompetitionState = "locked"
	CompetitionScheduled  CompetitionState = "scheduled"
	CompetitionInProgress CompetitionState = "in-progress"
	CompetitionCompleted  CompetitionState = "completed"
	CompetitionCancelled  CompetitionState = "cancelled"
)

type CompetitionProjection struct {
	CompetitionID    CompetitionID         `json:"competitionID"`
	AggregateVersion AggregateVersion      `json:"aggregateVersion"`
	ContentRevision  ContentRevision       `json:"contentRevision"`
	State            CompetitionState      `json:"state"`
	Title            string                `json:"title"`
	GameID           GameID                `json:"gameID"`
	RulesetVersion   RulesetVersion        `json:"rulesetVersion"`
	ParticipantKind  ParticipantKind       `json:"participantKind"`
	Visibility       Visibility            `json:"visibility"`
	EnrolmentPolicy  EnrolmentPolicy       `json:"enrolmentPolicy"`
	Format           FormatSelection       `json:"format"`
	Capacity         uint16                `json:"capacity"`
	EnrolledCount    uint16                `json:"enrolledCount"`
	WaitlistedCount  uint16                `json:"waitlistedCount"`
	EnrolmentLocked  bool                  `json:"enrolmentLocked"`
	Scheduling       *SchedulingProjection `json:"scheduling,omitempty"`
	Entries          []EntryProjection     `json:"entries,omitempty"`
	ApprovalQueue    []EntryProjection     `json:"approvalQueue,omitempty"`
	Waitlist         *WaitlistProjection   `json:"waitlist,omitempty"`
	Bracket          *BracketProjection    `json:"bracket,omitempty"`
	AvailableActions []string              `json:"availableActions,omitempty"`
}

type TeamRosterMemberProjection struct {
	MemberID    UserID `json:"memberID"`
	DisplayName string `json:"displayName,omitempty"`
}

type TeamRosterProjection struct {
	TeamSpaceID      SpaceID                      `json:"teamSpaceID"`
	AggregateVersion AggregateVersion             `json:"aggregateVersion"`
	ContentRevision  ContentRevision              `json:"contentRevision"`
	Members          []TeamRosterMemberProjection `json:"members"`
}
