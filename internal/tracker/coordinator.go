package tracker

import (
	"github.com/wyw14/solo-036/internal/model"
	"sort"
	"time"
)

func (c *Coordinator) Observe(runID, station, source string, position float64, at time.Time) model.PassageEvidence {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation[runID] == 0 {
		c.generation[runID] = 1
	}
	e := model.PassageEvidence{RunID: runID, Station: station, SourceID: source, Generation: c.generation[runID], Position: position, At: at, Valid: true}
	key := runID + "/" + station
	c.points[key] = append(c.points[key], e)
	if c.sources[key] == nil {
		c.sources[key] = make(map[string]bool)
	}
	c.sources[key][source] = true
	return e
}

// Retract removes an evidence point previously stored by Observe so that a
// tracking point rejected by a later validation never remains in the source
// records. Only an exact match is removed; the source index is rebuilt to
// drop the source when no remaining point references it.
func (c *Coordinator) Retract(e model.PassageEvidence) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := e.RunID + "/" + e.Station
	points := c.points[key]
	for i := len(points) - 1; i >= 0; i-- {
		if points[i] == e {
			c.points[key] = append(points[:i], points[i+1:]...)
			break
		}
	}
	if len(c.points[key]) == 0 {
		delete(c.points, key)
	}
	set, ok := c.sources[key]
	if !ok {
		return
	}
	for _, remaining := range c.points[key] {
		if remaining.SourceID == e.SourceID {
			return
		}
	}
	delete(set, e.SourceID)
	if len(set) == 0 {
		delete(c.sources, key)
	}
}
func (c *Coordinator) Revise(runID string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation[runID]++
	for key := range c.points {
		if len(key) >= len(runID) && key[:len(runID)] == runID {
			delete(c.points, key)
			delete(c.sources, key)
		}
	}
	return c.generation[runID]
}
func (c *Coordinator) Generation(runID string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation[runID] == 0 {
		c.generation[runID] = 1
	}
	return c.generation[runID]
}
func (c *Coordinator) Evidence(runID, station string) []model.PassageEvidence {
	c.mu.Lock()
	defer c.mu.Unlock()
	in := c.points[runID+"/"+station]
	out := append([]model.PassageEvidence(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}
