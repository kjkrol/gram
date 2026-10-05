package boardtest

import (
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
)

// InstallCtx is the plugin.Installer a Stage would hand over, minus the engine.
type InstallCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *InstallCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *InstallCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *InstallCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *InstallCtx) ECS() *goke.ECS                                  { return c.ecs }

// Mover is a unit's row: where it starts, where it keeps driving when heading is set, what it
// sees, and how it moves (zero: Land).
type Mover struct {
	Here    cell.ID
	Heading geom.Vec
	Sight   *vision.Sight
	Eye     world.Eye // how wide the sight sees, with it
	Domain  cell.Domain
	Offset  float64 // shifts the box right, to straddle two cells
	Brakes  bool    // standing, it brakes to rest as a unit does
}

// UnitSize is the side of a unit's box.
const UnitSize = 22

// World is world + collision + board + vision ticked as the demo ticks them.
type World struct {
	World *world.Plugin
	Board *board.Plugin
	ECS   *goke.ECS

	t        *testing.T
	base     goke.Comp[world.Base]
	sight    goke.OptComp[vision.Sighted]
	collider goke.OptComp[collision.Collider]
	q        *goke.Query
}

// NewWorld is a World over g, width x height, its terrain laid by terrain, units spawned as their
// rows say and rules hooked on the board or on collision.
func NewWorld(t *testing.T, g grid.Grid, width, height uint32, terrain func(*board.Board), units []Mover, rules ...rule.Rule) *World {
	t.Helper()
	return NewWorldWith(t, g, width, height, func(_ *world.Plugin, brd *board.Plugin) []rule.Rule {
		terrain(brd.Res.Logic.Board)
		return rules
	}, units)
}

// NewWorldWith is NewWorld with the world and the board handed to prepare before they are
// installed — to define roles and commands, seed a Layout and Populate it — and the rules it gives
// hooked on the board or on collision.
func NewWorldWith(t *testing.T, g grid.Grid, width, height uint32, prepare func(*world.Plugin, *board.Plugin) []rule.Rule, units []Mover) *World {
	t.Helper()
	bw := &World{t: t}
	bw.World = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: UnitSize, MaxSize: UnitSize},
	})
	c := collision.NewPlugin(bw.World)
	bw.Board = board.NewPlugin(g, &cell.MultipleOccupancy{}, bw.World).WithCollision(c)
	rules := prepare(bw.World, bw.Board)
	for _, b := range rules {
		err := bw.Board.Hook(b)
		if errors.Is(err, plugin.ErrUnhosted) {
			err = c.Hook(b)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	v := vision.NewPlugin(bw.World).WithBoard(bw.Board)

	ctx := NewInstallCtx()
	if err := bw.World.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := bw.Board.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatal(err)
	}

	for i, u := range units {
		spec := kind.Spec{
			comp.Load(func(m Mover) world.Position {
				box := CellBox(g, m.Here, UnitSize)
				box.TopLeft.X += m.Offset
				box.BottomRight.X += m.Offset
				return world.Position{AABB: box}
			}),
			comp.Const(world.Velocity{}),
			comp.Const(collision.Collider{}),
			comp.Const(collision.Physics{}),
			comp.Load(func(m Mover) unit.At { return unit.At{Cell: m.Here} }),
			comp.Load(func(m Mover) unit.Mover {
				if m.Domain == 0 {
					return unit.Mover{Domain: cell.Land}
				}
				return unit.Mover{Domain: m.Domain}
			}),
			comp.Load(func(m Mover) world.Layers { // as board.NewUnits gives them
				if m.Domain == 0 {
					return world.Layers(cell.Land)
				}
				return world.Layers(m.Domain)
			}),
		}
		switch {
		case u.Heading != (geom.Vec{}):
			spec = append(spec, comp.Const(steering.Steering{MaxSpeed: 64}), comp.Load(func(m Mover) steering.Course {
				return steering.Course{Want: m.Heading, WantSpeed: 64}
			}))
		case u.Brakes:
			spec = append(spec, comp.Const(steering.Steering{MaxSpeed: 64, Accel: 128, Brake: 256}))
		}
		if u.Sight != nil {
			spec = append(spec, comp.Const(*u.Sight), comp.Const(vision.Sighted{}), comp.Const(u.Eye))
		}
		name := string(rune('a' + i))
		bw.World.Seed(kind.Define[Mover](bw.World.Kinds(), name, spec).Entry(u))
	}
	if err := bw.World.Populate(); err != nil {
		t.Fatal(err)
	}

	systems := append(ctx.Systems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		bw.q = si.NewQueryBuilder(&bw.base).Optional(&bw.sight, &bw.collider).Build()
	}})
	ctx.ECS().Setup(systems...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		bw.World.RunPlan(rc, d)
		c.RunPlan(rc, d)
		bw.Board.RunPlan(rc, d)
		v.RunPlan(rc, d)
		rc.Sync()
		bw.World.Clock().Replay(rc, d)
	})
	bw.ECS = ctx.ECS()
	return bw
}

// Tick steps the world a sixtieth of a second.
func (bw *World) Tick() { bw.ECS.Tick(time.Second / 60) }

