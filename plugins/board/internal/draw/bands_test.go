package draw_test

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/draw"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

func TestBands_AWayIsThePartOfItsKindsSpriteUnderIt(t *testing.T) {
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	road := cell.Kind{SpriteID: 2, Cost: 1, Allows: cell.Land}
	green := color.RGBA{R: 60, G: 160, B: 60, A: 255}
	red := color.RGBA{R: 220, G: 40, B: 40, A: 255}
	blue := color.RGBA{R: 40, G: 60, B: 220, A: 255}
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(grass)
	for y := range uint32(8) {
		up, _ := grid.Link(g, g.At(3, y), g.At(3, (y+7)%8))
		down, _ := grid.Link(g, g.At(3, y), g.At(3, (y+1)%8))
		brdW.SetWay(g.At(3, y), cell.Way{Kind: road, Width: 8, Links: up | down})
	}
	atlas := render.NewAtlas()
	atlas.Add(grass.SpriteID, 32, render.Solid(green))
	atlas.Add(road.SpriteID, 32, func(dst *render.Canvas, size int) {
		dst.Fill(blue)
		dst.FillRect(0, 0, float32(size)/4, float32(size), red)
	})
	atlas.Close()
	space := world.SpaceCfg{Width: 256, Height: 256}
	r := look.NewRenderer(brd, atlas, covered{draw.NewBands(brd)}, space)
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(256, 256)
	screen := render.NewImage(256, 256)
	render.NewComposer(r).DrawWorld(screen, cam)
	pix := make([]byte, 4*256*256)
	screen.ReadPixels(pix)
	at := func(x, y int) color.RGBA {
		i := 4 * (y*256 + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	near := func(a, b color.RGBA) bool {
		for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
			if d := int(p[0]) - int(p[1]); d > 3 || d < -3 {
				return false
			}
		}
		return true
	}
	for _, x := range []int{3*32 + 13, 3*32 + 16, 3*32 + 19} {
		if c := at(x, 4*32+8); !near(c, blue) {
			t.Errorf("the road at x %d is %v, want the blue under it %v", x, c, blue)
		}
	}
	if c := at(3*32+4, 4*32+8); !near(c, green) {
		t.Errorf("the grass beside the road is %v, want %v", c, green)
	}
}
