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
			f.Overlay(&Overlay{Material: tint, World: Box(0, 0, 32, 32), Red: [4]float32{1, 1, 1, 1}})
		}
		f.Uniform("Sun", 0, 0, 1)
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

// tint is a material registered for these tests: it paints what it reads.
var tint = RegisterMaterials([]byte(`// TestTint paints red, the fraction and the first custom.
func TestTint(p vec2, red float, fraction float, custom vec4) vec4 {
	return vec4(red, fraction, custom.x, 1)
}
`), "TestTint")[0]

func TestComposer_ItsShaderCompilesWithTheMaterialsRegistered(t *testing.T) {
	if err := Compile(); err != nil {
		t.Fatal(err)
	}
	if src := string(ShaderSource()); !strings.Contains(src, "return TestTint(p, red, fraction, custom)") {
		t.Errorf("the shader hands no overlay to the material registered:\n%s", src)
	}
}

// An overlay is a quad over the sprite marked with its material and what the material reads:
// where each corner lies in the world, red, the fraction and the customs.
func TestFrame_AnOverlayIsAQuadOverTheSpriteMarkedWithItsMaterialAndWhatItReads(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}
	f.Tile(Ground, 3, sheet{}, 0, dst, Even(0.5))
	f.Overlay(&Overlay{Material: 3, World: Box(100, 200, 132, 232), Red: [4]float32{0.9, 0.9, 0.9, 0.9},
		Fraction: [4]float32{1, 0.5, 0, 0.25}, Custom: [4][4]float32{{1, 0, 8, 0.75}, {1, 0, 40, 0}, {1, 0, 8, 0.75}, {1, 0, 40, 0}}})
	if f.Len() != 2 || len(f.items) != 1 {
		t.Fatalf("%d pieces in %d items, want the tile and its overlay in one", f.Len(), len(f.items))
	}
	v := f.verts[4:]
	for k, want := range [4][4]float32{{0.9, 100, 200, 9}, {0.9, 132, 200, 8.5}, {0.9, 100, 232, 8}, {0.9, 132, 232, 8.25}} {
		if got := [4]float32{v[k].ColorR, v[k].ColorG, v[k].ColorB, v[k].ColorA}; got != want {
			t.Errorf("overlay corner %d is %v, want red, where it lies and 2 + 2·3 + its fraction: %v", k, got, want)
		}
		if v[k].DstX != dst[k][0] || v[k].DstY != dst[k][1] || v[k].SrcX != 40 || v[k].SrcY != 40 {
			t.Errorf("overlay corner %d lies at %v,%v sampling %v,%v; want the tile's corner and the white texel",
				k, v[k].DstX, v[k].DstY, v[k].SrcX, v[k].SrcY)
		}
	}
	if v[0].Custom0 != 1 || v[0].Custom2 != 8 || v[0].Custom3 != 0.75 || v[1].Custom2 != 40 {
		t.Errorf("the overlay carries customs %v %v %v / %v, want its corners'", v[0].Custom0, v[0].Custom2, v[0].Custom3, v[1].Custom2)
	}
	if f.verts[0].Custom0 != -1 {
		t.Errorf("the tile lost its outline: %v", f.verts[0].Custom0)
	}
}

// Over a rect split at a wrap seam an overlay follows each piece, what it reads blended to where
// the seam cuts.
func TestFrame_AnOverlayFollowsEachPieceOfARectSplitAtASeam(t *testing.T) {
	cam := icamera.NewFromSpace(1024, 1024, aabbworld.Torus)
	cam.Translate(1000, 0)
	var f Frame
	f.Reset(cam)
	f.TileRect(Ground, 0, sheet{}, 0, 992, 0, 1008, 10, Even(1)) // 8 before the seam, 8 after
	f.Overlay(&Overlay{World: Box(992, 0, 1008, 10), Fraction: [4]float32{0, 1, 0, 1}, Custom: [4][4]float32{{0}, {16}, {0}, {16}}})
	v := f.verts
	if len(v) != 16 {
		t.Fatalf("%d vertices, want the two pieces' 8 and their overlays' 8", len(v))
	}
	g := v[8:]
	if g[1].ColorG != 1000 || g[1].ColorA != 2.5 || g[4].ColorG != 1000 || g[5].ColorG != 1008 || g[4].DstX != v[4].DstX || g[1].Custom0 != 8 {
		t.Errorf("at the seam the overlays lie at x %v and %v, fraction %v, custom %v; want 1000 both sides, halfway 2.5 and 8",
			g[1].ColorG, g[4].ColorG, g[1].ColorA, g[1].Custom0)
	}
}

