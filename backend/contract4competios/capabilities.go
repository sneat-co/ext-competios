package contract4competios

import (
	"errors"
	"fmt"
)

type EconomicMode string

const (
	EconomicModeFree EconomicMode = "free"
	// EconomicModeGoldCommitment is a future provider-gated competition mode.
	EconomicModeGoldCommitment EconomicMode = "gold-commitment"
)

type FormatParameters struct {
	Rounds uint8 `json:"rounds,omitempty"`
}

// FormatSelection contains only bounded, approved template parameters. The
// participant capacity and topology derive from the approved template and
// cannot be supplied independently by a caller.
type FormatSelection struct {
	TemplateID FormatTemplateID `json:"templateID"`
	Parameters FormatParameters `json:"parameters,omitempty"`
}

type FormatCapacityOption struct {
	Rounds   uint8                   `json:"rounds"`
	Capacity uint16                  `json:"capacity"`
	Stages   []FormatStageCapability `json:"stages"`
}

// QualificationDestination maps one qualifier from one source contest to a
// stable destination. It supports N-player contests with multiple qualifiers.
type QualificationDestination struct {
	DestinationID  DestinationID `json:"destinationID"`
	SourceContest  uint16        `json:"sourceContest"`
	QualifierIndex uint8         `json:"qualifierIndex"`
	TargetStageID  StageID       `json:"targetStageID"`
	TargetContest  uint16        `json:"targetContest"`
	TargetSlot     uint8         `json:"targetSlot"`
}

type FormatStageCapability struct {
	StageID                    StageID                    `json:"stageID"`
	ContestCount               uint16                     `json:"contestCount"`
	ParticipantCountPerContest uint8                      `json:"participantCountPerContest"`
	AdvancementCountPerContest uint8                      `json:"advancementCountPerContest"`
	QualificationDestinations  []QualificationDestination `json:"qualificationDestinations,omitempty"`
}

type FormatCapability struct {
	TemplateID                 FormatTemplateID        `json:"templateID"`
	DisplayName                string                  `json:"displayName"`
	Advanced                   bool                    `json:"advanced,omitempty"`
	ParticipantCountPerContest uint8                   `json:"participantCountPerContest"`
	AdvancementCountPerContest uint8                   `json:"advancementCountPerContest"`
	CapacityOptions            []FormatCapacityOption  `json:"capacityOptions,omitempty"`
	Stages                     []FormatStageCapability `json:"stages"`
}

const (
	FormatTemplateFixedKnockout FormatTemplateID = "fixed-knockout"
	// FormatTemplateByeCapableKnockoutV1 is an additive descriptor. Clients
	// select rounds (capacity) as usual; lock freezes any confirmed count in
	// the upper half through that capacity and records explicit seeded byes.
	FormatTemplateByeCapableKnockoutV1 FormatTemplateID = "bye-capable-knockout-v1"
	FormatTemplatePreferans9           FormatTemplateID = "preferans-9"
)

// FixedKnockoutCapacity derives capacity without accepting a separately
// supplied, potentially inconsistent capacity.
func FixedKnockoutCapacity(rounds uint8) (uint16, bool) {
	if rounds == 0 || rounds > 8 {
		return 0, false
	}
	return uint16(1) << rounds, true
}

func FixedKnockoutFormatCapability() FormatCapability {
	options := make([]FormatCapacityOption, 0, 8)
	for rounds := uint8(1); rounds <= 8; rounds++ {
		capacity, _ := FixedKnockoutCapacity(rounds)
		stages, _ := FixedKnockoutStages(rounds)
		options = append(options, FormatCapacityOption{
			Rounds:   rounds,
			Capacity: capacity,
			Stages:   stages,
		})
	}
	return FormatCapability{
		TemplateID:                 FormatTemplateFixedKnockout,
		DisplayName:                "Fixed knockout",
		ParticipantCountPerContest: 2,
		AdvancementCountPerContest: 1,
		CapacityOptions:            options,
	}
}

// ByeCapableKnockoutV1FormatCapability describes the versioned additive
// template. It intentionally reuses the approved fixed-knockout topology and
// capacity bounds. Counts in the upper half produce only first-round byes;
// the semantic difference is immutable lock-time bye evidence, never a
// fabricated participant or game contest.
func ByeCapableKnockoutV1FormatCapability() FormatCapability {
	options := make([]FormatCapacityOption, 0, 8)
	for rounds := uint8(1); rounds <= 8; rounds++ {
		capacity, _ := FixedKnockoutCapacity(rounds)
		stages, _ := FixedKnockoutStages(rounds)
		options = append(options, FormatCapacityOption{Rounds: rounds, Capacity: capacity, Stages: stages})
	}
	return FormatCapability{
		TemplateID:                 FormatTemplateByeCapableKnockoutV1,
		DisplayName:                "Bye-capable knockout (v1)",
		Advanced:                   true,
		ParticipantCountPerContest: 2,
		AdvancementCountPerContest: 1,
		CapacityOptions:            options,
	}
}

