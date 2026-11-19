package gpu

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/gputypes"
)

// one pixel drawn by a fragment shader over the package's vertex stage, with U.Shade = shade
func fragment(t *testing.T, source string, shade float32) [4]byte {
	t.Helper()
	needGPU(t)
	layout := NewLayout([]Uniform{{"Shade", 1}})
	p := NewProgram("compile", layout.Fields, source)
	dst := NewTexture(1, 1)
	u := make([]byte, layout.Size)
	layout.Put(u, "Shade", []float32{shade})
	v := func(x, y float32) Vertex { return Vertex{DstX: x, DstY: y} }
	Triangles(&Draw{Target: Image{Texture: dst}, Program: p, Uniforms: u, Blend: Copy},
		[]Vertex{v(0, 0), v(1, 0), v(0, 1), v(1, 1)}, []uint16{0, 1, 2, 1, 2, 3})
	pix := make([]byte, 4)
	dst.ReadPixels(pix)
	return pixel(pix, 1, 0, 0)
}

func near(got [4]byte, want [4]byte) bool {
	for i := range got {
		if d := int(got[i]) - int(want[i]); d < -1 || d > 1 {
			return false
		}
	}
	return true
}

// naga v0.19 read a module-scope vector const as zero where it met a runtime value
func TestCompile_AModuleScopeVectorConstMeetsARuntimeValue(t *testing.T) {
	got := fragment(t, `
const TINT = vec3<f32>(0.25, 0.5, 0.75);

fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    return vec4<f32>(TINT * U.Shade, 1.0);
}`, 1)
	if want := [4]byte{64, 128, 191, 255}; !near(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// naga v0.19 miscompiled += on an element of a local array
func TestCompile_AnElementOfALocalArrayAddsInPlace(t *testing.T) {
	got := fragment(t, `
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    var counts = array<u32, 4>(0u, 0u, 0u, 0u);
    for (var i = 0u; i < 6u; i++) {
        let k = (i + u32(U.Shade)) % 3u;
        counts[k] += 1u;
    }
    return vec4<f32>(f32(counts[0]) * 0.25, f32(counts[1]) * 0.25, f32(counts[3]), 1.0);
}`, 0)
	if want := [4]byte{128, 128, 0, 255}; !near(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// naga v0.19 had no all() and any() in its SPIR-V writer: a panic at the first draw
func TestCompile_AllAndAnyTestAVector(t *testing.T) {
	got := fragment(t, `
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let v = vec3<f32>(U.Shade, 2.0, 3.0);
    return vec4<f32>(f32(all(v > vec3<f32>(0.0))), f32(any(v > vec3<f32>(2.5))),
        f32(any(v < vec3<f32>(0.0))), 1.0);
}`, 1)
	if want := [4]byte{255, 255, 0, 255}; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Metal, DX12 and GL take no SPIR-V: told one of them, the device hands wgpu the WGSL
func TestCompile_ABackendOtherThanVulkanIsHandedTheWGSL(t *testing.T) {
	needGPU(t)
	was := cur.backend
	cur.backend = gputypes.BackendMetal
	t.Cleanup(func() { cur.backend = was })
	got := fragment(t, `
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    return vec4<f32>(0.0, U.Shade, 0.0, 1.0);
}`, 1)
	if want := [4]byte{0, 255, 0, 255}; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// the fragment lies further than its triangle and says so: what comes after between the two
// covers it, on Vulkan, which takes the word, and on a backend handed the program without it
func TestMesh_AFragmentLyingNoNearerThanItsTriangleSaysSoOnEveryBackend(t *testing.T) {
	needGPU(t)
	for name, backend := range map[string]gputypes.Backend{
		"vulkan": gputypes.BackendVulkan,
		"metal":  gputypes.BackendMetal,
	} {
		was := cur.backend
		cur.backend = backend
		layout := NewLayout([]Uniform{{"Depth", 1}, {"Lies", 1}, {"Color", 4}})
		p := NewMesh("early depth "+name, layout.Fields, `
struct Out { @builtin(position) clip: vec4<f32> }
struct Lain {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}
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
@early_depth_test(less_equal)
fn fs_main(o: Out) -> Lain { return Lain(U.Color, min(U.Lies, o.clip.z)); }`)
		dst := NewTexture(4, 4)
		depth := NewDepth()
		quad := NewIndices([]uint32{0, 1, 2, 1, 2, 3})
		draw := func(d, lies float32, c [4]float32, clear bool) {
			u := make([]byte, layout.Size)
			layout.Put(u, "Depth", []float32{d})
			layout.Put(u, "Lies", []float32{lies})
			layout.Put(u, "Color", c[:])
			Mesh(&MeshDraw{Target: Image{Texture: dst}, Depth: depth, ClearDepth: clear,
				Write: true, Program: p, Uniforms: u, Indices: quad, Blend: Copy})
		}
		draw(0.8, 0.3, [4]float32{1, 0, 0, 1}, true)  // a red triangle near, lying far
		draw(0.5, 0.5, [4]float32{0, 0, 1, 1}, false) // blue between the two
		draw(0.9, 0.4, [4]float32{0, 1, 0, 1}, false) // a green triangle nearest, lying behind
		pix := make([]byte, 64)
		dst.ReadPixels(pix)
		if p := pixel(pix, 4, 1, 1); p != [4]byte{0, 0, 255, 255} {
			t.Errorf("%s: after red lying at 0.3, blue at 0.5 and green lying at 0.4 the "+
				"pixel is %v, want blue", name, p)
		}
		cur.backend = was
	}
}

func TestProgram_ABackendOtherThanVulkanTakesItWithoutTheWordOnItsDepth(t *testing.T) {
	const source = "@fragment\n@early_depth_test(less_equal)\nfn fs_main() {}\n"
	const want = "@fragment\nfn fs_main() {}\n"
	if got := earlyDepth.ReplaceAllString(source, ""); got != want {
		t.Errorf("the program is handed over as %q, want %q", got, want)
	}
}

func TestCompileAll_CompilesEveryProgramAtOnceAndNamesTheOnesThatFail(t *testing.T) {
	needGPU(t)
	const n = 12
	progs := make([]*Program, n)
	for i := range progs {
		progs[i] = NewProgram(fmt.Sprintf("many%d", i), "", fmt.Sprintf(`
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    return vec4<f32>(%d.0 / 255.0, 0.0, 0.0, 1.0);
}`, 10*i))
	}
	if err := CompileAll(progs); err != nil {
		t.Fatal(err)
	}
	v := func(x, y float32) Vertex { return Vertex{DstX: x, DstY: y} }
	for i, p := range progs {
		dst := NewTexture(1, 1)
		Triangles(&Draw{Target: Image{Texture: dst}, Program: p, Blend: Copy},
			[]Vertex{v(0, 0), v(1, 0), v(0, 1), v(1, 1)}, []uint16{0, 1, 2, 1, 2, 3})
		pix := make([]byte, 4)
		dst.ReadPixels(pix)
		if got := pixel(pix, 1, 0, 0); got != [4]byte{byte(10 * i), 0, 0, 255} {
			t.Errorf("program %d drew %v", i, got)
		}
	}
	bad := []*Program{progs[0], NewProgram("broken", "", "fn Fragment( -> {")}
	err := CompileAll(bad)
	if err == nil || !strings.Contains(err.Error(), "the broken program") {
		t.Errorf("err = %v, want the broken program named", err)
	}
}

// naga v0.19 miscompiled a function returning a struct in a search for an edge
func TestCompile_AFunctionReturnsAStructToASearch(t *testing.T) {
	got := fragment(t, `
struct Hit {
    at: f32,
    found: bool,
}

fn probe(i: u32, limit: f32) -> Hit {
    let x = f32(i) * 0.1;
    return Hit(x, x > limit);
}

fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    var at = 0.0;
    for (var i = 0u; i < 10u; i++) {
        let h = probe(i, U.Shade);
        if (h.found) {
            at = h.at;
            break;
        }
    }
    return vec4<f32>(at, 0.0, 0.0, 1.0);
}`, 0.45)
	if want := [4]byte{128, 0, 0, 255}; !near(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
