package billboards_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

var (
	headGreen = color.RGBA{R: 40, G: 150, B: 60, A: 255}
	markRed   = color.RGBA{R: 220, G: 40, B: 40, A: 255}
	postBrown = color.RGBA{R: 120, G: 80, B: 50, A: 255}
	dotPink   = color.RGBA{R: 255, G: 40, B: 255, A: 255}
)

// headedOf is a body 60 tall and 40 wide: a post under a ball in the paint given
func headedOf(head world.Paint) world.Body {
	return world.Body{
		Wide: 40,
		Tall: 60,
		Parts: []world.Part{
			{Shape: world.Cone, To: [3]float32{0, 0, 34}, Wide: 6, Tip: 4, Paint: 1},
			{Shape: world.Spheroid, From: [3]float32{0, 0, 30}, To: [3]float32{0, 0, 58}, Wide: 28},
		},
		Paints: []world.Paint{head, {Colour: postBrown}},
	}
}

// marked is a green head on a post with a red ball on the side its x runs to
func marked() world.Body {
	b := headedOf(world.Paint{Colour: headGreen})
	b.Parts = append(b.Parts, world.Part{Shape: world.Spheroid, From: [3]float32{14, 0, 38},
		To: [3]float32{14, 0, 50}, Wide: 12, Paint: 2})
	b.Paints = append(b.Paints, world.Paint{Colour: markRed})
	return b
}

// lowBody is three spheroids side by side, 40 wide and 22 tall: lower than it is wide
func lowBody() world.Body {
	const s = world.Spheroid
	return world.Body{
		Wide: 40,
		Tall: 22,
		Parts: []world.Part{
			{Shape: s, From: [3]float32{-9, -4, 3}, To: [3]float32{-9, -4, 17}, Wide: 16},
			{Shape: s, From: [3]float32{8, -6, 2}, To: [3]float32{8, -6, 16}, Wide: 18},
			{Shape: s, From: [3]float32{0, 8, 4}, To: [3]float32{0, 8, 20}, Wide: 18},
		},
		Paints: []world.Paint{{Colour: headGreen}},
	}
}

// placed is a body of the lawn's kind of that number standing at a place, turned about its
// foot, whole or as its card, swaying
type placed struct {
	at    geom.Vec
	kind  int
	turn  float32
	whole bool
	sway  float32
}

// lawn is a level board in relief under a sun and a weather, seen by an isometric camera or an
// eye, its atlas holding the bodies given as kinds
type lawn struct {
	t      *testing.T
	w      *world.Plugin
	l      *light.Plugin
	grid   grid.Grid
	relief topography.Relief
	drawn  []render.Source
	atlas  *world.Atlas
	bodies []*world.Bodied
	cam    camera.Camera
	sun    light.Distant
	air    air.Weather
}

func newLawn(t *testing.T, sun light.Distant, weather air.Weather, eye bool,
	bodies ...world.Body) *lawn {
	t.Helper()
	gputest.Need(t)
	g := &lawn{t: t, sun: sun, air: weather}
	g.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 64},
		Heights:  true,
	})
	gr := grid.DefaultGrids{}.Square(8, 8, 32)
	b := board.NewPlugin(gr, &cell.MultipleOccupancy{}, g.w)
	b.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	g.l = light.NewPlugin(g.w, light.Config{}).WithDistant(sun)
	p := topography.NewPlugin(g.w, b, g.l, topography.Config{}).WithAtmosphere(skyOf{sun})
	g.grid, g.relief = gr, p.Write().Relief()
	g.drawn = append(topotest.Drawn(t, g.l), topotest.Drawn(t, p)...)
	g.atlas = g.w.NewAtlas()
	kinds := make([]render.SpriteID, len(bodies))
	for i, body := range bodies {
		kinds[i] = g.w.Kinds().NewSprite()
		g.atlas.AddBody(kinds[i], 64, body)
	}
	g.atlas.Close()
	for _, k := range kinds {
		g.bodies = append(g.bodies, g.atlas.BodyOf(k))
	}
	g.cam = cameras.NewPlugin(g.w, cameras.Config{
		Turn:        true,
		Tilt:        true,
		Perspective: eye,
		Cell:        32,
		HeightUnit:  1,
		Cage:        p.Read(0),
	}).Write().New(cameras.Isometrically, camera.Config{})
	g.cam.SetViewport(320, 240)
	g.cam.CenterOn(128, 128, 0)
	if eye {
		cameratest.Switch(g.cam)
	}
	return g
}

