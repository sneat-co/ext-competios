package contract4competios

import (
	"context"
	"time"
)

// InteractionHandle is an opaque, short-lived capability reference. It is not
// a Series, edition, application, competition, roster, battle, Space, or user
// ID. Clients must treat it as an uninterpretable string.
type InteractionHandle string

// OperationRef is a server-issued opaque reference shared by every surface
// handle for one logical user operation. It contains no domain or person ID.
type OperationRef string

// BindingRef is a trusted codec/registry-issued sealed reference. Clients and
// presentation adapters cannot construct it from domain identifiers.
type BindingRef string

type InteractionPurpose string

const (
	InteractionPurposePublicDiscovery InteractionPurpose = "public-discovery"
	InteractionPurposePersonalStatus  InteractionPurpose = "personal-status"
	InteractionPurposeApplication     InteractionPurpose = "application"
	InteractionPurposeCommitment      InteractionPurpose = "commitment"
	InteractionPurposeQualifier       InteractionPurpose = "qualifier"
	InteractionPurposeEntryRoster     InteractionPurpose = "entry-roster"
	InteractionPurposePairing         InteractionPurpose = "pairing"
	InteractionPurposeHistory         InteractionPurpose = "history"
	InteractionPurposeNotification    InteractionPurpose = "notification"
)

type InteractionSurface string

const (
	InteractionSurfaceTelegram InteractionSurface = "telegram"
	InteractionSurfaceWeb      InteractionSurface = "web"
	InteractionSurfaceMiniApp  InteractionSurface = "mini-app"
)

// AuthenticatedPrincipalRef is an opaque host-validated Competios product
// principal. It is deliberately distinct from a Telegram account reference:
// Telegram validation identifies a transport account but never grants product
// authority on its own.
type AuthenticatedPrincipalRef string

// InteractionSubject is supplied by the authenticated host. Telegram is
// optional transport identity; PrincipalRef is the product identity used by
// Competios authority checks.
type InteractionSubject struct {
	PrincipalRef    AuthenticatedPrincipalRef `json:"-"`
	TelegramUserRef ValidatedTelegramUserRef  `json:"-"`
}

// InteractionAction is intentionally small. Complex application, review,
// qualifier, roster and result operations remain on the Competios web surface.
type InteractionAction string

const (
	InteractionActionRefresh       InteractionAction = "refresh"
	InteractionActionOpenWeb       InteractionAction = "open-web"
	InteractionActionAcceptInvite  InteractionAction = "accept"
	InteractionActionDeclineInvite InteractionAction = "decline"
	InteractionActionOpenQualifier InteractionAction = "qualifier"
	InteractionActionOpenEntry     InteractionAction = "entry"
	InteractionActionOpenPairing   InteractionAction = "pairing"
	InteractionActionOpenBattle    InteractionAction = "battle"
	InteractionActionOpenHistory   InteractionAction = "history"
)

type InteractionOutcomeStatus string

const (
	InteractionOutcomeCompleted  InteractionOutcomeStatus = "completed"
	InteractionOutcomeInFlight   InteractionOutcomeStatus = "in-flight"
	InteractionOutcomeStale      InteractionOutcomeStatus = "stale"
	InteractionOutcomeExpired    InteractionOutcomeStatus = "expired"
	InteractionOutcomeForbidden  InteractionOutcomeStatus = "forbidden"
	InteractionOutcomeValidation InteractionOutcomeStatus = "validation"
)

type EventRosterRole string

type TournamentNavigationKind string

// DocumentVersion identifies a published immutable Competios document revision.
// It is display-safe routing data, never an authority or a mutable document ID.
type DocumentVersion string

const (
	TournamentNavigationSeries    TournamentNavigationKind = "series"
	TournamentNavigationEdition   TournamentNavigationKind = "edition"
	TournamentNavigationQualifier TournamentNavigationKind = "player-qualifier"
	TournamentNavigationEntry     TournamentNavigationKind = "entry"
	TournamentNavigationPairing   TournamentNavigationKind = "pairing"
	TournamentNavigationBattle    TournamentNavigationKind = "battle"
	TournamentNavigationHistory   TournamentNavigationKind = "history"
	// TournamentNavigationCreatorApplication opens the authenticated creator
	// application flow. It is a typed route, not a URL or an application ID.
	TournamentNavigationCreatorApplication TournamentNavigationKind = "creator-application"
	// TournamentNavigationEditionParticipationRequirements opens the immutable
	// edition participation requirements selected by Competios. The host still
	// resolves the typed target to its configured origin.
	TournamentNavigationEditionParticipationRequirements TournamentNavigationKind = "edition-participation-requirements"
)

