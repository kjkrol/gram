package engine

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type settleMark struct{}

// migrator moves one entity to an archetype of its own in its RunPlan, outside the clock, as
// navigation's MoveTo puts a MoveOrder on a unit.
type migrator struct {
	handle goke.Runnable
	query  *goke.Query
	base   goke.Comp[world.Base]
	mark   goke.CompID
	moved  bool
}

func (m *migrator) Name() string { return "test.migrator" }

func (m *migrator) Install(ctx plugin.Installer) error {
	m.handle = ctx.RegSys(func() goke.System {
		return goke.SystemFn{
			OnInit: func(si *goke.SysInit) {
				m.mark = si.RegComp[settleMark]()
				m.query = si.NewQueryBuilder(&m.base).Build()
			},
			// the middle one leaves its table, the last moving into its slot: a stale run reads
			// the zeroed last slot as id 0, the first one's id too, so the first one stays
			OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
				for m.query.All(); m.query.Next() && !m.moved; {
					cb.AddOne(m.query.Cursor().IDs[1], m.mark, settleMark{})
					m.moved = true
				}
			},
		}
	})
	return nil
}

func (m *migrator) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.handle, d)
	ctx.Sync()
}

func (m *migrator) EventHandler() control.EventHandler { return nil }
func (m *migrator) Serializable() plugin.Serializable  { return nil }

// misread is what the space gets wrong of the entities m's query walks
func (m *migrator) misread(space *aabbworld.Space) []string {
	var found, want []uid.UID64
	var errs []string
	for m.query.All(); m.query.Next(); {
		cursor := m.query.Cursor()
		for i, id := range cursor.IDs {
			want = append(want, id)
			box := m.base.Slice(cursor)[i].Pos.AABB.AABB
			found = found[:0]
			space.Query(box, aabbworld.AnyCapability,
				func(id uid.UID64) { found = append(found, id) })
			if !slices.Equal(found, []uid.UID64{id}) {
				errs = append(errs, fmt.Sprintf("at %v it finds %v, want %v alone", box, found, id))
			}
		}
	}
	var everywhere []uid.UID64
	space.Query(geom.NewAABBAt(geom.NewVec(0, 0), 1000, 1000), aabbworld.AnyCapability,
		func(id uid.UID64) { everywhere = append(everywhere, id) })
	slices.Sort(everywhere)
	slices.Sort(want)
	if !slices.Equal(everywhere, want) {
		errs = append(errs, fmt.Sprintf("over the world it finds %v, want each of %v once",
			everywhere, want))
	}
	return errs
}

func TestStage_ASyncOutsideTheClockLeavesTheSpaceRightForWhatReadsItNext(t *testing.T) {
	var w *world.Plugin
	m := &migrator{}
	var errs []string
	st := stage.New("meadow").
		World(world.Config{
			Space:    world.SpaceCfg{Width: 1000, Height: 1000},
			Entities: world.EntitiesCfg{MaxCount: 3, MinSize: 1, MaxSize: 10},
		}).
		Plugins(func(wp *world.Plugin) {
			w = wp
			w.Made().Add(m)
		}).
		Kinds(func() {
			kind.Define[geom.Vec](w.Kinds(), "thing", kind.Spec{
				comp.Load(func(at geom.Vec) world.Position {
					return world.Position{AABB: plane.NewAABB(at, 10, 10)}
				}),
				comp.Const(world.Velocity{}),
			})
		}).
		Spawn(func() {
			things := kind.Named[geom.Vec](w.Kinds(), "thing")
			for _, at := range []float64{100, 300, 500} {
				w.Write().Seed(things.Entry(geom.NewVec(at, at)))
			}
		}).
		Update(func(ctx goke.RunCtx, d time.Duration) {
			w.RunPlan(ctx, d)
			m.RunPlan(ctx, d)
			errs = m.misread(w.Write().Space())
		})
	eng := NewEngine(oneStageGame{stage: st})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	eng.current.host.ecs.Tick(time.Second / 60)

	if !m.moved {
		t.Fatal("the migrator moved nobody")
	}
	for _, err := range errs {
		t.Errorf("after a Sync outside the clock, the space read in the same tick: %s", err)
	}
}
