package engine

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/dialog"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type sectioned struct {
	world *world.Plugin
	unit  kind.Of[struct{}]
	ran   []string
}

func (s *sectioned) note(name string) { s.ran = append(s.ran, name) }

func (s *sectioned) makeWorld(w *world.Plugin) {
	s.note("plugins")
	s.world = w
}

func (s *sectioned) defineUnit() {
	s.note("kinds")
	kind.Define[struct{}](s.world.Kinds(), "unit", walkerSpec())
	s.unit = kind.Named[struct{}](s.world.Kinds(), "unit")
}

func walkerSpec() kind.Spec {
	return kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(50, 50), 5, 5)}),
		comp.Const(world.Velocity{}),
	}
}

func (s *sectioned) update(ctx goke.RunCtx, d time.Duration) { s.world.RunPlan(ctx, d) }

func TestStage_RunsItsSectionsInOrder(t *testing.T) {
	s := &sectioned{}
	st := stage.New("meadow").
		World(testWorldConfig()).Plugins(s.makeWorld).
		Players(func() { s.note("players") }).
		Effects(func() error { s.note("effects"); return nil }).
		Rules(func(game.Initializer) error { s.note("rules"); return nil }).
		Kinds(func() { s.note("cells") }, s.defineUnit).
		Spawn(func() { s.note("ground") }, func() { s.note("units"); s.world.Write().Seed(s.unit.Entry(struct{}{})) }).
		Scenes(func() []game.Scene { s.note("scenes"); return []game.Scene{&scene{name: "main"}, &scene{name: "help"}} }).
		Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	want := []string{"plugins", "players", "effects", "rules", "cells", "kinds", "ground", "units", "scenes"}
	if !slices.Equal(s.ran, want) {
		t.Errorf("sections ran %v, want %v", s.ran, want)
	}
	if st.Name() != "meadow" {
		t.Errorf("the Stage is named %q, want meadow", st.Name())
	}
	if got := st.Stack().Composition().Order(); !slices.Equal(got, []string{"main"}) {
		t.Errorf("shown %v, want the first scene alone", got)
	}
}

// Shows names the scenes shown in place of the first; a loaded Stage spawns nothing and still
// makes its scenes.
func TestStage_ShowsAndALoadedStage(t *testing.T) {
	props := game.Props{Saves: t.TempDir()}
	define := func(s *sectioned) game.Stage {
		return stage.New("meadow").
			World(testWorldConfig()).Plugins(s.makeWorld).
			Spawn(func() { s.note("ground") }, func() { s.note("units") }).
			Scenes(func() []game.Scene {
				s.note("scenes")
				return []game.Scene{&scene{name: "main"}, &scene{name: "help"}, &scene{name: "hud"}}
			}).
			Shows("main", "hud").
			Update(s.update)
	}
	fresh := define(&sectioned{})
	eng := NewEngine(oneStageGame{stage: fresh, props: props})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := fresh.Stack().Composition().Order(); !slices.Equal(got, []string{"main", "hud"}) {
		t.Errorf("shown %v, want main and hud", got)
	}
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s := &sectioned{}
	eng = NewEngine(oneStageGame{stage: define(s), props: props})
	if err := eng.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng.Init(); err != nil {
		t.Fatalf("Init from the save: %v", err)
	}
	if !slices.Equal(s.ran, []string{"plugins", "scenes"}) {
		t.Errorf("ran %v: a loaded Stage spawns nothing and makes its scenes", s.ran)
	}
}

