package contract4competios

import (
	"errors"
	"testing"
)

// TestCallerContextPositionalLiteralCompiles protects the exact released
// two-field layout. The external contract suite also asserts the field count
// and names; this same-package compile guard avoids go vet's unkeyed-literal
// diagnostic while still making an added field a compile failure.
func TestCallerContextPositionalLiteralCompiles(t *testing.T) {
	_ = CallerContext{
		NewAuthenticatedPrincipal("m1-positional-owner"),
		"m1-positional-space",
	}
}

func TestPublishedCapabilitiesAreInternallyConsistent(t *testing.T) {
	for rounds := uint8(1); rounds <= 8; rounds++ {
		capacity, ok := FixedKnockoutCapacity(rounds)
		if !ok || capacity != uint16(1)<<rounds {
			t.Fatalf("capacity(%d) = %d/%v", rounds, capacity, ok)
		}
		stages, ok := FixedKnockoutStages(rounds)
		if !ok || len(stages) != int(rounds) {
			t.Fatalf("stages(%d) = %#v/%v", rounds, stages, ok)
		}
		if err := ValidateFormatStages(stages); err != nil {
			t.Fatalf("stages(%d) validation: %v", rounds, err)
		}
	}
	for _, rounds := range []uint8{0, 9} {
		if capacity, ok := FixedKnockoutCapacity(rounds); ok || capacity != 0 {
			t.Fatalf("invalid capacity(%d) = %d/%v", rounds, capacity, ok)
		}
		if stages, ok := FixedKnockoutStages(rounds); ok || stages != nil {
			t.Fatalf("invalid stages(%d) = %#v/%v", rounds, stages, ok)
		}
	}
	if err := ValidateFormatStages(Preferans9FormatCapability().Stages); err != nil {
		t.Fatalf("Preferans 9 topology validation: %v", err)
	}
	if got := FixedKnockoutFormatCapability(); len(got.CapacityOptions) != 8 || got.TemplateID != FormatTemplateFixedKnockout {
		t.Fatalf("fixed knockout capability = %#v", got)
	}
	byeCapable := ByeCapableKnockoutV1FormatCapability()
	if byeCapable.TemplateID != FormatTemplateByeCapableKnockoutV1 || !byeCapable.Advanced ||
		byeCapable.DisplayName != "Bye-capable knockout (v1)" || len(byeCapable.CapacityOptions) != 8 {
		t.Fatalf("bye-capable v1 capability = %#v", byeCapable)
	}
	for _, option := range byeCapable.CapacityOptions {
		if err := ValidateFormatStages(option.Stages); err != nil {
			t.Fatalf("bye-capable rounds %d descriptor = %v", option.Rounds, err)
		}
	}
}

func TestValidateFormatStagesRejectsTamperedTopology(t *testing.T) {
	valid, ok := FixedKnockoutStages(2)
	if !ok {
		t.Fatal("fixed knockout stages unavailable")
	}
	clone := func() []FormatStageCapability {
		result := make([]FormatStageCapability, len(valid))
		copy(result, valid)
		for index := range result {
			result[index].QualificationDestinations = append([]QualificationDestination(nil), result[index].QualificationDestinations...)
		}
		return result
	}
	tests := []struct {
		name   string
		stages func() []FormatStageCapability
	}{
		{"empty", func() []FormatStageCapability { return nil }},
		{"duplicate-stage", func() []FormatStageCapability {
			stages := clone()
			stages[1].StageID = stages[0].StageID
			return stages
		}},
		{"backward-destination", func() []FormatStageCapability {
			stages := clone()
			stages[0].QualificationDestinations[0].TargetStageID = stages[0].StageID
			return stages
		}},
		{"duplicate-target-slot", func() []FormatStageCapability {
			stages := clone()
			stages[0].QualificationDestinations[1].TargetContest = 0
			stages[0].QualificationDestinations[1].TargetSlot = 0
			return stages
		}},
		{"missing-incoming-slot", func() []FormatStageCapability {
			stages := clone()
			stages[0].QualificationDestinations = stages[0].QualificationDestinations[:1]
			return stages
		}},
		{"source-contest-out-of-range", func() []FormatStageCapability {
			stages := clone()
			stages[0].QualificationDestinations[0].SourceContest = stages[0].ContestCount
			return stages
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateFormatStages(test.stages()); !errors.Is(err, ErrInvalidFormatCapability) {
				t.Fatalf("validation error = %v, want ErrInvalidFormatCapability", err)
			}
		})
	}
}

func TestAuthorityAndEconomicCapabilityDescriptorsRemainNarrow(t *testing.T) {
	authenticated := NewAuthenticatedPrincipal("actor")
	if authenticated.Kind != PrincipalAuthenticated || authenticated.ActorID != "actor" || authenticated.Provisional != nil {
		t.Fatalf("authenticated principal = %#v", authenticated)
	}
	provisional := NewProvisionalTelegramPrincipal("bot", "telegram-user")
	if provisional.Kind != PrincipalProvisional || provisional.Provisional == nil || provisional.ActorID != "" {
		t.Fatalf("provisional principal = %#v", provisional)
	}
	if anonymous := NewAnonymousPrincipal(); anonymous.Kind != PrincipalAnonymous || anonymous.ActorID != "" || anonymous.Provisional != nil {
		t.Fatalf("anonymous principal = %#v", anonymous)
	}
	if modes := FreeOnlyEconomicModeCapabilities(); len(modes) != 1 || modes[0] != FreeEconomicModeCapability() || modes[0].Mode != EconomicModeFree {
		t.Fatalf("economic modes = %#v", modes)
	}
	descriptors := []CapabilityDescriptor{PaidParticipantEligibilityCapability(), ServiceFeeCapability(), TelegramStarsServicePaymentCapability()}
	for _, descriptor := range descriptors {
		if descriptor.ID == "" || descriptor.Category == "" || descriptor.Advanced {
			t.Fatalf("capability descriptor = %#v", descriptor)
		}
	}
	defaults := DefaultChessRaidersDraftContent("rules-v1")
	if defaults.GameID != "chess-raiders" || defaults.RulesetVersion != "rules-v1" || defaults.EconomicMode != EconomicModeFree || defaults.Format.Parameters.Rounds != 2 {
		t.Fatalf("draft defaults = %#v", defaults)
	}
}

func TestStaleVersionErrorRetainsOnlyOpaqueRetryValue(t *testing.T) {
	err := &StaleVersionError{CurrentAggregateVersion: "v42"}
	if err.Error() != ErrStaleVersion.Error() || !errors.Is(err, ErrStaleVersion) || err.CurrentAggregateVersion != "v42" {
		t.Fatalf("stale version error = %#v", err)
	}
}
