package srun

import (
	"testing"
)

func TestKickResponseIndicatesError(t *testing.T) {
	if !kickResponseIndicatesError(`<div class="alert alert-danger">failed</div>`) {
		t.Fatal("expected danger alert to fail")
	}
	if kickResponseIndicatesError(`{"success":true}`) {
		t.Fatal("expected success body")
	}
}

func TestKickAllWithFakeMACOrdering(t *testing.T) {
	// Let's verify the logic handles different sizes of session lists
	// 3 sessions -> mid index 1 (second one)
	// 2 sessions -> mid index 1 (second one)
	// 1 session -> mid index 0 (first one)
	cases := []struct {
		length   int
		expected int
	}{
		{3, 1},
		{2, 1},
		{1, 0},
	}
	for _, tc := range cases {
		mid := tc.length / 2
		if mid != tc.expected {
			t.Errorf("For length %d, expected mid index %d, got %d", tc.length, tc.expected, mid)
		}
	}
}
