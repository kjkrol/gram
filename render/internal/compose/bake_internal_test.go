package compose

import (
	"image"
	"math"
	"slices"
	"testing"

	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/light"
)

// a ground of 17 by 17 corners 32 apart, its heights as a test raises them
type ground struct {
	heights []float32
	version uint64
	img     *Image
}

func groundOf(raise func(x, y int) float32) *ground {
	g := &ground{heights: make([]float32, 17*17)}
	for y := range 17 {
		for x := range 17 {
			g.heights[y*17+x] = raise(x, y)
		}
	}
	return g
}

func level() *ground { return groundOf(func(int, int) float32 { return 0 }) }

// a ridge 30 high along x = 8 cells
func hill() *ground {
	return groundOf(func(x, _ int) float32 {
		if x == 8 {
			return 30
		}
		return 0
	})
}

// a wall 100 high east of x 288, level ground west of it
func wall() *ground {
	return groundOf(func(x, _ int) float32 {
		if x >= 9 {
			return 100
		}
		return 0
	})
}

func (g *ground) lattice() Lattice {
	low, high := float32(math.Inf(1)), float32(math.Inf(-1))
	for _, h := range g.heights {
		low, high = min(low, h), max(high, h)
	}
	return Lattice{Cols: 17, Rows: 17, Cell: 32, Low: low, Span: max(high-low, 1),
		Version: g.version}
}

// the frame's uniforms: the sun the way way and the ground's heights (GroundHeights), 16
// bits a corner as the terrain encodes them
func (g *ground) uniforms(way [3]float32) Uniforms {
	lat := g.lattice()
	if g.img == nil {
		g.img = gfx.NewImage(17, 17)
		var buf []byte
		for _, h := range g.heights {
			v := uint32(min(max((h-lat.Low)/lat.Span, 0), 1)*65535 + 0.5)
			buf = append(buf, byte(v>>8), byte(v&255), 0, 255)
		}
		g.img.WritePixels(buf)
	}
	return UniformsOf(map[string]any{
		"LightWay":      way[:],
		"LightStrength": []float32{1},
		"GroundHeights": g.img,
		"GroundLattice": []float32{17, 17, 32, 1},
		"GroundSpan":    []float32{0, 0, lat.Low, lat.Span},
	})
}

// frame plans and bakes a frame's shade of g under the sun the way way, with blocks standing
func frame(t *testing.T, b *Baker, g *ground, way [3]float32, blocks ...Block) {
	t.Helper()
	b.Plan(g.lattice(), way, []BlockSet{setOf(blocks...)})
	b.Bake(g.uniforms(way))
}

// setOf is blocks as one set, its version a hash of them: another whenever they stand otherwise
func setOf(blocks ...Block) BlockSet {
	set := BlockSet{Version: 1, Blocks: blocks}
	if len(blocks) == 0 {
		return set
	}
	lo := [2]float32{float32(math.Inf(1)), float32(math.Inf(1))}
	hi := [2]float32{float32(math.Inf(-1)), float32(math.Inf(-1))}
	h := uint64(14695981039346656037)
	for _, b := range blocks {
		ux, uy := float32(math.Abs(float64(b.UX))), float32(math.Abs(float64(b.UY)))
		hx, hy := ux*b.Along+uy*b.Wide, uy*b.Along+ux*b.Wide
		lo = [2]float32{min(lo[0], b.X-hx), min(lo[1], b.Y-hy)}
		hi = [2]float32{max(hi[0], b.X+hx), max(hi[1], b.Y+hy)}
		set.Top = max(set.Top, b.Top)
		for _, v := range [8]float32{b.X, b.Y, b.UX, b.UY, b.Along, b.Wide, b.Bottom, b.Top} {
			h = (h ^ uint64(math.Float32bits(v))) * 1099511628211
		}
	}
	set.Box = [4]float32{lo[0], lo[1], hi[0] - lo[0], hi[1] - lo[1]}
	set.Version = h | 1
	return set
}

