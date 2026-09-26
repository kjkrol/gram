package render

import (
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
)

// sheet is an AtlasSource of one 32x32 sprite with no image behind it.
type sheet struct{ name string }

func (sheet) Atlas() *ebiten.Image                     { return nil }
func (sheet) UV(SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 32, 32 }
func (sheet) White() (u, v float32)                    { return 40, 40 }

// items is a Source composing whatever its function says.
type items func(f *Frame)

func (items) Init(*goke.SysInit)                  {}
func (s items) Compose(f *Frame, _ camera.Camera) { s(f) }

var (
	white = color.RGBA{255, 255, 255, 255}
	black = color.RGBA{0, 0, 0, 255}
)

var unit = Corners{{0, 0}, {1, 0}, {0, 1}, {1, 1}}

func topDown() camera.Camera { return icamera.NewFromSpace(1024, 1024, 0) }

// sorted is a camera whose projection sorts by depth, as a view with height does.
func sorted() camera.Camera { return sortingCamera{icamera.NewFromSpace(1024, 1024, 0)} }

type sortingCamera struct{ camera.Camera }

func (sortingCamera) Projection() camera.Projection { return sortingProjection{} }

type sortingProjection struct{ camera.TopDown }

func (sortingProjection) Sorts() bool { return true }

// drawn composes the sources through cam and lists the items in the order they are drawn, as
// tier:depth.
func drawn(cam camera.Camera, sources ...Layer) (*Composer, []item) {
	c := NewComposer(sources...)
	c.compose(cam)
	var out []item
	for _, i := range c.frame.order {
		out = append(out, c.frame.items[i])
	}
	return c, out
}

func TestComposer_FromAboveDrawsByTierAloneKeepingTheOrderGiven(t *testing.T) {
	a, b := sheet{"a"}, sheet{"b"}
	_, got := drawn(topDown(),
		items(func(f *Frame) {
			f.Sprite(Overlays, 1, a, 0, unit, Even(1))
			f.Sprite(Ground, 9, a, 0, unit, Even(1))
		}),
		items(func(f *Frame) {
			f.Sprite(Ground, 2, b, 0, unit, Even(1))
			f.Sprite(Marks, 0, b, 0, unit, Even(1))
			f.Sprite(Objects, 5, b, 0, unit, Even(1))
		}))
	want := []struct {
		tier  Tier
		depth float32
	}{{Ground, 9}, {Ground, 2}, {Objects, 5}, {Overlays, 1}, {Marks, 0}}
	for i, w := range want {
		if got[i].tier != w.tier || got[i].depth != w.depth {
			t.Fatalf("drawn %v, want tiers in order and ties as given: %v", got, want)
		}
	}
}

func TestComposer_ThroughAProjectionThatSortsDrawsBackToFrontWithMarksOnTop(t *testing.T) {
	s := sheet{}
	_, got := drawn(sorted(), items(func(f *Frame) {
		f.Sprite(Marks, 0, s, 0, unit, Even(1))      // always on top, whatever its depth
		f.Sprite(Overlays, 3, s, 0, unit, Even(1))   // a route on the tile at 3
		f.Sprite(Objects, 3, s, 0, unit, Even(1))    // a unit on it
		f.Sprite(Ground, 3, s, 0, unit, Even(1))     // the tile
		f.Sprite(Ground, 7, s, 0, unit, Even(1))     // a mountain in front
		f.Sprite(Tier(250), 1, s, 0, unit, Even(1))  // a game's own tier, far back
		f.Sprite(Marks+10, -5, s, 0, unit, Even(1))  // a label above the marks
		f.Sprite(Overlays, 3, s, 0, unit, Even(0.5)) // a second route piece, after the first
	}))
	want := []struct {
		tier  Tier
		depth float32
	}{{250, 1}, {Ground, 3}, {Objects, 3}, {Overlays, 3}, {Overlays, 3}, {Ground, 7}, {Marks, 0}, {Marks + 10, -5}}
	for i, w := range want {
		if got[i].tier != w.tier || got[i].depth != w.depth {
			t.Fatalf("drawn %+v, want %v", got, want)
		}
	}
}