// steeply has the lawn's camera look down from 50° nearer straight down than it starts, where
// every card lies
func (g *lawn) steeply() *lawn { return g.tilted(50) }

// tilted has the lawn's camera look down from that many degrees nearer straight down than it
// starts, at 30°
func (g *lawn) tilted(degrees float32) *lawn {
	g.t.Helper()
	if !cameratest.Tilt(g.cam, degrees*math.Pi/180) {
		g.t.Fatal("the lawn's camera does not tilt")
	}
	g.cam.CenterOn(128, 128, 0)
	return g
}

// Compose and Draw: the lawn's bodies of a frame, a source of the world's picture
type bodiesStanding struct {
	g     *lawn
	stood []placed
}

func (s *bodiesStanding) Compose(f *render.Frame, cam camera.Camera) {
	look := s.g.w.Read(0).Look().(world.DirectLook)
	look.Begin(f, cam)
	cut := look.(world.CutoutLook)
	cut.Cutout(true)
	for _, tr := range s.stood {
		b := s.g.bodies[tr.kind]
		box := plane.NewAABB(geom.NewVec(tr.at.X-b.Wide/2, tr.at.Y-b.Wide/2), b.Wide, b.Wide)
		look.(world.BodyLook).Body(f, cam, box, world.Z{Height: b.Tall}, s.g.atlas, b, tr.turn,
			tr.whole, render.Appearance{Sway: tr.sway}, render.WhiteLight)
	}
	cut.Cutout(false)
	f.DrawCutout()
}

func (s *bodiesStanding) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	s.g.w.Read(0).Look().(world.DirectLook).DrawSprites(t, cam, u)
}

// shows is what of the bodies shows on the screen: the pixels of green, of red and of pink,
// the columns and the rows the greens' and the red's middles stand in, and how green the
// greens' left and right halves are
type shows struct {
	greens, red, pinks int
	greensAt, redAt    float32
	greensRow, redRow  float32
	green              float32
	leftLit, rightLit  float32
}

// seen draws the bodies stood through the lawn's camera and reads the screen
func (g *lawn) seen(stood ...placed) shows {
	g.t.Helper()
	c := render.NewComposer(append([]render.Source{shining{g.sun, g.air}},
		append(g.drawn, &bodiesStanding{g, stood})...)...)
	g.l.Publish()
	g.l.Swap()
	c.Light(g.w.Read(0).Lights)
	screen := render.NewImage(320, 240)
	for range 3 {
		c.DrawWorld(screen, g.cam)
	}
	pix := make([]byte, 4*320*240)
	screen.ReadPixels(pix)
	at := func(x, y int) (r, gr, bl int) {
		i := 4 * (y*320 + x)
		return int(pix[i]), int(pix[i+1]), int(pix[i+2])
	}
	greenish := func(r, gr, bl int) bool { return gr > r+35 && gr > bl+35 }
	var s shows
	var greensX, greensY, redX, redY float32
	for y := range 240 {
		for x := range 320 {
			switch r, gr, bl := at(x, y); {
			case r > gr+60 && bl > gr+60:
				s.pinks++
			case r > gr+60 && r > bl+60:
				s.red++
				redX, redY = redX+float32(x), redY+float32(y)
			case greenish(r, gr, bl):
				s.greens++
				greensX, greensY = greensX+float32(x), greensY+float32(y)
				s.green += float32(gr)
			}
		}
	}
	if n := float32(s.greens); n > 0 {
		s.greensAt, s.greensRow, s.green = greensX/n, greensY/n, s.green/n
	}
	if n := float32(s.red); n > 0 {
		s.redAt, s.redRow = redX/n, redY/n
	}
	var left, right float32
	var lefts, rights int
	for y := range 240 {
		for x := range 320 {
			if r, gr, bl := at(x, y); greenish(r, gr, bl) {
				if float32(x) < s.greensAt {
					left, lefts = left+float32(gr), lefts+1
				} else {
					right, rights = right+float32(gr), rights+1
				}
			}
		}
	}
	s.leftLit, s.rightLit = left/float32(max(lefts, 1)), right/float32(max(rights, 1))
	return s
}

