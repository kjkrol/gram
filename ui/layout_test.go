package ui

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

var screen = geom.NewAABBAt(geom.NewVec(0, 0), 1200, 600)

func box(x, y, w, h float64) geom.AABB { return geom.NewAABBAt(geom.NewVec(x, y), w, h) }

func wantBox(t *testing.T, what string, got, want geom.AABB) {
	t.Helper()
	if got != want {
		t.Errorf("%s laid at %v, want %v", what, got, want)
	}
}

func TestColumns_TwoEqualSharesAreHalves(t *testing.T) {
	left, right := Label("a"), Label("b")
	Columns(Share(1, left), Share(1, right)).lay(screen)
	wantBox(t, "left", left.Box(), box(0, 0, 600, 600))
	wantBox(t, "right", right.Box(), box(600, 0, 600, 600))
}

func TestColumns_SharesSplitWhatTheFixedLeave(t *testing.T) {
	side, wide, narrow := Label("s"), Label("w"), Label("n")
	Columns(Fixed(300, side), Share(2, wide), Share(1, narrow)).lay(screen)
	wantBox(t, "fixed", side.Box(), box(0, 0, 300, 600))
	wantBox(t, "two shares", wide.Box(), box(300, 0, 600, 600))
	wantBox(t, "one share", narrow.Box(), box(900, 0, 300, 600))
}

func TestRows_FitTakesTheSizeAskedFor(t *testing.T) {
	bar, rest := Label("bar").Size(100, 40), Label("rest")
	Rows(Fit(bar), Share(1, rest)).lay(screen)
	wantBox(t, "bar", bar.Box(), box(0, 0, 1200, 40))
	wantBox(t, "rest", rest.Box(), box(0, 40, 1200, 560))
}

func TestAnchors_PlaceTheirElementAtTheirPointKeptOffTheEdges(t *testing.T) {
	cases := []struct {
		name   string
		anchor func(*Element) *Element
		want   geom.AABB
	}{
		{"TopLeft", TopLeft, box(10, 10, 200, 100)},
		{"TopRight", TopRight, box(990, 10, 200, 100)},
		{"Center", Center, box(500, 250, 200, 100)},
		{"BottomMiddle", BottomMiddle, box(500, 490, 200, 100)},
		{"BottomRight", BottomRight, box(990, 490, 200, 100)},
	}
	for _, c := range cases {
		e := Label(c.name)
		c.anchor(e).Size(200, 100).Margin(10).lay(screen)
		wantBox(t, c.name, e.Box(), c.want)
	}
}

func TestLayers_GiveEveryElementTheWholeBox(t *testing.T) {
	under, over := Label("under"), Label("over")
	Layers(under, over).lay(screen)
	wantBox(t, "under", under.Box(), screen)
	wantBox(t, "over", over.Box(), screen)
}

func TestHits_AContainerIsHitOnlyWhereAChildIs(t *testing.T) {
	e := Label("x")
	root := Layers(BottomRight(e).Size(100, 100))
	root.lay(screen)
	if root.Hits(geom.NewVec(10, 10)) {
		t.Error("an anchor is hit far from its element")
	}
	if !root.Hits(geom.NewVec(1150, 550)) {
		t.Error("an anchor is not hit over its element")
	}
}

func TestHits_AMaskedElementIsHitOnlyInsideItsShape(t *testing.T) {
	e := Label("clock").Masked(Circle)
	e.lay(box(0, 0, 100, 100))
	if e.Hits(geom.NewVec(3, 3)) {
		t.Error("a round element is hit in its box's corner")
	}
	if !e.Hits(geom.NewVec(50, 50)) {
		t.Error("a round element is not hit in its middle")
	}
	hex := Label("hex").Masked(Polygon(geom.NewVec(0.25, 0), geom.NewVec(0.75, 0), geom.NewVec(1, 0.5),
		geom.NewVec(0.75, 1), geom.NewVec(0.25, 1), geom.NewVec(0, 0.5)))
	hex.lay(box(0, 0, 100, 100))
	if hex.Hits(geom.NewVec(2, 2)) || !hex.Hits(geom.NewVec(50, 50)) {
		t.Error("a hexagon is hit outside it or missed inside")
	}
}

func TestHits_AHiddenElementIsHitByNothing(t *testing.T) {
	e := Label("x").Hidden()
	e.lay(screen)
	if e.Hits(geom.NewVec(1, 1)) {
		t.Error("a hidden element is hit")
	}
}

// sized is a surface that notes the size it was given.
type sized struct{ w, h int }

func (s *sized) Resize(w, h int)   { s.w, s.h = w, h }
func (*sized) Draw() *render.Image { return nil }

func TestImage_GivesItsSurfaceItsBox(t *testing.T) {
	left, right := &sized{}, &sized{}
	Columns(Share(1, Image(left)), Share(1, Image(right).Padding(5))).lay(screen)
	if left.w != 600 || left.h != 600 || right.w != 590 || right.h != 590 {
		t.Fatalf("surfaces sized %dx%d and %dx%d, want 600x600 and 590x590", left.w, left.h, right.w, right.h)
	}
}

func TestScene_ShowsHidesAndTogglesByName(t *testing.T) {
	panel := Label("panel").Named("panel").Hidden()
	s := NewScene("main", nil, func() *Element { return Layers(panel) })
	s.Layers()
	s.Show("panel")
	if panel.hidden {
		t.Fatal("Show left it hidden")
	}
	s.Toggle("panel")
	if !panel.hidden {
		t.Fatal("Toggle left it shown")
	}
	s.Hide("panel")
	s.Toggle("panel")
	if panel.hidden {
		t.Fatal("Toggle left it hidden")
	}
}

// counting is a picture that counts its Inits.
type counting struct{ inits int }

func (c *counting) Init(*goke.SysInit)                   { c.inits++ }
func (*counting) DrawWorld(*render.Image, camera.Camera) {}

func TestScene_InitialisesAPictureOnceWhateverListsIt(t *testing.T) {
	p := &counting{}
	s := NewScene("main", func() []render.Picture { return []render.Picture{p, p} },
		func() *Element { return Layers() })
	for _, l := range s.Layers() {
		l.Init(nil)
	}
	if p.inits != 1 {
		t.Fatalf("picture initialised %d times, want once", p.inits)
	}
}

// straight is a surface that notes whether it was drawn straight onto the screen.
type straight struct {
	sized
	onto bool
}

func (s *straight) DrawOn(*render.Image) { s.onto = true }

func TestImage_FillingTheScreenDrawsStraightOntoIt(t *testing.T) {
	dst := render.NewImage(1200, 600)
	whole, half := &straight{}, &straight{}
	for _, c := range []struct {
		src *straight
		e   *Element
	}{{whole, Image(whole)}, {half, Columns(Share(1, Image(half)), Share(1, Blank()))}} {
		c.e.lay(screen)
		c.e.walk(func(e *Element) {
			if _, ok := e.content.(*picture); ok {
				e.content.draw(e, dst)
			}
		})
	}
	if !whole.onto || half.onto {
		t.Fatalf("drawn straight: the whole screen's %v, the half's %v; want the whole alone", whole.onto, half.onto)
	}
}
