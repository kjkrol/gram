package world

import (
	"image/color"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

var ballGreen = color.RGBA{R: 40, G: 160, B: 60, A: 255}

// ball is a body 40 wide and tall holding one ball 2*r across, its middle off the body's by off
func ball(r float32, off [2]float32) Body {
	return Body{
		Wide: 40,
		Tall: 40,
		Parts: []Part{{
			Shape: Spheroid,
			From:  [3]float32{off[0], off[1], 20 - r},
			To:    [3]float32{off[0], off[1], 20 + r},
			Wide:  2 * r,
		}},
		Paints: []Paint{{Colour: ballGreen}},
	}
}

// shotOf is the world's sheet with body added as a kind's own, 32 a side, and seen from above
// from L3, photographed: the pixel (x, y) of a sprite, and the kind's body
func shotOf(t *testing.T, body Body) (at func(id render.SpriteID, x, y int) color.RGBA,
	own, above render.SpriteID, b *Bodied) {
	t.Helper()
	gputest.Need(t)
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 100, MinSize: 1, MaxSize: 100},
	})
	a := p.NewAtlas()
	own = p.Kinds().NewSprite()
	a.AddBody(own, 32, body).Above(lod.L3)
	a.Close()
	if err := render.Prepare(); err != nil {
		t.Fatal(err)
	}
	sheet := a.Atlas()
	w, h := sheet.Bounds().Dx(), sheet.Bounds().Dy()
	pix := make([]byte, 4*w*h)
	sheet.ReadPixels(pix)
	above, _ = a.levels.at(own, lod.L3)
	if b := a.bodied[own]; above != b.Above {
		t.Fatalf("from L3 the kind is drawn as the sprite %d, want its body seen from above, "+
			"%d", above, b.Above)
	}
	return func(id render.SpriteID, x, y int) color.RGBA {
		x0, y0, _, _ := a.UV(id)
		i := 4 * ((int(y0)+y)*w + int(x0) + x)
		return color.RGBA{R: pix[i], G: pix[i+1], B: pix[i+2], A: pix[i+3]}
	}, own, above, a.bodied[own]
}

func TestBody_ItsOwnSpriteIsItPhotographedFromTheSideInItsTones(t *testing.T) {
	at, own, above, _ := shotOf(t, ball(20, [2]float32{}))
	for _, id := range []render.SpriteID{own, above} {
		if c := at(id, 16, 16); c.A != 255 || c.G < c.R+60 {
			t.Errorf("sprite %d holds %v in its middle, want the ball's green", id, c)
		}
		for _, p := range [][2]int{{1, 1}, {30, 1}, {1, 30}, {30, 30}} {
			if c := at(id, p[0], p[1]); c.A != 0 {
				t.Errorf("sprite %d holds %v in its corner %v, want nothing past the ball", id,
					c, p)
			}
		}
		// the photograph's light comes from the left and over it
		if lit, dark := at(id, 9, 9), at(id, 23, 23); int(lit.G) < int(dark.G)+30 {
			t.Errorf("sprite %d is %v up on its left and %v down on its right, want the "+
				"first lit and the second in the shade", id, lit, dark)
		}
	}
}

func TestBody_ItsCardsHoldItsPaintAndTheWaysOutOfIt(t *testing.T) {
	at, _, _, b := shotOf(t, ball(12, [2]float32{}))
	// the one from above the last
	for k := range BodyViews + 1 {
		view, ways := b.Top, b.TopWays
		if k < BodyViews {
			view, ways = b.Views[k], b.Ways[k]
		}
		// the picture reaches a margin under the body's foot and over its top: its middle row is
		// still the body's middle height
		if c := at(view, 16, 16); c != ballGreen {
			t.Errorf("view %d holds %v in its middle, want the paint %v untoned", k, c, ballGreen)
		}
		if c := at(view, 2, 2); c.A != 0 {
			t.Errorf("view %d holds %v in its corner, want nothing", k, c)
		}
		mid, right, up := at(ways, 16, 16), at(ways, 21, 16), at(ways, 16, 11)
		if mid.A != 255 || mid.R < 118 || mid.R > 138 || mid.G < 118 || mid.G > 138 {
			t.Errorf("the ways of view %d hold %v in the middle, want one straight at the eye, "+
				"a half across and a half up", k, mid)
		}
		if right.R < mid.R+40 || up.G < mid.G+40 {
			t.Errorf("the ways of view %d hold %v right of the middle and %v over it, want "+
				"the first out to the right and the second up", k, right, up)
		}
	}
}

