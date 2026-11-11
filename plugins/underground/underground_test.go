package underground_test

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/underground"
	"github.com/kjkrol/gram/plugins/underground/site"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type installer struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installer) UseModule(m goke.Module) {
	reg := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), reg) })
}
func (c *installer) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installer) RegSys(f func() goke.System) goke.Runnable { return c.ecs.RegSys(f()) }
func (c *installer) ECS() *goke.ECS                            { return c.ecs }
func (c *installer) Hosts(...host.Host)                        {}
func (c *installer) Draws(...render.Source)                    {}
func (c *installer) Looks(func())                              {}
func (*installer) Pool() *parallel.Pool                        { return nil }

// massif is a board 10 x 6 of cells 10 wide, the ground at 0 but for a ridge 20 up between x 40
// and 60, its sides climbing over a cell each
type massif struct {
	t     *testing.T
	g     grid.Grid
	w     *world.Plugin
	m     *motion.Plugin
	col   *collision.Plugin
	board *board.Plugin
	topo  *topography.Plugin
	under *underground.Plugin
	ecs   *goke.ECS
	all   *goke.Query
	base  goke.Comp[world.Base]
	z     goke.OptComp[world.Z]
	floor goke.OptComp[cell.Floor]
	on    goke.OptComp[cell.On]
	whole goke.OptComp[world.Whole]
	piece goke.OptComp[world.PieceOf]
}

func newMassif(t *testing.T) *massif {
	t.Helper()
	d := &massif{t: t, g: grid.DefaultGrids{}.Square(10, 6, 10)}
	d.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 60},
		Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 1, MaxSize: 60},
		Heights:  true,
	})
	d.m = motion.NewPlugin(d.w)
	d.col = collision.NewPlugin(d.w, d.m)
	d.board = board.NewPlugin(d.g, &cell.MultipleOccupancy{}, d.w).
		WithMotion(d.m, cell.DefaultHiking).
		WithCollision(d.col)
	d.topo = topography.NewPlugin(d.w, d.board, nil, topography.Config{})
	d.under = underground.NewPlugin(d.w, d.board,
		underground.Config{Rules: site.Rules{Cover: 2}}).WithShaper(d.topo.Write())
	d.board.Read(0).CellKinds().Define("earth", cell.Kind{Pace: 1, Allows: cell.Land})
	d.under.Types().Define("tunnel", underground.Spec{Allows: cell.Land, Width: 6, Clear: 3,
		Pace: 1})
	d.under.Types().Chamber("cave", underground.Spec{Allows: cell.Land, Clear: 4, Pace: 1})
	if err := d.w.Carry(d.under); err != nil {
		t.Fatal(err)
	}
	return d
}

// a level's file of the massif with the entities given
func level(entities ...string) fstest.MapFS { return terraced("0", entities...) }

// terraced is level with the ground east of the ridge at east
func terraced(east string, entities ...string) fstest.MapFS {
	var corners strings.Builder
	for y := 0; y <= 6; y++ {
		for x := 0; x <= 10; x++ {
			if x > 0 {
				corners.WriteString(" ")
			}
			switch {
			case x >= 4 && x <= 6:
				corners.WriteString("20")
			case x > 6:
				corners.WriteString(east)
			default:
				corners.WriteString("0")
			}
		}
		corners.WriteString("\n")
	}
	var lines strings.Builder
	for _, e := range entities {
		lines.WriteString("    - " + e + "\n")
	}
	body := strings.ReplaceAll(strings.TrimRight(corners.String(), "\n"), "\n", "\n    ")
	return fstest.MapFS{"map.yaml": {Data: []byte("board:\n  default: earth\n" +
		"  entities:\n" + lines.String() + "relief:\n  corners: |\n    " + body + "\n")}}
}

// lay digs the level's structures and starts the world; the error is the digging's
func (d *massif) lay(file fstest.MapFS) error {
	d.t.Helper()
	lvl, err := d.board.Lay(file, "map.yaml")
	if err != nil {
		d.t.Fatal(err)
	}
	if err := d.under.Write().Spawn(lvl); err != nil {
		return err
	}
	d.topo.Write().Seed(lvl.Heights())
	ctx := &installer{ecs: goke.New()}
	for _, p := range []plugin.Plugin{d.w, d.m, d.col, d.board, d.topo, d.under} {
		if err := p.Install(ctx); err != nil {
			d.t.Fatal(err)
		}
	}
	for _, populate := range []func() error{d.w.Populate, d.board.Populate, d.topo.Populate,
		d.under.Populate} {
		if err := populate(); err != nil {
			return err
		}
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		d.all = si.NewQueryBuilder(&d.base).
			Optional(&d.z, &d.floor, &d.on, &d.whole, &d.piece).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, dt time.Duration) {
		d.under.RunPlan(rc, dt)
		d.w.RunPlan(rc, dt)
		d.m.RunPlan(rc, dt)
		d.col.RunPlan(rc, dt)
		d.board.RunPlan(rc, dt)
		d.topo.RunPlan(rc, dt)
		rc.Sync()
		d.w.Write().Clock().Replay(rc, dt)
	})
	d.ecs = ctx.ecs
	for range 3 {
		d.ecs.Tick(time.Second / 60)
	}
	return nil
}

