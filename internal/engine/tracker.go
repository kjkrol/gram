package engine

import "time"

type tracker struct {
	lastUpdate    time.Time
	lastTPSUpdate time.Time
	accumulator   time.Duration
}

func newTracker() *tracker {
	now := time.Now()
	return &tracker{
		lastUpdate:    now,
		lastTPSUpdate: now,
	}
}

func (t *tracker) calculateSteps(physicsStep time.Duration, maxSteps int) int {
	now := time.Now()

	if t.lastUpdate.IsZero() {
		t.lastUpdate = now
		t.lastTPSUpdate = now
	}

	elapsed := now.Sub(t.lastUpdate)
	t.lastUpdate = now
	t.accumulator += elapsed

	steps := 0
	for t.accumulator >= physicsStep && steps < maxSteps {
		t.accumulator -= physicsStep
		steps++
	}

	if t.accumulator > physicsStep {
		t.accumulator = 0
	}

	return steps
}

// restart has the steps counted from now, the time before not the world's
func (t *tracker) restart() { t.lastUpdate, t.accumulator = time.Now(), 0 }

func (t *tracker) processStatsInterval() bool {
	duration := time.Since(t.lastTPSUpdate)
	if duration > 2*time.Second {
		t.lastTPSUpdate = time.Now()
		return true
	}
	if duration >= time.Second {
		t.lastTPSUpdate = t.lastTPSUpdate.Add(time.Second)
		return true
	}
	return false
}
