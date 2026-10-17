package terrain

import (
	"image"
	"testing"

	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/render"
)

// shaded is a renderer baking the ground g's shade under a sun a test turns
type shaded struct {
	*Renderer
	sky *litSky
}

func shadedOver(g Ground) shaded {
	sky := &litSky{}
	return shaded{New(g, none{}, sky, Config{Shadows: true}), sky}
}

// frame composes and draws a frame's shade under the sun the way way
func (s shaded) frame(t *testing.T, way [3]float32) {
	t.Helper()
	s.sky.way = way
	u := render.UniformsOf(map[string]any{"Sun": way[:], "SunStrength": []float32{1}})
	if !s.prepare(icamera.NewFromSpace(512, 512, 0), u) {
		t.Fatal("nothing to draw")
	}
	s.planBake()
	s.bake(u)
}

func TestRenderer_BakesTheShadeInStripsAsTheSunTurns(t *testing.T) {
	needGPU(t)
	// low in the east or the west
	sun := func(x float32) [3]float32 { return [3]float32{x, 0.1, 0.3} }
	// half a degree on: turned, not leapt
	nudged := func(x float32) [3]float32 { return [3]float32{x, 0.1 + 0.0055, 0.3} }
	shade := func(r shaded) []byte {
		cols, rows, _, _, _ := r.ground.Lattice()
		ks := bakedScale(cols, rows, r.coarse)
		part := r.baked.SubImage(image.Rect(0, 0, ks*(cols-1), ks*(rows-1)))
		pix := make([]byte, 4*part.Bounds().Dx()*part.Bounds().Dy())
		part.ReadPixels(pix)
		return pix
	}
	east := shadedOver(&hill{})
	east.frame(t, sun(0.9))
	west := shadedOver(&hill{})
	west.frame(t, nudged(0.9))
	want, before := shade(west), shade(east)

	r := shadedOver(&hill{})
	r.frame(t, sun(0.9))
	r.frame(t, nudged(0.9))
	got := shade(r)
	w := len(got) / (4 * 128)        // pixels a row
	per := 128 / shadeStrips * w * 4 // bytes a strip
	if string(got[:per]) != string(want[:per]) || string(got[per:]) != string(before[per:]) {
		t.Fatal("after one frame of a turned sun the shade is not its first strip anew and the rest as it was")
	}
	for range shadeStrips - 1 {
		r.frame(t, nudged(0.9))
	}
	if got := shade(r); string(got) != string(want) {
		t.Error("after a round of strips the shade is not what a bake of the whole holds")
	}
	if string(want) == string(before) {
		t.Fatal("the sun turned casts the same shade: the test sees nothing")
	}
	r.frame(t, sun(-0.9)) // leapt to the other side
	west = shadedOver(&hill{})
	west.frame(t, sun(-0.9))
	if string(shade(r)) != string(shade(west)) {
		t.Error("the sun leapt, the shade is not baked whole at once")
	}

	r.Coarse(true)
	r.frame(t, sun(-0.9))
	if cols, rows, _, _, _ := r.ground.Lattice(); bakedScale(cols, rows, true) != 4 || len(shade(r)) != 4*(4*(cols-1))*(4*(rows-1)) {
		t.Errorf("coarse, the baked image is %v, want 4 pixels a cell", r.baked.Bounds())
	}
}

type level struct{}

func (level) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	return 17, 17, 32, make([]float32, 17*17), true
}

func (level) Version() uint64 { return 0 }

func TestRenderer_BakesTheShadowsOfTheBlocks(t *testing.T) {
	needGPU(t)
	r := shadedOver(level{})
	// a deck 64 along x either side of (256, y), 16 across, from 40 up to 50, in two pieces
	deckAt := float32(256)
	r.findBlocks = func(dst []block) []block {
		piece := block{ux: 1, along: 32, wide: 16, bottom: 40, top: 50, y: deckAt}
		a, b := piece, piece
		a.x, b.x = 224, 288
		return append(dst, a, b)
	}
	frame := func() { r.frame(t, eastAt45) }
	shadeAt := func(x, y float32) byte {
		pix := make([]byte, 4)
		r.baked.SubImage(image.Rect(int(x/4), int(y/4), int(x/4)+1, int(y/4)+1)).ReadPixels(pix)
		return pix[0]
	}
	frame()
	if n := len(r.blocks) / 8; n != 1 {
		t.Errorf("the deck's two pieces are baked as %d blocks, want one", n)
	}
	// the line to the sun rises 1 east for 1 up: from x it meets the deck over x+40 to x+50
	if v := shadeAt(200, 256); v > 10 {
		t.Errorf("the ground the deck hides from the sun is lit %d, want dark", v)
	}
	if v := shadeAt(310, 256); v < 245 {
		t.Errorf("the ground right under the deck's sunny end is lit %d, want lit: the shadow "+
			"falls away from the sun", v)
	}
	if v := shadeAt(200, 210); v < 245 {
		t.Errorf("the ground beside the shadow is lit %d, want lit", v)
	}
	if v := shadeAt(200, 274); v < 25 || v > 230 {
		t.Errorf("the ground past the shadow's edge is lit %d, want half in the penumbra", v)
	}
	deckAt = 320
	frame()
	if v := shadeAt(200, 256); v < 245 {
		t.Errorf("the deck moved away, the ground is lit %d, want baked anew lit", v)
	}
	if v := shadeAt(200, 320); v > 10 {
		t.Errorf("the deck moved over, the ground is lit %d, want baked anew dark", v)
	}
}

