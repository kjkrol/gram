package selection_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// soldier is a roles test's unit: its kind (which roles it plays), whose it is, whether it is selected.
type soldier struct {
	name     string
	kind     string
	x        float64
	by       control.PlayerID
	selected bool
}

// squad is a stage of world, selection and players whose kinds play roles — scouts hasty, guards
// mortal, veterans both, peasants none — run through the plugins' Install, Setup and RunPlan.
type squad struct {
	t             *testing.T
	w             *world.Plugin
	sel           *selection.Plugin
	players       *players.Plugin
	me, rival     *players.Player
	haste, rally  effect.Effect
	names         map[effect.Effect]string // the effects' names, for messages
	mortal, hasty *rule.Part
	kinds         map[string]kind.Of[soldier]
	ecs           *goke.ECS
	base          goke.Comp[world.Base]
	q             *goke.Query
	soldiers      []soldier
}

func newSquad(t *testing.T) *squad {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 10, MaxSize: 10},
	})
	sel := selection.NewPlugin(w)
	s := &squad{t: t, w: w, sel: sel, players: players.NewPlugin(w, sel), kinds: map[string]kind.Of[soldier]{}}
	s.me, s.rival = s.players.Local("me"), s.players.Add("rival")
	w.Effects().Define("haste", effect.Spec{effect.Lasts(time.Hour)})
	s.haste = w.Effects().Named("haste")
	w.Effects().Define("rally", effect.Spec{effect.Lasts(time.Hour)})
	s.rally = w.Effects().Named("rally")
	s.names = map[effect.Effect]string{s.haste: "haste", s.rally: "rally"}
	w.Roles().Define("mortal")
	w.Roles().Define("hasty")
	s.mortal, s.hasty = w.Roles().Named("mortal"), w.Roles().Named("hasty")
	tags := sel.Tags()
	define := func(name string, roles ...*rule.Part) {
		spec := kind.Spec{
			comp.Load(func(so soldier) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(so.x, 100), 10, 10)}
			}),
			comp.Const(world.Velocity{}),
			comp.Load(func(so soldier) tag.Tags[selection.Family] {
				t := tag.Tags[selection.Family](0).With(tags.Selectable)
				if so.selected {
					t = t.With(tags.Selected)
				}
				return t
			}),
			comp.Load(func(so soldier) tag.Tags[owner.Family] { return tag.Tags[owner.Family](0).With(owner.Of(so.by)) }),
		}
		if len(roles) > 0 {
			spec = append(spec, rule.Plays(roles...))
		}
		s.kinds[name] = kind.Define[soldier](w.Kinds(), name, spec)
	}
	define("scout", s.hasty)
	define("guard", s.mortal)
	define("veteran", s.mortal, s.hasty)
	define("peasant")
	return s
}

// start binds bindings for me, seeds soldiers and installs the plugins — call once.
func (s *squad) start(soldiers []soldier, bindings ...control.Binding) {
	s.t.Helper()
	if err := s.me.Bind(bindings...); err != nil {
		s.t.Fatal(err)
	}
	s.soldiers = soldiers
	for _, so := range soldiers {
		s.w.Seed(s.kinds[so.kind].Entry(so))
	}
	if err := s.w.Populate(); err != nil {
		s.t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{s.w, s.sel, s.players} {
		if err := p.Install(ctx); err != nil {
			s.t.Fatal(err)
		}
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) { s.q = si.NewQueryBuilder(&s.base).Build() }})
	s.ecs = ctx.ecs
	s.ecs.Setup(systems...)
	s.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		s.sel.RunPlan(rc, d)
		s.w.RunPlan(rc, d)
		s.w.Clock().Replay(rc, d)
		rc.Sync()
		s.players.RunPlan(rc, d)
	})
}

// issue gives cmd as me and ticks it through.
func (s *squad) issue(cmd any) {
	s.t.Helper()
	if err := s.players.Issue(s.me, cmd); err != nil {
		s.t.Fatal(err)
	}
	s.tick()
}

// press presses key at the keyboard and ticks what it issued through.
func (s *squad) press(key control.Key) {
	events := &control.InputEvents{}
	events.AddKeyEvent(key, control.ActionPress)
	s.players.EventHandler().HandleEvents(events)
	s.tick()
}

func (s *squad) tick() {
	for range 2 {
		s.ecs.Tick(time.Second / 10)
	}
}

// under is the names of the soldiers e is on.
func (s *squad) under(e effect.Effect) map[string]bool {
	byX := map[float64]uid.UID64{}
	for s.q.All(); s.q.Next(); {
		cur := s.q.Cursor()
		for i, id := range cur.IDs {
			byX[s.base.Slice(cur)[i].Pos.TopLeft.X] = id
		}
	}
	on := map[string]bool{}
	for _, so := range s.soldiers {
		id, ok := byX[so.x]
		if !ok {
			s.t.Fatalf("soldier %s was not spawned", so.name)
		}
		if e.On(id) {
			on[so.name] = true
		}
	}
	return on
}

