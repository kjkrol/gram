package lod_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/gram/lod"
)

func TestLadder_AtIsTheFirstLevelWhoseThresholdTheScaleMeets(t *testing.T) {
	l := lod.Ladder{Scales: [6]float32{0, 0, 2, 1, 0.5, 0.25}}
	for _, c := range []struct {
		scale float32
		want  lod.Level
	}{
		{4, lod.L2},
		{2, lod.L2},
		{1.5, lod.L3},
		{1, lod.L3},
		{0.6, lod.L4},
		{0.25, lod.L5},
		{0.2, lod.L6},
		{0, lod.L6},
	} {
		if got := l.At(c.scale, 0); got != c.want {
			t.Errorf("at a scale of %v: %v, want %v", c.scale, got, c.want)
		}
	}
}

func TestLadder_AtMeetsAZeroThresholdFromStandardAtAnyScale(t *testing.T) {
	l := lod.Ladder{Scales: [6]float32{0, 0, 0.5}}
	if got := l.At(0.6, 0); got != lod.L2 {
		t.Errorf("over the one threshold: %v, want L2", got)
	}
	if got := l.At(0.01, 0); got != lod.L3 {
		t.Errorf("under the one threshold: %v, want L3, whose zero any scale meets", got)
	}
}

func TestLadder_TheZeroLadderIsStandardAtEveryScaleAndDistance(t *testing.T) {
	for _, scale := range []float32{0, 0.01, 1, 1e9} {
		for _, dist := range []float32{0, 1e9} {
			if got := (lod.Ladder{}).At(scale, dist); got != lod.Standard {
				t.Errorf("at a scale of %v %v off: %v, want Standard", scale, dist, got)
			}
		}
	}
}

func TestLadder_AtGoesCloseOnlyOverTheCloseThresholdsSet(t *testing.T) {
	l := lod.Ladder{Scales: [6]float32{8, 4, 2, 1, 0.5, 0.25}}
	for scale, want := range map[float32]lod.Level{16: lod.L0, 8: lod.L0, 5: lod.L1, 3: lod.L2} {
		if got := l.At(scale, 0); got != want {
			t.Errorf("at a scale of %v: %v, want %v", scale, got, want)
		}
	}
	coarse := lod.Ladder{Scales: [6]float32{0, 4, 0.5}}
	for scale, want := range map[float32]lod.Level{16: lod.L1, 4: lod.L1, 1: lod.L2, 0: lod.L3} {
		if got := coarse.At(scale, 0); got != want {
			t.Errorf("L1 alone set, at a scale of %v: %v, want %v", scale, got, want)
		}
	}
}

func TestLadder_AtIsNothingPastTheSight(t *testing.T) {
	l := lod.Ladder{Scales: [6]float32{0, 0, 0.5}, Sight: 1000, Fog: 300}
	if got := l.At(4, 1000); got != lod.L2 {
		t.Errorf("at the sight's end: %v, want L2", got)
	}
	if got := l.At(4, 1001); got != lod.Omitted {
		t.Errorf("past the sight: %v, want Omitted", got)
	}
}

func TestLadder_WidenedSeesFartherKeepingTheFogAndWithoutEndStaysSo(t *testing.T) {
	l := lod.Ladder{Scales: [6]float32{0, 0, 0.5}, Sight: 100, Fog: 10}
	if got := l.Widened(2); got.Sight != 200 || got.Fog != 10 || got.Scales != l.Scales {
		t.Errorf("widened twice: %+v, want a sight of 200, the fog and thresholds kept", got)
	}
	if got := (lod.Ladder{}).Widened(2); got.Sight != 0 {
		t.Errorf("a sight without end widened is %v, want it without end still", got.Sight)
	}
}

func TestLadder_CheckRefusesALadderThatCannotBe(t *testing.T) {
	for name, l := range map[string]lod.Ladder{
		"rising":              {Scales: [6]float32{0, 0, 1, 2}},
		"equal":               {Scales: [6]float32{0, 0, 1, 1}},
		"set after a zero":    {Scales: [6]float32{0, 0, 1, 0, 0.5}},
		"close rising":        {Scales: [6]float32{1, 2, 0.5}},
		"close equal":         {Scales: [6]float32{1, 1, 0.5}},
		"close under the one": {Scales: [6]float32{0.5, 0, 1}},
		"negative scale":      {Scales: [6]float32{0, 0, -1}},
		"negative close":      {Scales: [6]float32{-1}},
		"negative sight":      {Sight: -1},
		"fog past sight":      {Sight: 100, Fog: 200},
		"fog without sight":   {Fog: 10},
		"keeping past all":    {Keep: [lod.Levels]float32{1, 1, 1.5}},
		"keeping under none":  {Keep: [lod.Levels]float32{0, -0.1}},
		"keeping no number":   {Keep: [lod.Levels]float32{float32(math.NaN())}},
		"negative texel":      {Texel: -1},
		"negative fold":       {Fold: -1},
		"unknown fallback":    {Unobserved: lod.UnobservedL6 + 1},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%+v passed", l)
				}
			}()
			l.Check()
		})
	}
	for _, l := range []lod.Ladder{
		{},
		{Scales: [6]float32{0, 0, 2, 1, 0.5, 0.25}, Sight: 100, Fog: 100},
		{Scales: [6]float32{8, 4, 2, 1, 0.5, 0.25}},
		{Scales: [6]float32{0, 4, 0.5}},
		{Scales: [6]float32{4, 0, 0.5}},
		{Scales: [6]float32{4}},
		{Scales: [6]float32{0, 0, 0.5}, Sight: 1000},
		{Keep: [lod.Levels]float32{1, 1, 1, 0.5, 0.25, 0, 1}, Texel: 8},
		{Fold: 0.5},
		{Unobserved: lod.UnobservedL6},
	} {
		l.Check()
	}
}

