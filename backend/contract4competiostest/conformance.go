package contract4competiostest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type GameLauncherFactory func() contract4competios.GameLauncher
type ResultSinkFactory func() contract4competios.ResultSink
type ContestStartSinkFactory func() contract4competios.ContestStartSink

func CheckGameLauncher(factory GameLauncherFactory) []error {
	return CheckGameLauncherWithRequest(factory, launchFixture())
}

func CheckGameLauncherWithRequest(
	factory GameLauncherFactory,
	request contract4competios.LaunchRequest,
) []error {
	ctx := context.Background()
	launcher := factory()
	first, err := launcher.LaunchContest(ctx, request)
	if err != nil {
		return []error{fmt.Errorf("first launch: %w", err)}
	}
	if first.GameInstanceID == "" || first.Replayed {
		return []error{fmt.Errorf("first launch outcome = %+v", first)}
	}
	replay, err := launcher.LaunchContest(ctx, request)
	var violations []error
	if err != nil {
		violations = append(violations, fmt.Errorf("launch replay: %w", err))
	} else if replay.GameInstanceID != first.GameInstanceID || !replay.Replayed {
		violations = append(violations, fmt.Errorf(
			"launch replay = %+v, want game %q replayed",
			replay,
			first.GameInstanceID,
		))
	}
	conflict := request
	conflict.RulesetVersion = "different"
	if _, err := launcher.LaunchContest(ctx, conflict); !errors.Is(err, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf(
			"conflicting launch error = %v, want ErrCommandConflict",
			err,
		))
	}
	distinct := request
	distinct.CommandID += "-distinct"
	distinct.ContestID += "-distinct"
	second, err := launcher.LaunchContest(ctx, distinct)
	if err != nil {
		violations = append(violations, fmt.Errorf("distinct launch: %w", err))
	} else if second.GameInstanceID == "" ||
		second.GameInstanceID == first.GameInstanceID ||
		second.Replayed {
		violations = append(violations, fmt.Errorf(
			"distinct launch outcome = %+v, first game %q",
			second,
			first.GameInstanceID,
		))
	}
	return violations
}

func CheckResultSink(factory ResultSinkFactory) []error {
	return CheckResultSinkWithResult(factory, "result-command", resultFixture())
}

func CheckContestStartSink(factory ContestStartSinkFactory) []error {
	return CheckContestStartSinkWithStart(factory, "start-command", contestStartFixture())
}

func CheckContestStartSinkWithStart(
	factory ContestStartSinkFactory,
	commandID string,
	start contract4competios.ContestStart,
) []error {
	ctx := context.Background()
	sink := factory()
	first, err := sink.SubmitContestStart(ctx, commandID, start)
	if err != nil {
		return []error{fmt.Errorf("first contest start: %w", err)}
	}
	if first.Status != contract4competios.ContestStartAcknowledgementAccepted {
		return []error{fmt.Errorf("first contest start acknowledgement = %+v", first)}
	}
	replay, err := sink.SubmitContestStart(ctx, commandID, start)
	var violations []error
	if err != nil {
		violations = append(violations, fmt.Errorf("contest start replay: %w", err))
	} else if replay.Status != contract4competios.ContestStartAcknowledgementReplayed {
		violations = append(violations, fmt.Errorf(
			"contest start replay acknowledgement = %+v", replay,
		))
	}
	conflict := start
	conflict.GameInstanceID = "different-game"
	if _, err := sink.SubmitContestStart(ctx, commandID, conflict); !errors.Is(err, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf(
			"conflicting contest start error = %v, want ErrCommandConflict",
			err,
		))
	}
	return violations
}

