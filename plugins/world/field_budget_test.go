package world_test

import (
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

const (
	stripKind    = "strip"
	stripRole    = "strip"
	stripSectors = 40
	stripWidth   = stripSectors * sectorSide
)

// a field over a row of 40 sectors 512 a side, the whole world, its stretches planting half their
// plots; seen from above by one camera looking at its east end, at L4 (a quarter of a pixel a
// world unit) till zoomed to L3 (a half), or by none, every sector at L3
type strip struct {
	eng    *engine.Engine
	w      *world.Plugin
	ecs    *goke.ECS
	cam    camera.Camera
	orders heards
	told   []world.Unfold
	// by tick, the sectors told their Unfold
	steps [][]int

	trees    *goke.Query
	treeBase goke.Comp[world.Base]
	treeOf   goke.Comp[world.PartOf]
}

var _ sector.Observers = (*strip)(nil)

func (s *strip) Observers(each func(cam camera.Camera)) { each(s.cam) }

func (s *strip) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		s.trees = si.NewQueryBuilder(&s.treeBase, &s.treeOf).Build()
	}}}
}

func newStrip(t *testing.T, unfolds int, seen bool) *strip {
	t.Helper()
	s := &strip{}
	ladder := lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.8, 0.4}, Unfolds: unfolds}
	if seen {
		// its picture shows the whole strip: the budget goes by the distance alone
		s.cam = cameratest.Showing(cameratest.NewFromSpace(stripWidth, sectorSide, 0))
		s.cam.SetViewport(100, 100)
		s.cam.ZoomOut(4, 0, 0)
		s.cam.CenterOn(stripWidth, sectorSide/2, 0)
	} else {
		ladder.Unobserved = lod.UnobservedL3
	}
	st := stage.New("strip").
		World(world.Config{
			Space:    world.SpaceCfg{Width: stripWidth, Height: sectorSide},
			Entities: world.EntitiesCfg{MaxCount: 512, MinSize: 5, MaxSize: 20},
			Sectors:  world.SectorsCfg{Buckets: 2},
			LOD:      ladder,
		}).
		Plugins(func(w *world.Plugin) {
			s.w = w
			if seen {
				w.Write().Observe(s)
			}
			if err := w.Carry(&s.orders); err != nil {
				t.Fatal(err)
			}
		}).
		Effects(func(ctx game.Initializer) {
			s.ecs = ctx.ECS()
			ctx.Setup(s)
		}).
		Rules(func() { s.rules() }).
		Kinds(func() { s.kinds() }).
		Spawn(func() {
			s.w.Write().Seed(kind.Named[forestRow](s.w.Kinds(), stripKind).
				Entry(forestRow{stripWidth / 2, sectorSide / 2}))
		}).
		Update(func(ctx goke.RunCtx, dt time.Duration) { s.w.RunPlan(ctx, dt) })
	s.eng = engine.NewEngine(oneStageGame{stage: st})
	if err := s.eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

func (s *strip) rules() {
	noted := cond.NewWord("noted", func(u world.Unfold) bool {
		s.told = append(s.told, u)
		return true
	})
	rules := []act.Rule{
		act.Branch[world.Unfold]{Name: "unfold", When: cond.Self.If(noted), Then: said1("unfold")},
	}
	for k := range plots {
		rules = append(rules, act.Branch[world.Unfold]{
			Name: fmt.Sprintf("plant %d", k),
			When: cond.Self.If(plot(k, 2)),
			Then: act.Self.Do(world.SpawnPart{Entry: kind.EntryOf(treeKind, treeRow{}),
				At: plotAt[k]}),
		})
	}
	s.w.Roles().Define(stripRole, rules...)
}

