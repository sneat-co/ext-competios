package contract4competios

import (
	"context"
	"errors"
	"strings"
	"time"
)

// EventID is the public Competios Event identity. An Event is the umbrella for
// one or more Tournaments; it must never be confused with ExecutionEventID.
type EventID string

// TournamentID is an Event-scoped competition surface. CompetitionID remains
// the execution/engine aggregate identity during the migration.
type TournamentID string
type ParticipationPriceOfferID string
type ParticipationOfferChecksum string
type ParticipationPaymentAttemptID string
type BookingReference string
type AttendanceEventReference string
type AttendanceInvitationReference string
type BookOnlineTargetVersion uint64
type BookOnlineBookingRevision uint64
type AccountID string
type InviteProof string

var ErrInvalidEventRegistration = errors.New("competios contract: invalid event registration value")

type ChargeBasis string

const ChargeBasisPerEntry ChargeBasis = "per_entry"

type RegistrationFulfilmentMode string

const (
	RegistrationFulfilmentDirect     RegistrationFulfilmentMode = "direct"
	RegistrationFulfilmentBookOnline RegistrationFulfilmentMode = "book-online"
)

type EntryPayerRole string

const (
	EntryPayerApplicant EntryPayerRole = "applicant"
	EntryPayerGuardian  EntryPayerRole = "guardian"
	EntryPayerCaptain   EntryPayerRole = "captain"
)

type EntryPayerAuthority struct {
	AccountID AccountID      `json:"accountID"`
	Role      EntryPayerRole `json:"role"`
}

// TournamentParticipationPrice is an immutable, server-authored offer
// snapshot. Zero is an explicit free offer. A pair or team still pays once
// because the only supported basis is per_entry.
//
// Once registrations have opened, callers create a new OfferID/version rather
// than modifying an accepted offer. Consumers persist this exact snapshot with
// each registration/payment attempt.
type TournamentParticipationPrice struct {
	OfferID       ParticipationPriceOfferID  `json:"offerID"`
	OfferVersion  uint32                     `json:"offerVersion"`
	AmountMinor   int64                      `json:"amountMinor"`
	Currency      string                     `json:"currency"`
	ChargeBasis   ChargeBasis                `json:"chargeBasis"`
	OfferChecksum ParticipationOfferChecksum `json:"offerChecksum"`
}

func (value TournamentParticipationPrice) IsFree() bool { return value.AmountMinor == 0 }

func ValidateTournamentParticipationPrice(value TournamentParticipationPrice) error {
	if value.OfferID == "" || value.OfferVersion == 0 || value.AmountMinor < 0 || !validISOCurrency(value.Currency) || value.ChargeBasis != ChargeBasisPerEntry || !validSHA256Digest(string(value.OfferChecksum)) {
		return ErrInvalidEventRegistration
	}
	return nil
}

func validISOCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

type TournamentIdentity struct {
	EventID       EventID       `json:"eventID"`
	TournamentID  TournamentID  `json:"tournamentID"`
	CompetitionID CompetitionID `json:"competitionID"`
}

func ValidateTournamentIdentity(value TournamentIdentity) error {
	if value.EventID == "" || value.TournamentID == "" || value.CompetitionID == "" {
		return ErrInvalidEventRegistration
	}
	return nil
}

// BookOnlineEntryValidationRequest is the untrusted proposal Bookius sends to
// the Competios server boundary before it allocates capacity or opens a
// checkout. Competios resolves every trusted field in the returned validation.
type BookOnlineEntryValidationRequest struct {
	RequestID          CommandID          `json:"requestID"`
	Tournament         TournamentIdentity `json:"tournament"`
	ProposedEntryID    EntryID            `json:"proposedEntryID"`
	ParticipantKind    ParticipantKind    `json:"participantKind"`
	ParticipantID      ParticipantID      `json:"participantID"`
	ApplicantAccountID AccountID          `json:"applicantAccountID"`
	InviteProof        InviteProof        `json:"inviteProof,omitempty"`
}