// A top whose corner stands apart folds along the diagonal whose corners stand nearer in height,
// and what is laid over it folds with it; a level top keeps the usual diagonal.
func TestFrame_ATopFoldsAlongTheDiagonalOfTheNearerHeights(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {10, 0}, {0, 10}, {10, 10}}
	f.Sprite(Ground, 0, sheet{}, 0, dst, Even(1))
	f.Fold([4]float32{0, 0, 0, 0})
	f.Sprite(Ground, 0, sheet{}, 0, dst, Even(1))
	f.Fold([4]float32{5, 0, 5, 5}) // the top-right corner stands apart
	f.Overlay(&Overlay{World: Box(0, 0, 10, 10)})
	if len(f.items) != 2 || f.items[0].shape != quad || f.items[0].count != 4 || f.items[1].shape != folded || f.items[1].count != 8 {
		t.Fatalf("items %+v, want the level top as it is, the other and its overlay folded", f.items)
	}
	if got := appendQuad(nil, 0, folded); got[0] != 0 || got[2] != 3 || got[3] != 0 || got[5] != 3 {
		t.Errorf("a folded quad's triangles are %v, want both meeting along 0 to 3", got)
	}
}

// A tile's outline laid after what lies on it goes over it, along the tile's own edges.
func TestFrame_AnOutlineLiesOverWhatLiesOnTheTile(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}
	f.Sprite(Ground, 3, sheet{}, 0, dst, Even(1))
	top := f.Last()
	f.Sprite(Ground, 3, sheet{}, 0, Corners{{0, 0}, {5, 0}, {0, 20}, {5, 20}}, Even(1)) // lying on it
	f.OutlineOn(top)
	v := f.verts[8:]
	if len(v) != 4 || v[0].DstX != 0 || v[3].DstX != 10 || v[3].DstY != 20 {
		t.Fatalf("the outline lies at %v, want over the tile", v)
	}
	if v[0].Custom0 != -1 || v[1].Custom0 != -11 || v[3].Custom3 != -1 || v[0].Custom3 != -21 {
		t.Errorf("the outline's corners are %v %v %v, want the distances to the tile's edges", v[0].Custom0, v[1].Custom0, v[3].Custom3)
	}
}

// A glaze shows as much as its opacity at each corner: its light and its alpha scaled by it.
func TestFrame_AGlazeShowsAsMuchAsItsOpacity(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	f.Glaze(Ground, 0, sheet{}, 0, Corners{{0, 0}, {10, 0}, {0, 10}, {10, 10}}, Even(0.8), [4]float32{0, 0.5, 1, 2})
	for k, want := range [4]float32{0, 0.5, 1, 1} {
		if v := f.verts[k]; v.ColorA != want || math.Abs(float64(v.ColorR-0.8*want)) > 1e-6 {
			t.Errorf("corner %d glazed %v, alpha %v; want %v, %v", k, v.ColorR, v.ColorA, 0.8*want, want)
		}
	}
}

// A part of a sheet is drawn from the pixels asked for, not from a sprite.
func TestFrame_APartOfASheetSamplesThePixelsAskedFor(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	f.SpritePart(Ground, 0, sheet{}, [4]float32{16, 32, 24, 40}, Corners{{0, 0}, {10, 0}, {0, 10}, {10, 10}}, Even(1))
	v := f.verts
	if v[0].SrcX != 16 || v[0].SrcY != 32 || v[3].SrcX != 24 || v[3].SrcY != 40 {
		t.Errorf("the part samples %v,%v to %v,%v; want 16,32 to 24,40", v[0].SrcX, v[0].SrcY, v[3].SrcX, v[3].SrcY)
	}
}

