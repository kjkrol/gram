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
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// sectioned is what a Stage built in sections keeps between them, and the order they ran in.
type sectioned struct {
	world *world.Plugin
	unit  kind.Of[struct{}]
	ran   []string
}

func (s *sectioned) note(name string) { s.ran = append(s.ran, name) }

func (s *sectioned) useWorld(ctx game.Initializer) {
	s.note("plugins")
	s.world = ctx.UseWorld(testWorldConfig())
}

func (s *sectioned) defineUnit() {
	s.note("kinds")
	s.unit = kind.Define[struct{}](s.world.Kinds(), "unit", walkerSpec())
}

func walkerSpec() kind.Spec {
	return kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(50, 50), 5, 5)}),
		comp.Const(world.Velocity{}),
	}
}

func (s *sectioned) update(ctx goke.RunCtx, d time.Duration) { s.world.RunPlan(ctx, d) }

// The sections run in the chain's order, each once; the Stage keeps its name, shows the first of
// its scenes, starts fresh and seeds the layout before the units.
func TestStage_RunsItsSectionsInOrder(t *testing.T) {
	s := &sectioned{}
	st := stage.New("meadow").
		Plugins(s.useWorld).
		Players(func() { s.note("players") }).
		Effects(func() error { s.note("effects"); return nil }).
		Rules(func(game.Initializer) error { s.note("rules"); return nil }).
		Kinds(s.defineUnit).
		Scenes(func() []game.Scene { s.note("scenes"); return []game.Scene{&scene{name: "main"}, &scene{name: "help"}} }).
		Layout(func() { s.note("layout") }).
		Units(func() { s.note("units"); s.world.Seed(s.unit.Entry(struct{}{})) }).
		Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	want := []string{"plugins", "players", "effects", "rules", "kinds", "scenes", "layout", "units"}
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

// Shows names the scenes shown in place of the first; Restore reporting true leaves the layout and
// the units unseeded.
func TestStage_ShowsAndRestore(t *testing.T) {
	s := &sectioned{}
	st := stage.New("meadow").
		Plugins(s.useWorld).
		Scenes(func() []game.Scene {
			return []game.Scene{&scene{name: "main"}, &scene{name: "help"}, &scene{name: "hud"}}
		}).
		Shows("main", "hud").
		Restore(func(game.Persistence) (bool, error) { return true, nil }).
		Layout(func() { s.note("layout") }).
		Units(func() { s.note("units") }).
		Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := st.Stack().Composition().Order(); !slices.Equal(got, []string{"main", "hud"}) {
		t.Errorf("shown %v, want main and hud", got)
	}
	if slices.Contains(s.ran, "layout") || slices.Contains(s.ran, "units") {
		t.Errorf("ran %v: a restored Stage seeds nothing", s.ran)
	}
}

// A section's error stops Init and names the Stage and the section.
func TestStage_ASectionsErrorNamesIt(t *testing.T) {
	s := &sectioned{}
	boom := errors.New("boom")
	st := stage.New("meadow").Plugins(s.useWorld).Effects(func() error { return boom }).Update(s.update)
	err := NewEngine(oneStageGame{stage: st}).Init()
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "meadow") || !strings.Contains(err.Error(), "Effects") {
		t.Errorf("Init = %v, want boom of the stage meadow, in Effects", err)
	}
}