// Snapshot lists the units' boxes by TypeID order.
func (bw *World) Snapshot() []geom.AABB {
	byType := map[kind.ID]geom.AABB{}
	for bw.q.All(); bw.q.Next(); {
		cur := bw.q.Cursor()
		for _, b := range bw.base.Slice(cur) {
			byType[b.TypeID] = b.Pos.AABB.AABB
		}
	}
	units := make([]geom.AABB, 0, len(byType))
	for _, id := range slices.Sorted(maps.Keys(byType)) {
		units = append(units, byType[id])
	}
	return units
}

// Solid lists the boxes the board holds solid for an entity on layers, board-wide: its field's,
// the collision.Field the plugin hands over.
func (bw *World) Solid(layers world.Layers) []geom.AABB {
	var out []geom.AABB
	w, h := bw.World.Res.Config.Space.Width, bw.World.Res.Config.Space.Height
	bw.Board.Cover().(collision.Field).Solid(layers, collision.Everywhere, geom.NewAABBAt(geom.NewVec(0, 0), float64(w), float64(h)), func(fb collision.FieldBox) bool {
		out = append(out, fb.Box)
		return true
	})
	return out
}

// Seen returns what the one observer saw.
func (bw *World) Seen() (vision.Sighted, bool) {
	for bw.q.All(); bw.q.Next(); {
		cur := bw.q.Cursor()
		if !bw.sight.Present(cur) {
			continue
		}
		return bw.sight.Slice(cur)[0], true
	}
	return vision.Sighted{}, false
}

// Struck is what the units struck in the last tick, in the order they stand in the ECS.
func (bw *World) Struck() []collision.Contact {
	var out []collision.Contact
	for bw.q.All(); bw.q.Next(); {
		for _, c := range bw.collider.Slice(bw.q.Cursor()) {
			out = append(out, c.Contacts()...)
		}
	}
	return out
}

// Overlaps reports whether a and b share interior, not just an edge.
func Overlaps(a, b geom.AABB) bool {
	const eps = 1e-6
	return min(a.BottomRight.X, b.BottomRight.X)-max(a.TopLeft.X, b.TopLeft.X) > eps &&
		min(a.BottomRight.Y, b.BottomRight.Y)-max(a.TopLeft.Y, b.TopLeft.Y) > eps
}

// AssertClear fails the test where a unit overlaps solid ground.
func (bw *World) AssertClear(tick int, units, walls []geom.AABB) {
	bw.t.Helper()
	for i, u := range units {
		for _, b := range walls {
			if Overlaps(u, b) {
				bw.t.Fatalf("tick %d: unit %d at %v overlaps solid ground %v", tick, i, u, b)
			}
		}
	}
}

// CellSize is the side of a SquareWorld's cells.
const CellSize = 32

// SquareWorld is a World of 6 x 16 grass cells with a wall down column 3, rows 1 to 14; a unit
// starts at (1, 7) unless its row says otherwise. It gives the wall's cell beside them, (3, 7).
func SquareWorld(t *testing.T, units ...Mover) (*World, cell.ID) {
	t.Helper()
	return SquareWorldWith(t, nil, units...)
}

// SquareWorldWith is SquareWorld with a rule hooked on the board.
func SquareWorldWith(t *testing.T, hooked rule.Rule, units ...Mover) (*World, cell.ID) {
	t.Helper()
	grid := grid.DefaultGrids{}.Square(6, 16, CellSize)
	cellAt := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	for i := range units {
		if units[i].Here == 0 {
			units[i].Here = cellAt(1, 7)
		}
	}
	var rules []rule.Rule
	if hooked != nil {
		rules = append(rules, hooked)
	}
	bw := NewWorld(t, grid, 6*CellSize, 16*CellSize, func(brd *board.Board) {
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		for y := uint32(1); y <= 14; y++ {
			brd.Set(cellAt(3, y), Wall)
		}
	}, units, rules...)
	return bw, cellAt(3, 7)
}

// Wall is a solid kind that also cuts sight, as a wall of stone does.
var Wall = cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true, Veil: 1}

// NewInstallCtx is an InstallCtx over a new ECS.
func NewInstallCtx() *InstallCtx { return &InstallCtx{ecs: goke.New()} }

// Systems are the systems the installed plugins asked for, to hand to the ECS's Setup.
func (c *InstallCtx) Systems() []goke.System {
	var systems []goke.System
	for _, produce := range c.pending {
		systems = append(systems, produce()...)
	}
	return systems
}

// CellBox is the size x size box centred on cell c.
func CellBox(g grid.Grid, c cell.ID, size uint32) plane.AABB {
	at := g.CellCenter(c)
	half := float64(size) / 2
	return plane.NewAABB(geom.NewVec(at.X-half, at.Y-half), float64(size), float64(size))
}

// OneStageGame is a minimal game.Game wrapping a single Stage.
type OneStageGame struct {
	Stage     game.Stage
	GameProps game.Props
}

func (g OneStageGame) Props() game.Props { return g.GameProps }

func (g OneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.Stage.Name(): g.Stage}, g.Stage.Name()
}