type skyOf struct{ sun light.Distant }

func (s skyOf) Distant() light.Distant { return s.sun }
func (skyOf) DrawnAir() air.Weather    { return air.Weather{} }

// hands the frame the sun and the air's weather before the ground composes
type shining struct {
	sun light.Distant
	air air.Weather
}

func (s shining) Compose(f *render.Frame, _ camera.Camera) {
	s.sun.Frame(f)
	s.air.Frame(f, s.sun, 0, 0)
}
func (shining) Draw(render.Target, camera.Camera, render.Uniforms) {}

// the sun 60° up over the screen's left and over its right: the isometric eye looks north-west
var (
	leftSun  = light.Distant{Dir: [3]float32{-0.354, 0.354, 0.866}, Strength: 1, Ambient: 0.3}
	rightSun = light.Distant{Dir: [3]float32{0.354, -0.354, 0.866}, Strength: 1, Ambient: 0.3}
)

// the eye looks north-west: turned so, the mark hangs across its look, to its right
const markRight = -math.Pi / 4

var middle = geom.NewVec(128, 128)

func TestBodies_AWholeBodyAndItsCardStandAlikeInTheSameLight(t *testing.T) {
	// a card's picture reaches under its body's foot: a low body fills the foot of its card
	g := newLawn(t, leftSun, air.Weather{}, false, marked(), lowBody())
	for kind, name := range []string{"tall body", "low body"} {
		whole := g.seen(placed{at: middle, kind: kind, turn: markRight, whole: true})
		card := g.seen(placed{at: middle, kind: kind, turn: markRight})
		if whole.greens == 0 || card.greens == 0 {
			t.Fatalf("the %s's greens show in %d pixels whole and %d as a card, want both",
				name, whole.greens, card.greens)
		}
		if lo, hi := whole.greens*6/10, whole.greens*15/10; card.greens < lo || card.greens > hi {
			t.Errorf("the %s's card's greens cover %d pixels, the whole body's %d; want them "+
				"alike", name, card.greens, whole.greens)
		}
		if d := card.greensAt - whole.greensAt; d < -2 || d > 2 {
			t.Errorf("the %s's card's greens stand about column %.1f, the whole body's about "+
				"%.1f; want them in one place", name, card.greensAt, whole.greensAt)
		}
		if d := card.green / whole.green; d < 0.9 || d > 1.12 {
			t.Errorf("the %s's card's greens are %.0f green, the whole body's %.0f; want them "+
				"lit alike", name, card.green, whole.green)
		}
	}
}

func TestBodies_AreLitFromTheSideTheSunStandsOnWholeAndAsCards(t *testing.T) {
	for _, whole := range []bool{true, false} {
		stander := placed{at: middle, whole: whole}
		from := newLawn(t, leftSun, air.Weather{}, false, marked()).seen(stander)
		if from.leftLit < from.rightLit+6 {
			t.Errorf("whole %v, the sun on the left: the greens are %.0f on their left and "+
				"%.0f on their right, want the left lit", whole, from.leftLit, from.rightLit)
		}
		from = newLawn(t, rightSun, air.Weather{}, false, marked()).seen(stander)
		if from.rightLit < from.leftLit+6 {
			t.Errorf("whole %v, the sun on the right: the greens are %.0f on their left and "+
				"%.0f on their right, want the right lit", whole, from.leftLit, from.rightLit)
		}
	}
}

func TestBodies_ShowTheSideTheCameraSeesThemFromAsTheyAreTurned(t *testing.T) {
	g := newLawn(t, leftSun, air.Weather{}, false, marked())
	// the mark hangs on the side the body's x runs to: to the eye's right, turned half round
	// to its left
	for _, whole := range []bool{true, false} {
		one := g.seen(placed{at: middle, turn: markRight, whole: whole})
		other := g.seen(placed{at: middle, turn: markRight + math.Pi, whole: whole})
		if one.red == 0 || other.red == 0 {
			t.Fatalf("whole %v: the mark shows in %d pixels, turned round in %d; want it "+
				"either way", whole, one.red, other.red)
		}
		side, turned := one.redAt-one.greensAt, other.redAt-other.greensAt
		if side < 5 || turned > -5 {
			t.Errorf("whole %v: the mark hangs %.1f columns off the greens' middle, turned "+
				"half round %.1f; want it to the right and then to the left", whole, side,
				turned)
		}
	}
}