func bakedOver(t *testing.T, g *ground, sun [3]float32, blocks ...Block) *Baker {
	t.Helper()
	b := &Baker{}
	frame(t, b, g, sun, blocks...)
	return b
}

// pixelAt is the baked shade's pixel over the world point x, y, eight pixels a cell of 32
func pixelAt(b *Baker, x, y float32) [4]byte {
	pix := make([]byte, 4)
	b.baked.SubImage(image.Rect(int(x/4), int(y/4), int(x/4)+1, int(y/4)+1)).ReadPixels(pix)
	return [4]byte(pix)
}

var eastAt45 = [3]float32{0.7071, 0, 0.7071}

func TestBaker_BakesTheShadeInStripsAsTheSunTurns(t *testing.T) {
	gputest.Need(t)
	// low in the east or the west
	sun := func(x float32) [3]float32 { return [3]float32{x, 0.1, 0.3} }
	// a third of a degree on: turned, not leapt
	nudged := func(x float32) [3]float32 { return [3]float32{x, 0.1 + 0.0055, 0.3} }
	shade := func(b *Baker) []byte {
		pix := make([]byte, 4*b.baked.Bounds().Dx()*b.baked.Bounds().Dy())
		b.baked.ReadPixels(pix)
		return pix
	}
	want, before := shade(bakedOver(t, hill(), nudged(0.9))), shade(bakedOver(t, hill(), sun(0.9)))
	if string(want) == string(before) {
		t.Fatal("the sun turned casts the same shade: the test sees nothing")
	}

	b, g := &Baker{}, hill()
	frame(t, b, g, sun(0.9))
	frame(t, b, g, nudged(0.9))
	got := shade(b)
	w := len(got) / (4 * 128)        // pixels a row
	per := 128 / shadeStrips * w * 4 // bytes a strip
	if string(got[:per]) != string(want[:per]) || string(got[per:]) != string(before[per:]) {
		t.Fatal("after one frame of a turned sun the shade is not its first strip anew and the " +
			"rest as it was")
	}
	if b.Plan(g.lattice(), nudged(0.9), nil); b.Pending() {
		t.Error("the sun still, the next frame bakes a strip again")
	}
	b.Bake(g.uniforms(nudged(0.9)))

	frame(t, b, g, sun(-0.9)) // leapt to the other side
	west := shade(bakedOver(t, hill(), sun(-0.9)))
	if got := shade(b); string(got[:per]) != string(west[:per]) ||
		string(got[per:]) == string(west[per:]) {
		t.Error("the sun leapt, the shade is not its first strip anew alone")
	}
	for range shadeStrips - 1 {
		frame(t, b, g, sun(-0.9))
	}
	if string(shade(b)) != string(west) {
		t.Error("a round of frames after the sun leapt, the shade is not what a bake of the " +
			"whole holds")
	}
	if b.Plan(g.lattice(), sun(-0.9), nil); b.Pending() {
		t.Error("every strip baked anew after the leap, yet the frame bakes")
	}
	b.Bake(g.uniforms(sun(-0.9)))

	b.Coarse(true)
	frame(t, b, g, sun(-0.9))
	if bakedScale(17, 17, true) != 4 || len(shade(b)) != 4*(4*16)*(4*16) {
		t.Errorf("coarse, the baked image is %v, want 4 pixels a cell", b.baked.Bounds())
	}
}

