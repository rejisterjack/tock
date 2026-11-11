package httpapi

import (
	"sync"
	"time"
)

// failWindow counts recent failures for one key, used to slow password guessing.
type failWindow struct {
	mu sync.Mutex
	at map[string][]time.Time
}

func (f *failWindow) tooMany(key string, now time.Time) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.prune(key, now)) >= loginFailureLimit
}

func (f *failWindow) add(key string, now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.prune(key, now)
	if len(f.at) >= loginFailureMapSize {
		for k := range f.at {
			delete(f.at, k)
			break
		}
	}
	f.at[key] = append(kept, now)
}

func (f *failWindow) reset(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.at, key)
}

// prune drops expired timestamps. The caller holds f.mu.
func (f *failWindow) prune(key string, now time.Time) []time.Time {
	if f.at == nil {
		f.at = map[string][]time.Time{}
	}
	cut := now.Add(-loginFailureWindow)
	prev := f.at[key]
	kept := make([]time.Time, 0, len(prev))
	for _, t := range prev {
		if !t.Before(cut) {
			kept = append(kept, t)
		}
	}
	f.at[key] = kept
	return kept
}
