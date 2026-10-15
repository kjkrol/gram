package render

import (
	"image/color"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
)

var (
	groundGreen = color.RGBA{G: 200, A: 255}
	figureRed   = color.RGBA{R: 220, A: 255}
	skyBlue     = color.RGBA{B: 230, A: 255}
)

// the left half of a 64 by 64 world, opaque at height 0
type groundSource struct{ still *Still }

func (*groundSource) Init(*goke.SysInit)                {}
func (*groundSource) Compose(f *Frame, _ camera.Camera) { f.DrawOpaque() }
func (g *groundSource) Draw(t Target, cam camera.Camera, u Uniforms) {
	if t.Pass == Opaque {
		g.still.Draw(t, cam, u, 1, 0, 0, Light{1, 1, 1})
	}
}

// a figure over the world box (16, 16)-(32, 32) at height z, standing or lying
type figureSource struct {
	sprites Sprites
	atlas   AtlasSource
	z       float32
	lying   bool
	id      SpriteID
}

func (*figureSource) Init(*goke.SysInit) {}
func (s *figureSource) Compose(f *Frame, _ camera.Camera) {
	at := f.Lifted(24, 24, s.z, 2)
	s.sprites.Layer(at, s.lying)
	s.sprites.Rect(s.atlas, s.id, 16, 16, 32, 32, 0, 0, 1, 1, Light{1, 1, 1})
	f.DrawAt(at)
}
func (s *figureSource) Draw(t Target, cam camera.Camera, _ Uniforms) { s.sprites.Draw(t, cam) }

type skySource struct{}

func (skySource) Init(*goke.SysInit)                {}
func (skySource) Compose(f *Frame, _ camera.Camera) { f.DrawAt(Furthest) }
func (skySource) Draw(t Target, _ camera.Camera, _ Uniforms) {
	if t.Depth != nil {
		t.Screen.FillBehind(skyBlue, t.Depth)
	}
}

func atlasOf(t *testing.T) *Atlas {
	t.Helper()
	a := NewAtlas()
	a.Add(SpriteID(1), 8, Solid(groundGreen))
	a.Add(SpriteID(2), 8, Solid(figureRed))
	a.Add(SpriteID(3), 8, Solid(color.RGBA{R: 30, G: 30, B: 30, A: 255}))
	a.Close()
	return a
}

func groundOf(a *Atlas) *groundSource {
	s := NewStill()
	s.Compose(64, 64, func(f *Frame, _ camera.Camera) {
		f.SpriteRect(f.At(16, 32, 0), a, 1, 0, 0, 32, 64, Even(1))
	})
	return &groundSource{still: s}
}

func shot(t *testing.T, sources ...Layer) func(x, y int) [4]byte {
	t.Helper()
	cam := icamera.NewFromSpace(64, 64, 0)
	cam.SetViewport(64, 64)
	screen := NewImage(64, 64)
	NewComposer(sources...).DrawWorld(screen, cam)
	pix := make([]byte, 4*64*64)
	screen.ReadPixels(pix)
	return func(x, y int) [4]byte { i := 4 * (y*64 + x); return [4]byte(pix[i : i+4]) }
}