func TestBaker_BakesAStripEachSixteenthOfADegreeTheSunTurns(t *testing.T) {
	gputest.Need(t)
	// up a great circle from 45° high in the east: the ways deg apart
	at := func(deg float64) [3]float32 {
		a := (45 + deg) * math.Pi / 180
		return [3]float32{float32(math.Cos(a)), 0, float32(math.Sin(a))}
	}
	b, g := &Baker{}, hill()
	frame(t, b, g, at(0))
	// a hundredth of a degree a frame: a sixteenth turned every seventh
	var froms []int
	for k := 1; k <= 7*shadeStrips; k++ {
		b.Plan(g.lattice(), at(float64(k)/100), nil)
		if b.Pending() {
			if k%7 != 0 {
				t.Fatalf("a strip baked %d frames on, want every seventh", k)
			}
			froms = append(froms, b.plan.from)
		}
		b.Bake(g.uniforms(at(float64(k) / 100)))
	}
	if len(froms) != shadeStrips || !slices.IsSorted(froms) || froms[0] != 0 {
		t.Errorf("the strips baked begin at rows %v, want each of %d once, down the shade",
			froms, shadeStrips)
	}
}

func TestBaker_WaitsForTheReliefsHeights(t *testing.T) {
	gputest.Need(t)
	b, g := &Baker{}, wall()
	sun := [3]float32{0.9487, 0, 0.3162}
	if _, shown := b.Plan(g.lattice(), sun, nil); shown {
		t.Error("the shade shows before anything is baked")
	}
	b.Bake(UniformsOf(map[string]any{"LightWay": sun[:]}))
	if _, shown := b.Plan(g.lattice(), sun, nil); shown || !b.Pending() {
		t.Error("a frame without the ground's heights baked the shade, or the next plans nothing")
	}
	b.Bake(g.uniforms(sun))
	if _, shown := b.Plan(g.lattice(), sun, nil); !shown {
		t.Error("the heights handed, the shade still does not show")
	}
	if v := pixelAt(b, 228, 300)[0]; v > 10 {
		t.Errorf("60 west of the wall the ground is lit %d, want dark", v)
	}
}

func TestBaker_BakesTheShadowsOfTheBlocks(t *testing.T) {
	gputest.Need(t)
	b, g := &Baker{}, level()
	// a deck 64 along x either side of (256, y), 16 across, from 40 up to 50
	deck := func(y float32) Block {
		return Block{X: 256, Y: y, UX: 1, Along: 64, Wide: 16, Bottom: 40, Top: 50}
	}
	shadeAt := func(x, y float32) byte { return pixelAt(b, x, y)[0] }
	frame(t, b, g, eastAt45, deck(256))
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
	frame(t, b, g, eastAt45, deck(320))
	if v := shadeAt(200, 256); v < 245 {
		t.Errorf("the deck moved away, the ground is lit %d, want baked anew lit", v)
	}
	if v := shadeAt(200, 320); v > 10 {
		t.Errorf("the deck moved over, the ground is lit %d, want baked anew dark", v)
	}
}

func TestBaker_BakesTheBlocksOnLevelGroundWithoutHeights(t *testing.T) {
	gputest.Need(t)
	b := &Baker{}
	flat := Lattice{Cols: 17, Rows: 17, Cell: 32}
	post := Block{X: 256, Y: 256, UX: 1, Along: 8, Wide: 8, Top: 40}
	b.Plan(flat, eastAt45, []BlockSet{setOf(post)})
	b.Bake(UniformsOf(map[string]any{"LightWay": eastAt45[:],
		"LightStrength": []float32{1}}))
	if v := pixelAt(b, 236, 256)[0]; v > 10 {
		t.Errorf("west of the post on level ground the sun reaches %d, want its shadow", v)
	}
	if v := pixelAt(b, 300, 256)[0]; v < 245 {
		t.Errorf("east of the post the sun reaches %d, want all of it", v)
	}
	if v := pixelAt(b, 236, 300)[0]; v < 245 {
		t.Errorf("beside the post's shadow the sun reaches %d, want all of it", v)
	}
}

