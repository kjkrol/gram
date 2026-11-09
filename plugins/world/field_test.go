package world_test

import (
	"fmt"
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
	"github.com/kjkrol/ceremony/effect"
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
	"github.com/kjkrol/uid"
)

const (
	forestKind = "forest"
	treeKind   = "tree"
	forestRole = "forest"
	treeRole   = "tree"
	thinnedEf  = "thinned"
	// the forest's side: two sectors
	forestSide = 1024
	sectorSide = 512
	cols       = 8
	// how far from where the camera looks the world is ticked
	sight = 300
	// the places a stretch of the forest may hold a tree at
	plots = 8
	// a tree's height in a world with heights
	treeHeight = 4
)

// off the middle of a stretch
var plotAt = [plots]geom.Vec{
	{X: -192, Y: -128},
	{X: -64, Y: -128},
	{X: 64, Y: -128},
	{X: 192, Y: -128},
	{X: -192, Y: 128},
	{X: -64, Y: 128},
	{X: 64, Y: 128},
	{X: 192, Y: 128},
}

type forestRow struct{ x, y float32 }

type treeRow struct{}

// a forest over sectors 0, 1, 8 and 9 of a world 8x2 sectors 512 a side, its centre in 9, seen by
// one camera from above at L3 (half a pixel a world unit) and L4 (a quarter) within the sight of
// where it looks, L6 past it; in a world with heights where tall, its ground at ground; unfolding
// the ladder's Unfolds stretches a step
type forest struct {
	eng      *engine.Engine
	w        *world.Plugin
	ecs      *goke.ECS
	cam      camera.Camera
	at       geom.Vec
	tall     bool
	ground   float32
	unfolds  int
	orders   heards
	heard    []said
	unfolded []world.Unfold
	folded   []world.Fold

	trees    *goke.Query
	treeBase goke.Comp[world.Base]
	treeOf   goke.Comp[world.PartOf]
	forests  *goke.Query
	base     goke.Comp[world.Base]
	// whether the forest carries Unfolded
	marked goke.OptComp[world.Unfolded]
}

var _ sector.Observers = (*forest)(nil)

func (f *forest) Observers(each func(cam camera.Camera)) { each(f.cam) }

func (f *forest) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f.trees = si.NewQueryBuilder(&f.treeBase, &f.treeOf).Build()
		f.forests = si.NewQueryBuilder(&f.base).Include(goke.Include[world.Field]()).
			Optional(&f.marked).Build()
	}}}
}

// the forest's stage, its camera at L3 looking at at, with the forest unless a save is loaded, the
// figures given noted
func newForest(t *testing.T, at geom.Vec, figures ...world.Figure) *forest {
	t.Helper()
	return newForestOf(t, &forest{at: at}, figures...)
}

// newGroundedForest is newForest in a world with heights, its ground at ground, its trees
// treeHeight tall
func newGroundedForest(t *testing.T, at geom.Vec, ground float32,
	figures ...world.Figure) *forest {
	t.Helper()
	return newForestOf(t, &forest{at: at, tall: true, ground: ground}, figures...)
}

