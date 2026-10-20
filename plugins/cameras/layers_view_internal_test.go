package cameras

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestLayers_OutlineLiesOnACroppedPassInsetByTheFeather(t *testing.T) {
	p := NewPlugin(world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 800},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}), Config{})
	whole := cameratest.NewFromSpace(1000, 800, 0)
	part := cropped{whole}
	var f render.Frame
	pieces := func(cam camera.Camera) int {
		f.Reset(cam)
		p.layersView.Compose(&f, cam)
		return f.Len()
	}
	if n := pieces(part); n != 0 {
		t.Errorf("hidden, the layers drew %d pieces", n)
	}
	p.ShowLayers(true)
	if n := pieces(whole); n != 0 {
		t.Errorf("a camera drawn whole got %d pieces of the layers, want none", n)
	}
	if n := pieces(part); n != 4 {
		t.Fatalf("a camera drawn a part of its frame got %d pieces, want the four sides", n)
	}
	const in = render.LayerFeather
	f.Each(func(at render.At, verts []render.Vertex) {
		if at != render.OnTop(layersLevel) {
			t.Errorf("a side lies at %v, want a mark over the zones", at)
		}
		for _, v := range verts {
			if v.DstX < in-2 || v.DstX > 1000-in+2 || v.DstY < in-2 || v.DstY > 800-in+2 {
				t.Errorf("a side's corner at (%v, %v), want within the feather, %d pixels in",
					v.DstX, v.DstY, in)
			}
			if v.DstX > in+2 && v.DstX < 1000-in-2 && v.DstY > in+2 && v.DstY < 800-in-2 {
				t.Errorf("a side's corner at (%v, %v), want on the inset rectangle", v.DstX,
					v.DstY)
			}
		}
	})
}
