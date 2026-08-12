package grants

import (
	"context"
	"errors"
	"time"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"
	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

// dalgoReplayCollection is the flat DAL collection every direction's replay
// records share, keyed by jti. One collection is sufficient: jti is a random
// 128-bit value (see newTokenID), so directions and services never collide.
const dalgoReplayCollection = "competiosGrantReplay"

// dalgoReplayRecord is the durable record for one consumed jti. ExpiresAt
// mirrors the token's own expiry, so an operator's periodic cleanup (or a
// backend-native TTL policy on this collection, where the store supports
// one) can safely reclaim the record once no unexpired token could still
// carry that jti.
type dalgoReplayRecord struct {
	ExpiresAt time.Time `json:"expiresAt" firestore:"expiresAt"`
}

// ErrReplayStoreMisconfigured is returned by NewDalgoReplayStore when db is
// nil.
var ErrReplayStoreMisconfigured = errors.New("grants: replay store is misconfigured")

// dalgoReplayTransactionAttempts bounds retries for the check-then-insert
// transaction below a transient contention error.
const dalgoReplayTransactionAttempts = 3

// DalgoReplayStore is the durable ReplayStore for production deployments. It
// survives a process/revision restart, unlike MemoryReplayStore.
type DalgoReplayStore struct {
	db  dal.DB
	now func() time.Time
}

// NewDalgoReplayStore returns a ready-to-use durable store over db. now may
// be nil (defaults to time.Now().UTC()).
func NewDalgoReplayStore(db dal.DB, now func() time.Time) (*DalgoReplayStore, error) {
	if db == nil {
		return nil, ErrReplayStoreMisconfigured
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &DalgoReplayStore{db: db, now: now}, nil
}

// Seen implements ReplayStore with a serializable check-then-insert
// transaction: Get the jti record; if it already exists, the token is a
// replay; otherwise Set it and succeed. Isolation-level serializability is
// what makes this safe under concurrent verification of the same token.
func (s *DalgoReplayStore) Seen(ctx context.Context, jti string, expiresAt time.Time) error {
	if s == nil || s.db == nil {
		return contract4competios.ErrInvalidGrant
	}
	if jti == "" {
		return contract4competios.ErrInvalidGrant
	}
	key := record.NewKeyWithID(dalgoReplayCollection, jti)
	return s.db.RunReadwriteTransaction(ctx, func(txCtx context.Context, tx dal.ReadwriteTransaction) error {
		entry := record.NewDataWithID(jti, key, new(dalgoReplayRecord))
		if err := tx.Get(txCtx, entry.Record); err == nil {
			return contract4competios.ErrTokenReplayConflict
		} else if !record.IsNotFound(err) {
			return err
		}
		*entry.Data = dalgoReplayRecord{ExpiresAt: expiresAt}
		return tx.Set(txCtx, entry.Record)
	}, dal.TxWithIsolationLevel(dal.TxSerializable), dal.TxWithAttempts(dalgoReplayTransactionAttempts))
}

var _ ReplayStore = (*DalgoReplayStore)(nil)
