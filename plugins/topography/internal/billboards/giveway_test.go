package billboards_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// ballOf is a ball 12 across in the colour of its paint
func ballOf(p world.Paint) world.Body {
	return world.Body{
		Wide:   12,
		Tall:   12,
		Parts:  []world.Part{{Shape: world.Spheroid, To: [3]float32{0, 0, 12}, Wide: 12}},
		Paints: []world.Paint{p},
	}
}

// covering lays, through the world's look, the lawn's first body as a cover, whole or as its
// card, its foot at where and alt, giving way where gives says, and its second as a thing at
// at and up, uncovered where bares says
type covering struct {
	g            *lawn
	where, at    geom.Vec
	alt, up      float32
	whole        bool
	gives, bares bool
}

func (s *covering) Compose(f *render.Frame, cam camera.Camera) {
	look := s.g.w.Read(0).Look().(world.DirectLook)
	look.Begin(f, cam)
	ways, cut, bodies := look.(world.UncoveringLook), look.(world.CutoutLook), look.(world.BodyLook)
	stand := func(b *world.Bodied, at geom.Vec, alt float32, whole bool) {
		box := plane.NewAABB(geom.NewVec(at.X-b.Wide/2, at.Y-b.Wide/2), b.Wide, b.Wide)
		bodies.Body(f, cam, box, world.Z{Altitude: alt, Height: b.Tall}, s.g.atlas, b, 0, whole,
			render.Appearance{}, render.WhiteLight)
	}
	cut.Cutout(true)
	ways.GiveWay(s.gives)
	stand(s.g.bodies[0], s.where, s.alt, s.whole)
	ways.GiveWay(false)
	ways.Uncover(s.bares)
	stand(s.g.bodies[1], s.at, s.up, true)
	ways.Uncover(false)
	cut.Cutout(false)
	if ways.Uncovers() {
		f.DrawUncovered()
	}
	f.DrawCutout()
}

func (s *covering) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	s.g.w.Read(0).Look().(world.DirectLook).DrawSprites(t, cam, u)
}

// covered draws s through the lawn's camera and counts the pixels of the cover's greens and
// of the thing's red
func (g *lawn) covered(s *covering) (greens, red int) {
	g.t.Helper()
	s.g = g
	c := render.NewComposer(append([]render.Source{shining{g.sun, g.air}},
		append(g.drawn, s)...)...)
	g.l.Publish()
	g.l.Swap()
	c.Light(g.w.Read(0).Lights)
	screen := render.NewImage(320, 240)
	for range 3 {
		c.DrawWorld(screen, g.cam)
	}
	pix := make([]byte, 4*320*240)
	screen.ReadPixels(pix)
	for i := 0; i < len(pix); i += 4 {
		switch r, gr, bl := int(pix[i]), int(pix[i+1]), int(pix[i+2]); {
		case r > gr+60 && r > bl+60:
			red++
		case gr > r+35 && gr > bl+35:
			greens++
		}
	}
	return greens, red
}

// ahead is the way the lawn's camera looks over the ground
func (g *lawn) ahead() geom.Vec {
	f, _ := g.cam.(camera.Rays).Rays()
	n := f32.Hypot(f.Dir[0], f.Dir[1])
	return geom.NewVec(f.Dir[0]/n, f.Dir[1]/n)
}

func givingLawn(t *testing.T, eye bool) *lawn {
	return newLawn(t, leftSun, air.Weather{}, eye, headedOf(world.Paint{Colour: headGreen}),
		ballOf(world.Paint{Colour: markRed}))
}

