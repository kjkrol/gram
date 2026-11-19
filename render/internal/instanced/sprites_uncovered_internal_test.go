package instanced

import (
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/gram/render/internal/atlas"
	"github.com/kjkrol/gram/render/internal/compose"
	"github.com/kjkrol/gram/render/internal/gfx"
)

// the rectangle the last coverOverThing's uncovered lay within
var lastIn [4]float32

// a cover 48 across at the height high over a thing 8 across at 2, in the middle of a picture 64
// across, drawn as a frame's passes go; the cover white, the thing red
func coverOverThing(high float32, gives, uncover, cutout bool) (pixel func(x, y int) [4]byte) {
	sheet := atlas.NewAtlas()
	sheet.Add(SpriteID(1), 8, solid(white))
	sheet.Add(SpriteID(2), 8, solid(color.RGBA{R: 255, A: 255}))
	sheet.Close()
	cam := cameratest.NewFromSpace(64, 64, 0)
	cam.SetViewport(64, 64)
	var f compose.Frame
	var s Sprites
	s.Layer(f.At(32, 32, high), false)
	s.Cutout(cutout)
	s.GiveWay(gives)
	s.Rect(sheet, 1, 8, 8, 56, 56, 0, 0, 1, 1, Light{1, 1, 1})
	s.Layer(f.At(32, 32, 2), false)
	s.Cutout(false)
	s.GiveWay(false)
	s.Uncover(uncover)
	s.Rect(sheet, 2, 28, 28, 36, 36, 0, 0, 1, 1, Light{1, 1, 1})
	screen := gfx.NewImage(64, 64)
	depth, uncovered := gfx.NewDepth(), gfx.NewDepth().Readable()
	screen.ClearDepth(depth)
	t := Target{Screen: screen, Depth: depth}
	if s.Uncovers() && s.GivesWay() {
		screen.ClearDepth(uncovered)
		in := [4]float32{math.MaxFloat32, math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
		s.Draw(Target{Screen: screen, Depth: uncovered, Pass: Uncovered, UncoveredIn: &in}, cam,
			Uniforms{})
		t.Uncovered, t.UncoveredIn = uncovered, &in
		lastIn = in
	}
	t.Pass = Cutout
	s.Draw(t, cam, Uniforms{})
	t.Pass = compose.SeeThrough
	s.Draw(t, cam, Uniforms{})
	return pixelsOf(screen)
}

// how many of the pixels x0..x1 by y0..y1 show the thing, red, and how many the cover, white
func shown(pixel func(x, y int) [4]byte, x0, y0, x1, y1 int) (thing, cover int) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			switch p := pixel(x, y); {
			case p[0] > 200 && p[1] < 60:
				thing++
			case p[0] > 200 && p[1] > 200:
				cover++
			}
		}
	}
	return thing, cover
}

func TestSprites_APieceGivingWayIsLeftOutInPartOverOneUncoveredUnderIt(t *testing.T) {
	gputest.Need(t)
	for _, cutout := range []bool{true, false} {
		pixel := coverOverThing(10, true, true, cutout)
		// over the thing three pixels of four are left open, fewer towards its corners
		thing, cover := shown(pixel, 28, 28, 36, 36)
		if thing < 44 || thing > 48 || thing+cover != 64 {
			t.Errorf("cutout %v: the thing shows at %d of its 64 pixels and the cover at %d, "+
				"want some 48 and 16", cutout, thing, cover)
		}
		// past the disc, reaching as far as the thing's side from its middle, the cover is whole
		if thing, cover := shown(pixel, 8, 8, 56, 22); thing != 0 || cover != 48*14 {
			t.Errorf("cutout %v: away from the thing the cover shows at %d pixels of %d and "+
				"the thing at %d, want it whole", cutout, cover, 48*14, thing)
		}
		// the uncovered lie within the disc's square, as far as the thing's side off its middle
		if lastIn != [4]float32{24, 24, 40, 40} {
			t.Errorf("cutout %v: the uncovered lie within %v, want the disc's square", cutout,
				lastIn)
		}
		// between the thing's edge and the rim the cover thins: open here and there, ground
		// showing, neither whole nor as open as over the thing
		_, ring := shown(pixel, 22, 28, 28, 36)
		if ring <= 48*1/4 || ring >= 48 {
			t.Errorf("cutout %v: beside the thing the cover shows at %d pixels of 48, want it "+
				"thinned, more of it than over the thing", cutout, ring)
		}
	}
}

func TestSprites_NothingGivesWayUnlessAskedNorToWhatLiesOverIt(t *testing.T) {
	gputest.Need(t)
	whole := func(name string, pixel func(x, y int) [4]byte) {
		t.Helper()
		if thing, cover := shown(pixel, 8, 8, 56, 56); thing != 0 || cover != 48*48 {
			t.Errorf("%s: the cover shows at %d pixels of %d and the thing at %d, want the "+
				"cover whole", name, cover, 48*48, thing)
		}
	}
	whole("a cover not giving way", coverOverThing(10, false, true, true))
	whole("a thing not uncovered", coverOverThing(10, true, false, true))
	// a cover lying lower than the thing gives no way: the thing is drawn over it as ever
	pixel := coverOverThing(1, true, true, true)
	if thing, cover := shown(pixel, 8, 8, 56, 56); thing != 64 || cover != 48*48-64 {
		t.Errorf("a cover under the thing shows at %d pixels and the thing at %d, want %d "+
			"and 64", cover, thing, 48*48-64)
	}
}