// middleOf is the column and the row the middle of what the sprite id holds stands in
func middleOf(t *testing.T, at func(id render.SpriteID, x, y int) color.RGBA,
	id render.SpriteID) (col, row float32) {
	t.Helper()
	var n float32
	for y := range 32 {
		for x := range 32 {
			if at(id, x, y).A == 255 {
				col, row, n = col+float32(x)+0.5, row+float32(y)+0.5, n+1
			}
		}
	}
	if n == 0 {
		t.Fatalf("sprite %d holds nothing", id)
	}
	return col / n, row / n
}

func TestBody_ItsCardsShowItFromEightSides(t *testing.T) {
	// a ball off the body's middle along its x, the y running south as the world's: straight
	// ahead and behind looking along x, to the left looking along y, to the right looking back
	// along it
	at, _, _, b := shotOf(t, ball(6, [2]float32{10, 0}))
	for k, want := range map[int]float32{0: 16, 2: 8, 4: 16, 6: 24} {
		if got, _ := middleOf(t, at, b.Views[k]); got < want-1.5 || got > want+1.5 {
			t.Errorf("view %d shows the ball about column %.1f, want about %v", k, got, want)
		}
	}
	one, _ := middleOf(t, at, b.Views[1])
	three, _ := middleOf(t, at, b.Views[3])
	if one > 13 || three > 13 {
		t.Errorf("views 1 and 3 show the ball about columns %.1f and %.1f, want both to the "+
			"left of the middle", one, three)
	}
}

func TestBody_ItsCardFromAboveShowsItAsItLiesUnderTheEye(t *testing.T) {
	// the body's x runs across the picture and its y, south, down it, as the world lies from
	// above
	for off, want := range map[[2]float32][2]float32{
		{10, 0}:  {24, 16},
		{0, 10}:  {16, 24},
		{-10, 0}: {8, 16},
	} {
		at, _, above, b := shotOf(t, ball(6, off))
		for _, id := range []render.SpriteID{b.Top, b.TopWays, above} {
			col, row := middleOf(t, at, id)
			if col < want[0]-1.5 || col > want[0]+1.5 || row < want[1]-1.5 || row > want[1]+1.5 {
				t.Errorf("a ball %v off the body's middle shows about (%.1f, %.1f) of the "+
					"sprite %d, want about %v", off, col, row, id, want)
			}
		}
	}
}

func TestBodied_ItsCardIsItsSideNearestTheEyesWayOrFromSteeplyAboveTheOneThatLies(t *testing.T) {
	// the one from above for an eye looking down steeper than 60°
	b := &Bodied{
		Views:   [BodyViews]render.SpriteID{10, 11, 12, 13, 14, 15, 16, 17},
		Ways:    [BodyViews]render.SpriteID{20, 21, 22, 23, 24, 25, 26, 27},
		Top:     30,
		TopWays: 31,
		steep:   3,
	}
	for _, c := range []struct {
		way  [3]float32
		turn float32
		view render.SpriteID
	}{
		{way: [3]float32{1, 0, -0.5}, view: 10},
		{way: [3]float32{0, 3, -1}, view: 12},
		{way: [3]float32{0, 3, -1}, turn: math.Pi / 2, view: 10},
		{way: [3]float32{-1, 0, 0}, view: 14},
		{way: [3]float32{1, -1, 0}, view: 17},
		{way: [3]float32{1, 0.1, 0}, turn: -math.Pi / 4, view: 11},
		{way: [3]float32{1, 0, -1.7}, view: 10},
		{way: [3]float32{1, 0, -1.76}, view: 30},
		{way: [3]float32{0, 0, -1}, turn: 2, view: 30},
		// from under it, however steeply, it stands
		{way: [3]float32{0.1, 0, 5}, view: 10},
	} {
		view, ways, lies := b.Card(c.way, c.turn)
		if view != c.view || ways != c.view+10 && !lies || lies != (c.view == b.Top) ||
			lies && ways != b.TopWays {
			t.Errorf("looked at along %v, turned %v, its card is the sprite %d with the ways "+
				"%d, lying %v; want the sprite %d", c.way, c.turn, view, ways, lies, c.view)
		}
	}
	if _, _, lies := (&Bodied{}).Card([3]float32{0, 0, -1}, 0); lies {
		t.Error("a body told no steepness lies, seen from above; want it to stand")
	}
}

