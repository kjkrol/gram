package players

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestWritten_SpellsATriggerAsAHelpScreenDoes(t *testing.T) {
	for _, c := range []struct {
		trigger control.Trigger
		want    string
	}{
		{control.KeyPress{Key: control.KeyW, Mods: control.Mods{Shift: true}}, "Shift+W"},
		{control.KeyPress{Key: control.KeyBracketRight}, "]"},
		{control.KeyPress{Key: control.KeyEscape, Mods: control.Mods{Shift: true}}, "Shift+Esc"},
		{control.KeyHeld{Key: control.KeyQ}, "Q (held)"},
		{control.ButtonPress{Button: control.MouseButtonRight, Mods: control.Mods{Shift: true}.Holding(control.KeyS)}, "Shift+S+right click"},
		{control.Drag{Button: control.MouseButtonLeft, Mods: control.Mods{}.Holding(control.KeyL)}, "L+left drag"},
		{control.ButtonHeld{Button: control.MouseButtonMiddle}, "middle drag"},
		{control.Wheel{}, "wheel"},
		{control.CursorAtEdge{}, "cursor at an edge"},
		{control.CursorInCorner{}, "cursor in a corner"},
	} {
		if got := Written(c.trigger); got != c.want {
			t.Errorf("Written(%#v) = %q, want %q", c.trigger, got, c.want)
		}
	}
}

// text is the group's lines, a trigger and what it does each, for the tests to search
func (g group) text() string {
	lines := make([]string, len(g.keys))
	for i := range g.keys {
		lines[i] = g.keys[i] + " " + g.labels[i]
	}
	return strings.Join(lines, "\n")
}

func TestControls_ListTheBindingsByPluginAndTheScenesKeys(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	cams := cameras.NewPlugin(w, cameras.Config{})
	p := NewPlugin(w, cams)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	s := p.OwnKeys(SceneKeys{{Key: control.KeyR, Label: "Build a road"}}).controls
	groups := s.groups()
	names := map[string]string{}
	for _, g := range groups {
		names[g.name] = g.text()
	}
	if len(groups) != 4 || groups[0].name != "Game" || groups[1].name != "World" ||
		groups[2].name != "Cameras" || groups[3].name != "General" {
		t.Fatalf("groups %v, want Game, World, Cameras and General in that order", names)
	}
	if lines := names["Cameras"]; !strings.Contains(lines, "W (held)") || !strings.Contains(lines, "Scroll up") || !strings.Contains(lines, "wheel") {
		t.Errorf("the camera's lines %q, want WASD and the wheel among them", lines)
	}
	if lines := names["World"]; !strings.Contains(lines, "Shift+P") || !strings.Contains(lines, "Pause the game") {
		t.Errorf("the world's lines %q, want the clock's Shift+P", lines)
	}
	if lines := names["Game"]; lines != "R Build a road" {
		t.Errorf("the game's lines %q, want its own R alone", lines)
	}
	lines := names["General"]
	for _, want := range []string{"Shift+Esc", "Quit", "K ", "F11"} {
		if !strings.Contains(lines, want) {
			t.Errorf("the general lines %q, want the players' K, Shift+Esc and the engine's F11",
				lines)
			break
		}
	}
}

type partNow section.Part

func (p *partNow) Section() section.Part { return section.Part(*p) }