func TestBodies_OneGivingWayShowsWhatIsUncoveredBehindItAndNothingElse(t *testing.T) {
	// where a thing hides behind the cover's head and where it shows before the cover, as far
	// on along the camera's way and as high: seen from 35° over the ground, and by an eye
	for _, c := range []struct {
		eye                bool
		on, up, back, high float32
	}{{false, 30, 21, -20, 34}, {true, 20, 34, -20, 34}} {
		for _, whole := range []bool{false, true} {
			g := givingLawn(t, c.eye)
			mid, ahead := geom.NewVec(128, 128), g.ahead()
			behind := &covering{where: mid, whole: whole,
				at: geom.NewVec(mid.X+ahead.X*c.on, mid.Y+ahead.Y*c.on), up: c.up}
			hidden, none := g.covered(behind)
			if none != 0 {
				t.Fatalf("eye %v, whole %v: a thing behind the cover shows %d pixels, want it "+
					"hidden", c.eye, whole, none)
			}
			behind.gives, behind.bares = true, true
			greens, red := g.covered(behind)
			if red < 40 || greens >= hidden {
				t.Errorf("eye %v, whole %v: with the cover giving way to the thing behind it "+
					"%d of its pixels show and %d of the cover's, %d before; want the thing "+
					"showing through", c.eye, whole, red, greens, hidden)
			}
			for name, s := range map[string]covering{
				"a cover not giving way": {gives: false, bares: true},
				"a thing not uncovered":  {gives: true, bares: false},
			} {
				s.where, s.whole, s.at, s.up = mid, whole, behind.at, behind.up
				if greens, red := g.covered(&s); red != 0 || greens != hidden {
					t.Errorf("eye %v, whole %v, %s: %d pixels of the thing show and %d of the "+
						"cover, want none and the cover whole, %d", c.eye, whole, name, red,
						greens, hidden)
				}
			}
			// before the cover the thing is seen as ever, and the cover behind it gives no way
			before := &covering{where: mid, whole: whole,
				at: geom.NewVec(mid.X+ahead.X*c.back, mid.Y+ahead.Y*c.back), up: c.high}
			was, shown := g.covered(before)
			before.gives, before.bares = true, true
			if greens, red := g.covered(before); greens != was || red != shown || shown == 0 {
				t.Errorf("eye %v, whole %v: a thing before the cover shows %d pixels and the "+
					"cover %d, with the cover giving way %d and %d; want the same", c.eye, whole,
					shown, was, red, greens)
			}
		}
	}
}

func TestBodies_NothingGivesWayToWhatTheGroundHides(t *testing.T) {
	g := givingLawn(t, false)
	g.relief.SetHeights(ground.MeanOfCells(g.grid, func(c cell.ID) float32 {
		if x, y, _ := g.grid.Coords(c); x >= 3 && x <= 4 && y >= 3 && y <= 4 {
			return 90
		}
		return 0
	}))
	// the points of the screen that see the hill's slope with level ground behind it, down its
	// middle: the one in their midst has the hill stand well between
	picker := g.cam.(interface {
		Pick(sx, sy float32) (float32, float32, float32, bool)
	})
	type sight struct {
		sy, high float32
		behind   geom.Vec
	}
	var seen []sight
	for y := float32(0); y < 240; y++ {
		hx, hy, _, ok := picker.Pick(160, y)
		if !ok {
			continue
		}
		at := g.relief.At(geom.NewVec(hx, hy))
		ux, uy := g.cam.Unproject(160, y, 0)
		level := g.relief.At(geom.NewVec(ux, uy)) < 1
		if dx, dy := ux-hx, uy-hy; at > 40 && level && dx*dx+dy*dy > 60*60 {
			seen = append(seen, sight{y, at, geom.NewVec(ux, uy)})
		}
	}
	if len(seen) == 0 {
		t.Fatal("no point of the screen sees the hill with level ground behind it")
	}
	sy, high, behind := seen[len(seen)/2].sy, seen[len(seen)/2].high, seen[len(seen)/2].behind
	// the cover floats before the hill, its head over that point of the screen
	alt := high + 10
	tx, ty := g.cam.Unproject(160, sy, alt+44)
	s := &covering{where: geom.NewVec(tx, ty), alt: alt, at: behind}
	whole, none := g.covered(s)
	if none != 0 || whole == 0 {
		t.Fatalf("behind the hill the thing shows %d pixels and the cover before it %d, want "+
			"the thing hidden and the cover seen", none, whole)
	}
	s.gives, s.bares = true, true
	if greens, red := g.covered(s); red != 0 || greens != whole {
		t.Errorf("with the cover giving way, %d pixels of the thing behind the hill show and "+
			"%d of the cover, %d before; want the cover whole: the ground hides the thing",
			red, greens, whole)
	}
}

