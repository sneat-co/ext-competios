package contract4competios

import "context"

type EntryListRequest struct {
	Caller        CallerContext     `json:"caller"`
	CompetitionID CompetitionID     `json:"competitionID"`
	Purpose       ProjectionPurpose `json:"purpose"`
}

type GetCompetitionRequest struct {
	Caller        CallerContext     `json:"caller"`
	CompetitionID CompetitionID     `json:"competitionID"`
	Purpose       ProjectionPurpose `json:"purpose"`
}

type GetEntryRequest struct {
	Caller        CallerContext     `json:"caller"`
	CompetitionID CompetitionID     `json:"competitionID"`
	EntryID       EntryID           `json:"entryID"`
	Purpose       ProjectionPurpose `json:"purpose"`
}

type GetTeamRosterRequest struct {
	Caller      CallerContext `json:"caller"`
	TeamSpaceID SpaceID       `json:"teamSpaceID"`
}

type DraftOutcome struct {
	Draft    DraftProjection `json:"draft"`
	Replayed bool            `json:"replayed,omitempty"`
}

type SubmitDraftOutcome struct {
	Draft       DraftProjection       `json:"draft"`
	Competition CompetitionProjection `json:"competition"`
	Replayed    bool                  `json:"replayed,omitempty"`
}

type EntryOutcome struct {
	CompetitionID CompetitionID   `json:"competitionID"`
	Entry         EntryProjection `json:"entry"`
	Replayed      bool            `json:"replayed,omitempty"`
}

type CompetitionOutcome struct {
	Competition CompetitionProjection `json:"competition"`
	Replayed    bool                  `json:"replayed,omitempty"`
}

// IndividualEnrollmentApplication is the additive M4a application capability
// for individual competition enrolment. It deliberately remains independent
// of the released draft application interfaces.
type IndividualEnrollmentApplication interface {
	GetCompetition(context.Context, GetCompetitionRequest) (CompetitionProjection, error)
	GetEntry(context.Context, GetEntryRequest) (EntryProjection, error)
	RequestEntry(context.Context, RequestEntryCommand) (EntryOutcome, error)
	WithdrawEntry(context.Context, WithdrawEntryCommand) (EntryOutcome, error)
}

// ManagedEnrollmentApplication is the additive enrolment capability for
// invitation, team-acceptance, and organiser-managed participation flows. It
// inherits the individual-enrolment reads and commands so every client has one
// consistent minimal competition and entry projection boundary. The
// application, not callers, resolves all organiser and team authority,
// evidence, accepted roster, lifecycle state, and resulting outcome details.
type ManagedEnrollmentApplication interface {
	IndividualEnrollmentApplication
	InviteEntry(context.Context, InviteEntryCommand) (EntryOutcome, error)
	AcceptEntry(context.Context, AcceptEntryCommand) (EntryOutcome, error)
	ApproveEntry(context.Context, ApproveEntryCommand) (EntryOutcome, error)
	RejectEntry(context.Context, RejectEntryCommand) (EntryOutcome, error)
	RevokeInvitation(context.Context, RevokeInvitationCommand) (EntryOutcome, error)
}

// DraftApplication is the Milestone 1 surface-neutral authenticated draft
// boundary. Host surfaces provide CallerContext; the application authorises
// it independently before every protected operation. Later competition,
// enrolment and provisional-ownership APIs deliberately live in separate
// narrow interfaces when their delivery slices exist.
type DraftApplication interface {
	GetCapabilities(context.Context, CapabilitiesRequest) (CapabilitiesProjection, error)
	StartOrResumeDraft(context.Context, StartOrResumeDraftCommand) (DraftOutcome, error)
	GetDraft(context.Context, GetDraftRequest) (DraftProjection, error)
	ListDrafts(context.Context, ListDraftsRequest) (DraftListProjection, error)
	UpdateDraft(context.Context, UpdateDraftCommand) (DraftOutcome, error)
	AbandonDraft(context.Context, AbandonDraftCommand) (DraftOutcome, error)
	SubmitDraft(context.Context, SubmitDraftCommand) (SubmitDraftOutcome, error)
}

// MultiDraftApplication is the additive Milestone 2 application capability.
// Milestone 1 hosts can continue depending on DraftApplication unchanged.
type MultiDraftApplication interface {
	DraftApplication
	CreateAdditionalDraft(context.Context, CreateAdditionalDraftCommand) (DraftOutcome, error)
	MakePrimaryDraft(context.Context, MakePrimaryDraftCommand) (DraftOutcome, error)
}

// ProvisionalDraftApplication is the additive Milestone 3 capability. The
// inherited draft methods accept either authenticated callers or
// server-validated provisional Telegram callers where their individual
// command semantics permit it. ClaimAndSubmitDraft itself requires an
// authenticated caller and independently resolved provisional source proof.
type ProvisionalDraftApplication interface {
	MultiDraftApplication
	ClaimAndSubmitDraft(context.Context, ClaimAndSubmitDraftCommand) (SubmitDraftOutcome, error)
}
