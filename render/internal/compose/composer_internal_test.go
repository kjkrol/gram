package compose

import (
	"image/color"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/gpu"
	"github.com/kjkrol/gram/render/internal/wgsl"
)

type sheet struct{ name string }

func (sheet) Atlas() *Image                            { return nil }
func (sheet) UV(SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 32, 32 }
func (sheet) White() (u, v float32)                    { return 40, 40 }

type items func(f *Frame)

func (items) Init(*goke.SysInit)                  {}
func (s items) Compose(f *Frame, _ camera.Camera) { s(f) }

var (
	white = color.RGBA{255, 255, 255, 255}
	black = color.RGBA{0, 0, 0, 255}
)

var unit = Corners{{0, 0}, {1, 0}, {0, 1}, {1, 1}}

func topDown() camera.Camera { return cameratest.NewFromSpace(1024, 1024, 0) }

func sorted() camera.Camera { return sortingCamera{cameratest.NewFromSpace(1024, 1024, 0)} }

type sortingCamera struct{ camera.Camera }

func (sortingCamera) Projection() camera.Projection { return sortingProjection{} }

type sortingProjection struct{ camera.TopDown }

func (sortingProjection) Sorts() bool { return true }

func drawn(cam camera.Camera, sources ...Source) (*Composer, []item) {
	c := NewComposer(sources...)
	c.compose(cam, wholePart)
	var out []item
	for _, i := range c.frame.order {
		out = append(out, c.frame.items[i])
	}
	return c, out
}

// a piece at height z from above
func h(z float32) At { return At{z: z, key: z} }

func TestComposer_DrawsTheWorldBackToFrontAndTheMarksLastAsTheyCame(t *testing.T) {
	a, b := sheet{"a"}, sheet{"b"}
	_, got := drawn(topDown(),
		items(func(f *Frame) {
			f.Sprite(h(1), a, 0, unit, even(1))
			f.Sprite(OnTop(0), a, 0, unit, even(0.5))
			f.Sprite(h(9), a, 0, unit, even(1))
		}),
		items(func(f *Frame) {
			f.Sprite(h(1), b, 0, unit, even(1)) // as high: after the first, as it came
			f.Sprite(OnTop(0), b, 0, unit, even(1))
			f.Sprite(h(5), b, 0, unit, even(1))
		}))
	want := []At{h(1), h(1), h(5), h(9), OnTop(0), OnTop(0)}
	for i, w := range want {
		if got[i].at != w {
			t.Fatalf("drawn %v, want the world lowest first, ties as they came, the marks last: %v",
				got, want)
		}
	}
	if got[0].atlas != (sheet{"a"}) || got[4].atlas != (sheet{"a"}) ||
		got[5].atlas != (sheet{"b"}) {
		t.Errorf("drawn %v, want the first source's before the second's at one height and "+
			"among the marks", got)
	}
}

func TestComposer_DrawsTheMarksByLevelAndAtOneLevelAsTheyCame(t *testing.T) {
	a, b := sheet{"a"}, sheet{"b"}
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		sources := []Source{
			items(func(f *Frame) {
				f.Sprite(OnTop(3), a, 0, unit, even(1))
				f.Sprite(OnTop(1), a, 0, unit, even(1))
			}),
			items(func(f *Frame) {
				f.Sprite(OnTop(2), b, 0, unit, even(1))
				f.Sprite(OnTop(1), b, 0, unit, even(1))
			}),
		}
		_, got := drawn(topDown(), sources[order[0]], sources[order[1]])
		levels := []At{OnTop(1), OnTop(1), OnTop(2), OnTop(3)}
		for i, w := range levels {
			if got[i].at != w {
				t.Fatalf("sources %v drawn %v, want the marks by level", order, got)
			}
		}
		if got[2].atlas != b || got[3].atlas != a {
			t.Errorf("sources %v drawn %v, want each mark at its level", order, got)
		}
	}
}

func TestFrame_AnEdgesEndsKeepTheirHeightsAndWithNoDepthNone(t *testing.T) {
	c, _ := drawn(topDown(), items(func(f *Frame) {
		f.Line(h(5), 0, 0, 10, 0, 1, white)
		f.Edge(h(5), 0, 10, 2, 10, 10, 8, 1, white)
	}))
	if c.frame.Len() != 2 {
		t.Fatalf("%d pieces, want the line and the edge apart", c.frame.Len())
	}
	var gotV []Vertex
	c.draw = func(_ *Image, verts []Vertex, _ []uint16, _ *Image, _ [4]float32) {
		gotV = append(gotV, verts...)
	}
	for _, heights := range []bool{true, false} {
		gotV = gotV[:0]
		c.seen = heights
		c.paint(nil, Target{})
		want := [8]float32{5, 5, 5, 5, 2, 8, 2, 8}
		if !heights {
			want = [8]float32{}
		}
		for k, w := range want {
			if gotV[k].DstZ != w {
				t.Errorf("heights %v: vertex %d at %v, want %v", heights, k, gotV[k].DstZ, w)
			}
		}
	}
}

