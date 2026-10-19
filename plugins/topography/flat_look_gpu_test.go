package topography_test

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

type flatSource struct {
	look   world.Look
	direct bool
	// lays the deck slanting alone, else the sprite and the deck running east
	slant bool
	atlas render.AtlasSource
}

func (*flatSource) Init(*goke.SysInit) {}

func (s *flatSource) Compose(f *render.Frame, cam camera.Camera) {
	if s.direct {
		s.look.(world.DirectLook).Begin(f, cam)
		f.DrawAt(f.At(0, 0, 0))
	}
	a := render.Appearance{SpriteID: 1}
	if !s.slant {
		s.look.Sprite(
			f,
			cam,
			plane.NewAABB(geom.NewVec(40000, 30000), 16, 16),
			world.Z{},
			s.atlas,
			a,
			render.WhiteLight,
		)
	}
	lie := world.Lie{Length: 96, Width: 20, Edged: true}
	for k, dir := range []geom.Vec{geom.NewVec(0.6, 0.8), geom.NewVec(1, 0)} {
		if s.slant != (k == 0) {
			continue
		}
		box := plane.NewAABB(geom.NewVec(40030, 29960+float64(k)*60), 10, 10)
		s.look.(world.LyingLook).Lay(
			f,
			cam,
			box,
			dir,
			lie,
			world.Z{},
			s.atlas,
			a,
			2,
			render.WhiteLight,
		)
	}
}

func (s *flatSource) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if s.direct {
		s.look.(world.DirectLook).DrawSprites(t, cam, u)
	}
}

func TestFlatLook_FromAboveDrawsOnTheGPUAsOnTheFrame(t *testing.T) {
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 65536, Height: 65536},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(2048, 2048, 32), &cell.MultipleOccupancy{}, w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	cam := cameras.NewPlugin(w, cameras.Config{
		Turn:       true,
		Tilt:       true,
		Cell:       32,
		TileW:      64,
		HeightUnit: 1,
		Cage:       p,
	}).New(cameras.FromAbove, camera.Config{})
	cam.SetViewport(320, 240)
	cam.ZoomIn(2.5, 160, 120)
	cam.CenterOn(40030, 30010, 0)
	atlas := render.NewAtlas()
	stripes := func(dst *render.Canvas, size int) {
		dst.FillRect(0, 0, float32(size), float32(size), color.RGBA{R: 120, G: 60, B: 30, A: 255})
		for x := 0; x < size; x += 4 {
			dst.FillRect(float32(x), 0, 1, float32(size), color.RGBA{R: 30, G: 20, B: 10, A: 255})
		}
	}
	atlas.Add(render.SpriteID(1), 16, stripes)
	atlas.Add(render.SpriteID(2), 16, stripes)
	atlas.Close()
	pix := func(direct, slant bool) []byte {
		screen := render.NewImage(320, 240)
		src := &flatSource{look: w.FlatLook(), direct: direct, slant: slant, atlas: atlas}
		render.NewComposer(src).DrawWorld(screen, cam)
		out := make([]byte, 4*320*240)
		screen.ReadPixels(out)
		return out
	}
	frame, direct := pix(false, false), pix(true, false)
	drawn, uncovered, recoloured := 0, 0, 0
	for i := 0; i < len(frame); i += 4 {
		if frame[i+3] != 0 {
			drawn++
		}
		if d := int(frame[i+3]) - int(direct[i+3]); d > 1 || d < -1 {
			uncovered++
			continue
		}
		for k := range 3 {
			if d := int(frame[i+k]) - int(direct[i+k]); d > 1 || d < -1 {
				recoloured++
				break
			}
		}
	}
	if drawn == 0 {
		t.Fatal("nothing in view")
	}
	// the GPU lays a sprite through the camera's matrix, the frame through its projection, under a
	// hundredth of a pixel apart: a magnified texel's edge flips a pixel here and there
	if uncovered > drawn/100 || recoloured > drawn/20 {
		t.Errorf("of %d pixels drawn, %d covered otherwise on the GPU and %d coloured otherwise",
			drawn, uncovered, recoloured)
	}
	// slanting, the GPU reads the deck smooth: it lies where the frame lays it, half covered and
	// more where the frame covers a pixel
	frame, direct = pix(false, true), pix(true, true)
	drawn, uncovered = 0, 0
	for i := 0; i < len(frame); i += 4 {
		if frame[i+3] != 0 {
			drawn++
		}
		if (frame[i+3] >= 128) != (direct[i+3] >= 128) {
			uncovered++
		}
	}
	if drawn == 0 || uncovered > drawn/100 {
		t.Errorf("of %d pixels of the slanting deck, %d covered otherwise on the GPU", drawn,
			uncovered)
	}
}