// TournamentNavigationTarget is a canonical renderer-independent route. The
// host owns conversion to an origin URL; arbitrary URLs never cross this API.
type TournamentNavigationTarget struct {
	Kind        TournamentNavigationKind `json:"kind"`
	SeriesSlug  string                   `json:"seriesSlug,omitempty"`
	EditionSlug string                   `json:"editionSlug,omitempty"`
	// DocumentVersion is required only for a participation-requirements route.
	// A typed target must always select the exact immutable rules accepted by a
	// participant; clients may not substitute a current/latest document.
	DocumentVersion DocumentVersion `json:"documentVersion,omitempty"`
	OpaqueRef       string          `json:"opaqueRef,omitempty"`
}

// These internal Competios relation types are retained for domain ports only.
// They must never be placed in a bot-facing projection, callback, URL, or
// notification payload.
type QualificationRelationID string
type CreatorProgrammeRecordID string
type EventRosterID string

const (
	EventRosterRoleCreator           EventRosterRole = "creator"
	EventRosterRoleCommunityChampion EventRosterRole = "community-champion"
)

// PublicTournamentSeriesProjection is deliberately viewer-safe. It has no
// internal identifiers, contacts, private review material, raw authority, or
// unconsented creator metrics. Slugs and typed route targets are public
// navigation values; the rendering host supplies the configured origin.
type PublicTournamentSeriesProjection struct {
	Slug              string                     `json:"slug"`
	Name              string                     `json:"name"`
	Official          bool                       `json:"official,omitempty"`
	EditorialRank     int                        `json:"editorialRank,omitempty"`
	CurrentEdition    string                     `json:"currentEdition,omitempty"`
	MinimumParties    uint16                     `json:"minimumParties,omitempty"`
	TargetParties     uint16                     `json:"targetParties,omitempty"`
	MaximumParties    uint16                     `json:"maximumParties,omitempty"`
	Navigation        TournamentNavigationTarget `json:"navigation"`
	ProjectionVersion string                     `json:"projectionVersion"`
}

type PublicTournamentEditionProjection struct {
	Slug              string                     `json:"slug"`
	Name              string                     `json:"name"`
	State             string                     `json:"state"`
	ConfirmedParties  uint16                     `json:"confirmedParties,omitempty"`
	MaximumParties    uint16                     `json:"maximumParties,omitempty"`
	Bracket           TournamentNavigationTarget `json:"bracket,omitempty"`
	History           TournamentNavigationTarget `json:"history,omitempty"`
	ProjectionVersion string                     `json:"projectionVersion"`
}

// PersonalQualifierProjection is display-safe. The authoritative qualification
// relation and result provenance remain inside Competios; clients receive only
// the current state and canonical navigation URL.
type PersonalQualifierProjection struct {
	State        string                     `json:"state"`
	Registration TournamentNavigationTarget `json:"registration,omitempty"`
}

type PersonalEventRosterMemberProjection struct {
	Role        EventRosterRole `json:"role"`
	DisplayName string          `json:"displayName,omitempty"`
}

// PersonalEventRosterProjection is the immutable, role-bearing Competios
// snapshot. It has no bot-owned Team model or mutable membership inference.
type PersonalEntryRosterProjection struct {
	Members  []PersonalEventRosterMemberProjection `json:"members,omitempty"`
	LockedAt time.Time                             `json:"lockedAt,omitempty"`
}

type PersonalPairingProjection struct {
	State  string                     `json:"state"`
	Battle TournamentNavigationTarget `json:"battle,omitempty"`
}

type PersonalCompletionProjection struct {
	EditionSlug string                     `json:"editionSlug,omitempty"`
	State       string                     `json:"state"`
	History     TournamentNavigationTarget `json:"history,omitempty"`
}

// PersonalTournamentStatusProjection contains only data the current bound
// subject may see. It intentionally has no contact, reviewer notes, decline
// reason, internal flag, raw authority evidence, or raw domain ID.
type PersonalTournamentStatusProjection struct {
	ApplicationState  string                         `json:"applicationState,omitempty"`
	CommitmentState   string                         `json:"commitmentState,omitempty"`
	Qualifier         *PersonalQualifierProjection   `json:"qualifier,omitempty"`
	EntryRoster       *PersonalEntryRosterProjection `json:"entryRoster,omitempty"`
	Pairing           *PersonalPairingProjection     `json:"pairing,omitempty"`
	Completion        *PersonalCompletionProjection  `json:"completion,omitempty"`
	NextAction        string                         `json:"nextAction,omitempty"`
	ProjectionVersion string                         `json:"projectionVersion"`
}

// TournamentNotificationKind is a closed, Competios-issued delivery event.
// Notification content is rendered by the receiving surface; arbitrary body
// text is not part of this contract.
type TournamentNotificationKind string

