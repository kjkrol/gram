package compose

import (
	"image/color"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/wgsl"
)

// registered before any shader is built, as a package's init would
var _ = func() bool { wgsl.RegisterImage("TestProbe"); return true }()

type probing struct {
	img  *Image
	seen any
}

func (*probing) Init(*goke.SysInit) {}
func (p *probing) Compose(f *Frame, _ camera.Camera) {
	if p.img != nil {
		f.Image("TestProbe", p.img)
	}
	f.DrawAt(Furthest)
}
func (p *probing) Draw(t Target, _ camera.Camera, u Uniforms) {
	p.seen = u.m["TestProbe"]
	if t.Screen == nil {
		return
	}
	opts := gfx.DrawMeshOptions{Uniforms: map[string]any{}, Vertices: 3}
	u.Into(opts.Uniforms)
	t.Screen.DrawMesh(nil, probeShader, &opts)
}

var probeShader = gfx.NewMeshShaderWith("probe", []byte(`
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
    return textureSampleLevel(TestProbe, wrapped, vec2<f32>(0.5, 0.5), 0.0);
}
`), nil)

func TestFrame_HandsADirectItsImageAsAUniformAndNothingOnceItIsNotSet(t *testing.T) {
	img := gfx.NewImage(1, 1)
	p := &probing{img: img}
	c := NewComposer(p)
	c.draw = func(*Image, []Vertex, []uint16, *Image, [4]float32) {}
	c.compose(topDown(), wholePart)
	c.render(nil, false, 0)
	at := slices.Index(wgsl.Images(), "TestProbe")
	if p.seen != img || c.frameTex[at] != gfx.GPU(img).Texture {
		t.Errorf("a Direct is handed %v and the composer binds %v, want the frame's image", p.seen,
			c.frameTex[at])
	}
	p.img = nil
	c.compose(topDown(), wholePart)
	c.render(nil, false, 0)
	if seen, _ := p.seen.(*Image); seen != nil || c.frameTex[at] != nil {
		t.Errorf("a frame that set no image hands %v and binds %v, want none", p.seen,
			c.frameTex[at])
	}
}

func TestFrame_AShaderReadsTheFramesImageByName(t *testing.T) {
	gputest.Need(t)
	red := gfx.NewImage(1, 1)
	red.Fill(color.RGBA{R: 255, A: 255})
	screen := gfx.NewImage(4, 4)
	NewComposer(&probing{img: red}).DrawWorld(screen, topDown())
	pix := make([]byte, 4*16)
	screen.ReadPixels(pix)
	if got := [4]byte(pix[4*5 : 4*5+4]); got != [4]byte{255, 0, 0, 255} {
		t.Errorf("a shader reading the frame's image drew %v, want its red", got)
	}
	if !strings.Contains(ShaderSource(), "var TestProbe: texture_2d<f32>;") {
		t.Error("the library declares no TestProbe")
	}
}

func TestRegisterImage_RefusesALateNameAndAFrameAnUnregisteredOne(t *testing.T) {
	panics := func(what string, f func()) {
		t.Helper()
		defer func() {
			t.Helper()
			if recover() == nil {
				t.Errorf("%s did not panic", what)
			}
		}()
		f()
	}
	ShaderSource()
	panics("an image registered after a shader was built", func() { wgsl.RegisterImage("Late") })
	panics("a frame handed an image never registered", func() {
		var f Frame
		f.Image("Unknown", nil)
	})
}
