package route

import "fmt"

// Check validates that the run owns a route and that the position does not
// regress, without publishing any progress. Callers must only commit the
// position through Advance after every tracking validation has passed.
func (c *Coordinator) Check(runID string, position float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.check(runID, position)
}

func (c *Coordinator) check(runID string, position float64) error {
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.check(runID, position); err != nil {
		return err
	}
	c.progress[runID] = position
	return nil
}

// RevertProgress restores a previously observed progress value. It is the
// compensating action used when publishing a tracking point fails after
// Advance already committed.
func (c *Coordinator) RevertProgress(runID string, position float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, lease := range c.owners {
		if lease.RunID == runID {
			c.progress[runID] = position
			return
		}
	}
}