func TestBodied_TheAngleItsSideIsPickedByStaysWithinATwoHundredthOfAnEighth(t *testing.T) {
	const steps = 3600
	for i := range steps {
		at := (float64(i) + 0.5) * 2 * math.Pi / steps
		got := float64(eighths(float32(3*math.Cos(at)), float32(3*math.Sin(at))))
		if want := at / (math.Pi / 4); math.Abs(got-want) > 0.005 {
			t.Fatalf("the angle of %.2f degrees is %.4f eighths of a turn; want %.4f",
				at*180/math.Pi, got, want)
		}
	}
	for k := range BodyViews {
		at := float64(k) * math.Pi / 4
		got := eighths(float32(math.Cos(at)), float32(math.Sin(at)))
		if d := got - float32(k); d > 1e-4 || d < -1e-4 {
			t.Errorf("the angle of %d eighths of a turn is %v", k, got)
		}
	}
	if got := eighths(0, 0); got != 0 {
		t.Errorf("the angle of no way at all is %v; want 0", got)
	}
}

func TestBody_ItsCardStandsGrownTillTheOneFromAboveShowsItTruer(t *testing.T) {
	post := Part{Shape: Cone, To: [3]float32{0, 0, 34}, Wide: 6, Tip: 4, Paint: 1}
	paints := []Paint{{Colour: ballGreen}, {Colour: ballGreen}}
	// the degrees an eye looks down from for the card from above
	from := func(b Body) float64 {
		return math.Atan(math.Sqrt(float64(b.steep(bodyElevation)))) * 180 / math.Pi
	}
	// a ball is as tall on the screen from anywhere: its card grows up its plane as the plane
	// turns on edge to the eye
	round := ball(20, [2]float32{})
	for _, deg := range []float64{30, 45, 60} {
		e := deg * math.Pi / 180
		want := math.Cos(bodyElevation) / math.Cos(e)
		if got := round.grown(bodyElevation, e); math.Abs(got-want) > 1e-3 {
			t.Errorf("from %v° a ball's card stands %.3f times as tall, want %.3f", deg, got, want)
		}
	}
	if got := round.grown(bodyElevation, 0); got != 1 {
		t.Errorf("seen level a ball's card stands %.3f times as tall, want it as photographed",
			got)
	}
	// a head on a post: grown, a standing card keeps the head's height, the one from above
	// loses the post
	headed := Body{Wide: 40, Tall: 60, Paints: paints, Parts: []Part{
		post,
		{Shape: Spheroid, From: [3]float32{0, 0, 30}, To: [3]float32{0, 0, 58}, Wide: 28},
	}}
	if got := from(headed); got < 60 {
		t.Errorf("the card from above of a head on a post takes over from %.0f°, want it "+
			"standing well past the resting view's 35°, to 60° at the least", got)
	}
	// tiers one over another: from above they fall into one disc
	tiered := Body{Wide: 30, Tall: 70, Paints: paints, Parts: []Part{
		post,
		{Shape: Cone, From: [3]float32{0, 0, 12}, To: [3]float32{0, 0, 40}, Wide: 30, Tip: 6},
		{Shape: Cone, From: [3]float32{0, 0, 30}, To: [3]float32{0, 0, 56}, Wide: 22, Tip: 4},
		{Shape: Cone, From: [3]float32{0, 0, 48}, To: [3]float32{0, 0, 70}, Wide: 14},
	}}
	if got, head := from(tiered), from(headed); got < head || got > 70.5 {
		t.Errorf("the card from above of tiers on a post takes over from %.0f°, of a head on "+
			"one from %.0f°; want the tiers standing as long, and no card past 70°", got, head)
	}
	// low and wide, spheroids side by side, the ground hiding what of a standing card reaches
	// under its foot: true in the resting view, the one from above truer soon after
	low := Body{Wide: 40, Tall: 22, Paints: paints, Parts: []Part{
		{Shape: Spheroid, From: [3]float32{-9, -4, 3}, To: [3]float32{-9, -4, 17}, Wide: 16},
		{Shape: Spheroid, From: [3]float32{8, -6, 2}, To: [3]float32{8, -6, 16}, Wide: 18},
		{Shape: Spheroid, From: [3]float32{0, 8, 4}, To: [3]float32{0, 8, 20}, Wide: 18},
	}}
	if got := from(low); got < 38 || got > 55 {
		t.Errorf("a low body's card from above takes over from %.0f°, want it standing in the "+
			"resting view and lying well before a tall one's", got)
	}
}