// floors are the floors lying under the cells, by cell
func (d *massif) floors() map[cell.ID]cell.Floor {
	out := map[cell.ID]cell.Floor{}
	for d.all.All(); d.all.Next(); {
		cur := d.all.Cursor()
		fs, ons := d.floor.Slice(cur), d.on.Slice(cur)
		for i := range fs {
			out[ons[i].Cell] = fs[i]
		}
	}
	return out
}

func TestSpawn_ALevelsTunnelLiesUnderTheRidgeFromHeadToHead(t *testing.T) {
	d := newMassif(t)
	if err := d.lay(level("{kind: tunnel, from: [2, 3], to: [7, 3]}")); err != nil {
		t.Fatal(err)
	}
	g := d.g
	east, _ := grid.Direction(g, 1, 0)
	west, _ := grid.Direction(g, -1, 0)
	floors := d.floors()
	if len(floors) != 4 {
		t.Fatalf("%d pieces lie under the cells, want 4: %v", len(floors), floors)
	}
	for x, level := range map[uint32]float32{3: 10, 4: 20, 5: 20, 6: 10} {
		f := floors[g.At(x, 3)]
		want := cell.Floor{Allows: cell.Land, Height: -level, Clear: 3, Pace: 1, Graded: true,
			Links: 1<<east | 1<<west, Width: 6, Storey: 1}
		if f != want {
			t.Errorf("under [%d, 3]: %+v, want %+v", x, f, want)
		}
	}
	wholes, pieces := 0, 0
	for d.all.All(); d.all.Next(); {
		cur := d.all.Cursor()
		for range cur.IDs {
			switch {
			case d.whole.Slice(cur) != nil:
				wholes++
			case d.piece.Slice(cur) != nil:
				pieces++
			}
		}
	}
	if wholes != 1 || pieces != 4 {
		t.Errorf("%d handles, %d pieces; want one handle over the four pieces, its mouths none "+
			"of their own", wholes, pieces)
	}
	brd := d.board.Read(0).Board()
	inside := g.CellCenter(g.At(5, 3))
	if c, on, ok := brd.LevelFor(inside, 0, 1); !ok || c != g.At(5, 3) || on != cell.OnDeck+1 {
		t.Errorf("under the ridge with its feet at 0: on %v of %v, want the second storey", on, c)
	}
	if _, on, _ := brd.LevelFor(inside, 20, 1); on != cell.OnGround {
		t.Errorf("on the ridge with its feet at 20: on %v, want the ground", on)
	}
	floor, roof := d.board.Read(0).Footing().Surface(inside, cell.Land, 0, 1)
	if floor != 0 || roof != 3 {
		t.Errorf("under the ridge the floor at %v under a roof at %v, want 0 and 3", floor, roof)
	}
	if floor, _ := d.board.Read(0).Footing().Surface(inside, cell.Land, 20, 1); floor != 20 {
		t.Errorf("on the ridge the floor at %v, want the ground at 20", floor)
	}
}

func TestCage_IsFreeInTheRoomOverASunkFloorAlone(t *testing.T) {
	d := newMassif(t)
	if err := d.lay(level("{kind: tunnel, from: [2, 3], to: [7, 3]}")); err != nil {
		t.Fatal(err)
	}
	cage := d.topo.Read(0)
	for name, tc := range map[string]struct {
		x, y, z float32
		free    bool
	}{
		"in the tunnel under the ridge": {55, 35, 1.5, true},
		"in the rock over its roof":     {55, 35, 10, false},
		"in the rock under its floor":   {55, 35, -1, false},
		"in the rock beside it":         {55, 45, 1.5, false},
		"over the ridge":                {55, 35, 21, true},
		"in its mouth under the slope":  {31, 35, 1.5, true},
	} {
		if got := cage.Free(tc.x, tc.y, tc.z); got != tc.free {
			t.Errorf("%s, at (%v, %v, %v): free %v, want %v", name, tc.x, tc.y, tc.z, got,
				tc.free)
		}
	}
}

func (d *massif) give(cmd command.Command) {
	d.t.Helper()
	if !d.w.Write().Carrier().Put(1, cmd) {
		d.t.Fatalf("nobody takes %T", cmd)
	}
	d.ecs.Tick(time.Second / 60)
}

