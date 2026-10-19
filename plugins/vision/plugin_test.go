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
	if p.Serializable() == nil {
		t.Error("Serializable is nil — vision keeps its Settings")
	}
	rendererOf(t, nil, p)
}

func TestPlugin_WithStyleReachesTheRenderer(t *testing.T) {
	used := false
	style := vision.ConeStyleFn(func(*render.Frame, []vision.ConePoint) { used = true })

	p := vision.NewPlugin(testWorldPlugin()).WithStyle(style)

	rendererOf(t, nil, p).Style().Compose(nil, nil)
	if !used {
		t.Error("the renderer did not take the style handed to WithStyle")
	}
}
