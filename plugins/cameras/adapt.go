package cameras

import (
	"slices"
	"time"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
)

// adapter steps the pictures' resolution after the frame rate, sampled once a second and
// averaged over the sampling window (Settings.Sampling): coarser once a window's rate has fallen
// under the budget, finer, tentatively, once two windows running have met it. The vsync caps
// the rate at the screen's, so a budget met says nothing of the room above it: a finer step is
// tried, and one that costs the budget within ten seconds burns the trying for thirty.
type adapter struct {
	on       bool
	budget   time.Duration
	sampling time.Duration
	underAt  float64 // of the budget's rate: under it the picture goes coarser
	metAt    float64 // over it, finer
	sum      float64 // the window's samples so far, summed
	samples  int
	met      int       // windows running that met the budget
	last     time.Time // the last sample
	finer    time.Time // the last finer step
	burnt    time.Time // no finer step before
	// the frames paced since the last sample, and the window's time the drawing's side and the
	// simulation took
	frames             int
	counted            time.Time
	drawing, simulated time.Duration
}

// Sampling the frame rate is averaged over before the pictures' resolution follows it
// (Settings.Sampling): the least, the most and the one the cameras begin with.
const (
	LeastSampling   = time.Second
	MostSampling    = 20 * time.Second
	DefaultSampling = 2 * time.Second
)

// DefaultUnder and DefaultMet are the shares of the budget's rate the cameras begin with: under
// the first the picture goes coarser, over the second finer (Settings.Under, Settings.Met).
const (
	DefaultUnder = 0.92
	DefaultMet   = 0.98
)

const (
	sampleEvery = time.Second
	// a finer step costing the budget within burnFast stops the finer steps for burnFor
	burnFast = 10 * time.Second
	burnFor  = 30 * time.Second
	// windows running meeting the budget before a finer step
	metWindows = 2
)

// step is the factor to draw the pictures' resolution by at now, 1 for none, after fps frames a
// second; 0 frames (headless) is no sample.
func (a *adapter) step(now time.Time, fps float64) float32 {
	if !a.on || fps == 0 || now.Sub(a.last) < sampleEvery {
		return 1
	}
	a.last = now
	a.sum += fps
	a.samples++
	if time.Duration(a.samples)*sampleEvery < max(a.sampling, LeastSampling) {
		return 1
	}
	rate := a.sum / float64(a.samples)
	a.sum, a.samples = 0, 0
	want := float64(time.Second) / float64(a.budget)
	switch {
	case rate < a.underAt*want:
		a.met = 0
		if now.Sub(a.finer) < burnFast {
			a.burnt = now.Add(burnFor)
		}
		return ResolutionStep
	case rate >= a.metAt*want:
		a.met = min(a.met+1, metWindows)
		if a.met < metWindows || !now.After(a.burnt) {
			return 1
		}
		a.met, a.finer = 0, now
		return 1 / ResolutionStep
	}
	a.met = 0
	return 1
}

// pace counts the frame of times t, and steps as step does after the frames counted a second
// apiece; coarser only while over the window the drawing's side, the drawing, its presenting and
// the wait for the GPU, took longer than the simulation, which no lever lightens
func (a *adapter) pace(now time.Time, t plugin.FrameTimes) float32 {
	a.frames++
	a.drawing += t.Draw + t.Present + t.Wait
	a.simulated += t.Simulate
	if a.counted.IsZero() {
		a.counted = now
	}
	elapsed := now.Sub(a.counted)
	if elapsed < sampleEvery {
		return 1
	}
	fps := float64(a.frames) / elapsed.Seconds()
	a.frames, a.counted = 0, now
	f := a.step(now, fps)
	if a.samples > 0 {
		return f // the window goes on
	}
	if f < 1 && a.drawing <= a.simulated {
		f = 1
	}
	a.drawing, a.simulated = 0, 0
	return f
}

// clamped is s between LeastResolution and all the pixels
func clamped(s float32) float32 { return min(max(s, LeastResolution), 1) }

// the least each layer is stepped down to, the outermost first
var layerFloors = []float32{LeastResolution, 0.6, 0.7}

// stepLayers steps one of c's layers by factor, coarser the outermost still over its floor, finer
// the innermost still under all its pixels, and is the centre's resolution after
func stepLayers(c camera.Layered, factor float32) float32 {
	layers := c.Layers()
	switch {
	case factor < 1:
		for i, r := range layers {
			floor := float32(LeastResolution)
			if i < len(layerFloors) {
				floor = layerFloors[i]
			}
			if r.Resolution > floor {
				layers[i].Resolution = max(r.Resolution*factor, floor)
				break
			}
		}
	case factor > 1:
		for i := len(layers) - 1; i >= 0; i-- {
			if r := layers[i].Resolution; r < 1 {
				layers[i].Resolution = min(r*factor, 1)
				break
			}
		}
	}
	c.SetLayers(layers, c.Layout())
	return layers[len(layers)-1].Resolution
}

// resolutionsOf is the resolutions cam is drawn at: its own and its layers'
func resolutionsOf(cam camera.Camera) []float32 {
	out := []float32{camera.ResolutionOf(cam)}
	for _, l := range camera.LayersOf(cam) {
		out = append(out, l.Resolution)
	}
	return out
}

// sameResolutions reports whether a and b are the same resolutions
func sameResolutions(a, b []float32) bool { return slices.Equal(a, b) }
