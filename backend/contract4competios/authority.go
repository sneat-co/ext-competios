package contract4competios

import "errors"

// The IDs below are intentionally opaque to the application boundary. In
// particular, ValidatedTelegramUserRef is not a Telegram chat ID and carries
// no authorisation semantics by itself.
type ActorID string
type SpaceID string
type UserID string
type BotProfileID string
type ValidatedTelegramUserRef string
type ProvisionalContextRef string
type DraftID string
type CommandID string

// AggregateVersion is the opaque mutation precondition. It advances for
// every business mutation, including deterministic expiry. A receipt-only
// replay advances neither version.
type AggregateVersion string

// ContentRevision is the opaque rendering revision. It advances when
// projected content, state or available actions change, but not for a replay
// that only reads a durable command receipt.
type ContentRevision string

type GameID string
type RulesetVersion string
type FormatTemplateID string
type StageID string
type DestinationID string
type InvitationID string
type ParticipantID string

var (
	ErrNotFound          = errors.New("competios contract: resource not found")
	ErrStaleVersion      = errors.New("competios contract: stale aggregate version")
	ErrDraftExpired      = errors.New("competios contract: draft has expired")
	ErrInvalidTransition = errors.New("competios contract: invalid lifecycle transition")
)

// StaleVersionError is returned only after the caller has passed current
// authority for the target aggregate. CurrentAggregateVersion is an opaque
// retry precondition, not a sequence number clients may parse or derive.
// errors.Is(err, ErrStaleVersion) remains true for callers that do not need
// the current version.
type StaleVersionError struct {
	CurrentAggregateVersion AggregateVersion
}

func (e *StaleVersionError) Error() string { return ErrStaleVersion.Error() }

func (e *StaleVersionError) Unwrap() error { return ErrStaleVersion }

type PrincipalKind string

const (
	PrincipalAuthenticated PrincipalKind = "authenticated"
	PrincipalProvisional   PrincipalKind = "provisional-telegram"
	PrincipalAnonymous     PrincipalKind = "anonymous"
)

// ProvisionalTelegramPrincipal is produced by a host after validating the
// bot profile and Telegram user. It is not an application account and does
// not contain a Telegram chat identifier.
type ProvisionalTelegramPrincipal struct {
	BotProfileID    BotProfileID             `json:"botProfileID"`
	TelegramUserRef ValidatedTelegramUserRef `json:"telegramUserRef"`
}

// PrincipalRef deliberately has no role, ownership, authority evidence or
// chat fields. The application resolves those facts from configured ports.
type PrincipalRef struct {
	Kind        PrincipalKind                 `json:"kind"`
	ActorID     ActorID                       `json:"actorID,omitempty"`
	Provisional *ProvisionalTelegramPrincipal `json:"provisional,omitempty"`
}

func NewAuthenticatedPrincipal(actorID ActorID) PrincipalRef {
	return PrincipalRef{Kind: PrincipalAuthenticated, ActorID: actorID}
}

func NewProvisionalTelegramPrincipal(
	botProfileID BotProfileID,
	telegramUserRef ValidatedTelegramUserRef,
) PrincipalRef {
	return PrincipalRef{
		Kind: PrincipalProvisional,
		Provisional: &ProvisionalTelegramPrincipal{
			BotProfileID:    botProfileID,
			TelegramUserRef: telegramUserRef,
		},
	}
}

func NewAnonymousPrincipal() PrincipalRef {
	return PrincipalRef{Kind: PrincipalAnonymous}
}

// CallerContext is the released M1/M2 trusted host context. Keep this exact
// two-field layout source-compatible. M3 provisional opaque context is resolved
// independently from trusted request context by an additive authority port; it
// is never overloaded into SpaceID.
type CallerContext struct {
	Principal PrincipalRef `json:"principal"`
	SpaceID   SpaceID      `json:"spaceID,omitempty"`
}

type CommandMetadata struct {
	CommandID                CommandID        `json:"commandID"`
	ExpectedAggregateVersion AggregateVersion `json:"expectedAggregateVersion,omitempty"`
}
