package sky

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// shifted is a 100x100 screen over the world from x = dx on, drawn from above without wrapping.
type shifted struct {
	camera.Camera
	dx float32
}

func (s shifted) Viewport() (float32, float32)                   { return 100, 100 }
func (s shifted) Unproject(sx, sy, _ float32) (float32, float32) { return sx + s.dx, sy }

func TestBackdrop_FillsTheScreenWithTheSkyOnlyWhereTheGroundDoesNotCoverIt(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	w.SetSun(world.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.5, Sky: render.Light{0.5, 0.7, 1}})
	b := NewBackdrop(w)
	pieces := func(dx float32) (n int, tier render.Tier, depth float32, v ebiten.Vertex) {
		cam := shifted{Camera: w.Camera(), dx: dx}
		var f render.Frame
		f.Reset(cam)
		b.Compose(&f, cam)
		f.Each(func(t render.Tier, d float32, verts []ebiten.Vertex) { n, tier, depth, v = n+1, t, d, verts[3] })
		return
	}
	if n, _, _, _ := pieces(50); n != 0 {
		t.Errorf("over the middle of the world the backdrop drew %d pieces, want none: the ground covers the screen", n)
	}
	n, tier, depth, v := pieces(150)
	if n != 1 || tier != render.Backdrop || !math.IsInf(float64(depth), -1) {
		t.Fatalf("over the world's edge the backdrop drew %d pieces on tier %v at depth %v, want one behind everything", n, tier, depth)
	}
	if v.DstX != 100 || v.DstY != 100 || math.Abs(float64(v.ColorR-0.5)) > 0.01 || math.Abs(float64(v.ColorB-1)) > 0.01 {
		t.Errorf("the backdrop reaches (%v, %v) in %v %v %v, want the whole screen in the sky's colour", v.DstX, v.DstY, v.ColorR, v.ColorG, v.ColorB)
	}
}
