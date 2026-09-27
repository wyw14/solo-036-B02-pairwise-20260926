package route

import "fmt"

// CanAdvance reports whether advancing the run to position would be accepted,
// without changing route progress. It lets callers reject illegal positions
// before publishing any tracking state.
func (c *Coordinator) CanAdvance(runID string, position float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, lease := range c.owners {
		if lease.RunID == runID {
			if position < c.progress[runID] {
				return fmt.Errorf("position regressed")
			}
			return nil
		}
	}
	return fmt.Errorf("run has no route")
}

func (c *Coordinator) Advance(runID string, position float64) error {
	if err := c.CanAdvance(runID, position); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.progress[runID] = position
	return nil
}

// RestoreProgress returns a run's route progress to a prior value while the
// route is still leased to it. It is used to undo Advance when a later
// tracking write fails.
func (c *Coordinator) RestoreProgress(runID string, position float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, lease := range c.owners {
		if lease.RunID == runID {
			c.progress[runID] = position
			return
		}
	}
}
