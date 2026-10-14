package main

import (
	_ "embed"
	"math"
	"sync"

	"github.com/kjkrol/gram/render"
)

// eight rows of frames 16 wide and 32 tall, the ways east first against the clock; the columns
// standing, four steps, three pulls, four of boredom
//
//go:embed hero.png
var heroPNG []byte

const (
	heroWidth      = 16
	heroHeight     = 32
	heroWays       = 8
	heroStand      = 0
	heroWalk       = 1
	heroWalkFrames = 4
	heroPull       = 5
	heroPullFrames = 3
	heroIdle       = 8
	heroIdleFrames = 4
)

var heroSheet = sync.OnceValue(func() render.Sheet {
	return render.DecodeSheet("hero.png", heroPNG, heroWidth, heroHeight)
})

// the row of the way a heading looks, degrees, 0 east, against the clock
func heroWay(angleDeg float64) int { return int(math.Round(angleDeg/360*heroWays)) % heroWays }

func heroStanding(angleDeg float64) render.SpriteDrawer {
	return heroSheet().At(heroStand, heroWay(angleDeg))
}

// the frames of the column col and those after it, each the way of angleDeg
func heroPose(col int) func(angleDeg float64, frame int) render.SpriteDrawer {
	return func(angleDeg float64, frame int) render.SpriteDrawer {
		return heroSheet().At(col+frame, heroWay(angleDeg))
	}
}
