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
func (c *Coordinator) Remove(runID, station string, target model.PassageEvidence) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := runID + "/" + station
	points := c.points[key]
	for i, e := range points {
		if e == target {
			c.points[key] = append(points[:i], points[i+1:]...)
			break
		}
	}
	sources := make(map[string]bool)
	for _, e := range c.points[key] {
		sources[e.SourceID] = true
	}
	if len(sources) == 0 {
		delete(c.sources, key)
	} else {
		c.sources[key] = sources
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