// calls counts the draw calls of a render and the sheets they sampled.
func calls(c *Composer) []string {
	var out []string
	c.draw = func(_ *ebiten.Image, _ []ebiten.Vertex, _ []uint16, _ *ebiten.Image) { out = append(out, "call") }
	c.render(nil)
	return out
}

func TestComposer_DrawsARunSharingASheetInOneCallColoursIncluded(t *testing.T) {
	a, b := &sheet{"a"}, &sheet{"b"}
	c, _ := drawn(sorted(), items(func(f *Frame) {
		f.Sprite(Ground, 1, a, 0, unit, Even(1))
		f.Line(Ground, 1.5, 0, 0, 10, 0, 1, white)
		f.Soft(Overlays, 2, unit, black, Fade{Left: 2})
		f.Sprite(Objects, 3, a, 0, unit, Even(1))
	}))
	if n := len(calls(c)); n != 1 {
		t.Errorf("%d calls for one sheet with lines and a shadow among its sprites, want 1", n)
	}

	c, _ = drawn(sorted(), items(func(f *Frame) {
		f.Sprite(Ground, 1, a, 0, unit, Even(1))
		f.Sprite(Ground, 2, b, 0, unit, Even(1))
		f.Line(Ground, 3, 0, 0, 10, 0, 1, white) // joins b's run
		f.Sprite(Ground, 4, a, 0, unit, Even(1))
	}))
	if n := len(calls(c)); n != 3 {
		t.Errorf("%d calls for sheets a, b, a, want 3", n)
	}
}

func TestComposer_AColourJoiningARunSamplesItsSheetsWhite(t *testing.T) {
	a := &sheet{"a"}
	c, _ := drawn(topDown(), items(func(f *Frame) {
		f.Sprite(Ground, 0, a, 0, unit, Even(1))
		f.Line(Ground, 0, 0, 0, 10, 0, 1, white)
	}))
	var verts []ebiten.Vertex
	c.draw = func(_ *ebiten.Image, v []ebiten.Vertex, _ []uint16, _ *ebiten.Image) { verts = append(verts, v...) }
	c.render(nil)
	if len(verts) != 8 || verts[4].SrcX != 40 || verts[7].SrcY != 40 {
		t.Errorf("the line samples (%v, %v), want the sheet's white at (40, 40)", verts[4].SrcX, verts[4].SrcY)
	}
}

func TestComposer_SplitsACallBeforeItsIndicesOverflow(t *testing.T) {
	a := &sheet{"a"}
	quads := chunkVertices/4 + 5
	c, _ := drawn(topDown(), items(func(f *Frame) {
		for range quads {
			f.Sprite(Ground, 0, a, 0, unit, Even(1))
		}
	}))
	var sizes []int
	c.draw = func(_ *ebiten.Image, v []ebiten.Vertex, idx []uint16, _ *ebiten.Image) {
		sizes = append(sizes, len(v))
		for _, i := range idx {
			if int(i) >= len(v) {
				t.Fatalf("index %d past the %d vertices of its call", i, len(v))
			}
		}
	}
	c.render(nil)
	if len(sizes) != 2 || sizes[0] != chunkVertices || sizes[1] != 20 {
		t.Errorf("calls of %v vertices, want %d then 20", sizes, chunkVertices)
	}
}

func TestComposer_RefusesALayerThatIsNoSource(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "no Source") {
			t.Errorf("NewComposer took a layer that is no Source: %v", r)
		}
	}()
	NewComposer(screenOnly{})
}

type screenOnly struct{}

func (screenOnly) Init(*goke.SysInit) {}
func (screenOnly) Draw(*ebiten.Image) {}