func TestBody_AKindAddedAsOneIsACutoutAndFarOffItsPaintsMean(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 100, MinSize: 1, MaxSize: 100},
	})
	a := p.NewAtlas()
	own := p.Kinds().NewSprite()
	a.AddBody(own, 8, ball(10, [2]float32{}))
	a.Close()
	if !a.cutouts()[own] {
		t.Error("a kind added as a body is no cutout, want it drawn whole in the cutout pass")
	}
	want := [4]uint8{ballGreen.R, ballGreen.G, ballGreen.B, 255}
	if c, ok := a.farColour(own); !ok || c != want {
		t.Errorf("far off the kind is %v, %v; want its one paint %v", c, ok, ballGreen)
	}
	if b := a.BodyOf(own); b == nil || b.Table == nil || b.Parts != 1 || b.Wide != 40 {
		t.Errorf("the kind's body is %+v, want its one part, 40 wide, with the bodies' table", b)
	}
	if a.BodyOf(p.Kinds().NewSprite()) != nil {
		t.Error("a sprite of no body has one")
	}
}

func TestBody_OneThatDoesNotHoldTogetherPanics(t *testing.T) {
	whole := ball(10, [2]float32{})
	for name, spoil := range map[string]func(b *Body){
		"with no part":            func(b *Body) { b.Parts = nil },
		"with too many parts":     func(b *Body) { b.Parts = make([]Part, MaxParts+1) },
		"with no paint":           func(b *Body) { b.Paints = nil },
		"with a part in no paint": func(b *Body) { b.Parts[0].Paint = 1 },
		"with a part of no width": func(b *Body) { b.Parts[0].Wide = 0 },
		"past its width":          func(b *Body) { b.Parts[0].From[0] = 50 },
		"over its height":         func(b *Body) { b.Parts[0].To[2] = 45 },
		"of no size":              func(b *Body) { b.Wide = 0 },
		"with a part of no shape": func(b *Body) { b.Parts[0].Shape = Box + 1 },
		"with a box of no thickness": func(b *Body) {
			b.Parts[0] = Part{Shape: Box, To: [3]float32{0, 0, 30}, Wide: 10}
		},
		"with a box's corner past its width": func(b *Body) {
			b.Parts[0] = Part{Shape: Box, From: [3]float32{-15, 0, 5}, To: [3]float32{15, 0, 5},
				Wide: 50, Tip: 4}
		},
	} {
		body := whole
		body.Parts = append([]Part(nil), whole.Parts...)
		spoil(&body)
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("a body %s was taken, want a panic", name)
				}
			}()
			body.Check("a test's")
		}()
	}
}

func TestBody_APartHoldsTogetherAsFarAsItReaches(t *testing.T) {
	// each up to half the body's width off its middle across or deep, no further
	for name, part := range map[string]Part{
		"a box lying along x": {Shape: Box, From: [3]float32{-15, 0, 5},
			To: [3]float32{15, 0, 5}, Wide: 12, Tip: 4},
		"a box standing as wide and tall as the body": {Shape: Box, To: [3]float32{0, 0, 40},
			Wide: 40, Tip: 40},
		"a cylinder lying along x to its ends": {Shape: Cone, From: [3]float32{-20, 0, 10},
			To: [3]float32{20, 0, 10}, Wide: 8, Tip: 8},
		"a spheroid drawn out along y": {Shape: Spheroid, From: [3]float32{0, -20, 10},
			To: [3]float32{0, 20, 10}, Wide: 10},
	} {
		body := ball(10, [2]float32{})
		body.Parts = []Part{part}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panics: %v; want it taken", name, r)
				}
			}()
			body.Check("a test's")
		}()
	}
}

// crate is a body 40 wide and tall holding a box in the paint given, 24 across x, 12 along y
// and 30 tall
func crate(paint Paint) Body {
	return Body{
		Wide: 40,
		Tall: 40,
		Parts: []Part{{Shape: Box, From: [3]float32{0, 0, 5}, To: [3]float32{0, 0, 35}, Wide: 24,
			Tip: 12}},
		Paints: []Paint{paint},
	}
}

