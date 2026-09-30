package gpu

import (
	"os"
	"testing"
)

// needDevice readies the headless device, the software rasteriser with GRAM_GPU=software; a machine
// without either skips.
func needDevice(t *testing.T) {
	t.Helper()
	if err := Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU to draw on: %v", err)
	}
}

func pixel(pix []byte, w, x, y int) [4]byte {
	i := 4 * (y*w + x)
	return [4]byte{pix[i], pix[i+1], pix[i+2], pix[i+3]}
}

// A fill covers the rectangle asked for, and only it; what is written and drawn reads back.
func TestFill_CoversItsRectangleOnly(t *testing.T) {
	needDevice(t)
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

// Pixels written land after the draws before them; a program reads an image at its pixels, and
// its uniforms by name.
func TestTriangles_AProgramReadsItsImageAndUniforms(t *testing.T) {
	needDevice(t)
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

// An image drawn over another lays its premultiplied colours over what is there; outside the
// image a program reads transparent.
func TestDrawImage_LaysItOverWhatIsThere(t *testing.T) {
	needDevice(t)
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

// Uniforms lie as WGSL lays a struct out: a vec3 on 16 bytes, a float right after it.
func TestLayout_FollowsWGSL(t *testing.T) {
	l := NewLayout([]Uniform{{"A", 1}, {"B", 3}, {"C", 1}, {"D", 2}})
	if l.offsets["A"] != 0 || l.offsets["B"] != 16 || l.offsets["C"] != 28 || l.offsets["D"] != 32 || l.Size != 48 {
		t.Errorf("offsets %v size %d, want A 0, B 16, C 28, D 32, 48 in all", l.offsets, l.Size)
	}
}

// Pixels written into a part of a texture land there, rows of any width, and leave the rest as it
// was — gogpu's own write at an origin spoils what lies left of it.
func TestWritePixels_APartLandsInPlaceAndLeavesTheRest(t *testing.T) {
	needDevice(t)
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

// A mesh reads its vertices from their index alone; drawn with a depth buffer, the nearer of two
// covers the further whichever comes first.
func TestMesh_TheNearerCoversTheFurtherWhicheverComesFirst(t *testing.T) {
	needDevice(t)
	layout := NewLayout([]Uniform{{"Depth", 1}, {"Color", 4}})
	p := NewMesh("test mesh", layout.Fields, `
struct Out { @builtin(position) clip: vec4<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 4>(vec2<f32>(-1.0, -1.0), vec2<f32>(1.0, -1.0), vec2<f32>(-1.0, 1.0), vec2<f32>(1.0, 1.0));
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

// An instanced mesh is drawn once an instance, each reading its own vectors: two quads, from their
// vertices alone, each where and in the colour its instance says.
func TestMesh_AnInstancedMeshIsDrawnOnceAnInstance(t *testing.T) {
	needDevice(t)
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

// A new texture is transparent, as WebGPU has it, though its memory was another texture's: a part
// drawn into it leaves the rest clear.
func TestNewTexture_IsTransparent(t *testing.T) {
	needDevice(t)
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
