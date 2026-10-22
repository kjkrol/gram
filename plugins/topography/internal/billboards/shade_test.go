package billboards_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// the sun 60° up in the west
var westSun = light.Distant{Dir: [3]float32{-0.5, 0, 0.866}, Strength: 1, Ambient: 0.3}

type westSky struct{}

func (westSky) Distant() light.Distant { return westSun }
func (westSky) Air() air.Weather       { return air.Weather{} }

// hands the frame the west sun before the ground composes
type sunning struct{}

func (sunning) Init(*goke.SysInit) {}
func (sunning) Compose(f *render.Frame, _ camera.Camera) {
	westSun.Frame(f)
	air.Weather{}.Frame(f, westSun, 0, 0)
}
func (sunning) Draw(render.Target, camera.Camera, render.Uniforms) {}

// one magenta figure 6 tall standing on the ground at, lit as lit says
type standing struct {
	look   world.DirectLook
	relief topography.Relief
	at     geom.Vec
	atlas  *magenta
	lit    render.Illumination
}

func (*standing) Init(*goke.SysInit) {}
func (s *standing) Compose(f *render.Frame, cam camera.Camera) {
	s.look.Begin(f, cam)
	box := plane.NewAABB(geom.NewVec(s.at.X-3, s.at.Y-3), 6, 6)
	z := world.Z{Altitude: s.relief.At(s.at), Height: 6}
	s.look.Sprite(f, cam, box, z, s.atlas, render.Appearance{}, s.lit)
	f.DrawAt(f.OnGround(2))
}
func (s *standing) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	s.look.DrawSprites(t, cam, u)
}

func TestSprites_StandDarkerInTheReliefsShadow(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	b := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	b.Write().Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	l := light.NewPlugin(w, light.Config{}).WithDistant(westSun)
	p := topography.NewPlugin(w, b, l, topography.Config{}).
		WithAtmosphere(westSky{})
	drawn := append(topotest.Drawn(t, l), topotest.Drawn(t, p)...)
	// a wall 120 high west of x 96: its shadow reaches 69 east over the level ground, towards the
	// eye, which looks from the south-east
	p.Write().Relief().SetHeights(ground.MeanOfCells(g, func(c cell.ID) float64 {
		if x, _, _ := g.Coords(c); x <= 2 {
			return 120
		}
		return 0
	}))
	cam := cameras.NewPlugin(w, cameras.Config{
		Turn:       true,
		Tilt:       true,
		Cell:       32,
		HeightUnit: 1,
		Cage:       p.Read(0),
	}).New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(320, 240)
	cam.MoveTo(0, 0)
	cam.ZoomOut(2.5, 160, 120)
	cam.CenterOn(160, 140, 0)
	shine := func(at geom.Vec, takes render.Taking) (lit, n int) {
		fig := &standing{look: w.Read(0).Look().(world.DirectLook), relief: p.Write().Relief(), at: at,
			atlas: &magenta{}, lit: render.Illumination{Light: render.Light{1, 1, 1}, Takes: takes}}
		c := render.NewComposer(append([]render.Source{sunning{}}, append(drawn, fig)...)...)
		screen := render.NewImage(320, 240)
		var pix []byte
		for range 3 { // the first frame readies the ground, the next bake its shade and show it
			c.DrawWorld(screen, cam)
		}
		pix = make([]byte, 4*320*240)
		screen.ReadPixels(pix)
		for i := 0; i < len(pix); i += 4 {
			if purple(pix[i:]) {
				lit += int(pix[i])
				n++
			}
		}
		if n == 0 {
			t.Fatalf("the figure at %v does not show", at)
		}
		return lit / n, n
	}
	sunny, _ := shine(geom.NewVec(200, 140), render.TakesAll)
	shaded, _ := shine(geom.NewVec(120, 140), render.TakesAll)
	if shaded >= sunny*4/5 {
		t.Errorf("a figure in the wall's shadow is lit %d, one in the sun %d; want it darker",
			shaded, sunny)
	}
	if shining, _ := shine(geom.NewVec(120, 140), 0); shining < sunny-2 {
		t.Errorf("a figure taking no shade is lit %d in the wall's shadow, one in the sun %d; "+
			"want it as bright", shining, sunny)
	}
	if lamplit, _ := shine(geom.NewVec(120, 140), render.TakesBlocks); lamplit < sunny-2 {
		t.Errorf("a figure taking the blocks' shade alone is lit %d in the relief's shadow, one "+
			"in the sun %d; want it as bright", lamplit, sunny)
	}
}