func TestControls_ListTheGamesOwnBindingsUnderGame(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(w, cameras.NewPlugin(w, cameras.Config{}))
	now := partNow(section.Players)
	p.sections = &now
	one := p.Local("one")
	if err := one.Bind(w.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	now = partNow(section.Controls)
	spawn := control.Build(control.KeyPress{Key: control.KeyN}, "Spawn a walker",
		func(control.Context) (world.Spawn, bool) { return world.Spawn{}, false })
	if err := one.Bind(spawn); err != nil {
		t.Fatal(err)
	}
	groups := p.controls.groups()
	if len(groups) < 2 || groups[0].name != "Game" || groups[1].name != "World" {
		t.Fatalf("groups %+v, want the game's first, under Game, then World", groups)
	}
	if own := groups[0].text(); !strings.Contains(own, "Spawn a walker") ||
		strings.Contains(own, "Pause the game") {
		t.Errorf("the game's lines %q, want its N alone", own)
	}
	if world := groups[1].text(); strings.Contains(world, "Spawn a walker") ||
		!strings.Contains(world, "Pause the game") {
		t.Errorf("the world's lines %q, want its defaults without the game's N", world)
	}
}

func TestControls_ListWhatHoldsInTheCamerasMode(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	cams := cameras.NewPlugin(w, cameras.Config{})
	p := NewPlugin(w, cams)
	cam := &ridingCam{Camera: cams.Write().New(cameras.FromAbove, camera.Config{})}
	pl := p.Local("one")
	p.Through(pl).Over(geom.AABB{}, render.NewFeed(cam, nil))
	if err := pl.Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	s := p.controls
	behind := camera.Behind
	camera := func() string {
		for _, g := range s.groups() {
			if g.name == "Cameras" {
				return g.text()
			}
		}
		return ""
	}
	if lines := camera(); !strings.Contains(lines, "Scroll up") || !strings.Contains(lines, "cursor at an edge") || s.title() != "Controls" {
		t.Errorf("free, the camera's lines %q under %q, want the scrolling keys under Controls", lines, s.title())
	}
	cam.on = true
	if lines := camera(); strings.Contains(lines, "Scroll up") || strings.Contains(lines, "cursor at an edge") || !strings.Contains(lines, "wheel") {
		t.Errorf("riding, the camera's lines %q, want the wheel alone", lines)
	}
	if !strings.Contains(s.title(), "first person") {
		t.Errorf("riding, the title is %q, want it to say first person", s.title())
	}
	cam.how = behind
	if lines := camera(); strings.Contains(lines, "Scroll up") ||
		!strings.Contains(s.title(), "third person") {
		t.Errorf("behind, the camera's lines %q under %q, want no scrolling under third person",
			lines, s.title())
	}
}

type ridingCam struct {
	camera.Camera
	on  bool
	how camera.How // zero, Inside
}

func (r *ridingCam) Fasten(camera.Fastening) {}

func (r *ridingCam) Fastening() camera.Fastening {
	if !r.on {
		return camera.Fastening{}
	}
	how := r.how
	if how == 0 {
		how = camera.Inside
	}
	return camera.Fastening{Entity: 1, How: how}
}

func TestSceneKeys_HandleRunsTheKeyPressedWithItsModifiers(t *testing.T) {
	ran := ""
	keys := SceneKeys{
		{Key: control.KeyEscape, Shift: true, Label: "Quit", Do: func(game.Runtime, game.Composition) { ran += "quit " }},
		{Key: control.KeyB, Label: "Grid", Do: func(game.Runtime, game.Composition) { ran += "grid " }},
	}
	events := &control.InputEvents{}
	events.AddKeyEvent(control.KeyEscape, control.ActionPress) // no Shift: nothing
	events.AddKeyEvent(control.KeyB, control.ActionPress)
	events.AddKeyEvent(control.KeyB, control.ActionRelease)
	keys.Handle(events, nil, nil)
	if ran != "grid " {
		t.Errorf("ran %q, want the grid alone: Esc without Shift is nothing, a release nothing", ran)
	}
	events = &control.InputEvents{}
	events.Modifiers.Shift = true
	events.AddKeyEvent(control.KeyEscape, control.ActionPress)
	keys.Handle(events, nil, nil)
	if ran != "grid quit " {
		t.Errorf("ran %q, want the quit after Shift+Esc", ran)
	}
}

type engineStub struct {
	game.Runtime
	quit, paused bool
	props        game.Props
}

func (e *engineStub) Props() game.Props { return e.props }

func (e *engineStub) Quit()        { e.quit = true }
func (e *engineStub) Paused() bool { return e.paused }
func (e *engineStub) Pause()       { e.paused = true }
func (e *engineStub) Resume()      { e.paused = false }

type shown struct {
	game.Composition
	names []string
}

func (c *shown) Show(name string) { c.names = append(c.names, name) }

type engine struct {
	saves command.Queue[game.Save]
	loads command.Queue[game.Load]
	quits command.Queue[game.Quit]
}

func (e *engine) Queues() []command.CommandQueue {
	return []command.CommandQueue{&e.saves, &e.loads, &e.quits}
}

func (*engine) DefaultBindings() []control.Binding { return nil }

func TestHandle_TurnsTheGamesKeysIntoCommands(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	eng := &engine{}
	if err := w.Carry(eng); err != nil {
		t.Fatal(err)
	}
	cams := cameras.NewPlugin(w, cameras.Config{})
	p := NewPlugin(w, cams)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	built := 0
	p.OwnKeys(SceneKeys{{Key: control.KeyR, Label: "Build a road", Do: func(game.Runtime, game.Composition) { built++ }}})
	rt, comp := &engineStub{props: game.Props{ScreenWidth: 800, ScreenHeight: 600}}, &shown{}
	press := func(key control.Key, shift bool) {
		events := &control.InputEvents{}
		events.Modifiers.Shift = shift
		events.AddKeyEvent(key, control.ActionPress)
		p.Handle(events, rt, comp)
	}
	press(control.KeyEscape, false)
	if !eng.quits.Empty() {
		t.Fatal("Esc alone quit the game, want Shift+Esc")
	}
	press(control.KeyK, false)
	if len(comp.names) != 1 || comp.names[0] != ControlsName || !rt.paused {
		t.Errorf("K showed %v, paused %v; want the controls shown and the game held", comp.names, rt.paused)
	}
	press(control.KeyR, false)
	if built != 1 {
		t.Errorf("the game's own R ran %d times, want once", built)
	}
	press(control.KeyF5, false)
	press(control.KeyF9, false)
	press(control.KeyEscape, true)
	if eng.saves.Empty() || eng.loads.Empty() || eng.quits.Empty() {
		t.Errorf("F5, F9 and Shift+Esc gave the engine save %v, load %v, quit %v; want each",
			!eng.saves.Empty(), !eng.loads.Empty(), !eng.quits.Empty())
	}
}

func TestWrapped_BreaksBetweenWordsWithinTheWidth(t *testing.T) {
	font := render.DefaultFont()
	width, _ := font.Measure("Walk up to a")
	got := wrapped(font, "Walk up to a host and answer it.", width+1)
	want := []string{"Walk up to a", "host and", "answer it."}
	if !slices.Equal(got, want) {
		t.Errorf("wrapped %q, want %q", got, want)
	}
}

func TestControls_TheListingLaysTheTitleInItsPanelAndTheSectionsFramedInColumns(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	cams := cameras.NewPlugin(w, cameras.Config{})
	p := NewPlugin(w, cams)
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	var keys SceneKeys
	for i := range 30 { // enough of the game's own keys to need a second column
		keys = append(keys, SceneKey{Key: control.KeyA, Label: fmt.Sprintf("Do thing %d", i)})
	}
	s := p.OwnKeys(keys).controls
	lay := func(w, h float32) {
		s.screen.Replace(s.listing(int(w), int(h)))
		s.screen.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(w, h)))
	}
	lay(1200, 400)
	title := s.screen.Element("controls.title").Box()
	if mid := (title.TopLeft.X + title.BottomRight.X) / 2; mid < 590 || mid > 610 ||
		title.TopLeft.Y < menuMargin {
		t.Errorf("the title at %v, want it centred under the top margin", title)
	}
	game := s.screen.Element("frame:Game")
	if game == nil || s.screen.Element("section:Game") == nil {
		t.Fatal("the game's keys want a framed section with a heading")
	}
	if box := game.Box(); box.TopLeft.Y < title.BottomRight.Y {
		t.Errorf("the game's frame %v begins above the title's bottom %v", box, title.BottomRight.Y)
	}
	first, second := s.screen.Element("controls.column.0"), s.screen.Element("controls.column.1")
	if first == nil || second == nil {
		t.Fatal("thirty keys over a screen 400 tall need two columns")
	}
	a, b := first.Box(), second.Box()
	if a.BottomRight.X > b.TopLeft.X {
		t.Errorf("the columns overlap: %v and %v", a, b)
	}
	if a.TopLeft.Y < title.BottomRight.Y || b.TopLeft.Y < title.BottomRight.Y {
		t.Errorf("a column %v or %v begins above the title's bottom %v", a, b, title.BottomRight.Y)
	}
	lay(500, 400)
	if s.screen.Element("controls.column.1") != nil || s.screen.Element("controls.scroll") == nil {
		t.Error("a screen too narrow for two columns wants one, scrolled")
	}
}

