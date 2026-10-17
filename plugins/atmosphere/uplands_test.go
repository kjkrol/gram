package atmosphere_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

// a board 640 by 480 of hills under low grey clouds, damp and still; a hill's rule puts on what
// shower says as the clouds over it come to cover covered
type uplands struct {
	w      *world.Plugin
	a      *atmosphere.Plugin
	b      *board.Plugin
	ecs    *goke.ECS
	eng    *engine.Engine
	shower effect.Effect
	heard  []weathering.Overhead
	wrung  goke.Comp[weathering.Wring]
	cells  *goke.Query
}

func (u *uplands) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		u.cells = si.NewQueryBuilder(&u.wrung).Build()
	}}}
}

// falls is how many of the cells' Wring let rain fall
func (u *uplands) falls() (n int) {
	for u.cells.All(); u.cells.Next(); {
		for _, w := range u.wrung.Slice(u.cells.Cursor()) {
			if w.Falls > 0 {
				n++
			}
		}
	}
	return n
}

func newUplands(t *testing.T, covered float32, shower effect.Spec) *uplands {
	t.Helper()
	return uplandsFrom(t, covered, shower, false)
}

// uplandsFrom is newUplands, read back from the save in the working directory when load
func uplandsFrom(t *testing.T, covered float32, shower effect.Spec, load bool) *uplands {
	t.Helper()
	u := &uplands{}
	grey := weather.State{Name: "grey", Clouds: [2]float32{0.9, 0.9}, Billow: [2]float32{1, 1},
		Damp: 0.8, Lasts: [2]time.Duration{time.Hour, time.Hour}}
	st := stage.New("uplands").
		World(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true}).
		Plugins(func(made *world.Plugin) {
			u.w = made
			u.b = board.NewPlugin(grid.DefaultGrids{}.Square(20, 15, 32), &cell.MultipleOccupancy{},
				made)
			u.a = atmosphere.NewPlugin(made, atmosphere.Config{
				Climate: climate.Config{Weathers: []weather.State{grey}, Start: "grey"},
			}).WithWeathering(u.b, weathering.Config{})
		}).
		Effects(func(ctx game.Initializer) {
			u.ecs = ctx.ECS()
			ctx.Setup(u)
			u.w.Effects().Define("shower", shower)
			u.shower = u.w.Effects().Named("shower")
		}).
		Rules(func() {
			u.w.Roles().Define("hill",
				act.Branch[weathering.Overhead]{
					Name: "shower",
					When: cond.Self.If(weathering.Clouded(covered)),
					Then: act.Self.Put(u.shower),
				},
				act.Branch[weathering.Overhead]{
					Name: "hear",
					When: cond.Self.If(cond.NewWord("heard", func(o weathering.Overhead) bool {
						u.heard = append(u.heard, o)
						return false
					})),
					Then: act.Self.Put(u.shower),
				})
		}).
		Kinds(func() {
			u.b.CellKinds().Define("hill", cell.Kind{Cost: 1, Allows: cell.Land},
				u.w.Roles().Named("hill"))
		}).
		Spawn(func() error {
			u.b.Seed(board.Layout{Default: "hill"})
			return u.b.Populate()
		}).
		Update(func(ctx goke.RunCtx, d time.Duration) {
			u.w.RunPlan(ctx, d)
			u.b.RunPlan(ctx, d)
			u.a.RunPlan(ctx, d)
		})
	u.eng = engine.NewEngine(oneStage{stage: st})
	if load {
		if err := u.eng.Load("", ""); err != nil {
			t.Fatalf("Load: %v", err)
		}
	}
	if err := u.eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return u
}

func (u *uplands) run(d time.Duration) {
	for range int(d / (time.Second / 60)) {
		u.ecs.Tick(time.Second / 60)
	}
}

func TestWeathering_TellsEveryCellOfTheAirOverItOnceASecond(t *testing.T) {
	u := newUplands(t, 2, effect.Spec{}) // no cover is twice the sky: the shower never comes
	u.run(time.Second / 2)
	u.heard = nil
	u.run(time.Second)
	cells := map[cell.ID]int{}
	for _, o := range u.heard {
		cells[o.Cell]++
	}
	if len(cells) != 20*15 {
		t.Fatalf("in a second %d of the 300 cells heard of the air over them, want every one",
			len(cells))
	}
	for c, n := range cells {
		if n > 2 {
			t.Errorf("in a second the cell %v heard of the air over it %d times, want once", c, n)
		}
	}
	o := u.heard[0]
	if o.Low < 0.85 || o.Air.Damp < 0.75 || o.ID == 0 || o.Reach != 0 {
		t.Errorf("a cell heard %+v, want the grey's low clouds 0.9, damp 0.8, its entity, its "+
			"flat ground reaching nowhere", o)
	}
}

