package main

import (
	_ "embed"
	"math"
	"sync"
	"time"

	"github.com/kjkrol/gram/render"
)

// eight rows of frames 16 wide and 32 tall, the ways east first against the clock; the columns
// standing, four steps, three pulls, a blink, two of a stamp, two of braking
//
//go:embed hero.png
var heroPNG []byte

const (
	heroWidth       = 16
	heroHeight      = 32
	heroWays        = 8
	heroStand       = 0
	heroWalk        = 1
	heroWalkFrames  = 4
	heroPull        = 5
	heroPullFrames  = 3
	heroBlink       = 8
	heroStamp       = 9
	heroBrake       = 11
	heroBrakeFrames = 2
)

var heroSheet = sync.OnceValue(func() render.Sheet {
	return render.DecodeSheet("hero.png", heroPNG, heroWidth, heroHeight)
})

// the row of the way a heading looks, degrees, 0 east, against the clock
func heroWay(angleDeg float64) int { return int(math.Round(angleDeg/360*heroWays)) % heroWays }

func heroStanding(angleDeg float64) render.SpriteDrawer {
	return heroSheet().At(heroStand, heroWay(angleDeg))
}

// the frames of a beat: standing, the eyes shut
const (
	beatStand = iota
	beatBlink
)

var heroBeatCols = [...]int{beatStand: heroStand, beatBlink: heroBlink}

func beat(frame int, span time.Duration) render.Beat {
	return render.Beat{Frame: frame, Span: span}
}

func heroBeat(angleDeg float64, frame int) render.SpriteDrawer {
	return heroSheet().At(heroBeatCols[frame], heroWay(angleDeg))
}

// the eyes shut for 150 ms every 3 s
var heroBlinkBeat = []render.Beat{
	beat(beatStand, 2850*time.Millisecond),
	beat(beatBlink, 150*time.Millisecond),
}

// a series of three stamps along the effect's course: the foot up and down thrice
const heroStampFrames = 6

func heroStampSeries(angleDeg float64, frame int) render.SpriteDrawer {
	return heroSheet().At(heroStamp+frame%2, heroWay(angleDeg))
}

// the frames of the column col and those after it, each the way of angleDeg
func heroPose(col int) func(angleDeg float64, frame int) render.SpriteDrawer {
	return func(angleDeg float64, frame int) render.SpriteDrawer {
		return heroSheet().At(col+frame, heroWay(angleDeg))
	}
}