func TestStage_RunsThePluginsLooksOnceSetUpBeforeItSpawnsOrLoads(t *testing.T) {
	props := game.Props{Saves: t.TempDir()}
	define := func(s *sectioned) game.Stage {
		looking := &stubPlugin{name: "looking", installFn: func(ctx plugin.Installer) error {
			ctx.Looks(func() { s.note("looks") })
			return nil
		}}
		return stage.New("meadow").
			World(testWorldConfig()).
			Plugins(func(w *world.Plugin) {
				s.makeWorld(w)
				w.Made().Add(looking)
			}).
			Kinds(s.defineUnit).
			Spawn(func() { s.note("spawn") }).
			Scenes(func() []game.Scene {
				s.note("scenes")
				return []game.Scene{&scene{name: "main"}}
			}).
			Update(s.update)
	}
	s := &sectioned{}
	eng := NewEngine(oneStageGame{stage: define(s), props: props})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	want := []string{"plugins", "kinds", "looks", "spawn", "scenes"}
	if !slices.Equal(s.ran, want) {
		t.Errorf("a fresh Stage ran %v, want %v", s.ran, want)
	}
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s = &sectioned{}
	eng = NewEngine(oneStageGame{stage: define(s), props: props})
	if err := eng.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng.Init(); err != nil {
		t.Fatalf("Init from the save: %v", err)
	}
	want = []string{"plugins", "kinds", "looks", "scenes"}
	if !slices.Equal(s.ran, want) {
		t.Errorf("a loaded Stage ran %v, want %v", s.ran, want)
	}
}

type kept struct {
	scene
	note     string
	restored bool
}

func (k *kept) Persisted() []any { return []any{&k.note} }
func (k *kept) Restore()         { k.restored = true }

func TestStage_ScenesOfALoadedGameAreAsSaved(t *testing.T) {
	props := game.Props{Saves: t.TempDir()}
	define := func(s *sectioned, k *kept) game.Stage {
		return stage.New("meadow").
			World(testWorldConfig()).Plugins(s.makeWorld).
			Scenes(func() []game.Scene { return []game.Scene{&scene{name: "main"}, k, &scene{name: "hud"}} }).
			Update(s.update)
	}
	fresh := &kept{scene: scene{name: "help"}}
	eng := NewEngine(oneStageGame{stage: define(&sectioned{}, fresh), props: props})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	eng.current.stage.Stack().Composition().Show("hud")
	fresh.note = "seen"
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := &kept{scene: scene{name: "help"}}
	st := define(&sectioned{}, loaded)
	eng = NewEngine(oneStageGame{stage: st, props: props})
	if err := eng.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng.Init(); err != nil {
		t.Fatalf("Init of the loaded game: %v", err)
	}
	if got := st.Stack().Composition().Order(); !slices.Equal(got, []string{"main", "hud"}) {
		t.Errorf("a loaded game shows %v, want main and hud as saved", got)
	}
	if loaded.note != "seen" || !loaded.restored {
		t.Errorf("a loaded game's scene has %q, restored %v; want its saved state, restored", loaded.note, loaded.restored)
	}
}

func TestStage_ASectionsErrorNamesIt(t *testing.T) {
	s := &sectioned{}
	boom := errors.New("boom")
	st := stage.New("meadow").World(testWorldConfig()).Plugins(s.makeWorld).Effects(func() error { return boom }).Update(s.update)
	err := NewEngine(oneStageGame{stage: st}).Init()
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "meadow") || !strings.Contains(err.Error(), "Effects") {
		t.Errorf("Init = %v, want boom of the stage meadow, in Effects", err)
	}
}