func TestComposer_AWarmFrameAllocatesNothing(t *testing.T) {
	a := &sheet{"a"}
	c := NewComposer(items(func(f *Frame) {
		for i := range 200 {
			f.Sprite(Ground, float32(i%7), a, 0, unit, Even(1))
			f.Line(Overlays, float32(i%5), 0, 0, 5, 5, 1, white)
			f.Soft(Overlays, 1, unit, black, Fade{Top: 3})
			f.Tile(Ground, float32(i%7), a, 0, unit, Even(1))
			f.Glint(0, 0, 32, 32, 1, [4]float32{1, 1, 1, 1}, Shore{})
		}
		f.Daylight(Daylight{Dir: [3]float32{0, 0, 1}, Strength: 0.7, Sun: Light{1, 1, 1}})
		f.Fan(Overlays, 2, [][2]float32{{0, 0}, {5, 0}, {5, 5}, {0, 5}}, white)
	}))
	c.draw = func(*ebiten.Image, []ebiten.Vertex, []uint16, *ebiten.Image) {}
	cam := sorted()
	c.compose(cam)
	c.render(nil)
	if n := testing.AllocsPerRun(20, func() { c.compose(cam); c.render(nil) }); n > 0 {
		t.Errorf("a warm frame allocates %v times, want none", n)
	}
}

func TestFrame_SpriteRectSplitsAtAWrapSeamWithoutStretching(t *testing.T) {
	cam := icamera.NewFromSpace(1024, 1024, aabbworld.Torus)
	cam.Translate(1000, 0)
	var f Frame
	f.Reset(cam)
	f.SpriteRect(Objects, 0, sheet{}, 0, 998, 0, 1010, 10, Even(1))
	if f.Len() != 2 {
		t.Fatalf("%d quads across the seam, want 2", f.Len())
	}
	w := (f.verts[1].DstX - f.verts[0].DstX) + (f.verts[5].DstX - f.verts[4].DstX)
	if w != 12 {
		t.Errorf("the two pieces are %v wide together, want 12", w)
	}
}

func TestFrame_ALineIsAQuadOfItsWidthFadingAtItsSides(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	f.Line(Marks, 0, 10, 20, 30, 20, 2, color.RGBA{R: 20, A: 120})
	if f.Len() != 1 {
		t.Fatalf("%d items, want 1", f.Len())
	}
	v := f.verts
	if across := v[2].DstY - v[0].DstY; math.Abs(float64(across)) != 3 {
		t.Errorf("the line spans %v across, want its width 2 and a pixel of fade", across)
	}
	if v[0].Custom0 != 1 || v[0].Custom1 != 4 || v[2].Custom0 != 4 || v[2].Custom1 != 1 || v[0].Custom2 != 0 {
		t.Errorf("fades %v %v / %v %v, want 1 plus 0 and 3 to either side, no fade along", v[0].Custom0, v[0].Custom1, v[2].Custom0, v[2].Custom1)
	}
	if math.Abs(float64(v[0].ColorR)-20.0/255) > 1e-6 || math.Abs(float64(v[0].ColorA)-120.0/255) > 1e-6 {
		t.Errorf("colour %v alpha %v, want the premultiplied red 20/255 at alpha 120/255", v[0].ColorR, v[0].ColorA)
	}
	f.Line(Marks, 0, 5, 5, 5, 5, 1, white)
	if f.Len() != 1 {
		t.Error("a line of no length made an item")
	}
}

func TestFrame_SoftFadesOnlyTheSidesAsked(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	f.Soft(Overlays, 0, Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}, black, Fade{Left: 5, Bottom: 4})
	v := f.verts
	if v[0].Custom0 != 1 || v[1].Custom0 != 3 || v[3].Custom0 != 3 {
		t.Errorf("left fade %v %v %v, want 1 on the left, 1 + 10/5 on the right", v[0].Custom0, v[1].Custom0, v[3].Custom0)
	}
	if v[2].Custom3 != 1 || v[0].Custom3 != 6 {
		t.Errorf("bottom fade %v %v, want 1 at the bottom, 1 + 20/4 at the top", v[2].Custom3, v[0].Custom3)
	}
	if v[0].Custom1 != 0 || v[0].Custom2 != 0 {
		t.Errorf("right and top fade %v %v, want none", v[0].Custom1, v[0].Custom2)
	}
}