func TestFrame_AtFromAboveIsTheHeightAndThroughAViewThatSortsTheCamerasDepth(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	if at := f.At(10, 20, 3); at.key != 3 || at.Z() != 3 {
		t.Errorf("from above (10, 20) at 3 lies at key %v, height %v; want 3, 3", at.key, at.Z())
	}
	if at := f.Lifted(10, 20, 3, 0.5); at.key != 3.5 || at.Z() != 3.5 {
		t.Errorf("lifted by a half it lies at %v, %v; want 3.5", at.key, at.Z())
	}
	f.Reset(sorted())
	if at := f.At(10, 20, 3); at.key != 20 || at.Z() != 3 {
		t.Errorf("through a view that sorts it lies at key %v, height %v; want the camera's "+
			"depth 20, 3", at.key, at.Z())
	}
	if !Furthest.Before(h(-1e9)) || !h(1e9).Before(Nearest) {
		t.Error("the sky is not behind everything, or the rain before it")
	}
}

func calls(c *Composer) []string {
	var out []string
	c.draw = func(_ *Image, _ []Vertex, _ []uint16, _ *Image, _ [4]float32) {
		out = append(out, "call")
	}
	c.render(nil, false, 0)
	return out
}

func TestComposer_DrawsARunSharingASheetInOneCallColoursIncluded(t *testing.T) {
	a, b := &sheet{"a"}, &sheet{"b"}
	c, _ := drawn(sorted(), items(func(f *Frame) {
		f.Sprite(h(1), a, 0, unit, even(1))
		f.Line(h(1.5), 0, 0, 10, 0, 1, white)
		f.Soft(h(2), unit, black, Fade{Left: 2})
		f.Sprite(h(3), a, 0, unit, even(1))
	}))
	if n := len(calls(c)); n != 1 {
		t.Errorf("%d calls for one sheet with lines and a shadow among its sprites, want 1", n)
	}

	c, _ = drawn(sorted(), items(func(f *Frame) {
		f.Sprite(h(1), a, 0, unit, even(1))
		f.Sprite(h(2), b, 0, unit, even(1))
		f.Line(h(3), 0, 0, 10, 0, 1, white) // joins b's run
		f.Sprite(h(4), a, 0, unit, even(1))
	}))
	if n := len(calls(c)); n != 3 {
		t.Errorf("%d calls for sheets a, b, a, want 3", n)
	}
}

func TestComposer_AColourJoiningARunSamplesItsSheetsWhite(t *testing.T) {
	a := &sheet{"a"}
	c, _ := drawn(topDown(), items(func(f *Frame) {
		f.Sprite(h(0), a, 0, unit, even(1))
		f.Line(h(0), 0, 0, 10, 0, 1, white)
	}))
	var verts []Vertex
	c.draw = func(_ *Image, v []Vertex, _ []uint16, _ *Image, _ [4]float32) {
		verts = append(verts, v...)
	}
	c.render(nil, false, 0)
	if len(verts) != 8 || verts[4].SrcX != 40 || verts[7].SrcY != 40 {
		t.Errorf("the line samples (%v, %v), want the sheet's white at (40, 40)", verts[4].SrcX, verts[4].SrcY)
	}
}

func TestComposer_SplitsACallBeforeItsIndicesOverflow(t *testing.T) {
	a := &sheet{"a"}
	quads := chunkVertices/4 + 5
	c, _ := drawn(topDown(), items(func(f *Frame) {
		for range quads {
			f.Sprite(h(0), a, 0, unit, even(1))
		}
	}))
	var sizes []int
	c.draw = func(_ *Image, v []Vertex, idx []uint16, _ *Image, _ [4]float32) {
		sizes = append(sizes, len(v))
		for _, i := range idx {
			if int(i) >= len(v) {
				t.Fatalf("index %d past the %d vertices of its call", i, len(v))
			}
		}
	}
	c.render(nil, false, 0)
	if len(sizes) != 2 || sizes[0] != chunkVertices || sizes[1] != 20 {
		t.Errorf("calls of %v vertices, want %d then 20", sizes, chunkVertices)
	}
}

type counted struct {
	items
	inits int
}

