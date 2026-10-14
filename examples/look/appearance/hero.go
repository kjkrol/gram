package main

import (
	_ "embed"
	"math"
	"time"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// eight rows of frames 16 wide and 32 tall, the ways east first against the clock; the columns
// standing, four steps, three pulls, four of idling
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
	heroIdle       = 8
	heroIdleFrames = 4
)

// hero is the walkers' figure: drawn from its sheet in eight ways, stepping the way it goes;
// angry, it stamps.
type hero struct{ sheet render.Sheet }

var _ world.Figure = (*hero)(nil)

func newHero() *hero {
	return &hero{sheet: render.DecodeSheet("hero.png", heroPNG, heroWidth, heroHeight)}
}

func (h *hero) Define(w *world.Plugin) { kind.Define[walker](w.Kinds(), WalkerKind, walkerSpec(w)) }

func (h *hero) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[walker](w.Kinds(), WalkerKind), heroHeight, h.standing(0)).
		Facing(heroWays, h.standing).
		Animated(heroWalkFrames, 120*time.Millisecond, h.pose(heroWalk)).WhileMoving().
		Under(w.Effects().Named(AngryEf),
			render.LoopedFacing(heroIdleFrames, 200*time.Millisecond, h.pose(heroIdle)))
}

// the row of the way a heading looks, degrees, 0 east, against the clock
func heroWay(angleDeg float64) int { return int(math.Round(angleDeg/360*heroWays)) % heroWays }

func (h *hero) standing(angleDeg float64) render.SpriteDrawer {
	return h.sheet.At(heroStand, heroWay(angleDeg))
}

// the frames of the column col and those after it, each the way of angleDeg
func (h *hero) pose(col int) func(angleDeg float64, frame int) render.SpriteDrawer {
	return func(angleDeg float64, frame int) render.SpriteDrawer {
		return h.sheet.At(col+frame, heroWay(angleDeg))
	}
}
