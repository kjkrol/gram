package main

import (
	_ "embed"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
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

	heroSpeed = CellSize * 2
)

// hero is a figure on a patrol: drawn from its sheet in eight ways, stepping the way it goes.
type hero struct{ sheet render.Sheet }

var _ board.Figure = (*hero)(nil)

func newHero() *hero {
	return &hero{sheet: render.DecodeSheet("hero.png", heroPNG, heroWidth, heroHeight)}
}

func (h *hero) Define(b *board.Plugin) {
	units := board.NewUnits[unitRow](b, board.Shape{Size: heroHeight, Width: heroWidth},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: heroSpeed,
		Accel:    heroSpeed * 2,
		Brake:    heroSpeed * 4,
		V0:       heroSpeed / 2,
		TurnRate: 0.08,
	}
	patrol := plan.New(PatrolPlan, plan.Steps(
		plan.Loop[any]{
			Name:  "leg",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always),
			Then: plan.Self.Wait(time.Second)},
	))
	round := comp.Load(func(u unitRow) navigation.Round { return navigation.NewRound(u.Round[:]...) })
	units.Define(HeroKind, unit.Mover{Domain: cell.Land}, profile, round, patrol)
}

func (h *hero) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), HeroKind), heroHeight, h.standing(0)).
		Facing(heroWays, h.standing).
		Animated(heroWalkFrames, 120*time.Millisecond, h.pose(heroWalk)).WhileMoving()
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
