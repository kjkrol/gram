package draw_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/kjkrol/gram/entity/tag"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/internal/draw"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
	"github.com/kjkrol/gram/rule/effect"
)

// A cell alone under an effect weighs a quarter at its corners, a half at its sides, one at its
// middle; a cell among others under it weighs one everywhere, and one beside them what they share.
func TestWeigh_IsTheShareOfTheCellsMeetingAtEachPoint(t *testing.T) {
	alone := draw.Weigh([3][3]bool{{}, {false, true, false}, {}})
	if want := [3][3]float32{{0.25, 0.5, 0.25}, {0.5, 1, 0.5}, {0.25, 0.5, 0.25}}; alone != want {
		t.Errorf("a cell alone weighs %v, want %v", alone, want)
	}
	all := [3][3]bool{{true, true, true}, {true, true, true}, {true, true, true}}
	if w := draw.Weigh(all); w[0][0] != 1 || w[1][1] != 1 || w[2][2] != 1 {
		t.Errorf("a cell among others weighs %v, want one everywhere", w)
	}
	beside := draw.Weigh([3][3]bool{{true, false, false}, {true, false, false}, {true, false, false}})
	if want := [3][3]float32{{0.5, 0, 0}, {0.5, 0, 0}, {0.5, 0, 0}}; beside != want {
		t.Errorf("a cell beside a column weighs %v, want %v", beside, want)
	}
}

// A quarter is drawn only where the cover shows somewhere on it.
func TestQuarters_LeaveOutWhereTheCoverShowsNowhere(t *testing.T) {
	if q := draw.Quarters([3][3]float32{}, 0.2); len(q) != 0 {
		t.Errorf("a tile nobody weighs has %d quarters, want none", len(q))
	}
	corner := [3][3]float32{{0.75, 0.5, 0.25}, {0.5, 0, 0}, {0.25, 0, 0}}
	q := draw.Quarters(corner, 0.2)
	if len(q) != 3 {
		t.Fatalf("a tile weighed at one corner has %d quarters, want the three its weight reaches", len(q))
	}
	if q[0].Part != [4]float32{0, 0, 0.5, 0.5} || q[0].Weight != [4]float32{0.75, 0.5, 0.5, 0} {
		t.Errorf("the first quarter is %+v, want the top-left weighed 0.75, 0.5, 0.5, 0", q[0])
	}
}

// under is a board some of whose cells lie under an effect.
type under struct {
	*board.Board
	mark  tag.Tag[effect.States]
	cells map[cell.ID]bool
}

func (u under) States(c cell.ID) tag.Tags[effect.States] {
	if u.cells[c] {
		return tag.Tags[effect.States](0).With(u.mark)
	}
	return 0
}

// covered is a flat map dressed by d.
type covered struct{ d *draw.Bands }

func (m covered) Look() look.Look                               { return look.FlatLook() }
func (covered) Heights() ground.Heights                         { return nil }
func (m covered) Dressing() look.Dressing                       { return m.d }
func (covered) Top(cell.ID) (corners [4]float32, level float32) { return corners, 0 }

// A cover lies over the cells under its effect along a line of its own: the middle of a covered
// cell is the cover's colour, the middle of a bare one its kind's, and at the outer corner of a
// block of covered cells the kind shows through — the line cuts the corner, it does not follow the
// cells' edges.
func TestCover_LiesAlongALineOfItsOwn(t *testing.T) {
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
	grass := cell.Kind{SpriteID: 1, Cost: 1, Allows: cell.Land, Color: color.RGBA{R: 60, G: 160, B: 60, A: 255}}
	snow := color.RGBA{R: 240, G: 240, B: 250, A: 255}
	const mark tag.Tag[effect.States] = 3
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	brd := board.NewBoard(g)
	brd.SetAll(grass)
	u := under{Board: brd, mark: mark, cells: map[cell.ID]bool{}}
	for _, at := range [][2]uint32{{2, 2}, {3, 2}, {2, 3}, {3, 3}, {4, 3}, {4, 4}, {5, 5}} {
		c := g.CellIndex(at[0], at[1])
		u.cells[c] = true
	}
	atlas := render.NewAtlas()
	atlas.Add(1, 32, render.Solid(grass.Color))
	atlas.Add(2, 32, render.Solid(snow))
	atlas.Close()
	d := draw.NewBands(u)
	d.Cover(mark, 2)
	space := world.SpaceCfg{Width: 256, Height: 256}
	r := look.NewRenderer(brd, atlas, covered{d}, space)
	cam := icamera.NewFromSpace(space.Width, space.Height, 0)
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
	if c := at(6*32+16, 1*32+16); !near(c, grass.Color) {
		t.Errorf("the middle of a bare cell is %v, want its kind's %v", c, grass.Color)
	}
	if c := at(2*32+2, 2*32+2); !near(c, grass.Color) {
		t.Errorf("the outer corner of the covered block is %v, want the kind's %v: the line cuts the corner", c, grass.Color)
	}
	if c := at(3*32, 2*32+16); !near(c, snow) {
		t.Errorf("the edge between two covered cells is %v, want the cover's %v unbroken", c, snow)
	}
}
