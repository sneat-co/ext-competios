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

var ErrInvalidEventRegistration = errors.New("competios contract: invalid event registration value")

type ChargeBasis string

const ChargeBasisPerEntry ChargeBasis = "per_entry"

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

// PaidParticipationConfirmation is emitted only after Bookius reports a
// settled payment for the exact held offer. Consumers use AttemptID and the
// Bookius booking reference as idempotency keys when confirming the Entry.
type PaidParticipationConfirmation struct {
	AttemptID        ParticipationPaymentAttemptID `json:"attemptID"`
	BookingReference BookingReference              `json:"bookingReference"`
	Tournament       TournamentIdentity            `json:"tournament"`
	Price            TournamentParticipationPrice  `json:"price"`
	SettledAt        time.Time                     `json:"settledAt"`
}

func ValidatePaidParticipationConfirmation(value PaidParticipationConfirmation) error {
	if value.AttemptID == "" || value.BookingReference == "" || value.SettledAt.IsZero() || ValidateTournamentIdentity(value.Tournament) != nil || ValidateTournamentParticipationPrice(value.Price) != nil {
		return ErrInvalidEventRegistration
	}
	return nil
}

// PaidEntryConfirmationSink is the narrow Competios ingress that Bookius (or
// its host adapter) calls after settlement. Implementations must atomically
// deduplicate on AttemptID and BookingReference before confirming an Entry.
type PaidEntryConfirmationSink interface {
	ConfirmPaidEntry(context.Context, PaidParticipationConfirmation) (EntryID, error)
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
