package board_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

const (
	campKind     = "camp"
	campPartKind = "camp part"
	campRole     = "camp"
	campPartRole = "camp part"
	// how long the camp's sector lies at L4 before it folds
	campFolds = 500 * time.Millisecond
)

type campRow struct{ at cell.ID }

type campPartRow struct{}

// camped is a camp, an aggregate unit given no order on a board of single occupancy cut into
// sectors, unfolding as its one camera comes near into three parts in a row a cell apart, the
// middle one on the camp's own cell, and folding as the camera goes
type camped struct {
	g     grid.Grid
	w     *world.Plugin
	occ   cell.Occupancy
	cam   camera.Camera
	ecs   *goke.ECS
	start cell.ID

	q      *goke.Query
	base   goke.Comp[world.Base]
	at     goke.Comp[unit.At]
	partOf goke.OptComp[world.PartOf]
	open   goke.OptComp[world.Unfolded]
}

func (c *camped) Observers(each func(cam camera.Camera)) { each(c.cam) }

func (c *camped) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		c.q = si.NewQueryBuilder(&c.base, &c.at).Optional(&c.partOf, &c.open).Build()
	}}}
}

func newCamped(t *testing.T) *camped {
	t.Helper()
	const cols, rows = 32, 16
	c := &camped{g: grid.DefaultGrids{}.Square(cols, rows, boardtest.CellSize)}
	w, h := uint32(cols*boardtest.CellSize), uint32(rows*boardtest.CellSize)
	c.start = c.g.At(16, 8)
	c.cam = cameratest.NewFromSpace(w, h, 0)
	c.cam.SetViewport(100, 100)
	c.look(1)
	var m *motion.Plugin
	var b *board.Plugin
	st := stage.New("camp").
		World(world.Config{
			Space:    world.SpaceCfg{Width: w, Height: h},
			Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: boardtest.CellSize},
			Heights:  true,
			Sectors:  world.SectorsCfg{Buckets: 2},
			LOD: lod.Ladder{
				Scales: [lod.Levels - 1]float32{0, 0, 0.8, 0.4},
				Fold:   float32(campFolds.Seconds()),
			},
		}).
		Plugins(func(wp *world.Plugin) {
			c.w = wp
			m = motion.NewPlugin(wp)
			b = board.NewPlugin(c.g, &cell.SingleOccupancy{}, wp).
				WithMotion(m, cell.DefaultHiking)
			b.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
			c.occ = b.Write().Occupancy()
			wp.Write().Observe(c)
		}).
		Effects(func(ctx game.Initializer) {
			c.ecs = ctx.ECS()
			ctx.Setup(c)
		}).
		Rules(func() {
			var parts []act.Step
			for _, dx := range []float32{-boardtest.CellSize, 0, boardtest.CellSize} {
				e := kind.EntryOf(campPartKind, campPartRow{})
				parts = append(parts, act.Self.Do(world.SpawnPart{Entry: e,
					At: geom.NewVec(dx, 0)}))
			}
			c.w.Roles().Define(campRole, act.Branch[world.Unfold]{
				Name: "pitch",
				When: cond.Self.If(cond.Always),
				Then: act.Steps(parts...),
			})
			c.w.Roles().Define(campPartRole, act.Branch[world.Folding]{
				Name: "strike",
				When: cond.Self.If(cond.Always),
				Then: act.Self.Do(world.Despawn{}),
			})
		}).
		Kinds(func() {
			profile := steering.Steering{MaxSpeed: 64}
			camps := board.NewUnits[campRow](b, board.Shape{Size: 10},
				func(r campRow) geom.Vec { return c.g.CellCenter(r.at) })
			camps.Define(campKind, unit.Mover{Domain: cell.Land}, profile,
				comp.Const(world.Aggregate{}),
				ceremony.Plays(c.w.Roles().Named(campRole)))
			parts := board.NewUnits[campPartRow](b, board.Shape{Size: 10},
				func(campPartRow) geom.Vec { return geom.Vec{} })
			parts.Define(campPartKind, unit.Mover{Domain: cell.Land}, profile,
				ceremony.Plays(c.w.Roles().Named(campPartRole)))
		}).
		Spawn(func() {
			c.w.Write().Seed(kind.Named[campRow](c.w.Kinds(), campKind).Entry(campRow{c.start}))
		}).
		Update(func(rc goke.RunCtx, d time.Duration) {
			c.w.RunPlan(rc, d)
			m.RunPlan(rc, d)
			b.RunPlan(rc, d)
			rc.Sync()
		})
	if err := engine.NewEngine(boardtest.OneStageGame{Stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return c
}

// look has the camera over the camp's start cell at zoom
func (c *camped) look(zoom float32) {
	c.cam.ZoomIn(zoom/c.cam.Zoom(), 0, 0)
	mid := c.g.CellCenter(c.start)
	c.cam.CenterOn(mid.X, mid.Y, 0)
}

func (c *camped) run(ticks int) {
	for range ticks {
		c.ecs.Tick(time.Second / 60)
	}
}

// one of the camp: its cell, the cell under its middle and whether it is Unfolded
type camper struct {
	id       uid.UID64
	at, on   cell.ID
	unfolded bool
}

// the camp and its parts, by the cells under them from west to east
func (c *camped) cast() (camp camper, parts []camper) {
	for c.q.All(); c.q.Next(); {
		cur := c.q.Cursor()
		bases, ats := c.base.Slice(cur), c.at.Slice(cur)
		for i, id := range cur.IDs {
			on, _ := c.g.CellAt(bases[i].Pos.Center())
			one := camper{id: id, at: ats[i].Cell, on: on, unfolded: c.open.Present(cur)}
			if c.partOf.Present(cur) {
				parts = append(parts, one)
			} else {
				camp = one
			}
		}
	}
	slices.SortFunc(parts, func(a, b camper) int { return int(a.on) - int(b.on) })
	return camp, parts
}

// the cells id holds
func (c *camped) held(id uid.UID64) []cell.ID {
	var cells []cell.ID
	c.g.EachCell(func(cl cell.ID) {
		c.occ.Holders(cl, func(e uid.UID64, _ cell.Domain, _ bool) {
			if e == id {
				cells = append(cells, cl)
			}
		})
	})
	return cells
}

func TestOccupancy_AnAggregateUnfoldedWithNoOrderHoldsNoCellTillItFolds(t *testing.T) {
	c := newCamped(t)
	c.run(4)
	camp, parts := c.cast()
	if !camp.unfolded || len(parts) != 3 {
		t.Fatalf("the camp unfolded %v into %d parts, want three", camp.unfolded, len(parts))
	}
	if held := c.held(camp.id); len(held) != 0 {
		t.Errorf("unfolded, the camp holds %v, want none", held)
	}
	west, _ := grid.Direction(c.g, -1, 0)
	east, _ := grid.Direction(c.g, 1, 0)
	before, _ := c.g.Toward(c.start, west)
	after, _ := c.g.Toward(c.start, east)
	for k, want := range []cell.ID{before, c.start, after} {
		p := parts[k]
		if held := c.held(p.id); p.on != want || p.at != want || !slices.Equal(held,
			[]cell.ID{want}) {
			t.Errorf("part %d stands on %v at %v holding %v, want on cell %v, holding it", k,
				p.on, p.at, held, want)
		}
	}

	c.look(0.25)
	for range 2*int(campFolds/(time.Second/60)) + 4 {
		c.run(1)
		if _, parts = c.cast(); len(parts) == 0 {
			break
		}
	}
	c.run(2)
	camp, parts = c.cast()
	if camp.unfolded || len(parts) != 0 {
		t.Fatalf("at L4, the camp Unfolded %v with %d parts, want it folded, none", camp.unfolded,
			len(parts))
	}
	if held := c.held(camp.id); camp.on != c.start || !slices.Equal(held, []cell.ID{c.start}) {
		t.Errorf("folded on %v, the camp holds %v, want its start cell %v again", camp.on, held,
			c.start)
	}
}