func newForestOf(t *testing.T, f *forest, figures ...world.Figure) *forest {
	t.Helper()
	at := f.at
	f.cam = cameratest.NewFromSpace(4096, 1024, 0)
	f.cam.SetViewport(100, 100)
	f.cam.ZoomOut(2, 0, 0)
	f.look(at)
	st := stage.New("forest").
		World(world.Config{
			Space:    world.SpaceCfg{Width: 4096, Height: 1024},
			Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 5, MaxSize: 20},
			Sectors:  world.SectorsCfg{Buckets: 2},
			LOD: lod.Ladder{
				Scales:  [lod.Levels - 1]float32{0, 0, 0.8, 0.4},
				Sight:   sight,
				Fold:    float32(foldAfter.Seconds()),
				Unfolds: f.unfolds,
			},
			Heights: f.tall,
		}).
		Plugins(func(w *world.Plugin) {
			f.w = w
			if f.tall {
				w.Write().SetGround(func(geom.Vec) float32 { return f.ground })
			}
			w.Figures(figures...)
			w.Write().Observe(f)
			if err := w.Carry(&f.orders); err != nil {
				t.Fatal(err)
			}
		}).
		Effects(func(ctx game.Initializer) {
			f.ecs = ctx.ECS()
			ctx.Setup(f)
			f.w.Write().Effects().Define(thinnedEf, effect.Spec{})
		}).
		Rules(func() { f.rules() }).
		Kinds(func() { f.kinds() }).
		Spawn(func() {
			f.w.Write().Seed(kind.Named[forestRow](f.w.Kinds(), forestKind).
				Entry(forestRow{forestSide / 2, forestSide / 2}))
		}).
		Update(func(ctx goke.RunCtx, dt time.Duration) { f.w.RunPlan(ctx, dt) })
	f.eng = engine.NewEngine(oneStageGame{stage: st})
	return f
}

// the k-th plot of a stretch, where it lies in it, holds a tree by its seed in density of every
// four plots
func planted(whole uid.UID64, sector int, box geom.AABB, k int, density uint64) bool {
	at := middleOf(box).Add(plotAt[k])
	inside := at.X >= box.TopLeft.X && at.X < box.BottomRight.X && at.Y >= box.TopLeft.Y &&
		at.Y < box.BottomRight.Y
	return inside && lod.Seed(whole, uint64(sector*plots+k))%4 < density
}

func plantedIn(whole uid.UID64, sector int, density uint64) int {
	n := 0
	for k := range plots {
		if planted(whole, sector, boxOf(sector), k, density) {
			n++
		}
	}
	return n
}

func plot(k int, density uint64) cond.Word[world.Unfold] {
	return cond.NewWord(fmt.Sprintf("plot %d of %d", k, density), func(u world.Unfold) bool {
		return planted(u.ID, u.Sector, u.Box, k, density)
	})
}

// a stretch unfolding plants a tree at each plot its seed lets, half of them, a quarter once the
// forest is thinned; one folding short of what it planted thins the forest; a tree folds in by
// going
func (f *forest) rules() {
	thinned := f.w.Write().Effects().Named(thinnedEf)
	unfolded := cond.NewWord("noted", func(u world.Unfold) bool {
		f.unfolded = append(f.unfolded, u)
		return true
	})
	folded := cond.NewWord("noted", func(u world.Fold) bool {
		f.folded = append(f.folded, u)
		return true
	})
	standing := cond.NewWord("standing", func(u world.Fold) bool {
		return u.Parts >= plantedIn(u.ID, u.Sector, 2)
	})
	rules := []act.Rule{
		act.Branch[world.Unfold]{
			Name: "unfold",
			When: cond.Self.If(unfolded),
			Then: said1("unfold"),
		},
		act.Branch[world.Fold]{Name: "fold", When: cond.Self.If(folded), Then: said1("fold")},
		act.Branch[world.Fold]{
			Name: "thin",
			When: cond.Self.If(cond.Not(thinned)).And(cond.Self.If(cond.Not(standing))),
			Then: act.Self.Put(thinned),
		},
	}
	for k := range plots {
		tree := act.Self.Do(world.SpawnPart{Entry: kind.EntryOf(treeKind, treeRow{}),
			At: plotAt[k]})
		rules = append(rules,
			act.Branch[world.Unfold]{
				Name: fmt.Sprintf("plant %d", k),
				When: cond.Self.If(plot(k, 2)).And(cond.Self.If(cond.Not(thinned))),
				Then: tree,
			},
			act.Branch[world.Unfold]{
				Name: fmt.Sprintf("plant %d thinned", k),
				When: cond.Self.If(plot(k, 1)).And(cond.Self.If(thinned)),
				Then: tree,
			})
	}
	f.w.Roles().Define(forestRole, rules...)
	f.w.Roles().Define(treeRole, act.Branch[world.Folding]{
		Name: "fold in",
		When: cond.Self.If(cond.Always),
		Then: act.Steps(said1("folded"), act.Self.Do(world.Despawn{})),
	})
}

