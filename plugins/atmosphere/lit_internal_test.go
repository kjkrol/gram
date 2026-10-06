package atmosphere

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
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
// they are — the light a tile composed every frame takes and the even light a still is drawn in
// alike.
func TestWithBoard_LightsAFlatBoardAndItsSpritesByTheHour(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
	})
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	b := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	b.Res.Logic.Board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	b.WithRenderer(litSheet{})
	cam := w.Camera()
	even := func() render.Light {
		l, ok := b.Map().Dressing().(look.EvenLit).EvenLight()
		if !ok {
			t.Fatal("a flat board's dressing does not light its tiles alike")
		}
		return l
	}
	tile := func() render.Light { return b.Map().Dressing().Light(nil)[0] }
	sprite := func() render.Vertex {
		var f render.Frame
		f.Reset(cam)
		w.Look().Sprite(&f, cam, plane.NewAABB(geom.NewVec(40, 40), 10, 10), world.Z{}, litSheet{}, render.Appearance{}, render.Light{1, 1, 1})
		var v render.Vertex
		f.Each(func(_ render.Tier, _ float32, verts []render.Vertex) { v = verts[0] })
		return v
	}
	if l := tile(); l != (render.Light{1, 1, 1}) {
		t.Fatalf("without an atmosphere a tile is lit %v, want as it is", l)
	}
	if l := even(); l != (render.Light{1, 1, 1}) {
		t.Fatalf("without an atmosphere the tiles' even light is %v, want white", l)
	}
	a := NewPlugin(w, Config{Sky: sky.Config{Frozen: true, Hour: 29 * time.Minute}}).WithBoard(b) // frozen at 00:29
	if sun := a.Heavens().Sun; sun[2] > 0 {
		t.Fatalf("the sun at half past midnight stands at %v, want under the horizon", sun)
	}
	if l := tile(); l[0] >= 0.5 || l[2] <= l[0] {
		t.Errorf("under the midnight sky a tile is lit %v, want it dark and blue", l)
	}
	if l := even(); l != tile() {
		t.Errorf("under the midnight sky the tiles' even light is %v, want the tile's %v", l, tile())
	}
	if v := sprite(); v.ColorR >= 0.5 || v.ColorB <= v.ColorR {
		t.Errorf("under the midnight sky a sprite is lit %v %v %v, want it dark and blue", v.ColorR, v.ColorG, v.ColorB)
	}
}
