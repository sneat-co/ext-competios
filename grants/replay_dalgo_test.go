package grants

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dal-go/dalgo/adapters/dalgo2memory"
	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

func TestNewDalgoReplayStoreRejectsNilDB(t *testing.T) {
	if _, err := NewDalgoReplayStore(nil, nil); !errors.Is(err, ErrReplayStoreMisconfigured) {
		t.Fatalf("err = %v, want ErrReplayStoreMisconfigured", err)
	}
}

func TestDalgoReplayStoreFirstSeenSucceeds(t *testing.T) {
	store, err := NewDalgoReplayStore(dalgo2memory.NewDB(), nil)
	if err != nil {
		t.Fatalf("NewDalgoReplayStore: %v", err)
	}
	if err := store.Seen(context.Background(), "jti-1", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDalgoReplayStoreSecondSeenConflicts(t *testing.T) {
	store, err := NewDalgoReplayStore(dalgo2memory.NewDB(), nil)
	if err != nil {
		t.Fatalf("NewDalgoReplayStore: %v", err)
	}
	ctx := context.Background()
	expires := time.Now().Add(time.Minute)
	if err := store.Seen(ctx, "jti-1", expires); err != nil {
		t.Fatalf("first Seen: %v", err)
	}
	if err := store.Seen(ctx, "jti-1", expires); !errors.Is(err, contract4competios.ErrTokenReplayConflict) {
		t.Fatalf("second Seen err = %v, want ErrTokenReplayConflict", err)
	}
}

func TestDalgoReplayStoreDistinctJTIsBothSucceed(t *testing.T) {
	store, err := NewDalgoReplayStore(dalgo2memory.NewDB(), nil)
	if err != nil {
		t.Fatalf("NewDalgoReplayStore: %v", err)
	}
	ctx := context.Background()
	expires := time.Now().Add(time.Minute)
	if err := store.Seen(ctx, "jti-a", expires); err != nil {
		t.Fatalf("jti-a: %v", err)
	}
	if err := store.Seen(ctx, "jti-b", expires); err != nil {
		t.Fatalf("jti-b: %v", err)
	}
}

func TestDalgoReplayStoreRejectsEmptyJTI(t *testing.T) {
	store, err := NewDalgoReplayStore(dalgo2memory.NewDB(), nil)
	if err != nil {
		t.Fatalf("NewDalgoReplayStore: %v", err)
	}
	if err := store.Seen(context.Background(), "", time.Now()); err == nil {
		t.Fatalf("expected an error for an empty jti")
	}
}

func TestNilDalgoReplayStoreIsSafe(t *testing.T) {
	var store *DalgoReplayStore
	if err := store.Seen(context.Background(), "jti", time.Now()); err == nil {
		t.Fatalf("expected an error from a nil store")
	}
}

// TestDalgoReplayStoreSurvivesFreshHandle proves the store's state lives in
// the DB, not in the DalgoReplayStore value itself -- constructing a SECOND
// handle over the SAME dal.DB still sees the first handle's recorded jti.
func TestDalgoReplayStoreSurvivesFreshHandle(t *testing.T) {
	db := dalgo2memory.NewDB()
	first, err := NewDalgoReplayStore(db, nil)
	if err != nil {
		t.Fatalf("NewDalgoReplayStore: %v", err)
	}
	ctx := context.Background()
	if err := first.Seen(ctx, "jti-1", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("Seen: %v", err)
	}
	second, err := NewDalgoReplayStore(db, nil)
	if err != nil {
		t.Fatalf("NewDalgoReplayStore (second handle): %v", err)
	}
	if err := second.Seen(ctx, "jti-1", time.Now().Add(time.Minute)); !errors.Is(err, contract4competios.ErrTokenReplayConflict) {
		t.Fatalf("err = %v, want ErrTokenReplayConflict", err)
	}
}
