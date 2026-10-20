package cameras

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

// a second a step from the start, fps frames a second each
type ticking struct {
	a   *adapter
	now time.Time
}

func newTicking() *ticking {
	return &ticking{a: &adapter{on: true, budget: time.Second / 60, sampling: DefaultSampling,
		underAt: DefaultUnder, metAt: DefaultMet},
		now: time.Unix(1000, 0)}
}

func (k *ticking) steps(fps float64, n int) (factors []float32) {
	for range n {
		k.now = k.now.Add(time.Second)
		factors = append(factors, k.a.step(k.now, fps))
	}
	return factors
}

// the step want at at, none elsewhere
func only(factors []float32, want float32, at int) bool {
	for i, f := range factors {
		if i == at && f != want || i != at && f != 1 {
			return false
		}
	}
	return true
}

func none(factors []float32) bool { return only(factors, 1, -1) }

func TestAdapter_AWindowUnderTheBudgetStepsCoarser(t *testing.T) {
	k := newTicking()
	if got := k.steps(50, 3); !only(got, ResolutionStep, 1) {
		t.Errorf("at 50 of 60 frames the steps are %v, want the coarser one as the window ends", got)
	}
	k = newTicking()
	if got := k.steps(50, 1); got[0] != 1 {
		t.Fatalf("one second under the budget stepped by %v, want nothing: the window runs", got[0])
	}
	if got := k.steps(62, 1); got[0] != 1 {
		t.Errorf("a second well over the budget after one under stepped by %v, want nothing: the "+
			"window's rate meets it", got[0])
	}
	k.a.sampling = 4 * time.Second
	if got := k.steps(50, 4); !only(got, ResolutionStep, 3) {
		t.Errorf("sampled over four seconds the steps are %v, want the coarser one at the fourth",
			got)
	}
}

func TestAdapter_TwoWindowsMeetingTheBudgetStepFinerTentatively(t *testing.T) {
	k := newTicking()
	if got := k.steps(60, 5); !only(got, 1/ResolutionStep, 3) {
		t.Errorf("at 60 frames the steps are %v, want the finer one at the fourth", got)
	}
	k = newTicking()
	k.steps(60, 2)
	if got := k.steps(57, 2); !none(got) {
		t.Fatalf("a window between stepped by %v, want nothing", got)
	}
	if got := k.steps(60, 2); !none(got) {
		t.Errorf("a window meeting the budget after one between stepped %v, want nothing: "+
			"the count began anew", got)
	}
}

func TestAdapter_AFinerStepCostingTheBudgetBurnsTheFinerStepsForAWhile(t *testing.T) {
	k := newTicking()
	k.steps(60, 4)
	if got := k.steps(50, 2); got[1] != ResolutionStep {
		t.Fatalf("under the budget right after a finer step the steps are %v, want coarser", got)
	}
	if got := k.steps(60, 30); !none(got) {
		t.Errorf("meeting the budget within the burn stepped %v, want nothing for thirty seconds",
			got)
	}
	if got := k.steps(60, 2); got[1] != 1/ResolutionStep {
		t.Errorf("the burn over, the steps are %v, want finer as the window ends", got)
	}
}

func TestAdapter_SamplesOnceASecondAndNeverHeadlessOrOff(t *testing.T) {
	k := newTicking()
	for range 10 {
		k.now = k.now.Add(100 * time.Millisecond)
		k.a.step(k.now, 50)
	}
	if k.a.samples != 1 {
		t.Errorf("ten passes within a second took %d samples, want 1", k.a.samples)
	}
	if got := k.steps(0, 5); !none(got) || k.a.samples != 1 {
		t.Errorf("without frames the steps are %v and the samples %d, want nothing and 1 still",
			got, k.a.samples)
	}
	k.a.on = false
	if got := k.steps(50, 5); !none(got) {
		t.Errorf("switched off the steps are %v, want nothing", got)
	}
}

func TestClamped_StaysBetweenTheLeastResolutionAndAllThePixels(t *testing.T) {
	for in, want := range map[float32]float32{0.1: LeastResolution, 0.7: 0.7, 1.3: 1} {
		if got := clamped(in); got != want {
			t.Errorf("clamped(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestStepLayers_StepsTheOutermostRingCoarserFirstAndTheInnermostFinerFirst(t *testing.T) {
	near := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }
	for _, c := range []struct {
		name   string
		from   [3]float32
		factor float32
		steps  [][3]float32
	}{
		{"coarser from the preset", [3]float32{0.6, 0.8, 1}, ResolutionStep, [][3]float32{
			{0.51, 0.8, 1}, {0.4335, 0.8, 1}, {0.4, 0.8, 1}, {0.4, 0.68, 1},
		}},
		{"finer from the floors", [3]float32{0.4, 0.6, 0.7}, 1 / ResolutionStep, [][3]float32{
			{0.4, 0.6, 0.8235}, {0.4, 0.6, 0.9689}, {0.4, 0.6, 1}, {0.4, 0.7059, 1},
		}},
	} {
		cam := cameratest.NewFromSpace(1000, 800, 0).(camera.Layered)
		shares := [3]float32{1, 0.6, 0.35}
		layers := make([]camera.Layer, 3)
		for i, s := range c.from {
			layers[i] = camera.Layer{Share: shares[i], Resolution: s}
		}
		cam.SetLayers(layers, camera.Inset)
		for n, want := range c.steps {
			centre := stepLayers(cam, c.factor)
			var got [3]float32
			for i, layer := range cam.Layers() {
				got[i] = layer.Resolution
			}
			for i := range want {
				if !near(got[i], want[i]) {
					t.Errorf("%s, step %d: the layers are %.4v, want %.4v", c.name, n+1, got, want)
					break
				}
			}
			if centre != got[2] {
				t.Errorf("%s, step %d: stepLayers gave %v, want the centre's %v", c.name, n+1,
					centre, got[2])
			}
		}
	}
}
