package world_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
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
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

const (
	bandKind   = "band"
	memberKind = "member"
	troopKind  = "troop"
	campKind   = "camp"
	bandRole   = "band"
	memberRole = "member"
	meetRole   = "meeting"
	fewEf      = "few"
	gatheredEf = "gathered"
	// a band's members stand this far from its centre either way
	apart = 15
	frame = time.Second / 60
	// how long a band's sector lies at L4 before it folds
	foldAfter = 500 * time.Millisecond
)

type bandRow struct{ x, y float32 }

type memberRow struct{ dx, dy float32 }

// a band of four members, two once it has lost some, its centre at (256, 256) in a world of
// sectors 512 a side seen by one camera from above: a pixel a world unit is L2, a half L3, a
// quarter L4; with troops, aggregates on the abstract graph, and camps, aggregates off it
type folds struct {
	eng    *engine.Engine
	w      *world.Plugin
	ecs    *goke.ECS
	cam    camera.Camera
	orders heards
	heard  []said

	// the band's declared strength, zero for none
	strength int

	bands, members, abstract *goke.Query
	base, memberBase         goke.Comp[world.Base]
	aggregate                goke.Comp[world.Aggregate]
	partOf                   goke.Comp[world.PartOf]
	abstractBase             goke.Comp[world.Base]
	unfolded                 goke.OptComp[world.Unfolded]
	keen                     goke.OptComp[sector.Keen]
}

// a rule's word, by whom and on the clock's time after the tick it came in
type said struct {
	rule string
	by   uid.UID64
	at   time.Duration
}

var _ sector.Observers = (*folds)(nil)

func (f *folds) Observers(each func(cam camera.Camera)) { each(f.cam) }

func (f *folds) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f.bands = si.NewQueryBuilder(&f.base, &f.aggregate).
			Exclude(goke.Exclude[sector.Abstract]()).Optional(&f.unfolded, &f.keen).Build()
		f.members = si.NewQueryBuilder(&f.memberBase, &f.partOf).Build()
		f.abstract = si.NewQueryBuilder(&f.abstractBase).
			Include(goke.Include[sector.Abstract]()).Build()
	}}}
}

func member(dx, dy float32) act.Step {
	return act.Self.Do(world.SpawnPart{Entry: kind.EntryOf(memberKind, memberRow{dx, dy})})
}

func said1(rule string) act.Step { return act.Self.Do(heard{Rule: rule}) }

// the folds' stage, its camera at zoom, with a band at (256, 256) unless a save is loaded, and
// troops and camps at the xs given
func newFolds(t *testing.T, zoom float32, troops, camps []float32) *folds {
	t.Helper()
	f := &folds{cam: cameratest.NewFromSpace(4096, 512, 0)}
	f.cam.SetViewport(100, 100)
	f.cam.ZoomOut(1/zoom, 0, 0)
	f.cam.CenterOn(256, 256, 0)
	st := stage.New("folds").
		World(world.Config{
			Space:    world.SpaceCfg{Width: 4096, Height: 512},
			Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 5, MaxSize: 20},
			Sectors:  world.SectorsCfg{Buckets: 2},
			LOD: lod.Ladder{
				Scales: [lod.Levels - 1]float32{0, 0, 0.8, 0.4},
				Fold:   float32(foldAfter.Seconds()),
			},
		}).
		Plugins(func(w *world.Plugin) {
			f.w = w
			w.Write().Observe(f)
			if err := w.Carry(&f.orders); err != nil {
				t.Fatal(err)
			}
		}).
		Effects(func(ctx game.Initializer) {
			f.ecs = ctx.ECS()
			ctx.Setup(f)
			f.w.Write().Effects().Define(fewEf, effect.Spec{})
			f.w.Write().Effects().Define(gatheredEf, effect.Spec{})
		}).
		Rules(func() { f.rules() }).
		Kinds(func() { f.kinds() }).
		Spawn(func() {
			f.w.Write().Seed(kind.Named[bandRow](f.w.Kinds(), bandKind).Entry(bandRow{256, 256}))
			for _, x := range troops {
				f.w.Write().Seed(kind.Named[bandRow](f.w.Kinds(), troopKind).Entry(bandRow{x, 256}))
			}
			for _, x := range camps {
				f.w.Write().Seed(kind.Named[bandRow](f.w.Kinds(), campKind).Entry(bandRow{x, 256}))
			}
		}).
		Update(func(ctx goke.RunCtx, dt time.Duration) { f.w.RunPlan(ctx, dt) })
	f.eng = engine.NewEngine(oneStageGame{stage: st})
	return f
}

