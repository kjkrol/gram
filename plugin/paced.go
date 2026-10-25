package plugin

import "time"

// FrameTimes is how long a frame took from the last one's start, and its steps: the wait for the
// GPU's next image, the simulation (the ticks and the publishing), the barrier, the drawing on
// the CPU and the presenting.
type FrameTimes struct {
	Frame, Wait, Simulate, Barrier, Draw, Present time.Duration
}

// Paced is a tracked value paced on the frame's barrier, where nothing ticks or draws: told the
// last frame's times, it may draw what follows the frame budget coarser or finer.
type Paced interface {
	Pace(t FrameTimes)
}