func TestBody_ABoxShowsItsFacesFlatAndFromAboveAsWideAsItIs(t *testing.T) {
	at, _, _, b := shotOf(t, crate(Paint{Colour: ballGreen}))
	// from above 0.8 of a pixel a unit: 9.6 either way across x, 4.8 along y
	for _, p := range [][2]int{{16, 16}, {24, 16}, {8, 16}, {16, 19}, {16, 13}} {
		if c := at(b.Top, p[0], p[1]); c != ballGreen {
			t.Errorf("from above the box holds %v at %v, want its paint", c, p)
		}
		if c := at(b.TopWays, p[0], p[1]); c.R < 126 || c.R > 130 || c.G < 126 || c.G > 130 ||
			c.B != 0 {
			t.Errorf("from above the ways out of the box are %v at %v, want its top's, straight "+
				"at the eye", c, p)
		}
	}
	for _, p := range [][2]int{{28, 16}, {4, 16}, {16, 23}, {16, 9}} {
		if c := at(b.Top, p[0], p[1]); c.A != 0 {
			t.Errorf("from above the box holds %v at %v, want nothing past it", c, p)
		}
	}
	// looking along x from the cards' elevation: its face to the eye level, its top over it
	front, lower, top := at(b.Ways[0], 16, 16), at(b.Ways[0], 16, 21), at(b.Ways[0], 16, 9)
	if d := int(front.G) - int(lower.G); front.A != 255 || d < -2 || d > 2 || front.G > 90 ||
		front.R < 124 || front.R > 132 {
		t.Errorf("the ways out of the box's face to the eye are %v and %v down it, want one "+
			"and the same, level, a quarter up the picture", front, lower)
	}
	if top.G < 220 {
		t.Errorf("the ways out of the box's top are %v, want it facing up", top)
	}
}

func TestBody_APaintThatShinesIsTakenAloneFromAboveWhereNothingHidesIt(t *testing.T) {
	glass := color.RGBA{R: 250, G: 220, B: 120, A: 255}
	lantern := Body{
		Wide: 40,
		Tall: 40,
		Parts: []Part{
			// a ball of glass under a cap 10 a side
			{Shape: Spheroid, From: [3]float32{0, 0, 10}, To: [3]float32{0, 0, 30}, Wide: 20,
				Paint: 1},
			{Shape: Box, From: [3]float32{0, 0, 32}, To: [3]float32{0, 0, 34}, Wide: 10,
				Tip: 10},
		},
		Paints: []Paint{{Colour: ballGreen}, {Colour: glass, Shines: true}},
	}
	at, _, _, b := shotOf(t, lantern)
	if !b.Shines {
		t.Fatal("a body with a paint that shines does not say it shines")
	}
	// 0.8 of a pixel a unit from above: the cap 4 either way of the middle, the ball 8
	if c := at(b.Glow, 16, 16); c.A != 0 {
		t.Errorf("under the cap its glow holds %v, want nothing: the cap hides the glass", c)
	}
	if c := at(b.Glow, 22, 16); c != glass {
		t.Errorf("beside the cap its glow holds %v, want the glass as it is, %v", c, glass)
	}
	if c := at(b.Glow, 26, 16); c.A != 0 {
		t.Errorf("past the ball its glow holds %v, want nothing", c)
	}
	if glows, plain := at(b.TopWays, 22, 16), at(b.TopWays, 16, 16); glows.B != 255 ||
		plain.B != 0 {
		t.Errorf("the ways from above are %v on the glass and %v on the cap, want the glass's "+
			"blue whole and the cap's none", glows, plain)
	}
	if lit := at(b.Above, 22, 16); lit.R < glass.R-1 || lit.G < glass.G-1 || lit.B < glass.B-1 {
		t.Errorf("seen from above in the photograph's light the glass is %v, want no darker "+
			"than %v", lit, glass)
	}
	if _, _, _, plain := shotOf(t, ball(10, [2]float32{})); plain.Shines {
		t.Error("a body of plain paints says it shines")
	}
}