func TestStage_RefusesAThingOutOfItsSection(t *testing.T) {
	type part func(s *sectioned, ctx game.Initializer) error
	useAPlugin := func(s *sectioned, ctx game.Initializer) error { return ctx.Use(&stubPlugin{name: "late"}) }
	plays := func(s *sectioned, _ game.Initializer) error {
		s.world.Roles().Define("sectioned", act.Branch[clock.Moment]{
			Name: "leave",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(world.Despawn{}),
		})
		s.world.Plays(s.world.Roles().Named("sectioned"))
		return nil
	}
	effectDefined := func(s *sectioned, _ game.Initializer) error {
		s.world.Write().Effects().Define("glow", effect.Spec{})
		return nil
	}
	kindDefined := func(s *sectioned, _ game.Initializer) error { s.defineUnit(); return nil }
	commands := func(s *sectioned, _ game.Initializer) error {
		s.world.Write().Effects().Define("lit", effect.Spec{})
		s.world.Commands().Define("light",
			ceremony.Put(s.world.Write().Effects().Named("lit")).On(ceremony.World))
		return nil
	}
	for name, tc := range map[string]struct {
		do     part
		wants  string // the section it belongs in
		kinds  bool   // done in Kinds, a wrong place, rather than in Effects
		panics bool
		plugin bool // in Plugins, the plugin is handed over rather than used
	}{
		"a plugin used":     {do: useAPlugin, wants: "Plugins", plugin: true},
		"a role defined":    {do: plays, wants: "Rules", kinds: true, panics: true},
		"an effect defined": {do: effectDefined, wants: "Effects", kinds: true, panics: true},
		"a kind defined":    {do: kindDefined, wants: "Kinds", panics: true},
		"a command defined": {do: commands, wants: "Commands", panics: true},
	} {
		t.Run(name, func(t *testing.T) {
			s := &sectioned{}
			wrong := func(ctx game.Initializer) (err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic: %v", r)
					}
				}()
				return tc.do(s, ctx)
			}
			chain := stage.New("meadow").World(testWorldConfig()).Plugins(s.makeWorld)
			var st game.Stage
			if tc.kinds {
				st = chain.Kinds(wrong).Update(s.update)
			} else {
				st = chain.Effects(wrong).Update(s.update)
			}
			err := NewEngine(oneStageGame{stage: st}).Init()
			if err == nil || !strings.Contains(err.Error(), "it belongs in "+tc.wants) {
				t.Fatalf("out of its section: Init = %v, want it refused as belonging in %s", err, tc.wants)
			}
			if tc.panics != strings.Contains(err.Error(), "panic:") {
				t.Errorf("refused by %v, want a panic %v", err, tc.panics)
			}

			// in Plugins anything goes
			s = &sectioned{}
			st = stage.New("meadow").World(testWorldConfig()).Plugins(func(w *world.Plugin) {
				s.makeWorld(w)
				if tc.plugin {
					w.Made().Add(&stubPlugin{name: "late"})
					return
				}
				if err := tc.do(s, nil); err != nil {
					t.Errorf("in Plugins: %v, want it taken", err)
				}
			}).Update(s.update)
			if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
				t.Errorf("in Plugins: Init = %v, want it taken", err)
			}
		})
	}
}

func TestStage_SeedsInTheirSections(t *testing.T) {
	s := &sectioned{}
	var during section.Part
	st := stage.New("meadow").World(testWorldConfig()).Plugins(s.makeWorld).Kinds(s.defineUnit).
		Controls(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			s.world.Write().Seed(s.unit.Entry(struct{}{}))
			return nil
		}).Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err == nil || !strings.Contains(err.Error(), "it belongs in Spawn") {
		t.Errorf("units seeded in Controls: Init = %v, want them refused as belonging in Spawn", err)
	}
	s = &sectioned{}
	spy := &stubPlugin{name: "spy", installFn: func(ctx plugin.Installer) error {
		during = ctx.(section.Reader).Section()
		return nil
	}}
	st = stage.New("meadow").
		World(testWorldConfig()).Plugins(func(w *world.Plugin) { s.makeWorld(w); w.Made().Add(spy) }).
		Kinds(s.defineUnit).
		Spawn(func() { s.world.Write().Seed(s.unit.Entry(struct{}{})) }).Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Errorf("units seeded in Spawn: Init = %v", err)
	}
	if during != section.Plugins {
		t.Errorf("in Plugins the engine says %v", during)
	}
}

func TestStage_ThePluginsRefuseTheirsOutOfItsSection(t *testing.T) {
	type built struct {
		sectioned
		board   *board.Plugin
		players *players.Plugin
		player  *players.Player
	}
	cfg := testWorldConfig()
	cfg.Heights = true
	plugins := func(b *built) func(w *world.Plugin) {
		return func(w *world.Plugin) {
			b.makeWorld(w)
			b.board = board.NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32), &cell.MultipleOccupancy{}, b.world)
			b.players = players.NewPlugin(b.world)
		}
	}
	grass := cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land}
	for name, tc := range map[string]struct {
		in    func(b *built) error // done in Effects, a wrong place
		wants string
	}{
		"a kind of cell defined": {func(b *built) error { b.board.Read(0).CellKinds().Define("grass", grass); return nil }, "Kinds"},
		"the board seeded":       {func(b *built) error { b.board.Write().Seed(board.Layout{}); return nil }, "Spawn"},
		"a player added":         {func(b *built) error { b.players.Add("late"); return nil }, "Players"},
		"keys bound":             {func(b *built) error { return b.player.Bind() }, "Players"},
	} {
		t.Run(name, func(t *testing.T) {
			b := &built{}
			st := stage.New("meadow").World(cfg).Plugins(plugins(b)).
				Players(func() { b.player = b.players.Add("ai") }).
				Effects(func() (err error) {
					defer func() {
						if r := recover(); r != nil {
							err = fmt.Errorf("panic: %v", r)
						}
					}()
					return tc.in(b)
				}).Update(b.update)
			err := NewEngine(oneStageGame{stage: st}).Init()
			if err == nil || !strings.Contains(err.Error(), "it belongs in "+tc.wants) {
				t.Errorf("in Effects: Init = %v, want it refused as belonging in %s", err, tc.wants)
			}
		})
	}
	// each in its own section is taken
	b := &built{}
	st := stage.New("meadow").World(cfg).Plugins(plugins(b)).
		Players(func() { b.player = b.players.Add("ai") }).
		Kinds(func() { b.board.Read(0).CellKinds().Define("grass", grass) }).
		Controls(func() error { return b.player.Bind() }).
		Spawn(func() { b.board.Write().Seed(board.Layout{Default: "grass"}) }).
		Update(b.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Errorf("each in its section: Init = %v", err)
	}
}