// the band unfolds into four members round it, or two under few; it folds in their count, few
// under three; each member folds in by gathering it and going
func (f *folds) rules() {
	few := f.w.Write().Effects().Named(fewEf)
	f.w.Roles().Define(bandRole,
		act.Branch[world.Unfold]{
			Name: "unfold whole",
			When: cond.Self.If(cond.Not(few)),
			Then: act.Steps(said1("unfold"), member(-apart, -apart), member(apart, -apart),
				member(-apart, apart), member(apart, apart)),
		},
		act.Branch[world.Unfold]{
			Name: "unfold few",
			When: cond.Self.If(few),
			Then: act.Steps(said1("unfold"), member(-apart, 0), member(apart, 0)),
		},
		act.Branch[world.Fold]{
			Name: "fold whole",
			When: cond.Self.If(world.Parts(3)),
			Then: act.Steps(said1("fold"), act.Self.Take(few)),
		},
		act.Branch[world.Fold]{
			Name: "fold few",
			When: cond.Self.If(cond.Not(world.Parts(3))),
			Then: act.Steps(said1("fold"), act.Self.Put(few)),
		})
	gathered := f.w.Write().Effects().Named(gatheredEf)
	f.w.Roles().Define(memberRole, act.Branch[world.Folding]{
		Name: "fold in",
		When: cond.Self.If(cond.Always),
		Then: act.Steps(said1("folded"), act.Other.Put(gathered), act.Self.Do(world.Despawn{})),
	})
	f.w.Roles().Define(meetRole, act.Branch[world.Met]{
		Name: "meet",
		When: cond.Self.If(cond.Always),
		Then: said1("met"),
	})
}

func (f *folds) kinds() {
	box := func(r bandRow) world.Position {
		return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x-10, r.y-10), 20, 20)}
	}
	roles := f.w.Roles()
	kind.Define[bandRow](f.w.Kinds(), bandKind, kind.Spec{
		comp.Load(box),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{Strength: f.strength}),
		ceremony.Plays(roles.Named(bandRole)),
	})
	kind.Define[memberRow](f.w.Kinds(), memberKind, kind.Spec{
		comp.Load(func(r memberRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.dx-5, r.dy-5), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
		ceremony.Plays(roles.Named(memberRole)),
	})
	kind.Define[bandRow](f.w.Kinds(), troopKind, kind.Spec{
		comp.Load(box),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{}),
		comp.Const(sector.Abstract{}),
		ceremony.Plays(roles.Named(meetRole)),
	})
	kind.Define[bandRow](f.w.Kinds(), campKind, kind.Spec{
		comp.Load(box),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{}),
		ceremony.Plays(roles.Named(meetRole)),
	})
}