func TestFrame_AFanIsTrianglesRoundItsFirstPoint(t *testing.T) {
	a := &sheet{}
	c, _ := drawn(topDown(), items(func(f *Frame) {
		f.Sprite(Ground, 0, a, 0, unit, Even(1))
		f.Fan(Overlays, 0, [][2]float32{{0, 0}, {5, 0}, {5, 5}, {0, 5}, {-5, 5}}, white)
	}))
	var idx []uint16
	c.draw = func(_ *ebiten.Image, _ []ebiten.Vertex, i []uint16, _ *ebiten.Image) { idx = append(idx, i...) }
	c.render(nil)
	fanIdx := idx[6:]
	if len(fanIdx) != 9 || fanIdx[0] != 4 || fanIdx[1] != 5 || fanIdx[8] != 8 {
		t.Errorf("fan indices %v, want three triangles round vertex 4", fanIdx)
	}
}

func TestComposer_ItsShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader(composeKage); err != nil {
		t.Fatalf("the composer's shader: %v", err)
	}
}

func TestFrame_ATileIsOutlinedAlongItsOwnEdges(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	f.Tile(Ground, 0, sheet{}, 0, Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}, Even(1))
	v := f.verts
	// the top-left corner is on the left and top edges, 10 from the right, 20 from the bottom
	if v[0].Custom0 != -1 || v[0].Custom2 != -1 || v[0].Custom1 != -11 || v[0].Custom3 != -21 {
		t.Errorf("top-left outline values %v %v %v %v, want -1 -11 -1 -21", v[0].Custom0, v[0].Custom1, v[0].Custom2, v[0].Custom3)
	}
	if v[3].Custom1 != -1 || v[3].Custom3 != -1 {
		t.Errorf("bottom-right on its right and bottom edges gives %v %v, want -1", v[3].Custom1, v[3].Custom3)
	}
}

func TestFrame_ATileSplitAtAWrapSeamIsOutlinedOnlyAlongItsOwnEdges(t *testing.T) {
	cam := icamera.NewFromSpace(1024, 1024, aabbworld.Torus)
	cam.Translate(1000, 0)
	var f Frame
	f.Reset(cam)
	f.TileRect(Ground, 0, sheet{}, 0, 998, 0, 1010, 10, Even(1)) // 2 before the seam, 10 after
	if f.Len() != 2 {
		t.Fatalf("%d pieces, want 2", f.Len())
	}
	v := f.verts
	// the first piece's right side is the seam: 2 from the tile's left edge, 10 from its right
	if v[1].Custom0 != -3 || v[1].Custom1 != -11 {
		t.Errorf("at the seam the first piece gives %v %v, want -3 and -11: no outline along the seam", v[1].Custom0, v[1].Custom1)
	}
}

func TestFrame_AGlintIsAQuadOverTheSpriteMarkedWithTheWorldTheShineTheSunAndTheShore(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}
	f.Tile(Ground, 3, sheet{}, 0, dst, Even(0.5))
	shore := Shore{{X: 1, Dist: 8, Near: 0.75}, {X: 1, Dist: 40}, {X: 1, Dist: 8, Near: 0.75}, {X: 1, Dist: 40}}
	f.Glint(100, 200, 132, 232, 0.9, [4]float32{1, 0.5, 0, 0.25}, shore)
	if f.Len() != 2 || len(f.items) != 1 {
		t.Fatalf("%d pieces in %d items, want the tile and its glint in one", f.Len(), len(f.items))
	}
	v := f.verts[4:]
	for k, want := range [4][4]float32{{0.9, 100, 200, 3}, {0.9, 132, 200, 2.5}, {0.9, 100, 232, 2}, {0.9, 132, 232, 2.25}} {
		if got := [4]float32{v[k].ColorR, v[k].ColorG, v[k].ColorB, v[k].ColorA}; got != want {
			t.Errorf("glint corner %d is %v, want its shine, where it lies and 2 plus the sun reaching it: %v", k, got, want)
		}
		if v[k].DstX != dst[k][0] || v[k].DstY != dst[k][1] || v[k].SrcX != 40 || v[k].SrcY != 40 {
			t.Errorf("glint corner %d lies at %v,%v sampling %v,%v; want the tile's corner and the white texel",
				k, v[k].DstX, v[k].DstY, v[k].SrcX, v[k].SrcY)
		}
	}
	if v[0].Custom0 != 1 || v[0].Custom2 != 8 || v[0].Custom3 != 0.75 || v[1].Custom2 != 40 {
		t.Errorf("the glint carries the shore %v %v %v / %v, want the way, the distance and how near", v[0].Custom0, v[0].Custom2, v[0].Custom3, v[1].Custom2)
	}
	if f.verts[0].Custom0 != -1 {
		t.Errorf("the tile lost its outline: %v", f.verts[0].Custom0)
	}
}