var testPart = section.Define("Test", section.Commands, section.Kinds)

type partPlugin struct {
	stubPlugin
	ctx plugin.Installer
}

func newPartPlugin() *partPlugin {
	p := &partPlugin{stubPlugin: stubPlugin{name: "test.part"}}
	p.installFn = func(ctx plugin.Installer) error { p.ctx = ctx; return nil }
	return p
}

func TestStage_RunsAPluginsSectionWhereItIsWritten(t *testing.T) {
	s := &sectioned{}
	pp := newPartPlugin()
	var got *partPlugin
	var during section.Part
	st := stage.New("meadow").
		World(testWorldConfig()).Plugins(func(w *world.Plugin) { s.makeWorld(w); w.Made().Add(pp) }).
		Rules(func() { s.note("rules") }).
		With(section.Of(testPart, func(p *partPlugin) error {
			s.note("test")
			got, during = p, p.ctx.(section.Reader).Section()
			return nil
		})).
		Kinds(s.defineUnit).
		Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !slices.Equal(s.ran, []string{"plugins", "rules", "test", "kinds"}) {
		t.Errorf("ran %v, want the plugin's section between the rules and the kinds", s.ran)
	}
	if got != pp || during != testPart {
		t.Errorf("the section had plugin %v during %v, want the one used, during Test", got, during)
	}
}

func TestStage_PanicsForAPluginsSectionWrittenOutOfItsPlace(t *testing.T) {
	sec := section.Of(testPart, func(*partPlugin) error { return nil })
	for name, build := range map[string]func(){
		"too late":  func() { stage.New("meadow").Kinds(func() {}).With(sec) },
		"too early": func() { stage.New("meadow").Rules(func() {}).With(sec).Commands(func() {}) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "Test") {
					t.Errorf("built: %v, want a panic naming the section", r)
				}
			}()
			build()
		})
	}
}

func TestStage_APluginsSectionNeedsItsPlugin(t *testing.T) {
	s := &sectioned{}
	st := stage.New("meadow").World(testWorldConfig()).Plugins(s.makeWorld).
		With(section.Of(testPart, func(*partPlugin) error { return nil })).
		Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err == nil || !strings.Contains(err.Error(), "partPlugin") {
		t.Errorf("Init = %v, want it refused for want of the plugin", err)
	}
}

