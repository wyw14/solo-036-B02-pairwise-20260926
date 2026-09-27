package service

import (
	"context"
	"testing"
	"time"

	"github.com/wyw14/solo-036/internal/model"
	"github.com/wyw14/solo-036/internal/run"
	"github.com/wyw14/solo-036/internal/tracker"
)

func launchedRunWithoutRoute(t *testing.T, r *Runtime) string {
	t.Helper()
	created, err := r.CreateOperation("station-a")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := run.Start(r.Runs, created.ID); err != nil {
		t.Fatalf("start run: %v", err)
	}
	return created.ID
}

func trackingCount(r *Runtime, runID, station string) int {
	return len(r.Lineage.Observations(runID, station, r.Tracker.Generation(runID)))
}

// A position rejected by route progress must not move the run into tracking
// or leave the point in any source record.
func TestTrackRejectedPositionPublishesNothing(t *testing.T) {
	r := NewRuntime()
	runID := launchedRunWithoutRoute(t, r)
	station := "station-a"

	_, err := r.Track(runID, TrackingRequest{
		StationID:  station,
		SourceID:   "mag-1",
		Position:   10,
		ObservedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected track to be rejected without a leased route")
	}

	current, ok := r.Runs.Get(runID)
	if !ok {
		t.Fatal("run disappeared")
	}
	if current.State != model.StateLaunched {
		t.Fatalf("run state = %s, want launched", current.State)
	}
	if points := r.Tracker.Evidence(runID, station); len(points) != 0 {
		t.Fatalf("tracker retained %d rejected points", len(points))
	}
	if count := trackingCount(r, runID, station); count != 0 {
		t.Fatalf("lineage retained %d rejected observations", count)
	}
	if _, exists := r.ensureControl().Tracking[runID]; exists {
		t.Fatal("rejected track published a control tracking state")
	}
}

// A point rejected by a later validation (invalid source kind) must likewise
// leave no partial state behind.
func TestTrackInvalidKindPublishesNothing(t *testing.T) {
	r := NewRuntime()
	record, err := r.Launch(context.Background(), LaunchRequest{
		StationID:      "station-a",
		RouteID:        "main-station-a-station-c",
		TargetPressure: 10,
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	runID := record.Run.ID
	station := "station-a"

	before := r.Routes.Position(runID)
	_, err = r.Track(runID, TrackingRequest{
		StationID:  station,
		SourceID:   "mag-1",
		Kind:       tracker.SourceKind("bogus"),
		Position:   20,
		ObservedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected track with invalid kind to be rejected")
	}
	current, _ := r.Runs.Get(runID)
	if current.State != model.StateLaunched {
		t.Fatalf("run state = %s, want launched", current.State)
	}
	if r.Routes.Position(runID) != before {
		t.Fatalf("route progress changed to %v on rejection", r.Routes.Position(runID))
	}
	if points := r.Tracker.Evidence(runID, station); len(points) != 0 {
		t.Fatalf("tracker retained %d rejected points", len(points))
	}
	if count := trackingCount(r, runID, station); count != 0 {
		t.Fatalf("lineage retained %d rejected observations", count)
	}
}

// A position regression after a valid point must not record the rejected
// point anywhere nor move progress backwards.
func TestTrackRegressionPublishesNothing(t *testing.T) {
	r := NewRuntime()
	record, err := r.Launch(context.Background(), LaunchRequest{
		StationID:      "station-a",
		RouteID:        "main-station-a-station-c",
		TargetPressure: 10,
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	runID := record.Run.ID
	station := "station-a"

	if _, err := r.Track(runID, TrackingRequest{
		StationID: station, SourceID: "mag-1", Position: 20, ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("first track: %v", err)
	}
	if _, err := r.Track(runID, TrackingRequest{
		StationID: station, SourceID: "mag-2", Position: 10, ObservedAt: time.Now().UTC(),
	}); err == nil {
		t.Fatal("expected regression to be rejected")
	}

	current, _ := r.Runs.Get(runID)
	if current.State != model.StateTracking {
		t.Fatalf("run state = %s, want tracking", current.State)
	}
	if r.Routes.Position(runID) != 20 {
		t.Fatalf("route progress = %v, want 20", r.Routes.Position(runID))
	}
	if points := r.Tracker.Evidence(runID, station); len(points) != 1 {
		t.Fatalf("tracker has %d points, want 1", len(points))
	}
	if count := trackingCount(r, runID, station); count != 1 {
		t.Fatalf("lineage has %d observations, want 1", count)
	}
}

// A legal point advances the run into tracking, the route progress, and the
// tracking/source records.
func TestTrackValidPointAdvancesState(t *testing.T) {
	r := NewRuntime()
	record, err := r.Launch(context.Background(), LaunchRequest{
		StationID:      "station-a",
		RouteID:        "main-station-a-station-c",
		TargetPressure: 10,
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	runID := record.Run.ID
	station := "station-a"

	if _, err := r.Track(runID, TrackingRequest{
		StationID: station, SourceID: "mag-1", Position: 20, ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("track: %v", err)
	}

	current, _ := r.Runs.Get(runID)
	if current.State != model.StateTracking {
		t.Fatalf("run state = %s, want tracking", current.State)
	}
	if r.Routes.Position(runID) != 20 {
		t.Fatalf("route progress = %v, want 20", r.Routes.Position(runID))
	}
	if points := r.Tracker.Evidence(runID, station); len(points) != 1 {
		t.Fatalf("tracker has %d points, want 1", len(points))
	}
	if count := trackingCount(r, runID, station); count != 1 {
		t.Fatalf("lineage has %d observations, want 1", count)
	}
	if states := r.ensureControl().Tracking[runID]; len(states) != 1 || states[0].Position != 20 {
		t.Fatalf("control tracking states = %v, want one point at 20", states)
	}
}