func TestFrame_AGlintFollowsEachPieceOfARectSplitAtASeam(t *testing.T) {
	cam := icamera.NewFromSpace(1024, 1024, aabbworld.Torus)
	cam.Translate(1000, 0)
	var f Frame
	f.Reset(cam)
	f.TileRect(Ground, 0, sheet{}, 0, 992, 0, 1008, 10, Even(1)) // 8 before the seam, 8 after
	f.Glint(992, 0, 1008, 10, 1, [4]float32{0, 1, 0, 1}, Shore{})
	v := f.verts
	if len(v) != 16 {
		t.Fatalf("%d vertices, want the two pieces' 8 and their glints' 8", len(v))
	}
	g := v[8:]
	if g[1].ColorG != 1000 || g[1].ColorA != 2.5 || g[4].ColorG != 1000 || g[5].ColorG != 1008 || g[4].DstX != v[4].DstX {
		t.Errorf("at the seam the glints lie at x %v and %v with the sun at %v, want 1000 both sides and halfway sun 2.5",
			g[1].ColorG, g[4].ColorG, g[1].ColorA)
	}
}

func TestFrame_AStreamIsAQuadOverTheSpriteMarkedWithTheWorldTheShineTheSunAndTheFlow(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}
	f.Sprite(Ground, 3, sheet{}, 0, dst, Even(0.5))
	flow := Flow{{4, 0}, {8, 0}, {4, -2}, {8, -2}}
	f.Stream(World{{100, 200}, {132, 200}, {100, 232}, {132, 232}}, 0.9, [4]float32{1, 0.5, 0, 0.25}, flow)
	if f.Len() != 2 || len(f.items) != 1 {
		t.Fatalf("%d pieces in %d items, want the sprite and its stream in one", f.Len(), len(f.items))
	}
	v := f.verts[4:]
	for k, want := range [4][4]float32{{0.9, 100, 200, 6}, {0.9, 132, 200, 5.5}, {0.9, 100, 232, 5}, {0.9, 132, 232, 5.25}} {
		if got := [4]float32{v[k].ColorR, v[k].ColorG, v[k].ColorB, v[k].ColorA}; got != want {
			t.Errorf("stream corner %d is %v, want its shine, where it lies and 5 plus the sun reaching it: %v", k, got, want)
		}
		if got := [2]float32{v[k].Custom0, v[k].Custom1}; got != flow[k] {
			t.Errorf("stream corner %d runs at %v, want %v", k, got, flow[k])
		}
	}
}

// A stream over a slanted band carries each corner's place in the world, and a cloud's shadow laid
// over it too.
func TestFrame_AStreamAndItsShadowFollowTheCornersOfASlantedBand(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	f.Weather(Weather{Clouds: 0.5})
	f.Sprite(Ground, 0, sheet{}, 0, Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}, Even(1))
	band := World{{10, 0}, {14, 4}, {0, 10}, {4, 14}}
	f.OvercastAt(band)
	f.Stream(band, 1, [4]float32{1, 1, 1, 1}, Flow{})
	v := f.verts
	if len(v) != 12 {
		t.Fatalf("%d vertices, want the sprite's, its shadow's and its stream's", len(v))
	}
	for k, want := range band {
		for _, o := range [][]ebiten.Vertex{v[4:8], v[8:12]} {
			if got := [2]float32{o[k].ColorG, o[k].ColorB}; got != want {
				t.Errorf("corner %d lies at %v, want %v", k, got, want)
			}
		}
	}
}

