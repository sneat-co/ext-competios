package grants

import (
	"context"
	"sync"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

// MemoryReplayStore is an in-process ReplayStore for tests and single-
// revision dev use. It is NOT durable across a process restart -- a
// production deployment must use DalgoReplayStore instead.
type MemoryReplayStore struct {
	mu      sync.Mutex
	seen    map[string]time.Time // jti -> expiresAt
	nowFunc func() time.Time
}

// NewMemoryReplayStore returns a ready-to-use in-memory store. now may be
// nil (defaults to time.Now().UTC()); tests pass a deterministic clock.
func NewMemoryReplayStore(now func() time.Time) *MemoryReplayStore {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &MemoryReplayStore{seen: make(map[string]time.Time), nowFunc: now}
}

// Seen implements ReplayStore. It also sweeps any previously recorded jti
// whose expiry has already passed: since a Verifier only ever calls Seen
// after confirming the token is still within its time window, an entry
// surviving sweep can never legitimately reappear once expired -- there is
// no unexpired token that could carry that jti again.
func (s *MemoryReplayStore) Seen(_ context.Context, jti string, expiresAt time.Time) error {
	if s == nil {
		return contract4competios.ErrInvalidGrant
	}
	if jti == "" {
		return contract4competios.ErrInvalidGrant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowFunc()
	s.sweepLocked(now)
	if _, exists := s.seen[jti]; exists {
		return contract4competios.ErrTokenReplayConflict
	}
	s.seen[jti] = expiresAt
	return nil
}

func (s *MemoryReplayStore) sweepLocked(now time.Time) {
	for jti, expiresAt := range s.seen {
		if !expiresAt.After(now) {
			delete(s.seen, jti)
		}
	}
}

var _ ReplayStore = (*MemoryReplayStore)(nil)