type composed struct {
	game.Composition
	names []string
}

func (c *composed) Show(name string) { c.names = append(c.names, name) }
func (c *composed) Hide(name string) {
	c.names = slices.DeleteFunc(c.names, func(n string) bool { return n == name })
}
func (c *composed) Order() []string { return c.names }

func TestHandle_TheTildeTurnsTheTelemetryPanelBasicExtendedHiddenUnderGeneral(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	tp := telemetry.NewPlugin(w)
	p := NewPlugin(w, cameras.NewPlugin(w, cameras.Config{}))
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	general := ""
	for _, g := range p.controls.groups() {
		if g.name == "General" {
			general = g.text()
		}
	}
	if !strings.Contains(general, "~") || !strings.Contains(general, "telemetry panel") {
		t.Errorf("the general lines %q, want ~ for the telemetry panel", general)
	}
	rt, comp := &engineStub{}, &composed{}
	press := func() {
		events := &control.InputEvents{}
		events.AddKeyEvent(control.KeyBackquote, control.ActionPress)
		p.Handle(events, rt, comp)
	}
	for presses, want := range []struct {
		shown bool
		mode  telemetry.Mode
	}{
		{true, telemetry.Basic},
		{true, telemetry.Extended},
		{false, telemetry.Extended},
		{true, telemetry.Basic},
	} {
		press()
		if tp.Read(0).Shown(comp) != want.shown || tp.Read(0).Mode() != want.mode {
			t.Errorf("after %d presses of ~ the panel is shown %v in mode %v, want shown %v in %v",
				presses+1, tp.Read(0).Shown(comp), tp.Read(0).Mode(), want.shown, want.mode)
		}
	}
}

type toolHandler struct{ takes command.Queue[game.Save] }

func (h *toolHandler) Name() string                   { return "gram.bridges" }
func (h *toolHandler) Queues() []command.CommandQueue { return nil }
func (h *toolHandler) DefaultBindings() []control.Binding {
	return []control.Binding{control.Give(control.KeyPress{Key: control.KeyB,
		Mods: control.Mods{Shift: true}}, "Build a bridge", game.Save{})}
}

func TestControls_ListAKeyUnderThePluginWhoseDefaultsGaveIt(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}})
	if err := w.Carry(&engine{}); err != nil {
		t.Fatal(err)
	}
	p := NewPlugin(w, cameras.NewPlugin(w, cameras.Config{}), &toolHandler{})
	if err := p.Local("one").Bind(p.Defaults()...); err != nil {
		t.Fatal(err)
	}
	for _, g := range p.controls.groups() {
		lines := g.text()
		if g.name == "Bridges" && !strings.Contains(lines, "Build a bridge") {
			t.Errorf("the bridges' lines %q, want Build a bridge among them", lines)
		}
		if g.name == "General" && strings.Contains(lines, "Build a bridge") {
			t.Errorf("the general lines %q list Build a bridge: it is the bridges' key", lines)
		}
	}
}