func (c *counted) Init(*goke.SysInit) { c.inits++ }

func TestComposer_InitsItsSourcesOnceHoweverManyScenesShowIt(t *testing.T) {
	src := &counted{}
	c := NewComposer(src)
	c.Init(nil)
	c.Init(nil)
	if src.inits != 1 {
		t.Errorf("the source was readied %d times, want once", src.inits)
	}
}

func TestComposer_AWarmFrameAllocatesNothing(t *testing.T) {
	a := &sheet{"a"}
	c := NewComposer(items(func(f *Frame) {
		for i := range 200 {
			f.Sprite(h(float32(i%7)), a, 0, unit, even(1))
			f.Line(h(float32(i%5)), 0, 0, 5, 5, 1, white)
			f.Soft(h(1), unit, black, Fade{Top: 3})
			f.Overlay(&Overlay{Material: tint, World: box(0, 0, 32, 32), Red: [4]float32{1, 1, 1, 1}})
		}
		f.Uniform("LightWay", 0, 0, 1)
		f.Image("TestProbe", nil)
		f.SpriteFan(h(2), a, 0, [][2]float32{{0, 0}, {5, 0}, {5, 5}, {0, 5}}, [4]float32{0, 0, 5, 5}, 1)
	}), overlyingSource{items(func(f *Frame) {
		for i := range 50 {
			f.Sprite(h(float32(2+i%3)), a, 0, unit, even(1))
			f.Line(OnTop(1), 0, 0, 5, 5, 1, white)
		}
	})})
	c.draw = func(*Image, []Vertex, []uint16, *Image, [4]float32) {}
	cam := sorted()
	c.compose(cam, wholePart)
	c.render(nil, false, 0)
	if n := testing.AllocsPerRun(20, func() {
		c.compose(cam, wholePart)
		c.render(nil, false, 0)
	}); n > 0 {
		t.Errorf("a warm frame allocates %v times, want none", n)
	}
	c.drawApart(cam)
	if n := testing.AllocsPerRun(20, func() { c.drawApart(cam) }); n > 0 {
		t.Errorf("a warm frame of the ground and what lies over it allocates %v times, want none",
			n)
	}
	outer, inner, screen := gfx.NewImage(32, 32), gfx.NewImage(24, 24), gfx.NewImage(64, 64)
	layered := func() {
		c.Lay(cam, true)
		c.DrawLayer(outer, cam, camera.Window{W: 64, H: 64}, camera.Window{})
		c.DrawLayer(inner, cam, camera.Window{X: 16, Y: 16, W: 32, H: 32}, camera.Window{})
		c.DrawOver(screen, cam)
	}
	layered()
	if n := testing.AllocsPerRun(20, layered); n > 0 {
		t.Errorf("a warm frame laid in two layers allocates %v times, want none", n)
	}
}

func TestFrame_SpriteRectSplitsAtAWrapSeamWithoutStretching(t *testing.T) {
	cam := cameratest.NewFromSpace(1024, 1024, aabbworld.Torus)
	cam.Translate(1000, 0)
	var f Frame
	f.Reset(cam)
	f.SpriteRect(h(0), sheet{}, 0, 998, 0, 1010, 10, even(1))
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
	f.Line(OnTop(0), 10, 20, 30, 20, 2, color.RGBA{R: 20, A: 120})
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
	// straight alpha in, premultiplied for the GPU
	red, alpha := float64(v[0].ColorR), float64(v[0].ColorA)
	if math.Abs(red-20.0/255*120/255) > 1e-6 || math.Abs(alpha-120.0/255) > 1e-6 {
		t.Errorf("colour %v alpha %v, want red 20/255 premultiplied by alpha 120/255", red, alpha)
	}
	f.Line(OnTop(0), 5, 5, 5, 5, 1, white)
	if f.Len() != 1 {
		t.Error("a line of no length made an item")
	}
}

func TestFrame_SoftFadesOnlyTheSidesAsked(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	f.Soft(h(0), Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}, black, Fade{Left: 5, Bottom: 4})
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
		f.Sprite(h(0), a, 0, unit, even(1))
		f.SpriteFan(h(0), a, 0, [][2]float32{{0, 0}, {5, 0}, {5, 5}, {0, 5}, {-5, 5}}, [4]float32{-5, 0, 5, 5}, 1)
	}))
	var idx []uint16
	c.draw = func(_ *Image, _ []Vertex, i []uint16, _ *Image, _ [4]float32) {
		idx = append(idx, i...)
	}
	c.render(nil, false, 0)
	fanIdx := idx[6:]
	if len(fanIdx) != 9 || fanIdx[0] != 4 || fanIdx[1] != 5 || fanIdx[8] != 8 {
		t.Errorf("fan indices %v, want three triangles round vertex 4", fanIdx)
	}
}

