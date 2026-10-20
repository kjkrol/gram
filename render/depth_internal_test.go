package render

import (
	"image/color"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
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
		x, y := cam.FromScreen(0, 0)
		g.still.Draw(t, cam, u, cam.Zoom(), x, y, Light{1, 1, 1})
	}
}

// a figure over the world box (16, 16)-(32, 32) at height z, standing or lying
type figureSource struct {
	sprites Sprites
	atlas   AtlasSource
	z       float32
	lying   bool
	id      SpriteID
	fog     float32
}

func (*figureSource) Init(*goke.SysInit) {}
func (s *figureSource) Compose(f *Frame, _ camera.Camera) {
	s.sprites.Reset()
	at := f.Lifted(24, 24, s.z, 2)
	s.sprites.Layer(at, s.lying)
	s.sprites.Fog(s.fog)
	s.sprites.Rect(s.atlas, s.id, 16, 16, 32, 32, 0, 0, 1, 1, Light{1, 1, 1})
	f.DrawAt(at)
}
func (s *figureSource) Draw(t Target, cam camera.Camera, u Uniforms) { s.sprites.Draw(t, cam, u) }

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

func shot(t *testing.T, sources ...Source) func(x, y int) [4]byte {
	t.Helper()
	cam := cameratest.NewFromSpace(64, 64, 0)
	cam.SetViewport(64, 64)
	screen := NewImage(64, 64)
	NewComposer(sources...).DrawWorld(screen, cam)
	return pixelsOf(screen)
}

func pixelsOf(screen *Image) func(x, y int) [4]byte {
	w := screen.Bounds().Dx()
	pix := make([]byte, 4*w*screen.Bounds().Dy())
	screen.ReadPixels(pix)
	return func(x, y int) [4]byte { i := 4 * (y*w + x); return [4]byte(pix[i : i+4]) }
}

// the ground laid through cam at every pixel and drawn at a half of the 64 by 64 screen's
// pixels, laid over it scaled, and what lies over the figures drawn onto it at every pixel
func groundShot(t *testing.T, c *Composer, sources ...Source) func(x, y int) [4]byte {
	t.Helper()
	cam := cameratest.NewFromSpace(64, 64, 0)
	cam.SetViewport(64, 64)
	cam.(camera.Resolved).SetResolution(0.5)
	crop := cam.(camera.Cropping)
	crop.Crop(64, 64, 1)
	c.Lay(cam, true)
	crop.Uncrop()
	ground, screen := NewImage(32, 32), NewImage(64, 64)
	c.DrawLayer(ground, cam, camera.Window{W: 64, H: 64})
	op := DrawImageOptions{Filter: FilterLinear}
	op.GeoM.Scale(2, 2)
	screen.DrawImage(ground, &op)
	crop.Crop(64, 64, 1)
	c.DrawOver(screen, cam)
	crop.Uncrop()
	return pixelsOf(screen)
}

func TestComposer_TheHigherCoversTheLowerWhateverTheOrderOfTheSources(t *testing.T) {
	needGPU(t)
	a := atlasOf(t)
	veil := color.RGBA{B: 255, A: 128}
	decal := items(func(f *Frame) {
		f.Soft(f.Lifted(32, 32, 0, 0.75), Corners{{0, 0}, {64, 0}, {0, 64}, {64, 64}}, veil, Fade{})
	})
	mark := func(level float32, x0 float32, c color.RGBA) Source {
		return items(func(f *Frame) {
			square := Corners{{x0, 20}, {28, 20}, {x0, 28}, {28, 28}}
			f.Quad(OnTop(level), square, [4]color.RGBA{c, c, c, c})
		})
	}
	one := color.RGBA{R: 255, G: 255, A: 255}
	two := color.RGBA{G: 255, B: 255, A: 255}
	orders := [][]int{
		{0, 1, 2, 3, 4, 5, 6},
		{6, 5, 4, 3, 2, 1, 0},
		{2, 0, 6, 4, 1, 3, 5},
		{1, 4, 0, 5, 3, 6, 2},
	}
	var first [6][4]byte
	for k, order := range orders {
		sources := []Source{
			groundOf(a),
			&figureSource{atlas: a, z: 10, id: 2},
			decal,
			skySource{},
			mark(3, 25, skyBlue),
			mark(1, 20, one),
			mark(2, 20, two),
		}
		var shuffled []Source
		for _, i := range order {
			shuffled = append(shuffled, sources[i])
		}
		at := shot(t, shuffled...)
		got := [6][4]byte{at(17, 17), at(10, 40), at(50, 40), at(22, 22), at(26, 26), at(30, 30)}
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
		if got[3] != [4]byte{0, 255, 255, 255} || got[4] != [4]byte{0, 0, 230, 255} {
			t.Errorf("order %v: the marks show %v and %v, want the higher level over the lower",
				order, got[3], got[4])
		}
		if k == 0 {
			first = got
		} else if got != first {
			t.Errorf("order %v drew %v, the first %v: the picture hangs on the order of the "+
				"sources", order, got, first)
		}
	}
}