func (f *forest) kinds() {
	roles := f.w.Roles()
	kind.Define[forestRow](f.w.Kinds(), forestKind, kind.Spec{
		comp.Load(func(r forestRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x-10, r.y-10), 20, 20)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{}),
		comp.Const(world.Field{Size: geom.NewVec(forestSide, forestSide)}),
		ceremony.Plays(roles.Named(forestRole)),
	})
	tree := kind.Spec{
		comp.Load(func(treeRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(-5, -5), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
		ceremony.Plays(roles.Named(treeRole)),
	}
	if f.tall {
		tree = append(tree, comp.Const(world.Z{Height: treeHeight}))
	}
	kind.Define[treeRow](f.w.Kinds(), treeKind, tree)
}

func (f *forest) start(t *testing.T, load bool) {
	t.Helper()
	if load {
		if err := f.eng.Load("", ""); err != nil {
			t.Fatalf("Load: %v", err)
		}
	}
	if err := f.eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
}

func (f *forest) run(ticks int) {
	for range ticks {
		f.ecs.Tick(frame)
		now := f.w.Read(0).Clock().Time()
		f.orders.Drain(func(i command.Issued[heard]) {
			f.heard = append(f.heard, said{rule: i.Command.Rule, by: i.Entity, at: now})
		})
	}
}

func (f *forest) times(rule string, k int) []time.Duration {
	var at []time.Duration
	for _, s := range f.heard[k:] {
		if s.rule == rule {
			at = append(at, s.at)
		}
	}
	return at
}

func (f *forest) until(t *testing.T, rule string, ticks int) time.Duration {
	t.Helper()
	k := len(f.heard)
	for range ticks {
		f.run(1)
		if at := f.times(rule, k); len(at) > 0 {
			return at[0]
		}
	}
	t.Fatalf("no %s in %d ticks", rule, ticks)
	return 0
}

// has the camera look at at
func (f *forest) look(at geom.Vec) {
	f.at = at
	f.cam.CenterOn(at.X, at.Y, 0)
}

func (f *forest) zoom(zoom float32) {
	f.cam.ZoomIn(zoom/f.cam.Zoom(), 0, 0)
	f.look(f.at)
}

func (f *forest) id() uid.UID64 {
	for f.forests.All(); f.forests.Next(); {
		for _, id := range f.forests.Cursor().IDs {
			return id
		}
	}
	return 0
}

// the forest's trees by the sector their centre lies in
func (f *forest) bySector() map[int][]tree {
	got := map[int][]tree{}
	for f.trees.All(); f.trees.Next(); {
		cur := f.trees.Cursor()
		for i, b := range f.treeBase.Slice(cur) {
			c := b.Pos.Center()
			s := int(c.Y/sectorSide)*cols + int(c.X/sectorSide)
			got[s] = append(got[s], tree{id: cur.IDs[i], at: c, of: f.treeOf.Slice(cur)[i].Whole})
		}
	}
	for _, ts := range got {
		slices.SortFunc(ts, func(a, b tree) int {
			if a.at.Y != b.at.Y {
				return int(a.at.Y - b.at.Y)
			}
			return int(a.at.X - b.at.X)
		})
	}
	return got
}

type tree struct {
	id uid.UID64
	at geom.Vec
	of uid.UID64
}

func places(ts []tree) []geom.Vec {
	at := make([]geom.Vec, len(ts))
	for i, t := range ts {
		at[i] = t.at
	}
	return at
}

func boxOf(sector int) geom.AABB {
	x, y := float32(sector%cols*sectorSide), float32(sector/cols*sectorSide)
	return geom.NewAABB(geom.NewVec(x, y), geom.NewVec(x+sectorSide, y+sectorSide))
}

func middleOf(box geom.AABB) geom.Vec {
	return geom.NewVec((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2)
}

func sectorsOf[M interface{ world.Unfold | world.Fold }](ms []M) []int {
	var got []int
	for _, m := range ms {
		switch m := any(m).(type) {
		case world.Unfold:
			got = append(got, m.Sector)
		case world.Fold:
			got = append(got, m.Sector)
		}
	}
	slices.Sort(got)
	return got
}

// has the forest's trees stand in the sectors given alone
func (f *forest) standIn(t *testing.T, sectors ...int) {
	t.Helper()
	for s, ts := range f.bySector() {
		if !slices.Contains(sectors, s) {
			t.Errorf("%d trees in sector %d, want none", len(ts), s)
		}
	}
}

// has each of the forest's sectors given hold the trees its seed plants at density, round the
// stretch's plots within the sector's box
func (f *forest) plantedIn(t *testing.T, density uint64, sectors ...int) {
	t.Helper()
	whole, got := f.id(), f.bySector()
	for _, s := range sectors {
		var want []geom.Vec
		for k := range plots {
			if planted(whole, s, boxOf(s), k, density) {
				want = append(want, middleOf(boxOf(s)).Add(plotAt[k]))
			}
		}
		slices.SortFunc(want, func(a, b geom.Vec) int {
			if a.Y != b.Y {
				return int(a.Y - b.Y)
			}
			return int(a.X - b.X)
		})
		if have := places(got[s]); !slices.Equal(have, want) {
			t.Errorf("sector %d holds trees at %v, want %v", s, have, want)
		}
		for _, tr := range got[s] {
			if tr.of != whole {
				t.Errorf("a tree of sector %d is a part of %v, want the forest %v", s, tr.of, whole)
			}
		}
	}
}

func TestWholes_AFieldUnfoldsEachSectorComeToL3OnceAsleepInItsOwn(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256))
	f.start(t, false)
	f.run(2)
	if got := f.w.Read(0).LevelAt(forestSide/2, forestSide/2); got != lod.L6 {
		t.Fatalf("the forest's own sector at %v, want asleep at L6", got)
	}
	if got := sectorsOf(f.unfolded); !slices.Equal(got, []int{0, 1, 8}) {
		t.Fatalf("unfolded sectors %v, want the three within sight at L3", got)
	}
	for _, u := range f.unfolded {
		if u.ID != f.id() || u.Box != boxOf(u.Sector) {
			t.Errorf("the Unfold of sector %d of %v boxed %v, want of the forest %v boxed %v",
				u.Sector, u.ID, u.Box, f.id(), boxOf(u.Sector))
		}
	}
	f.plantedIn(t, 2, 0, 1, 8)
	f.standIn(t, 0, 1, 8)
	f.run(60)
	if got := f.times("unfold", 0); len(got) != 3 {
		t.Errorf("unfolded %d times in 62 ticks, want once a sector", len(got))
	}
}