func TestBody_FarOffIsItsPaintsByHowMuchOfItEachIs(t *testing.T) {
	body := Body{
		Wide: 40,
		Tall: 40,
		Parts: []Part{
			{Shape: Spheroid, From: [3]float32{0, 0, 20}, To: [3]float32{0, 0, 40}, Wide: 20},
			{Shape: Cone, To: [3]float32{0, 0, 20}, Wide: 2, Tip: 2, Paint: 1},
		},
		Paints: []Paint{{Colour: ballGreen}, {Colour: color.RGBA{R: 120, G: 80, B: 50, A: 255}}},
	}
	// the spheroid 20 by 20, the cone 20 by 2
	if got := body.mean(); got.G < 150 || got.R > 50 || got.A != 255 {
		t.Errorf("far off the body is %v, want nearly its spheroid's green", got)
	}
	// from above the spheroid covers nearly all that shows: its middle stands 30 high
	if got := body.topAt(); got < 29 || got > 30 {
		t.Errorf("the middle of what the body shows from above stands %v high, want nearly its "+
			"spheroid's 30", got)
	}
}

type bodyCounting struct {
	*counting
	whole, cards []*Bodied
	turns        []float32
}

func (c *bodyCounting) Body(_ *render.Frame, _ camera.Camera, _ plane.AABB, _ Z,
	_ render.AtlasSource, b *Bodied, turn float32, whole bool, _ render.Appearance,
	_ render.Illumination) {
	c.turns = append(c.turns, turn)
	if whole {
		c.whole = append(c.whole, b)
		return
	}
	c.cards = append(c.cards, b)
}

// stoodBy draws one entity of the sprite 1, which has the body b, headed dir, through cam as a
// look with the body's seam lays it, the bodies reaching reach
func stoodBy(t *testing.T, cam camera.Camera, b *Bodied, reach lod.Level,
	dir geom.Vec) *bodyCounting {
	t.Helper()
	look := &bodyCounting{counting: &counting{Look: ilook.NewFlat(1000, 1000, false, false)}}
	drawAsThrough(t, cam, look, func(r *renderer) {
		r.bodies(map[render.SpriteID]*Bodied{1: b})
		r.bodiesReach = func() lod.Level { return reach }
	}, nil, nil, dir, quarters[0])
	return look
}

func TestRenderer_Compose_AKindWithABodyStandsAsItInReliefWholeAsFarAsTheBodiesReach(t *testing.T) {
	b := &Bodied{Row: 3, Above: 7}
	above := cameratest.NewFromSpace(1000, 1000, 0)
	relief := reliefCamera{above}
	// the view sees the entity at Standard
	for reach, whole := range map[lod.Level]bool{lod.L1: false, lod.L2: true, lod.L3: true} {
		look := stoodBy(t, relief, b, reach, geom.Vec{})
		stood, cards := len(look.whole), len(look.cards)
		if whole && (stood != 1 || cards != 0 || look.whole[0] != b) ||
			!whole && (stood != 0 || cards != 1 || look.cards[0] != b) {
			t.Errorf("with the bodies reaching L%d it stood whole %d times and as a card %d, "+
				"want whole %v", reach, stood, cards, whole)
		}
		if len(look.sprites) != 0 {
			t.Errorf("with the bodies reaching L%d its sprite was drawn too: %v", reach,
				look.sprites)
		}
	}
	// headed south, a quarter of a turn from x towards y: an Angle runs against the clock
	if look := stoodBy(t, above, b, lod.L3, geom.NewVec(0, 1)); len(look.whole)+
		len(look.cards) != 0 || len(look.sprites) != 1 || look.sprites[0] != b.Above ||
		look.angles[0] != -90 {
		t.Errorf("from above it stood as its body %d times and was drawn as %v turned %v, "+
			"want it seen from above alone, turned a quarter with the clock",
			len(look.whole)+len(look.cards), look.sprites, look.angles)
	}
	plain := &counting{Look: ilook.NewFlat(1000, 1000, false, false)}
	drawAsThrough(t, relief, plain, func(r *renderer) {
		r.bodies(map[render.SpriteID]*Bodied{1: b})
	}, nil, nil, geom.Vec{}, quarters[0])
	if len(plain.sprites) != 1 {
		t.Errorf("a look standing no bodies drew %v, want its sprite", plain.sprites)
	}
}

func TestRenderer_Compose_FromAboveTheGlowOfABodyThatShinesLiesOverItTurnedAlike(t *testing.T) {
	b := &Bodied{Row: 3, Above: 7, Shines: true, Glow: 9}
	above := cameratest.NewFromSpace(1000, 1000, 0)
	look := stoodBy(t, above, b, lod.L3, geom.NewVec(0, 1))
	if len(look.sprites) != 2 || look.sprites[0] != b.Above || look.sprites[1] != b.Glow ||
		look.angles[0] != look.angles[1] || look.boxes[0] != look.boxes[1] {
		t.Errorf("from above a body that shines was drawn as %v turned %v over %v, want it "+
			"seen from above and its glow after it, turned and placed alike", look.sprites,
			look.angles, look.boxes)
	}
	b.Shines = false
	if look := stoodBy(t, above, b, lod.L3, geom.NewVec(0, 1)); len(look.sprites) != 1 {
		t.Errorf("from above a body of plain paints was drawn as %v, want it seen from above "+
			"alone", look.sprites)
	}
}