func TestComposer_WhatLiesOverTheFiguresIsHiddenByTheGroundAlone(t *testing.T) {
	needGPU(t)
	a := atlasOf(t)
	look := color.RGBA{G: 255, B: 255, A: 255}
	over := func(z float32) Source {
		return items(func(f *Frame) {
			square := Corners{{4, 20}, {28, 20}, {4, 28}, {28, 28}}
			f.Quad(f.At(16, 24, z).OverFigures(), square, [4]color.RGBA{look, look, look, look})
		})
	}
	at := shot(t, groundOf(a), &figureSource{atlas: a, z: 10, id: 2}, over(1))
	if p := at(24, 24); p != [4]byte{0, 255, 255, 255} {
		t.Errorf("over a figure standing higher it shows %v, want itself", p)
	}
	at = shot(t, groundOf(a), &figureSource{atlas: a, z: 10, id: 2}, over(-5))
	if p := at(8, 24); p[1] != 200 || p[2] != 0 {
		t.Errorf("under the ground it shows %v, want the ground hiding it", p)
	}
}

func TestComposer_TheOverPassBindsOverFiguresToTheGroundsDepth(t *testing.T) {
	needGPU(t)
	a := atlasOf(t)
	look := color.RGBA{G: 255, B: 255, A: 255}
	over := func(z float32) Source {
		return overlyingSource{items(func(f *Frame) {
			band := Corners{{4, 20}, {60, 20}, {4, 28}, {60, 28}}
			f.Quad(f.At(16, 24, z).OverFigures(), band, [4]color.RGBA{look, look, look, look})
		})}
	}
	figure := overlyingDirect{&figureSource{atlas: a, z: 10, id: 2}}
	c := NewComposer(groundOf(a), figure, over(1))
	at := groundShot(t, c)
	if c.ground == nil || c.depth == c.ground {
		t.Error("over the figures the composer bound what lies over them to the figures' depth, " +
			"want the ground's apart")
	}
	if p := at(24, 24); p != [4]byte{0, 255, 255, 255} {
		t.Errorf("over a figure standing higher it shows %v, want itself", p)
	}
	if p := at(30, 18); p != [4]byte{220, 0, 0, 255} {
		t.Errorf("beside it the figure shows %v, want it red at every pixel", p)
	}
	if p := at(50, 24); p != [4]byte{0, 255, 255, 255} {
		t.Errorf("beside the ground it shows %v, want itself: nothing hides it there", p)
	}
	c = NewComposer(groundOf(a), figure, over(-5))
	at = groundShot(t, c)
	if p := at(8, 24); p[1] != 200 || p[2] != 0 {
		t.Errorf("under the ground it shows %v, want the ground hiding it through its depth "+
			"spread to the screen", p)
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
	s.sprites.Reset()
	at := f.Lifted(24, 24, 3, 2)
	s.sprites.Layer(at, false)
	s.sprites.Rect(s.atlas, 2, 16, 16, 32, 32, 0, 0, 1, 1, Light{1, 1, 1})
	s.sprites.Layer(at, true)
	s.sprites.Rect(s.atlas, 3, 16, 16, 32, 32, 0, 0, 1, 1, Light{1, 1, 1})
	f.DrawAt(at)
}
func (s *twoFigures) Draw(t Target, cam camera.Camera, u Uniforms) { s.sprites.Draw(t, cam, u) }

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

// a camera from above with the figures at every pixel at its word
type figured struct {
	sharpened
	sharp bool
}

type sharpened interface {
	camera.Camera
	camera.Resolved
	camera.Layered
	camera.Cropping
	camera.Rays
}

func (f *figured) SetSharpFigures(on bool) { f.sharp = on }
func (f *figured) SharpFigures() bool      { return f.sharp }

// a checker of two world units at 0 over the 64 by 64 world, and a block 32 high over its left
// half
func checkeredGround(a *Atlas) *groundSource {
	s := NewStill()
	s.Compose(64, 64, func(f *Frame, _ camera.Camera) {
		for y := float32(0); y < 64; y += 2 {
			for x := float32(0); x < 64; x += 2 {
				id := SpriteID(1 + 2*(int(x/2+y/2)%2))
				f.SpriteRect(f.At(x, y, 0), a, id, x, y, x+2, y+2, Even(1))
			}
		}
		f.SpriteRect(f.At(16, 32, 32), a, 3, 0, 0, 32, 64, Even(1))
	})
	return &groundSource{still: s}
}

func TestFeed_DrawsAFigureAtEveryPixelOverAHalfResolutionGround(t *testing.T) {
	needGPU(t)
	a := atlasOf(t)
	line := overlyingSource{items(func(f *Frame) {
		x0, y0 := f.Camera().Project(4, 16.5, 0)
		x1, _ := f.Camera().Project(60, 16.5, 0)
		f.Line(OnTop(0), x0, y0, x1, y0, 1, white)
	})}
	figure := overlyingSource{items(func(f *Frame) {
		corners := ProjectCorners(f.Camera(), 16, 40, 48, 56, 10)
		f.Sprite(f.At(32, 48, 10), a, 2, corners, Even(1))
	})}
	picture := NewComposer(checkeredGround(a), figure, line)
	// the rows of the column 8, left of the figure, the line covers over a half, and the pixels
	shown := func(sharp bool) (int, func(x, y int) [4]byte) {
		cam := &figured{cameratest.NewFromSpace(64, 64, 0).(sharpened), sharp}
		cam.SetResolution(0.5)
		screen := NewImage(64, 64)
		NewFeed(cam, picture).DrawOn(screen)
		at := pixelsOf(screen)
		rows := 0
		for y := range 64 {
			if at(8, y)[0] >= 128 {
				rows++
			}
		}
		return rows, at
	}
	rows, at := shown(true)
	if rows != 1 {
		t.Errorf("with the figures sharp the line covers %d rows, want 1: a pixel wide at every "+
			"shown pixel", rows)
	}
	if p := at(40, 48); p != [4]byte{220, 0, 0, 255} {
		t.Errorf("right of the block the figure shows %v, want it red over the ground", p)
	}
	if p := at(24, 48); p[0] > 100 {
		t.Errorf("behind the block the figure shows %v, want the block hiding it through the "+
			"ground's depth spread to the screen", p)
	}
	if rows, _ := shown(false); rows < 2 {
		t.Errorf("with the figures soft the line covers %d rows, want 2 or more: drawn at a "+
			"half and scaled up", rows)
	}
}

// the ground at 0 over the 64 by 64 world with a ridge across it two units wide at x 31 to 33,
// 32 high
func ridgedGround(a *Atlas) *groundSource {
	s := NewStill()
	s.Compose(64, 64, func(f *Frame, _ camera.Camera) {
		f.SpriteRect(f.At(32, 32, 0), a, 1, 0, 0, 64, 64, Even(1))
		f.SpriteRect(f.At(32, 32, 32), a, 3, 31, 0, 33, 64, Even(1))
	})
	return &groundSource{still: s}
}

func TestLaid_AFigureBehindAHillIsHiddenThroughAnInnerLayersDepth(t *testing.T) {
	needGPU(t)
	a := atlasOf(t)
	figure := overlyingSource{items(func(f *Frame) {
		corners := ProjectCorners(f.Camera(), 20, 20, 44, 44, 10)
		f.Sprite(f.At(32, 32, 10), a, 2, corners, Even(1))
	})}
	picture := NewComposer(ridgedGround(a), figure)
	cam := &figured{cameratest.NewFromSpace(64, 64, 0).(sharpened), true}
	shown := func() func(x, y int) [4]byte {
		screen := NewImage(64, 64)
		NewFeed(cam, picture).DrawOn(screen)
		return pixelsOf(screen)
	}
	// at a quarter of the pixels the middles of the ground's pixels lie either side of the ridge
	cam.SetResolution(0.25)
	if p := shown()(32, 30); p != [4]byte{220, 0, 0, 255} {
		t.Fatalf("over a ground at a quarter of the pixels the figure shows %v on the ridge, "+
			"want it red: the ridge missed", p)
	}
	cam.SetLayers([]camera.Layer{{Share: 1, Resolution: 0.25}, {Share: 0.5, Resolution: 1}},
		camera.Inset)
	at := shown()
	for _, x := range []int{31, 32} {
		if p := at(x, 30); p[0] > 100 {
			t.Errorf("behind the ridge at (%d, 30) the figure shows %v, want the ridge hiding it "+
				"through the inner layer's depth", x, p)
		}
	}
	for _, x := range []int{24, 40} {
		if p := at(x, 30); p != [4]byte{220, 0, 0, 255} {
			t.Errorf("beside the ridge at (%d, 30) the figure shows %v, want it red", x, p)
		}
	}
}