// FixedKnockoutStages returns the complete zero-based topology for one approved
// round count. SourceContest, QualifierIndex, TargetContest and TargetSlot use
// the same zero-based convention as the Competios domain graph.
func FixedKnockoutStages(rounds uint8) ([]FormatStageCapability, bool) {
	if _, valid := FixedKnockoutCapacity(rounds); !valid {
		return nil, false
	}
	stages := make([]FormatStageCapability, rounds)
	for stageIndex := uint8(0); stageIndex < rounds; stageIndex++ {
		contestCount := uint16(1) << (rounds - stageIndex - 1)
		stage := FormatStageCapability{
			StageID:                    StageID(fmt.Sprintf("round-%d", stageIndex)),
			ContestCount:               contestCount,
			ParticipantCountPerContest: 2,
			AdvancementCountPerContest: 1,
		}
		if stageIndex+1 < rounds {
			stage.QualificationDestinations = make(
				[]QualificationDestination,
				0,
				contestCount,
			)
			for sourceContest := uint16(0); sourceContest < contestCount; sourceContest++ {
				stage.QualificationDestinations = append(
					stage.QualificationDestinations,
					QualificationDestination{
						DestinationID: DestinationID(fmt.Sprintf(
							"round-%d-contest-%d-qualifier-0",
							stageIndex,
							sourceContest,
						)),
						SourceContest:  sourceContest,
						QualifierIndex: 0,
						TargetStageID:  StageID(fmt.Sprintf("round-%d", stageIndex+1)),
						TargetContest:  sourceContest / 2,
						TargetSlot:     uint8(sourceContest % 2),
					},
				)
			}
		}
		stages[stageIndex] = stage
	}
	return stages, true
}

func Preferans9FormatCapability() FormatCapability {
	stages := []FormatStageCapability{
		{
			StageID:                    StageID("semifinal"),
			ContestCount:               3,
			ParticipantCountPerContest: 3,
			AdvancementCountPerContest: 1,
			QualificationDestinations: []QualificationDestination{
				{DestinationID: DestinationID("final-slot-0"), SourceContest: 0, QualifierIndex: 0, TargetStageID: StageID("final"), TargetContest: 0, TargetSlot: 0},
				{DestinationID: DestinationID("final-slot-1"), SourceContest: 1, QualifierIndex: 0, TargetStageID: StageID("final"), TargetContest: 0, TargetSlot: 1},
				{DestinationID: DestinationID("final-slot-2"), SourceContest: 2, QualifierIndex: 0, TargetStageID: StageID("final"), TargetContest: 0, TargetSlot: 2},
			},
		},
		{
			StageID:                    StageID("final"),
			ContestCount:               1,
			ParticipantCountPerContest: 3,
			AdvancementCountPerContest: 1,
		},
	}
	return FormatCapability{
		TemplateID:                 FormatTemplatePreferans9,
		DisplayName:                "Preferans 9",
		ParticipantCountPerContest: 3,
		AdvancementCountPerContest: 1,
		CapacityOptions:            []FormatCapacityOption{{Capacity: 9, Stages: stages}},
		Stages:                     stages,
	}
}

var ErrInvalidFormatCapability = errors.New("competios contract: invalid format capability")

// ValidateFormatStages validates an approved adapter descriptor. It does not
// expose a user-editable graph: callers select only a published template and
// its bounded parameters.
func ValidateFormatStages(stages []FormatStageCapability) error {
	if len(stages) == 0 {
		return ErrInvalidFormatCapability
	}
	stageIndexes := make(map[StageID]int, len(stages))
	for index, stage := range stages {
		if stage.StageID == "" || stage.ContestCount == 0 ||
			stage.ParticipantCountPerContest < 2 ||
			stage.AdvancementCountPerContest == 0 ||
			stage.AdvancementCountPerContest > stage.ParticipantCountPerContest {
			return ErrInvalidFormatCapability
		}
		if _, duplicate := stageIndexes[stage.StageID]; duplicate {
			return ErrInvalidFormatCapability
		}
		stageIndexes[stage.StageID] = index
	}
	targetSlots := make(map[string]struct{})
	incomingByStage := make(map[StageID]int)
	sourceQualifiers := make(map[string]struct{})
	for sourceStageIndex, stage := range stages {
		for _, destination := range stage.QualificationDestinations {
			targetStageIndex, exists := stageIndexes[destination.TargetStageID]
			if destination.DestinationID == "" || !exists ||
				targetStageIndex <= sourceStageIndex ||
				destination.SourceContest >= stage.ContestCount ||
				destination.QualifierIndex >= stage.AdvancementCountPerContest {
				return ErrInvalidFormatCapability
			}
			target := stages[targetStageIndex]
			if destination.TargetContest >= target.ContestCount ||
				destination.TargetSlot >= target.ParticipantCountPerContest {
				return ErrInvalidFormatCapability
			}
			targetKey := fmt.Sprintf(
				"%s:%d:%d",
				destination.TargetStageID,
				destination.TargetContest,
				destination.TargetSlot,
			)
			if _, duplicate := targetSlots[targetKey]; duplicate {
				return ErrInvalidFormatCapability
			}
			targetSlots[targetKey] = struct{}{}
			sourceKey := fmt.Sprintf(
				"%s:%d:%d",
				stage.StageID,
				destination.SourceContest,
				destination.QualifierIndex,
			)
			if _, duplicate := sourceQualifiers[sourceKey]; duplicate {
				return ErrInvalidFormatCapability
			}
			sourceQualifiers[sourceKey] = struct{}{}
			incomingByStage[destination.TargetStageID]++
		}
	}
	for index := 1; index < len(stages); index++ {
		stage := stages[index]
		requiredIncoming := int(stage.ContestCount) * int(stage.ParticipantCountPerContest)
		if incomingByStage[stage.StageID] != requiredIncoming {
			return ErrInvalidFormatCapability
		}
	}
	return nil
}

