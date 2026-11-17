package billboards_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// hands the frame the west sun and the air's weather before the ground composes
type blowing struct{ air air.Weather }

func (b blowing) Compose(f *render.Frame, _ camera.Camera) {
	westSun.Frame(f)
	b.air.Frame(f, westSun, 0, 0)
}
func (blowing) Draw(render.Target, camera.Camera, render.Uniforms) {}

// one magenta figure 100 tall standing on level ground at, swaying as much as sway
type swaying struct {
	look  world.DirectLook
	at    geom.Vec
	atlas *magenta
	sway  float32
}

func (s *swaying) Compose(f *render.Frame, cam camera.Camera) {
	s.look.Begin(f, cam)
	box := plane.NewAABB(geom.NewVec(s.at.X-3, s.at.Y-3), 6, 6)
	s.look.Sprite(f, cam, box, world.Z{Height: 100}, s.atlas, render.Appearance{Sway: s.sway},
		render.WhiteLight)
	f.DrawAt(f.OnGround(2))
}
func (s *swaying) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	s.look.DrawSprites(t, cam, u)
}

func TestSprites_LeanTheirTopsWithTheWindAsFarAsTheySway(t *testing.T) {
	gputest.Need(t)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	b := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	b.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	l := light.NewPlugin(w, light.Config{}).WithDistant(westSun)
	p := topography.NewPlugin(w, b, l, topography.Config{}).WithAtmosphere(westSky{})
	drawn := append(topotest.Drawn(t, l), topotest.Drawn(t, p)...)
	cam := cameras.NewPlugin(w, cameras.Config{
		Turn:       true,
		Tilt:       true,
		Cell:       32,
		HeightUnit: 1,
		Cage:       p.Read(0),
	}).Write().New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(320, 240)
	cam.CenterOn(128, 128, 0)
	// the screen's column the figure's top and its foot stand in, the means of their rows
	stands := func(weather air.Weather, sway float32) (top, foot float32) {
		fig := &swaying{look: w.Read(0).Look().(world.DirectLook), at: geom.NewVec(128, 128),
			atlas: &magenta{}, sway: sway}
		c := render.NewComposer(append([]render.Source{blowing{weather}}, append(drawn, fig)...)...)
		l.Publish()
		l.Swap()
		c.Light(w.Read(0).Lights)
		screen := render.NewImage(320, 240)
		for range 3 {
			c.DrawWorld(screen, cam)
		}
		pix := make([]byte, 4*320*240)
		screen.ReadPixels(pix)
		first, last := -1, -1
		for y := range 240 {
			var sum, n float32
			for x := range 320 {
				if purple(pix[4*(y*320+x):]) {
					sum += float32(x)
					n++
				}
			}
			if n == 0 {
				continue
			}
			if first < 0 {
				first, top = y, sum/n
			}
			last, foot = y, sum/n
		}
		if first < 0 || last-first < 40 {
			t.Fatalf("the figure shows over the rows %d to %d, want it standing tall", first, last)
		}
		return top, foot
	}
	east := air.Weather{Wind: [2]float32{60, 0}}
	stillTop, stillFoot := stands(air.Weather{}, 1)
	stiffTop, _ := stands(east, 0)
	blownTop, blownFoot := stands(east, 1)
	if stiffTop != stillTop {
		t.Errorf("what does not sway stands with its top at column %v in the wind, %v in the "+
			"calm; want it upright", stiffTop, stillTop)
	}
	// east runs right and down the screen of a camera looking from the south-east
	if blownTop < stillTop+3 || blownFoot != stillFoot {
		t.Errorf("in an east wind what sways stands with its top at column %v and its foot at "+
			"%v, in the calm at %v and %v; want its top leant east and its foot where it stood",
			blownTop, blownFoot, stillTop, stillFoot)
	}
}
