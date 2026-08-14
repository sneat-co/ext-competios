package contract4competios

import (
	"errors"
	"testing"
	"time"
)

func validParticipationPrice() TournamentParticipationPrice {
	return TournamentParticipationPrice{OfferID: "offer-1", OfferVersion: 1, AmountMinor: 0, Currency: "EUR", ChargeBasis: ChargeBasisPerEntry, OfferChecksum: ParticipationOfferChecksum(testPayloadDigest("1"))}
}

func TestTournamentParticipationPriceIsExplicitAndClosed(t *testing.T) {
	price := validParticipationPrice()
	if !price.IsFree() || ValidateTournamentParticipationPrice(price) != nil {
		t.Fatalf("explicit zero free price rejected: %+v", price)
	}
	for name, value := range map[string]TournamentParticipationPrice{
		"blank offer":      {OfferVersion: 1, Currency: "EUR", ChargeBasis: ChargeBasisPerEntry},
		"zero version":     {OfferID: "offer", Currency: "EUR", ChargeBasis: ChargeBasisPerEntry},
		"negative amount":  {OfferID: "offer", OfferVersion: 1, AmountMinor: -1, Currency: "EUR", ChargeBasis: ChargeBasisPerEntry},
		"non ISO currency": {OfferID: "offer", OfferVersion: 1, Currency: "eur", ChargeBasis: ChargeBasisPerEntry},
		"per person basis": {OfferID: "offer", OfferVersion: 1, Currency: "EUR", ChargeBasis: "per_person"},
		"missing checksum": {OfferID: "offer", OfferVersion: 1, Currency: "EUR", ChargeBasis: ChargeBasisPerEntry},
	} {
		if err := ValidateTournamentParticipationPrice(value); !errors.Is(err, ErrInvalidEventRegistration) {
			t.Errorf("%s error = %v, want ErrInvalidEventRegistration", name, err)
		}
	}
}

func TestBookOnlineValidationBindsAdmissionPayerCapacityAndOffer(t *testing.T) {
	identity := TournamentIdentity{EventID: "event-1", TournamentID: "tournament-1", CompetitionID: "competition-1"}
	request := BookOnlineEntryValidationRequest{RequestID: "request-1", Tournament: identity, ProposedEntryID: "entry-1", ParticipantKind: ParticipantTeam, ParticipantID: "team-1", ApplicantAccountID: "captain-1"}
	if err := ValidateBookOnlineEntryValidationRequest(request); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	validation := BookOnlineEntryValidation{
		RequestID: "request-1", Tournament: identity, TournamentVersion: 3, TargetVersion: 2,
		ProposedEntryID: "entry-1", ParticipantKind: ParticipantTeam, ParticipantID: "team-1",
		EnrolmentPolicy: EnrolmentApprovalRequired, Capacity: 16, FulfilmentMode: RegistrationFulfilmentBookOnline,
		Price: validParticipationPrice(), Payer: EntryPayerAuthority{AccountID: "captain-1", Role: EntryPayerCaptain},
	}
	if err := ValidateBookOnlineEntryValidation(validation); err != nil {
		t.Fatalf("valid validation: %v", err)
	}
	validation.Payer.Role = EntryPayerApplicant
	if err := ValidateBookOnlineEntryValidation(validation); !errors.Is(err, ErrInvalidEventRegistration) {
		t.Fatalf("team applicant payer error = %v", err)
	}
}

func TestBookOnlineConfirmationBindsRevisionAndFreeOrSettlementEvidence(t *testing.T) {
	confirmed := BookOnlineEntryConfirmation{
		AttemptID: "attempt-1", BookingReference: "bookius:booking-1", TargetVersion: 2, BookingRevision: 4,
		Tournament:      TournamentIdentity{EventID: "event-1", TournamentID: "tournament-1", CompetitionID: "competition-1"},
		ProposedEntryID: "entry-1", ParticipantKind: ParticipantTeam, Payer: EntryPayerAuthority{AccountID: "captain-1", Role: EntryPayerCaptain},
		Price: validParticipationPrice(), Evidence: ParticipationConfirmationEvidence{Kind: ParticipationConfirmationFree}, ConfirmedAt: time.Now().UTC(),
	}
	if err := ValidateBookOnlineEntryConfirmation(confirmed); err != nil {
		t.Fatalf("valid free confirmation: %v", err)
	}
	confirmed.Price.AmountMinor = 2500
	confirmed.Evidence = ParticipationConfirmationEvidence{Kind: ParticipationConfirmationSettled, Reference: "stripe:checkout-1"}
	if err := ValidateBookOnlineEntryConfirmation(confirmed); err != nil {
		t.Fatalf("valid settled confirmation: %v", err)
	}
	confirmed.Evidence.Reference = ""
	if err := ValidateBookOnlineEntryConfirmation(confirmed); !errors.Is(err, ErrInvalidEventRegistration) {
		t.Fatalf("missing settlement error = %v", err)
	}
}