func TestWeathering_ARuleOfACellWringsRainOutOfTheAirOverIt(t *testing.T) {
	u := newUplands(t, 0.5, effect.Spec{effect.Lasts(2 * time.Second),
		effect.Alter(func(w *weathering.Wring) { w.Falls = 1 })})
	u.run(time.Second / 60)
	changes := u.b.Board().Changes()
	u.run(6 * time.Second)
	if got := u.b.Board().Changes(); got != changes {
		t.Errorf("the hills' showers changed the board %d times, want none: they turn the air's "+
			"knob alone", got-changes)
	}
	c, _ := u.b.Board().CellAt(geom.NewVec(320, 240))
	id, _ := u.b.CellEntity(c)
	if !u.w.Effects().Has(id, u.shower) {
		t.Fatal("under low clouds 0.9 the hill's shower is not on")
	}
	if rain, _ := u.a.Air().FallsAt(320, 240); rain < 0.5 {
		t.Errorf("a hill wringing all of the low clouds over it: it rains %v there, want most of "+
			"them", rain)
	}
}

func TestSpread_AnEffectTurnsHowMuchOfTheWorldTheRainTakes(t *testing.T) {
	var w *world.Plugin
	var a *atmosphere.Plugin
	var ecs *goke.ECS
	var wide effect.Effect
	st := stage.New("wide rain").
		World(world.Config{Space: world.SpaceCfg{Width: 6400, Height: 3200},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20}, Heights: true}).
		Plugins(func(made *world.Plugin) {
			w = made
			a = atmosphere.NewPlugin(made, atmosphere.Config{
				Climate: climate.Config{Start: weather.Fair, Seed: 3}})
		}).
		Effects(func(ctx game.Initializer) {
			ecs = ctx.ECS()
			w.Effects().Define("wide rain", effect.Spec{
				effect.Alter(func(s *climate.Spread) { s.Rain = 0.5 })})
			wide = w.Effects().Named("wide rain")
		}).
		Update(func(ctx goke.RunCtx, d time.Duration) {
			w.RunPlan(ctx, d)
			a.RunPlan(ctx, d)
		})
	if err := engine.NewEngine(oneStage{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	tick := func(d time.Duration) {
		for range int(d / (time.Second / 60)) {
			ecs.Tick(time.Second / 60)
		}
	}
	tick(time.Second / 10)
	w.Effects().Cast(a.Entity(), wide)
	tick(time.Second / 10)
	a.Climate().Set(weather.Rain)
	tick(12 * time.Second)
	raining, n := 0, 0
	for x := float32(25); x < 6400; x += 50 {
		for y := float32(25); y < 3200; y += 50 {
			n++
			if a.Air().At(x, y).Falls > 0.3 {
				raining++
			}
		}
	}
	if share := float32(raining) / float32(n); share < 0.45 || share > 0.51 {
		t.Errorf("the spread turned to rain on half the world: it rains on %.2f of it", share)
	}
}

func TestWeathering_TheWringOutlastsASave(t *testing.T) {
	t.Chdir(t.TempDir())
	shower := effect.Spec{effect.Lasts(2 * time.Second),
		effect.Alter(func(w *weathering.Wring) { w.Falls = 1 })}
	u := newUplands(t, 0.5, shower)
	u.run(2 * time.Second)
	n := u.falls()
	if n == 0 {
		t.Fatal("no hill's shower is on under low clouds 0.9")
	}
	if err := u.eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back := uplandsFrom(t, 2, shower, true) // its hills shower no more: the save's shower ends
	back.run(time.Second / 60)
	if got := back.falls(); got != n {
		t.Errorf("read back from a save %d cells let rain fall, want the %d saved", got, n)
	}
	back.run(3 * time.Second)
	if got := back.falls(); got != 0 {
		t.Errorf("the showers over since the load, %d cells let rain fall, want none", got)
	}
}