func TestBodies_TheNearerHidesTheOneBehindItWholeAndAsCards(t *testing.T) {
	g := newLawn(t, leftSun, air.Weather{}, false, headedOf(world.Paint{Colour: headGreen}),
		headedOf(world.Paint{Colour: markRed}))
	// the eye looks from the south-east: what stands south-east stands before
	before, behind := geom.NewVec(138, 138), geom.NewVec(118, 118)
	for _, whole := range []bool{true, false} {
		redBefore := g.seen(placed{at: behind, kind: 0, whole: whole},
			placed{at: before, kind: 1, whole: whole})
		greenBefore := g.seen(placed{at: before, kind: 0, whole: whole},
			placed{at: behind, kind: 1, whole: whole})
		if redBefore.red*10 < greenBefore.red*12 || greenBefore.greens*10 < redBefore.greens*12 {
			t.Errorf("whole %v: the red body shows in %d pixels before the green and %d behind "+
				"it, the green in %d before and %d behind; want each hidden where it stands "+
				"behind", whole, redBefore.red, greenBefore.red, greenBefore.greens,
				redBefore.greens)
		}
	}
}

func TestBodies_LeanTheirTopsWithTheWindAsFarAsTheySwayWholeAndAsCards(t *testing.T) {
	east := air.Weather{Wind: [2]float32{60, 0}}
	calm := newLawn(t, leftSun, air.Weather{}, false, headedOf(world.Paint{Colour: headGreen}))
	blown := newLawn(t, leftSun, east, false, headedOf(world.Paint{Colour: headGreen}))
	for _, whole := range []bool{true, false} {
		still := calm.seen(placed{at: middle, whole: whole, sway: 1})
		stiff := blown.seen(placed{at: middle, whole: whole})
		leant := blown.seen(placed{at: middle, whole: whole, sway: 1})
		if d := stiff.greensAt - still.greensAt; d < -0.5 || d > 0.5 {
			t.Errorf("whole %v: what does not sway stands with its greens about column %.1f "+
				"in the wind, %.1f in the calm; want it upright", whole, stiff.greensAt,
				still.greensAt)
		}
		// east runs right and down the screen of a camera looking from the south-east
		if leant.greensAt < still.greensAt+2 {
			t.Errorf("whole %v: in an east wind what sways stands with its greens about "+
				"column %.1f, in the calm %.1f; want them leant east", whole, leant.greensAt,
				still.greensAt)
		}
	}
}

func TestBodies_ThroughAnEyeAWholeBodyStandsWhereItsCardDoes(t *testing.T) {
	g := newLawn(t, leftSun, air.Weather{}, true, marked())
	whole := g.seen(placed{at: middle, whole: true})
	card := g.seen(placed{at: middle})
	if whole.greens == 0 || card.greens == 0 {
		t.Fatalf("through an eye the greens show in %d pixels whole and %d as a card, want "+
			"both", whole.greens, card.greens)
	}
	if lo, hi := whole.greens*5/10, whole.greens*2; card.greens < lo || card.greens > hi {
		t.Errorf("through an eye the card's greens cover %d pixels, the whole body's %d; want "+
			"them alike", card.greens, whole.greens)
	}
	if d := card.greensAt - whole.greensAt; d < -4 || d > 4 {
		t.Errorf("through an eye the card's greens stand about column %.1f, the whole body's "+
			"about %.1f; want them in one place", card.greensAt, whole.greensAt)
	}
}