func TestFrame_ASpriteFanIsThePartOfTheSpriteUnderIt(t *testing.T) {
	var f Frame
	pts := [][2]float32{{150, 100}, {200, 150}, {150, 200}, {250, 150}}
	f.SpriteFan(h(0), &sheet{}, 0, pts, [4]float32{100, 100, 200, 200}, 0.5)
	var got []Vertex
	f.Each(func(_ At, v []Vertex) { got = append(got, v...) })
	if len(got) != 4 {
		t.Fatalf("%d vertices, want one a point", len(got))
	}
	// the sheet's sprite spans 0.5 to 31.5 once inset by half a texel
	for i, want := range [][2]float32{{16, 0.5}, {31.5, 16}, {16, 31.5}, {31.5, 16}} {
		if got[i].SrcX != want[0] || got[i].SrcY != want[1] {
			t.Errorf("point %d samples %v, %v; want %v", i, got[i].SrcX, got[i].SrcY, want)
		}
	}
	if c := got[0]; c.ColorR != 0.5 || c.ColorA != 0.5 {
		t.Errorf("colour %v %v, want half opaque, premultiplied", c.ColorR, c.ColorA)
	}
	if it := f.items[0]; it.shape != fan || it.atlas == nil {
		t.Errorf("the piece is %v on %v, want a fan on the sheet", it.shape, it.atlas)
	}
}

var tint = wgsl.RegisterMaterials([]byte(`// TestTint paints red, the fraction and the first custom.
fn TestTint(p: vec2<f32>, red: f32, fraction: f32, custom: vec4<f32>) -> vec4<f32> {
    return vec4<f32>(red, fraction, custom.x, 1.0);
}
`), nil, "TestTint")[0]

func TestComposer_ItsShaderCompilesWithTheMaterialsRegistered(t *testing.T) {
	gputest.Need(t)
	if err := Compile(); err != nil {
		t.Fatal(err)
	}
	if src := ShaderSource(); !strings.Contains(src, "return TestTint(p, red, fraction, custom);") {
		t.Errorf("the shader hands no overlay to the material registered:\n%s", src)
	}
}

func TestFrame_AnOverlayIsAQuadOverTheSpriteMarkedWithItsMaterialAndWhatItReads(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {10, 0}, {0, 20}, {10, 20}}
	f.Sprite(h(3), sheet{}, 0, dst, even(0.5))
	f.Overlay(&Overlay{Material: 3, World: box(100, 200, 132, 232), Red: [4]float32{0.9, 0.9, 0.9, 0.9},
		Fraction: [4]float32{1, 0.5, 0, 0.25}, Custom: [4][4]float32{{1, 0, 8, 0.75}, {1, 0, 40, 0}, {1, 0, 8, 0.75}, {1, 0, 40, 0}}})
	if f.Len() != 2 || len(f.items) != 1 {
		t.Fatalf("%d pieces in %d items, want the sprite and its overlay in one", f.Len(),
			len(f.items))
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
}