func TestBaker_BakesTheShadowsOfAsManyBlocksAsStand(t *testing.T) {
	gputest.Need(t)
	var posts []Block
	for i := range 200 {
		x, y := 40+float32(i%20)*24, 40+float32(i/20)*48
		posts = append(posts, Block{X: x, Y: y, UX: 1, Along: 3, Wide: 3, Top: 20 + float32(i%3)})
	}
	b := bakedOver(t, level(), eastAt45, posts...)
	for i, p := range posts {
		// the line to the sun rises 1 east for 1 up: from 10 west of a post it meets it
		if v := pixelAt(b, p.X-10, p.Y)[0]; v > 40 {
			t.Fatalf("10 west of post %d the ground is lit %d, want in its shadow", i, v)
		}
	}
}

func TestBaker_BakesOverlappingShadowsAsTheDarkerOfThem(t *testing.T) {
	gputest.Need(t)
	a := Block{X: 256, Y: 250, UX: 1, Along: 30, Wide: 8, Top: 40}
	c := Block{X: 270, Y: 262, UX: 1, Along: 30, Wide: 8, Top: 60}
	ra, rb := bakedOver(t, level(), eastAt45, a), bakedOver(t, level(), eastAt45, c)
	both := bakedOver(t, level(), eastAt45, a, c)
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

// a pit 1000 deep at the north-western corner, level ground everywhere else: the lowest ground far
// below what stands on the level
func pit() *ground {
	return groundOf(func(x, y int) float32 {
		if x == 0 && y == 0 {
			return -1000
		}
		return 0
	})
}

// the sun 3° over the eastern horizon
var eastAt3 = [3]float32{0.9986, 0, 0.0523}

func TestBaker_ReachesNoFartherThanAnyShadowUnderALowSun(t *testing.T) {
	gputest.Need(t)
	post := Block{X: 400, Y: 256, UX: 1, Along: 4, Wide: 4, Bottom: 0, Top: 40}
	b := bakedOver(t, pit(), eastAt3, post)
	// the line from the post's top meets the level ground 765 west; a shadow reaches six heights
	if v := pixelAt(b, 200, 256)[0]; v > 10 {
		t.Errorf("200 west of the post the ground is lit %d, want in its shadow", v)
	}
	if v := pixelAt(b, 100, 256)[0]; v < 245 {
		t.Errorf("300 west of the post the ground is lit %d, want lit: a shadow reaches %d "+
			"heights and no farther", v, light.ShadowReach)
	}
	if v := pixelAt(b, 440, 256)[0]; v < 245 {
		t.Errorf("east of the post the ground is lit %d, want all of it", v)
	}
}

func TestBaker_ReachesToTheLowestGroundAndNoFartherThanAnyShadow(t *testing.T) {
	b := &Baker{lat: Lattice{Cols: 17, Rows: 17, Cell: 32, Low: -1000, Span: 1000}}
	box := [4]float32{396, 252, 8, 8}
	// a post 40 tall with the lowest ground 1000 below: under the sun 45° high its shadow would
	// come down to it 1040 west, under the sun 3° high 20 000 west; a shadow reaches six heights,
	// 240 west, and a cell and the penumbra more
	for _, sun := range [][3]float32{eastAt45, eastAt3} {
		if r := b.reach(box, 40, 40, sun); r[0] < 396-240-32-30 || r[0] > 396-240-32 {
			t.Errorf("under the sun %v the shadows reach west to %v, want about %v", sun, r[0],
				396-240-32-20)
		}
	}
	// on level ground the shadow comes down 40 west under the sun 45° high, short of six heights
	b.lat.Low, b.lat.Span = 0, 1
	if r := b.reach(box, 40, 40, eastAt45); r[0] < 396-40-32-5 || r[0] > 396-40-32 {
		t.Errorf("on level ground the shadows reach west to %v, want about %v", r[0], 396-40-32-3)
	}
	b.lat.Low, b.lat.Span = -1000, 1000
	if r := b.reach(box, 40, 40, [3]float32{0, 0, 1}); r[0] < 396-32-1040*shadowSoft-1 ||
		r[2] > 404+32+1040*shadowSoft+1 {
		t.Errorf("under the sun overhead the shadows reach %v, want the box, a cell and the "+
			"penumbra", r)
	}
	// shading takes each block by the same measure
	near := b.shading([4]float32{0, 0, 300, 512}, eastAt3)
	b.sets = []bakedSet{{box: box, top: 40, rise: 40,
		blocks: []Block{{X: 400, Y: 256, UX: 1, Along: 4, Wide: 4, Top: 40}}}}
	if near = b.shading([4]float32{0, 0, 300, 512}, eastAt3); len(near) == 0 {
		t.Errorf("the post's shadow, 240 long, misses the ground within 300 of the west edge")
	}
	if near = b.shading([4]float32{0, 0, 100, 512}, eastAt3); len(near) != 0 {
		t.Errorf("the post's shadow, 240 long, falls on the ground within 100 of the west edge")
	}
}

func TestBaker_BakesHowHighTheReliefsShadowReaches(t *testing.T) {
	gputest.Need(t)
	// low in the east: the line to the sun rises 1 for 3 across
	b := bakedOver(t, wall(), [3]float32{0.9487, 0, 0.3162})
	reach := func(x float32) float32 {
		g := float32(pixelAt(b, x, 256)[1]) / 255
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

func TestBaker_BakesHowHighABlocksShadowReaches(t *testing.T) {
	gputest.Need(t)
	// a deck 128 along x round (256, 256), 32 across, from 40 up to 50
	deck := Block{X: 256, Y: 256, UX: 1, Along: 64, Wide: 16, Bottom: 40, Top: 50}
	b := bakedOver(t, level(), eastAt45, deck)
	top := func(x, y float32) float32 {
		v := 1 - float32(pixelAt(b, x, y)[2])/255
		return v * v * 50
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
	if a := pixelAt(b, 184, 256)[3]; a > 10 {
		t.Errorf("8 west of the deck the ground is lit %d past the blocks, want dark", a)
	}
}

// what the library reads of the shade (groundShadeAt, standingShadeAt) at the points U.Probe0..3,
// (x, y, how high over the ground, 0 the relief's or 1 the blocks'), a pixel each
var shadeProbe = gfx.NewMeshShaderWith("shade probe", []byte(`
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
`), []gfx.Uniform{
	{Name: "Probe0", Size: 4},
	{Name: "Probe1", Size: 4},
	{Name: "Probe2", Size: 4},
	{Name: "Probe3", Size: 4},
})

func TestGroundShade_TellsWhatStandsInAShadowAtItsHeight(t *testing.T) {
	gputest.Need(t)
	// low in the east: rising 1 for 3 across
	b := bakedOver(t, wall(), [3]float32{0.9487, 0, 0.3162},
		Block{X: 96, Y: 256, UX: 1, Along: 32, Wide: 16, Bottom: 16, Top: 20})
	shade := b.Shade()
	u := map[string]any{
		"GroundShade":     shade.Now,
		"GroundShadeWas":  shade.Was,
		"GroundShadeArea": shade.Area[:],
		"GroundShadeSpan": []float32{shade.Span, shade.Band},
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
	img := gfx.NewImage(4, 1)
	img.DrawMesh(nil, shadeProbe, &gfx.DrawMeshOptions{Vertices: 3, Uniforms: u})
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

func TestBaker_SetKeepsASliceForEachName(t *testing.T) {
	b := &Baker{}
	b.baking.Uniforms, b.own = map[string]any{}, map[string][]float32{}
	b.set("Strip", 1, 2, 3, 4)
	first := b.baking.Uniforms["Strip"].([]float32)
	b.set("Strip", 5, 6, 7, 8)
	again := b.baking.Uniforms["Strip"].([]float32)
	if &first[0] != &again[0] || again[1] != 6 {
		t.Errorf("Strip set twice is %v then %v; want one slice holding the values last set",
			first, again)
	}
	if n := testing.AllocsPerRun(10, func() { b.set("Strip", 1, 2, 3, 4) }); n != 0 {
		t.Errorf("set allocates %v times a call, want none", n)
	}
}
