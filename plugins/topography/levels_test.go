package topography_test

import (
	"fmt"
	"image/color"
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

func TestSettings_TheHazeAloneOnTheSettingsScreen(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	settings := p.Settings()
	if len(settings) != 1 {
		t.Fatalf("%d settings, want the haze alone", len(settings))
	}
	if haze := settings[0].(control.Switch); haze.Label != "Haze over the ground" ||
		haze.Section != control.Graphics {
		t.Errorf("the setting is %q under %q, want the haze under the graphics", haze.Label,
			haze.Section)
	}
	for _, bd := range p.DefaultBindings() {
		if bd.Type() == reflect.TypeFor[topography.Haze]() {
			t.Errorf("%v is bound to %v, want the settings screen alone", bd.Type(),
				players.Written(bd.Trigger))
		}
	}
}

func TestReporter_SaysNothingInAWorldOfNoSectors(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	p.Read(0).Reporter().Report(func(label, value string) {
		t.Errorf("the panel says %s: %q, want nothing without sectors", label, value)
	})
}

var landColours = []color.RGBA{
	{R: 90, G: 140, B: 70, A: 255},
	{R: 200, G: 180, B: 110, A: 255},
	{R: 120, G: 120, B: 130, A: 255},
}

// a camera from above seeing the sectors of each column of 128 a step coarser than the column
// before: each world unit a pixel, then 0.3, 0.1 and 0.03 of one
type stepped struct{ camera.Camera }

func (s stepped) ScaleAt(x, _, _ float32) float32 {
	return [4]float32{1, 0.3, 0.1, 0.03}[min(max(int(x/128), 0), 3)]
}

func (s stepped) Rays() (camera.RayField, bool) { return s.Camera.(camera.Rays).Rays() }

func (s stepped) Observers(each func(camera.Camera)) { each(s) }

var _ sector.Observers = stepped{}

// a world of 4 by 4 sectors of 128, L0 down to L3 by column as the camera sees them, the ground
// over a board on g of kinds of colours in turn, gridded in line where it has a width, drawn,
// read back and swapped
func drawnAtLevels(t *testing.T, g grid.Grid, line board.Line) (*topography.Plugin, []byte) {
	t.Helper()
	const size = 512
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: size, Height: size},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 4, MaxSize: 4},
		Heights:  true,
		Sectors:  world.SectorsCfg{Buckets: 4},
		LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0.5, 0.2, 0.05, 0}},
	})
	b := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	b.Write().ShowGrid(line.Width > 0)
	kinds := make([]cell.Kind, len(landColours))
	for k := range landColours {
		name := fmt.Sprintf("land%d", k)
		b.Read(0).CellKinds().Define(name, cell.Kind{Pace: 1, Allows: cell.Land})
		kinds[k], _ = b.Read(0).CellKinds().Get(name)
	}
	g.EachCell(func(c cell.ID) {
		x, y, _ := g.Coords(c)
		b.Write().Board().Set(c, kinds[(int(x)+2*int(y))%len(kinds)])
	})
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	a := b.NewAtlas(8)
	for k, c := range landColours {
		a.Add(fmt.Sprintf("land%d", k), render.Solid(c))
	}
	a.Grid(line)
	a.Close()
	ctx := topotest.NewInstallCtx()
	for _, plugin := range []interface {
		Install(plugin.Installer) error
	}{w, b, p} {
		if err := plugin.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	ecs := ctx.ECS()
	ecs.Setup(ctx.Systems()...)
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		b.RunPlan(rc, d)
		p.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	cam := stepped{cameratest.NewFromSpace(size, size, 0)}
	w.Write().Observe(cam)
	w.Write().NewView(cam)
	ecs.Tick(time.Second / 60)
	ecs.Tick(time.Second / 60)
	screen := render.NewImage(size, size)
	defer screen.Deallocate()
	render.NewComposer(ctx.Sources()...).DrawWorld(screen, cam)
	pix := make([]byte, 4*size*size)
	screen.ReadPixels(pix)
	if got := groundReport(p); got != "L0 0\nL1 0\nL2 0\nL3 0" {
		t.Errorf("drawn, not yet swapped, the panel says %q, want the levels of no frame", got)
	}
	p.Swap()
	return p, pix
}

