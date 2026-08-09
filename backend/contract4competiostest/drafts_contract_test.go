package contract4competiostest

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

func TestDraftContractHelpersExposeBoundedFormatsAndDefaults(t *testing.T) {
	for _, test := range []struct {
		rounds uint8
		want   uint16
		valid  bool
	}{
		{rounds: 0, valid: false},
		{rounds: 1, want: 2, valid: true},
		{rounds: 2, want: 4, valid: true},
		{rounds: 8, want: 256, valid: true},
		{rounds: 9, valid: false},
	} {
		capacity, valid := contract4competios.FixedKnockoutCapacity(test.rounds)
		if capacity != test.want || valid != test.valid {
			t.Errorf("FixedKnockoutCapacity(%d) = (%d, %t), want (%d, %t)", test.rounds, capacity, valid, test.want, test.valid)
		}
	}

	knockout := contract4competios.FixedKnockoutFormatCapability()
	if len(knockout.CapacityOptions) != 8 || knockout.ParticipantCountPerContest != 2 || knockout.AdvancementCountPerContest != 1 {
		t.Fatalf("knockout capability = %+v", knockout)
	}
	if knockout.CapacityOptions[0].Capacity != 2 || knockout.CapacityOptions[7].Capacity != 256 {
		t.Fatalf("knockout capacities = %+v", knockout.CapacityOptions)
	}
	for _, option := range knockout.CapacityOptions {
		if err := contract4competios.ValidateFormatStages(option.Stages); err != nil {
			t.Fatalf("invalid %d-round knockout topology: %v", option.Rounds, err)
		}
		if len(option.Stages) != int(option.Rounds) {
			t.Fatalf("%d-round topology has %d stages", option.Rounds, len(option.Stages))
		}
	}

	preferans := contract4competios.Preferans9FormatCapability()
	if len(preferans.Stages) != 2 || preferans.Stages[0].ContestCount != 3 ||
		preferans.Stages[0].ParticipantCountPerContest != 3 ||
		preferans.Stages[0].AdvancementCountPerContest != 1 {
		t.Fatalf("Preferans 9 capability = %+v", preferans)
	}
	destinations := preferans.Stages[0].QualificationDestinations
	if len(destinations) != 3 {
		t.Fatalf("Preferans 9 destinations = %+v", destinations)
	}
	seen := make(map[contract4competios.DestinationID]bool, len(destinations))
	for index, destination := range destinations {
		if seen[destination.DestinationID] ||
			destination.SourceContest != uint16(index) ||
			destination.QualifierIndex != 0 ||
			destination.TargetStageID != "final" ||
			destination.TargetContest != 0 ||
			destination.TargetSlot != uint8(index) {
			t.Fatalf("Preferans 9 destination = %+v", destination)
		}
		seen[destination.DestinationID] = true
	}
	if err := contract4competios.ValidateFormatStages(preferans.Stages); err != nil {
		t.Fatalf("invalid Preferans 9 topology: %v", err)
	}
	generic := contract4competios.FormatCapability{
		TemplateID:                 "n-player-fixture",
		ParticipantCountPerContest: 4,
		AdvancementCountPerContest: 2,
		Stages: []contract4competios.FormatStageCapability{{
			StageID:                    "fixture-stage",
			ContestCount:               1,
			ParticipantCountPerContest: 4,
			AdvancementCountPerContest: 2,
			QualificationDestinations: []contract4competios.QualificationDestination{
				{DestinationID: "fixture-slot-0", SourceContest: 0, QualifierIndex: 0, TargetStageID: "next", TargetContest: 0, TargetSlot: 0},
				{DestinationID: "fixture-slot-1", SourceContest: 0, QualifierIndex: 1, TargetStageID: "next", TargetContest: 0, TargetSlot: 1},
			},
		}, {
			StageID:                    "next",
			ContestCount:               1,
			ParticipantCountPerContest: 2,
			AdvancementCountPerContest: 1,
		}},
	}
	firstDestination := generic.Stages[0].QualificationDestinations[0]
	secondDestination := generic.Stages[0].QualificationDestinations[1]
	if generic.Stages[0].AdvancementCountPerContest < 2 ||
		firstDestination.SourceContest != secondDestination.SourceContest ||
		firstDestination.QualifierIndex == secondDestination.QualifierIndex ||
		firstDestination.TargetSlot == secondDestination.TargetSlot {
		t.Fatalf("generic N-player capability = %+v", generic)
	}
	if err := contract4competios.ValidateFormatStages(generic.Stages); err != nil {
		t.Fatalf("invalid generic N-player topology: %v", err)
	}
	parametersType := reflect.TypeOf(contract4competios.FormatParameters{})
	for _, forbidden := range []string{"ParticipantCount", "AdvancementCount"} {
		if _, exists := parametersType.FieldByName(forbidden); exists {
			t.Fatalf("FormatParameters accepts caller-supplied %s", forbidden)
		}
	}

	defaults := contract4competios.DefaultChessRaidersDraftContent("current")
	if defaults.Title != "Chess Raiders Cup" || defaults.GameID != "chess-raiders" ||
		defaults.ParticipantKind != contract4competios.ParticipantTeam ||
		defaults.Format.Parameters.Rounds != 2 || defaults.EconomicMode != contract4competios.EconomicModeFree {
		t.Fatalf("Chess Raiders defaults = %+v", defaults)
	}
	freeOnly := contract4competios.FreeOnlyEconomicModeCapabilities()
	if len(freeOnly) != 1 || freeOnly[0].Mode != contract4competios.EconomicModeFree {
		t.Fatalf("free-only economic modes = %+v", freeOnly)
	}
	serviceCapabilities := []contract4competios.CapabilityDescriptor{
		contract4competios.PaidParticipantEligibilityCapability(),
		contract4competios.ServiceFeeCapability(),
		contract4competios.TelegramStarsServicePaymentCapability(),
	}
	ids := make(map[contract4competios.CapabilityID]bool, len(serviceCapabilities))
	categories := make(map[contract4competios.CapabilityCategory]bool, len(serviceCapabilities))
	for _, capability := range serviceCapabilities {
		if ids[capability.ID] || categories[capability.Category] {
			t.Fatalf("service capabilities are not distinct: %+v", serviceCapabilities)
		}
		ids[capability.ID] = true
		categories[capability.Category] = true
	}
	starsJSON, err := json.Marshal(contract4competios.TelegramStarsServicePaymentCapability())
	if err != nil {
		t.Fatal(err)
	}
	stars := contract4competios.TelegramStarsServicePaymentCapability()
	if stars.Category != contract4competios.CapabilityCategoryServicePayment {
		t.Fatalf("Stars capability is not a service payment: %+v", stars)
	}
	starsType := reflect.TypeOf(stars)
	for _, forbidden := range []string{"EconomicMode", "Currency", "Mint"} {
		if _, exists := starsType.FieldByName(forbidden); exists {
			t.Fatalf("Stars capability can select or mint value through %s", forbidden)
		}
	}
	baselineJSON, err := json.Marshal(contract4competios.CapabilitiesProjection{EconomicModes: freeOnly})
	if err != nil {
		t.Fatal(err)
	}
	lowerBaseline := strings.ToLower(string(baselineJSON))
	if strings.Contains(lowerBaseline, "gold-commitment") ||
		strings.Contains(lowerBaseline, "coin") {
		t.Fatalf("free baseline advertises non-free value: %s", baselineJSON)
	}
	starsProjectionJSON, err := json.Marshal(contract4competios.CapabilitiesProjection{
		EconomicModes:     freeOnly,
		GatedCapabilities: []contract4competios.CapabilityDescriptor{stars},
	})
	if err != nil {
		t.Fatal(err)
	}
	lowerStarsProjection := strings.ToLower(string(starsProjectionJSON))
	if !strings.Contains(lowerStarsProjection, `"gatedcapabilities"`) ||
		strings.Contains(lowerStarsProjection, "gold-commitment") ||
		strings.Contains(lowerStarsProjection, "coin") ||
		strings.Contains(strings.ToLower(string(starsJSON)), "gold") ||
		strings.Contains(strings.ToLower(string(starsJSON)), "coin") {
		t.Fatalf("Stars capability is not service-only: %s", starsProjectionJSON)
	}
}

