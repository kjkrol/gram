package render

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render/gputest"
)

// "88" in the default font by the point (x, y), its top-left dx, dy from where the point shows,
// lying at height deep
type labelSource struct {
	labels     Labels
	x, y, deep float32
	dx, dy     float32
	colour     color.RGBA
}

func (s *labelSource) Compose(f *Frame, _ camera.Camera) {
	s.labels.Reset()
	s.labels.Text(DefaultFont(), "88", s.x, s.y, 0, s.deep, s.dx, s.dy, s.colour)
	f.DrawAt(f.At(s.x, s.y, s.deep))
}

func (s *labelSource) Draw(t Target, cam camera.Camera, u Uniforms) { s.labels.Draw(t, cam, u) }

func TestLabels_LayTheFontsTextWhereTheirPointShows(t *testing.T) {
	gputest.Need(t)
	got := shot(t, &labelSource{x: 20.3, y: 40.6, dx: -3.5, dy: -20, colour: figureRed})
	want := NewImage(64, 64)
	DefaultFont().Draw(want, "88", 20.3-3.5, 40.6-20, figureRed)
	at, drawn := pixelsOf(want), 0
	for y := range 64 {
		for x := range 64 {
			if got(x, y) != at(x, y) {
				t.Fatalf("pixel (%d, %d) is %v, the font draws %v there", x, y, got(x, y),
					at(x, y))
			}
			if at(x, y)[3] != 0 {
				drawn++
			}
		}
	}
	if drawn == 0 {
		t.Fatal("the font drew nothing to compare with")
	}
}

func TestLabels_AreHiddenWhereWhatLiesNearerThanTheirDepthIs(t *testing.T) {
	gputest.Need(t)
	// the upper half of the world 10 high, the lower on the ground
	ground := new(groundSource).quad(0, 0, 64, 32, 10, groundGrey).
		quad(0, 32, 64, 64, 0, groundGrey)
	for _, tc := range []struct {
		y, deep float32
		shown   bool
	}{
		{30, 4, false},
		{30, 14, true},
		{60, 4, true},
	} {
		at := shot(t, ground, &labelSource{x: 20, y: tc.y, deep: tc.deep, dy: -20,
			colour: figureRed})
		red := 0
		for y := range 64 {
			for x := range 64 {
				if p := at(x, y); p[0] > 200 && p[1] == 0 {
					red++
				}
			}
		}
		if shown := red > 0; shown != tc.shown {
			t.Errorf("a label lying %v high by (20, %v) shows %d pixels, want shown %v",
				tc.deep, tc.y, red, tc.shown)
		}
	}
}

// a badge of shape width across in fill and rim, its middle (2, -2) from the point (30, 34), n
// in it
type badgeSource struct {
	labels    Labels
	n         int
	shape     BadgeShape
	width     float32
	fill, rim color.RGBA
}

func (s *badgeSource) Compose(f *Frame, _ camera.Camera) {
	s.labels.Reset()
	s.labels.Badge(s.n, s.shape, 30, 34, 0, 0, 2, -2, s.width, s.fill, s.rim)
	f.DrawAt(f.At(32, 32, 0))
}

func (s *badgeSource) Draw(t Target, cam camera.Camera, u Uniforms) { s.labels.Draw(t, cam, u) }

var (
	badgeBlue = color.RGBA{R: 40, G: 60, B: 200, A: 255}
	badgeRim  = color.RGBA{R: 10, G: 10, B: 10, A: 255}
)

func TestLabels_Badge_FillsItsShapeWithinItsRim(t *testing.T) {
	gputest.Need(t)
	near := func(want [4]byte) func([4]byte) bool {
		return func(p [4]byte) bool {
			for i := range p {
				if d := int(p[i]) - int(want[i]); d < -8 || d > 8 {
					return false
				}
			}
			return true
		}
	}
	fill, rim, clear := near([4]byte{40, 60, 200, 255}), near([4]byte{10, 10, 10, 255}),
		near([4]byte{})
	opaque := func(p [4]byte) bool { return p[3] > 250 }
	for _, tc := range []struct {
		shape BadgeShape
		x, y  int
		is    func([4]byte) bool
		want  string
	}{
		{BadgeDisc, 31, 31, fill, "filled"},
		{BadgeDisc, 35, 32, fill, "filled"},
		{BadgeDisc, 38, 32, rim, "the rim"},
		{BadgeDisc, 32, 25, rim, "the rim"},
		{BadgeDisc, 39, 32, clear, "clear"},
		{BadgeDisc, 37, 37, clear, "clear"},
		{BadgeDisc, 36, 35, opaque, "opaque"},
		{BadgeDiamond, 31, 31, fill, "filled"},
		{BadgeDiamond, 35, 32, fill, "filled"},
		{BadgeDiamond, 37, 32, rim, "the rim"},
		{BadgeDiamond, 36, 35, clear, "clear"},
		{BadgeDiamond, 34, 34, opaque, "opaque"},
	} {
		at := shot(t, &badgeSource{shape: tc.shape, width: 14, fill: badgeBlue, rim: badgeRim})
		if got := at(tc.x, tc.y); !tc.is(got) {
			t.Errorf("a badge of shape %d at (%d, %d) is %v, want %s", tc.shape, tc.x, tc.y,
				got, tc.want)
		}
	}
}

