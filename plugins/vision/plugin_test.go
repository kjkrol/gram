package vision_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func testWorldPlugin() *world.Plugin {
	return world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 2000, Height: 2000},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 100},
	})
}

func TestPlugin_Contract(t *testing.T) {
	p := vision.NewPlugin(testWorldPlugin())

	if p.Name() != "gram.vision" {
		t.Errorf("Name = %q", p.Name())
	}
	if p.EventHandler() != nil {
		t.Error("EventHandler is not nil — vision reads no input")
	}
	if p.Serializable() != nil {
		t.Error("Serializable is not nil — vision's state lives in components")
	}
	if p.Renderer() != nil {
		t.Error("Renderer is not nil before WithRenderer")
	}

	p.WithRenderer(nil)
	if p.Renderer() == nil {
		t.Error("Renderer is nil after WithRenderer")
	}
}

// A style set before Use has to survive into the renderer WithRenderer builds.
func TestPlugin_WithStyleReachesTheRenderer(t *testing.T) {
	used := false
	style := vision.ConeStyleFn(func(*render.Frame, []vision.ConePoint) { used = true })

	p := vision.NewPlugin(testWorldPlugin()).WithStyle(style)
	p.WithRenderer(nil)

	r, ok := p.Renderer().(*vision.Renderer)
	if !ok {
		t.Fatalf("Renderer is %T, want *vision.Renderer", p.Renderer())
	}
	r.Style().Compose(nil, nil)
	if !used {
		t.Error("the renderer did not take the style handed to WithStyle")
	}
}
