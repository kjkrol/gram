package players

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
)

// glow is a handler with a switch and a range beside it
type glow struct {
	shades command.Queue[shade]
	on     bool
	level  float64
}

func (h *glow) Name() string                       { return "gram.lamps" }
func (h *glow) Queues() []command.CommandQueue     { return []command.CommandQueue{&h.shades} }
func (h *glow) DefaultBindings() []control.Binding { return nil }
func (h *glow) Settings() []control.Setting {
	return []control.Setting{
		control.Switch{
			Label: "Lamps",
			Get:   func() bool { return h.on },
			Set:   func(on bool) command.Command { return shade{On: on} },
		},
		control.Range{
			Label:  "Glow",
			Beside: "Lamps",
			Min:    0,
			Max:    1,
			Step:   0.1,
			Get:    func() float64 { return h.level },
			Set:    func(v float64) command.Command { return shade{On: v > 0} },
		},
	}
}

func TestSettings_ARangeBesideASwitchSharesItsRowAndLivesWhileItIsOn(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	h := &glow{}
	p := NewPlugin(w, cameras.NewPlugin(w, cameras.Config{}), h)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1920, ScreenHeight: 1080}}, &composed{}
	p.settings.Open(rt, comp)
	s := p.settings.screen
	s.Drawers()
	s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1920, 1080)))
	box, slider := s.Element("setting:Lamps/Lamps"), s.Element("setting:Lamps/Glow")
	if box == nil || slider == nil {
		t.Fatalf("the screen has the checkbox %v and the slider %v, want both", box != nil,
			slider != nil)
	}
	middle := func(b geom.AABB) float32 { return (b.TopLeft.Y + b.BottomRight.Y) / 2 }
	if bb, sb := box.Box(), slider.Box(); f32.Abs(middle(bb)-middle(sb)) > 1 ||
		sb.TopLeft.X < bb.BottomRight.X {
		t.Errorf("the checkbox lies at %v and the slider at %v, want one row, the slider right "+
			"of the checkbox", bb, sb)
	}
	at := slider.Box().TopLeft.Add(geom.NewVec(4, 4))
	if slider.Hits(at) {
		t.Error("the lamps off, the glow slider is hit, want it passed by")
	}
	h.on = true
	if !slider.Hits(at) {
		t.Error("the lamps on, the glow slider is not hit")
	}
}