func TestFrame_AnOverlayFollowsEachPieceOfARectSplitAtASeam(t *testing.T) {
	cam := cameratest.NewFromSpace(1024, 1024, aabbworld.Torus)
	cam.Translate(1000, 0)
	var f Frame
	f.Reset(cam)
	f.SpriteRect(h(0), sheet{}, 0, 992, 0, 1008, 10, even(1)) // 8 before the seam, 8 after
	f.Overlay(&Overlay{World: box(992, 0, 1008, 10), Fraction: [4]float32{0, 1, 0, 1}, Custom: [4][4]float32{{0}, {16}, {0}, {16}}})
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

func TestFrame_AGlazeShowsAsMuchAsItsOpacity(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	square := Corners{{0, 0}, {10, 0}, {0, 10}, {10, 10}}
	f.Glaze(h(0), sheet{}, 0, square, even(0.8), [4]float32{0, 0.5, 1, 2})
	for k, want := range [4]float32{0, 0.5, 1, 1} {
		if v := f.verts[k]; v.ColorA != want || math.Abs(float64(v.ColorR-0.8*want)) > 1e-6 {
			t.Errorf("corner %d glazed %v, alpha %v; want %v, %v", k, v.ColorR, v.ColorA, 0.8*want, want)
		}
	}
}

func TestFrame_APartOfASheetSamplesThePixelsAskedFor(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	square := Corners{{0, 0}, {10, 0}, {0, 10}, {10, 10}}
	f.SpritePart(h(0), sheet{}, [4]float32{16, 32, 24, 40}, square, even(1))
	v := f.verts
	if v[0].SrcX != 16 || v[0].SrcY != 32 || v[3].SrcX != 24 || v[3].SrcY != 40 {
		t.Errorf("the part samples %v,%v to %v,%v; want 16,32 to 24,40", v[0].SrcX, v[0].SrcY, v[3].SrcX, v[3].SrcY)
	}
}

func TestFrame_AnOverlayTakesWhatItShouldOfTheSpriteUnderIt(t *testing.T) {
	var f Frame
	f.Reset(topDown())
	dst := Corners{{0, 0}, {8, 0}, {0, 10}, {8, 10}}
	band := World{{10, 0}, {14, 4}, {0, 10}, {4, 14}}
	weight := [4]float32{0, 0.5, 0.25, 1}
	f.SpriteBlend(h(0), sheet{}, 0, dst, even(1), weight, 0.2)
	f.Overlay(&Overlay{Material: 2, World: band, Custom: [4][4]float32{{3}, {3}, {3}, {3}}, Blended: true})
	v := f.verts
	for k, w := range weight {
		if s := v[4+k]; s.Custom0 != 3 || s.Custom2 != w || s.Custom3 != v[k].Custom3 {
			t.Errorf("blended corner %d: customs %v %v %v; want its own 3, the sprite's weight %v and mark", k, s.Custom0, s.Custom2, s.Custom3, w)
		}
		if got := [2]float32{v[4+k].ColorG, v[4+k].ColorB}; got != band[k] {
			t.Errorf("corner %d lies at %v, want %v", k, got, band[k])
		}
	}
}

func TestComposer_HandsTheShaderTheFramesUniformsAndZeroesTheStale(t *testing.T) {
	var at float32
	set := true
	c := NewComposer(items(func(f *Frame) {
		if set {
			f.Uniform("LightWay", 0.6, 0, 0.8)
			f.Uniform("Cover", 0.6)
		}
		at = f.Time()
	}))
	c.draw = func(*Image, []Vertex, []uint16, *Image, [4]float32) {}
	c.compose(topDown(), wholePart)
	c.render(nil, false, 0)
	sun, cover := c.uniforms["LightWay"], c.uniforms["Cover"]
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
		if v, ok := c.boxed[name].([]float32); !ok || &v[0] != &u[0] {
			t.Errorf("a Direct source is handed another %s than the composer keeps", name)
		}
	}
	set = false
	c.compose(topDown(), wholePart)
	c.render(nil, false, 0)
	if sun, cover := c.uniforms["LightWay"], c.uniforms["Cover"]; sun[0] != 0 || sun[2] != 0 || cover[0] != 0 {
		t.Errorf("a frame that set nothing hands the shader Sun %v and Cover %v, want zero", sun, cover)
	}
	set = true
	cam := topDown()
	c.compose(cam, wholePart)
	c.render(nil, false, 0)
	if n := testing.AllocsPerRun(20, func() {
		c.compose(cam, wholePart)
		c.render(nil, false, 0)
	}); n > 0 {
		t.Errorf("a warm frame with uniforms allocates %v times, want none", n)
	}
}

type direct struct {
	opaque bool
	at     *At
	ground bool
	after  *int
	drawn  []int
	passes []Pass
	sun    []float32
}

func (*direct) Init(*goke.SysInit) {}
func (d *direct) Compose(f *Frame, _ camera.Camera) {
	if d.opaque {
		f.DrawOpaque()
	}
	if d.at != nil {
		f.DrawAt(*d.at)
	}
	if d.ground {
		f.ReadGround()
	}
}
func (d *direct) Draw(t Target, _ camera.Camera, u Uniforms) {
	d.drawn = append(d.drawn, *d.after)
	d.passes = append(d.passes, t.Pass)
	d.sun = u.Get("Seen")
}