func (f *folds) start(t *testing.T, load bool) {
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

func (f *folds) run(ticks int) {
	for range ticks {
		f.ecs.Tick(frame)
		now := f.w.Read(0).Clock().Time()
		f.orders.Drain(func(i command.Issued[heard]) {
			f.heard = append(f.heard, said{rule: i.Command.Rule, by: i.Entity, at: now})
		})
	}
}

// the times rule was heard since the k-th word
func (f *folds) times(rule string, k int) []time.Duration {
	var at []time.Duration
	for _, s := range f.heard[k:] {
		if s.rule == rule {
			at = append(at, s.at)
		}
	}
	return at
}

// runs till rule is heard, at most ticks, and is when
func (f *folds) until(t *testing.T, rule string, ticks int) time.Duration {
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

func (f *folds) band() (uid.UID64, geom.Vec) {
	for f.bands.All(); f.bands.Next(); {
		cur := f.bands.Cursor()
		for i, id := range cur.IDs {
			return id, f.base.Slice(cur)[i].Pos.Center()
		}
	}
	return 0, geom.Vec{}
}

// the band's members, by id, and their centroid
func (f *folds) parts(whole uid.UID64) ([]uid.UID64, geom.Vec) {
	var ids []uid.UID64
	var sum geom.Vec
	for f.members.All(); f.members.Next(); {
		cur := f.members.Cursor()
		bases, of := f.memberBase.Slice(cur), f.partOf.Slice(cur)
		for i, id := range cur.IDs {
			if of[i].Whole == whole {
				ids = append(ids, id)
				c := bases[i].Pos.Center()
				sum = geom.NewVec(sum.X+c.X, sum.Y+c.Y)
			}
		}
	}
	if len(ids) == 0 {
		return nil, sum
	}
	n := float32(len(ids))
	return ids, geom.NewVec(sum.X/n, sum.Y/n)
}

func near(a, b geom.Vec) bool {
	return f32.Abs(a.X-b.X) <= f32.Eps(b.X) && f32.Abs(a.Y-b.Y) <= f32.Eps(b.Y)
}

// has the band unfolded into n members round it
func (f *folds) unfoldedInto(t *testing.T, n int) {
	t.Helper()
	id, at := f.band()
	got, centroid := f.parts(id)
	if len(got) != n || !near(centroid, at) {
		t.Fatalf("the band at %v unfolded into %d members round %v, want %d round it", at,
			len(got), centroid, n)
	}
}

// zooms the camera to zoom and has it on the band again
func (f *folds) zoom(zoom float32) {
	f.cam.ZoomIn(zoom/f.cam.Zoom(), 0, 0)
	f.cam.CenterOn(256, 256, 0)
}

// folds the band after its sector has lain at L4 for foldAfter, not before, and has its members
// gone the step after
func (f *folds) foldsAfterTheWait(t *testing.T) {
	t.Helper()
	f.zoom(0.25)
	from := f.w.Read(0).Clock().Time()
	at := f.until(t, "fold", 2*int(foldAfter/frame))
	if at < from+foldAfter || at > from+foldAfter+frame {
		t.Errorf("the band folded %v after its sector came to L4, want %v", at-from, foldAfter)
	}
	f.run(1)
	id, _ := f.band()
	fx := f.w.Write().Effects()
	if got, _ := f.parts(id); len(f.times("folded", 0)) == 0 || len(got) != 0 {
		t.Errorf("%d members told they folded in, %d still there, want all gone",
			len(f.times("folded", 0)), len(got))
	}
	if !fx.Has(id, fx.Named(gatheredEf)) {
		t.Error("the band is not under what its members put on it folding in")
	}
}

func TestWholes_FoldUndoesAnUnfoldTheAggregatesStateAndCentroidKept(t *testing.T) {
	f := newFolds(t, 1, nil, nil)
	f.start(t, false)
	f.run(2)
	f.unfoldedInto(t, 4)
	f.run(60)
	if got := f.times("unfold", 0); len(got) != 1 {
		t.Fatalf("unfolded at %v at L2, want once", got)
	}
	f.foldsAfterTheWait(t)
	if id, _ := f.band(); f.w.Write().Effects().Has(id, f.w.Write().Effects().Named(fewEf)) {
		t.Error("the band folded from four members is under few")
	}

	k := len(f.heard)
	f.zoom(0.5)
	f.run(2)
	f.zoom(1)
	f.run(30)
	if got := f.times("unfold", k); len(got) != 1 {
		t.Errorf("came down through L3 to L2, unfolded at %v, want once", got)
	}
	f.unfoldedInto(t, 4)

	id, _ := f.band()
	lost, _ := f.parts(id)
	for _, m := range lost[:2] {
		f.w.Write().Carrier().PutFrom(m, world.Despawn{})
	}
	f.run(2)
	f.foldsAfterTheWait(t)
	if !f.w.Write().Effects().Has(id, f.w.Write().Effects().Named(fewEf)) {
		t.Error("the band folded from two members is not under few")
	}
	f.zoom(1)
	f.run(2)
	f.unfoldedInto(t, 2)
}

// has each part of whole stand centred at the next of at
func (f *folds) walk(whole uid.UID64, at ...geom.Vec) {
	k := 0
	for f.members.All(); f.members.Next(); {
		cur := f.members.Cursor()
		bases, of := f.memberBase.Slice(cur), f.partOf.Slice(cur)
		for i := range cur.IDs {
			if of[i].Whole == whole {
				bases[i].Pos.AABB = plane.NewAABB(at[k].Sub(geom.NewVec(5, 5)), 10, 10)
				k++
			}
		}
	}
}

func TestWholes_APointAggregateFoldsWhereItsPartsStandStayingWithNoneLeft(t *testing.T) {
	f := newFolds(t, 1, nil, nil)
	f.start(t, false)
	f.run(2)
	id, _ := f.band()
	f.walk(id, geom.NewVec(300, 270), geom.NewVec(340, 270), geom.NewVec(300, 320),
		geom.NewVec(330, 330))
	_, want := f.parts(id)
	f.zoom(0.25)
	f.until(t, "fold", 2*int(foldAfter/frame))
	if _, at := f.band(); !near(at, want) {
		t.Fatalf("the band folded at %v, want its members' centroid %v", at, want)
	}
	found := false
	round := geom.NewAABB(want.Sub(geom.NewVec(1, 1)), want.Add(geom.NewVec(1, 1)))
	f.w.Read(0).Space().Query(round, aabbworld.AnyCapability, func(e uid.UID64) {
		found = found || e == id
	})
	if !found {
		t.Error("the space finds the band folded not where it stands")
	}

	f.zoom(1)
	f.run(2)
	f.unfoldedInto(t, 4)
	members, _ := f.parts(id)
	for _, m := range members {
		f.w.Write().Carrier().PutFrom(m, world.Despawn{})
	}
	f.run(2)
	f.zoom(0.25)
	f.until(t, "fold", 2*int(foldAfter/frame))
	if _, at := f.band(); !near(at, want) {
		t.Errorf("the band folded with no member left at %v, want where it stood, %v", at, want)
	}
}

func TestWholes_AFoldWritesAStrengthDeclaredAsThePartsFoldedInLeavingNoneAlone(t *testing.T) {
	for _, declared := range []int{9, 0} {
		f := newFolds(t, 1, nil, nil)
		f.strength = declared
		f.start(t, false)
		f.run(2)
		id, _ := f.band()
		f.unfoldedInto(t, 4)
		f.zoom(0.25)
		f.until(t, "fold", 2*int(foldAfter/frame))
		if !f.bands.Seek(id) {
			t.Fatal("the band is gone")
		}
		want := 0
		if declared > 0 {
			want = 4
		}
		if got := f.aggregate.At(f.bands.Cursor()).Strength; got != want {
			t.Errorf("declared %d, the band folded with four members has strength %d, want %d",
				declared, got, want)
		}
	}
}

func TestWholes_APointAggregateUnfoldedStandsAtTheMiddleOfItsPartsASaveBetween(t *testing.T) {
	t.Chdir(t.TempDir())
	was := newFolds(t, 1, nil, nil)
	was.start(t, false)
	was.run(2)
	id, _ := was.band()
	east := geom.NewVec(356, 256)
	was.walk(id, geom.NewVec(341, 241), geom.NewVec(371, 241), geom.NewVec(341, 271),
		geom.NewVec(371, 271))
	was.run(1)
	if _, at := was.band(); !near(at, east) {
		t.Fatalf("the band at %v, its members walked 100 east, want at their middle %v", at, east)
	}
	found := false
	round := geom.NewAABB(east.Sub(geom.NewVec(1, 1)), east.Add(geom.NewVec(1, 1)))
	was.w.Read(0).Space().Query(round, aabbworld.AnyCapability, func(e uid.UID64) {
		found = found || e == id
	})
	if !found {
		t.Error("the space finds the band not at its members' middle")
	}
	if err := was.eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := newFolds(t, 1, nil, nil)
	now.start(t, true)
	now.run(2)
	if _, at := now.band(); !near(at, east) || len(now.times("unfold", 0)) != 0 {
		t.Errorf("loaded, the band at %v unfolded %d times again, want at %v, never again", at,
			len(now.times("unfold", 0)), east)
	}
	if members, centroid := now.parts(id); len(members) != 4 || !near(centroid, east) {
		t.Errorf("loaded, %d members round %v, want the four round %v", len(members), centroid,
			east)
	}
}

// whether the aggregate id, off the abstract graph, carries Unfolded, and the Keen it carries
func (f *folds) marks(id uid.UID64) (unfolded bool, keen *sector.Keen) {
	if !f.bands.Seek(id) {
		return false, nil
	}
	return f.unfolded.Present(f.bands.Cursor()), f.keen.At(f.bands.Cursor())
}

func TestWholes_AnAggregateIsUnfoldedFromItsUnfoldToItsFoldASaveBetween(t *testing.T) {
	t.Chdir(t.TempDir())
	was := newFolds(t, 1, nil, nil)
	was.start(t, false)
	if id, _ := was.band(); func() bool { on, _ := was.marks(id); return on }() {
		t.Fatal("the band is Unfolded before its Unfold")
	}
	was.run(2)
	id, _ := was.band()
	if on, _ := was.marks(id); !on {
		t.Fatal("unfolded, the band carries no Unfolded")
	}
	if err := was.eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := newFolds(t, 1, nil, nil)
	now.start(t, true)
	now.run(2)
	if on, _ := now.marks(id); !on || len(now.times("unfold", 0)) != 0 {
		t.Fatalf("loaded unfolded, the band carries Unfolded %v, unfolded %d times again; want "+
			"it carried, never unfolded again", on, len(now.times("unfold", 0)))
	}
	now.zoom(0.25)
	now.until(t, "fold", 2*int(foldAfter/frame)+2)
	if on, _ := now.marks(id); !on {
		t.Error("the band carries no Unfolded the step it folds, its members still there")
	}
	now.run(1)
	if on, _ := now.marks(id); on {
		t.Error("folded, the band carries Unfolded still")
	}
}

func TestKeen_AnEntityKeepsItsSectorKeenTillItCalms(t *testing.T) {
	f := newFolds(t, 0.25, nil, []float32{3000})
	f.start(t, false)
	f.run(2)
	band, _ := f.band()
	camp := slices.DeleteFunc(f.camps(), func(id uid.UID64) bool { return id == band })[0]
	if level := f.w.Read(0).LevelAt(3000, 256); level != lod.L4 {
		t.Fatalf("the camp lies at %v, want L4", level)
	}
	f.w.Write().Carrier().PutFrom(camp, world.Keen{Level: lod.L3})
	f.run(2)
	if _, keen := f.marks(camp); keen == nil || keen.Level != lod.L3 ||
		f.w.Read(0).LevelAt(3000, 256) != lod.L3 {
		t.Fatalf("keen at L3, the camp carries %v, its sector lies at %v; want L3 both", keen,
			f.w.Read(0).LevelAt(3000, 256))
	}
	f.w.Write().Carrier().PutFrom(camp, world.Calm{})
	f.run(2)
	if _, keen := f.marks(camp); keen != nil || f.w.Read(0).LevelAt(3000, 256) != lod.L4 {
		t.Errorf("calm, the camp carries %v, its sector lies at %v; want no Keen, L4", keen,
			f.w.Read(0).LevelAt(3000, 256))
	}
}

func TestWholes_NeverFoldWhileTheSectorComesBackToL3InTime(t *testing.T) {
	f := newFolds(t, 1, nil, nil)
	f.start(t, false)
	f.run(2)
	for range 4 {
		f.zoom(0.25)
		f.run(int(foldAfter/frame) - 2)
		f.zoom(0.5)
		f.run(2)
	}
	if got := f.times("fold", 0); len(got) != 0 {
		t.Errorf("folded at %v, each stay at L4 short of %v, want never", got, foldAfter)
	}
	if got := f.times("unfold", 0); len(got) != 1 {
		t.Errorf("unfolded at %v, want once", got)
	}
}

func TestWholes_ASaveWhileTheAggregateWaitsToFoldKeepsTheWait(t *testing.T) {
	t.Chdir(t.TempDir())
	was := newFolds(t, 1, nil, nil)
	was.start(t, false)
	was.run(2)
	was.zoom(0.25)
	from := was.w.Read(0).Clock().Time()
	was.run(int(foldAfter/frame) / 2)
	if err := was.eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := newFolds(t, 0.25, nil, nil)
	now.start(t, true)
	at := now.until(t, "fold", 2*int(foldAfter/frame))
	if at < from+foldAfter || at > from+foldAfter+frame {
		t.Errorf("loaded mid-wait, the band folded %v after its sector came to L4, want %v",
			at-from, foldAfter)
	}
	if got := now.times("unfold", 0); len(got) != 0 {
		t.Errorf("unfolded again at %v after the load, want never", got)
	}
}

// the troop or camp whose box is centred at x moved to be centred at to
func (f *folds) move(x, to float32) {
	for f.abstract.All(); f.abstract.Next(); {
		bases := f.abstractBase.Slice(f.abstract.Cursor())
		for i := range bases {
			if bases[i].Pos.Center().X == x {
				bases[i].Pos.AABB = plane.NewAABB(geom.NewVec(to-10, 246), 20, 20)
			}
		}
	}
}

// how many Met were heard since the k-th word, and whether a camp was told one
func (f *folds) met(k int, camps ...uid.UID64) (n int, camp bool) {
	for _, s := range f.heard[k:] {
		if s.rule == "met" {
			n++
			camp = camp || slices.Contains(camps, s.by)
		}
	}
	return n, camp
}

// the camps, off the abstract graph
func (f *folds) camps() []uid.UID64 {
	var ids []uid.UID64
	for f.bands.All(); f.bands.Next(); {
		ids = append(ids, f.bands.Cursor().IDs...)
	}
	return ids
}

func TestWholes_MeetTwoOnTheAbstractGraphOnceTillTheyComeApart(t *testing.T) {
	f := newFolds(t, 1, []float32{1000, 1010}, []float32{1500, 1510})
	f.start(t, false)
	f.run(30)
	if n, camp := f.met(0, f.camps()...); n != 1 || camp {
		t.Fatalf("Met %d times overlapping for 30 ticks, a camp among them %v; want the troops "+
			"once, no camp", n, camp)
	}
	k := len(f.heard)
	f.move(1010, 1100)
	f.run(5)
	if n, _ := f.met(k); n != 0 {
		t.Errorf("Met %d times coming apart, want never", n)
	}
	f.move(1100, 1015)
	f.run(5)
	if n, _ := f.met(k); n != 1 {
		t.Errorf("Met %d times overlapping again, want once", n)
	}
}

func TestWholes_AMeetingOutlastsASave(t *testing.T) {
	t.Chdir(t.TempDir())
	was := newFolds(t, 1, []float32{1000, 1010}, nil)
	was.start(t, false)
	was.run(5)
	if n, _ := was.met(0); n != 1 {
		t.Fatalf("Met %d times, want once", n)
	}
	if err := was.eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := newFolds(t, 1, nil, nil)
	now.start(t, true)
	now.run(5)
	if n, _ := now.met(0); n != 0 {
		t.Errorf("Met %d times again after the load, want never", n)
	}
}

func TestSpawnPart_IsRefusedGivenByAnyButAnAggregate(t *testing.T) {
	f := newFolds(t, 1, nil, nil)
	f.start(t, false)
	f.run(2)
	id, _ := f.band()
	members, _ := f.parts(id)
	count := f.w.Res.Telemetry.Count
	entry := kind.EntryOf(memberKind, memberRow{})
	f.w.Write().Carrier().PutFrom(members[0], world.SpawnPart{Entry: entry})
	f.w.Write().Carrier().Put(1, world.SpawnPart{Entry: entry})
	f.run(2)
	if got := f.w.Res.Telemetry.Count; got != count {
		t.Errorf("%d entities after parts given by a member and a player, want %d as before",
			got, count)
	}
}