func TestWholes_AFieldFoldsEachSectorAfterItsWaitItsPartsThereAlone(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256))
	f.start(t, false)
	f.run(2)
	before := f.bySector()
	f.look(geom.NewVec(1300, 256))
	from := f.w.Read(0).Clock().Time()
	at := f.until(t, "fold", 2*int(foldAfter/frame))
	if at < from+foldAfter || at > from+foldAfter+frame {
		t.Errorf("folded %v after sectors 0 and 8 went past the sight, want %v", at-from,
			foldAfter)
	}
	if got := sectorsOf(f.folded); !slices.Equal(got, []int{0, 8}) {
		t.Fatalf("folded sectors %v, want 0 and 8, past the sight", got)
	}
	for _, u := range f.folded {
		if want := len(before[u.Sector]); u.Parts != want || u.Box != boxOf(u.Sector) {
			t.Errorf("sector %d folded with %d parts boxed %v, want %d boxed %v", u.Sector,
				u.Parts, u.Box, want, boxOf(u.Sector))
		}
	}
	f.run(1)
	var told, want []uid.UID64
	for _, s := range f.heard {
		if s.rule == "folded" {
			told = append(told, s.by)
		}
	}
	for _, tr := range append(before[0], before[8]...) {
		want = append(want, tr.id)
	}
	slices.Sort(told)
	slices.Sort(want)
	if !slices.Equal(told, want) {
		t.Errorf("told %v they folded in, want the trees of sectors 0 and 8, %v", told, want)
	}
	after := f.bySector()
	if len(after[0])+len(after[8]) != 0 || !slices.Equal(places(after[1]), places(before[1])) {
		t.Errorf("after the fold %d trees in sectors 0 and 8 and %v in 1, want none and %v",
			len(after[0])+len(after[8]), places(after[1]), places(before[1]))
	}
	f.run(60)
	if n, m := len(f.times("fold", 0)), len(f.times("unfold", 0)); n != 2 || m != 3 {
		t.Errorf("%d folds and %d unfolds after a second, want 2 and 3 as before", n, m)
	}
}

