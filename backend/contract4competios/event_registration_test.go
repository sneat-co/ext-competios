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