// An overlay laid on a sprite drawn earlier lies over it, at its tier and depth, not over the last;
// over a rect split at a seam it follows each of its pieces still.
func TestFrame_AnOverlayOnAnEarlierSpriteLiesOverIt(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}
	f.Tile(Ground, 3, sheet{}, 0, dst, Even(1))
	top := f.Last()
	f.Sprite(Ground+5, 7, sheet{}, 0, Corners{{50, 50}, {60, 50}, {50, 60}, {60, 60}}, Even(1))
	f.OverlayOn(top, &Overlay{Material: 1, World: Box(0, 0, 10, 20)})
	last := f.items[len(f.items)-1]
	if last.tier != Ground || last.depth != 3 {
		t.Errorf("the overlay is at tier %v, depth %v; want the tile's %v, 3", last.tier, last.depth, Ground)
	}
	for k, v := range f.verts[8:] {
		if v.DstX != dst[k][0] || v.DstY != dst[k][1] {
			t.Errorf("overlay corner %d lies at %v,%v; want the tile's %v", k, v.DstX, v.DstY, dst[k])
		}
	}

	cam := icamera.NewFromSpace(1024, 1024, aabbworld.Torus)
	cam.Translate(1000, 0)
	f.Reset(cam)
	f.TileRect(Ground, 0, sheet{}, 0, 992, 0, 1008, 10, Even(1)) // 8 before the seam, 8 after
	split := f.Last()
	f.TileRect(Ground, 0, sheet{}, 0, 0, 100, 10, 110, Even(1))
	f.OverlayOn(split, &Overlay{World: Box(992, 0, 1008, 10)})
	if v := f.verts; len(v) != 20 || v[12].DstX != v[0].DstX || v[16].DstX != v[4].DstX {
		t.Fatalf("%d vertices, the overlays at x %v and %v; want 20, over the split rect's pieces at %v and %v",
			len(v), v[12].DstX, v[16].DstX, v[0].DstX, v[4].DstX)
	}
}

