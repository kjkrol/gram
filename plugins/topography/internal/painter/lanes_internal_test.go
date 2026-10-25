package painter

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// a hex board's tops from above, laid on the GPU: the tiles, the still water glinting, the ways
// over them, fading and the river turning into the sea
func TestPainter_DrawsAHexBoardsTopsOnTheGPU(t *testing.T) {
	gputest.Need(t)
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
	reliefFor(brd).SetHeights(func(p geom.Vec) float32 { return 40 - p.Y/8 })
	colours := map[render.SpriteID]color.RGBA{1: {90, 150, 70, 255}, 2: {150, 170, 80, 255},
		3: {40, 80, 170, 255}, 4: {60, 110, 190, 255}, 5: {150, 110, 70, 255}}
	atlas := render.NewAtlas()
	for id, c := range colours {
		atlas.Add(id, 28, render.Hexagon(c))
	}
	atlas.Close()
	space := world.SpaceCfg{Width: 260, Height: 200}
	const px = 2
	w, h := int(space.Width*px), int(space.Height*px)
	u := render.UniformsOf(map[string]any{"Pixel": []float32{1.0 / px}, "Clock": []float32{1.3},
		"Toward": []float32{0, 0, 1}})
	d := New(brd, fixed(reliefFor(brd)), true, st)
	d.kinds = kindsOf{land, meadow, sea, river, road}
	top := render.NewImage(w, h)
	d.Above()
	look.NewRenderer(brd, atlas, testMap{look: look.Flat, d: d}, space).
		Whole(top, px, render.Light{1, 1, 1}, u)
	d.DrawWays(top, atlas, px, u)
	if !d.WaysMoving() {
		t.Error("the river's water neither runs nor glints")
	}
	pix := make([]byte, 4*w*h)
	top.ReadPixels(pix)
	if dir := os.Getenv("GRAM_TILES_PNG"); dir != "" {
		f, _ := os.Create(dir + "/" + t.Name() + ".png")
		_ = png.Encode(f, &image.RGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)})
		f.Close()
	}
	at := func(q, r uint32) color.RGBA {
		mid := brd.CellCenter(g.At(q, r))
		i := 4 * (int(mid.Y*px)*w + int(mid.X*px))
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	close := func(a, b color.RGBA, d int) bool {
		for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
			if k := int(p[0]) - int(p[1]); k > d || k < -d {
				return false
			}
		}
		return true
	}
	for _, c := range []struct {
		q, r uint32
		want color.RGBA
		what string
	}{
		{6, 4, colours[1], "a cell of land"}, {6, 3, colours[2], "a meadow"},
		{5, 1, colours[5], "the road across the land"},
	} {
		if got := at(c.q, c.r); !close(got, c.want, 2) {
			t.Errorf("the middle of %s is %v, want %v", c.what, got, c.want)
		}
	}
	if got := at(7, 1); close(got, colours[5], 10) || close(got, colours[1], 10) {
		t.Errorf("the road fading out is %v, want it between the road's %v and the land's %v", got,
			colours[5], colours[1])
	}
	if got := at(2, 4); got.B < got.R+60 {
		t.Errorf("the river is %v, want its water blue", got)
	}
	glints := false
	for q := range uint32(8) {
		glints = glints || !close(at(q, 7), colours[3], 10)
	}
	if !glints {
		t.Errorf("the sea is drawn as its sprite alone, %v, want it glinting", colours[3])
	}
}

// the ways laid in as many pieces as the most any of them lays, every piece there: as laid in as
// many as any way could, a stem through eight with six joining it, all running into water
func TestPainter_LaysEveryPieceOfTheWays(t *testing.T) {
	gputest.Need(t)
	for _, g := range []grid.Grid{grid.DefaultGrids{}.Square(16, 12, 32),
		grid.DefaultGrids{}.Hex(10, 8, 16)} {
		d, _, atlas := wayBoard(t, g)
		const px = 2
		w, h := 2*16*32, 2*12*32
		u := render.UniformsOf(map[string]any{"Pixel": []float32{1.0 / px},
			"Clock": []float32{1.3}, "Toward": []float32{0, 0, 1}})
		laid, all := render.NewImage(w, h), render.NewImage(w, h)
		d.Above()
		d.DrawWays(laid, atlas, px, u)
		if most := d.baking.most; most <= 0 || most >= 46 {
			t.Fatalf("the ways lay %d pieces at most, want fewer than the 46 any could", most)
		}
		d.baking.opts.Vertices = 6 * 46
		all.DrawMesh(nil, topsShader, &d.baking.opts)
		a, b := make([]byte, 4*w*h), make([]byte, 4*w*h)
		laid.ReadPixels(a)
		all.ReadPixels(b)
		off := 0
		for i := 0; i < len(a); i += 4 {
			if [4]byte(a[i:i+4]) != [4]byte(b[i:i+4]) {
				off++
			}
		}
		if off > 0 {
			t.Errorf("%d pixels of the ways laid in %d pieces at most differ from them laid in 46",
				off, d.baking.most)
		}
	}
}