const (
	TournamentNotificationInvitation TournamentNotificationKind = "invitation"
	TournamentNotificationQualifier  TournamentNotificationKind = "qualifier"
	TournamentNotificationPairing    TournamentNotificationKind = "pairing"
	TournamentNotificationCompleted  TournamentNotificationKind = "completed"
)

type NotificationVisibility string

const (
	NotificationVisibilityPersonal NotificationVisibility = "personal"
	NotificationVisibilityPublic   NotificationVisibility = "public"
)

// TournamentNotificationProjection is a minimal, recipient-scoped delivery
// instruction. It contains no free text, internal ID, or private material.
type TournamentNotificationProjection struct {
	Kind              TournamentNotificationKind `json:"kind"`
	Purpose           InteractionPurpose         `json:"purpose"`
	Visibility        NotificationVisibility     `json:"visibility"`
	Action            TournamentNavigationTarget `json:"action,omitempty"`
	ProjectionVersion string                     `json:"projectionVersion"`
}

type TournamentSeriesDiscoveryRequest struct {
	BotProfileID BotProfileID `json:"botProfileID"`
}

// TournamentEditionRequest contains public slugs only. Lookup still performs
// trusted visibility and association checks; a slug never grants access.
type TournamentEditionRequest struct {
	BotProfileID BotProfileID `json:"botProfileID"`
	SeriesSlug   string       `json:"seriesSlug"`
	EditionSlug  string       `json:"editionSlug"`
}

// IssueInteractionRequest is authenticated host input, never a Telegram
// callback payload. Binding is an opaque Competios-owned reference held only
// by the implementation; external clients receive Handle only.
type IssueInteractionRequest struct {
	BotProfileID    BotProfileID        `json:"botProfileID"`
	Subject         InteractionSubject  `json:"-"`
	Purpose         InteractionPurpose  `json:"purpose"`
	Surface         InteractionSurface  `json:"surface"`
	AllowedActions  []InteractionAction `json:"allowedActions,omitempty"`
	OperationRef    OperationRef        `json:"-"`
	BindingRef      BindingRef          `json:"-"`
	ExpectedVersion string              `json:"-"`
	ExpiresAt       time.Time           `json:"-"`
}

type InteractionCallback struct {
	// Data is the compact, versioned transport field, e.g. i1.<handle>.<a>.<v>.
	// It must stay within Telegram's 64-byte callback-data limit.
	Data string `json:"data"`
}

type ResolveInteractionRequest struct {
	BotProfileID BotProfileID       `json:"botProfileID"`
	Subject      InteractionSubject `json:"-"`
	Handle       InteractionHandle  `json:"handle"`
	Surface      InteractionSurface `json:"surface"`
}

type ExecuteInteractionRequest struct {
	BotProfileID BotProfileID        `json:"botProfileID"`
	Subject      InteractionSubject  `json:"-"`
	Surface      InteractionSurface  `json:"surface"`
	Callback     InteractionCallback `json:"callback"`
}

type InteractionOutcome struct {
	Status            InteractionOutcomeStatus            `json:"status"`
	Handle            InteractionHandle                   `json:"handle,omitempty"`
	Action            InteractionAction                   `json:"action,omitempty"`
	Projection        *PersonalTournamentStatusProjection `json:"projection,omitempty"`
	Public            *PublicTournamentSeriesProjection   `json:"public,omitempty"`
	Notification      *TournamentNotificationProjection   `json:"notification,omitempty"`
	ProjectionVersion string                              `json:"projectionVersion,omitempty"`
	RetryAfter        time.Time                           `json:"retryAfter,omitempty"`
}

// TournamentSeriesInteractionApplication is the stable boundary for web and
// bot clients. Implementations re-check current authority before exposing a
// replay or stateful outcome; callback data is never authority evidence.
type TournamentSeriesInteractionApplication interface {
	DiscoverTournamentSeries(context.Context, TournamentSeriesDiscoveryRequest) ([]PublicTournamentSeriesProjection, error)
	GetPublicTournamentEdition(context.Context, TournamentEditionRequest) (PublicTournamentEditionProjection, error)
	IssueInteraction(context.Context, IssueInteractionRequest) (InteractionHandle, error)
	EncodeCallback(InteractionHandle, InteractionAction, string) (InteractionCallback, error)
	ResolveInteraction(context.Context, ResolveInteractionRequest) (InteractionOutcome, error)
	ExecuteInteraction(context.Context, ExecuteInteractionRequest) (InteractionOutcome, error)
	RefreshInteraction(context.Context, ResolveInteractionRequest) (InteractionOutcome, error)
	RevokeInteraction(context.Context, ResolveInteractionRequest) error
}
