package atmosphere

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// litSheet is an AtlasSource of one sprite with no image behind it.
type litSheet struct{}

func (litSheet) Atlas() *render.Image                            { return nil }
func (litSheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 8, 8 }
func (litSheet) White() (u, v float32)                           { return 9, 9 }

// Under the atmosphere a flat board's tiles and the world's sprites take the sun's light on level
// ground: at midnight, frozen, they are dark and blue; without the atmosphere they are drawn as
// they are.
func TestWithBoard_LightsAFlatBoardAndItsSpritesByTheHour(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
	})
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	b := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	b.Res.Logic.Board.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	b.WithRenderer(litSheet{})
	b.Res.Render.ShowGridLines = false
	cam := w.Camera()
	tile := func() render.Vertex {
		var f render.Frame
		f.Reset(cam)
		b.Renderer().(render.Source).Compose(&f, cam)
		var v render.Vertex
		f.Each(func(_ render.Tier, _ float32, verts []render.Vertex) { v = verts[0] })
		return v
	}
	sprite := func() render.Vertex {
		var f render.Frame
		f.Reset(cam)
		w.Look().Sprite(&f, cam, plane.NewAABB(geom.NewVec(40, 40), 10, 10), world.Z{}, litSheet{}, 0, render.Light{1, 1, 1}, 0)
		var v render.Vertex
		f.Each(func(_ render.Tier, _ float32, verts []render.Vertex) { v = verts[0] })
		return v
	}
	if v := tile(); v.ColorR != 1 || v.ColorB != 1 {
		t.Fatalf("without an atmosphere a tile is lit %v %v %v, want as it is", v.ColorR, v.ColorG, v.ColorB)
	}
	a := NewPlugin(w, Config{Sky: sky.Config{Frozen: true, Hour: 0.02}}).WithBoard(b) // frozen at 00:29
	if s := a.Sun(); s.Dir[2] > 0 {
		t.Fatalf("the sun at half past midnight stands at %v, want under the horizon", s.Dir)
	}
	if v := tile(); v.ColorR >= 0.5 || v.ColorB <= v.ColorR {
		t.Errorf("under the midnight sky a tile is lit %v %v %v, want it dark and blue", v.ColorR, v.ColorG, v.ColorB)
	}
	if v := sprite(); v.ColorR >= 0.5 || v.ColorB <= v.ColorR {
		t.Errorf("under the midnight sky a sprite is lit %v %v %v, want it dark and blue", v.ColorR, v.ColorG, v.ColorB)
	}
}