func TestStage_TheDialogsDefineInTheirOwnSection(t *testing.T) {
	type mood struct{ N int8 }
	warm := dialog.Alter(func(m *mood, _ uid.UID64) { m.N++ })
	run := func(chain func(d **dialog.Plugin) game.Stage) error {
		var d *dialog.Plugin
		return NewEngine(oneStageGame{stage: chain(&d)}).Init()
	}
	err := run(func(d **dialog.Plugin) game.Stage {
		return stage.New("meadow").
			World(testWorldConfig()).Plugins(func(w *world.Plugin) { *d = dialog.NewPlugin(w, dialog.Config{}) }).
			Effects(func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic: %v", r)
					}
				}()
				(*d).DefineEffects()
				(*d).Changes().Define("warm", warm)
				return nil
			}).
			Update(func(goke.RunCtx, time.Duration) {})
	})
	if err == nil || !strings.Contains(err.Error(), "it belongs in Conversations") {
		t.Errorf("a change defined in Effects: Init = %v, want it refused as belonging in Conversations", err)
	}
	err = run(func(d **dialog.Plugin) game.Stage {
		return stage.New("meadow").
			World(testWorldConfig()).Plugins(func(w *world.Plugin) { *d = dialog.NewPlugin(w, dialog.Config{}) }).
			Effects(func() { (*d).DefineEffects() }).
			With(dialog.Conversations(func(d *dialog.Plugin) error { d.Changes().Define("warm", warm); return nil })).
			Update(func(goke.RunCtx, time.Duration) {})
	})
	if err != nil {
		t.Errorf("a change defined in Conversations: Init = %v", err)
	}
}

func TestStage_UsesThePluginsInTheOrderMade(t *testing.T) {
	var installed []string
	stub := func(name string) *stubPlugin {
		return &stubPlugin{name: name, installFn: func(plugin.Installer) error { installed = append(installed, name); return nil }}
	}
	var made []string
	st := stage.New("meadow").World(testWorldConfig()).Plugins(func(w *world.Plugin) {
		w.Made().Add(stub("first"))
		players.NewPlugin(w) // tells the world of itself
		w.Made().Add(stub("second"))
		for _, p := range w.Made().All() {
			made = append(made, p.Name())
		}
	}).Update(func(goke.RunCtx, time.Duration) {})
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !slices.Equal(made, []string{"first", "gram.players", "second"}) || !slices.Equal(installed, []string{"first", "second"}) {
		t.Errorf("made %v, installed %v; want them in the order made", made, installed)
	}

	installed = nil
	st = stage.New("menu").Plugins(func(made *plugin.Made) {
		made.Add(stub("third"))
		made.Add(stub("fourth"))
	}).Update(func(goke.RunCtx, time.Duration) {})
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init without a world: %v", err)
	}
	if !slices.Equal(installed, []string{"third", "fourth"}) {
		t.Errorf("without a world installed %v, want third, then fourth", installed)
	}
}

type notedFigure struct{ s *sectioned }

func (f notedFigure) Effects(*world.Plugin)            { f.s.note("figure effects") }
func (f notedFigure) Rules(*world.Plugin)              { f.s.note("figure rules") }
func (f notedFigure) Define(*world.Plugin)             { f.s.note("figure define") }
func (f notedFigure) Look(*world.Atlas, *world.Plugin) { f.s.note("figure look") }

func TestStage_LeadsItsFiguresThroughTheSectionsAfterTheGamesOwn(t *testing.T) {
	s := &sectioned{}
	st := stage.New("meadow").
		World(testWorldConfig()).Plugins(s.makeWorld).
		Figures(func() { s.note("figures"); s.world.Figures(notedFigure{s}) }).
		Players(func() { s.note("players") }).
		Effects(func() { s.note("effects") }).
		Rules(func() { s.note("rules") }).
		Kinds(func() { s.note("cells") }, s.defineUnit).
		Spawn(func() { s.world.Write().Seed(s.unit.Entry(struct{}{})) }).
		Scenes(func() []game.Scene { return []game.Scene{&scene{name: "main"}} }).
		Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	want := []string{"plugins", "figures", "players", "effects", "figure effects", "rules",
		"figure rules", "cells", "kinds", "figure define", "figure look"}
	if !slices.Equal(s.ran, want) {
		t.Errorf("sections ran %v, want %v", s.ran, want)
	}
}

func TestStage_LeadsItsFiguresThroughSectionsTheGameLeftOut(t *testing.T) {
	s := &sectioned{}
	st := stage.New("meadow").
		World(testWorldConfig()).Plugins(s.makeWorld).
		Figures(func() { s.world.Figures(notedFigure{s}) }).
		Commands(func() { s.note("commands") }).
		Spawn(func() {}).
		Scenes(func() []game.Scene { return []game.Scene{&scene{name: "main"}} }).
		Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	want := []string{"plugins", "figure effects", "figure rules", "commands", "figure define",
		"figure look"}
	if !slices.Equal(s.ran, want) {
		t.Errorf("sections ran %v, want %v", s.ran, want)
	}
}
