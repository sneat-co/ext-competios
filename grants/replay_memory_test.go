package grants

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

func TestMemoryReplayStoreFirstSeenSucceeds(t *testing.T) {
	store := NewMemoryReplayStore(nil)
	if err := store.Seen(context.Background(), "jti-1", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryReplayStoreSecondSeenConflicts(t *testing.T) {
	store := NewMemoryReplayStore(nil)
	ctx := context.Background()
	expires := time.Now().Add(time.Minute)
	if err := store.Seen(ctx, "jti-1", expires); err != nil {
		t.Fatalf("first Seen: %v", err)
	}
	if err := store.Seen(ctx, "jti-1", expires); !errors.Is(err, contract4competios.ErrTokenReplayConflict) {
		t.Fatalf("second Seen err = %v, want ErrTokenReplayConflict", err)
	}
}

func TestMemoryReplayStoreRejectsEmptyJTI(t *testing.T) {
	store := NewMemoryReplayStore(nil)
	if err := store.Seen(context.Background(), "", time.Now().Add(time.Minute)); err == nil {
		t.Fatalf("expected an error for an empty jti")
	}
}

func TestMemoryReplayStoreSweepsExpiredEntries(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	store := NewMemoryReplayStore(clock)
	ctx := context.Background()
	if err := store.Seen(ctx, "jti-1", now.Add(time.Second)); err != nil {
		t.Fatalf("first Seen: %v", err)
	}
	now = now.Add(time.Hour) // advance past expiry
	if len(store.seen) != 1 {
		t.Fatalf("precondition: expected 1 stored entry, got %d", len(store.seen))
	}
	// A distinct jti's Seen call sweeps the now-expired "jti-1" entry as a
	// side effect (see MemoryReplayStore.Seen's doc comment).
	if err := store.Seen(ctx, "jti-2", now.Add(time.Minute)); err != nil {
		t.Fatalf("second Seen: %v", err)
	}
	store.mu.Lock()
	_, stillPresent := store.seen["jti-1"]
	store.mu.Unlock()
	if stillPresent {
		t.Fatalf("expired jti-1 was not swept")
	}
}

func TestNilMemoryReplayStoreIsSafe(t *testing.T) {
	var store *MemoryReplayStore
	if err := store.Seen(context.Background(), "jti", time.Now()); err == nil {
		t.Fatalf("expected an error from a nil store")
	}
}