func TestComposer_TheHigherCoversTheLowerWhateverTheOrderOfTheSources(t *testing.T) {
	needGPU(t)
	a := atlasOf(t)
	veil := color.RGBA{B: 255, A: 128}
	decal := items(func(f *Frame) {
		f.Soft(f.Lifted(32, 32, 0, 0.75), Corners{{0, 0}, {64, 0}, {0, 64}, {64, 64}}, veil, Fade{})
	})
	mark := items(func(f *Frame) {
		f.Quad(
			OnTop,
			Corners{{20, 20}, {28, 20}, {20, 28}, {28, 28}},
			[4]color.RGBA{skyBlue, skyBlue, skyBlue, skyBlue},
		)
	})
	orders := [][]int{{0, 1, 2, 3, 4}, {4, 3, 2, 1, 0}, {2, 0, 4, 1, 3}, {1, 4, 0, 3, 2}}
	var first [5][4]byte
	for k, order := range orders {
		sources := []Layer{
			groundOf(a),
			&figureSource{atlas: a, z: 10, id: 2},
			decal,
			skySource{},
			mark,
		}
		var shuffled []Layer
		for _, i := range order {
			shuffled = append(shuffled, sources[i])
		}
		at := shot(t, shuffled...)
		got := [5][4]byte{at(17, 17), at(10, 40), at(50, 40), at(24, 24), at(30, 30)}
		if got[0] != [4]byte{220, 0, 0, 255} {
			t.Errorf("order %v: the figure shows %v, want it red over the veil and the ground",
				order, got[0])
		}
		if got[1][1] == 0 || got[1][2] == 0 {
			t.Errorf("order %v: the ground shows %v, want green under the veil", order, got[1])
		}
		if got[2][2] != 230 && got[2][2] < 200 {
			t.Errorf("order %v: beside the ground %v, want the sky's blue", order, got[2])
		}
		if got[3] != [4]byte{0, 0, 230, 255} {
			t.Errorf("order %v: the mark shows %v, want it on top of the figure", order, got[3])
		}
		if k == 0 {
			first = got
		} else if got != first {
			t.Errorf("order %v drew %v, the first %v: the picture hangs on the order of the "+
				"sources", order, got, first)
		}
	}
}

func TestComposer_AtOneHeightTheStandingCoversTheLying(t *testing.T) {
	needGPU(t)
	a := atlasOf(t)
	at := shot(t, groundOf(a), &twoFigures{atlas: a})
	if p := at(24, 24); p != [4]byte{220, 0, 0, 255} {
		t.Errorf("a standing and a lying figure at one height show %v, want the standing red", p)
	}
}

// a lying and a standing figure at one height, the standing added first
type twoFigures struct {
	sprites Sprites
	atlas   AtlasSource
}

func (*twoFigures) Init(*goke.SysInit) {}
func (s *twoFigures) Compose(f *Frame, _ camera.Camera) {
	at := f.Lifted(24, 24, 3, 2)
	s.sprites.Layer(at, false)
	s.sprites.Rect(s.atlas, 2, 16, 16, 32, 32, 0, 0, 1, 1, Light{1, 1, 1})
	s.sprites.Layer(at, true)
	s.sprites.Rect(s.atlas, 3, 16, 16, 32, 32, 0, 0, 1, 1, Light{1, 1, 1})
	f.DrawAt(at)
}
func (s *twoFigures) Draw(t Target, cam camera.Camera, _ Uniforms) { s.sprites.Draw(t, cam) }

// what Draw was handed: whether the frame had a depth and the ground's apart
type peek struct {
	opaque, reads bool
	depth, ground bool
}

func (*peek) Init(*goke.SysInit) {}
func (p *peek) Compose(f *Frame, _ camera.Camera) {
	if p.opaque {
		f.DrawOpaque()
	}
	if p.reads {
		f.ReadGround()
	}
	f.DrawAt(Nearest)
}
func (p *peek) Draw(t Target, _ camera.Camera, _ Uniforms) {
	if t.Pass == SeeThrough {
		p.depth, p.ground = t.Depth != nil, t.Ground != nil
	}
}

func TestComposer_ADepthOnlyUnderAGroundAndTheGroundsOnlyForOneReadingIt(t *testing.T) {
	needGPU(t)
	for _, tc := range []struct {
		opaque, reads, depth, ground bool
	}{
		{opaque: false, reads: false, depth: false, ground: false},
		{opaque: true, reads: false, depth: true, ground: false},
		{opaque: true, reads: true, depth: true, ground: true},
	} {
		p := &peek{opaque: tc.opaque, reads: tc.reads}
		shot(t, p)
		if p.depth != tc.depth || p.ground != tc.ground {
			t.Errorf("a ground %v, read %v: handed a depth %v and the ground's %v, want %v and %v",
				tc.opaque, tc.reads, p.depth, p.ground, tc.depth, tc.ground)
		}
	}
}
