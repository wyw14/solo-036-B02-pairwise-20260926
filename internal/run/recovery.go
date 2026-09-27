package run

import (
	"github.com/wyw14/solo-036/internal/model"
	"time"
)

func Restore(store *Store, records []model.Run) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, r := range records {
		if model.ValidateRun(r) {
			store.runs[r.ID] = r
		}
	}
}

// RevertTracking undoes a launched -> tracking transition performed earlier in
// the same request, when subsequent validation rejects the tracking point. It
// bypasses the forward-only transition machine and unwinds the epoch bump.
func RevertTracking(store *Store, id string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	r, ok := store.runs[id]
	if !ok || r.State != model.StateTracking {
		return
	}
	r.State = model.StateLaunched
	if r.Epoch > 1 {
		r.Epoch--
	}
	r.UpdatedAt = time.Now().UTC()
	store.runs[id] = r
}
func Active(store *Store) []model.Run {
	records := store.Snapshot()
	out := make([]model.Run, 0, len(records))
	for _, r := range records {
		if !r.State.Terminal() {
			out = append(out, r)
		}
	}
	return out
}