func TestWholes_AFieldsSectorFoldingDrawsItsTreesOrItsPartNeverBoth(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256))
	f.start(t, false)
	f.w.Write().NewView(f.cam)
	f.run(2)
	if len(f.bySector()[0]) == 0 {
		t.Fatal("no tree stands in sector 0")
	}
	forest, sector0 := f.id(), boxOf(0)
	in0 := func(b geom.AABB) bool {
		m := middleOf(b)
		return m.X < sectorSide && m.Y < sectorSide
	}
	// drawn at L4 the sector waits to fold, its trees drawn as points till they go
	f.zoom(0.25)
	trees, pieces, folded := 0, 0, false
	for range 2*int(foldAfter/frame) + 4 {
		f.run(1)
		tr, piece := 0, false
		for _, d := range f.w.Published() {
			switch {
			case d.ID == forest && d.Piece && d.Box == sector0:
				piece = true
			case d.ID != forest && in0(d.Box):
				tr++
			}
		}
		if piece && tr > 0 {
			t.Fatalf("at %v the forest's part in sector 0 is drawn beside %d of its trees",
				f.w.Read(0).Clock().Time(), tr)
		}
		trees, folded = max(trees, tr), folded || len(f.folded) > 0
		if piece {
			pieces++
		}
	}
	if !folded || trees == 0 || pieces == 0 {
		t.Errorf("folded %v, its trees drawn %d at most, its part drawn %d frames; want the trees "+
			"drawn, then the part", folded, trees, pieces)
	}
}

// the forest's centre
func (f *forest) centre() geom.Vec {
	for f.forests.All(); f.forests.Next(); {
		for _, b := range f.base.Slice(f.forests.Cursor()) {
			return b.Pos.Center()
		}
	}
	return geom.Vec{}
}

func TestWholes_AFieldFoldsWhereItStands(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256))
	f.start(t, false)
	f.run(2)
	at := f.centre()
	f.look(geom.NewVec(1300, 256))
	f.until(t, "fold", 2*int(foldAfter/frame))
	if got := f.centre(); got != at {
		t.Errorf("the forest folded sectors 0 and 8 and stands at %v, want where it stood, %v",
			got, at)
	}
}

func TestWholes_AFieldFoldedAndUnfoldedAgainKeepsItsStateAndItsPlaces(t *testing.T) {
	f := newForest(t, geom.NewVec(1300, 512))
	f.start(t, false)
	f.run(2)
	f.plantedIn(t, 2, 1, 9)
	f.standIn(t, 1, 9)
	first := f.bySector()

	f.zoom(0.25)
	f.run(int(foldAfter/frame) + 2)
	if got := sectorsOf(f.folded); !slices.Equal(got, []int{1, 9}) || len(f.bySector()) != 0 {
		t.Fatalf("at L4 folded sectors %v with %d trees left, want 1 and 9, none left", got,
			len(f.bySector()))
	}
	f.zoom(0.5)
	f.run(2)
	again := f.bySector()
	for _, s := range []int{1, 9} {
		if !slices.Equal(places(again[s]), places(first[s])) {
			t.Errorf("sector %d unfolded again at %v, want where it stood, %v", s,
				places(again[s]), places(first[s]))
		}
	}
	fx := f.w.Write().Effects()
	if fx.Has(f.id(), fx.Named(thinnedEf)) {
		t.Fatal("the forest folded whole is thinned")
	}

	f.w.Write().Carrier().PutFrom(again[1][0].id, world.Despawn{})
	f.run(2)
	f.zoom(0.25)
	f.run(int(foldAfter/frame) + 10)
	if !fx.Has(f.id(), fx.Named(thinnedEf)) {
		t.Fatal("the forest folded short of a tree is not thinned")
	}
	f.zoom(0.5)
	f.run(2)
	f.plantedIn(t, 1, 1, 9)
}

