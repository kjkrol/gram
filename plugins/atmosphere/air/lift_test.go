package air_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/world"
)

func TestMass_WithLowKeepsTheHighLayer(t *testing.T) {
	for _, c := range []struct {
		m    air.Mass
		low  float32
		kept bool
	}{
		{air.Mass{Cover: 0.2, Billow: 0.5}, 0.0675, true},
		{air.Mass{Cover: 0.2, Billow: 0.5}, 0.2025, true},
		{air.Mass{Cover: 0.6, Billow: 0.3}, 0.1635, true},
		{air.Mass{Cover: 0.6, Billow: 0.3}, 0.49, true},
		{air.Mass{Cover: 0.9, Billow: 0.9}, 0.42, true},
		// more heaps than a whole sky holds with these shreds over them
		{air.Mass{Cover: 0.9, Billow: 0.9}, 0.95, false},
		{air.Mass{Cover: 0.5, Billow: 0.5}, 0, false},
	} {
		got := c.m.WithLow(c.low)
		low, high := got.Layer(air.Low).Cover, got.Layer(air.High).Cover
		want := c.m.Layer(air.High).Cover
		if math.Abs(float64(low-c.low)) > 1e-4 ||
			c.kept && math.Abs(float64(high-want)) > 5e-3 {
			t.Errorf("%+v with low clouds %v is %+v: low %v, high %v; want low %v, the high %v "+
				"kept %v", c.m, c.low, got, low, high, c.low, want, c.kept)
		}
	}
}

// the air over a world 2048 by 1024 under wind, a ridge reaching the low clouds' base across it
// from 1024 to 1408 east, the land wringing rain and gathers from the band 1536 to 1792 east
type ridged struct {
	w     air.Weather
	d     *air.Dome
	land  *air.Land
	born  air.Mass
	drift [2]float32
}

func newRidged(born air.Mass, wind [2]float32, rain, gathers float32) *ridged {
	relief := air.ReliefOf(world.Scale{}, 0, 1000)
	r := &ridged{d: air.NewDome(2048, 1024, 3000), land: air.NewLand(2048, 1024), born: born}
	r.w = air.Weather{Wind: wind, Relief: relief, Bases: air.BasesOver(world.Scale{}, relief),
		Ceilings: [3]float32{1, 0, 0}, Dome: r.d}
	r.d.Own()
	r.d.Carry(r.drift, r.mass, true)
	for x := float32(0); x < 2048; x += 32 {
		for y := float32(0); y < 1024; y += 32 {
			if x >= 1024 && x < 1408 {
				r.land.Note(x, y, 1000, 0, 0)
			} else if x >= 1536 && x < 1792 {
				r.land.Note(x, y, 0, rain, gathers)
			}
		}
	}
	r.land.Turn()
	return r
}

func (r *ridged) mass(float32, float32) (air.Mass, int16) { return r.born, 0 }

func (r *ridged) run(seconds float32) {
	const dt = 0.1
	for range int(seconds / dt) {
		r.drift[0] += r.w.Wind[0] * dt
		r.drift[1] += r.w.Wind[1] * dt
		r.d.Carry(r.drift, r.mass, false)
		r.d.Lift(r.w, r.land, func(int16) float32 { return 0 }, dt)
	}
}

func (r *ridged) low(x float32) float32 { return r.w.At(x, 512).Layer(air.Low).Cover }

func TestDome_LiftHoldsTheLowCloudsBackBeforeARidge(t *testing.T) {
	born := air.Mass{Cover: 0.3, Billow: 1, Damp: 0.5}
	r := newRidged(born, [2]float32{20, 0}, 0, 0)
	was := born.Layer(air.Low).Cover
	r.run(60)
	before := float32(0)
	for x := float32(700); x <= 1100; x += 25 {
		before = max(before, r.low(x))
	}
	if past := r.low(1600); before < 2*was || past > 0.7*was {
		t.Errorf("a wind of 20 for a minute over a ridge in the low clouds (%v): up to %v of "+
			"them before it, %v past it; want them piled up before it and clearing past it", was,
			before, past)
	}
	still := newRidged(born, [2]float32{}, 0, 0)
	still.run(40)
	for _, x := range []float32{900, 1200, 1600} {
		if got := still.low(x); math.Abs(float64(got-was)) > 1e-3 {
			t.Errorf("without wind the low clouds at %v east are %v, want them as born, %v", x,
				got, was)
		}
	}
}

func TestDome_LiftGathersDampIntoCloudsNeverOutOfNothing(t *testing.T) {
	dry := newRidged(air.Mass{Cover: 0.2, Billow: 1}, [2]float32{}, 0, 0.1)
	dry.run(10)
	if got := dry.low(1660); math.Abs(float64(got-0.2)) > 1e-3 {
		t.Errorf("gathering dry air: low clouds %v, want them as they were, 0.2", got)
	}
	damp := newRidged(air.Mass{Cover: 0.2, Billow: 1, Damp: 0.6}, [2]float32{}, 0, 0.1)
	damp.run(10)
	if got, m := damp.low(1660), damp.w.At(1660, 512); got < 0.4 || m.Damp > 0.45 {
		t.Errorf("gathering air 0.6 damp for 10 s: low clouds %v, damp %v; want more clouds "+
			"and less damp", got, m.Damp)
	}
	if got := damp.low(500); math.Abs(float64(got-0.2)) > 1e-3 {
		t.Errorf("away from where the land gathers the low clouds are %v, want 0.2", got)
	}
}

func TestDome_LiftWringsRainOutWhereTheLandSays(t *testing.T) {
	r := newRidged(air.Mass{Cover: 0.7, Billow: 1, Damp: 0.8}, [2]float32{}, 0.8, 0)
	r.run(10)
	there, away := r.w.At(1660, 512), r.w.At(500, 512)
	if there.Falls < 0.4 || there.Damp >= 0.8 || away.Falls != 0 {
		t.Errorf("wringing 0.8 of heavy low clouds: rain %v and damp %v there, rain %v away; "+
			"want rain there taking the damp, none away", there.Falls, there.Damp, away.Falls)
	}
	r.land.Turn() // nothing noted: the land wrings no more
	r.run(15)
	if got := r.w.At(1660, 512).Falls; got > 0.01 {
		t.Errorf("15 s after the land stopped wringing it still rains %v, want the weather's 0",
			got)
	}
}