// expect fails for every soldier e is on and not listed in want, or listed and not under it.
func (s *squad) expect(e effect.Effect, want ...string) {
	s.t.Helper()
	got := s.under(e)
	wanted := map[string]bool{}
	for _, name := range want {
		wanted[name] = true
	}
	for _, so := range s.soldiers {
		if got[so.name] != wanted[so.name] {
			s.t.Errorf("%s: under %s %v, want %v", so.name, s.names[e], got[so.name], wanted[so.name])
		}
	}
}

// soldiers of me and of the rival: every kind selected, a scout and a veteran not, and the rival's
// scout and veteran selected.
func roster(me, rival control.PlayerID) []soldier {
	return []soldier{
		{name: "my scout", kind: "scout", x: 100, by: me, selected: true},
		{name: "my guard", kind: "guard", x: 150, by: me, selected: true},
		{name: "my veteran", kind: "veteran", x: 200, by: me, selected: true},
		{name: "my peasant", kind: "peasant", x: 250, by: me, selected: true},
		{name: "my idle scout", kind: "scout", x: 300, by: me},
		{name: "my idle veteran", kind: "veteran", x: 350, by: me},
		{name: "the rival's scout", kind: "scout", x: 400, by: rival, selected: true},
		{name: "the rival's veteran", kind: "veteran", x: 450, by: rival, selected: true},
	}
}

// A command for the selected, no role named, casts on every selected unit of mine whatever it
// plays — none too.
func TestSelected_WithoutARoleIsEverySelectedUnitOfMine(t *testing.T) {
	s := newSquad(t)
	s.start(roster(s.me.ID, s.rival.ID))
	s.issue(rule.Cast(s.haste).On(s.sel.Selected()))
	s.expect(s.haste, "my scout", "my guard", "my veteran", "my peasant")
}

// A command for the selected playing a role casts on my selected units playing it alone: not on
// those playing another role or none, not on my unselected ones, not on the rival's.
func TestSelected_OfARoleIsMySelectedUnitsPlayingIt(t *testing.T) {
	for _, c := range []struct {
		name  string
		role  func(s *squad) *rule.Part
		under []string
	}{
		{"hasty", func(s *squad) *rule.Part { return s.hasty }, []string{"my scout", "my veteran"}},
		{"mortal", func(s *squad) *rule.Part { return s.mortal }, []string{"my guard", "my veteran"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newSquad(t)
			s.start(roster(s.me.ID, s.rival.ID))
			s.issue(rule.Cast(s.haste).On(s.sel.Selected(c.role(s))))
			s.expect(s.haste, c.under...)
		})
	}
}

// A command bound to a key is given at the key: on my selected units playing its role alone.
func TestSelected_AKeyGivesTheCommand(t *testing.T) {
	s := newSquad(t)
	s.start(roster(s.me.ID, s.rival.ID),
		control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts", rule.Cast(s.haste).On(s.sel.Selected(s.hasty))),
		control.Give(control.KeyPress{Key: control.KeyK}, "Rally the selected mortals", rule.Cast(s.rally).On(s.sel.Selected(s.mortal))))
	s.press(control.KeyJ)
	s.expect(s.haste, "my scout", "my veteran")
	s.expect(s.rally)
	s.press(control.KeyK)
	s.expect(s.rally, "my guard", "my veteran")
}

// Lift takes the effect off the selected; Toggle takes it off where any of them is under it and
// puts it on them all otherwise.
func TestSelected_LiftAndToggle(t *testing.T) {
	s := newSquad(t)
	s.start(roster(s.me.ID, s.rival.ID))
	s.issue(rule.Cast(s.haste).On(s.sel.Selected(s.hasty)))
	s.expect(s.haste, "my scout", "my veteran")
	s.issue(rule.Toggle(s.haste).On(s.sel.Selected())) // some are under it: off them all
	s.expect(s.haste)
	s.issue(rule.Toggle(s.haste).On(s.sel.Selected())) // none is: on them all
	s.expect(s.haste, "my scout", "my guard", "my veteran", "my peasant")
	s.issue(rule.Lift(s.haste).On(s.sel.Selected(s.mortal)))
	s.expect(s.haste, "my scout", "my peasant")
}

// A command for the one pointed at reaches the entity drawn under the cursor its key was pressed
// with, whoever's it is and selected or not; pointing at nobody, or given without a cursor, it
// reaches no one.
func TestPointed_IsTheEntityUnderTheCursor(t *testing.T) {
	s := newSquad(t)
	s.start(roster(s.me.ID, s.rival.ID))
	cure := control.Give(control.KeyPress{Key: control.KeyC}, "Hasten the one pointed at", rule.Cast(s.haste).On(s.sel.Pointed()))
	point := func(x, y float64) {
		cmd, ok := cure.Build(control.Context{Player: s.me.ID, Camera: s.w.Res.Camera, Cursor: geom.NewVec(x, y)})
		if !ok {
			t.Fatal("the binding built nothing")
		}
		s.issue(cmd)
	}
	point(600, 600) // empty ground
	s.expect(s.haste)
	s.issue(rule.Cast(s.haste).On(s.sel.Pointed())) // no cursor
	s.expect(s.haste)
	point(405, 105) // the rival's scout, at 400
	s.expect(s.haste, "the rival's scout")
	point(305, 105) // my idle scout, at 300
	s.expect(s.haste, "the rival's scout", "my idle scout")
}