// A blended sprite carries its corners' weights and how soft it is, and a cloud's shadow over it
// carries them too, so what lies under it is not shaded twice.
func TestFrame_ABlendedSpriteAndItsShadowCarryItsWeights(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	f.Weather(Weather{Clouds: 0.5})
	dst := Corners{{0, 0}, {8, 0}, {0, 10}, {8, 10}}
	weight := [4]float32{0, 0.5, 0.25, 1}
	f.SpriteBlend(Ground, 0, sheet{}, 0, dst, Even(1), weight, 0.2)
	f.OvercastAt(World{{0, 0}, {8, 0}, {0, 10}, {8, 10}})
	v := f.verts
	if len(v) != 8 {
		t.Fatalf("%d vertices, want the sprite's and its shadow's", len(v))
	}
	for k, w := range weight {
		if v[k].ColorA != w || math.Abs(float64(v[k].Custom3-10.2)) > 1e-6 || v[k].ColorR != 1 {
			t.Errorf("sprite corner %d: weight %v, mark %v, red %v; want %v, 10.2 and its light", k, v[k].ColorA, v[k].Custom3, v[k].ColorR, w)
		}
		if s := v[4+k]; s.ColorR != w || s.Custom3 != v[k].Custom3 || s.ColorA != 4 {
			t.Errorf("shadow corner %d: weight %v, mark %v, alpha %v; want the sprite's", k, s.ColorR, s.Custom3, s.ColorA)
		}
	}
}

// Running water over a blended sprite carries the sprite's weights and softness, so it shows only
// where the sprite does.
func TestFrame_AStreamOverABlendedSpriteCarriesItsWeights(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	weight := [4]float32{1, 0.5, 1, 0}
	f.SpriteBlend(Ground, 0, sheet{}, 0, Corners{{0, 0}, {8, 0}, {0, 10}, {8, 10}}, Even(1), weight, 0.5)
	f.Stream(World{{0, 0}, {8, 0}, {0, 10}, {8, 10}}, 1, [4]float32{1, 1, 1, 1}, Flow{{3, 0}, {3, 0}, {3, 0}, {3, 0}})
	v := f.verts[4:]
	for k, w := range weight {
		if v[k].Custom0 != 3 || v[k].Custom2 != w || v[k].Custom3 != 10.5 {
			t.Errorf("corner %d runs at %v with weight %v, mark %v; want 3, %v and 10.5", k, v[k].Custom0, v[k].Custom2, v[k].Custom3, w)
		}
	}
}

func TestFrame_AStreamFollowsEachPieceOfARectSplitAtASeam(t *testing.T) {
	cam := icamera.NewFromSpace(1024, 1024, aabbworld.Torus)
	cam.Translate(1000, 0)
	var f Frame
	f.Reset(cam)
	f.SpriteRect(Ground, 0, sheet{}, 0, 992, 0, 1008, 10, Even(1)) // 8 before the seam, 8 after
	f.Stream(World{{992, 0}, {1008, 0}, {992, 10}, {1008, 10}}, 1, [4]float32{1, 1, 1, 1}, Flow{{0, 0}, {16, 0}, {0, 0}, {16, 0}})
	v := f.verts
	if len(v) != 16 {
		t.Fatalf("%d vertices, want the two pieces' 8 and their streams' 8", len(v))
	}
	g := v[8:]
	if g[1].Custom0 != 8 || g[4].Custom0 != 8 || g[5].Custom0 != 16 {
		t.Errorf("at the seam the streams run at %v and %v, want halfway 8 both sides", g[1].Custom0, g[4].Custom0)
	}
}

func TestComposer_HandsTheShaderTheFramesDaylightAndTheEye(t *testing.T) {
	c := NewComposer(items(func(f *Frame) {
		f.Daylight(Daylight{Dir: [3]float32{0.6, 0, 0.8}, Strength: 0.7, Sun: Light{1, 0.8, 0.6}, Sky: Light{0.5, 0.7, 1}, Ambient: Light{0.2, 0.25, 0.3}})
	}))
	c.draw = func(*ebiten.Image, []ebiten.Vertex, []uint16, *ebiten.Image) {}
	c.compose(topDown())
	c.render(nil)
	if c.sun[0] != 0.6 || c.sun[2] != 0.8 || c.glint[1] != 0.7 || c.toward[2] != 1 {
		t.Errorf("the shader is handed sun %v, strength %v, eye %v; want the frame's sun and the eye above", c.sun, c.glint[1], c.toward)
	}
	if c.sunColor[2] != 0.6 || c.skyColor[1] != 0.7 || c.ambience[0] != 0.2 {
		t.Errorf("the shader is handed colours sun %v, sky %v, ambience %v; want the frame's", c.sunColor, c.skyColor, c.ambience)
	}
	c.frame.Reset(topDown())
	c.render(nil)
	if c.glint[1] != 0 {
		t.Errorf("a frame no one lit hands the shader a sun of strength %v, want 0: nothing glints", c.glint[1])
	}
}