func TestTool_DigsTheTunnelItsPlayerMarkedAndConfirmed(t *testing.T) {
	d := newMassif(t)
	if err := d.lay(level()); err != nil {
		t.Fatal(err)
	}
	g := d.g
	d.give(underground.Mark{At: g.CellCenter(g.At(2, 3))})
	d.give(underground.Aim{At: g.CellCenter(g.At(3, 3))})
	d.give(underground.Mark{At: g.CellCenter(g.At(3, 3))})
	d.give(underground.Dig{})
	if n := len(d.floors()); n != 0 {
		t.Fatalf("a tunnel to the cell beside its head, refused, dug %d pieces", n)
	}
	d.give(underground.Aim{At: g.CellCenter(g.At(7, 3))})
	d.give(underground.Mark{At: g.CellCenter(g.At(7, 3))})
	if n := len(d.floors()); n != 0 {
		t.Fatalf("marked and not confirmed, %d pieces are dug", n)
	}
	d.give(underground.Dig{})
	for range 3 {
		d.ecs.Tick(time.Second / 60)
	}
	floors := d.floors()
	if len(floors) != 4 || len(d.under.Res.Orders) != 0 {
		t.Fatalf("confirmed: %d pieces dug, %d tunnels still ordered; want 4 and none",
			len(floors), len(d.under.Res.Orders))
	}
	if f := floors[g.At(5, 3)]; f.Height != -20 || f.Clear != 3 || f.Storey != 1 {
		t.Errorf("under the ridge's top %+v, want its floor at the heads' level", f)
	}
}

func TestSurface_ATunnelsFloorClimbsEvenlyFromHeadToHead(t *testing.T) {
	d := newMassif(t)
	if err := d.lay(terraced("5", "{kind: tunnel, from: [2, 3], to: [7, 3]}")); err != nil {
		t.Fatal(err)
	}
	footing := d.board.Read(0).Footing()
	for x, want := range map[float32]float32{35: 1, 40: 1.5, 45: 2, 50: 2.5, 55: 3, 65: 4} {
		floor, roof := footing.Surface(geom.NewVec(x, 35), cell.Land, want, 1)
		if f32.Abs(floor-want) > 1e-3 || f32.Abs(roof-want-3) > 1e-3 {
			t.Errorf("at x %v the floor at %v under a roof at %v, want %v and its room over it",
				x, floor, roof, want)
		}
	}
}

func TestSpawn_RefusesWhatBreaksTheRulesWithItsLine(t *testing.T) {
	for name, tc := range map[string]struct {
		entity, why string
	}{
		"a head on the slope": {"{kind: tunnel, from: [3, 3], to: [7, 3]}", "off level"},
		"under no ridge":      {"{kind: tunnel, from: [0, 3], to: [2, 3]}", "the ground lies"},
		"a shallow chamber":   {"{kind: cave, from: [4, 2], to: [5, 3], depth: 5}", "of rock"},
	} {
		d := newMassif(t)
		err := d.lay(level(tc.entity))
		if err == nil || !strings.Contains(err.Error(), "map.yaml:4") ||
			!strings.Contains(err.Error(), tc.why) {
			t.Errorf("%s: %v; want a refusal of map.yaml:4 saying %q", name, err, tc.why)
		}
	}
}

func TestSpawn_ATunnelOpensIntoAChamber(t *testing.T) {
	d := newMassif(t)
	err := d.lay(level("{kind: tunnel, from: [2, 3], to: [4, 3]}",
		"{kind: cave, from: [4, 2], to: [5, 4], depth: 20}"))
	if err != nil {
		t.Fatal(err)
	}
	g := d.g
	floors := d.floors()
	if len(floors) != 7 {
		t.Fatalf("%d pieces lie under the cells, want a tunnel's one and a chamber's six",
			len(floors))
	}
	if f := floors[g.At(3, 3)]; f.Height != -10 || f.Width != 6 || f.Storey != 1 {
		t.Errorf("the tunnel's piece %+v, want its floor at the head's level on the chamber's "+
			"storey", f)
	}
	if f := floors[g.At(4, 3)]; f.Height != -20 || f.Width != 0 || f.Clear != 4 {
		t.Errorf("the chamber's piece %+v, want the whole cell 20 under the ridge", f)
	}
}

// walls are the walls a body spanning band meets round box
func (d *massif) walls(band collision.Band, box geom.AABB) []collide.FieldWall {
	var out []collide.FieldWall
	d.board.Read(0).Cover().(collision.Field).Walls(world.Layers(cell.Land), band, box,
		func(w collide.FieldWall) bool {
			out = append(out, w)
			return true
		})
	return out
}