type RulesetCapability struct {
	RulesetVersion RulesetVersion `json:"rulesetVersion"`
	DisplayName    string         `json:"displayName"`
}

type GameCapability struct {
	GameID              GameID              `json:"gameID"`
	DisplayName         string              `json:"displayName"`
	Rulesets            []RulesetCapability `json:"rulesets"`
	FormatTemplateIDs   []FormatTemplateID  `json:"formatTemplateIDs"`
	DefaultDraftContent *DraftContent       `json:"defaultDraftContent,omitempty"`
}

type EconomicModeCapability struct {
	Mode        EconomicMode `json:"mode"`
	DisplayName string       `json:"displayName"`
	Advanced    bool         `json:"advanced,omitempty"`
}

type CapabilityID string
type CapabilityCategory string

const (
	CapabilityPaidParticipantEligibility  CapabilityID = "paid-participant-eligibility"
	CapabilityServiceFee                  CapabilityID = "service-fee"
	CapabilityTelegramStarsServicePayment CapabilityID = "telegram-stars-service-payment"
)

const (
	CapabilityCategoryParticipantEligibility CapabilityCategory = "participant-eligibility"
	CapabilityCategoryServiceFee             CapabilityCategory = "service-fee"
	CapabilityCategoryServicePayment         CapabilityCategory = "service-payment"
)

// CapabilityDescriptor describes an independently gated service capability.
// Telegram Stars service payment pays only for an eligible service; it cannot
// select or mint Gold or Coins.
type CapabilityDescriptor struct {
	ID          CapabilityID       `json:"id"`
	Category    CapabilityCategory `json:"category"`
	DisplayName string             `json:"displayName"`
	Advanced    bool               `json:"advanced,omitempty"`
}

type CapabilitiesRequest struct {
	Caller CallerContext `json:"caller"`
	GameID GameID        `json:"gameID,omitempty"`
}

type CapabilitiesProjection struct {
	Games             []GameCapability         `json:"games"`
	Formats           []FormatCapability       `json:"formats"`
	EconomicModes     []EconomicModeCapability `json:"economicModes"`
	GatedCapabilities []CapabilityDescriptor   `json:"gatedCapabilities,omitempty"`
}

func FreeEconomicModeCapability() EconomicModeCapability {
	return EconomicModeCapability{Mode: EconomicModeFree, DisplayName: "Free"}
}

// FreeOnlyEconomicModeCapabilities is the first-slice economic baseline.
func FreeOnlyEconomicModeCapabilities() []EconomicModeCapability {
	return []EconomicModeCapability{FreeEconomicModeCapability()}
}

func PaidParticipantEligibilityCapability() CapabilityDescriptor {
	return CapabilityDescriptor{
		ID:          CapabilityPaidParticipantEligibility,
		Category:    CapabilityCategoryParticipantEligibility,
		DisplayName: "Paid participant eligibility",
	}
}

func ServiceFeeCapability() CapabilityDescriptor {
	return CapabilityDescriptor{
		ID:          CapabilityServiceFee,
		Category:    CapabilityCategoryServiceFee,
		DisplayName: "Service fee",
	}
}

func TelegramStarsServicePaymentCapability() CapabilityDescriptor {
	return CapabilityDescriptor{
		ID:          CapabilityTelegramStarsServicePayment,
		Category:    CapabilityCategoryServicePayment,
		DisplayName: "Telegram Stars service payment",
	}
}

func DefaultChessRaidersDraftContent(rulesetVersion RulesetVersion) DraftContent {
	return DraftContent{
		Title:            "Chess Raiders Cup",
		GameID:           GameID("chess-raiders"),
		RulesetVersion:   rulesetVersion,
		ParticipantKind:  ParticipantTeam,
		Visibility:       VisibilityPublic,
		EnrolmentPolicy:  EnrolmentApprovalRequired,
		Format:           FormatSelection{TemplateID: FormatTemplateFixedKnockout, Parameters: FormatParameters{Rounds: 2}},
		SeedingPolicy:    SeedingRandom,
		SchedulingPolicy: SchedulingAfterDraw,
		EconomicMode:     EconomicModeFree,
	}
}
