package render

import (
	"fmt"
	"image"
	"time"

	xdraw "golang.org/x/image/draw"
)

// Phases is a look of n frames spread along an effect's course, from the first as the effect
// begins to the last as it runs out, once; a look an atlas takes Under an effect that Lasts.
// One of Draw and DrawFacing draws a frame, the latter for each way a facing sprite looks.
type Phases struct {
	N          int
	Draw       func(frame int) SpriteDrawer
	DrawFacing func(angleDeg float64, frame int) SpriteDrawer
}

// Phased is the look of n frames along an effect's course, each drawn by draw for its frame.
func Phased(n int, draw func(frame int) SpriteDrawer) Phases {
	if n <= 0 || draw == nil {
		panic(fmt.Sprintf("render: a look phased over %d frames", n))
	}
	return Phases{N: n, Draw: draw}
}

// PhasedFacing is Phased for a sprite that faces n ways: draw draws each frame at each angle.
func PhasedFacing(n int, draw func(angleDeg float64, frame int) SpriteDrawer) Phases {
	if n <= 0 || draw == nil {
		panic(fmt.Sprintf("render: a look phased over %d frames", n))
	}
	return Phases{N: n, DrawFacing: draw}
}

// Loops is a look of n frames shown in turn, each for Period of game time, round and round
// while an effect is on. One of Draw and DrawFacing draws a frame.
type Loops struct {
	N          int
	Period     time.Duration
	Draw       func(frame int) SpriteDrawer
	DrawFacing func(angleDeg float64, frame int) SpriteDrawer
}

// Looped is the look of n frames round and round, each shown period, drawn by draw.
func Looped(n int, period time.Duration, draw func(frame int) SpriteDrawer) Loops {
	if n <= 0 || period <= 0 || draw == nil {
		panic(fmt.Sprintf("render: a look looped over %d frames of %v", n, period))
	}
	return Loops{N: n, Period: period, Draw: draw}
}

// LoopedFacing is Looped for a sprite that faces n ways: draw draws each frame at each angle.
func LoopedFacing(n int, period time.Duration,
	draw func(angleDeg float64, frame int) SpriteDrawer) Loops {
	if n <= 0 || period <= 0 || draw == nil {
		panic(fmt.Sprintf("render: a look looped over %d frames of %v", n, period))
	}
	return Loops{N: n, Period: period, DrawFacing: draw}
}

// Frame is the frame shown at progress, 0 as the effect begins to 1 as it runs out.
func (p Phases) Frame(progress float32) int {
	return min(max(int(progress*float32(p.N)), 0), p.N-1)
}

// FromImage is a SpriteDrawer laying img over the whole sprite, scaled pixel for pixel without
// smoothing: a frame cut out of a sheet keeps its pixels.
func FromImage(img image.Image) SpriteDrawer {
	return func(dst *Canvas, size int) {
		xdraw.NearestNeighbor.Scale(dst.RGBA, image.Rect(0, 0, size, size), img, img.Bounds(),
			xdraw.Over, nil)
	}
}