func TestWholes_AFieldAsleepInItsOwnSectorTakesWhatItsOthersFoldInto(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256))
	f.start(t, false)
	f.run(2)
	f.w.Write().Carrier().PutFrom(f.bySector()[8][0].id, world.Despawn{})
	f.run(2)
	f.look(geom.NewVec(1300, 256))
	f.run(int(foldAfter/frame) + 2)
	fx := f.w.Write().Effects()
	if !fx.Has(f.id(), fx.Named(thinnedEf)) {
		t.Fatal("the forest asleep, sector 8 folded short of a tree, is not thinned")
	}
	f.look(geom.NewVec(256, 256))
	f.run(2)
	if got := f.w.Read(0).LevelAt(forestSide/2, forestSide/2); got != lod.L6 {
		t.Fatalf("the forest's own sector at %v, want asleep at L6", got)
	}
	f.plantedIn(t, 1, 0, 8)
	f.plantedIn(t, 2, 1)
}

func TestWholes_ASaveWhileAFieldWaitsToFoldKeepsEachSectorsWait(t *testing.T) {
	t.Chdir(t.TempDir())
	was := newForest(t, geom.NewVec(256, 256))
	was.start(t, false)
	was.run(2)
	was.look(geom.NewVec(1300, 256))
	from := was.w.Read(0).Clock().Time()
	was.run(int(foldAfter/frame) / 2)
	if err := was.eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := newForest(t, geom.NewVec(1300, 256))
	now.start(t, true)
	at := now.until(t, "fold", 2*int(foldAfter/frame))
	if at < from+foldAfter || at > from+foldAfter+frame {
		t.Errorf("loaded mid-wait, folded %v after sectors 0 and 8 went past the sight, want %v",
			at-from, foldAfter)
	}
	now.run(60)
	if got := sectorsOf(now.folded); !slices.Equal(got, []int{0, 8}) {
		t.Errorf("after the load folded sectors %v, want 0 and 8", got)
	}
	if got := now.times("unfold", 0); len(got) != 0 {
		t.Errorf("unfolded again at %v after the load, want never", got)
	}
}

func TestWholes_ALoadedFieldHasTheSectorsUnfoldedItHad(t *testing.T) {
	t.Chdir(t.TempDir())
	was := newForest(t, geom.NewVec(1300, 512))
	was.start(t, false)
	was.run(2)
	if got := sectorsOf(was.unfolded); !slices.Equal(got, []int{1, 9}) {
		t.Fatalf("unfolded sectors %v, want 1 and 9 within sight", got)
	}
	if err := was.eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := newForest(t, geom.NewVec(1300, 512))
	now.start(t, true)
	now.run(30)
	if n, m := len(now.unfolded), len(now.folded); n != 0 || m != 0 {
		t.Fatalf("%d unfolds and %d folds after the load, want none", n, m)
	}
	now.plantedIn(t, 2, 1, 9)
	now.standIn(t, 1, 9)
	now.look(geom.NewVec(256, 512))
	now.run(2)
	if got := sectorsOf(now.unfolded); !slices.Equal(got, []int{0, 8}) {
		t.Errorf("with all four within sight unfolded sectors %v, want 0 and 8 alone", got)
	}
	now.plantedIn(t, 2, 0, 1, 8, 9)
}
