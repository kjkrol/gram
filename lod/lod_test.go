package lod_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/lod"
)

func TestLadder_AtIsTheFirstLevelWhoseThresholdTheScaleMeets(t *testing.T) {
	l := lod.Ladder{Scales: [4]float32{2, 1, 0.5, 0.25}}
	for _, c := range []struct {
		scale float32
		want  lod.Level
	}{
		{4, lod.L0},
		{2, lod.L0},
		{1.5, lod.L1},
		{1, lod.L1},
		{0.6, lod.L2},
		{0.25, lod.L3},
		{0.2, lod.L4},
		{0, lod.L4},
	} {
		if got := l.At(c.scale, 0); got != c.want {
			t.Errorf("at a scale of %v: %v, want %v", c.scale, got, c.want)
		}
	}
}

func TestLadder_AtMeetsAZeroThresholdAtAnyScale(t *testing.T) {
	l := lod.Ladder{Scales: [4]float32{0.5}}
	if got := l.At(0.6, 0); got != lod.L0 {
		t.Errorf("over the one threshold: %v, want L0", got)
	}
	if got := l.At(0.01, 0); got != lod.L1 {
		t.Errorf("under the one threshold: %v, want L1, whose zero any scale meets", got)
	}
	if got := (lod.Ladder{}).At(0, 1e9); got != lod.L0 {
		t.Errorf("the zero ladder far off: %v, want L0 everywhere without end", got)
	}
}

func TestLadder_AtIsNothingPastTheSight(t *testing.T) {
	l := lod.Ladder{Scales: [4]float32{0.5}, Sight: 1000, Fog: 300}
	if got := l.At(4, 1000); got != lod.L0 {
		t.Errorf("at the sight's end: %v, want L0", got)
	}
	if got := l.At(4, 1001); got != lod.L4 {
		t.Errorf("past the sight: %v, want L4", got)
	}
}

func TestLadder_WidenedSeesFartherKeepingTheFogAndWithoutEndStaysSo(t *testing.T) {
	l := lod.Ladder{Scales: [4]float32{0.5}, Sight: 100, Fog: 10}
	if got := l.Widened(2); got.Sight != 200 || got.Fog != 10 || got.Scales != l.Scales {
		t.Errorf("widened twice: %+v, want a sight of 200, the fog and thresholds kept", got)
	}
	if got := (lod.Ladder{}).Widened(2); got.Sight != 0 {
		t.Errorf("a sight without end widened is %v, want it without end still", got.Sight)
	}
}

func TestLadder_CheckRefusesALadderThatCannotBe(t *testing.T) {
	for name, l := range map[string]lod.Ladder{
		"rising":             {Scales: [4]float32{1, 2}},
		"equal":              {Scales: [4]float32{1, 1}},
		"set after a zero":   {Scales: [4]float32{1, 0, 0.5}},
		"negative scale":     {Scales: [4]float32{-1}},
		"negative sight":     {Sight: -1},
		"fog past sight":     {Sight: 100, Fog: 200},
		"fog without sight":  {Fog: 10},
		"keeping past all":   {Keep: [lod.Levels]float32{1, 1, 1.5}},
		"keeping under none": {Keep: [lod.Levels]float32{0, -0.1}},
		"keeping no number":  {Keep: [lod.Levels]float32{float32(math.NaN())}},
		"negative texel":     {Texel: -1},
		"unknown fallback":   {Unobserved: lod.UnobservedL4 + 1},
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
		{Scales: [4]float32{2, 1, 0.5, 0.25}, Sight: 100, Fog: 100},
		{Scales: [4]float32{0.5}, Sight: 1000},
		{Keep: [lod.Levels]float32{1, 0.5, 0.25, 0, 1}, Texel: 8},
		{Unobserved: lod.UnobservedL4},
	} {
		l.Check()
	}
}

func TestFallback_LevelIsTheOneItsNameSaysItsZeroL2(t *testing.T) {
	for f, want := range map[lod.Fallback]lod.Level{
		lod.UnobservedL0: lod.L0,
		lod.UnobservedL1: lod.L1,
		lod.UnobservedL2: lod.L2,
		lod.UnobservedL3: lod.L3,
		lod.UnobservedL4: lod.L4,
	} {
		if got := f.Level(); got != want {
			t.Errorf("Fallback %d at %v, want %v", f, got, want)
		}
	}
	if got := (lod.Ladder{}).Unobserved.Level(); got != lod.L2 {
		t.Errorf("the zero ladder unobserved at %v, want L2", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("an unknown Fallback had a level")
		}
	}()
	(lod.UnobservedL4 + 1).Level()
}

func TestPaces_CheckRefusesPacesThatCannotBe(t *testing.T) {
	for name, p := range map[string]lod.Paces{
		"L0 slowed":       {2, 2, 2, 2, 0},
		"L0 still":        {0, 0, 0, 0, 0},
		"L4 going":        {1, 1, 1, 1, 1},
		"faster farther":  {1, 4, 2, 2, 0},
		"after a stop":    {1, 0, 2, 0, 0},
		"after a stop L3": {1, 2, 0, 4, 0},
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
	for _, p := range []lod.Paces{lod.Exact, lod.Steady, {1, 1, 1, 0, 0}, {1, 2, 2, 8, 0}} {
		p.Check()
	}
}

func TestSlowing_RepeatsTheLastStepAndRefusesNoneAZeroOrTooMany(t *testing.T) {
	for _, c := range []struct {
		n    []uint8
		want lod.Paces
	}{
		{[]uint8{4}, lod.Paces{1, 4, 4, 4, 0}},
		{[]uint8{2, 4}, lod.Paces{1, 2, 4, 4, 0}},
		{[]uint8{2, 4, 8}, lod.Paces{1, 2, 4, 8, 0}},
		{[]uint8{1}, lod.Steady},
	} {
		if got := lod.Slowing(c.n...); got != c.want {
			t.Errorf("Slowing(%v) = %v, want %v", c.n, got, c.want)
		}
	}
	for name, n := range map[string][]uint8{
		"none":     nil,
		"a zero":   {2, 0},
		"too many": {2, 4, 8, 16},
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
	l := lod.Ladder{Scales: [4]float32{2, 1, 0.5, 0.25}}
	for _, c := range []struct {
		from  lod.Level
		scale float32
		want  lod.Level
	}{
		{lod.L2, 1, lod.L1},
		{lod.L3, 2, lod.L0},
		{lod.L2, 0.6, lod.L2},
		{lod.L1, 0.95, lod.L1},
		{lod.L1, 0.89, lod.L2},
		{lod.L1, 0.47, lod.L2},
		{lod.L1, 0.4, lod.L3},
		{lod.L3, 0.24, lod.L3},
		{lod.L0, 0.1, lod.L4},
	} {
		if got := l.Hold(c.from, c.scale, 0, 0); got != c.want {
			t.Errorf("from %v at a scale of %v: %v, want %v", c.from, c.scale, got, c.want)
		}
	}
}

func TestLadder_HoldGoesPastTheSightOnlyByTheMargin(t *testing.T) {
	l := lod.Ladder{Scales: [4]float32{0.5}, Sight: 1000}
	for _, c := range []struct {
		from lod.Level
		dist float32
		want lod.Level
	}{
		{lod.L0, 1100, lod.L0},
		{lod.L0, 1201, lod.L4},
		{lod.L4, 1100, lod.L4},
		{lod.L4, 999, lod.L0},
	} {
		if got := l.Hold(c.from, 4, c.dist, 200); got != c.want {
			t.Errorf("from %v %v off, a margin of 200: %v, want %v", c.from, c.dist, got, c.want)
		}
	}
}