// walled says whether p lies in one of walls
func walled(walls []collide.FieldWall, p geom.Vec) bool {
	for _, w := range walls {
		ax, ay := w.B.X-w.A.X, w.B.Y-w.A.Y
		t := ((p.X-w.A.X)*ax + (p.Y-w.A.Y)*ay) / (ax*ax + ay*ay)
		if t < 0 || t > 1 {
			continue
		}
		if f32.Hypot(p.X-w.A.X-t*ax, p.Y-w.A.Y-t*ay) <= w.Half {
			return true
		}
	}
	return false
}

func TestWalls_StandEitherSideOfATunnelForABodyInItsRoomAlone(t *testing.T) {
	d := newMassif(t)
	if err := d.lay(level("{kind: tunnel, from: [2, 3], to: [7, 3]}")); err != nil {
		t.Fatal(err)
	}
	under := geom.NewAABBAt(geom.NewVec(50, 30), 10, 10)
	walls := d.walls(collision.Band{Bottom: 0, Top: 2}, under)
	if len(walls) != 2 {
		t.Fatalf("a body in the tunnel meets %d walls under a cell, want one either side", len(walls))
	}
	for name, tc := range map[string]struct {
		at   geom.Vec
		want bool
	}{
		"beyond its north side":  {geom.NewVec(55, 31.8), true},
		"beyond its south side":  {geom.NewVec(55, 38.2), true},
		"at the foot of a wall":  {geom.NewVec(55, 32.5), false},
		"in the middle of it":    {geom.NewVec(55, 35), false},
		"in the rock far beside": {geom.NewVec(55, 30.5), false},
	} {
		if got := walled(walls, tc.at); got != tc.want {
			t.Errorf("%s, at %v: in a wall %v, want %v", name, tc.at, got, tc.want)
		}
	}
	if on := d.walls(collision.Band{Bottom: 20, Top: 22}, under); len(on) != 0 {
		t.Errorf("a body on the ridge over the tunnel meets %d walls, want none", len(on))
	}
}

func TestWalls_CloseATurnOutsideAndLeaveItOpenInside(t *testing.T) {
	d := newMassif(t)
	err := d.lay(level("{kind: tunnel, from: [2, 1], to: [7, 4], via: [[5, 1], [5, 4]]}"))
	if err != nil {
		t.Fatal(err)
	}
	// the piece under [5, 1] turns from the west to the south
	walls := d.walls(collision.Band{Bottom: 0, Top: 2},
		geom.NewAABBAt(geom.NewVec(50, 10), 10, 10))
	for name, tc := range map[string]struct {
		at   geom.Vec
		want bool
	}{
		"its middle":                  {geom.NewVec(55, 15), false},
		"the way in from the west":    {geom.NewVec(51, 15), false},
		"the way out to the south":    {geom.NewVec(55, 19), false},
		"across the way to the south": {geom.NewVec(55, 18.3), false},
		"the north side":              {geom.NewVec(55, 11.7), true},
		"the east side":               {geom.NewVec(58.3, 15), true},
		"the corner outside":          {geom.NewVec(58.3, 11.7), true},
		"the south side to the west":  {geom.NewVec(51, 18.3), true},
		"the west side to the south":  {geom.NewVec(51.7, 19.5), true},
	} {
		if got := walled(walls, tc.at); got != tc.want {
			t.Errorf("%s, at %v: in a wall %v, want %v", name, tc.at, got, tc.want)
		}
	}
}

func TestWalls_StandRoundAChamberButForTheDoorOfATunnelComingIn(t *testing.T) {
	d := newMassif(t)
	err := d.lay(level("{kind: tunnel, from: [2, 3], to: [4, 3]}",
		"{kind: cave, from: [4, 2], to: [5, 4], depth: 20}"))
	if err != nil {
		t.Fatal(err)
	}
	room := geom.NewAABBAt(geom.NewVec(40, 20), 20, 30)
	walls := d.walls(collision.Band{Bottom: 0, Top: 2}, room)
	if len(walls) != 11 {
		t.Errorf("a body in the chamber meets %d walls, want its ten edges, one in two "+
			"either side of the door", len(walls))
	}
	for name, tc := range map[string]struct {
		at   geom.Vec
		want bool
	}{
		"the door":               {geom.NewVec(39.8, 35), false},
		"beside the door":        {geom.NewVec(39.8, 31), true},
		"the west wall north":    {geom.NewVec(39.8, 25), true},
		"the east wall":          {geom.NewVec(60.2, 35), true},
		"the north wall":         {geom.NewVec(50, 19.8), true},
		"between two of its own": {geom.NewVec(50, 35), false},
		"the middle of a cell":   {geom.NewVec(45, 35), false},
	} {
		if got := walled(walls, tc.at); got != tc.want {
			t.Errorf("%s, at %v: in a wall %v, want %v", name, tc.at, got, tc.want)
		}
	}
	if over := d.walls(collision.Band{Bottom: 20, Top: 22}, room); len(over) != 0 {
		t.Errorf("a body on the ridge over the chamber meets %d walls, want none", len(over))
	}
}
