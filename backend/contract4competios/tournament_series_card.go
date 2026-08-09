package contract4competios

import (
	"context"
	"time"
)

// TournamentLaunchHandle is a Competios-issued, expiring capability for one
// tournament-series card. It is opaque to every client. In particular, it is
// not a Series, edition, account, Telegram, BindingRef, or OperationRef ID.
type TournamentLaunchHandle string

// TournamentSeriesCardPresentation is display-safe public information for a
// single series edition. The format, timeline and deadline are published
// product copy/facts selected by the composed Competios adapter; they must not
// contain private application, reviewer, roster, authority, or game-session
// data.
type TournamentSeriesCardPresentation struct {
	Series                    PublicTournamentSeriesProjection  `json:"series"`
	Edition                   PublicTournamentEditionProjection `json:"edition"`
	ParticipationRequirements TournamentNavigationTarget        `json:"participationRequirements"`
	FormatSummary             string                            `json:"formatSummary"`
	PublishedTimeline         string                            `json:"publishedTimeline"`
	// PublishedDeadline is absent until the organiser has announced it. A nil
	// value is truthful public information, not a placeholder date.
	PublishedDeadline *time.Time `json:"publishedDeadline,omitempty"`
	ProjectionVersion string     `json:"projectionVersion"`
}

type TournamentSeriesCardActionKind string

const (
	TournamentSeriesCardActionRefresh                TournamentSeriesCardActionKind = "refresh"
	TournamentSeriesCardActionOpenApplication        TournamentSeriesCardActionKind = "open-application"
	TournamentSeriesCardActionOpenParticipationTerms TournamentSeriesCardActionKind = "open-participation-requirements"
	TournamentSeriesCardActionOpenQualifier          TournamentSeriesCardActionKind = "open-qualifier"
	TournamentSeriesCardActionOpenEntry              TournamentSeriesCardActionKind = "open-entry"
	TournamentSeriesCardActionOpenPairing            TournamentSeriesCardActionKind = "open-pairing"
	TournamentSeriesCardActionOpenBattle             TournamentSeriesCardActionKind = "open-battle"
	TournamentSeriesCardActionOpenBracket            TournamentSeriesCardActionKind = "open-bracket"
	TournamentSeriesCardActionOpenHistory            TournamentSeriesCardActionKind = "open-history"
)

// TournamentWebHandoff has no URL. The receiving host turns Target into an
// origin-specific URL only after it has accepted the subject-bound launch
// handle. OpaqueRef, including a nav1 battle reference, remains data and must
// never be interpreted as a URL by a bot or client.
type TournamentWebHandoff struct {
	Target       TournamentNavigationTarget `json:"target"`
	LaunchHandle TournamentLaunchHandle     `json:"launchHandle"`
	ExpiresAt    time.Time                  `json:"expiresAt"`
}

// TournamentSeriesCardAction is either a compact callback or a typed web
// handoff. It deliberately provides neither arbitrary links nor raw domain
// identifiers.
type TournamentSeriesCardAction struct {
	Kind     TournamentSeriesCardActionKind `json:"kind"`
	Callback *InteractionCallback           `json:"callback,omitempty"`
	Handoff  *TournamentWebHandoff          `json:"handoff,omitempty"`
}

// TournamentSeriesCard is the high-level bot/mobile presentation boundary.
// Personal is already redacted to the requesting, host-authenticated subject.
type TournamentSeriesCard struct {
	LaunchHandle      TournamentLaunchHandle              `json:"launchHandle"`
	ExpiresAt         time.Time                           `json:"expiresAt"`
	Presentation      TournamentSeriesCardPresentation    `json:"presentation"`
	Personal          *PersonalTournamentStatusProjection `json:"personal,omitempty"`
	Actions           []TournamentSeriesCardAction        `json:"actions"`
	ProjectionVersion string                              `json:"projectionVersion"`
}

// OpenTournamentSeriesCardRequest accepts only host-authenticated identity,
// public series selection or a previously issued opaque launch handle, and a
// surface. Exactly one of SeriesSlug and LaunchHandle must be present.
type OpenTournamentSeriesCardRequest struct {
	BotProfileID BotProfileID           `json:"botProfileID"`
	Subject      InteractionSubject     `json:"-"`
	SeriesSlug   string                 `json:"seriesSlug,omitempty"`
	LaunchHandle TournamentLaunchHandle `json:"launchHandle,omitempty"`
	Surface      InteractionSurface     `json:"surface"`
}

// RefreshTournamentSeriesCardRequest intentionally exposes no series or
// durable identifiers. The launch handle remains bound to the bot, subject,
// surface and expiry by Competios.
type RefreshTournamentSeriesCardRequest struct {
	BotProfileID BotProfileID           `json:"botProfileID"`
	Subject      InteractionSubject     `json:"-"`
	LaunchHandle TournamentLaunchHandle `json:"launchHandle"`
	Surface      InteractionSurface     `json:"surface"`
}

// TournamentSeriesCardApplication is the only interface a bot client needs
// for a tournament-series card. Lower-level IssueInteraction intentionally
// remains an internal composition detail because it requires BindingRef and
// OperationRef values that callers must never construct or inspect.
type TournamentSeriesCardApplication interface {
	OpenTournamentSeriesCard(context.Context, OpenTournamentSeriesCardRequest) (TournamentSeriesCard, error)
	RefreshTournamentSeriesCard(context.Context, RefreshTournamentSeriesCardRequest) (TournamentSeriesCard, error)
}
