package contract4competios

import "time"

type ParticipantKind string

const (
	ParticipantIndividual ParticipantKind = "individual"
	ParticipantTeam       ParticipantKind = "team-space"
)

type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
)

type EnrolmentPolicy string

const (
	EnrolmentOpen             EnrolmentPolicy = "open"
	EnrolmentApprovalRequired EnrolmentPolicy = "approval-required"
	EnrolmentInviteOnly       EnrolmentPolicy = "invite-only"
)

type SeedingPolicy string

const (
	SeedingRandom SeedingPolicy = "random"
	SeedingManual SeedingPolicy = "manual"
)

type SchedulingPolicy string

const (
	SchedulingAfterDraw SchedulingPolicy = "after-draw"
)

type DraftContent struct {
	Title            string           `json:"title"`
	GameID           GameID           `json:"gameID"`
	RulesetVersion   RulesetVersion   `json:"rulesetVersion"`
	ParticipantKind  ParticipantKind  `json:"participantKind"`
	Visibility       Visibility       `json:"visibility"`
	EnrolmentPolicy  EnrolmentPolicy  `json:"enrolmentPolicy"`
	Format           FormatSelection  `json:"format"`
	SeedingPolicy    SeedingPolicy    `json:"seedingPolicy"`
	SchedulingPolicy SchedulingPolicy `json:"schedulingPolicy"`
	EconomicMode     EconomicMode     `json:"economicMode"`
}

// DraftPatch uses pointers so an omitted field is distinct from an explicit
// zero value. It contains no provider attestations or authority evidence;
// implementations obtain those from trusted configured providers.
type DraftPatch struct {
	Title            *string           `json:"title,omitempty"`
	GameID           *GameID           `json:"gameID,omitempty"`
	RulesetVersion   *RulesetVersion   `json:"rulesetVersion,omitempty"`
	ParticipantKind  *ParticipantKind  `json:"participantKind,omitempty"`
	Visibility       *Visibility       `json:"visibility,omitempty"`
	EnrolmentPolicy  *EnrolmentPolicy  `json:"enrolmentPolicy,omitempty"`
	Format           *FormatSelection  `json:"format,omitempty"`
	SeedingPolicy    *SeedingPolicy    `json:"seedingPolicy,omitempty"`
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`
	EconomicMode     *EconomicMode     `json:"economicMode,omitempty"`
}

type DraftState string

const (
	DraftActive    DraftState = "active"
	DraftSubmitted DraftState = "submitted"
	DraftAbandoned DraftState = "abandoned"
	DraftExpired   DraftState = "expired"
)

type DraftValidationError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type DraftAction string

const (
	DraftActionEdit        DraftAction = "edit"
	DraftActionSubmit      DraftAction = "submit"
	DraftActionAbandon     DraftAction = "abandon"
	DraftActionMakePrimary DraftAction = "make-primary"
)

type DraftProjection struct {
	DraftID                DraftID                `json:"draftID"`
	AggregateVersion       AggregateVersion       `json:"aggregateVersion"`
	ContentRevision        ContentRevision        `json:"contentRevision"`
	State                  DraftState             `json:"state"`
	OwnerKind              PrincipalKind          `json:"ownerKind,omitempty"`
	OwningSpaceID          SpaceID                `json:"owningSpaceID,omitempty"`
	Content                DraftContent           `json:"content"`
	Capacity               uint16                 `json:"capacity,omitempty"`
	CreatedAt              time.Time              `json:"createdAt"`
	UpdatedAt              time.Time              `json:"updatedAt"`
	ExpiresAt              time.Time              `json:"expiresAt"`
	SubmittedCompetitionID CompetitionID          `json:"submittedCompetitionID,omitempty"`
	ValidationErrors       []DraftValidationError `json:"validationErrors,omitempty"`
	AvailableActions       []DraftAction          `json:"availableActions,omitempty"`
}

type DraftListProjection struct {
	PrimaryDraftID DraftID           `json:"primaryDraftID,omitempty"`
	Drafts         []DraftProjection `json:"drafts"`
}

type StartOrResumeDraftCommand struct {
	Caller   CallerContext   `json:"caller"`
	Metadata CommandMetadata `json:"metadata"`
	GameID   GameID          `json:"gameID"`
}

type CreateAdditionalDraftCommand struct {
	Caller   CallerContext   `json:"caller"`
	Metadata CommandMetadata `json:"metadata"`
	GameID   GameID          `json:"gameID"`
}

type MakePrimaryDraftCommand struct {
	Caller   CallerContext   `json:"caller"`
	Metadata CommandMetadata `json:"metadata"`
	DraftID  DraftID         `json:"draftID"`
}

type GetDraftRequest struct {
	Caller  CallerContext `json:"caller"`
	DraftID DraftID       `json:"draftID"`
}

type ListDraftsRequest struct {
	Caller CallerContext `json:"caller"`
	GameID GameID        `json:"gameID,omitempty"`
}

type UpdateDraftCommand struct {
	Caller   CallerContext   `json:"caller"`
	Metadata CommandMetadata `json:"metadata"`
	DraftID  DraftID         `json:"draftID"`
	Patch    DraftPatch      `json:"patch"`
}

type AbandonDraftCommand struct {
	Caller   CallerContext   `json:"caller"`
	Metadata CommandMetadata `json:"metadata"`
	DraftID  DraftID         `json:"draftID"`
}

type SubmitDraftCommand struct {
	Caller   CallerContext   `json:"caller"`
	Metadata CommandMetadata `json:"metadata"`
	DraftID  DraftID         `json:"draftID"`
}

// ClaimAndSubmitDraftCommand contains only the authenticated caller and draft
// reference. The application verifies the draft's stored provisional owner;
// the caller cannot assert or replace that identity.
type ClaimAndSubmitDraftCommand struct {
	Caller   CallerContext   `json:"caller"`
	Metadata CommandMetadata `json:"metadata"`
	DraftID  DraftID         `json:"draftID"`
}
