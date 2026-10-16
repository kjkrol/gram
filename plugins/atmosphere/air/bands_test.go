package air_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/world"
)

// a metre a tenth of a world unit
var tenth = world.Scale{Metres: 10}

func over(low, peaks float64, ceilings [3]float32, drift [2]float32) air.Weather {
	relief := air.ReliefOf(tenth, low, peaks)
	return air.Weather{Relief: relief, Bases: air.BasesOver(tenth, relief), Ceilings: ceilings,
		Drift: drift}
}

func TestLowBand_HangsAsSharesOfTheRelief(t *testing.T) {
	shares := [3]float32{0.3, 0.15, 0.55}
	madeira := over(0, 186, shares, [2]float32{700, -300})
	alps := over(50, 450, shares, [2]float32{700, -300})
	for i := range 200 {
		x, y := float32(i)*97-9000, float32(i)*31+400
		mb, mt := madeira.LowBand(x, y)
		ab, at := alps.LowBand(x, y)
		m := [2]float32{(mb - madeira.Relief[0]) / madeira.Relief[1],
			(mt - madeira.Relief[0]) / madeira.Relief[1]}
		a := [2]float32{(ab - alps.Relief[0]) / alps.Relief[1],
			(at - alps.Relief[0]) / alps.Relief[1]}
		if math.Abs(float64(m[0]-a[0])) > 1e-5 || math.Abs(float64(m[1]-a[1])) > 1e-5 {
			t.Fatalf("at (%v, %v) the low clouds hang %v of a relief 1860 m high and %v of one "+
				"4000 m high, want alike", x, y, m, a)
		}
	}
}

func TestReliefOf_TakesLowHillsAsTheLeastRelief(t *testing.T) {
	if got := air.ReliefOf(tenth, 5, 30); got != [2]float32{5, 100} {
		t.Errorf("hills 250 m over ground 50 m up: relief %v, want [5 100] (1 km)", got)
	}
	if got := air.ReliefOf(tenth, 0, 186); got != [2]float32{0, 186} {
		t.Errorf("peaks 1860 m up: relief %v, want [0 186]", got)
	}
}

func TestLowBand_OneShareHangsEverywhereInItsBand(t *testing.T) {
	for _, c := range []struct {
		shares    [3]float32
		base, top float32
	}{
		{[3]float32{1, 0, 0}, 0.68, 0.79},
		{[3]float32{0, 1, 0}, 0.74, 1.05},
		{[3]float32{0, 0, 1}, 1.05, 1.25},
	} {
		w := over(0, 200, c.shares, [2]float32{})
		for i := range 400 {
			x, y := float32(i)*53-7000, float32(i)*41-3000
			base, top := w.LowBand(x, y)
			if math.Abs(float64(base-200*c.base)) > 1e-3 ||
				math.Abs(float64(top-200*c.top)) > 1e-3 {
				t.Fatalf("shares %v: at (%v, %v) the low clouds hang from %v to %v, want %v to %v",
					c.shares, x, y, base, top, 200*c.base, 200*c.top)
			}
		}
	}
}

func TestLowBand_ChangesWithoutJumps(t *testing.T) {
	w := over(0, 200, [3]float32{0.3, 0.3, 0.4}, [2]float32{123, 45})
	was, _ := w.LowBand(-20000, 77)
	low, high := was, was
	for x := float32(-20000); x < 20000; x++ {
		base, _ := w.LowBand(x, 77)
		if math.Abs(float64(base-was)) > 0.005*200 {
			t.Fatalf("a world unit on to (%v, 77) the low clouds' base jumps from %v to %v",
				x, was, base)
		}
		was, low, high = base, min(low, base), max(high, base)
	}
	if low > 200*0.7 || high < 200*1.0 {
		t.Errorf("along 40000 world units the low clouds' base ranges from %v to %v, want "+
			"every band", low, high)
	}
}

func TestLowBand_DriftsSlowerThanTheClouds(t *testing.T) {
	shares := [3]float32{0.3, 0.3, 0.4}
	still := over(0, 200, shares, [2]float32{})
	blown := over(0, 200, shares, [2]float32{1000, 0})
	moved := 0
	for i := range 300 {
		x, y := float32(i)*67-9000, float32(i)*13
		was, _ := still.LowBand(x, y)
		now, _ := blown.LowBand(x+600, y)
		if math.Abs(float64(now-was)) > 1e-3 {
			t.Fatalf("the wind carried the clouds 1000 on: the base at (%v, %v) is %v, 600 on "+
				"%v; want the bands carried 600", x, y, was, now)
		}
		if carried, _ := blown.LowBand(x+1000, y); math.Abs(float64(carried-was)) > 1 {
			moved++
		}
	}
	if moved == 0 {
		t.Error("a cloud carried 1000 on hangs where it hung everywhere, want it risen or sunk " +
			"somewhere")
	}
}

func TestBasesOver_HangsTheHighCloudsOverTheLowOnes(t *testing.T) {
	madeira := over(0, 186, [3]float32{}, [2]float32{})
	if madeira.Bases[air.High] != 600 {
		t.Errorf("over peaks 1860 m up the high clouds hang %v up, want 600 (6000 m)",
			madeira.Bases[air.High])
	}
	alps := over(0, 450, [3]float32{}, [2]float32{})
	if top := alps.Top(air.Low); alps.Bases[air.High] < top+100 {
		t.Errorf("over peaks 4500 m up the high clouds hang %v up, the low ones' lumps up to %v; "+
			"want the high ones 1 km over them", alps.Bases[air.High], top)
	}
	lowest, highest := alps.BaseSpan(air.Low)
	if lowest != alps.Base(air.Low) || math.Abs(float64(lowest-450*0.68)) > 1e-3 ||
		math.Abs(float64(highest-450*1.05)) > 1e-3 {
		t.Errorf("over peaks 4500 m up the low clouds' bases span %v to %v, want %v to %v",
			lowest, highest, 450*0.68, 450*1.05)
	}
}