// pixelAt is the baked shade's pixel over the world point x, y, eight pixels a cell of 32
func pixelAt(r *Renderer, x, y float32) [4]byte {
	pix := make([]byte, 4)
	r.baked.SubImage(image.Rect(int(x/4), int(y/4), int(x/4)+1, int(y/4)+1)).ReadPixels(pix)
	return [4]byte(pix)
}

func bakedOver(t *testing.T, g Ground, sun [3]float32, blocks ...block) *Renderer {
	t.Helper()
	r := shadedOver(g)
	r.findBlocks = func(dst []block) []block { return append(dst, blocks...) }
	r.frame(t, sun)
	return r.Renderer
}

var eastAt45 = [3]float32{0.7071, 0, 0.7071}

func TestRenderer_BakesTheShadowsOfAsManyBlocksAsStand(t *testing.T) {
	needGPU(t)
	var posts []block
	for i := range 200 {
		x, y := 40+float32(i%20)*24, 40+float32(i/20)*48
		posts = append(posts, block{x: x, y: y, ux: 1, along: 3, wide: 3, top: 20 + float32(i%3)})
	}
	r := bakedOver(t, level{}, eastAt45, posts...)
	for i, p := range posts {
		// the line to the sun rises 1 east for 1 up: from 10 west of a post it meets it
		if v := pixelAt(r, p.x-10, p.y)[0]; v > 40 {
			t.Fatalf("10 west of post %d the ground is lit %d, want in its shadow", i, v)
		}
	}
}

func TestRenderer_BakesOverlappingShadowsAsTheDarkerOfThem(t *testing.T) {
	needGPU(t)
	a := block{x: 256, y: 250, ux: 1, along: 30, wide: 8, top: 40}
	b := block{x: 270, y: 262, ux: 1, along: 30, wide: 8, top: 60}
	ra, rb := bakedOver(t, level{}, eastAt45, a), bakedOver(t, level{}, eastAt45, b)
	both := bakedOver(t, level{}, eastAt45, a, b)
	for x := float32(160); x < 320; x += 6 {
		for y := float32(236); y < 280; y += 3 {
			want := min(pixelAt(ra, x, y)[0], pixelAt(rb, x, y)[0])
			if got := pixelAt(both, x, y)[0]; int(got)-int(want) > 1 || int(want)-int(got) > 1 {
				t.Fatalf("at %v, %v two blocks leave the ground lit %d, want the darker of "+
					"their shadows alone, %d", x, y, got, want)
			}
		}
	}
}

// a wall 100 high east of x 288, level ground west of it
type wall struct{}

func (wall) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	h := make([]float32, 17*17)
	for i := range h {
		if i%17 >= 9 {
			h[i] = 100
		}
	}
	return 17, 17, 32, h, true
}

func (wall) Version() uint64 { return 0 }

func TestRenderer_BakesHowHighTheReliefsShadowReaches(t *testing.T) {
	needGPU(t)
	// low in the east: the line to the sun rises 1 for 3 across
	r := bakedOver(t, wall{}, [3]float32{0.9487, 0, 0.3162})
	reach := func(x float32) float32 {
		g := float32(pixelAt(r, x, 256)[1]) / 255
		return g * g * 100
	}
	// 60 west of where the wall stands full: shaded up to 100 less a third of 60
	if got := reach(288 - 60); got < 68 || got > 92 {
		t.Errorf("60 west of the wall its shadow reaches %v over the ground, want about 80", got)
	}
	// the march towards the sun steps 0.75 of a cell, so a reach a step's rise short of the line
	if got := reach(288 - 240); got < 10 || got > 22 {
		t.Errorf("240 west of the wall its shadow reaches %v over the ground, want about 20", got)
	}
	if got := reach(400); got > 0.5 {
		t.Errorf("on top of the wall, in the sun, a shadow reaches %v over it, want nothing", got)
	}
}

