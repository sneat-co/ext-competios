package grants

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dal-go/dalgo/adapters/dalgo2memory"
	"github.com/dal-go/dalgo/dal"
	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

// newMemoryDB builds this module's in-memory dal.DB test double, naming
// Firestore as the backend it emulates (dal-go/dalgo v0.74.0 made the
// emulated backend an explicit, compile-enforced parameter of
// dalgo2memory.New instead of an implicit default). This module does not
// call sneat-co/sneat-go-core's sneatcoretesting.NewInMemoryTestDB, the
// fleet's usual single place for that choice: ext-competios is an
// extension repo, and grants may depend only on *-contract libraries plus
// the storage/crypto primitives (dalgo, jwt) its purpose-separation logic
// actually needs -- adding a dependency on sneat-go-core, a non-contract
// implementation module, just to reuse a one-line test helper would violate
// that boundary.
func newMemoryDB() dal.DB {
	return dalgo2memory.New(dalgo2memory.FirestoreProfile())
}

func TestNewDalgoReplayStoreRejectsNilDB(t *testing.T) {
	if _, err := NewDalgoReplayStore(nil, nil); !errors.Is(err, ErrReplayStoreMisconfigured) {
		t.Fatalf("err = %v, want ErrReplayStoreMisconfigured", err)
	}
}

func TestDalgoReplayStoreFirstSeenSucceeds(t *testing.T) {
	store, err := NewDalgoReplayStore(newMemoryDB(), nil)
	if err != nil {
		t.Fatalf("NewDalgoReplayStore: %v", err)
	}
	if err := store.Seen(context.Background(), "jti-1", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDalgoReplayStoreSecondSeenConflicts(t *testing.T) {
	store, err := NewDalgoReplayStore(newMemoryDB(), nil)
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
	store, err := NewDalgoReplayStore(newMemoryDB(), nil)
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
	store, err := NewDalgoReplayStore(newMemoryDB(), nil)
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
	db := newMemoryDB()
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