// A thing defined out of its section is refused, by error or by panic, naming the section it
// belongs in; in its own section and in Plugins it is taken.
func TestStage_RefusesAThingOutOfItsSection(t *testing.T) {
	type part func(s *sectioned, ctx game.Initializer) error
	useAPlugin := func(s *sectioned, ctx game.Initializer) error { return ctx.Use(&stubPlugin{name: "late"}) }
	plays := func(s *sectioned, _ game.Initializer) error {
		s.world.Plays(rule.Role("sectioned").Obeys(rule.Then[clock.Moment]("leave", rule.All, rule.Order(world.Despawn{}))))
		return nil
	}
	effectDefined := func(s *sectioned, _ game.Initializer) error {
		s.world.Effects().Define("glow", effect.Spec{})
		return nil
	}
	kindDefined := func(s *sectioned, _ game.Initializer) error { s.defineUnit(); return nil }
	draw := func(s *sectioned, _ game.Initializer) error { return s.world.Draw() }
	commands := func(s *sectioned, ctx game.Initializer) error { return ctx.Commands() }
	for name, tc := range map[string]struct {
		do     part
		wants  string // the section it belongs in
		kinds  bool   // done in Kinds, a wrong place, rather than in Effects
		panics bool
	}{
		"a plugin used":         {do: useAPlugin, wants: "Plugins"},
		"the world given roles": {do: plays, wants: "Rules", kinds: true, panics: true},
		"an effect defined":     {do: effectDefined, wants: "Effects", kinds: true, panics: true},
		"a kind defined":        {do: kindDefined, wants: "Kinds", panics: true},
		"drawing rules given":   {do: draw, wants: "Looks"},
		"commands handed":       {do: commands, wants: "Commands"},
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
			chain := stage.New("meadow").Plugins(s.useWorld)
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
			st = stage.New("meadow").Plugins(func(ctx game.Initializer) error {
				s.useWorld(ctx)
				return tc.do(s, ctx)
			}).Update(s.update)
			if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
				t.Errorf("in Plugins: Init = %v, want it taken", err)
			}
		})
	}
}

// Units seeded in Layout are refused; in Units they are taken. The engine tells the section it is
// in to whoever asks.
func TestStage_SeedsInTheirSections(t *testing.T) {
	s := &sectioned{}
	var during section.Part
	st := stage.New("meadow").Plugins(s.useWorld).Kinds(s.defineUnit).
		Layout(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			s.world.Seed(s.unit.Entry(struct{}{}))
			return nil
		}).Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err == nil || !strings.Contains(err.Error(), "it belongs in Units") {
		t.Errorf("units seeded in Layout: Init = %v, want them refused as belonging in Units", err)
	}
	s = &sectioned{}
	st = stage.New("meadow").
		Plugins(func(ctx game.Initializer) { s.useWorld(ctx); during = ctx.(section.Reader).Section() }).
		Kinds(s.defineUnit).
		Units(func() { s.world.Seed(s.unit.Entry(struct{}{})) }).Update(s.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Errorf("units seeded in Units: Init = %v", err)
	}
	if during != section.Plugins {
		t.Errorf("in Plugins the engine says %v", during)
	}
}

// The board and the players refuse what is theirs out of its section: a kind of cell outside
// Cells, a board seeded outside Layout, a player added outside Players, keys bound outside
// Players and Controls.
func TestStage_ThePluginsRefuseTheirsOutOfItsSection(t *testing.T) {
	type built struct {
		sectioned
		board   *board.Plugin
		players *players.Plugin
		player  *players.Player
	}
	plugins := func(b *built) func(ctx game.Initializer) error {
		return func(ctx game.Initializer) error {
			b.useWorld(ctx)
			b.board = board.NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32), &cell.MultipleOccupancy{}, b.world)
			b.players = players.NewPlugin(b.world)
			if err := ctx.Use(b.board); err != nil {
				return err
			}
			return ctx.Use(b.players)
		}
	}
	grass := cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land}
	for name, tc := range map[string]struct {
		in    func(b *built) error // done in Effects, a wrong place
		wants string
	}{
		"a kind of cell created": {func(b *built) error { b.board.CellKinds().Create(grass); return nil }, "Cells"},
		"the board seeded":       {func(b *built) error { b.board.Seed(board.Layout{}); return nil }, "Layout"},
		"a player added":         {func(b *built) error { b.players.Add("late"); return nil }, "Players"},
		"keys bound":             {func(b *built) error { return b.player.Bind() }, "Players"},
	} {
		t.Run(name, func(t *testing.T) {
			b := &built{}
			st := stage.New("meadow").Plugins(plugins(b)).
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
	st := stage.New("meadow").Plugins(plugins(b)).
		Players(func() { b.player = b.players.Add("ai") }).
		Cells(func() { b.board.CellKinds().Create(grass) }).
		Controls(func() error { return b.player.Bind() }).
		Layout(func() { b.board.Seed(board.Layout{Default: "grass"}) }).
		Update(b.update)
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Errorf("each in its section: Init = %v", err)
	}
}