func TestComposer_ADirectDrawsWhereItAskedAndOnlyThere(t *testing.T) {
	a := sheet{"a"}
	calls := 0
	sky, rain := Furthest, Nearest
	ground := &direct{opaque: true, after: &calls}
	views := &direct{at: &[]At{h(0.25)}[0], after: &calls}
	figures := &direct{at: &[]At{h(2)}[0], after: &calls}
	nothing := &direct{after: &calls}
	c := NewComposer(figures, &direct{at: &rain, after: &calls}, items(func(f *Frame) {
		f.Sprite(h(0.75), a, 0, unit, even(1))  // a route
		f.Sprite(h(5), a, 0, unit, even(1))     // a figure's material
		f.Sprite(OnTop(0), a, 0, unit, even(1)) // a mark
		f.Uniform("Seen", 7)
	}), nil, views, ground, &direct{at: &sky, after: &calls}, nothing)
	c.draw = func(*Image, []Vertex, []uint16, *Image, [4]float32) { calls++ }
	c.compose(topDown(), wholePart)
	c.render(nil, false, 0)
	if len(c.sources) != 7 || len(c.directs) != 7 {
		t.Fatalf("%d sources, %d directs, want the nil layer left out", len(c.sources),
			len(c.directs))
	}
	if got := ground.drawn; len(got) != 1 || got[0] != 0 || ground.passes[0] != Opaque {
		t.Errorf("the ground drew after %v calls in %v, want once before all, in the opaque pass",
			got, ground.passes)
	}
	if got := views.drawn; len(got) != 1 || got[0] != 0 || views.passes[0] != SeeThrough {
		t.Errorf("the views drew after %v calls in %v, want once before the route's",
			got, views.passes)
	}
	if got := figures.drawn; len(got) != 1 || got[0] != 1 {
		t.Errorf(
			"the figures drew after %v calls, want after the route's, before the material's", got)
	}
	if got := c.directs[1].(*direct).drawn; len(got) != 1 || got[0] != 2 {
		t.Errorf("the rain drew after %v calls, want after the world's, before the marks'", got)
	}
	if len(nothing.drawn) != 0 || calls != 3 {
		t.Errorf("a direct asking for nothing drew %v times; %d calls, want 3",
			len(nothing.drawn), calls)
	}
	if len(views.sun) != 1 || views.sun[0] != 7 {
		t.Errorf("the direct was handed the uniform Seen as %v, want the 7 a source set", views.sun)
	}
	var places []Pass
	c.frame.Places(func(p Pass, _ At) { places = append(places, p) })
	if len(places) != 5 || places[0] != Opaque {
		t.Errorf("the frame holds the places %v, want the ground's opaque one and four seen "+
			"through", places)
	}
}

func TestNewShaderWith_BuildsOnTheLibraryAndSealsTheMaterials(t *testing.T) {
	gputest.Need(t)
	s := gfx.NewShaderWith("test", []byte(`
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    return TestTint(vec2<f32>(0.0), U.Shade, noise(src), vec4<f32>(U.Clock));
}
`), []gfx.Uniform{{Name: "Shade", Size: 1}})
	if err := s.Compile(); err != nil {
		t.Fatalf("a fragment on the library does not compile: %v", err)
	}
	if !strings.Contains(s.Source(), "return TestTint(p, red, fraction, custom);") {
		t.Error("the library holds no material registered")
	}
	if src := ShaderSource(); !strings.Contains(src, "fn faded(") ||
		!strings.Contains(src, "fn TestTint(") {
		t.Error("the composer's shader is not built on the library and its materials")
	}
	defer func() {
		if recover() == nil {
			t.Error("a material registered after a shader was built on them did not panic")
		}
	}()
	wgsl.RegisterMaterials([]byte("fn Late(p: vec2<f32>, red: f32, fraction: f32, custom: vec4<f32>) -> vec4<f32> { return vec4<f32>(0.0); }"), nil, "Late")
}

func TestComposer_TheMarksTakeNoSight(t *testing.T) {
	var c Composer
	c.sight = gpu.Sight{Eye: [3]float32{1, 2, 3}}
	if c.sightFor(gpu.Tested) != nil {
		t.Error("a composer that cannot see hands out a sight")
	}
	c.seen = true
	if c.sightFor(gpu.Tested) != &c.sight {
		t.Error("the world's pieces take no sight")
	}
	if c.sightFor(gpu.Untested) != nil {
		t.Error("a mark takes the sight, which cannot say its depth over the sky")
	}
}

// drawApart is a frame of the ground laid and what lies over it drawn with no screen, as the
// tests draw a whole one through compose and render
func (c *Composer) drawApart(cam camera.Camera) {
	c.Lay(cam, true)
	c.DrawLayer(nil, cam, camera.Window{}, camera.Window{})
	c.render(nil, true, 0)
	c.DrawOver(nil, cam)
	c.renderOver(nil)
}

// overlyingSource and overlyingDirect are a source and a Direct whose pieces lie over the figures
type overlyingSource struct{ Source }

func (overlyingSource) Overlies() {}

type overlyingDirect struct{ Direct }

func (overlyingDirect) Overlies() {}