// decked lays a magenta deck 40 across level at 30 over the lawn's middle, giving way where
// gives says, and the lawn's second body on the ground behind it as the camera sees it, under
// the deck's middle on the screen, uncovered where bares says
type decked struct {
	g            *lawn
	deck         *magenta
	gives, bares bool
}

func (s *decked) Compose(f *render.Frame, cam camera.Camera) {
	look := s.g.w.Read(0).Look().(world.DirectLook)
	look.Begin(f, cam)
	ways, cut := look.(world.UncoveringLook), look.(world.CutoutLook)
	mid, ahead := geom.NewVec(128, 128), s.g.ahead()
	ways.GiveWay(s.gives)
	look.(world.LyingLook).Lie(f, cam, plane.NewAABB(geom.NewVec(mid.X-20, mid.Y-20), 40, 40),
		world.Z{Altitude: 30}, s.deck, render.Appearance{}, render.WhiteLight)
	ways.GiveWay(false)
	ways.Uncover(s.bares)
	b := s.g.bodies[1]
	// the line of sight through it climbs to the deck's 30 over 42 of the ground, from 35°
	at := geom.NewVec(mid.X+ahead.X*34, mid.Y+ahead.Y*34)
	cut.Cutout(true)
	look.(world.BodyLook).Body(f, cam,
		plane.NewAABB(geom.NewVec(at.X-b.Wide/2, at.Y-b.Wide/2), b.Wide, b.Wide),
		world.Z{Height: b.Tall}, s.g.atlas, b, 0, true, render.Appearance{}, render.WhiteLight)
	cut.Cutout(false)
	ways.Uncover(false)
	if ways.Uncovers() {
		f.DrawUncovered()
	}
	f.DrawCutout()
	f.DrawAt(f.OnGround(2))
}

func (s *decked) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	s.g.w.Read(0).Look().(world.DirectLook).DrawSprites(t, cam, u)
}

func TestSprites_WhatLiesGivesWayToWhatIsUncoveredUnderIt(t *testing.T) {
	g := givingLawn(t, false)
	deck := &magenta{}
	seen := func(gives, bares bool) (purples, red int) {
		c := render.NewComposer(append([]render.Source{shining{g.sun, g.air}},
			append(g.drawn, &decked{g, deck, gives, bares})...)...)
		g.l.Publish()
		g.l.Swap()
		c.Light(g.w.Read(0).Lights)
		screen := render.NewImage(320, 240)
		for range 3 {
			c.DrawWorld(screen, g.cam)
		}
		pix := make([]byte, 4*320*240)
		screen.ReadPixels(pix)
		for i := 0; i < len(pix); i += 4 {
			switch r, gr, bl := int(pix[i]), int(pix[i+1]), int(pix[i+2]); {
			case purple(pix[i:]):
				purples++
			case r > gr+60 && r > bl+60:
				red++
			}
		}
		return purples, red
	}
	whole, none := seen(false, false)
	if whole == 0 || none != 0 {
		t.Fatalf("under the deck the thing shows %d pixels and the deck %d, want the thing "+
			"hidden and the deck seen", none, whole)
	}
	if deck, red := seen(true, true); red < 40 || deck >= whole {
		t.Errorf("with the deck giving way %d pixels of the thing under it show and %d of the "+
			"deck, %d before; want the thing showing through", red, deck, whole)
	}
	if deck, red := seen(true, false); red != 0 || deck != whole {
		t.Errorf("with nothing uncovered %d pixels of the thing show and %d of the deck, want "+
			"none and the deck whole, %d", red, deck, whole)
	}
}
