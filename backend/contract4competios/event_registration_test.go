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

func TestPaidConfirmationBindsServerAuthoredPriceAndIdentity(t *testing.T) {
	confirmed := PaidParticipationConfirmation{
		AttemptID: "attempt-1", BookingReference: "bookius:booking-1",
		Tournament: TournamentIdentity{EventID: "event-1", TournamentID: "tournament-1", CompetitionID: "competition-1"},
		Price:      validParticipationPrice(), SettledAt: time.Now().UTC(),
	}
	if err := ValidatePaidParticipationConfirmation(confirmed); err != nil {
		t.Fatalf("valid confirmation: %v", err)
	}
	confirmed.Price.Currency = "usd"
	if err := ValidatePaidParticipationConfirmation(confirmed); !errors.Is(err, ErrInvalidEventRegistration) {
		t.Fatalf("untrusted currency mutation error = %v", err)
	}
}
