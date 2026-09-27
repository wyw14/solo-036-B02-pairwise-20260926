package service

import (
	"context"
	"testing"
	"time"

	"github.com/wyw14/solo-036/internal/model"
	"github.com/wyw14/solo-036/internal/tracker"
)

func launchRun(t *testing.T, rt *Runtime) string {
	t.Helper()
	op, err := rt.Launch(context.Background(), LaunchRequest{
		StationID:      "station-a",
		RouteID:        "main-station-a-station-c",
		TargetPressure: 8,
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	return op.Run.ID
}

func validTrackRequest(position float64) TrackingRequest {
	return TrackingRequest{
		StationID:  "station-b",
		SourceID:   "sensor-1",
		Kind:       tracker.SourceMagnetic,
		Position:   position,
		ObservedAt: time.Now().UTC(),
	}
}

// A position that route progress rejects must not publish any partial state:
// the run stays launched, and no point reaches the tracker or lineage.
func TestTrackRejectsIllegalPositionBeforePublishing(t *testing.T) {
	rt := NewRuntime()
	runID := launchRun(t, rt)

	_, err := rt.Track(runID, validTrackRequest(-5))
	if err == nil {
		t.Fatal("expected regressed position to be rejected")
	}

	current, ok := rt.Runs.Get(runID)
	if !ok {
		t.Fatalf("run %s missing", runID)
	}
	if current.State != model.StateLaunched {
		t.Fatalf("run state = %s, want launched", current.State)
	}
	if points := rt.Tracker.Evidence(runID, "station-b"); len(points) != 0 {
		t.Fatalf("tracker holds %d rejected points", len(points))
	}
	if obs := rt.Lineage.Observations(runID, "station-b", 1); len(obs) != 0 {
		t.Fatalf("lineage holds %d rejected observations", len(obs))
	}
	if progress := rt.Routes.Position(runID); progress != 0 {
		t.Fatalf("route progress = %v, want 0", progress)
	}
}

// Once a run is tracking, a later point that regresses must not enter any
// store, while the previously accepted point remains intact.
func TestTrackRejectsRegressionAfterValidPoint(t *testing.T) {
	rt := NewRuntime()
	runID := launchRun(t, rt)

	if _, err := rt.Track(runID, validTrackRequest(10)); err != nil {
		t.Fatalf("valid track: %v", err)
	}
	if _, err := rt.Track(runID, validTrackRequest(5)); err == nil {
		t.Fatal("expected regressed position to be rejected")
	}

	current, _ := rt.Runs.Get(runID)
	if current.State != model.StateTracking {
		t.Fatalf("run state = %s, want tracking", current.State)
	}
	points := rt.Tracker.Evidence(runID, "station-b")
	if len(points) != 1 || points[0].Position != 10 {
		t.Fatalf("tracker points = %v, want only position 10", points)
	}
	if obs := rt.Lineage.Observations(runID, "station-b", 1); len(obs) != 1 || obs[0].Position != 10 {
		t.Fatalf("lineage observations = %v, want only position 10", obs)
	}
	if progress := rt.Routes.Position(runID); progress != 10 {
		t.Fatalf("route progress = %v, want 10", progress)
	}
	snapshot := rt.Snapshot()
	if len(snapshot.Tracking) != 1 {
		t.Fatalf("published tracking states = %d, want 1", len(snapshot.Tracking))
	}
}

// A failure in a later validation (invalid source kind) must roll back the run
// transition and the staged tracker point, leaving no partial state.
func TestTrackRollsBackWhenLineageRejects(t *testing.T) {
	rt := NewRuntime()
	runID := launchRun(t, rt)

	req := validTrackRequest(10)
	req.Kind = tracker.SourceKind("bogus")
	if _, err := rt.Track(runID, req); err == nil {
		t.Fatal("expected invalid source kind to be rejected")
	}

	current, _ := rt.Runs.Get(runID)
	if current.State != model.StateLaunched {
		t.Fatalf("run state = %s, want launched", current.State)
	}
	if points := rt.Tracker.Evidence(runID, "station-b"); len(points) != 0 {
		t.Fatalf("tracker holds %d points after rollback", len(points))
	}
	if obs := rt.Lineage.Observations(runID, "station-b", 1); len(obs) != 0 {
		t.Fatalf("lineage holds %d observations after rollback", len(obs))
	}
}

// Legal points still advance the run into tracking and move route progress.
func TestTrackValidPointAdvancesRunAndPosition(t *testing.T) {
	rt := NewRuntime()
	runID := launchRun(t, rt)

	if _, err := rt.Track(runID, validTrackRequest(10)); err != nil {
		t.Fatalf("first track: %v", err)
	}
	if _, err := rt.Track(runID, validTrackRequest(42)); err != nil {
		t.Fatalf("second track: %v", err)
	}

	current, _ := rt.Runs.Get(runID)
	if current.State != model.StateTracking {
		t.Fatalf("run state = %s, want tracking", current.State)
	}
	if points := rt.Tracker.Evidence(runID, "station-b"); len(points) != 2 {
		t.Fatalf("tracker points = %d, want 2", len(points))
	}
	if obs := rt.Lineage.Observations(runID, "station-b", 1); len(obs) != 2 {
		t.Fatalf("lineage observations = %d, want 2", len(obs))
	}
	if progress := rt.Routes.Position(runID); progress != 42 {
		t.Fatalf("route progress = %v, want 42", progress)
	}
}
