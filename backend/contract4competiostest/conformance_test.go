package contract4competiostest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type invalidLauncher struct{}

func (invalidLauncher) LaunchContest(
	context.Context,
	contract4competios.LaunchRequest,
) (contract4competios.LaunchOutcome, error) {
	return contract4competios.LaunchOutcome{GameInstanceID: "new-every-time"}, nil
}

type invalidResultSink struct{}

func (invalidResultSink) SubmitResult(
	context.Context,
	string,
	contract4competios.OrderedResult,
) (contract4competios.ResultAcknowledgement, error) {
	return contract4competios.ResultAcknowledgement{
		Status: contract4competios.ResultAcknowledgementAccepted,
	}, nil
}

type invalidContestStartSink struct{}

func (invalidContestStartSink) SubmitContestStart(
	context.Context,
	string,
	contract4competios.ContestStart,
) (contract4competios.ContestStartAcknowledgement, error) {
	return contract4competios.ContestStartAcknowledgement{
		Status: contract4competios.ContestStartAcknowledgementAccepted,
	}, nil
}

type conformingLauncher struct {
	mu           sync.Mutex
	fingerprints map[string][]byte
	games        map[string]string
}

func (f *conformingLauncher) LaunchContest(
	_ context.Context,
	request contract4competios.LaunchRequest,
) (contract4competios.LaunchOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fingerprint, err := json.Marshal(request)
	if err != nil {
		return contract4competios.LaunchOutcome{}, err
	}
	if f.fingerprints == nil {
		f.fingerprints = make(map[string][]byte)
		f.games = make(map[string]string)
	}
	if previous, ok := f.fingerprints[request.CommandID]; ok {
		if !bytes.Equal(previous, fingerprint) {
			return contract4competios.LaunchOutcome{}, contract4competios.ErrCommandConflict
		}
		return contract4competios.LaunchOutcome{
			GameInstanceID: f.games[request.CommandID],
			Replayed:       true,
		}, nil
	}
	gameID := fmt.Sprintf("game-%d", len(f.fingerprints)+1)
	f.fingerprints[request.CommandID] = fingerprint
	f.games[request.CommandID] = gameID
	return contract4competios.LaunchOutcome{GameInstanceID: gameID}, nil
}

type conformingResultSink struct {
	mu          sync.Mutex
	commandID   string
	fingerprint []byte
}

type conformingContestStartSink struct {
	mu          sync.Mutex
	commandID   string
	fingerprint []byte
}

func (f *conformingContestStartSink) SubmitContestStart(
	_ context.Context,
	commandID string,
	start contract4competios.ContestStart,
) (contract4competios.ContestStartAcknowledgement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fingerprint, err := json.Marshal(start)
	if err != nil {
		return contract4competios.ContestStartAcknowledgement{}, err
	}
	if f.fingerprint != nil {
		if f.commandID != commandID || !bytes.Equal(f.fingerprint, fingerprint) {
			return contract4competios.ContestStartAcknowledgement{}, contract4competios.ErrCommandConflict
		}
		return contract4competios.ContestStartAcknowledgement{
			Status: contract4competios.ContestStartAcknowledgementReplayed,
		}, nil
	}
	if start.ID == "" || start.CompetitionID != "cup" || start.ContestID != "contest" ||
		start.GameAdapterID != "chess-raiders" || start.GameInstanceID != "game-1" || start.StartedAt.IsZero() {
		return contract4competios.ContestStartAcknowledgement{}, errors.New("invalid contest start")
	}
	f.commandID = commandID
	f.fingerprint = fingerprint
	return contract4competios.ContestStartAcknowledgement{
		Status: contract4competios.ContestStartAcknowledgementAccepted,
	}, nil
}

func (f *conformingResultSink) SubmitResult(
	_ context.Context,
	commandID string,
	result contract4competios.OrderedResult,
) (contract4competios.ResultAcknowledgement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fingerprint, err := json.Marshal(result)
	if err != nil {
		return contract4competios.ResultAcknowledgement{}, err
	}
	if f.fingerprint != nil {
		if f.commandID != commandID || !bytes.Equal(f.fingerprint, fingerprint) {
			return contract4competios.ResultAcknowledgement{}, contract4competios.ErrCommandConflict
		}
		return contract4competios.ResultAcknowledgement{
			Status: contract4competios.ResultAcknowledgementReplayed,
		}, nil
	}
	if len(result.Placements) != 2 ||
		result.Placements[0].EntryID != "white" ||
		result.Placements[0].Rank != 1 ||
		result.Placements[1].EntryID != "black" ||
		result.Placements[1].Rank != 2 {
		return contract4competios.ResultAcknowledgement{}, errors.New("invalid ordered result")
	}
	f.commandID = commandID
	f.fingerprint = fingerprint
	return contract4competios.ResultAcknowledgement{
		Status: contract4competios.ResultAcknowledgementAccepted,
	}, nil
}

func TestGameLauncherConformanceAcceptsReferenceFake(t *testing.T) {
	violations := CheckGameLauncher(func() contract4competios.GameLauncher {
		return &conformingLauncher{}
	})
	if len(violations) != 0 {
		t.Fatalf("conforming launcher violations: %v", violations)
	}
}

func TestResultSinkConformanceAcceptsReferenceFake(t *testing.T) {
	violations := CheckResultSink(func() contract4competios.ResultSink {
		return &conformingResultSink{}
	})
	if len(violations) != 0 {
		t.Fatalf("conforming result sink violations: %v", violations)
	}
}

func TestContestStartSinkConformanceAcceptsReferenceFake(t *testing.T) {
	violations := CheckContestStartSink(func() contract4competios.ContestStartSink {
		return &conformingContestStartSink{}
	})
	if len(violations) != 0 {
		t.Fatalf("conforming contest start sink violations: %v", violations)
	}
}

func TestGameLauncherConformanceRejectsInvalidImplementation(t *testing.T) {
	violations := CheckGameLauncher(func() contract4competios.GameLauncher {
		return invalidLauncher{}
	})
	if len(violations) < 2 {
		t.Fatalf("invalid launcher produced %d violations, want at least 2: %v", len(violations), violations)
	}
}

func TestResultSinkConformanceRejectsInvalidImplementation(t *testing.T) {
	violations := CheckResultSink(func() contract4competios.ResultSink {
		return invalidResultSink{}
	})
	if len(violations) < 2 {
		t.Fatalf("invalid sink produced %d violations, want at least 2: %v", len(violations), violations)
	}
}

func TestContestStartSinkConformanceRejectsUnsafeImplementation(t *testing.T) {
	violations := CheckContestStartSink(func() contract4competios.ContestStartSink {
		return invalidContestStartSink{}
	})
	if len(violations) < 2 {
		t.Fatalf("invalid contest start sink produced %d violations, want at least 2: %v", len(violations), violations)
	}
}
