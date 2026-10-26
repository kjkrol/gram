package vision_test

import (
	"io"
	"log"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type oneStage struct {
	stage game.Stage
	props game.Props
}

func (g oneStage) Props() game.Props { return g.props }
func (g oneStage) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

// lookout is a Stage of observers looking east and what they look at, entered by the engine as
// a game's is
type lookout struct {
	w       *world.Plugin
	v       *vision.Plugin
	ecs     *goke.ECS
	noticed effect.Effect

	base    goke.Comp[world.Base]
	sighted goke.Comp[vision.Sighted]
	query   *goke.Query

	unread goke.Runnable
	off    bool // every observer's Watched and roles taken off at each step
}

// group is a kind of a lookout's entities: what it carries, given once the roles are defined,
// and where each stands
type group struct {
	name  string
	looks bool
	with  func(l *lookout) []comp.Comp
	rows  []spawn
}

func newLookout(t *testing.T, props game.Props, plugin func(*vision.Plugin), roles func(*lookout),
	groups ...group) *lookout {
	t.Helper()
	l := &lookout{}
	st := stage.New("lookout").
		World(world.Config{
			Space:    world.SpaceCfg{Width: 2000, Height: 2000},
			Entities: world.EntitiesCfg{MaxCount: 256, MinSize: 1, MaxSize: 100},
		}).
		Plugins(func(w *world.Plugin) {
			l.w, l.v = w, vision.NewPlugin(w)
			if plugin != nil {
				plugin(l.v)
			}
		}).
		Effects(func(ctx game.Initializer) {
			l.ecs = ctx.ECS()
			ctx.Setup(l)
			l.unread = ctx.RegSys(unreading)
			l.w.Write().Effects().Define("noticed", effect.Spec{})
			l.noticed = l.w.Write().Effects().Named("noticed")
		}).
		Rules(func() {
			if roles != nil {
				roles(l)
			}
		}).
		Kinds(func() {
			for _, g := range groups {
				spec := kind.Spec{comp.Load(at), comp.Const(world.Velocity{})}
				if g.looks {
					spec = append(spec,
						comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 600}),
						comp.Const(world.Eye{Angle: math.Pi / 2}),
						comp.Const(vision.Sighted{}))
				}
				if g.with != nil {
					spec = append(spec, g.with(l)...)
				}
				kind.Define[spawn](l.w.Kinds(), g.name, spec)
			}
		}).
		Spawn(func() {
			// one of each group in turn, so the groups mix in the blocks they share
			for i := 0; ; i++ {
				seeded := false
				for _, g := range groups {
					if i < len(g.rows) {
						l.w.Write().Seed(kind.Named[spawn](l.w.Kinds(), g.name).Entry(g.rows[i]))
						seeded = true
					}
				}
				if !seeded {
					return
				}
			}
		}).
		Update(func(ctx goke.RunCtx, d time.Duration) {
			if l.off {
				ctx.Run(l.unread, d)
			}
			l.w.RunPlan(ctx, d)
			l.v.RunPlan(ctx, d)
		})
	if err := engine.NewEngine(oneStage{stage: st, props: props}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return l
}

func (l *lookout) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		l.query = si.NewQueryBuilder(&l.base, &l.sighted).Build()
	}}}
}

// unreading takes Watched off every observer and has each give up its roles, which leaves it in
// its block
func unreading() goke.System {
	var marked, playing *goke.Query
	var watched goke.CompID
	var roles goke.Comp[tag.Tags[cond.Roles]]
	return goke.SystemFn{
		OnInit: func(si *goke.SysInit) {
			marked = si.NewQueryBuilder().Include(goke.Include[vision.Watched]()).Build()
			playing = si.NewQueryBuilder(&roles).Include(goke.Include[vision.Sight]()).Build()
			watched = si.RegComp[vision.Watched]()
		},
		OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
			for marked.All(); marked.Next(); {
				for _, id := range marked.Cursor().IDs {
					cb.RemoveCompOne(id, watched)
				}
			}
			for playing.All(); playing.Next(); {
				clear(roles.Slice(playing.Cursor()))
			}
		},
	}
}

func (l *lookout) tick(n int) {
	for range n {
		l.ecs.Tick(time.Second / 60)
	}
}

func (l *lookout) seen() map[uid.UID64]vision.Sighted {
	out := map[uid.UID64]vision.Sighted{}
	for l.query.All(); l.query.Next(); {
		cursor := l.query.Cursor()
		for i, id := range cursor.IDs {
			out[id] = l.sighted.Slice(cursor)[i]
		}
	}
	return out
}

func (l *lookout) kinds() map[uid.UID64]kind.ID {
	out := map[uid.UID64]kind.ID{}
	for l.query.All(); l.query.Next(); {
		cursor := l.query.Cursor()
		for i, id := range cursor.IDs {
			out[id] = l.base.Slice(cursor)[i].TypeID
		}
	}
	return out
}

func (l *lookout) notice() act.Rule {
	return act.Branch[vision.Sighting]{
		Name: "notice",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Put(l.noticed),
	}
}

func carries[T any](v T) func(*lookout) []comp.Comp {
	return func(*lookout) []comp.Comp { return []comp.Comp{comp.Const(v)} }
}