// BookOnlineEntryValidation is server-authored eligibility, admission, payer,
// capacity-mode and offer evidence. Bookius persists this exact snapshot; a
// browser cannot construct or override it.
type BookOnlineEntryValidation struct {
	RequestID          CommandID                    `json:"requestID"`
	Tournament         TournamentIdentity           `json:"tournament"`
	TournamentVersion  uint64                       `json:"tournamentVersion"`
	TargetVersion      BookOnlineTargetVersion      `json:"targetVersion"`
	ProposedEntryID    EntryID                      `json:"proposedEntryID"`
	ParticipantKind    ParticipantKind              `json:"participantKind"`
	ParticipantID      ParticipantID                `json:"participantID"`
	EnrolmentPolicy    EnrolmentPolicy              `json:"enrolmentPolicy"`
	Capacity           uint16                       `json:"capacity"`
	FulfilmentMode     RegistrationFulfilmentMode   `json:"fulfilmentMode"`
	Price              TournamentParticipationPrice `json:"price"`
	Payer              EntryPayerAuthority          `json:"payer"`
	RegistrationLockAt *time.Time                   `json:"registrationLockAt,omitempty"`
}

func ValidateBookOnlineEntryValidationRequest(value BookOnlineEntryValidationRequest) error {
	if value.RequestID == "" || ValidateTournamentIdentity(value.Tournament) != nil || value.ProposedEntryID == "" || !validParticipantKind(value.ParticipantKind) || value.ParticipantID == "" || value.ApplicantAccountID == "" {
		return ErrInvalidEventRegistration
	}
	return nil
}

func ValidateBookOnlineEntryValidation(value BookOnlineEntryValidation) error {
	if value.RequestID == "" || ValidateTournamentIdentity(value.Tournament) != nil || value.TournamentVersion == 0 || value.TargetVersion == 0 || value.ProposedEntryID == "" || !validParticipantKind(value.ParticipantKind) || value.ParticipantID == "" || !validEnrolmentPolicy(value.EnrolmentPolicy) || value.Capacity == 0 || value.FulfilmentMode != RegistrationFulfilmentBookOnline || ValidateTournamentParticipationPrice(value.Price) != nil || value.Payer.AccountID == "" || !validPayer(value.ParticipantKind, value.Payer.Role) {
		return ErrInvalidEventRegistration
	}
	if value.RegistrationLockAt != nil && value.RegistrationLockAt.IsZero() {
		return ErrInvalidEventRegistration
	}
	return nil
}

// BookOnlineEntryValidator is implemented by Competios and called server to
// server by Bookius before any hold or payment-provider interaction.
type BookOnlineEntryValidator interface {
	ValidateBookOnlineEntry(context.Context, BookOnlineEntryValidationRequest) (BookOnlineEntryValidation, error)
}

type ParticipationPaymentState string

const (
	ParticipationPaymentAwaitingCheckout ParticipationPaymentState = "awaiting-checkout"
	ParticipationPaymentHeld             ParticipationPaymentState = "held"
	ParticipationPaymentPaid             ParticipationPaymentState = "paid"
	ParticipationPaymentFailed           ParticipationPaymentState = "failed"
	ParticipationPaymentExpired          ParticipationPaymentState = "expired"
	ParticipationPaymentRefunded         ParticipationPaymentState = "refunded"
)

// PaidParticipationAttempt is the safe, idempotency-bearing payment
// projection. Checkout credentials and payment-provider tokens are deliberately
// excluded; Bookius owns both and Competios only stores its opaque booking ref.
type PaidParticipationAttempt struct {
	AttemptID             ParticipationPaymentAttemptID `json:"attemptID"`
	RegistrationCommandID CommandID                     `json:"registrationCommandID"`
	Tournament            TournamentIdentity            `json:"tournament"`
	Price                 TournamentParticipationPrice  `json:"price"`
	BookingReference      BookingReference              `json:"bookingReference"`
	State                 ParticipationPaymentState     `json:"state"`
	ExpiresAt             *time.Time                    `json:"expiresAt,omitempty"`
}

func ValidatePaidParticipationAttempt(value PaidParticipationAttempt) error {
	if value.AttemptID == "" || value.RegistrationCommandID == "" || value.BookingReference == "" || ValidateTournamentIdentity(value.Tournament) != nil || ValidateTournamentParticipationPrice(value.Price) != nil {
		return ErrInvalidEventRegistration
	}
	switch value.State {
	case ParticipationPaymentAwaitingCheckout, ParticipationPaymentHeld, ParticipationPaymentPaid, ParticipationPaymentFailed, ParticipationPaymentExpired, ParticipationPaymentRefunded:
	default:
		return ErrInvalidEventRegistration
	}
	if (value.State == ParticipationPaymentAwaitingCheckout || value.State == ParticipationPaymentHeld) && (value.ExpiresAt == nil || value.ExpiresAt.IsZero()) {
		return ErrInvalidEventRegistration
	}
	return nil
}