// a Direct noting when it is composed and drawn, the pass and the camera's viewport of each draw
type staged struct {
	name     string
	opaque   bool
	draws    bool
	at       At
	ground   bool
	set      func(f *Frame)
	log      *[]string
	composed int
	passes   []Pass
	views    [][2]float32
	u        Uniforms
}

func (*staged) Init(*goke.SysInit) {}
func (s *staged) Compose(f *Frame, _ camera.Camera) {
	s.composed++
	*s.log = append(*s.log, s.name+" composed")
	if s.opaque {
		f.DrawOpaque()
	}
	if s.draws {
		f.DrawAt(s.at)
	}
	if s.ground {
		f.ReadGround()
	}
	if s.set != nil {
		s.set(f)
	}
}
func (s *staged) Draw(t Target, cam camera.Camera, u Uniforms) {
	w, h := cam.Viewport()
	*s.log = append(*s.log, s.name+" drawn")
	s.passes = append(s.passes, t.Pass)
	s.views = append(s.views, [2]float32{w, h})
	s.u = u
}

func TestComposer_LayAndDrawOverComposeEachSourceOnceInItsPart(t *testing.T) {
	var log []string
	ground := &staged{name: "ground", opaque: true, log: &log}
	route := &staged{name: "route", draws: true, at: h(0.75), log: &log}
	figures := &staged{name: "figures", draws: true, at: h(2), log: &log}
	rain := &staged{name: "rain", opaque: true, draws: true, at: Nearest, log: &log}
	c := NewComposer(ground, overlyingDirect{figures}, route, overlyingDirect{rain})
	c.draw = func(*Image, []Vertex, []uint16, *Image, [4]float32) {}
	cam := topDown()
	c.drawApart(cam)
	if want := []bool{false, true, false, true}; !slices.Equal(c.overlying, want) {
		t.Errorf("the sources lie over the figures %v, want %v: the ones saying so", c.overlying,
			want)
	}
	want := []string{"ground composed", "route composed", "ground drawn", "route drawn",
		"figures composed", "rain composed", "figures drawn", "rain drawn"}
	if !slices.Equal(log, want) {
		t.Errorf("the frame went %v, want %v: the ground's sources composed and drawn, then the "+
			"ones over the figures", log, want)
	}
	for _, s := range []*staged{ground, route, figures, rain} {
		if s.composed != 1 {
			t.Errorf("%s composed %d times, want once", s.name, s.composed)
		}
	}
	if !slices.Equal(ground.passes, []Pass{Opaque}) ||
		!slices.Equal(route.passes, []Pass{SeeThrough}) {
		t.Errorf("the ground drew in %v and the route in %v, want the opaque pass and the one "+
			"seen through", ground.passes, route.passes)
	}
	if !slices.Equal(figures.passes, []Pass{SeeThrough}) ||
		!slices.Equal(rain.passes, []Pass{SeeThrough}) {
		t.Errorf("over the figures the figures drew in %v and the rain in %v, want once each "+
			"seen through: a source over the figures asking for the opaque pass is not heard",
			figures.passes, rain.passes)
	}
	log = log[:0]
	c.Lay(cam, false)
	want = []string{"ground composed", "figures composed", "route composed", "rain composed"}
	if !slices.Equal(log, want) {
		t.Errorf("laid whole the frame went %v, want %v: every source composed once", log, want)
	}
}

func TestComposer_DrawLayerPlacesThePiecesAndDrawsTheDirectsThroughTheCroppedCamera(
	t *testing.T,
) {
	var log []string
	route := &staged{name: "route", draws: true, at: h(0.75), log: &log}
	a := &sheet{"a"}
	c := NewComposer(route, items(func(f *Frame) { f.Sprite(h(1), a, 0, unit, even(1)) }))
	var places [][4]float32
	c.draw = func(_ *Image, _ []Vertex, _ []uint16, _ *Image, place [4]float32) {
		places = append(places, place)
	}
	cam := topDown()
	cam.SetViewport(64, 48)
	crop := cam.(camera.Cropping)
	crop.Crop(camera.Window{W: 64, H: 48}, 1)
	c.Lay(cam, false)
	crop.Uncrop()
	crop.Crop(camera.Window{X: 16, Y: 12, W: 32, H: 24}, 0.5)
	c.DrawLayer(gfx.NewImage(16, 12), cam, camera.Window{X: 16, Y: 12, W: 32, H: 24}, camera.Window{})
	crop.Uncrop()
	c.DrawWorld(gfx.NewImage(64, 48), cam)
	if want := [][4]float32{{0.5, 0.5, -8, -6}, {}}; !slices.Equal(places, want) {
		t.Errorf("the pieces were placed %v, want %v: scaled by the layer's half and moved by its "+
			"window's corner, a whole frame after them not at all", places, want)
	}
	if want := [][2]float32{{16, 12}, {64, 48}}; !slices.Equal(route.views, want) {
		t.Errorf("the Direct drew through viewports %v, want %v: the camera cropped to the layer, "+
			"then whole", route.views, want)
	}
}