func TestBodies_AreDottedWithWhatTheirPaintSaysWholeAndAsCards(t *testing.T) {
	g := newLawn(t, leftSun, air.Weather{}, false,
		headedOf(world.Paint{Colour: headGreen, Dots: dotPink}),
		headedOf(world.Paint{Colour: headGreen}))
	for _, whole := range []bool{true, false} {
		in := g.seen(placed{at: middle, kind: 0, whole: whole})
		if in.pinks < 10 || in.pinks > in.greens {
			t.Errorf("whole %v: a body dotted pink shows %d pixels of it among %d of greens, "+
				"want a share of them", whole, in.pinks, in.greens)
		}
		if bare := g.seen(placed{at: middle, kind: 1, whole: whole}); bare.pinks != 0 {
			t.Errorf("whole %v: a body with no dots shows %d pixels of pink, want none",
				whole, bare.pinks)
		}
	}
}

func TestBodies_SeenFromSteeplyAboveACardLiesAsItsBodyShowsTurnedAsItStands(t *testing.T) {
	g := newLawn(t, leftSun, air.Weather{}, false, marked()).steeply()
	for _, turn := range []float32{0, math.Pi / 2, math.Pi, -math.Pi / 2, 2.3} {
		whole := g.seen(placed{at: middle, turn: turn, whole: true})
		card := g.seen(placed{at: middle, turn: turn})
		if whole.greens == 0 || whole.red == 0 || card.red == 0 {
			t.Fatalf("turned %v the greens show in %d pixels and the mark in %d whole, the "+
				"mark in %d as a card; want them all", turn, whole.greens, whole.red, card.red)
		}
		// a card standing would be seen from over its top, a quarter as tall
		if lo, hi := whole.greens*8/10, whole.greens*12/10; card.greens < lo || card.greens > hi {
			t.Errorf("turned %v the card's greens cover %d pixels, the whole body's %d; want "+
				"them alike", turn, card.greens, whole.greens)
		}
		if dx, dy := card.greensAt-whole.greensAt, card.greensRow-whole.greensRow; dx < -3 ||
			dx > 3 || dy < -3 || dy > 3 {
			t.Errorf("turned %v the card's greens stand (%.1f, %.1f) off the whole body's, "+
				"want them in one place", turn, dx, dy)
		}
		wx, wy := whole.redAt-whole.greensAt, whole.redRow-whole.greensRow
		cx, cy := card.redAt-card.greensAt, card.redRow-card.greensRow
		if dx, dy := cx-wx, cy-wy; dx < -4 || dx > 4 || dy < -4 || dy > 4 {
			t.Errorf("turned %v the card's mark hangs (%.1f, %.1f) off its greens, the whole "+
				"body's (%.1f, %.1f); want them alike", turn, cx, cy, wx, wy)
		}
	}
}

func TestBodies_SeenFromSteeplyAboveAreLitFromTheSideTheSunStandsOn(t *testing.T) {
	for _, whole := range []bool{true, false} {
		stander := placed{at: middle, turn: 2.3, whole: whole}
		from := newLawn(t, leftSun, air.Weather{}, false, marked()).steeply().seen(stander)
		if from.leftLit < from.rightLit+6 {
			t.Errorf("whole %v, the sun on the left: the greens are %.0f on their left and "+
				"%.0f on their right, want the left lit", whole, from.leftLit, from.rightLit)
		}
		from = newLawn(t, rightSun, air.Weather{}, false, marked()).steeply().seen(stander)
		if from.rightLit < from.leftLit+6 {
			t.Errorf("whole %v, the sun on the right: the greens are %.0f on their left and "+
				"%.0f on their right, want the right lit", whole, from.leftLit, from.rightLit)
		}
	}
}

