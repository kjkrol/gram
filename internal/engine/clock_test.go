package engine

import (
	"testing"
	"time"
)

func TestTracker_AFrameFarBehindRunsAtMostMaxStepsAndDropsTheRest(t *testing.T) {
	const step = time.Second / 120
	tr := newTracker()
	tr.lastUpdate = time.Now().Add(-time.Second)

	if steps := tr.calculateSteps(step, 5); steps != 5 {
		t.Fatalf("a frame a second behind ran %d steps, want the cap of 5", steps)
	}
	if steps := tr.calculateSteps(step, 5); steps > 1 {
		t.Errorf("the next frame ran %d steps, want the dropped second not to be caught up", steps)
	}
}