func TestLabels_Badge_WritesItsNumberInItsMiddleInkStandingOutOfItsFill(t *testing.T) {
	gputest.Need(t)
	for _, tc := range []struct {
		fill, rim color.RGBA
		ink       func(p [4]byte) bool
	}{
		{badgeBlue, badgeRim,
			func(p [4]byte) bool { return p[0] > 200 && p[1] > 200 && p[2] > 200 }},
		{color.RGBA{R: 240, G: 220, B: 60, A: 255}, color.RGBA{R: 120, G: 60, A: 255},
			func(p [4]byte) bool { return p[0] < 60 && p[1] < 60 && p[2] < 60 && p[3] > 200 }},
	} {
		width := BadgeWidth(88, BadgeDisc, 14)
		at := shot(t, &badgeSource{n: 88, width: width, fill: tc.fill, rim: tc.rim})
		x0, y0, x1, y1, n := 64, 64, 0, 0, 0
		for y := range 64 {
			for x := range 64 {
				if !tc.ink(at(x, y)) {
					continue
				}
				n++
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
			}
		}
		r := width / 2
		if n == 0 || float32(x0) < 32-r || float32(x1) > 32+r || float32(y0) < 32-r ||
			float32(y1) > 32+r {
			t.Fatalf("on %v %d pixels of ink from (%d, %d) to (%d, %d), want the number within "+
				"the badge %v across by (32, 32)", tc.fill, n, x0, y0, x1, y1, width)
		}
		if mx, my := x0+x1-64, y0+y1-64; mx < -2 || mx > 2 || my < -2 || my > 2 {
			t.Errorf("on %v the number's ink from (%d, %d) to (%d, %d), want its middle within "+
				"a pixel of the badge's (32, 32)", tc.fill, x0, y0, x1, y1)
		}
	}
}

func TestBadgeWidth_GrowsPastTheSizeWhereTheNumberNeedsIt(t *testing.T) {
	disc, one, two := BadgeWidth(0, BadgeDisc, 14), BadgeWidth(8, BadgeDisc, 14),
		BadgeWidth(88, BadgeDisc, 14)
	three, diamond := BadgeWidth(888, BadgeDisc, 14), BadgeWidth(88, BadgeDiamond, 14)
	if disc != 14 || one != 14 || two <= 14 || three <= two || diamond <= two {
		t.Errorf("badges %v, %v, %v, %v across holding nothing, 8, 88, 888, a diamond %v "+
			"holding 88; want 14, 14, then growing, the diamond wider", disc, one, two, three,
			diamond)
	}
	if n := testing.AllocsPerRun(100, func() { BadgeWidth(123, BadgeDisc, 14) }); n != 0 {
		t.Errorf("BadgeWidth allocates %v times, want none", n)
	}
}

// a badge 12 pixels over the point (128, 128) on the ground
type pinSource struct{ labels Labels }

func (s *pinSource) Compose(f *Frame, _ camera.Camera) {
	s.labels.Reset()
	s.labels.Badge(0, BadgeDisc, 128, 128, 0, 0, 0, -12, 14, badgeBlue, badgeRim)
	f.DrawAt(f.At(128, 128, 0))
}

func (s *pinSource) Draw(t Target, cam camera.Camera, u Uniforms) { s.labels.Draw(t, cam, u) }

func TestLabels_StandUprightOnTheirPointHiddenByWhatStandsBeforeIt(t *testing.T) {
	gputest.Need(t)
	shown := func(g *groundSource) int {
		cam := cameratest.NewFromAbove(256, 256)
		cam.(interface{ SetViewport(w, h float32) }).SetViewport(64, 64)
		cameratest.Tilt(cam, -0.6)
		screen := NewImage(64, 64)
		NewComposer(g, &pinSource{}).DrawWorld(screen, cam)
		at, n := pixelsOf(screen), 0
		for y := range 64 {
			for x := range 64 {
				if p := at(x, y); p[2] > 150 && p[0] < 80 {
					n++
				}
			}
		}
		return n
	}
	ground := func() *groundSource { return new(groundSource).quad(0, 0, 256, 256, 0, groundGrey) }
	bare := shown(ground())
	// a roof 20 high just behind the point, across where the badge shows over it
	behind := shown(ground().quad(100, 100, 156, 126, 20, groundGrey))
	// one 30 high just before it
	before := shown(ground().quad(100, 130, 156, 150, 30, groundGrey))
	if bare < 60 || behind != bare || before > bare*2/3 {
		t.Errorf("a badge over its point shows %d pixels on bare ground, %d by a roof behind the "+
			"point, %d by one before it; want all of them behind, a part before", bare, behind,
			before)
	}
}
