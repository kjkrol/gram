package billboards_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/billboards"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

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

// the magenta sprite's pixel in whatever light: red and blue, no green
func purple(px []byte) bool { return px[0] > 60 && px[2] > 60 && px[1] < px[0]/4 }

type aHill struct {
	relief        topography.Relief
	look          world.DirectLook
	cam           camera.Camera
	slope, behind geom.Vec
	draw          func(lay func(f *render.Frame)) []byte
}

func newHill(t *testing.T) aHill {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	grid := grid.DefaultGrids{}.Square(8, 8, 32)
	b := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	b.Write().Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	terrain := topotest.Drawn(t, p)[0]
	p.Write().Relief().SetHeights(ground.MeanOfCells(grid, func(c cell.ID) float32 {
		if x, y, _ := grid.Coords(c); x >= 3 && x <= 4 && y >= 3 && y <= 4 {
			return 90
		}
		return 0
	}))
	cam := cameras.NewPlugin(w, cameras.Config{
		Turn:       true,
		Tilt:       true,
		Cell:       32,
		HeightUnit: 1,
		Cage:       p.Read(0),
	}).Write().New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(320, 240)
	cam.MoveTo(0, 0)
	cam.CenterOn(128, 128, 0)
	picker := cam.(interface {
		Pick(sx, sy float32) (float32, float32, float32, bool)
	})
	var seen [][2]geom.Vec // the hill's slope and the level ground behind it, under one screen point
	vw, vh := cam.Viewport()
	for sy := float32(0); sy < vh; sy++ {
		hx, hy, _, ok := picker.Pick(vw/2, sy)
		if !ok || p.Write().Relief().At(geom.NewVec(float32(hx), float32(hy))) < 20 {
			continue
		}
		ux, uy := cam.Unproject(vw/2, sy, 0)
		if dx, dy := ux-hx, uy-hy; dx*dx+dy*dy > 60*60 {
			seen = append(seen, [2]geom.Vec{geom.NewVec(float32(hx), float32(hy)), geom.NewVec(float32(ux), float32(uy))})
		}
	}
	if len(seen) == 0 {
		t.Fatal("no point of the screen sees the hill with level ground behind it")
	}
	screen := render.NewImage(int(vw), int(vh))
	depth := render.NewDepth()
	pix := make([]byte, 4*int(vw)*int(vh))
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0.4, 0.3, 0.8}, "LightStrength": []float32{1}, "LightColor": []float32{1, 1, 1}, "Ambience": []float32{0.3, 0.3, 0.3}})
	look := w.Read(0).Look().(world.DirectLook)
	h := aHill{relief: p.Write().Relief(), look: look, cam: cam}
	h.slope, h.behind = seen[len(seen)/2][0], seen[len(seen)/2][1]
	h.draw = func(lay func(f *render.Frame)) []byte {
		screen.Clear()
		screen.ClearDepth(depth)
		target := render.Target{Screen: screen, Depth: depth}
		terrain.(render.Direct).Draw(target, cam, u)
		var f render.Frame
		f.Reset(cam)
		look.Begin(&render.Frame{}, cam)
		if lay != nil {
			lay(&f)
		}
		look.DrawSprites(target, cam, u)
		screen.ReadPixels(pix)
		return pix
	}
	return h
}

func purples(pix []byte) (n int) {
	for i := 0; i < len(pix); i += 4 {
		if purple(pix[i:]) {
			n++
		}
	}
	return n
}

func TestSprites_TheHillHidesWhatStandsBehindIt(t *testing.T) {
	gputest.Need(t)
	h := newHill(t)
	atlas := &magenta{}
	stand := func(at geom.Vec) func(f *render.Frame) {
		return func(f *render.Frame) {
			box := plane.NewAABB(geom.NewVec(at.X-3, at.Y-3), 6, 6)
			z := world.Z{Altitude: h.relief.At(at), Height: 6}
			h.look.Sprite(f, h.cam, box, z, atlas, render.Appearance{}, render.WhiteLight)
		}
	}
	if n := purples(h.draw(stand(h.behind))); n != 0 {
		t.Errorf("a sprite behind the hill shows %d pixels, want it hidden", n)
	}
	if purples(h.draw(stand(h.slope))) == 0 {
		t.Fatal("a sprite on the hill's slope does not show")
	}
}

func TestSprites_ALyingSpriteLiesOnTheGround(t *testing.T) {
	gputest.Need(t)
	h := newHill(t)
	atlas := &magenta{}
	lie := func(at geom.Vec, alt float32) func(f *render.Frame) {
		return func(f *render.Frame) {
			box := plane.NewAABB(geom.NewVec(at.X-8, at.Y-8), 16, 16)
			lies := h.look.(world.LyingLook)
			lies.Lie(f, h.cam, box, world.Z{Altitude: alt}, atlas, render.Appearance{},
				render.WhiteLight)
		}
	}
	if n := purples(h.draw(lie(h.behind, 0))); n != 0 {
		t.Errorf("a deck lying behind the hill shows %d pixels, want it hidden", n)
	}
	before := geom.NewVec(h.behind.X, 256-h.behind.Y)
	if n := purples(h.draw(lie(before, 0))); n == 0 {
		t.Error("a deck lying on the level ground before the hill does not show")
	}
	if n := purples(h.draw(lie(h.slope, 0))); n == 0 {
		t.Error("a deck lying under the slope does not show on it")
	}
	corners := billboards.Lain(h.look)
	if len(corners) != 4 {
		t.Fatalf("took %d corners lying, want the deck's 4", len(corners))
	}
	for _, k := range corners {
		g := h.relief.At(geom.NewVec(float32(k[0]), float32(k[1])))
		if float32(k[2]) < g-0.01 || k[2] <= 0 {
			t.Errorf("a corner of the deck under the slope lies at %v over ground at %.1f, "+
				"want on it", k, g)
		}
	}
}
