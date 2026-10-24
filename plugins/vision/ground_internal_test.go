package vision

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type flatAt float64

func (l flatAt) At(geom.Vec) float64 { return float64(l) }
func (flatAt) Step() float64         { return 1 }

type installs struct{ plugin.Installer }

func (installs) UseModule(goke.Module)  {}
func (installs) Hosts(...host.Host)     {}
func (installs) Draws(...render.Source) {}
func (installs) Pool() *parallel.Pool   { return nil }

func TestPlugin_SightScansTheGroundAsItStandsTheViewsAreDrawnOverTheFrames(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 64, Height: 64},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 8}, Heights: true})
	p := NewPlugin(w).WithHeights(func() ground.Heights { return flatAt(5) })
	p.drawnHeights = func() ground.Heights { return flatAt(1) }
	if err := p.Install(installs{}); err != nil {
		t.Fatal(err)
	}
	at := geom.NewVec(10, 10)
	if got := p.module.sys.groundOf().At(at); got != 5 {
		t.Errorf("sight scans a ground %v high, want the ground as it stands, 5", got)
	}
	if got := p.renderer.groundOf().At(at); got != 1 {
		t.Errorf("the views are drawn over a ground %v high, want the frame's, 1", got)
	}
}
