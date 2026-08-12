package grants

import (
	"errors"
	"testing"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

func TestNewIssuerRejectsMissingFields(t *testing.T) {
	key := testHMACKey(t, "kid")
	replay := NewMemoryReplayStore(nil)
	cases := map[string]Direction{
		"no issuer":   {Subject: "s", Audience: "a", Purposes: chessPurposes(), Key: key},
		"no subject":  {Issuer: "i", Audience: "a", Purposes: chessPurposes(), Key: key},
		"no audience": {Issuer: "i", Subject: "s", Purposes: chessPurposes(), Key: key},
		"no purposes": {Issuer: "i", Subject: "s", Audience: "a", Key: key},
		"no key":      {Issuer: "i", Subject: "s", Audience: "a", Purposes: chessPurposes()},
		"unknown purpose": {
			Issuer: "i", Subject: "s", Audience: "a", Key: key,
			Purposes: []contract4competios.GrantPurpose{"not-a-real-purpose"},
		},
		"duplicate purpose": {
			Issuer: "i", Subject: "s", Audience: "a", Key: key,
			Purposes: []contract4competios.GrantPurpose{
				contract4competios.GrantPurposeContestLaunch,
				contract4competios.GrantPurposeContestLaunch,
			},
		},
	}
	_ = replay
	for name, direction := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewIssuer(direction); !errors.Is(err, ErrDirectionMisconfigured) {
				t.Fatalf("NewIssuer(%s) err = %v, want ErrDirectionMisconfigured", name, err)
			}
		})
	}
}

func TestNewVerifierRejectsMissingFields(t *testing.T) {
	key := testHMACKey(t, "kid")
	replay := NewMemoryReplayStore(nil)
	cases := map[string]Direction{
		"no issuer":   {Audience: "a", Purposes: chessPurposes(), Key: key, Replay: replay},
		"no audience": {Issuer: "i", Purposes: chessPurposes(), Key: key, Replay: replay},
		"no purposes": {Issuer: "i", Audience: "a", Key: key, Replay: replay},
		"no key":      {Issuer: "i", Audience: "a", Purposes: chessPurposes(), Replay: replay},
		"no replay":   {Issuer: "i", Audience: "a", Purposes: chessPurposes(), Key: key},
		"leeway too big": {
			Issuer: "i", Audience: "a", Purposes: chessPurposes(), Key: key, Replay: replay,
			Leeway: MaxVerifierLeeway + time.Second,
		},
		"negative leeway": {
			Issuer: "i", Audience: "a", Purposes: chessPurposes(), Key: key, Replay: replay,
			Leeway: -time.Second,
		},
	}
	for name, direction := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewVerifier(direction); !errors.Is(err, ErrDirectionMisconfigured) {
				t.Fatalf("NewVerifier(%s) err = %v, want ErrDirectionMisconfigured", name, err)
			}
		})
	}
}

func TestNewVerifierAcceptsLeewayAtBound(t *testing.T) {
	key := testHMACKey(t, "kid")
	direction := Direction{
		Issuer: "i", Audience: "a", Purposes: chessPurposes(), Key: key,
		Replay: NewMemoryReplayStore(nil), Leeway: MaxVerifierLeeway,
	}
	if _, err := NewVerifier(direction); err != nil {
		t.Fatalf("unexpected error at leeway bound: %v", err)
	}
}

func TestDirectionKeyByIDFindsTrustedAndActiveKeys(t *testing.T) {
	active := testHMACKey(t, "active")
	trusted := testHMACKey(t, "trusted-old")
	direction := Direction{Key: active, Trusted: []KeyMaterial{trusted}}
	if got := direction.keyByID("active"); got == nil || got.KeyID() != "active" {
		t.Fatalf("keyByID(active) = %v", got)
	}
	if got := direction.keyByID("trusted-old"); got == nil || got.KeyID() != "trusted-old" {
		t.Fatalf("keyByID(trusted-old) = %v", got)
	}
	if got := direction.keyByID("nope"); got != nil {
		t.Fatalf("keyByID(nope) = %v, want nil", got)
	}
	if got := direction.keyByID(""); got != nil {
		t.Fatalf("keyByID(\"\") = %v, want nil", got)
	}
}