func TestComposer_DrawLayerTwiceDrawsEveryDirectTwice(t *testing.T) {
	gputest.Need(t)
	var log []string
	ground := &staged{name: "ground", opaque: true, log: &log}
	route := &staged{name: "route", draws: true, at: h(0.75), log: &log}
	figures := &staged{name: "figures", draws: true, at: h(2), log: &log}
	c := NewComposer(ground, route, overlyingDirect{figures})
	c.draw = func(*Image, []Vertex, []uint16, *Image, [4]float32) {}
	cam := topDown()
	c.Lay(cam, false)
	c.DrawLayer(gfx.NewImage(32, 32), cam, camera.Window{W: 64, H: 64}, camera.Window{})
	c.DrawLayer(gfx.NewImage(16, 16), cam, camera.Window{X: 16, Y: 16, W: 32, H: 32}, camera.Window{})
	want := []string{"ground composed", "route composed", "figures composed",
		"ground drawn", "route drawn", "figures drawn", "ground drawn", "route drawn",
		"figures drawn"}
	if !slices.Equal(log, want) {
		t.Errorf("two layers went %v, want %v: composed once, every Direct drawn in each", log,
			want)
	}
	if len(c.laid) != 0 {
		t.Errorf("laid whole %d layers kept a ground for a pass over them, want none", len(c.laid))
	}
}

func TestComposer_TwoLayersOfOneSizeKeepTheirGroundsApart(t *testing.T) {
	gputest.Need(t)
	var log []string
	c := NewComposer(&staged{name: "ground", opaque: true, log: &log})
	c.draw = func(*Image, []Vertex, []uint16, *Image, [4]float32) {}
	cam := topDown()
	c.Lay(cam, true)
	c.DrawLayer(gfx.NewImage(32, 32), cam, camera.Window{W: 64, H: 64}, camera.Window{})
	c.DrawLayer(gfx.NewImage(32, 32), cam, camera.Window{X: 16, Y: 16, W: 32, H: 32}, camera.Window{})
	if len(c.laid) != 2 || c.laid[0].ground == c.laid[1].ground {
		t.Errorf("two layers of 32 by 32 laid %d grounds, the same depth %v; want two apart",
			len(c.laid), len(c.laid) == 2 && c.laid[0].ground == c.laid[1].ground)
	}
	c.Lay(cam, true)
	if len(c.laid) != 0 {
		t.Errorf("a new Lay keeps %d layers laid before, want none", len(c.laid))
	}
}

func TestComposer_TheOverPassInheritsTheGroundPassesUniformsAndImages(t *testing.T) {
	var log []string
	img := gfx.NewImage(1, 1)
	var fogged, imaged bool
	ground := &staged{name: "ground", log: &log, set: func(f *Frame) {
		f.Uniform("Fog", 0.1, 0.2, 0.3)
		f.Image("TestProbe", img)
	}}
	figures := &staged{name: "figures", draws: true, at: h(2), log: &log, set: func(f *Frame) {
		fogged, imaged = f.Uniformed("Fog"), false
		for _, im := range f.images {
			imaged = imaged || im.name == "TestProbe" && im.img == img
		}
	}}
	c := NewComposer(ground, overlyingDirect{figures})
	c.draw = func(*Image, []Vertex, []uint16, *Image, [4]float32) {}
	cam := topDown()
	c.drawApart(cam)
	if !fogged || !imaged {
		t.Errorf("composing over the figures the frame holds Fog %v and the image %v, want both "+
			"as the ground's sources set them", fogged, imaged)
	}
	if fog := figures.u.Get("Fog"); len(fog) != 3 || fog[0] != 0.1 || fog[2] != 0.3 {
		t.Errorf("over the figures a Direct is handed Fog %v, want the ground's (0.1, 0.2, 0.3)",
			fog)
	}
	if figures.u.m["TestProbe"] != img {
		t.Errorf("over the figures a Direct is handed %v as the image, want the ground's",
			figures.u.m["TestProbe"])
	}
	ground.set = nil
	c.drawApart(cam)
	if fogged || imaged || figures.u.Get("Fog")[0] != 0 {
		t.Errorf("after a ground pass setting nothing the frame holds Fog %v and the image %v, "+
			"Fog %v handed; want none", fogged, imaged, figures.u.Get("Fog"))
	}
}