func TestDraftContractCarriesDistinctProjectionVersions(t *testing.T) {
	draft := contract4competios.DraftProjection{
		DraftID:          "draft-1",
		AggregateVersion: contract4competios.AggregateVersion("aggregate-7"),
		ContentRevision:  contract4competios.ContentRevision("content-12"),
		State:            contract4competios.DraftActive,
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		t.Fatal(err)
	}
	payload := string(encoded)
	for _, field := range []string{`"aggregateVersion":"aggregate-7"`, `"contentRevision":"content-12"`} {
		if !strings.Contains(payload, field) {
			t.Fatalf("draft JSON %q does not contain %s", payload, field)
		}
	}

	metadata := contract4competios.CommandMetadata{
		CommandID:                "command-1",
		ExpectedAggregateVersion: "aggregate-7",
	}
	if metadata.CommandID == "" || metadata.ExpectedAggregateVersion != draft.AggregateVersion {
		t.Fatalf("command metadata = %+v", metadata)
	}
	update := contract4competios.UpdateDraftCommand{
		Caller:   contract4competios.CallerContext{Principal: contract4competios.NewAuthenticatedPrincipal("actor")},
		Metadata: metadata,
		DraftID:  "draft-1",
		Patch:    contract4competios.DraftPatch{Title: stringPointer("New title")},
	}
	encoded, err = json.Marshal(update)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"commandID":"command-1"`, `"expectedAggregateVersion":"aggregate-7"`, `"draftID":"draft-1"`, `"title":"New title"`} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("update JSON %q does not contain %s", encoded, field)
		}
	}
}

func TestStaleVersionErrorCarriesOnlyAnOpaqueRetryVersion(t *testing.T) {
	err := &contract4competios.StaleVersionError{CurrentAggregateVersion: "v7"}
	if !errors.Is(err, contract4competios.ErrStaleVersion) {
		t.Fatalf("typed stale error does not unwrap ErrStaleVersion: %v", err)
	}
	var typed *contract4competios.StaleVersionError
	if !errors.As(err, &typed) || typed.CurrentAggregateVersion != "v7" {
		t.Fatalf("typed stale error = %+v", typed)
	}
	field, ok := reflect.TypeOf(contract4competios.StaleVersionError{}).FieldByName("CurrentAggregateVersion")
	if !ok || field.Type != reflect.TypeOf(contract4competios.AggregateVersion("")) {
		t.Fatalf("stale version detail is not opaque AggregateVersion: %+v", field)
	}
}

func stringPointer(value string) *string { return &value }

func TestDraftContractPrincipalAndCommandShapesDoNotExposeSurfaceAuthority(t *testing.T) {
	provisional := contract4competios.NewProvisionalTelegramPrincipal("bot", "validated-user")
	if provisional.Kind != contract4competios.PrincipalProvisional || provisional.Provisional == nil {
		t.Fatalf("provisional principal = %+v", provisional)
	}
	if anonymous := contract4competios.NewAnonymousPrincipal(); anonymous.Kind != contract4competios.PrincipalAnonymous || anonymous.ActorID != "" {
		t.Fatalf("anonymous principal = %+v", anonymous)
	}

	types := []reflect.Type{
		reflect.TypeOf(contract4competios.PrincipalRef{}),
		reflect.TypeOf(contract4competios.CallerContext{}),
		reflect.TypeOf(contract4competios.CommandMetadata{}),
		reflect.TypeOf(contract4competios.StartOrResumeDraftCommand{}),
		reflect.TypeOf(contract4competios.CreateAdditionalDraftCommand{}),
		reflect.TypeOf(contract4competios.MakePrimaryDraftCommand{}),
		reflect.TypeOf(contract4competios.UpdateDraftCommand{}),
		reflect.TypeOf(contract4competios.AbandonDraftCommand{}),
		reflect.TypeOf(contract4competios.SubmitDraftCommand{}),
		reflect.TypeOf(contract4competios.ClaimAndSubmitDraftCommand{}),
		reflect.TypeOf(contract4competios.InviteEntryCommand{}),
		reflect.TypeOf(contract4competios.RequestEntryCommand{}),
		reflect.TypeOf(contract4competios.AcceptEntryCommand{}),
		reflect.TypeOf(contract4competios.ApproveEntryCommand{}),
		reflect.TypeOf(contract4competios.RejectEntryCommand{}),
		reflect.TypeOf(contract4competios.RevokeInvitationCommand{}),
		reflect.TypeOf(contract4competios.WithdrawEntryCommand{}),
		reflect.TypeOf(contract4competios.LockEnrolmentCommand{}),
		reflect.TypeOf(contract4competios.SeedCompetitionCommand{}),
		reflect.TypeOf(contract4competios.DraftOutcome{}),
		reflect.TypeOf(contract4competios.CompetitionProjection{}),
	}
	for _, typ := range types {
		assertNoForbiddenFields(t, typ, map[reflect.Type]bool{})
	}
	claimType := reflect.TypeOf(contract4competios.ClaimAndSubmitDraftCommand{})
	if _, exists := claimType.FieldByName("ProvisionalPrincipal"); exists {
		t.Fatal("claim-and-submit accepts a caller-supplied provisional principal")
	}
	if claimType.NumField() != 3 {
		t.Fatalf("claim-and-submit fields = %d, want caller, metadata and draft ID only", claimType.NumField())
	}
	contextType := reflect.TypeOf(contract4competios.CallerContext{})
	if contextType.NumField() != 2 {
		t.Fatalf("CallerContext fields = %d, want released principal and Space only", contextType.NumField())
	}
}

func assertNoForbiddenFields(t *testing.T, typ reflect.Type, visited map[reflect.Type]bool) {
	t.Helper()
	if typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		assertNoForbiddenFields(t, typ.Elem(), visited)
		return
	}
	if typ.Kind() != reflect.Struct || visited[typ] {
		return
	}
	visited[typ] = true
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		name := strings.ToLower(field.Name)
		for _, forbidden := range []string{"authority", "chat", "role", "evidence", "wallet", "receipt"} {
			if strings.Contains(name, forbidden) {
				t.Fatalf("%s exposes forbidden field %q", typ, field.Name)
			}
		}
		assertNoForbiddenFields(t, field.Type, visited)
	}
}

func TestDraftApplicationPreservesMilestoneOneOperations(t *testing.T) {
	typeName := reflect.TypeOf((*contract4competios.DraftApplication)(nil)).Elem()
	want := []string{
		"GetCapabilities", "StartOrResumeDraft", "GetDraft", "ListDrafts", "UpdateDraft", "AbandonDraft", "SubmitDraft",
	}
	methods := make(map[string]bool, typeName.NumMethod())
	for index := 0; index < typeName.NumMethod(); index++ {
		methods[typeName.Method(index).Name] = true
	}
	for _, method := range want {
		if !methods[method] {
			t.Errorf("DraftApplication is missing %s", method)
		}
	}
	if typeName.NumMethod() != len(want) {
		t.Errorf("DraftApplication has %d methods, want the %d Milestone 1 methods", typeName.NumMethod(), len(want))
	}
}

func TestMultiDraftApplicationAddsMilestoneTwoOperations(t *testing.T) {
	typeName := reflect.TypeOf((*contract4competios.MultiDraftApplication)(nil)).Elem()
	want := []string{
		"GetCapabilities", "StartOrResumeDraft", "CreateAdditionalDraft", "MakePrimaryDraft", "GetDraft", "ListDrafts", "UpdateDraft", "AbandonDraft", "SubmitDraft",
	}
	methods := make(map[string]bool, typeName.NumMethod())
	for index := 0; index < typeName.NumMethod(); index++ {
		methods[typeName.Method(index).Name] = true
	}
	for _, method := range want {
		if !methods[method] {
			t.Errorf("MultiDraftApplication is missing %s", method)
		}
	}
	if typeName.NumMethod() != len(want) {
		t.Errorf("MultiDraftApplication has %d methods, want the %d Milestone 2 methods", typeName.NumMethod(), len(want))
	}
}

func TestProvisionalDraftApplicationAddsOnlyMilestoneThreeClaim(t *testing.T) {
	typeName := reflect.TypeOf((*contract4competios.ProvisionalDraftApplication)(nil)).Elem()
	want := []string{
		"GetCapabilities", "StartOrResumeDraft", "CreateAdditionalDraft",
		"MakePrimaryDraft", "GetDraft", "ListDrafts", "UpdateDraft",
		"AbandonDraft", "SubmitDraft", "ClaimAndSubmitDraft",
	}
	methods := make(map[string]bool, typeName.NumMethod())
	for index := 0; index < typeName.NumMethod(); index++ {
		methods[typeName.Method(index).Name] = true
	}
	for _, method := range want {
		if !methods[method] {
			t.Errorf("ProvisionalDraftApplication is missing %s", method)
		}
	}
	if typeName.NumMethod() != len(want) {
		t.Errorf("ProvisionalDraftApplication has %d methods, want %d", typeName.NumMethod(), len(want))
	}
}
