package gpu

import (
	"fmt"
	"strings"
	"testing"
)

const lainMesh = `
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
    o.clip = vec4<f32>(corners[vid], 0.5, 1.0);
    return o;
}
@fragment
fn fs_main(o: Out) -> Lain { return Lain(U.Color, 0.0); }`

func TestNewMesh_KnowsWhetherItsFragmentWritesADepth(t *testing.T) {
	lain := NewMesh("lain", "", `struct Lain { @builtin( frag_depth ) depth: f32 }`)
	flat := NewMesh("flat", "", `fn fs_main() -> @location(0) vec4<f32> { return vec4(1.0); }`)
	if !lain.writesDepth || flat.writesDepth {
		t.Errorf("a fragment writing a depth %v, one writing none %v; want true, false",
			lain.writesDepth, flat.writesDepth)
	}
}

// a pipeline without a depth state for such a fragment is what Metal refuses to make
func TestMesh_AFragmentWritingADepthIsDrawnWithNoBufferAsThoughThereWereNone(t *testing.T) {
	needGPU(t)
	layout := NewLayout([]Uniform{{"Color", 4}})
	prog := NewMesh("lain", layout.Fields, lainMesh)
	quad := NewIndices([]uint32{0, 1, 2, 1, 2, 3})
	draw := func(dst *Texture, c [4]float32) {
		u := make([]byte, layout.Size)
		layout.Put(u, "Color", c[:])
		Mesh(&MeshDraw{Target: Image{Texture: dst}, Program: prog, Uniforms: u, Indices: quad,
			Blend: Copy})
	}
	dst := NewTexture(4, 4)
	draw(dst, [4]float32{1, 0, 0, 1})
	// the farthest depth of all, drawn over the red all the same
	draw(dst, [4]float32{0, 1, 0, 1})
	pix := make([]byte, 64)
	dst.ReadPixels(pix)
	if p := pixel(pix, 4, 1, 1); p != [4]byte{0, 255, 0, 255} {
		t.Errorf("the second draw %v, want green over the red", p)
	}
	for key := range prog.pipelines {
		if key.depth != depthIgnore {
			t.Errorf("a pipeline of depth mode %d, want the stand-in's alone, untested", key.depth)
		}
	}
	for n := 1; n <= standIns+1; n++ {
		draw(NewTexture(n, 2), [4]float32{0, 0, 1, 1})
	}
	cur.mu.Lock()
	defer cur.mu.Unlock()
	if len(cur.standIns) > standIns {
		t.Errorf("%d stand-ins kept, want %d at most", len(cur.standIns), standIns)
	}
	if got := cur.standIns[0]; got.w != standIns+1 || got.h != 2 {
		t.Errorf("the latest stand-in %dx%d, want %dx2", got.w, got.h, standIns+1)
	}
}

func TestProgram_MakesNoPipelineWithoutADepthForAFragmentWritingOne(t *testing.T) {
	defer func() {
		if r := recover(); !strings.Contains(fmt.Sprint(r), "writes a depth") {
			t.Errorf("recovered %v, want the program refused", r)
		}
	}()
	NewMesh("lain", "", lainMesh).pipeline(nil, Copy, 0, noDepth)
}