func plays(role string) func(*lookout) []comp.Comp {
	return func(l *lookout) []comp.Comp {
		return []comp.Comp{ceremony.Plays(l.w.Roles().Named(role))}
	}
}

func TestWatched_OnlyAnObserverSomethingReadsIsScanned(t *testing.T) {
	quiet := log.New(io.Discard, "", 0)
	for name, c := range map[string]struct {
		plugin func(*vision.Plugin)
		roles  func(*lookout)
		with   func(*lookout) []comp.Comp
		target func(*lookout) []comp.Comp
		want   uint8
	}{
		"nothing reads it":        {},
		"it carries Watched":      {with: carries(vision.Watched{}), want: 1},
		"it carries SightOutline": {with: carries(vision.SightOutline{}), want: 1},
		"a rule of a role it plays": {
			roles: func(l *lookout) { l.w.Roles().Define("watcher", l.notice()) },
			with:  plays("watcher"),
			want:  1,
		},
		"a rule of a role only the target plays": {
			roles:  func(l *lookout) { l.w.Roles().Define("watcher", l.notice()) },
			target: plays("watcher"),
		},
		"the plugin scans all": {
			plugin: func(v *vision.Plugin) { v.WithScanAll() },
			want:   1,
		},
		"the plugin logs, which reads nobody": {
			plugin: func(v *vision.Plugin) { v.WithLog(quiet) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			l := newLookout(t, game.Props{}, c.plugin, c.roles,
				group{name: "observer", looks: true, with: c.with, rows: []spawn{{x: 500, y: 500}}},
				group{name: "target", with: c.target, rows: []spawn{{x: 700, y: 500}}})
			l.tick(3)
			seen := l.seen()
			if len(seen) != 1 {
				t.Fatalf("%d observers, want 1", len(seen))
			}
			for _, s := range seen {
				if s.Count != c.want {
					t.Errorf("the observer sees %d with the target in its cone, want %d",
						s.Count, c.want)
				}
			}
		})
	}
}

func TestWatched_AnObserverNoLongerReadSeesNothing(t *testing.T) {
	for name, c := range map[string]struct {
		roles func(*lookout)
		with  func(*lookout) []comp.Comp
	}{
		"Watched taken off, no rule hosted": {with: carries(vision.Watched{})},
		"its role given up": {
			roles: func(l *lookout) { l.w.Roles().Define("watcher", l.notice()) },
			with:  plays("watcher"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			l := newLookout(t, game.Props{}, nil, c.roles,
				group{name: "observer", looks: true, with: c.with, rows: []spawn{{x: 500, y: 500}}},
				group{name: "target", rows: []spawn{{x: 700, y: 500}}})
			count := func() uint8 {
				for _, s := range l.seen() {
					return s.Count
				}
				return 0
			}
			l.tick(3)
			if n := count(); n != 1 {
				t.Fatalf("read, the observer sees %d, want the target", n)
			}
			l.off = true
			for step := range 4 {
				l.tick(1)
				if n := count(); step > 0 && n != 0 {
					t.Errorf("%d steps after it was last read the observer sees %d, want nothing",
						step+1, n)
				}
			}
		})
	}
}

func TestWatched_ObserversReadAmongOthersScannedTogetherSeeWhatTheyDoOneByOne(t *testing.T) {
	var watchers, idlers, targets []spawn
	for i := range 24 {
		y := float32(20 + 60*i)
		watchers = append(watchers, spawn{x: 100, y: y})
		idlers = append(idlers, spawn{x: 100, y: y + 30})
		targets = append(targets, spawn{x: 300, y: y + 15})
	}
	roles := func(l *lookout) {
		l.w.Roles().Define("watcher", l.notice())
		l.w.Roles().Define("idler")
	}
	// the first step, while the two kinds share their blocks: what the watchers' rule puts on
	// them moves them to blocks of their own by the next
	run := func(workers int) (map[uid.UID64]vision.Sighted, *lookout) {
		l := newLookout(t, game.Props{Workers: 4},
			func(v *vision.Plugin) { v.WithWorkers(workers) }, roles,
			group{name: "watcher", looks: true, with: plays("watcher"), rows: watchers},
			group{name: "idler", looks: true, with: plays("idler"), rows: idlers},
			group{name: "target", rows: targets})
		l.tick(1)
		seen := l.seen()
		l.tick(1)
		return seen, l
	}
	alone, _ := run(1)
	together, l := run(0)
	if len(together) != 48 || len(alone) != 48 {
		t.Fatalf("%d observers scanned together, %d one by one, want 48", len(together), len(alone))
	}
	watcher, _, _, _ := l.w.Kinds().Lookup("watcher")
	kinds, saw := l.kinds(), 0
	for id, s := range together {
		if s != alone[id] {
			t.Errorf("observer %v saw %v together, %v one by one",
				id, sighted(s), sighted(alone[id]))
		}
		read := kinds[id] == watcher
		if noticed := l.w.Read(0).Effects().Has(id, l.noticed); noticed != read {
			t.Errorf("observer %v of kind %v noticed %v, want the watchers alone to",
				id, kinds[id], noticed)
		}
		if !read && s.Count != 0 {
			t.Errorf("an idler saw %d, want nothing: nothing reads it", s.Count)
		}
		if read {
			saw += int(s.Count)
		}
	}
	if saw == 0 {
		t.Error("no watcher saw anything: the scene proves nothing")
	}
}