func TestSway_LeansWithTheWindTheHarderTheFurtherAndNotAtAllInTheCalm(t *testing.T) {
	if x, y := Sway(1, [2]float32{}, 5, 5, 1); x != 0 || y != 0 {
		t.Errorf("in the calm a tree leans %v, %v, want not at all", x, y)
	}
	if x, y := Sway(1, [2]float32{30, 0}, 5, 5, 0); x != 0 || y != 0 {
		t.Errorf("what does not sway leans %v, %v", x, y)
	}
	breeze, _ := Sway(1, [2]float32{10, 0}, 5, 5, 1)
	gale, across := Sway(1, [2]float32{50, 0}, 5, 5, 1)
	if breeze <= 0 || gale <= breeze || across != 0 {
		t.Errorf("an east wind leans a tree %v in a breeze, %v across and %v in a gale; want east, further in the gale", breeze, across, gale)
	}
	a, _ := Sway(0, [2]float32{30, 0}, 0, 0, 1)
	b, _ := Sway(0.7, [2]float32{30, 0}, 0, 0, 1)
	if a == b {
		t.Error("a tree in the wind stands still: want it rocking")
	}
}

func TestFrame_OvercastMarksTheGroundWithTheWorldOnlyUnderClouds(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}
	f.Tile(Ground, 3, sheet{}, 0, dst, Even(1))
	f.Overcast(100, 200, 132, 232)
	if f.Len() != 1 {
		t.Fatalf("under a clear sky the ground got %d pieces, want the tile alone", f.Len())
	}
	f.Weather(Weather{Clouds: 0.5})
	f.Overcast(100, 200, 132, 232)
	if f.Len() != 2 {
		t.Fatalf("under clouds %d pieces, want the tile and the clouds' shadow over it", f.Len())
	}
	v := f.verts[4:]
	for k, want := range [4][3]float32{{100, 200, 4}, {132, 200, 4}, {100, 232, 4}, {132, 232, 4}} {
		if got := [3]float32{v[k].ColorG, v[k].ColorB, v[k].ColorA}; got != want {
			t.Errorf("overcast corner %d is %v, want where it lies and the mark 4: %v", k, got, want)
		}
	}
}

func TestComposer_HandsTheShaderTheWeatherAndTheFrameItsTime(t *testing.T) {
	var at float32
	c := NewComposer(items(func(f *Frame) {
		f.Weather(Weather{Wind: [2]float32{3, 4}, Drift: [2]float32{10, 20}, Clouds: 0.6})
		at = f.Time()
	}))
	c.draw = func(*ebiten.Image, []ebiten.Vertex, []uint16, *ebiten.Image) {}
	c.compose(topDown())
	c.render(nil)
	if c.wind[1] != 4 || c.drift[0] != 10 || c.weather[0] != 0.6 {
		t.Errorf("the shader is handed wind %v, drift %v, weather %v; want the frame's", c.wind, c.drift, c.weather)
	}
	if at != c.frame.time || c.glint[0] != at {
		t.Errorf("the sources saw time %v and the shader %v, want the one clock", at, c.glint[0])
	}
}

func TestOvercast_GreysTheSkyTheMoreItIsCovered(t *testing.T) {
	blue := Light{0.5, 0.72, 0.98}
	if Overcast(blue, 0) != blue {
		t.Errorf("a clear sky is %v, want it as it is", Overcast(blue, 0))
	}
	half, full := Overcast(blue, 0.5), Overcast(blue, 1)
	if !(full[2]-full[0] < half[2]-half[0] && half[2]-half[0] < blue[2]-blue[0]) {
		t.Errorf("the sky goes %v, %v, %v as clouds cover it; want it greyer each time", blue, half, full)
	}
}
