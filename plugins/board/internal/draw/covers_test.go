package draw_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/internal/draw"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type under struct {
	*board.BoardRead
	mark  tag.Tag[effect.States]
	cells map[cell.ID]bool
}

func (u under) States(c cell.ID) tag.Tags[effect.States] {
	if u.cells[c] {
		return tag.Tags[effect.States](0).With(u.mark)
	}
	return 0
}

type covered struct{ d *draw.Bands }

func (m covered) Look() look.Look                               { return look.Flat }
func (covered) Heights() ground.Heights                         { return nil }
func (m covered) Dressing() look.Dressing                       { return m.d }
func (covered) Top(cell.ID) (corners [4]float32, level float32) { return corners, 0 }

func TestCover_LiesAlongALineOfItsOwn(t *testing.T) {
	gputest.Need(t)
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land}
	green := color.RGBA{R: 60, G: 160, B: 60, A: 255}
	snow := color.RGBA{R: 240, G: 240, B: 250, A: 255}
	const mark tag.Tag[effect.States] = 3
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(grass)
	u := under{BoardRead: brd, mark: mark, cells: map[cell.ID]bool{}}
	for _, at := range [][2]uint32{{2, 2}, {3, 2}, {2, 3}, {3, 3}, {4, 3}, {4, 4}, {5, 5}} {
		c := g.At(at[0], at[1])
		u.cells[c] = true
	}
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 32, render.Solid(green))
	atlas.Add(render.SpriteID(2), 32, render.Solid(snow))
	atlas.Close()
	d := draw.NewBands(u)
	d.Cover(mark, 2)
	space := world.SpaceCfg{Width: 256, Height: 256}
	r := look.NewRenderer(brd, atlas, covered{d}, space)
	cam := cameratest.NewFromSpace(space.Width, space.Height, 0)
	cam.SetViewport(256, 256)
	screen := render.NewImage(256, 256)
	render.NewComposer(r).DrawWorld(screen, cam)
	pix := make([]byte, 4*256*256)
	screen.ReadPixels(pix)
	if path := os.Getenv("GRAM_COVER_PNG"); path != "" {
		img := &image.RGBA{Pix: pix, Stride: 4 * 256, Rect: image.Rect(0, 0, 256, 256)}
		f, _ := os.Create(path)
		_ = png.Encode(f, img)
		f.Close()
	}
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
	if c := at(2*32+16, 2*32+16); !near(c, snow) {
		t.Errorf("the middle of a covered cell is %v, want the cover's %v", c, snow)
	}
	if c := at(6*32+16, 1*32+16); !near(c, green) {
		t.Errorf("the middle of a bare cell is %v, want its kind's %v", c, green)
	}
	if c := at(2*32+2, 2*32+2); !near(c, green) {
		t.Errorf("the outer corner of the covered block is %v, want the kind's %v: the line cuts the corner", c, green)
	}
	if c := at(3*32, 2*32+16); !near(c, snow) {
		t.Errorf("the edge between two covered cells is %v, want the cover's %v unbroken", c, snow)
	}
}