type ParticipationConfirmationKind string

const (
	ParticipationConfirmationFree    ParticipationConfirmationKind = "free"
	ParticipationConfirmationSettled ParticipationConfirmationKind = "verified-settlement"
)

type ParticipationConfirmationEvidence struct {
	Kind      ParticipationConfirmationKind `json:"kind"`
	Reference string                        `json:"reference,omitempty"`
}

// BookOnlineEntryConfirmation is emitted after Bookius confirms either an
// explicit free booking or an exact verified settlement for an active hold.
// Consumers deduplicate on AttemptID and BookingReference and reject stale
// target/booking revisions or conflicting evidence.
type BookOnlineEntryConfirmation struct {
	AttemptID        ParticipationPaymentAttemptID     `json:"attemptID"`
	BookingReference BookingReference                  `json:"bookingReference"`
	TargetVersion    BookOnlineTargetVersion           `json:"targetVersion"`
	BookingRevision  BookOnlineBookingRevision         `json:"bookingRevision"`
	Tournament       TournamentIdentity                `json:"tournament"`
	ProposedEntryID  EntryID                           `json:"proposedEntryID"`
	ParticipantKind  ParticipantKind                   `json:"participantKind"`
	Payer            EntryPayerAuthority               `json:"payer"`
	Price            TournamentParticipationPrice      `json:"price"`
	Evidence         ParticipationConfirmationEvidence `json:"evidence"`
	ConfirmedAt      time.Time                         `json:"confirmedAt"`
}

func ValidateBookOnlineEntryConfirmation(value BookOnlineEntryConfirmation) error {
	if value.AttemptID == "" || value.BookingReference == "" || value.TargetVersion == 0 || value.BookingRevision == 0 || value.ProposedEntryID == "" || !validParticipantKind(value.ParticipantKind) || value.Payer.AccountID == "" || !validPayer(value.ParticipantKind, value.Payer.Role) || value.ConfirmedAt.IsZero() || ValidateTournamentIdentity(value.Tournament) != nil || ValidateTournamentParticipationPrice(value.Price) != nil {
		return ErrInvalidEventRegistration
	}
	if value.Price.IsFree() {
		if value.Evidence.Kind != ParticipationConfirmationFree || strings.TrimSpace(value.Evidence.Reference) != "" {
			return ErrInvalidEventRegistration
		}
	} else if value.Evidence.Kind != ParticipationConfirmationSettled || strings.TrimSpace(value.Evidence.Reference) == "" {
		return ErrInvalidEventRegistration
	}
	return nil
}

// BookOnlineEntryConfirmationSink is the narrow Competios ingress Bookius (or
// its host adapter) calls after confirmation.
type BookOnlineEntryConfirmationSink interface {
	ConfirmBookOnlineEntry(context.Context, BookOnlineEntryConfirmation) (EntryID, error)
}

// AttendanceProjection is the only Eventius information Competios may persist
// or return. RSVP token/link authority must never enter this contract.
type AttendanceProjection struct {
	EventReference      AttendanceEventReference      `json:"eventReference"`
	InvitationReference AttendanceInvitationReference `json:"invitationReference,omitempty"`
	Status              string                        `json:"status"`
}

func ValidateAttendanceProjection(value AttendanceProjection) error {
	if value.EventReference == "" || strings.TrimSpace(value.Status) == "" {
		return ErrInvalidEventRegistration
	}
	return nil
}

func validParticipantKind(value ParticipantKind) bool {
	return value == ParticipantIndividual || value == ParticipantPair || value == ParticipantTeam
}

func validEnrolmentPolicy(value EnrolmentPolicy) bool {
	return value == EnrolmentOpen || value == EnrolmentApprovalRequired || value == EnrolmentInviteOnly
}

func validPayer(kind ParticipantKind, role EntryPayerRole) bool {
	switch kind {
	case ParticipantIndividual:
		return role == EntryPayerApplicant || role == EntryPayerGuardian
	case ParticipantPair, ParticipantTeam:
		return role == EntryPayerCaptain
	default:
		return false
	}
}