func TestBookOnlineCancellationUsesCurrentLockAndTrustedRefundDecision(t *testing.T) {
	identity := TournamentIdentity{EventID: "event-1", TournamentID: "tournament-1", CompetitionID: "competition-1"}
	request := BookOnlineEntryCancellationRequest{
		RequestID: "cancel-1", BookingReference: "bookius:booking-1", TargetVersion: 2, BookingRevision: 4,
		Tournament: identity, EntryID: "entry-1", Origin: BookOnlineCancellationParticipant,
		ActorAccountID: "player-1", AuthorityEvidence: "session-proof-1", Reason: "cannot attend",
	}
	if err := ValidateBookOnlineEntryCancellationRequest(request); err != nil {
		t.Fatalf("valid cancellation request: %v", err)
	}
	validation := BookOnlineEntryCancellationValidation{
		RequestID: request.RequestID, BookingReference: request.BookingReference, TargetVersion: request.TargetVersion,
		BookingRevision: request.BookingRevision, Tournament: identity, EntryID: request.EntryID, Origin: request.Origin,
		Authorized: true, RefundAuthorized: true, CurrentTournamentVersion: 9,
		AuthorityEvidence: "competios-proof-1", ValidatedAt: time.Now().UTC(),
	}
	if err := ValidateBookOnlineEntryCancellationValidation(validation); err != nil {
		t.Fatalf("unlocked participant refund: %v", err)
	}
	if err := ValidateBookOnlineEntryCancellationBinding(request, validation); err != nil {
		t.Fatalf("exact cancellation binding: %v", err)
	}
	mismatched := validation
	mismatched.BookingRevision++
	if err := ValidateBookOnlineEntryCancellationBinding(request, mismatched); !errors.Is(err, ErrInvalidEventRegistration) {
		t.Fatalf("mismatched booking revision = %v", err)
	}

	validation.RegistrationLocked = true
	validation.RefundAuthorized = false
	if err := ValidateBookOnlineEntryCancellationValidation(validation); err != nil {
		t.Fatalf("locked participant non-refund cancellation: %v", err)
	}
	validation.RefundAuthorized = true
	if err := ValidateBookOnlineEntryCancellationValidation(validation); !errors.Is(err, ErrInvalidEventRegistration) {
		t.Fatalf("locked participant refund without organiser = %v", err)
	}
	validation.AuthoriserAccountID = "organiser-1"
	if err := ValidateBookOnlineEntryCancellationValidation(validation); err != nil {
		t.Fatalf("locked participant override: %v", err)
	}

	validation.Origin = BookOnlineCancellationOrganiser
	validation.AuthoriserAccountID = ""
	if err := ValidateBookOnlineEntryCancellationValidation(validation); err != nil {
		t.Fatalf("organiser cancellation: %v", err)
	}
	validation.RefundAuthorized = false
	if err := ValidateBookOnlineEntryCancellationValidation(validation); !errors.Is(err, ErrInvalidEventRegistration) {
		t.Fatalf("organiser cancellation without refund = %v", err)
	}
}

func TestBookOnlineCancellationRejectsUntrustedOrStaleEvidence(t *testing.T) {
	request := BookOnlineEntryCancellationRequest{
		RequestID: "cancel-1", BookingReference: "bookius:booking-1", TargetVersion: 2, BookingRevision: 4,
		Tournament: TournamentIdentity{EventID: "event-1", TournamentID: "tournament-1", CompetitionID: "competition-1"},
		EntryID:    "entry-1", Origin: BookOnlineCancellationParticipant, ActorAccountID: "player-1",
		AuthorityEvidence: "session-proof-1", Reason: "cannot attend",
	}
	for name, mutate := range map[string]func(*BookOnlineEntryCancellationRequest){
		"missing booking revision": func(value *BookOnlineEntryCancellationRequest) { value.BookingRevision = 0 },
		"missing authority":        func(value *BookOnlineEntryCancellationRequest) { value.AuthorityEvidence = "" },
		"unknown origin":           func(value *BookOnlineEntryCancellationRequest) { value.Origin = "browser-choice" },
	} {
		value := request
		mutate(&value)
		if err := ValidateBookOnlineEntryCancellationRequest(value); !errors.Is(err, ErrInvalidEventRegistration) {
			t.Errorf("%s error = %v, want ErrInvalidEventRegistration", name, err)
		}
	}
}