func TestBodies_SeenFromSteeplyAboveACardLeansWithTheWindAndHidesTheOneBehindIt(t *testing.T) {
	green, red := headedOf(world.Paint{Colour: headGreen}), headedOf(world.Paint{Colour: markRed})
	east := air.Weather{Wind: [2]float32{60, 0}}
	calm := newLawn(t, leftSun, air.Weather{}, false, green, red).steeply()
	blown := newLawn(t, leftSun, east, false, green, red).steeply()
	still := calm.seen(placed{at: middle, sway: 1})
	stiff := blown.seen(placed{at: middle})
	leant := blown.seen(placed{at: middle, sway: 1})
	if d := stiff.greensAt - still.greensAt; d < -0.5 || d > 0.5 {
		t.Errorf("what does not sway lies with its greens about column %.1f in the wind, %.1f "+
			"in the calm; want it in its place", stiff.greensAt, still.greensAt)
	}
	if leant.greensAt < still.greensAt+2 {
		t.Errorf("in an east wind what sways lies with its greens about column %.1f, in the "+
			"calm %.1f; want them leant east", leant.greensAt, still.greensAt)
	}
	// the eye looks from the south-east: of two heads as high, one over the other on the
	// screen, the nearer covers the further whichever is laid first
	before, behind := geom.NewVec(134, 134), geom.NewVec(122, 122)
	redBefore := calm.seen(placed{at: behind, kind: 0}, placed{at: before, kind: 1})
	greenBefore := calm.seen(placed{at: before, kind: 0}, placed{at: behind, kind: 1})
	if redBefore.red*10 < greenBefore.red*12 || greenBefore.greens*10 < redBefore.greens*12 {
		t.Errorf("the red body shows in %d pixels before the green and %d behind it, the "+
			"green in %d before and %d behind; want each hidden where it stands behind",
			redBefore.red, greenBefore.red, greenBefore.greens, redBefore.greens)
	}
}

func TestBodies_SeenFromSteeperThanItWasPhotographedACardStandsAsTallAsItsBody(t *testing.T) {
	// photographed from 30°: standing as it was, from 57° its head would show two thirds as
	// tall
	for _, eye := range []bool{false, true} {
		for _, tilt := range []float32{15, 21} {
			g := newLawn(t, leftSun, air.Weather{}, eye, marked()).tilted(tilt)
			whole := g.seen(placed{at: middle, turn: markRight, whole: true})
			card := g.seen(placed{at: middle, turn: markRight})
			if whole.greens == 0 || whole.red == 0 {
				t.Fatalf("eye %v, tilted %v°: the whole body shows %d pixels of greens and %d "+
					"of its mark, want both", eye, tilt, whole.greens, whole.red)
			}
			if lo, hi := whole.greens*88/100, whole.greens*12/10; card.greens < lo ||
				card.greens > hi {
				t.Errorf("eye %v, tilted %v°: the card's greens cover %d pixels, the whole "+
					"body's %d; want them alike", eye, tilt, card.greens, whole.greens)
			}
			// an eye near the body sees its head a little off the middle the card leans about
			off := float32(2)
			if eye {
				off = 7
			}
			if dx, dy := card.greensAt-whole.greensAt, card.greensRow-whole.greensRow; dx < -off ||
				dx > off || dy < -off || dy > off {
				t.Errorf("eye %v, tilted %v°: the card's greens stand (%.1f, %.1f) off the "+
					"whole body's, want them in one place", eye, tilt, dx, dy)
			}
			if card.red == 0 || card.redAt < card.greensAt+4 {
				t.Errorf("eye %v, tilted %v°: the card shows %d pixels of mark about column "+
					"%.1f, its greens about %.1f; want the mark on its right, standing",
					eye, tilt, card.red, card.redAt, card.greensAt)
			}
		}
	}
}

func TestBodies_ThroughAnEyeLookingSteeplyDownACardLiesWhereItsBodyStands(t *testing.T) {
	g := newLawn(t, leftSun, air.Weather{}, true, marked()).steeply()
	whole := g.seen(placed{at: middle, turn: 2.3, whole: true})
	card := g.seen(placed{at: middle, turn: 2.3})
	if whole.greens == 0 || card.greens == 0 {
		t.Fatalf("through an eye the greens show in %d pixels whole and %d as a card, want "+
			"both", whole.greens, card.greens)
	}
	if lo, hi := whole.greens*6/10, whole.greens*16/10; card.greens < lo || card.greens > hi {
		t.Errorf("through an eye the card's greens cover %d pixels, the whole body's %d; want "+
			"them alike", card.greens, whole.greens)
	}
	if dx, dy := card.greensAt-whole.greensAt, card.greensRow-whole.greensRow; dx < -5 ||
		dx > 5 || dy < -5 || dy > 5 {
		t.Errorf("through an eye the card's greens stand (%.1f, %.1f) off the whole body's, "+
			"want them in one place", dx, dy)
	}
}