func TestRenderer_Compose_ABodyIsTurnedTheWayItsEntityIsHeadedOrItsOwnWay(t *testing.T) {
	relief := reliefCamera{cameratest.NewFromSpace(1000, 1000, 0)}
	b := &Bodied{}
	for dir, want := range map[geom.Vec]float32{
		geom.NewVec(1, 0):  0,
		geom.NewVec(0, 1):  math.Pi / 2,
		geom.NewVec(-1, 0): math.Pi,
	} {
		look := stoodBy(t, relief, b, lod.L3, dir)
		if len(look.turns) != 1 || math.Abs(float64(look.turns[0]-want)) > 1e-5 {
			t.Errorf("headed %v it is turned %v, want %v", dir, look.turns, want)
		}
	}
	first := stoodBy(t, relief, b, lod.L3, geom.Vec{}).turns
	again := stoodBy(t, relief, b, lod.L3, geom.Vec{}).turns
	if len(first) != 1 || first[0] != again[0] || first[0] < 0 || first[0] >= 2*math.Pi {
		t.Errorf("standing with no heading it is turned %v, then %v; want one turn within a "+
			"whole one, the same every time", first, again)
	}
	if elsewhere := turnOf(geom.Vec{}, geom.NewVec(431, 77)); elsewhere == first[0] {
		t.Errorf("standing elsewhere it is turned %v too, want its own way", elsewhere)
	}
}

func TestBodies_TheirReachIsSetAndDrawnInByTheFrameBudgetAndLetOutAgain(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 100, MinSize: 1, MaxSize: 100},
	})
	a := p.NewAtlas()
	a.AddBody(p.Kinds().NewSprite(), 8, ball(10, [2]float32{}))
	a.Close()
	lever := p.Bodies()
	if got := p.read.BodiesReach(); got != lod.L3 || !lever.Following() {
		t.Fatalf("the bodies start reaching L%d, following %v; want L3 and following", got,
			lever.Following())
	}
	for _, want := range []lod.Level{lod.L2, lod.L1} {
		if !lever.Coarser() || p.read.BodiesReach() != want {
			t.Fatalf("drawn in, the bodies reach L%d, want L%d", p.read.BodiesReach(), want)
		}
	}
	if lever.Coarser() {
		t.Error("the bodies were drawn in past the close scale")
	}
	for _, want := range []lod.Level{lod.L2, lod.L3} {
		if !lever.Finer() || p.read.BodiesReach() != want {
			t.Fatalf("let out, the bodies reach L%d, want L%d", p.read.BodiesReach(), want)
		}
	}
	if lever.Finer() {
		t.Error("the bodies were let out past the furthest cards")
	}
	// set to the standard scale, the budget draws them in from there and lets them out to it
	p.write.SetBodies(lod.L2)
	if !lever.Coarser() || lever.Coarser() || p.read.BodiesReach() != lod.L1 {
		t.Errorf("set to L2 and drawn in, the bodies reach L%d, want L1 and no nearer",
			p.read.BodiesReach())
	}
	if !lever.Finer() || lever.Finer() || p.read.BodiesReach() != lod.L2 {
		t.Errorf("let out again, the bodies reach L%d, want the L2 they were set to",
			p.read.BodiesReach())
	}
	p.write.AdaptBodies(false)
	if lever.Following() {
		t.Error("the bodies follow the frame budget after they were told to stay")
	}
}

