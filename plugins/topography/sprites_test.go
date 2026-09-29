package topography_test

import (
	"os"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// magenta is an atlas of one sprite, 8 pixels of magenta.
type magenta struct{ img *render.Image }

func (a *magenta) Atlas() *render.Image {
	if a.img == nil {
		a.img = render.NewImage(8, 8)
		pix := make([]byte, 4*64)
		for i := 0; i < len(pix); i += 4 {
			pix[i], pix[i+1], pix[i+2], pix[i+3] = 255, 0, 255, 255
		}
		a.img.WritePixels(pix)
	}
	return a.img
}
func (*magenta) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 8, 8 }
func (*magenta) White() (u, v float32)                           { return 4, 4 }

// purple reports whether the pixel is the magenta sprite's, in whatever light: red and blue, no
// green.
func purple(px []byte) bool { return px[0] > 60 && px[2] > 60 && px[1] < px[0]/4 }

// Drawn on the GPU in the isometric view, a billboard standing behind a hill is hidden by it, one
// standing on the hill's slope shows, and its shadow darkens the ground beside it.
func TestSprites_TheHillHidesWhatStandsBehindIt(t *testing.T) {
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Camera:   camera.Config{ViewportWidth: 320, ViewportHeight: 240},
		Heights:  true,
	})
	grid := board.DefaultGrids{}.Square(8, 8, 32)
	b := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	b.Res.Logic.Board.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	p.Relief().SetHeights(topography.MeanOfCells(grid, func(c board.CellID) float64 {
		if x, y, _ := grid.Coords(c); x >= 3 && x <= 4 && y >= 3 && y <= 4 {
			return 90
		}
		return 0
	}))
	cam := w.Camera()
	cam.CenterOn(128, 128, 0)
	picker := cam.(interface {
		Pick(sx, sy float32) (float32, float32, bool)
	})
	// down the middle of the screen, a point seeing the hill's slope with the level ground far behind
	var seen [][2]geom.Vec // the hill's slope and the level ground behind it, under one screen point
	vw, vh := cam.Viewport()
	for sy := float32(0); sy < vh; sy++ {
		hx, hy, ok := picker.Pick(vw/2, sy)
		if !ok || p.Relief().GroundAt(geom.NewVec(float64(hx), float64(hy))) < 20 {
			continue
		}
		ux, uy := cam.Unproject(vw/2, sy, 0)
		if dx, dy := ux-hx, uy-hy; dx*dx+dy*dy > 60*60 {
			seen = append(seen, [2]geom.Vec{geom.NewVec(float64(hx), float64(hy)), geom.NewVec(float64(ux), float64(uy))})
		}
	}
	if len(seen) == 0 {
		t.Fatal("no point of the screen sees the hill with level ground behind it")
	}
	slope, behind := seen[len(seen)/2][0], seen[len(seen)/2][1]
	screen := render.NewImage(int(vw), int(vh))
	depth := render.NewDepth()
	pix := make([]byte, 4*int(vw)*int(vh))
	atlas := &magenta{}
	u := render.UniformsOf(map[string]any{"Sun": []float32{0.4, 0.3, 0.8}, "SunStrength": []float32{1}, "SunColor": []float32{1, 1, 1}, "Ambience": []float32{0.3, 0.3, 0.3}})
	look := w.Look().(world.DirectLook)
	// draw lays the ground and, where at is not nil, a sprite 6 wide standing there, and reads the
	// screen: how many pixels are the sprite's, purple
	draw := func(at *geom.Vec) (seen int) {
		screen.Clear()
		screen.ClearDepth(depth)
		target := render.Target{Screen: screen, Depth: depth}
		p.Renderer().(render.Direct).Draw(target, cam, u)
		var f render.Frame
		f.Reset(cam)
		look.Begin(cam)
		if at != nil {
			box := plane.NewAABB(geom.NewVec(at.X-3, at.Y-3), 6, 6)
			look.Sprite(&f, cam, box, world.Z{Altitude: p.Relief().GroundAt(*at), Height: 6}, atlas, 0, render.Light{1, 1, 1}, 0)
		}
		look.DrawSprites(target, cam, u)
		screen.ReadPixels(pix)
		for i := 0; i < len(pix); i += 4 {
			if purple(pix[i:]) {
				seen++
			}
		}
		return seen
	}
	if n := draw(&behind); n != 0 {
		t.Errorf("a sprite behind the hill shows %d pixels, want it hidden", n)
	}
	bare := append([]byte(nil), pix[:0]...)
	draw(nil)
	bare = append(bare, pix...)
	n := draw(&slope)
	if n == 0 {
		t.Fatal("a sprite on the hill's slope does not show")
	}
	darker := 0
	for i := 0; i < len(pix); i += 4 {
		if !purple(pix[i:]) && int(pix[i])+int(pix[i+1])+int(pix[i+2]) < int(bare[i])+int(bare[i+1])+int(bare[i+2])-6 {
			darker++
		}
	}
	if darker == 0 {
		t.Error("the sprite on the slope casts no shadow on the ground")
	}
}
