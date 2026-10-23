package painter

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// a hex board's tops from above, laid on the GPU, as the CPU composed them: the tiles, the still
// water glinting, the ways curving, fading, turning into the water and running into it
func TestPainter_DrawsAHexBoardsTopsAsTheCPUComposedThem(t *testing.T) {
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Hex(8, 8, 16)
	brd, brdW := newBoard(g)
	land := styled(st, cell.Kind{Name: cell.Named("h1"), Allows: cell.Land, SpriteID: 1}, Style{})
	meadow := styled(st, cell.Kind{Name: cell.Named("h2"), Allows: cell.Land, SpriteID: 2},
		Style{})
	sea := styled(st, cell.Kind{Name: cell.Named("h3"), Allows: cell.Water, SpriteID: 3},
		Style{Under: true, Shine: 0.9})
	river := styled(st, cell.Kind{Name: cell.Named("h4"), Allows: cell.Land | cell.Water,
		SpriteID: 4}, Style{Shine: 0.9, Flow: 22, MixWith: "h3"})
	road := styled(st, cell.Kind{Name: cell.Named("h5"), Allows: cell.Land, SpriteID: 5},
		Style{})
	for r := range uint32(8) {
		for q := range uint32(8) {
			k := land
			switch {
			case r >= 6:
				k = sea
			case (q+r)%3 == 0:
				k = meadow
			}
			brdW.Set(g.At(q, r), k)
		}
	}
	east, west := cell.Links(1<<0), cell.Links(1<<3)
	southEast, northWest := cell.Links(1<<5), cell.Links(1<<2)
	for q := range uint32(7) {
		brdW.SetWay(g.At(q, 1), cell.Way{Kind: road, Width: 6, Links: east | west})
	}
	brdW.SetWay(g.At(7, 1), cell.Way{Kind: road, Width: 6, Links: west, Fade: 0.6})
	for r := range uint32(6) {
		links := southEast | northWest
		if r == 0 {
			links = southEast
		}
		brdW.SetWay(g.At(2, r), cell.Way{Kind: river, Width: 7, Links: links,
			Mix: float32(r) / 5})
	}
	brdW.SetWay(g.At(4, 3), cell.Way{Kind: road, Width: 5, Links: east | west | southEast})
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 40 - p.Y/8 })
	atlas := render.NewAtlas()
	for id, c := range map[render.SpriteID]color.RGBA{1: {90, 150, 70, 255},
		2: {150, 170, 80, 255}, 3: {40, 80, 170, 255}, 4: {60, 110, 190, 255},
		5: {150, 110, 70, 255}} {
		atlas.Add(id, 28, render.Hexagon(c))
	}
	atlas.Close()
	space := world.SpaceCfg{Width: 260, Height: 200}
	const px = 2
	w, h := int(space.Width*px), int(space.Height*px)
	u := render.UniformsOf(map[string]any{"Pixel": []float32{1.0 / px}, "Clock": []float32{1.3},
		"Toward": []float32{0, 0, 1}})
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
	d.kinds = kindsOf{land, meadow, sea, river, road}
	mp := testMap{look: look.FlatLook(), d: d}

	cpu := render.NewImage(w, h)
	still := render.NewStill()
	still.Compose(float32(space.Width), float32(space.Height),
		look.NewRenderer(brd, atlas, mp, world.SpaceCfg{}).Compose)
	still.Draw(render.Target{Screen: cpu}, nil, u, px, 0, 0, render.Light{1, 1, 1})

	laid := render.NewImage(w, h)
	d.Above()
	look.NewRenderer(brd, atlas, mp, space).Whole(laid, px, render.Light{1, 1, 1}, u)
	d.DrawWays(laid, atlas, px, u)
	if !d.WaysMoving() {
		t.Error("the river's water neither runs nor glints")
	}

	a, b := make([]byte, 4*w*h), make([]byte, 4*w*h)
	laid.ReadPixels(a)
	cpu.ReadPixels(b)
	off, most := 0, 0
	for i := 0; i < len(a); i += 4 {
		worst := 0
		for k := range 4 {
			worst = max(worst, int(a[i+k])-int(b[i+k]), int(b[i+k])-int(a[i+k]))
		}
		most = max(most, worst)
		if worst > 24 {
			off++
		}
	}
	t.Logf("%d pixels differ by over 24, %d at most", off, most)
	if dir := os.Getenv("GRAM_TILES_PNG"); dir != "" {
		for name, pix := range map[string][]byte{"gpu": a, "cpu": b} {
			f, _ := os.Create(dir + "/" + t.Name() + "-" + name + ".png")
			_ = png.Encode(f, &image.RGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)})
			f.Close()
		}
	}
	if off > w*h/200 {
		t.Errorf("%d pixels differ from the tops composed on the CPU by over 24, %d at most; want "+
			"at most half a percent of them", off, most)
	}
}