func TestRenderer_BakesHowHighABlocksShadowReaches(t *testing.T) {
	needGPU(t)
	// a deck 128 along x round (256, 256), 32 across, from 40 up to 50
	deck := block{x: 256, y: 256, ux: 1, along: 64, wide: 16, bottom: 40, top: 50}
	r := bakedOver(t, level{}, eastAt45, deck)
	top := func(x, y float32) float32 {
		b := 1 - float32(pixelAt(r, x, y)[2])/255
		return b * b * 50
	}
	if got := top(256, 256); got < 47 {
		t.Errorf("under the deck its shadow reaches %v over the ground, want its top, 50", got)
	}
	// the line from 8 west of the deck rises 8 before it comes over it
	if got := top(184, 256); got < 38 || got > 46 {
		t.Errorf("8 west of the deck its shadow reaches %v over the ground, want about 42", got)
	}
	if got := top(184, 200); got > 0.5 {
		t.Errorf("beside the deck's shadow it reaches %v over the ground, want nothing", got)
	}
	if a := pixelAt(r, 184, 256)[3]; a > 10 {
		t.Errorf("8 west of the deck the ground is lit %d past the blocks, want dark", a)
	}
}

// what the library reads of the shade (groundShadeAt, standingShadeAt) at the points U.Probe0..3,
// (x, y, how high over the ground, 0 the relief's or 1 the blocks'), a pixel each
var shadeProbe = render.NewMeshShaderWith("shade probe", []byte(`
struct Out { @builtin(position) clip: vec4<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(3.0, -1.0),
        vec2<f32>(-1.0, 3.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    return o;
}
@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    var probes = array<vec4<f32>, 4>(U.Probe0, U.Probe1, U.Probe2, U.Probe3);
    let q = probes[min(u32(o.clip.x), 3u)];
    var lit = groundShadeAt(q.xy, q.z, 1.0);
    if q.w > 0.5 {
        lit = standingShadeAt(q.xy, q.z, 1.0);
    }
    return vec4<f32>(lit, lit, lit, 1.0);
}
`), []render.Uniform{
	{Name: "Probe0", Size: 4},
	{Name: "Probe1", Size: 4},
	{Name: "Probe2", Size: 4},
	{Name: "Probe3", Size: 4},
})

func TestGroundShade_TellsWhatStandsInAShadowAtItsHeight(t *testing.T) {
	needGPU(t)
	r := shadedOver(wall{})
	r.findBlocks = func(dst []block) []block {
		return append(dst, block{x: 96, y: 256, ux: 1, along: 32, wide: 16, bottom: 16, top: 20})
	}
	r.frame(t, [3]float32{0.9487, 0, 0.3162}) // low in the east: rising 1 for 3 across
	u := map[string]any{
		"GroundShade":     r.baked,
		"GroundShadeWas":  r.was,
		"GroundShadeArea": r.shade.Area[:],
		"GroundShadeSpan": []float32{r.shade.Span, r.shade.Band},
		"GroundShadeMix0": []float32{1, 1, 1, 1},
		"GroundShadeMix1": []float32{1, 1, 1, 1},
		"GroundShadeMix2": []float32{1, 1, 1, 1},
		"GroundShadeMix3": []float32{1, 1, 1, 1},
		// 60 west of the wall, its shadow reaching about 80 up: at the ground, and over it
		"Probe0": []float32{228, 300, 2, 0},
		"Probe1": []float32{228, 300, 95, 0},
		// on the deck's top, and on the ground west of it its shadow falls on: the line to the sun
		// from there passes through it
		"Probe2": []float32{96, 256, 21, 1},
		"Probe3": []float32{40, 256, 2, 1},
	}
	img := render.NewImage(4, 1)
	img.DrawMesh(nil, shadeProbe, &render.DrawMeshOptions{Vertices: 3, Uniforms: u})
	pix := make([]byte, 16)
	img.ReadPixels(pix)
	for k, c := range []struct {
		what string
		lit  bool
	}{
		{"at the ground in the wall's shadow", false},
		{"high over the wall's shadow", true},
		{"on the deck's top", true},
		{"on the ground in the deck's shadow", false},
	} {
		if got := pix[4*k]; (got > 200) != c.lit || (got > 40 && got <= 200) {
			t.Errorf("%s the sun reaches %d, want lit %v", c.what, got, c.lit)
		}
	}
}
