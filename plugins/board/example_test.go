package board_test

import (
	"fmt"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/world"
)

// ramp is a game's own map: flat to the eye, the ground rising a unit every ten eastwards
type ramp struct{ g grid.Grid }

func (ramp) Look() look.Look           { return look.Flat }
func (r ramp) Heights() ground.Heights { return rise{} }
func (ramp) Dressing() look.Dressing   { return nil }

func (r ramp) Top(c cell.ID, height float32) (corners [4]float32, level float32) {
	at := r.g.CellCenter(c)
	w, _ := r.g.CellBounds()
	west, east := (at.X-w/2)/10+height, (at.X+w/2)/10+height
	return [4]float32{west, east, west, east}, at.X / 10
}

func (r ramp) Rise(from, to cell.ID) float64 {
	a, b := r.g.CellCenter(from), r.g.CellCenter(to)
	run := math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y))
	if run == 0 {
		return 0
	}
	return float64(b.X-a.X) / 10 / run
}

type rise struct{}

func (rise) At(p geom.Vec) float32 { return p.X / 10 }
func (rise) Step() float32         { return 32 }

// A game gives the board a map of its own: how it looks, how high its ground lies and how steep
// a step is.
func ExamplePlugin_WithMap() {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 320, Height: 320},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 32},
		Heights:  true,
	})
	g := grid.DefaultGrids{}.Square(10, 10, 32)
	b := board.NewPlugin(g, &cell.SingleOccupancy{}, w).WithMap(ramp{g})
	fmt.Println(b.Read(0).Heights().At(geom.NewVec(160, 0)))
	// Output: 16
}