func (s *strip) kinds() {
	kind.Define[forestRow](s.w.Kinds(), stripKind, kind.Spec{
		comp.Load(func(r forestRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x-10, r.y-10), 20, 20)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{}),
		comp.Const(world.Field{Size: geom.NewVec(stripWidth, sectorSide)}),
		ceremony.Plays(s.w.Roles().Named(stripRole)),
	})
	kind.Define[treeRow](s.w.Kinds(), treeKind, kind.Spec{
		comp.Load(func(treeRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(-5, -5), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
	})
}

func (s *strip) run(ticks int) {
	for range ticks {
		k := len(s.told)
		s.ecs.Tick(frame)
		s.orders.Drain(func(command.Issued[heard]) {})
		var told []int
		for _, u := range s.told[k:] {
			told = append(told, u.Sector)
		}
		s.steps = append(s.steps, told)
	}
}

func (s *strip) zoom(zoom float32) {
	s.cam.ZoomIn(zoom/s.cam.Zoom(), 0, 0)
	s.cam.CenterOn(stripWidth, sectorSide/2, 0)
}

// the steps that told any Unfold, from the first such on, an empty one between them kept
func (s *strip) batches() [][]int {
	from := slices.IndexFunc(s.steps, func(told []int) bool { return len(told) > 0 })
	if from < 0 {
		return nil
	}
	to := len(s.steps)
	for to > from && len(s.steps[to-1]) == 0 {
		to--
	}
	return s.steps[from:to]
}

// the places of the trees, sorted
func (s *strip) places() []geom.Vec {
	var at []geom.Vec
	for s.trees.All(); s.trees.Next(); {
		for _, b := range s.treeBase.Slice(s.trees.Cursor()) {
			at = append(at, b.Pos.Center())
		}
	}
	slices.SortFunc(at, func(a, b geom.Vec) int {
		if a.X != b.X {
			return int(a.X - b.X)
		}
		return int(a.Y - b.Y)
	})
	return at
}

// the sectors the trees stand in, each once, sorted
func (s *strip) planted() []int {
	var in []int
	for _, at := range s.places() {
		in = append(in, int(at.X/sectorSide))
	}
	return slices.Compact(in)
}

// the sectors from the east end, by k
func fromEast(k int) [][]int {
	var want [][]int
	for i := stripSectors - 1; i >= 0; i -= k {
		var batch []int
		for j := i; j > i-k && j >= 0; j-- {
			batch = append(batch, j)
		}
		want = append(want, batch)
	}
	return want
}

func sorted(sectors []int) []int {
	out := slices.Clone(sectors)
	slices.Sort(out)
	return out
}

func TestWholes_UnfoldTheBudgetOfStretchesAStepNearestTheCameraFirst(t *testing.T) {
	whole := newStrip(t, 100, true)
	whole.run(3)
	whole.zoom(0.5)
	whole.run(10)
	all := whole.places()
	if len(all) == 0 {
		t.Fatal("no trees planted unfolding the whole strip at once")
	}
	for _, c := range []struct {
		unfolds, budget int
	}{{0, 16}, {10, 10}, {100, 100}} {
		t.Run(fmt.Sprint(c.unfolds), func(t *testing.T) {
			s := newStrip(t, c.unfolds, true)
			s.run(3)
			if len(s.told) > 0 {
				t.Fatalf("told %d Unfolds at L4, want none", len(s.told))
			}
			s.zoom(0.5)
			s.run(10)
			got, want := s.batches(), fromEast(c.budget)
			if len(got) != len(want) {
				t.Fatalf("unfolded in %d steps %v, want %d", len(got), got, len(want))
			}
			for i := range want {
				if !slices.Equal(sorted(got[i]), sorted(want[i])) {
					t.Errorf("step %d unfolded sectors %v, want the nearest %v", i, sorted(got[i]),
						sorted(want[i]))
				}
			}
			if have := s.places(); !slices.Equal(have, all) {
				t.Errorf("planted %d trees, want the %d of the strip unfolded at once, in place",
					len(have), len(all))
			}
		})
	}
}

func TestWholes_AStretchBackAtL4BeforeItsTurnUnfoldsNot(t *testing.T) {
	s := newStrip(t, 10, true)
	s.run(3)
	s.zoom(0.5)
	s.run(1)
	s.zoom(0.25)
	s.run(30)
	near := fromEast(10)[0]
	if got := sorted(s.batches()[0]); !slices.Equal(got, sorted(near)) || len(s.batches()) != 1 {
		t.Errorf("unfolded %v, want the nearest %v alone", s.batches(), sorted(near))
	}
	if got := s.planted(); !slices.Equal(got, sorted(near)) {
		t.Errorf("trees stand in sectors %v, want %v alone", got, sorted(near))
	}
}

func TestWholes_UnfoldTheBudgetAStepInTheSameOrderWithNoObserver(t *testing.T) {
	once := newStrip(t, 0, false)
	once.run(6)
	again := newStrip(t, 0, false)
	again.run(6)
	got := once.batches()
	if len(got) != 3 || len(got[0]) != 16 || len(got[1]) != 16 || len(got[2]) != 8 {
		t.Fatalf("unfolded %v, want 16, 16 and 8 a step", got)
	}
	for i := range got {
		if !slices.Equal(got[i], again.batches()[i]) {
			t.Errorf("step %d unfolded %v, then %v", i, got[i], again.batches()[i])
		}
	}
	if have := once.planted(); len(have) != stripSectors {
		t.Errorf("trees stand in %d sectors, want all %d", len(have), stripSectors)
	}
}

// whether the forest's part over box is drawn
func (f *forest) drawsPart(box geom.AABB) bool {
	for _, d := range f.w.Published() {
		if d.ID == f.id() && d.Piece && d.Box == box {
			return true
		}
	}
	return false
}

func TestWholes_AStretchWaitingItsTurnStaysFoldedItsPartsKeptAndItsPartDrawn(t *testing.T) {
	f := newForestOf(t, &forest{at: geom.NewVec(256, 256), unfolds: 1}, forestLooks{})
	// 200 by 600 world units, so sector 8 is seen and 9 lies past the sight
	f.cam.SetViewport(100, 300)
	f.look(f.at)
	f.start(t, false)
	f.w.Sheets()
	f.w.Write().NewView(f.cam)
	// the parts come a step after their Unfold
	f.run(4)
	if got := sectorsOf(f.unfolded); !slices.Equal(got, []int{0, 1, 8}) {
		t.Fatalf("unfolded sectors %v in four steps, want the three within sight", got)
	}
	before := f.bySector()
	f.look(geom.NewVec(1300, 256))
	f.until(t, "fold", 2*int(foldAfter/frame))
	f.run(2)
	kept := map[int]int{0: len(before[0]), 8: len(before[8])}
	if got := f.w.SheetShots(); !maps.Equal(got, kept) {
		t.Fatalf("kept %v parts by sector, want those folded, %v", got, kept)
	}
	f.unfolded = f.unfolded[:0]
	f.look(geom.NewVec(256, 256))
	f.run(1)
	if got := sectorsOf(f.unfolded); !slices.Equal(got, []int{0}) {
		t.Fatalf("unfolded sectors %v, want the nearest, 0, alone", got)
	}
	if got := f.w.SheetShots(); !maps.Equal(got, map[int]int{8: kept[8]}) {
		t.Errorf("kept %v parts by sector, want sector 8's waiting, %d", got, kept[8])
	}
	f.standIn(t, 0, 1)
	if !f.drawsPart(boxOf(8)) || f.drawsPart(boxOf(0)) {
		t.Errorf("the forest's part drawn over sector 8 %v, over sector 0 %v; want 8's alone",
			f.drawsPart(boxOf(8)), f.drawsPart(boxOf(0)))
	}
	f.run(1)
	if got := sectorsOf(f.unfolded); !slices.Equal(got, []int{0, 8}) {
		t.Fatalf("unfolded sectors %v, want 8 after 0", got)
	}
	if got := f.w.SheetShots(); len(got) != 0 {
		t.Errorf("kept %v parts by sector, want none", got)
	}
	f.run(1)
	f.plantedIn(t, 2, 0, 1, 8)
	if f.drawsPart(boxOf(8)) {
		t.Error("the forest's part drawn over sector 8 unfolded")
	}
}
