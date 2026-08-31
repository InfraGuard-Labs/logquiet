package pattern

import (
	"container/list"
	"time"

	"github.com/InfraGuard-Labs/logquiet/internal/fingerprint"
	"github.com/InfraGuard-Labs/logquiet/internal/severity"
)

// Store holds all currently tracked patterns, bounded to Config.MaxTrackedPatterns
// via least-recently-seen eviction. Eviction only drops a pattern's rolling
// history and counters; if the same structural pattern reappears later it is
// simply re-learned as if it were new, which is safe by construction (see
// docs/TECHNICAL_METHOD.md, "Bounded memory").
type Store struct {
	cfg     Config
	byID    map[fingerprint.ID]*State
	lru     *list.List // front = most recently used
	evicted uint64
}

// NewStore creates an empty Store using cfg.
func NewStore(cfg Config) *Store {
	return &Store{
		cfg:  cfg,
		byID: make(map[fingerprint.ID]*State),
		lru:  list.New(),
	}
}

// GetOrCreate returns the tracked State for fp, creating it (and recording
// example/template/severity) on first sight. isNew reports whether this
// fingerprint had never been observed by this store before - including
// having been evicted and now reappearing, which is treated identically to
// genuine novelty since its history was discarded.
func (st *Store) GetOrCreate(fp fingerprint.ID, tmpl string, lvl severity.Level, example []string, now time.Time) (state *State, isNew bool) {
	if s, ok := st.byID[fp]; ok {
		st.lru.MoveToFront(s.elem)
		return s, false
	}

	s := newState(st.cfg, fp, tmpl, lvl, example, now)
	s.elem = st.lru.PushFront(fp)
	st.byID[fp] = s

	if len(st.byID) > st.cfg.MaxTrackedPatterns {
		st.evictOldest()
	}
	return s, true
}

func (st *Store) evictOldest() {
	back := st.lru.Back()
	if back == nil {
		return
	}
	fp := back.Value.(fingerprint.ID)
	st.lru.Remove(back)
	delete(st.byID, fp)
	st.evicted++
}

// Len returns the number of distinct patterns currently tracked.
func (st *Store) Len() int { return len(st.byID) }

// Evicted returns the number of patterns dropped to stay within
// MaxTrackedPatterns since the store was created.
func (st *Store) Evicted() uint64 { return st.evicted }