// the columns and rows of the sectors of 128 the middles of g's cells lie in
func sectorsIn(g grid.Grid) map[[2]int]bool {
	in := map[[2]int]bool{}
	g.EachCell(func(c cell.ID) {
		m := g.CellCenter(c)
		in[[2]int{min(int(m.X/128), 3), min(int(m.Y/128), 3)}] = true
	})
	return in
}

func groundReport(p *topography.Plugin) string {
	var got string
	p.Read(0).Reporter().Report(func(label, value string) {
		if label == "Ground" {
			got = value
		}
	})
	return got
}

func TestReporter_SaysHowManySectorsTheFrameDrawnDrewAtEachLevel(t *testing.T) {
	gputest.Need(t)
	// the square board covers every sector, the hex one, a rhombus, those its cells' middles lie
	// in, a column's at its level
	for _, tc := range []struct {
		name string
		grid grid.Grid
		want string
	}{
		{"square", grid.DefaultGrids{}.Square(16, 16, 32), "L0 4\nL1 4\nL2 4\nL3 4"},
		{"hex", grid.DefaultGrids{}.Hex(12, 14, 16), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.want
			if want == "" {
				var n [lod.L4]int
				for at := range sectorsIn(tc.grid) {
					n[at[0]]++
				}
				want = fmt.Sprintf("L0 %d\nL1 %d\nL2 %d\nL3 %d", n[0], n[1], n[2], n[3])
			}
			p, _ := drawnAtLevels(t, tc.grid, board.Line{})
			if got := groundReport(p); got != want {
				t.Errorf("swapped, the panel says %q, want %q: each column of sectors at its "+
					"level", got, want)
			}
		})
	}
}

func TestHexes_DrawTheSectorsAtL3AsASheetOfTheTopsFromAbove(t *testing.T) {
	gputest.Need(t)
	g := grid.DefaultGrids{}.Hex(12, 14, 16)
	p, pix := drawnAtLevels(t, g, board.Line{})
	in := sectorsIn(g)
	top, perUnit := p.Write().Miniature()
	if top == nil || perUnit != 2 {
		t.Fatalf("the tops from above %v, %v pixels a unit; want them, 2 a unit", top, perUnit)
	}
	w, h := top.Bounds().Dx(), top.Bounds().Dy()
	tops := make([]byte, 4*w*h)
	top.ReadPixels(tops)
	texel := func(x, y int) color.RGBA {
		i := 4 * (y*w + x)
		return color.RGBA{tops[i], tops[i+1], tops[i+2], tops[i+3]}
	}
	// in the white light the frame holds without a light plugin, a pixel of the last column's
	// sectors holding cells, a world unit, is the four texels of the tops round its middle;
	// where they are one colour, that colour
	checked := 0
	for y := range 512 {
		for x := 384; x < 512; x++ {
			c := texel(2*x, 2*y)
			if !in[[2]int{3, y / 128}] {
				continue
			}
			if c.A != 255 || texel(2*x+1, 2*y) != c || texel(2*x, 2*y+1) != c ||
				texel(2*x+1, 2*y+1) != c {
				continue
			}
			i := 4 * (y*512 + x)
			if got := (color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}); !nearColour(got, c, 2) {
				t.Fatalf("the sheet at (%d, %d) is %v, the tops' %v", x, y, got, c)
			}
			checked++
		}
	}
	if checked < 1000 {
		t.Errorf("%d pixels of the sheet checked, want the board's in the last column", checked)
	}
}

func nearColour(a, b color.RGBA, by int) bool {
	d := func(x, y byte) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) <= by && d(a.G, b.G) <= by && d(a.B, b.B) <= by && d(a.A, b.A) <= by
}

func TestHexes_DrawTheGridAtL0Alone(t *testing.T) {
	gputest.Need(t)
	magenta := color.RGBA{R: 255, B: 255, A: 255}
	_, pix := drawnAtLevels(t, grid.DefaultGrids{}.Hex(12, 14, 16),
		board.Line{Color: magenta, Width: 3})
	// by column, clear of the cells of the column before reaching over its edge, a hex's half
	var lined [4]int
	for y := range 512 {
		for x := range 512 {
			if x%128 < 16 {
				continue
			}
			i := 4 * (y*512 + x)
			if nearColour(color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}, magenta, 2) {
				lined[x/128]++
			}
		}
	}
	if lined[0] == 0 || lined[1] != 0 || lined[2] != 0 || lined[3] != 0 {
		t.Errorf("pixels of the grid's line by column of sectors, L0 to L3: %v; want some at L0 "+
			"alone", lined)
	}
}
