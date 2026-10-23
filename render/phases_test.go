package render_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/kjkrol/gram/render"
)

func TestPhases_FrameRunsFromTheFirstToTheLastAlongTheCourse(t *testing.T) {
	p := render.Phased(4, func(int) render.SpriteDrawer { return render.Solid(color.RGBA{}) })
	for progress, want := range map[float32]int{0: 0, 0.24: 0, 0.26: 1, 0.5: 2, 0.99: 3, 1: 3, 2: 3} {
		if got := p.Frame(progress); got != want {
			t.Errorf("at %v: frame %d, want %d", progress, got, want)
		}
	}
}

func TestLoops_FrameGoesRoundByPeriodOrToABeat(t *testing.T) {
	even := render.Loops{N: 4, Period: 100 * time.Millisecond}
	ms := time.Millisecond
	got := []int{even.Frame(0), even.Frame(150 * ms), even.Frame(450 * ms)}
	if got[0] != 0 || got[1] != 1 || got[2] != 0 {
		t.Errorf("even frames %v, want 0, 1, 0", got)
	}
	blink := render.Timed([]render.Beat{
		{Frame: 0, Span: 2850 * time.Millisecond},
		{Frame: 1, Span: 150 * time.Millisecond},
	}, nil)
	for now, want := range map[time.Duration]int{
		0:                        0,
		2849 * time.Millisecond:  0,
		2850 * time.Millisecond:  1,
		2999 * time.Millisecond:  1,
		3000 * time.Millisecond:  0,
		5850 * time.Millisecond:  1,
		12000 * time.Millisecond: 0,
	} {
		if got := blink.Frame(now); got != want {
			t.Errorf("at %v the frame is %d, want %d", now, got, want)
		}
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("a span of nothing was taken")
		}
	}()
	render.Timed([]render.Beat{{Frame: 0, Span: time.Second}, {Frame: 1}}, nil)
}