func TestBodies_AReachLetOutAndDrawnInAtOnceWaitsEverLongerToBeLetOutAgain(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 100, MinSize: 1, MaxSize: 100},
	})
	a := p.NewAtlas()
	a.AddBody(p.Kinds().NewSprite(), 8, ball(10, [2]float32{}))
	a.Close()
	clock := time.Unix(1000, 0)
	p.now = func() time.Time { return clock }
	after := func(d time.Duration) { clock = clock.Add(d) }
	lever := p.Bodies()
	lever.Coarser()
	lever.Coarser()
	// each try costs the budget: drawn in again three seconds on
	for _, wait := range []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute} {
		if !lever.Finer() || p.read.BodiesReach() != lod.L2 {
			t.Fatalf("the wait before %v over, the bodies reach L%d; want them let out to L2",
				wait, p.read.BodiesReach())
		}
		after(3 * time.Second)
		lever.Coarser()
		after(wait - 4*time.Second)
		if lever.Finer() {
			t.Fatalf("the bodies were let out again %v after a try that cost the budget, "+
				"want %v waited", wait-time.Second, wait)
		}
		after(time.Second)
	}
	// a try that holds clears the wait: the next level out at once
	if !lever.Finer() {
		t.Fatal("the bodies were not let out after the wait")
	}
	after(11 * time.Second)
	if !lever.Finer() || p.read.BodiesReach() != lod.L3 {
		t.Errorf("a try held, the bodies reach L%d; want them let out to L3 at once",
			p.read.BodiesReach())
	}
	// drawn in long after, by a heavier picture: no try failed, so none is waited for
	after(time.Minute)
	lever.Coarser()
	if !lever.Finer() {
		t.Error("the bodies wait to be let out after a reach that held was drawn in")
	}
	// the wait is sixteen minutes at the most, and a reach set anew waits for none
	for range 8 {
		after(time.Second)
		lever.Coarser()
		after(time.Hour)
		lever.Finer()
	}
	after(time.Second)
	lever.Coarser()
	after(16*time.Minute - 2*time.Second)
	if lever.Finer() {
		t.Error("the bodies were let out under sixteen minutes after their ninth try in a row")
	}
	after(2 * time.Second)
	if !lever.Finer() {
		t.Error("the bodies wait over sixteen minutes to be let out")
	}
	after(time.Second)
	lever.Coarser()
	p.write.SetBodies(lod.L3)
	lever.Coarser()
	if !lever.Finer() {
		t.Error("set anew, the bodies wait to be let out")
	}
}

func TestBodies_TheirReachIsASettingUnderGraphicsWithNoKeyAndItsCommandsSetIt(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	settings := func() map[string]control.Setting {
		labels := map[string]control.Setting{}
		for _, s := range r.p.Settings() {
			switch s := s.(type) {
			case control.Choice:
				labels[s.Label] = s
			case control.Switch:
				labels[s.Label] = s
			}
		}
		return labels
	}
	if _, ok := settings()["Bodies stand whole"]; ok {
		t.Fatal("a world with no body has the bodies' setting")
	}
	a := r.p.NewAtlas()
	a.AddBody(r.p.Kinds().NewSprite(), 8, ball(10, [2]float32{}))
	a.Close()
	labels := settings()
	reach, ok := labels["Bodies stand whole"].(control.Choice)
	if !ok || reach.Section != control.Graphics || len(reach.Options) != 3 || reach.Get() != 2 ||
		reach.Set(0) != (Bodies{Reach: lod.L1}) || reach.Set(2) != (Bodies{Reach: lod.L3}) {
		t.Errorf("the bodies' reach is the setting %+v, want a choice of three in Graphics, "+
			"the furthest, setting Bodies from L1 to L3", reach)
	}
	follows, ok := labels["Bodies follow the frame budget"].(control.Switch)
	if !ok || follows.Section != control.Graphics || !follows.Get() ||
		follows.Set(false) != (AdaptBodies{On: false}) {
		t.Errorf("the bodies' following is the setting %+v, want a switch in Graphics, on, "+
			"setting AdaptBodies", follows)
	}
	for _, k := range r.p.DefaultBindings() {
		if k.Type() == reflect.TypeFor[Bodies]() || k.Type() == reflect.TypeFor[AdaptBodies]() {
			t.Errorf("%v is bound to %v, want the settings screen alone", k.Type(), k.Trigger)
		}
	}
	r.p.module.commands.Put(command.Nobody, Bodies{Reach: lod.L2})
	r.p.module.commands.Put(command.Nobody, AdaptBodies{On: false})
	r.run()
	if got := r.p.read.BodiesReach(); got != lod.L2 || r.p.Bodies().Following() ||
		labels["Bodies stand whole"].(control.Choice).Get() != 1 {
		t.Errorf("after the commands the bodies reach L%d, following %v; want L2, set and still",
			got, r.p.Bodies().Following())
	}
}
