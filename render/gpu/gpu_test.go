package gpu

import (
	"os"
	"strings"
	"testing"
)

func needGPU(t *testing.T) {
	t.Helper()
	if err := Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

func pixel(pix []byte, w, x, y int) [4]byte {
	i := 4 * (y*w + x)
	return [4]byte{pix[i], pix[i+1], pix[i+2], pix[i+3]}
}

func TestFill_CoversItsRectangleOnly(t *testing.T) {
	needGPU(t)
	tex := NewTexture(8, 6)
	Fill(Image{Texture: tex}, 0, 0, 1, 1)
	Fill(Image{Texture: tex, Rect: Rect{2, 1, 3, 2}}, 1, 0, 0, 1)
	pix := make([]byte, 4*8*6)
	tex.ReadPixels(pix)
	if p := pixel(pix, 8, 0, 0); p != [4]byte{0, 0, 255, 255} {
		t.Errorf("outside the rectangle %v, want blue", p)
	}
	if p := pixel(pix, 8, 3, 2); p != [4]byte{255, 0, 0, 255} {
		t.Errorf("inside the rectangle %v, want red", p)
	}
	if p := pixel(pix, 8, 5, 2); p != [4]byte{0, 0, 255, 255} {
		t.Errorf("just past the rectangle %v, want blue", p)
	}
}

func TestTriangles_AProgramReadsItsImageAndUniforms(t *testing.T) {
	needGPU(t)
	src := NewTexture(2, 1)
	src.WritePixels(0, 0, 2, 1, []byte{255, 0, 0, 255, 0, 255, 0, 255})
	layout := NewLayout([]Uniform{{"Shade", 1}, {"Tint", 3}})
	p := NewProgram("test", layout.Fields, `
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let c = imageSrc0At(vec2<f32>(dst.x * 0.5, 0.5));
    return vec4<f32>(c.rgb * U.Shade + U.Tint, 1.0);
}`)
	dst := NewTexture(4, 1)
	u := make([]byte, layout.Size)
	layout.Put(u, "Shade", []float32{0.5})
	layout.Put(u, "Tint", []float32{0, 0, 0.25})
	v := func(x, y float32) Vertex { return Vertex{DstX: x, DstY: y} }
	Triangles(&Draw{Target: Image{Texture: dst}, Program: p, Images: [4]Image{{Texture: src}}, Uniforms: u, Blend: Copy},
		[]Vertex{v(0, 0), v(4, 0), v(0, 1), v(4, 1)}, []uint16{0, 1, 2, 1, 2, 3})
	pix := make([]byte, 16)
	dst.ReadPixels(pix)
	if p := pixel(pix, 4, 0, 0); p[0] < 126 || p[0] > 129 || p[2] < 62 || p[2] > 65 {
		t.Errorf("left %v, want half red over a quarter of blue", p)
	}
	if p := pixel(pix, 4, 3, 0); p[1] < 126 || p[1] > 129 {
		t.Errorf("right %v, want half green", p)
	}
}

func TestDrawImage_LaysItOverWhatIsThere(t *testing.T) {
	needGPU(t)
	dst := NewTexture(4, 4)
	Fill(Image{Texture: dst}, 1, 1, 1, 1)
	src := NewTexture(2, 2)
	half := []byte{0, 0, 128, 128}
	src.WritePixels(0, 0, 2, 2, append(append(append(append([]byte{}, half...), half...), half...), half...))
	DrawImage(Image{Texture: dst}, Image{Texture: src}, Affine{A: 1, D: 1, TX: 1, TY: 1}, [4]float32{1, 1, 1, 1}, false, SourceOver)
	pix := make([]byte, 64)
	dst.ReadPixels(pix)
	if p := pixel(pix, 4, 2, 2); p[0] < 126 || p[0] > 129 || p[2] < 253 {
		t.Errorf("under the half-blue %v, want white half covered by blue", p)
	}
	if p := pixel(pix, 4, 0, 0); p != [4]byte{255, 255, 255, 255} {
		t.Errorf("outside it %v, want white", p)
	}
}

func TestLayout_FollowsWGSL(t *testing.T) {
	l := NewLayout([]Uniform{{"A", 1}, {"B", 3}, {"C", 1}, {"D", 2}})
	if l.offsets["A"] != 0 || l.offsets["B"] != 16 || l.offsets["C"] != 28 || l.offsets["D"] != 32 || l.Size != 48 {
		t.Errorf("offsets %v size %d, want A 0, B 16, C 28, D 32, 48 in all", l.offsets, l.Size)
	}
	l = NewLayout([]Uniform{{"A", 1}, {"E", 20}, {"F", 1}})
	if l.offsets["E"] != 16 || l.offsets["F"] != 96 || l.Size != 112 {
		t.Errorf("offsets %v size %d, want an array of 5 vec4s at 16, F after it at 96, 112",
			l.offsets, l.Size)
	}
	if !strings.Contains(l.Fields, "E: array<vec4<f32>, 5>,") {
		t.Errorf("fields %q, want E an array of 5 vec4s", l.Fields)
	}
}

func TestTriangles_AProgramReadsAnArrayOfUniforms(t *testing.T) {
	needGPU(t)
	layout := NewLayout([]Uniform{{"Count", 1}, {"Values", 20}})
	p := NewProgram("test array", layout.Fields, `
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let k = i32(U.Count);
    return vec4<f32>(U.Values[k].x, U.Values[k].w, U.Values[4].y, 1.0);
}`)
	dst := NewTexture(1, 1)
	u := make([]byte, layout.Size)
	values := make([]float32, 20)
	values[8], values[11], values[17] = 1, 0.5, 0.25
	layout.Put(u, "Count", []float32{2})
	layout.Put(u, "Values", values)
	v := func(x, y float32) Vertex { return Vertex{DstX: x, DstY: y} }
	Triangles(&Draw{Target: Image{Texture: dst}, Program: p, Uniforms: u, Blend: Copy},
		[]Vertex{v(0, 0), v(1, 0), v(0, 1), v(1, 1)}, []uint16{0, 1, 2, 1, 2, 3})
	pix := make([]byte, 4)
	dst.ReadPixels(pix)
	if pix[0] < 254 || pix[1] < 126 || pix[1] > 129 || pix[2] < 62 || pix[2] > 65 {
		t.Errorf("read %v, want the third vec4's x and w, the fifth's y: 255, 128, 64", pix)
	}
}

func TestWritePixels_APartLandsInPlaceAndLeavesTheRest(t *testing.T) {
	needGPU(t)
	const w, h = 97, 3
	tex := NewTexture(w, h)
	all := make([]byte, 4*w*h)
	for i := range w * h {
		all[4*i], all[4*i+3] = byte(i%200), 255
	}
	tex.WritePixels(0, 0, w, h, all)
	part := []byte{7, 7, 7, 255, 8, 8, 8, 255, 9, 9, 9, 255, 10, 10, 10, 255}
	tex.WritePixels(50, 1, 2, 2, part)
	got := make([]byte, 4*w*h)
	tex.ReadPixels(got)
	for y := range h {
		for x := range w {
			want := [4]byte{byte((y*w + x) % 200), 0, 0, 255}
			if x >= 50 && x < 52 && y >= 1 {
				k := 4 * ((y-1)*2 + x - 50)
				want = [4]byte{part[k], part[k+1], part[k+2], part[k+3]}
			}
			if p := pixel(got, w, x, y); p != want {
				t.Fatalf("pixel (%d, %d) is %v, want %v", x, y, p, want)
			}
		}
	}
}

func TestMesh_TheNearerCoversTheFurtherWhicheverComesFirst(t *testing.T) {
	needGPU(t)
	layout := NewLayout([]Uniform{{"Depth", 1}, {"Color", 4}})
	p := NewMesh("test mesh", layout.Fields, `
struct Out { @builtin(position) clip: vec4<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 4>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(1.0, -1.0),
        vec2<f32>(-1.0, 1.0),
        vec2<f32>(1.0, 1.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], U.Depth, 1.0);
    return o;
}
@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> { return U.Color; }`)
	dst := NewTexture(4, 4)
	depth := NewDepth()
	quad := NewIndices([]uint32{0, 1, 2, 1, 2, 3})
	draw := func(d float32, c [4]float32, clear bool) {
		u := make([]byte, layout.Size)
		layout.Put(u, "Depth", []float32{d})
		layout.Put(u, "Color", c[:])
		Mesh(&MeshDraw{Target: Image{Texture: dst}, Depth: depth, ClearDepth: clear, Write: true, Program: p, Uniforms: u, Indices: quad, Blend: Copy})
	}
	draw(0.8, [4]float32{1, 0, 0, 1}, true)  // near, red
	draw(0.2, [4]float32{0, 0, 1, 1}, false) // far, blue: behind
	pix := make([]byte, 64)
	dst.ReadPixels(pix)
	if p := pixel(pix, 4, 1, 1); p != [4]byte{255, 0, 0, 255} {
		t.Errorf("after the near red and the far blue %v, want red", p)
	}
	draw(0.2, [4]float32{0, 0, 1, 1}, true)  // far, blue
	draw(0.8, [4]float32{0, 1, 0, 1}, false) // near, green: in front
	dst.ReadPixels(pix)
	if p := pixel(pix, 4, 2, 2); p != [4]byte{0, 255, 0, 255} {
		t.Errorf("after the far blue and the near green %v, want green", p)
	}
}

func TestMesh_AnInstancedMeshIsDrawnOnceAnInstance(t *testing.T) {
	needGPU(t)
	p := NewMesh("test instances", "", `
struct Out {
    @builtin(position) clip: vec4<f32>,
    @location(0) color: vec4<f32>,
}
@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>, @location(1) color: vec4<f32>) -> Out {
    var corners = array<vec2<f32>, 6>(vec2<f32>(0.0, 0.0), vec2<f32>(1.0, 0.0), vec2<f32>(0.0, 1.0), vec2<f32>(1.0, 0.0), vec2<f32>(1.0, 1.0), vec2<f32>(0.0, 1.0));
    let px = at.xy + corners[vid] * at.zw; // a rectangle in the target's pixels
    var o: Out;
    o.clip = vec4<f32>(px.x / D.target.x * 2.0 - 1.0, 1.0 - px.y / D.target.y * 2.0, 0.0, 1.0);
    o.color = color;
    return o;
}
@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> { return o.color; }`).Instanced(2)
	dst := NewTexture(4, 4)
	Mesh(&MeshDraw{Target: Image{Texture: dst}, Program: p, Vertices: 6, Blend: Copy, Instances: []float32{
		0, 0, 2, 4, 1, 0, 0, 1, // the left half, red
		2, 0, 2, 4, 0, 0, 1, 1, // the right half, blue
	}})
	pix := make([]byte, 64)
	dst.ReadPixels(pix)
	if l, r := pixel(pix, 4, 0, 2), pixel(pix, 4, 3, 1); l != [4]byte{255, 0, 0, 255} || r != [4]byte{0, 0, 255, 255} {
		t.Errorf("the left is %v and the right %v, want red and blue", l, r)
	}
}

func TestWritePixels_GoesInTheOrderOfTheDraws(t *testing.T) {
	needGPU(t)
	red, green, blue := []byte{255, 0, 0, 255}, []byte{0, 255, 0, 255}, []byte{0, 0, 255, 255}
	src := NewTexture(3, 1)
	Fill(Image{Texture: src}, 1, 0, 0, 1)
	src.WritePixels(1, 0, 1, 1, green)
	dst := NewTexture(3, 1)
	DrawImage(Image{Texture: dst}, Image{Texture: src}, Affine{A: 1, D: 1}, [4]float32{1, 1, 1, 1},
		false, Copy)
	// written after the draw that copied it, with no submission between
	src.WritePixels(2, 0, 1, 1, blue)
	got, was := make([]byte, 12), make([]byte, 12)
	dst.ReadPixels(got)
	src.ReadPixels(was)
	for x, want := range [][]byte{red, green, red} {
		if p := pixel(got, 3, x, 0); p != [4]byte(want) {
			t.Errorf("drawn at %d is %v, want %v", x, p, want)
		}
	}
	if p := pixel(was, 3, 2, 0); p != [4]byte(blue) {
		t.Errorf("written last at 2 is %v, want blue", p)
	}
}

func TestWritePixels_ManyInAFrameAllLand(t *testing.T) {
	needGPU(t)
	const side, n = 16, 24
	tex := NewTexture(side*n, side)
	for k := range n {
		part := make([]byte, 4*side*side)
		for i := range side * side {
			part[4*i], part[4*i+1], part[4*i+3] = byte(k), byte(i), 255
		}
		tex.WritePixels(k*side, 0, side, side, part)
	}
	got := make([]byte, 4*side*n*side)
	tex.ReadPixels(got)
	for k := range n {
		for i := range side * side {
			x, y := k*side+i%side, i/side
			if p := pixel(got, side*n, x, y); p != [4]byte{byte(k), byte(i), 0, 255} {
				t.Fatalf("pixel (%d, %d) of write %d is %v", x, y, k, p)
			}
		}
	}
}

// wider than a draw puts at once, and more than drawing puts at all
func TestWritePixels_WideAndBigPartsLandWhole(t *testing.T) {
	needGPU(t)
	for _, c := range []struct{ tw, th, x, y, w, h int }{
		{1500, 3, 0, 1, 1500, 1},
		{400, 400, 10, 10, 300, 300},
	} {
		tex := NewTexture(c.tw, c.th)
		part := make([]byte, 4*c.w*c.h)
		for i := range c.w * c.h {
			part[4*i], part[4*i+1], part[4*i+3] = byte(i), byte(i>>8), 255
		}
		tex.WritePixels(c.x, c.y, c.w, c.h, part)
		got := make([]byte, 4*c.tw*c.th)
		tex.ReadPixels(got)
		for y := range c.th {
			for x := range c.tw {
				want := [4]byte{}
				if x >= c.x && x < c.x+c.w && y >= c.y && y < c.y+c.h {
					i := (y-c.y)*c.w + x - c.x
					want = [4]byte{byte(i), byte(i >> 8), 0, 255}
				}
				if p := pixel(got, c.tw, x, y); p != want {
					t.Fatalf("%dx%d at (%d, %d): pixel (%d, %d) is %v, want %v", c.w, c.h, c.x, c.y,
						x, y, p, want)
				}
			}
		}
	}
}

func TestNewTexture_IsTransparent(t *testing.T) {
	needGPU(t)
	used := NewTexture(64, 64)
	Fill(Image{Texture: used}, 0.3, 0.6, 0.9, 1)
	used.Release()
	tex := NewTexture(64, 64)
	tex.WritePixels(10, 10, 4, 4, make([]byte, 4*4*4))
	got := make([]byte, 4*64*64)
	tex.ReadPixels(got)
	for i, b := range got {
		if b != 0 {
			t.Fatalf("byte %d of a new texture is %d, want transparent", i, b)
		}
	}
}

// a 4 by 4 quad over the whole target at depth z, in c
func depthQuad(dst *Texture, dp *Depth, test DepthTest, z float32, c [4]float32) {
	v := func(x, y float32) Vertex {
		return Vertex{
			DstX:   x,
			DstY:   y,
			DstZ:   z,
			ColorR: c[0],
			ColorG: c[1],
			ColorB: c[2],
			ColorA: c[3],
		}
	}
	Triangles(
		&Draw{
			Target:  Image{Texture: dst},
			Program: fillProgram,
			Blend:   Copy,
			Depth:   dp,
			Test:    test,
		},
		[]Vertex{v(0, 0), v(4, 0), v(0, 4), v(4, 4)},
		[]uint16{0, 1, 2, 1, 2, 3},
	)
}

var (
	red   = [4]float32{1, 0, 0, 1}
	green = [4]float32{0, 1, 0, 1}
	blue  = [4]float32{0, 0, 1, 1}
)

func TestTriangles_TheNearerCoversTheFurtherAndAtOneDepthTheLaterWins(t *testing.T) {
	needGPU(t)
	dst := NewTexture(4, 4)
	dp := NewDepth()
	pix := make([]byte, 64)
	for _, tc := range []struct {
		name   string
		draws  []func()
		want   [4]byte
		reason string
	}{
		{"near then far", []func(){
			func() { depthQuad(dst, dp, Written, 0.8, red) },
			func() { depthQuad(dst, dp, Written, 0.2, blue) },
		}, [4]byte{255, 0, 0, 255}, "the far blue drawn later stays behind"},
		{"far then near", []func(){
			func() { depthQuad(dst, dp, Written, 0.2, blue) },
			func() { depthQuad(dst, dp, Written, 0.8, green) },
		}, [4]byte{0, 255, 0, 255}, "the near green drawn later covers"},
		{"one depth", []func(){
			func() { depthQuad(dst, dp, Written, 0.5, red) },
			func() { depthQuad(dst, dp, Tested, 0.5, blue) },
		}, [4]byte{0, 0, 255, 255}, "at one depth the later wins"},
		{"untested", []func(){
			func() { depthQuad(dst, dp, Written, 0.9, red) },
			func() { depthQuad(dst, dp, Untested, 0.1, green) },
		}, [4]byte{0, 255, 0, 255}, "an untested draw covers whatever the depth"},
	} {
		ClearDepth(Image{Texture: dst}, dp)
		for _, d := range tc.draws {
			d()
		}
		dst.ReadPixels(pix)
		if p := pixel(pix, 4, 1, 2); p != tc.want {
			t.Errorf("%s: %v, want %v: %s", tc.name, p, tc.want, tc.reason)
		}
	}
}

func TestCopyDepth_AReaderOfTheCopyIsHiddenWhereSomethingNearerCameAfter(t *testing.T) {
	needGPU(t)
	reader := NewMesh("test reader", "", `
struct Out { @builtin(position) clip: vec4<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 4>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(1.0, -1.0),
        vec2<f32>(-1.0, 1.0),
        vec2<f32>(1.0, 1.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    return o;
}
struct Laid { @location(0) color: vec4<f32>, @builtin(frag_depth) depth: f32 }
@fragment
fn fs_main(o: Out) -> Laid { return Laid(vec4<f32>(0.0, 0.0, 1.0, 1.0), depthAt(o.clip.xy)); }`)
	dst := NewTexture(4, 4)
	main, ground := NewDepth().Readable(), NewDepth().Readable()
	ClearDepth(Image{Texture: dst}, main)
	depthQuad(dst, main, Written, 0.3, red) // the ground
	CopyDepth(Image{Texture: dst}, main, ground)
	v := func(x, y float32) Vertex {
		return Vertex{DstX: x, DstY: y, DstZ: 0.7, ColorG: 1, ColorA: 1}
	}
	// a figure, left half
	Triangles(
		&Draw{
			Target:  Image{Texture: dst},
			Program: fillProgram,
			Blend:   Copy,
			Depth:   main,
			Test:    Written,
		},
		[]Vertex{v(0, 0), v(2, 0), v(0, 4), v(2, 4)},
		[]uint16{0, 1, 2, 1, 2, 3},
	)
	Mesh(&MeshDraw{Target: Image{Texture: dst}, Depth: main, ReadDepth: ground, Program: reader,
		Vertices: 4, Indices: NewIndices([]uint32{0, 1, 2, 1, 2, 3}), Blend: Copy})
	pix := make([]byte, 64)
	dst.ReadPixels(pix)
	if l, r := pixel(pix, 4, 0, 1), pixel(pix, 4, 3, 1); l != [4]byte{0, 255, 0, 255} ||
		r != [4]byte{0, 0, 255, 255} {
		t.Errorf("the left is %v and the right %v, want the figure green over the laid blue", l, r)
	}
}

func TestReadable_ADepthMadeReadableBeforeItsFirstDrawKeepsWhatItHolds(t *testing.T) {
	needGPU(t)
	dst := NewTexture(4, 4)
	held, copied := NewDepth().Readable(), NewDepth().Readable()
	ClearDepth(Image{Texture: dst}, held)
	depthQuad(dst, held, Written, 0.5, red)
	CopyDepth(Image{Texture: dst}, held, copied)
	depthQuad(dst, copied, Tested, 0.4, blue) // behind what was copied
	pix := make([]byte, 64)
	dst.ReadPixels(pix)
	if p := pixel(pix, 4, 2, 2); p != [4]byte{255, 0, 0, 255} {
		t.Errorf("behind the copied depth %v, want the red kept", p)
	}
}

func TestMesh_ADepthReadByALargerTargetKeepsItsSizeAndWhatItHolds(t *testing.T) {
	needGPU(t)
	reader := NewMesh("test half reader", "", `
struct Out { @builtin(position) clip: vec4<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 4>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(1.0, -1.0),
        vec2<f32>(-1.0, 1.0),
        vec2<f32>(1.0, 1.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    return o;
}
@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    return vec4<f32>(depthAt(o.clip.xy / 2.0), 0.0, 0.0, 1.0);
}`)
	small, big := NewTexture(4, 4), NewTexture(8, 8)
	held := NewDepth().Readable()
	ClearDepth(Image{Texture: small}, held)
	depthQuad(small, held, Written, 0.6, red)
	Mesh(&MeshDraw{Target: Image{Texture: big}, ReadDepth: held, Program: reader, Vertices: 4,
		Indices: NewIndices([]uint32{0, 1, 2, 1, 2, 3}), Blend: Copy})
	pix := make([]byte, 4*8*8)
	big.ReadPixels(pix)
	if p := pixel(pix, 8, 6, 5); p[0] < 152 || p[0] > 154 {
		t.Errorf("an 8 by 8 target reads %v of a 4 by 4 depth holding 0.6, want 0.6 in red", p)
	}
	if w, h := held.Size(); w != 4 || h != 4 {
		t.Errorf("read by an 8 by 8 target the depth is %d by %d, want 4 by 4 as drawn", w, h)
	}
}

func TestTriangles_ASightTakesHeightsForDepths(t *testing.T) {
	needGPU(t)
	// straight down from 100, a pixel a unit: the depth (z + 200) / 400, 1 nearest
	sight := &Sight{Origin: [3]float32{0, 0, 100}, DX: [3]float32{1, 0, 0}, DY: [3]float32{0, 1, 0},
		Dir: [3]float32{
			0,
			0,
			-1,
		}, Z: [4]float32{0, 0, 1.0 / 400, 200.0 / 400}, W: [4]float32{0, 0, 0, 1}}
	dst := NewTexture(4, 4)
	dp := NewDepth()
	at := func(z float32, test DepthTest, c [4]float32) {
		v := func(x, y float32) Vertex {
			return Vertex{
				DstX:   x,
				DstY:   y,
				DstZ:   z,
				ColorR: c[0],
				ColorG: c[1],
				ColorB: c[2],
				ColorA: c[3],
			}
		}
		Triangles(
			&Draw{
				Target:  Image{Texture: dst},
				Program: fillProgram,
				Blend:   Copy,
				Depth:   dp,
				Test:    test,
				Sight:   sight,
			},
			[]Vertex{v(0, 0), v(4, 0), v(0, 4), v(4, 4)},
			[]uint16{0, 1, 2, 1, 2, 3},
		)
	}
	pix := make([]byte, 64)
	ClearDepth(Image{Texture: dst}, dp)
	at(40, Written, red)
	at(0, Tested, blue)
	dst.ReadPixels(pix)
	if p := pixel(pix, 4, 1, 1); p != [4]byte{255, 0, 0, 255} {
		t.Errorf("the ground drawn after a roof 40 up %v, want the roof red", p)
	}
	ClearDepth(Image{Texture: dst}, dp)
	at(0, Written, blue)
	at(40, Tested, green)
	dst.ReadPixels(pix)
	if p := pixel(pix, 4, 1, 1); p != [4]byte{0, 255, 0, 255} {
		t.Errorf("a roof 40 up drawn after the ground %v, want it green", p)
	}
}

func TestFillBehind_FillsOnlyWhereNothingWasDrawn(t *testing.T) {
	needGPU(t)
	dst := NewTexture(4, 4)
	dp := NewDepth()
	ClearDepth(Image{Texture: dst}, dp)
	v := func(x, y float32) Vertex {
		return Vertex{DstX: x, DstY: y, DstZ: 0.3, ColorR: 1, ColorA: 1}
	}
	Triangles(
		&Draw{
			Target:  Image{Texture: dst},
			Program: fillProgram,
			Blend:   Copy,
			Depth:   dp,
			Test:    Written,
		},
		[]Vertex{v(0, 0), v(2, 0), v(0, 4), v(2, 4)},
		[]uint16{0, 1, 2, 1, 2, 3},
	)
	FillBehind(Image{Texture: dst}, dp, 0, 0, 1, 1)
	pix := make([]byte, 64)
	dst.ReadPixels(pix)
	if l, r := pixel(pix, 4, 1, 1), pixel(pix, 4, 2, 1); l != [4]byte{255, 0, 0, 255} ||
		r != [4]byte{0, 0, 255, 255} {
		t.Errorf("the drawn left is %v and the empty right %v, want red kept and blue filled", l, r)
	}
}

func TestMesh_AProgramReadsTheFramesImagesByName(t *testing.T) {
	needGPU(t)
	layout := NewLayout([]Uniform{{"At", 1}})
	p := NewMesh("test frame", layout.Fields, `
@group(3) @binding(2) var Probe: texture_2d<f32>;
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
    return textureSampleLevel(Probe, wrapped, vec2<f32>(U.At, 0.5), 0.0);
}`)
	// red on the left, green on the right
	img := NewTexture(2, 1)
	img.WritePixels(0, 0, 2, 1, []byte{255, 0, 0, 255, 0, 255, 0, 255})
	blue := NewTexture(1, 1)
	blue.WritePixels(0, 0, 1, 1, []byte{0, 0, 255, 255})
	dst := NewTexture(2, 2)
	pix := make([]byte, 16)
	draw := func(at float32, frame *Texture) [4]byte {
		u := make([]byte, layout.Size)
		layout.Put(u, "At", []float32{at})
		dw := &MeshDraw{Target: Image{Texture: dst}, Program: p, Uniforms: u, Vertices: 3,
			Blend: Copy}
		dw.Frame[2] = frame
		Mesh(dw)
		dst.ReadPixels(pix)
		return pixel(pix, 2, 1, 1)
	}
	if c := draw(0.25, img); c != [4]byte{255, 0, 0, 255} {
		t.Errorf("the frame's image read on its left is %v, want red", c)
	}
	if c := draw(1.75, img); c != [4]byte{0, 255, 0, 255} {
		t.Errorf("the frame's image read past its right edge is %v, want it repeated: green", c)
	}
	if c := draw(0.25, nil); c != [4]byte{} {
		t.Errorf("a frame without the image reads %v, want nothing", c)
	}
	groups := len(cur.frames)
	if c := draw(0.25, blue); c != [4]byte{0, 0, 255, 255} {
		t.Errorf("another frame's image reads %v, want blue", c)
	}
	draw(0.75, img)
	draw(0.75, blue)
	if n := len(cur.frames); n != groups+1 {
		t.Errorf("two sets of the frame's images drawn again made %d groups, want the one new",
			n-groups)
	}
}

// the pass calls themselves allocate in gogpu's Vulkan HAL (every vkCmd* boxes its arguments
// for the FFI), so what is measured is the device's own work a frame: the pass's descriptor,
// each draw's vertices, indices, block and offsets
func TestDevice_ARedrawnFrameAllocatesNothingOfItsOwn(t *testing.T) {
	needGPU(t)
	dst := NewTexture(64, 64)
	src := NewTexture(4, 4)
	src.WritePixels(0, 0, 4, 4, make([]byte, 64))
	dp := NewDepth()
	verts := []Vertex{
		{DstX: 0, DstY: 0}, {DstX: 16, DstY: 0}, {DstX: 0, DstY: 16}, {DstX: 16, DstY: 16},
	}
	indices := []uint16{0, 1, 2, 1, 2, 3}
	dw := &Draw{Target: Image{Texture: dst}, Program: blitProgram, Images: [4]Image{{Texture: src}},
		Uniforms: make([]byte, blitLayout.Size), Depth: dp, Test: Written}
	for range 8 {
		Triangles(dw, verts, indices)
	}
	Submit()
	d := cur
	d.mu.Lock()
	defer d.mu.Unlock()
	target := dst.gpuTarget(d)
	frame := func() {
		d.passDescriptor(target, dp, false)
		for range 8 {
			d.gather(verts, indices)
			d.offsets(d.drawBlock(target, dw.Images, dw.Uniforms, dw.Place, dw.Tint, dw.Sight))
		}
		d.verts, d.indices, d.draws = d.verts[:0], d.indices[:0], d.draws[:0]
		d.uniforms, d.offs = d.uniforms[:0], d.offs[:0]
	}
	if n := testing.AllocsPerRun(10, frame); n != 0 {
		t.Errorf("a frame redrawn allocates %v times of its own, want none", n)
	}
}
