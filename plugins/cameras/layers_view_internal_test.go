package cameras

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func layersPlugin() *Plugin {
	return NewPlugin(world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 800},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}), Config{})
}

func TestLayers_OutlineEveryInnerLayerInsetByTheFeatherWhateverTheCrop(t *testing.T) {
	p := layersPlugin()
	cam := cameratest.NewFromSpace(1000, 800, 0)
	var f render.Frame
	pieces := func() int {
		f.Reset(cam)
		p.layersView.Compose(&f, cam)
		return f.Len()
	}
	p.write.ShowLayers(Tactical, true)
	if n := pieces(); n != 0 {
		t.Errorf("a camera drawn whole got %d pieces of the layers, want none", n)
	}
	layers := []camera.Layer{{Share: 1, Resolution: 0.5}, {Share: 0.5, Resolution: 1}}
	cam.(camera.Layered).SetLayers(layers, camera.Inset)
	p.write.ShowLayers(Tactical, false)
	if n := pieces(); n != 0 {
		t.Errorf("hidden, the layers drew %d pieces", n)
	}
	p.write.ShowLayers(Tactical, true)
	if n := pieces(); n != 4 {
		t.Fatalf("a camera drawn in two layers got %d pieces, want the inner one's four sides", n)
	}
	// the inner layer's window is (250, 200), 500 by 400; each side the feather in
	const in = render.LayerFeather
	onTheInset := func(x, y, k float32) bool {
		x0, y0, x1, y1 := (250+in)*k, (200+in)*k, (750-in)*k, (600-in)*k
		// a line's corners lie half its width and its fade out from the side
		near := func(a, b float32) bool { return a >= b-3*k && a <= b+3*k }
		return (near(x, x0) || near(x, x1)) && y >= y0-3*k && y <= y1+3*k ||
			(near(y, y0) || near(y, y1)) && x >= x0-3*k && x <= x1+3*k
	}
	f.Each(func(at render.At, verts []render.Vertex) {
		if at != render.OnTop(layersLevel) {
			t.Errorf("a side lies at %v, want a mark over the zones", at)
		}
		for _, v := range verts {
			if !onTheInset(v.DstX, v.DstY, 1) {
				t.Errorf("a side's corner at (%v, %v), want on the inset rectangle", v.DstX, v.DstY)
			}
		}
	})
	// composed for the inner layer's own pass, the outline lies in that crop's pixels
	c := cam.(camera.Cropping)
	c.Crop(camera.Window{X: 250, Y: 200, W: 500, H: 400}, 0.5)
	defer c.Uncrop()
	if n := pieces(); n != 4 {
		t.Fatalf("cropped to the inner layer the outline has %d pieces, want four", n)
	}
	f.Each(func(_ render.At, verts []render.Vertex) {
		for _, v := range verts {
			if x, y := v.DstX/0.5+250, v.DstY/0.5+200; !onTheInset(x, y, 1) {
				t.Errorf("cropped, a side's corner at (%v, %v) of the crop, want the inset "+
					"rectangle in the crop's pixels", v.DstX, v.DstY)
			}
		}
	})
}

func TestLayers_OutlineOfAFootedLayerHasNoSideOnTheBottomEdge(t *testing.T) {
	p := layersPlugin()
	p.write.ShowLayers(Tactical, true)
	cam := cameratest.NewFromSpace(1000, 800, 0)
	cam.(camera.Layered).SetLayers([]camera.Layer{{Share: 1, Resolution: 0.5},
		{Share: 0.5, Resolution: 1}}, camera.Footed)
	var f render.Frame
	f.Reset(cam)
	p.layersView.Compose(&f, cam)
	if n := f.Len(); n != 3 {
		t.Fatalf("a layer on the bottom edge got %d pieces, want three sides: the top and the legs",
			n)
	}
	// the inner layer's window is (250, 400), 500 by 400, flush with the bottom
	const in = render.LayerFeather
	var lowest float32
	f.Each(func(_ render.At, verts []render.Vertex) {
		top := float32(800)
		for _, v := range verts {
			if v.DstX < 250+in-2 || v.DstX > 750-in+2 || v.DstY < 400+in-2 || v.DstY > 800 {
				t.Errorf("a side's corner at (%v, %v), want the feather in but at the bottom",
					v.DstX, v.DstY)
			}
			top, lowest = min(top, v.DstY), max(lowest, v.DstY)
		}
		if top > 800-in-2 {
			t.Errorf("a side lies along the bottom, from y %v, want none there", top)
		}
	})
	if lowest != 800 {
		t.Errorf("the legs reach down to y %v, want 800: the bottom edge", lowest)
	}
}