func TestFallback_LevelIsTheOneItsNameSaysItsZeroL4(t *testing.T) {
	for f, want := range map[lod.Fallback]lod.Level{
		lod.UnobservedL0: lod.L0,
		lod.UnobservedL1: lod.L1,
		lod.UnobservedL2: lod.L2,
		lod.UnobservedL3: lod.L3,
		lod.UnobservedL4: lod.L4,
		lod.UnobservedL5: lod.L5,
		lod.UnobservedL6: lod.L6,
	} {
		if got := f.Level(); got != want {
			t.Errorf("Fallback %d at %v, want %v", f, got, want)
		}
	}
	if got := (lod.Ladder{}).Unobserved.Level(); got != lod.L4 {
		t.Errorf("the zero ladder unobserved at %v, want L4", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("an unknown Fallback had a level")
		}
	}()
	(lod.UnobservedL6 + 1).Level()
}

func TestPaces_CheckRefusesPacesThatCannotBe(t *testing.T) {
	for name, p := range map[string]lod.Paces{
		"the finest slowed": {2, 2, 2, 2, 2, 2, 0},
		"the finest still":  {0, 0, 0, 0, 0, 0, 0},
		"six coarser going": {1, 1, 1, 1, 1, 1, 1},
		"faster farther":    {1, 4, 2, 2, 2, 2, 0},
		"faster at five":    {1, 2, 4, 8, 8, 4, 0},
		"after a stop":      {1, 0, 2, 0, 0, 0, 0},
		"after a stop at 5": {1, 2, 4, 8, 0, 8, 0},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%v passed", p)
				}
			}()
			p.Check()
		})
	}
	for _, p := range []lod.Paces{
		lod.Exact,
		lod.Steady,
		lod.Contact,
		{1, 1, 1, 0, 0, 0, 0},
		{1, 2, 2, 8, 8, 16, 0},
	} {
		p.Check()
	}
}

func TestSlowing_RepeatsTheLastStepAndRefusesNoneAZeroOrTooMany(t *testing.T) {
	for _, c := range []struct {
		n    []uint8
		want lod.Paces
	}{
		{[]uint8{4}, lod.Paces{1, 4, 4, 4, 4, 4, 0}},
		{[]uint8{2, 4}, lod.Paces{1, 2, 4, 4, 4, 4, 0}},
		{[]uint8{2, 4, 8}, lod.Paces{1, 2, 4, 8, 8, 8, 0}},
		{[]uint8{2, 4, 8, 16, 32}, lod.Paces{1, 2, 4, 8, 16, 32, 0}},
		{[]uint8{1}, lod.Steady},
	} {
		if got := lod.Slowing(c.n...); got != c.want {
			t.Errorf("Slowing(%v) = %v, want %v", c.n, got, c.want)
		}
	}
	for name, n := range map[string][]uint8{
		"none":     nil,
		"a zero":   {2, 0},
		"too many": {2, 4, 8, 16, 32, 64},
		"falling":  {4, 2},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("Slowing(%v) passed", n)
				}
			}()
			lod.Slowing(n...)
		})
	}
}

func TestLadder_HoldGoesFinerAtOnceAndCoarserOnlyATenthPastAThreshold(t *testing.T) {
	l := lod.Ladder{Scales: [6]float32{0, 0, 2, 1, 0.5, 0.25}}
	for _, c := range []struct {
		from  lod.Level
		scale float32
		want  lod.Level
	}{
		{lod.L4, 1, lod.L3},
		{lod.L5, 2, lod.L2},
		{lod.L4, 0.6, lod.L4},
		{lod.L3, 0.95, lod.L3},
		{lod.L3, 0.89, lod.L4},
		{lod.L3, 0.47, lod.L4},
		{lod.L3, 0.4, lod.L5},
		{lod.L5, 0.24, lod.L5},
		{lod.L2, 0.1, lod.L6},
	} {
		if got := l.Hold(c.from, c.scale, 0, 0); got != c.want {
			t.Errorf("from %v at a scale of %v: %v, want %v", c.from, c.scale, got, c.want)
		}
	}
}

func TestLadder_HoldGoesPastTheSightOnlyByTheMargin(t *testing.T) {
	l := lod.Ladder{Scales: [6]float32{0, 0, 0.5}, Sight: 1000}
	for _, c := range []struct {
		from lod.Level
		dist float32
		want lod.Level
	}{
		{lod.L2, 1100, lod.L2},
		{lod.L2, 1201, lod.L6},
		{lod.L6, 1100, lod.L6},
		{lod.L6, 999, lod.L2},
	} {
		if got := l.Hold(c.from, 4, c.dist, 200); got != c.want {
			t.Errorf("from %v %v off, a margin of 200: %v, want %v", c.from, c.dist, got, c.want)
		}
	}
}

func TestLadder_FoldAfterIsItsSecondsThreeForNone(t *testing.T) {
	if got := (lod.Ladder{}).FoldAfter(); got != 3*time.Second {
		t.Errorf("no Fold folds after %v, want 3s", got)
	}
	if got := (lod.Ladder{Fold: 0.5}).FoldAfter(); got != 500*time.Millisecond {
		t.Errorf("a Fold of 0.5 folds after %v, want 500ms", got)
	}
}