// An overlay Under a sprite takes how faint it is and how it fades; one Blended over a blended
// sprite takes its weight and mark; one over a slanted band lies where its corners do.
func TestFrame_AnOverlayTakesWhatItShouldOfTheSpriteUnderIt(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {8, 0}, {0, 10}, {8, 10}}
	band := World{{10, 0}, {14, 4}, {0, 10}, {4, 14}}
	weight := [4]float32{0, 0.5, 0.25, 1}
	f.SpriteBlend(Ground, 0, sheet{}, 0, dst, Even(1), weight, 0.2)
	f.Overlay(&Overlay{Material: 1, World: band, Under: true})
	f.Overlay(&Overlay{Material: 2, World: band, Custom: [4][4]float32{{3}, {3}, {3}, {3}}, Blended: true})
	v := f.verts
	for k, w := range weight {
		if s := v[4+k]; s.ColorR != w || s.Custom3 != v[k].Custom3 {
			t.Errorf("under corner %d: red %v, mark %v; want the sprite's weight %v and mark %v", k, s.ColorR, s.Custom3, w, v[k].Custom3)
		}
		if s := v[8+k]; s.Custom0 != 3 || s.Custom2 != w || s.Custom3 != v[k].Custom3 {
			t.Errorf("blended corner %d: customs %v %v %v; want its own 3, the sprite's weight %v and mark", k, s.Custom0, s.Custom2, s.Custom3, w)
		}
		if got := [2]float32{v[8+k].ColorG, v[8+k].ColorB}; got != band[k] {
			t.Errorf("corner %d lies at %v, want %v", k, got, band[k])
		}
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

// The shader is handed the composer's own uniforms — the way towards the eye, the clock, the
// world units a pixel spans — and every one the sources set; one set in an earlier frame and not
// this one is zeroed, and a warm frame allocates none of it.
func TestComposer_HandsTheShaderTheFramesUniformsAndZeroesTheStale(t *testing.T) {
	var at float32
	set := true
	c := NewComposer(items(func(f *Frame) {
		if set {
			f.Uniform("Sun", 0.6, 0, 0.8)
			f.Uniform("Cover", 0.6)
		}
		at = f.Time()
	}))
	c.draw = func(*ebiten.Image, []ebiten.Vertex, []uint16, *ebiten.Image) {}
	c.compose(topDown())
	c.render(nil)
	sun, cover := c.uniforms["Sun"], c.uniforms["Cover"]
	if len(sun) != 3 || sun[0] != 0.6 || sun[2] != 0.8 || len(cover) != 1 || cover[0] != 0.6 {
		t.Errorf("the shader is handed Sun %v and Cover %v, want the frame's (0.6, 0, 0.8) and 0.6", sun, cover)
	}
	if toward := c.uniforms["Toward"]; toward[2] != 1 {
		t.Errorf("the shader is handed Toward %v, want the eye above", toward)
	}
	if clock := c.uniforms["Clock"]; clock[0] != at || at != c.frame.time {
		t.Errorf("the sources saw time %v and the shader %v, want the one clock", at, clock[0])
	}
	if pixel := c.uniforms["Pixel"]; pixel[0] != 1 {
		t.Errorf("the shader is handed Pixel %v at zoom 1, want 1", pixel)
	}
	for name, u := range c.uniforms {
		if v, ok := c.opts.Uniforms[name].([]float32); !ok || &v[0] != &u[0] {
			t.Errorf("the draw options hand the shader another %s than the composer keeps", name)
		}
	}
	set = false
	c.compose(topDown())
	c.render(nil)
	if sun, cover := c.uniforms["Sun"], c.uniforms["Cover"]; sun[0] != 0 || sun[2] != 0 || cover[0] != 0 {
		t.Errorf("a frame that set nothing hands the shader Sun %v and Cover %v, want zero", sun, cover)
	}
	set = true
	cam := topDown()
	c.compose(cam)
	c.render(nil)
	if n := testing.AllocsPerRun(20, func() { c.compose(cam); c.render(nil) }); n > 0 {
		t.Errorf("a warm frame with uniforms allocates %v times, want none", n)
	}
}

// direct is a Direct source for tests: it hands the frame nothing and notes, when it draws, how
// many calls the composer had issued before it.
type direct struct {
	tier  Tier
	after *int
	drawn []int
}

func (*direct) Init(*goke.SysInit)                  {}
func (*direct) Compose(*Frame, camera.Camera)       {}
func (d *direct) Tier() Tier                        { return d.tier }
func (d *direct) Draw(*ebiten.Image, camera.Camera) { d.drawn = append(d.drawn, *d.after) }

// A Direct source draws after every piece the frame orders before its tier and before the rest,
// whatever order the sources came in; a nil layer is left out.
func TestComposer_ADirectSourceDrawsWhereItsTierComes(t *testing.T) {
	a := sheet{"a"}
	calls := 0
	ground := &direct{tier: Ground, after: &calls}
	marks := &direct{tier: Marks, after: &calls}
	c := NewComposer(marks, items(func(f *Frame) {
		f.Sprite(Backdrop, 0, a, 0, unit, Even(1)) // the sky, drawn first
		f.Sprite(Ground, 0, a, 0, unit, Even(1))   // a tile
		f.Sprite(Overlays, 0, a, 0, unit, Even(1)) // a route
	}), nil, ground)
	c.draw = func(*ebiten.Image, []ebiten.Vertex, []uint16, *ebiten.Image) { calls++ }
	c.compose(topDown())
	c.render(nil)
	if len(c.sources) != 3 || len(c.directs) != 2 || c.directs[0] != ground {
		t.Fatalf("%d sources, %d directs in order %v; want the nil layer left out and the directs by tier", len(c.sources), len(c.directs), c.directs)
	}
	if got := ground.drawn; len(got) != 1 || got[0] != 1 {
		t.Errorf("the ground's direct drew after %v calls, want once after the sky's one", got)
	}
	if got := marks.drawn; len(got) != 1 || got[0] != 2 || calls != 2 {
		t.Errorf("the marks' direct drew after %v calls of %d, want once after the tile's and the route's run", got, calls)
	}
}
