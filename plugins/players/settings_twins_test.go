package players

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
)

// stubTunable is a handler with the settings given, one of the demo's plugins
type stubTunable struct {
	name     string
	settings []control.Setting
}

func (h *stubTunable) Name() string                       { return h.name }
func (h *stubTunable) Queues() []command.CommandQueue     { return nil }
func (h *stubTunable) DefaultBindings() []control.Binding { return nil }
func (h *stubTunable) Settings() []control.Setting        { return h.settings }

func sw(label, section string) control.Setting {
	return control.Switch{Label: label, Section: section, Get: func() bool { return false },
		Set: func(bool) command.Command { return shade{} }}
}
func rg(label, section string) control.Setting {
	return control.Range{Label: label, Section: section, Min: 0, Max: 10, Step: 1,
		Get: func() float64 { return 1 }, Set: func(float64) command.Command { return shade{} }}
}

// demoTunables are the settings of board/topography's plugins, the cameras' aside
func demoTunables() []plugin.CommandHandler {
	return []plugin.CommandHandler{
		&stubTunable{"gram.collision", []control.Setting{sw("Entities' boxes", "")}},
		&stubTunable{"gram.board", []control.Setting{sw("Grid", ""), sw("Cells held", ""),
			sw("Build a road", "")}},
		&stubTunable{"gram.navigation", []control.Setting{sw("Routes of selected units", "")}},
		&stubTunable{"gram.atmosphere", []control.Setting{sw("Clouds", control.Graphics),
			sw("High clouds' shadows", control.Graphics), sw("Frozen light", ""),
			rg("Hour of the frozen light", "")}},
		&stubTunable{"gram.topography", []control.Setting{sw("Haze over the ground", control.Graphics),
			rg("Full care out to", control.PictureCare), sw("Show the care", control.PictureAreas)}},
		&stubTunable{"gram.vision", []control.Setting{sw("Cones of sight", "")}},
		&stubTunable{"gram.minimap", []control.Setting{sw("Minimap", ""),
			sw("Minimap turns with the camera", "")}},
		&stubTunable{"gram.light", []control.Setting{sw("Lamps", ""),
			sw("Lamps' shadows", control.Graphics), sw("Coarse shadows", control.Graphics),
			sw("Shadows", control.Graphics)}},
		&stubTunable{"gram.world", []control.Setting{sw("Sectors drawn", ""), sw("Sectors' bands", "")}},
		&stubTunable{"gram.clock", []control.Setting{rg("Tempo", "")}},
	}
}

func demoRig(t *testing.T) *Plugin {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	all := append([]plugin.CommandHandler{cameras.NewPlugin(w, cameras.Config{})}, demoTunables()...)
	p := NewPlugin(w, all...)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSettings_TwinSectionsLieSideBySideALineASetting(t *testing.T) {
	p := demoRig(t)
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1920, ScreenHeight: 1080}}, &composed{}
	p.settings.Open(rt, comp)
	s := p.settings.screen
	s.Drawers()
	s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1920, 1080)))
	boxOf := func(name string) geom.AABB {
		t.Helper()
		e := s.Element(name)
		if e == nil {
			t.Fatalf("no %s on the screen", name)
		}
		return e.Box()
	}
	tactical, riding := boxOf("section:Picture/Tactical"), boxOf("section:Picture/Riding")
	if tactical.TopLeft.Y != riding.TopLeft.Y || riding.TopLeft.X <= tactical.TopLeft.X {
		t.Errorf("Tactical at %v, Riding at %v; want the twins' names on one row, Riding to the "+
			"right", tactical.TopLeft, riding.TopLeft)
	}
	a, b := boxOf("setting:Picture/Tactical/Resolution"), boxOf("setting:Picture/Riding/Resolution")
	if a.TopLeft.Y != b.TopLeft.Y || b.TopLeft.X-a.TopLeft.X != twinColumn {
		t.Errorf("the resolutions at %v and %v; want one row, a twin's column apart", a.TopLeft,
			b.TopLeft)
	}
	shown := boxOf("setting:Picture/Tactical/Layers/Show the layers")
	if shown.TopLeft.Y <= boxOf("section:Picture/Tactical/Layers").TopLeft.Y {
		t.Errorf("the layers shown at %v lie over their subheading", shown.TopLeft)
	}
}

func TestSettings_EveryControlLiesOnAPageOfTheScreenHoweverSmall(t *testing.T) {
	p := demoRig(t)
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1024, ScreenHeight: 768}}, &composed{}
	p.settings.Open(rt, comp)
	s := p.settings.screen
	screen := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1024, 768))
	if n := len(p.settings.pages); n < 2 {
		t.Fatalf("at 1024 by 768 the settings fill %d page, want more: the columns past the "+
			"width go to the next", n)
	}
	for _, name := range []string{
		"setting:Picture/Tactical/Layers/Show the layers",
		"setting:Picture/Riding/Layers/Show the layers",
		"setting:Picture/Areas/Show the care",
		"setting:World/Sectors drawn",
		"setting:Minimap/Minimap",
		"setting:Clock/Tempo",
		"settings.page",
	} {
		found := false
		for i := range p.settings.pages {
			p.settings.page = i
			s.Replace(p.settings.pageScreen())
			s.Drawers()
			s.Lay(screen)
			if e := s.Element(name); e != nil && within(e.Box(), screen) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s lies on no page of the screen", name)
		}
	}
	keys := &control.InputEvents{KeyEvents: []control.KeyEvent{{Key: control.KeyPageDown,
		Action: control.ActionPress}}}
	p.settings.page = 0
	p.settings.HandleEvents(keys, rt, comp)
	if p.settings.page != 1 {
		t.Errorf("PgDn turned to page %d, want 1", p.settings.page)
	}
}
