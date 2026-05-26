package srun

import "testing"

func TestKickResponseIndicatesError(t *testing.T) {
	if !kickResponseIndicatesError(`<div class="alert alert-danger">failed</div>`) {
		t.Fatal("expected danger alert to fail")
	}
	if kickResponseIndicatesError(`{"success":true}`) {
		t.Fatal("expected success body")
	}
}
