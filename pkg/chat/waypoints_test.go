package chat

import (
	"testing"
)

func TestParseWaypoint(t *testing.T) {
	input := "xaero-waypoint:end portal nether portal:E:159:70:139:0:false:0:Internal-the-nether"
	w, err := ParseWaypoint(input)
	if err != nil {
		t.Fatalf("failed to parse waypoint: %v", err)
	}

	if w.Name != "end portal nether portal" {
		t.Errorf("expected name %q, got %q", "end portal nether portal", w.Name)
	}
	if w.Symbol != "E" {
		t.Errorf("expected symbol %q, got %q", "E", w.Symbol)
	}
	if w.X != 159 || w.Y != 70 || w.Z != 139 {
		t.Errorf("expected pos (159, 70, 139), got (%d, %d, %d)", w.X, w.Y, w.Z)
	}
	if w.Dimension != "Internal-the-nether" {
		t.Errorf("expected dimension %q, got %q", "Internal-the-nether", w.Dimension)
	}
}

func TestParseWaypointWithColons(t *testing.T) {
	input := "xaero-waypoint:base:with:colons:B:100:64:-250:2:false:0:Internal-overworld"
	w, err := ParseWaypoint(input)
	if err != nil {
		t.Fatalf("failed to parse waypoint: %v", err)
	}

	if w.Name != "base:with:colons" {
		t.Errorf("expected name %q, got %q", "base:with:colons", w.Name)
	}
	if w.Symbol != "B" {
		t.Errorf("expected symbol %q, got %q", "B", w.Symbol)
	}
	if w.X != 100 {
		t.Errorf("expected X 100, got %d", w.X)
	}
}

func TestFindWaypoint(t *testing.T) {
	input := "2026/06/12 11:06:50 [CHAT] halocem3 ≫ xaero-waypoint:test:T:10:20:30:1:true:0:Internal-overworld"
	w, err := FindWaypoint(input)
	if err != nil {
		t.Fatalf("failed to find waypoint: %v", err)
	}

	if w.Name != "test" {
		t.Errorf("expected name %q, got %q", "test", w.Name)
	}
	if w.X != 10 {
		t.Errorf("expected X 10, got %d", w.X)
	}
}

func TestFindWaypointUserReported(t *testing.T) {
	// The user reported this didn't work. Let's see if it's the ≫ character or something else.
	input := "2026/06/12 11:25:19 [CHAT] frenki1234 ≫ xaero-waypoint:spawn:S:0:310:0:15:false:0:Internal-overworld"
	w, err := FindWaypoint(input)
	if err != nil {
		t.Fatalf("failed to find waypoint: %v", err)
	}

	if w.Name != "spawn" {
		t.Errorf("expected name %q, got %q", "spawn", w.Name)
	}
	if w.X != 0 || w.Y != 310 || w.Z != 0 {
		t.Errorf("expected pos (0, 310, 0), got (%d, %d, %d)", w.X, w.Y, w.Z)
	}
}