func CheckResultSinkWithResult(
	factory ResultSinkFactory,
	commandID string,
	result contract4competios.OrderedResult,
) []error {
	ctx := context.Background()
	sink := factory()
	first, err := sink.SubmitResult(ctx, commandID, result)
	if err != nil {
		return []error{fmt.Errorf("first result: %w", err)}
	}
	if first.Status != contract4competios.ResultAcknowledgementAccepted {
		return []error{fmt.Errorf("first acknowledgement = %+v", first)}
	}
	replay, err := sink.SubmitResult(ctx, commandID, result)
	var violations []error
	if err != nil {
		violations = append(violations, fmt.Errorf("result replay: %w", err))
	} else if replay.Status != contract4competios.ResultAcknowledgementReplayed {
		violations = append(violations, fmt.Errorf("result replay acknowledgement = %+v", replay))
	}
	conflict := result
	conflict.Placements = append([]contract4competios.Placement(nil), result.Placements...)
	conflict.Placements[0].Rank = 2
	if _, err := sink.SubmitResult(ctx, commandID, conflict); !errors.Is(err, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf(
			"conflicting result error = %v, want ErrCommandConflict",
			err,
		))
	}
	outOfOrder := result
	outOfOrder.Placements = append([]contract4competios.Placement(nil), result.Placements...)
	if len(outOfOrder.Placements) > 1 {
		outOfOrder.Placements[0], outOfOrder.Placements[1] =
			outOfOrder.Placements[1], outOfOrder.Placements[0]
		if _, err := factory().SubmitResult(
			ctx,
			commandID+"-out-of-order",
			outOfOrder,
		); err == nil {
			violations = append(violations, errors.New("out-of-order placements were accepted"))
		}
	}
	unknown := result
	unknown.Placements = append([]contract4competios.Placement(nil), result.Placements...)
	if len(unknown.Placements) > 0 {
		unknown.Placements[0].EntryID = "unknown-entry"
		if _, err := factory().SubmitResult(
			ctx,
			commandID+"-unknown",
			unknown,
		); err == nil {
			violations = append(violations, errors.New("unknown participant was accepted"))
		}
	}
	duplicateRanks := result
	duplicateRanks.Placements = append([]contract4competios.Placement(nil), result.Placements...)
	if len(duplicateRanks.Placements) > 1 {
		duplicateRanks.Placements[1].Rank = duplicateRanks.Placements[0].Rank
		if _, err := factory().SubmitResult(
			ctx,
			commandID+"-duplicate-ranks",
			duplicateRanks,
		); err == nil {
			violations = append(violations, errors.New("forbidden duplicate ranks were accepted"))
		}
	}
	return violations
}

func launchFixture() contract4competios.LaunchRequest {
	return contract4competios.LaunchRequest{
		CompetitionID:  "cup",
		ContestID:      "contest",
		CommandID:      "launch-command",
		GameID:         "chess-raiders",
		RulesetVersion: "v1",
		Assignments: []contract4competios.SideAssignment{
			{EntryID: "white", Side: "white", Lineup: []contract4competios.LineupMember{{UserID: "w1"}}},
			{EntryID: "black", Side: "black", Lineup: []contract4competios.LineupMember{{UserID: "b1"}}},
		},
		StartsAt: time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
}

func resultFixture() contract4competios.OrderedResult {
	return contract4competios.OrderedResult{
		ID:             "result",
		CompetitionID:  "cup",
		ContestID:      "contest",
		RulesetVersion: "v1",
		GameAdapterID:  "chess-raiders",
		GameInstanceID: "game-1",
		Placements: []contract4competios.Placement{
			{EntryID: "white", Rank: 1, Status: contract4competios.PlacementStatusFinished},
			{EntryID: "black", Rank: 2, Status: contract4competios.PlacementStatusFinished},
		},
		CompletedAt: time.Date(2026, 7, 27, 13, 0, 0, 0, time.UTC),
	}
}

func contestStartFixture() contract4competios.ContestStart {
	return contract4competios.ContestStart{
		ID:             "start",
		CompetitionID:  "cup",
		ContestID:      "contest",
		GameAdapterID:  "chess-raiders",
		GameInstanceID: "game-1",
		StartedAt:      time.Date(2026, 7, 27, 12, 30, 0, 0, time.UTC),
	}
}
