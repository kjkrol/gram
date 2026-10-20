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
	"github.com/kjkrol/gram/ui"
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
			rg("Hour", "")}},
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

// columnsAt is how many columns the settings screen lays at a screen of w by h
func columnsAt(t *testing.T, p *Plugin, w, h int) int {
	t.Helper()
	rt, comp := &engineStub{props: game.Props{ScreenWidth: w, ScreenHeight: h}}, &composed{}
	p.settings.Open(rt, comp)
	s := p.settings.screen
	s.Drawers()
	s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(float64(w), float64(h))))
	// the tick after the first frame builds the listing for the screen as it is
	p.settings.HandleEvents(&control.InputEvents{}, rt, comp)
	s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(float64(w), float64(h))))
	n := 0
	for s.Element(settingsColumn(n)) != nil {
		n++
	}
	return n
}

func TestSettings_FewerColumnsWhereTheWidthIsShortTheRestScrolledIn(t *testing.T) {
	p := demoRig(t)
	wide, narrow := columnsAt(t, p, 1920, 1080), columnsAt(t, p, 1024, 768)
	if wide < 2 || narrow >= wide {
		t.Fatalf("%d columns at 1920 by 1080, %d at 1024 by 768; want several, then fewer", wide,
			narrow)
	}
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1024, ScreenHeight: 768}}, &composed{}
	p.settings.Open(rt, comp)
	s := p.settings.screen
	s.Drawers()
	screen := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1024, 768))
	s.Lay(screen)
	p.settings.HandleEvents(&control.InputEvents{}, rt, comp)
	s.Lay(screen)
	if s.Element("settings.scroll") == nil {
		t.Fatal("no scroll on the screen")
	}
	left := map[string]bool{
		"setting:Picture/Tactical/Layers/Show the layers": true,
		"setting:Picture/Riding/Layers/Show the layers":   true,
		"setting:Picture/Areas/Show the care":             true,
		"setting:World/Sectors drawn":                     true,
		"setting:Minimap/Minimap":                         true,
		"setting:Clock/Tempo":                             true,
	}
	wheel := &control.InputEvents{MousePos: geom.NewVec(180, 400), ScrollDelta: -1}
	for range 80 {
		for name := range left {
			if e := s.Element(name); e != nil && within(e.Box(), screen) {
				delete(left, name)
			}
		}
		if len(left) == 0 {
			break
		}
		p.settings.HandleEvents(wheel, rt, comp)
		s.Lay(screen)
	}
	for name := range left {
		t.Errorf("%s never scrolled into the screen", name)
	}
}

func TestSettings_TheListingFollowsTheScreenAsTheWindowGrows(t *testing.T) {
	p := demoRig(t)
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 1024, ScreenHeight: 768}}, &composed{}
	p.settings.Open(rt, comp)
	s := p.settings.screen
	s.Drawers()
	columns := func() int {
		n := 0
		for s.Element(settingsColumn(n)) != nil {
			n++
		}
		return n
	}
	s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1024, 768)))
	small := columns()
	// the window goes full screen: the next tick's events find the scene laid wider
	s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(2560, 1440)))
	p.settings.HandleEvents(&control.InputEvents{}, rt, comp)
	s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(2560, 1440)))
	if large := columns(); large <= small {
		t.Errorf("%d columns at 1024 by 768 and %d at 2560 by 1440 after the resize, want more",
			small, large)
	}
	if e := s.Element("setting:Clock/Tempo"); e == nil ||
		!within(e.Box(), geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(2560, 1440))) {
		t.Error("the last setting is not on the full screen without scrolling")
	}
}

func TestSettings_EverySectionIsFramedUnderALargerTitle(t *testing.T) {
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
	for _, c := range []struct{ frame, heading, setting string }{
		{"frame:Picture", "section:Picture", "setting:Picture/Riding/Resolution"},
		{"frame:Atmosphere", "section:Atmosphere", "setting:Atmosphere/Hour"},
	} {
		frame := boxOf(c.frame)
		inset := geom.NewAABB(frame.TopLeft.Add(geom.NewVec(sectionPadding, sectionPadding)),
			frame.BottomRight.Sub(geom.NewVec(sectionPadding, sectionPadding)))
		for _, name := range []string{c.heading, c.setting} {
			if box := boxOf(name); !within(box, inset) {
				t.Errorf("%s at %v lies out of its frame %v inset by the padding", name, box, frame)
			}
		}
	}
	title := boxOf("settings.title")
	_, large := ui.DefaultTheme().Large.Measure("Settings")
	if tall := title.BottomRight.Y - title.TopLeft.Y; tall <= float64(large) {
		t.Errorf("the title is %v tall, want taller than the theme's large font's %v", tall, large)
	}
}
