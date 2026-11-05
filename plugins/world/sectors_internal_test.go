package world

import (
	"maps"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// the cameras the rig's world lives round
type watchers struct{ cams []camera.Camera }

var _ sector.Observers = (*watchers)(nil)

func (w *watchers) Observers(each func(cam camera.Camera)) {
	for _, cam := range w.cams {
		each(cam)
	}
}

type sectorRig struct {
	p      *Plugin
	things kind.Of[thing]
	keens  kind.Of[keenThing]
	ecs    *goke.ECS
	watch  *watchers
	all    *goke.Query
	base   *goke.Comp[Base]
	// by the level the entities carry, and by how many levels coarser than the finest seen
	marked [lod.Levels]*goke.Query
	far    [lod.Omitted]*goke.Query
	keen   *goke.Query
	keenOf goke.Comp[sector.Keen]
	keenID goke.CompID
	// those travelling the abstract graph
	abstract   *goke.Query
	abstractID goke.CompID
	// by the paces of paced
	paced [4]pacedQuery
	// Within the close, the standard and the strategic scale, and the close one's detail at Exact
	within [3]*goke.Query
	detail pacedQuery
}

type pacedQuery struct {
	q    *goke.Query
	base goke.Comp[Base]
}

// a thing keeping its sector and those round it ticked at L4 at most
type keenThing struct{ x float32 }

var paced = [4]lod.Paces{lod.Steady, lod.Slowing(4), lod.Exact, lod.Slowing(2, 4, 8)}

// a ladder seeing 600 at L2 from half a pixel a world unit; as the rig's two observers see it,
// L2 within 600 of them, L3 within 1200, L6 beyond
var twoLevels = lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5}, Sight: 600}

// a world 4096x512 cut into sectors 512 a side, its ladder as given, and two observers both
// looking at (100, 256): one a pixel a world unit, the other a quarter of a pixel seeing twice as
// far
func sectorWorld(t *testing.T, ladder lod.Ladder) *sectorRig {
	t.Helper()
	return sectorWorldOf(t, 0, 2, ladder)
}

// sectorWorld with its edges and its sectors' buckets, none uncut
func sectorWorldOf(t *testing.T, edges aabbworld.Edges, buckets int, ladder lod.Ladder) *sectorRig {
	t.Helper()
	r := newSectorRig(edges, buckets, ladder)
	r.setup(t, "")
	return r
}

// the rig's plugin and observers, its ECS not yet made
func newSectorRig(edges aabbworld.Edges, buckets int, ladder lod.Ladder) *sectorRig {
	near := cameratest.NewFromSpace(4096, 512, edges)
	far := cameratest.NewFromSpace(4096, 512, edges)
	near.SetViewport(100, 100)
	far.SetViewport(100, 100)
	far.ZoomOut(4, 0, 0)
	far.(interface{ See(factor float32) }).See(2)
	r := &sectorRig{watch: &watchers{cams: []camera.Camera{near, far}}}
	r.observe(100)
	r.p = NewPlugin(Config{
		Space:    SpaceCfg{Width: 4096, Height: 512, Edges: edges},
		Entities: EntitiesCfg{MaxCount: 8, MinSize: 5, MaxSize: 20},
		Sectors:  SectorsCfg{Buckets: buckets},
		LOD:      ladder,
	})
	r.p.write.Observe(r.watch)
	kind.Define[thing](r.p.Kinds(), "thing", kind.Spec{
		comp.Load(func(t thing) Position {
			return Position{AABB: plane.NewAABB(geom.NewVec(t.x, t.y), t.size, t.size)}
		}),
		comp.Const(Velocity{}),
	})
	kind.Define[keenThing](r.p.Kinds(), "keen", kind.Spec{
		comp.Load(func(t keenThing) Position {
			return Position{AABB: plane.NewAABB(geom.NewVec(t.x, 250), 10, 10)}
		}),
		comp.Const(Velocity{}),
		comp.Const(sector.Keen{Level: lod.L4}),
	})
	r.things = kind.Named[thing](r.p.Kinds(), "thing")
	r.keens = kind.Named[keenThing](r.p.Kinds(), "keen")
	return r
}

// makes the rig's ECS, from the save at load where one is given
func (r *sectorRig) setup(t *testing.T, load string) {
	t.Helper()
	r.base = new(goke.Comp[Base])
	var b goke.Comp[Base]
	r.ecs = goke.New()
	systems := r.p.module.SetupSystems()
	if load != "" {
		if err := r.ecs.Load(load, r.p.module.LoadComps()...); err != nil {
			t.Fatal(err)
		}
		systems = append(systems, r.p.module.PostLoad())
	}
	r.ecs.Setup(append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.all = si.NewQueryBuilder(r.base).Build()
		r.marked = [lod.Levels]*goke.Query{
			si.NewQueryBuilder(&b).Include(goke.Include[sector.L0]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.L1]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.L2]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.L3]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.L4]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.L5]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.L6]()).Build(),
		}
		r.far = [lod.Omitted]*goke.Query{
			si.NewQueryBuilder(&b).Exclude(goke.Exclude[sector.Far1](),
				goke.Exclude[sector.Far2](), goke.Exclude[sector.Far3](),
				goke.Exclude[sector.Far4](), goke.Exclude[sector.Far5]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.Far1]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.Far2]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.Far3]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.Far4]()).Build(),
			si.NewQueryBuilder(&b).Include(goke.Include[sector.Far5]()).Build(),
		}
		r.keen = si.NewQueryBuilder(&r.keenOf).Build()
		r.keenID = si.RegComp[sector.Keen]()
		r.abstract = si.NewQueryBuilder(&b).Include(goke.Include[sector.Abstract]()).Build()
		r.abstractID = si.RegComp[sector.Abstract]()
		for k := range r.paced {
			pq := &r.paced[k]
			pq.q = si.NewQueryBuilder(&pq.base).With(r.p.Pace(paced[k])).Build()
		}
		for k, first := range []lod.Level{lod.Close, lod.Standard, lod.Strategic} {
			r.within[k] = si.NewQueryBuilder(&b).With(r.p.Within(first)).Build()
		}
		r.detail.q = si.NewQueryBuilder(&r.detail.base).
			With(r.p.Within(lod.Close), r.p.Pace(lod.Exact)).Build()
	}})...)
	r.p.module.RegSystems(r.ecs)
}

// has the observers look at (x, 256)
func (r *sectorRig) observe(x float32) {
	for _, cam := range r.watch.cams {
		cam.CenterOn(x, 256, 0)
	}
}

func (r *sectorRig) spawn(xs ...float32) {
	for _, x := range xs {
		r.p.write.Spawn(r.things.Entry(thing{x: x, y: 250, size: 10}))
	}
	r.run()
}

func (r *sectorRig) run() { run(r.ecs, r.p.module, func(*goke.CmdBuf) {}) }

// the entities carrying the level's mark, each by its x
func (r *sectorRig) at(level lod.Level) []float32 { return r.found(r.marked[level]) }

// the entities q finds, each by its x
func (r *sectorRig) found(q *goke.Query) []float32 {
	xs := map[uid.UID64]float32{}
	for r.all.All(); r.all.Next(); {
		cur := r.all.Cursor()
		for j, id := range cur.IDs {
			xs[id] = r.base.Slice(cur)[j].Pos.TopLeft.X
		}
	}
	var found []float32
	for id := range living(q) {
		found = append(found, xs[id])
	}
	return found
}

// the entities by the level they carry, each by its x
func (r *sectorRig) levels() (near, second, off []float32) {
	return r.at(lod.L2), r.at(lod.L3), r.at(lod.L6)
}

func (r *sectorRig) moveTo(x float32, to float32) {
	for r.all.All(); r.all.Next(); {
		cur := r.all.Cursor()
		bases := r.base.Slice(cur)
		for j := range cur.IDs {
			if bases[j].Pos.TopLeft.X == x {
				bases[j].Pos.AABB = plane.NewAABB(geom.NewVec(to, 250), 10, 10)
			}
		}
	}
}

func same(got []float32, want ...float32) bool {
	if len(got) != len(want) {
		return false
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			found = found || g == w
		}
		if !found {
			return false
		}
	}
	return true
}

func TestSectors_MarkEachEntityByItsSectorsLevel(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 900, 1200, 2000, 3500)
	r.run()
	near, second, off := r.levels()
	if !same(near, 100, 900) || !same(second, 1200) || !same(off, 2000, 3500) {
		t.Errorf("L2 %v, L3 %v, L6 %v; want 100 and 900, 1200, 2000 and 3500", near, second, off)
	}
}

func TestSectors_FollowTheObservers(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 1200, 2000, 3500)
	r.run()
	r.observe(3500)
	r.run()
	near, second, off := r.levels()
	if !same(near, 3500) || len(second) != 0 || !same(off, 100, 1200, 2000) {
		t.Errorf("L2 %v, L3 %v, L6 %v; want 3500, none, the rest", near, second, off)
	}
}

func TestSectors_TickAWorldWithoutObserversAtTheLaddersUnobservedLevel(t *testing.T) {
	for _, c := range []struct {
		unobserved lod.Fallback
		want       lod.Level
	}{
		{0, lod.L4},
		{lod.UnobservedL0, lod.L0},
		{lod.UnobservedL1, lod.L1},
		{lod.UnobservedL2, lod.L2},
		{lod.UnobservedL3, lod.L3},
		{lod.UnobservedL5, lod.L5},
		{lod.UnobservedL6, lod.L6},
	} {
		ladder := twoLevels
		ladder.Unobserved = c.unobserved
		r := sectorWorld(t, ladder)
		r.watch.cams = nil
		r.spawn(100, 2000, 3500)
		r.run()
		if got := r.at(c.want); !same(got, 100, 2000, 3500) {
			t.Errorf("unobserved at %v: %v there, want every entity", c.want, got)
		}
	}
}

func TestSectors_AKeenEntityKeepsItsSectorsFinerThanAWorldWithoutObservers(t *testing.T) {
	ladder := twoLevels
	ladder.Unobserved = lod.UnobservedL5
	r := sectorWorld(t, ladder)
	r.watch.cams = nil
	r.spawn(100, 1200, 2300, 3500)
	r.p.write.Spawn(r.keens.Entry(keenThing{x: 2000}))
	r.run()
	r.run()
	want := map[lod.Level][]float32{lod.L4: {1200, 2000, 2300}, lod.L5: {100, 3500}}
	if got := r.byLevel(); !sameLevels(got, want) {
		t.Errorf("entities %v, want %v: the keen one's sector and those round it at L4", got, want)
	}
}

func TestSectors_MoveTheMarkOfAnEntityCrossingIntoAnotherLevel(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100)
	r.run()
	r.moveTo(100, 1200)
	r.run()
	near, second, _ := r.levels()
	if len(near) != 0 || !same(second, 1200) {
		t.Errorf("L2 %v, L3 %v; want the one walked off into L3", near, second)
	}
	r.moveTo(1200, 2000)
	r.run()
	if _, _, off := r.levels(); !same(off, 2000) {
		t.Errorf("L6 %v, want the one walked past the sight", off)
	}
}

func TestSectors_MarkAWorldOfTheZeroLadderStandard(t *testing.T) {
	r := sectorWorld(t, lod.Ladder{})
	r.spawn(100, 2000, 3500)
	r.run()
	near, second, off := r.levels()
	if len(near) != 3 || len(second) != 0 || len(off) != 0 {
		t.Errorf("L2 %v, L3 %v, L6 %v; want every entity at L2", near, second, off)
	}
}

func TestSectors_ReportTheSectorsAndEntitiesByLevel(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 1200, 2000, 3500)
	r.run()
	r.run()
	got := map[string]string{}
	r.p.read.Sectors().Report(func(label, value string) { got[label] = value })
	if want := "L0 0\nL1 0\nL2 2\nL3 1\nL4 0\nL5 0\nL6 5"; got["Sectors"] != want {
		t.Errorf("sectors %q, want %q", got["Sectors"], want)
	}
	if want := "L0 0\nL1 0\nL2 1\nL3 1\nL4 0\nL5 0\nL6 2"; got["Entities by level"] != want {
		t.Errorf("entities %q, want %q", got["Entities by level"], want)
	}
}

func TestSectors_SwitchedOffLiveEverywhereAndOnAgainGoBackAtOnce(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 1200, 2000, 3500)
	r.run()
	r.p.module.commands.Put(command.Nobody, SwitchBands{On: false})
	r.run()
	r.run()
	near, second, off := r.levels()
	if r.p.read.Banded() || len(near) != 4 || len(second) != 0 || len(off) != 0 {
		t.Errorf("levels off (on %v): L2 %v, L3 %v, L6 %v; want every entity at L2",
			r.p.read.Banded(), near, second, off)
	}
	got := map[string]string{}
	r.p.read.Sectors().Report(func(label, value string) { got[label] = value })
	if want := "L0 0\nL1 0\nL2 8\nL3 0\nL4 0\nL5 0\nL6 0\nlevels off"; got["Sectors"] != want {
		t.Errorf("sectors %q, want %q", got["Sectors"], want)
	}
	r.p.module.commands.Put(command.Nobody, SwitchBands{On: true})
	r.run()
	r.run()
	near, second, off = r.levels()
	if !r.p.read.Banded() || !same(near, 100) || !same(second, 1200) || !same(off, 2000, 3500) {
		t.Errorf("levels on (on %v): L2 %v, L3 %v, L6 %v; want 100, 1200, 2000 and 3500",
			r.p.read.Banded(), near, second, off)
	}
}

func TestSectors_SettingsOnlyInAWorldCutIntoSectorsAndNoKeys(t *testing.T) {
	levels := func(p *Plugin) (control.Switch, bool) {
		for _, s := range p.Settings() {
			if sw, ok := s.(control.Switch); ok && sw.Label == "Sectors' levels" {
				return sw, true
			}
		}
		return control.Switch{}, false
	}
	p := sectorWorld(t, twoLevels).p
	if sw, ok := levels(p); !ok || !sw.Get() || sw.Set(false) != (SwitchBands{On: false}) {
		t.Errorf("the levels' setting found %v, want it on and setting SwitchBands off", ok)
	}
	for _, k := range p.DefaultBindings() {
		if k.Type() == reflect.TypeFor[SwitchBands]() ||
			k.Type() == reflect.TypeFor[SwitchSectors]() {
			t.Errorf("%v is bound to %v, want the settings screen alone", k.Type(), k.Trigger)
		}
	}
	if _, ok := levels(sectorWorldOf(t, 0, 0, lod.Ladder{}).p); ok {
		t.Error("a world of no sectors has the levels' setting")
	}
}

func TestSectors_RefuseALadderThatCannotBeOrHasNoSectors(t *testing.T) {
	for name, c := range map[string]struct {
		buckets int
		ladder  lod.Ladder
	}{
		"rising":     {2, lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 1}}},
		"fog past":   {2, lod.Ladder{Sight: 100, Fog: 200}},
		"no sectors": {0, twoLevels},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("the world took the ladder")
				}
			}()
			sectorWorldOf(t, 0, c.buckets, c.ladder)
		})
	}
}

// the entities going this step at the paces paced[k], each by its x with the step it is given
func (r *sectorRig) going(k int) map[float32]time.Duration {
	pq := &r.paced[k]
	got := map[float32]time.Duration{}
	for pq.q.Paced(10 * time.Millisecond); pq.q.Next(); {
		for _, b := range pq.base.Slice(pq.q.Cursor()) {
			got[b.Pos.TopLeft.X] = pq.q.Dt()
		}
	}
	return got
}

func TestPace_GoesAtTheLevelsAsItsPacesSay(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 1200, 2000)
	r.run()
	const d = 10 * time.Millisecond
	for range 4 {
		r.run()
		slowed := map[float32]time.Duration{100: d}
		if r.p.read.Clock().Steps()%4 == 3 {
			slowed[1200] = 4 * d
		}
		for k, want := range [3]map[float32]time.Duration{{100: d, 1200: d}, slowed, {100: d}} {
			if got := r.going(k); !maps.Equal(got, want) {
				t.Errorf("step %d, paces %v: going %v, want %v", r.p.read.Clock().Steps(),
					paced[k], got, want)
			}
		}
	}
}

func TestPace_LetsEveryBlockGoWithoutSectors(t *testing.T) {
	r := sectorWorldOf(t, 0, 0, lod.Ladder{})
	r.spawn(100, 2000)
	r.run()
	want := map[float32]time.Duration{100: 10 * time.Millisecond, 2000: 10 * time.Millisecond}
	if got := r.going(2); !maps.Equal(got, want) {
		t.Errorf("going %v, want every entity every step", got)
	}
}

func TestPace_RefusesPacesThatCannotBe(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("the world took paces going six levels coarser than the finest seen")
		}
	}()
	sectorWorld(t, twoLevels).p.Pace(lod.Paces{1, 1, 1, 1, 1, 1, 1})
}

func (r *sectorRig) held(x float32) bool {
	found := false
	box := geom.NewAABB(geom.NewVec(x, 250), geom.NewVec(x+10, 260))
	r.p.read.Space().Query(box, aabbworld.AnyCapability, func(uid.UID64) { found = true })
	return found
}

func TestSectors_LeaveWhatSleepsOutOfTheSpaceAndBringItBack(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 2000)
	r.run()
	if !r.held(100) || r.held(2000) {
		t.Fatalf("the space holds the near entity %v and the sleeping one %v; want only the near",
			r.held(100), r.held(2000))
	}
	r.observe(2000)
	r.run()
	if !r.held(2000) {
		t.Error("the entity woken is not back in the space")
	}
}

// each pass reads the space first, so it answers from cells built before the crossing
func TestSectors_TheSpaceReadBeforeACrossingLetsGoOfThoseAsleepAndTakesThemBack(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 300)
	r.run()
	if !r.held(100) || !r.held(300) {
		t.Fatalf("the space holds %v and %v near the observers, want both", r.held(100),
			r.held(300))
	}
	r.observe(3500)
	r.run()
	if r.held(100) || r.held(300) {
		t.Errorf("the space holds %v and %v gone to sleep, want neither", r.held(100), r.held(300))
	}
	r.observe(100)
	r.run()
	if !r.held(100) || !r.held(300) {
		t.Errorf("the space holds %v and %v woken, want both", r.held(100), r.held(300))
	}
}

func TestSectors_AfterALoadLayTheSleepersAnew(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 3500)
	r.run()
	s := r.p.module.sectors
	// a load brings other entities than those the sectors knew
	for i := range s.sleep {
		s.sleep[i] = nil
	}
	s.restart()
	r.observe(3500)
	r.run()
	if near, _, _ := r.levels(); !same(near, 3500) {
		t.Errorf("L2 %v after the load, want the sleeper the observers came to", near)
	}
}

func TestSectors_AreSeenByEachObserverAtItsOwnLevelAndByAnyOtherCameraAsTicked(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.run()
	s, near, far := r.p.module.sectors, r.watch.cams[0], r.watch.cams[1]
	other := cameratest.NewFromSpace(4096, 512, 0)
	for i, want := range [][3]lod.Level{{2, 3, 2}, {2, 3, 2}, {6, 3, 3}, {6, 6, 6}} {
		got := [3]lod.Level{s.seenBy(near, i), s.seenBy(far, i), s.seenBy(other, i)}
		if got != want {
			t.Errorf("sector %d seen by the near, the far and another camera at %v, want %v",
				i, got, want)
		}
	}
}

// the keen entity and the level it keeps, false for none
func (r *sectorRig) keenOne() (uid.UID64, lod.Level, bool) {
	for r.keen.All(); r.keen.Next(); {
		cur := r.keen.Cursor()
		for j, id := range cur.IDs {
			return id, r.keenOf.Slice(cur)[j].Level, true
		}
	}
	return 0, lod.L6, false
}

// the level each of the rig's eight sectors is ticked at
func ticked(s *sectors) [8]lod.Level {
	var out [8]lod.Level
	copy(out[:], s.level)
	return out
}

// the level cam draws each of the rig's eight sectors at
func drawn(s *sectors, cam camera.Camera) [8]lod.Level {
	var out [8]lod.Level
	for i := range out {
		out[i] = s.seenBy(cam, i)
	}
	return out
}

// the entities by level, each by its x
func (r *sectorRig) byLevel() map[lod.Level][]float32 {
	out := map[lod.Level][]float32{}
	for k := range lod.Level(lod.Levels) {
		if xs := r.at(k); len(xs) > 0 {
			out[k] = xs
		}
	}
	return out
}

// the entities by how many levels coarser than the finest seen they are marked, each by its x
func (r *sectorRig) byFar() map[lod.Level][]float32 {
	out := map[lod.Level][]float32{}
	for k, q := range r.far {
		if xs := r.found(q); len(xs) > 0 {
			out[lod.Level(k)] = xs
		}
	}
	return out
}

func sameLevels(got, want map[lod.Level][]float32) bool {
	if len(got) != len(want) {
		return false
	}
	for k, xs := range want {
		if !same(got[k], xs...) {
			return false
		}
	}
	return true
}

// a camera from above looking from (100, 256) whose scale falls from near to fall past x 1000, as
// a perspective's falls with the distance
type sloped struct {
	camera.Camera
	near, fall float32
}

var (
	_ camera.Scaler  = (*sloped)(nil)
	_ camera.Focused = (*sloped)(nil)
)

func (s *sloped) ScaleAt(x, _, _ float32) float32 {
	if x < 1000 {
		return s.near
	}
	return s.fall
}

func (s *sloped) Focus() (x, y, z float32) { return 100, 256, 0 }

var fourLevels = lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 1, 0.5, 0.25, 0.1}}

func TestSectors_DrawNoTwoSectorsSideBySideMoreThanALevelApart(t *testing.T) {
	r := sectorWorld(t, fourLevels)
	cam := &sloped{Camera: cameratest.NewFromSpace(4096, 512, 0), near: 1, fall: 0.15}
	r.watch.cams = []camera.Camera{cam}
	r.run()
	s := r.p.module.sectors
	// the scale falls from L2's to L5's between the second sector and the third
	want := [8]lod.Level{2, 2, 3, 4, 5, 5, 5, 5}
	if got := drawn(s, cam); got != want {
		t.Errorf("drawn at %v, want %v", got, want)
	}
	if got := ticked(s); got != want {
		t.Errorf("ticked at %v, want %v", got, want)
	}
	reported := map[string]string{}
	r.p.read.Sectors().Report(func(label, value string) { reported[label] = value })
	if got := reported["Sectors lifted"]; got != "2" {
		t.Errorf("the panel says %q lifted, want the 2", got)
	}
	// under the last threshold the far sectors are nothing, and those lifted fall with them
	cam.fall = 0.05
	r.run()
	want = [8]lod.Level{2, 2, 6, 6, 6, 6, 6, 6}
	if got := drawn(s, cam); got != want {
		t.Errorf("past the last threshold drawn at %v, want %v", got, want)
	}
	if s.lifts != 0 {
		t.Errorf("%d lifted beside sectors at L6, want none", s.lifts)
	}
}

func TestSectors_GradeByRingsAcrossTheSeamsAndNeverPastL6(t *testing.T) {
	s := &sectors{grid: sector.NewGrid(7*64, 7*64, true, true, 64)}
	raw := make([]lod.Level, 49)
	for i := range raw {
		raw[i] = lod.L5
	}
	raw[0] = lod.L2
	graded := make([]lod.Level, 49)
	lifts, finest := s.grade(raw, graded)
	ring := func(k int) int { return min(k, 7-k) }
	for i, l := range graded {
		if want := lod.L2 + lod.Level(min(max(ring(i%7), ring(i/7)), 3)); l != want {
			t.Errorf("sector %d graded %v, want %v", i, l, want)
		}
	}
	if lifts != 8+16 || finest != lod.L2 {
		t.Errorf("%d lifted, the finest %v; want the 24 of the two rings round the corner, L2",
			lifts, finest)
	}
	if raw[1] != lod.L5 {
		t.Errorf("raw changed to %v by the grading, want it kept", raw[1])
	}
	open := &sectors{grid: sector.NewGrid(6*64, 64, false, false, 64)}
	raw = []lod.Level{2, 5, 5, 6, 5, 5}
	graded = make([]lod.Level, 6)
	open.grade(raw, graded)
	if want := []lod.Level{2, 3, 4, 6, 5, 5}; !slices.Equal(graded, want) {
		t.Errorf("graded %v, want %v: nothing lifted past L6", graded, want)
	}
	if _, finest = open.grade([]lod.Level{6, 5, 4, 6, 5, 5}, graded); finest != lod.L4 {
		t.Errorf("the finest of L6, L5 and L4 graded %v, want L4", finest)
	}
}

func TestSectors_ZoomedOutGoCoarserATenthPastAThresholdAndZoomedInFinerAtOnce(t *testing.T) {
	r := sectorWorld(t, lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 1, 0.5}})
	cam := r.watch.cams[0]
	r.watch.cams = r.watch.cams[:1]
	r.spawn(100, 2000, 3500)
	s := r.p.module.sectors
	for _, c := range []struct {
		zoom float32
		want lod.Level
	}{
		{1, lod.L2},
		{0.95, lod.L2},
		{0.85, lod.L3},
		{0.95, lod.L3},
		{1, lod.L2},
	} {
		cam.ZoomIn(c.zoom/cam.Zoom(), 0, 0)
		r.run()
		for i, l := range s.level {
			if l != c.want {
				t.Errorf("zoomed to %v, sector %d at %v, want %v", c.zoom, i, l, c.want)
			}
		}
		if got := r.at(c.want); !same(got, 100, 2000, 3500) {
			t.Errorf("zoomed to %v, %v at %v, want every entity", c.zoom, got, c.want)
		}
	}
}

func TestSectors_GoPastTheSightHalfASectorLateAndComeBackAtOnce(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100)
	// the far observer sees 1200; the first sector ends at 512
	for _, c := range []struct {
		x    float32
		want lod.Level
	}{
		{1700, lod.L3},
		{1800, lod.L3},
		{2000, lod.L6},
		{1800, lod.L6},
		{1700, lod.L3},
	} {
		r.observe(c.x)
		r.run()
		if got := r.at(c.want); !same(got, 100) {
			t.Errorf("observed from %v, %v at %v, want the one at 100", c.x, got, c.want)
		}
	}
}

func TestSectors_AKeenEntityKeepsItsSectorAndThoseRoundItAcrossTheSeamWhileItIsKeen(t *testing.T) {
	for name, c := range map[string]struct {
		gone   func(r *sectorRig, cb *goke.CmdBuf, id uid.UID64)
		asleep []float32
	}{
		"despawned": {
			gone: func(r *sectorRig, cb *goke.CmdBuf, id uid.UID64) {
				r.p.module.despawn(cb, id)
			},
			asleep: []float32{3900},
		},
		"keen no more": {
			gone: func(r *sectorRig, cb *goke.CmdBuf, id uid.UID64) {
				cb.RemoveCompOne(id, r.keenID)
			},
			asleep: []float32{100, 3900},
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := sectorWorldOf(t, aabbworld.Torus, 2, twoLevels)
			r.observe(2000)
			r.spawn(700, 3900)
			r.p.write.Spawn(r.keens.Entry(keenThing{x: 100}))
			r.run()
			r.run()
			s := r.p.module.sectors
			// the last sector across the seam at the keen one's L4, the second at the far
			// observer's L3
			if got, want := ticked(s), [8]lod.Level{4, 3, 2, 2, 2, 2, 3, 4}; got != want {
				t.Errorf("ticked at %v with the keen one, want %v", got, want)
			}
			want := map[lod.Level][]float32{lod.L3: {700}, lod.L4: {100, 3900}}
			if got := r.byLevel(); !sameLevels(got, want) {
				t.Errorf("entities %v with the keen one, want %v", got, want)
			}
			id, _, _ := r.keenOne()
			run(r.ecs, r.p.module, func(cb *goke.CmdBuf) { c.gone(r, cb, id) })
			r.run()
			if got, want := ticked(s), [8]lod.Level{6, 3, 2, 2, 2, 2, 3, 6}; got != want {
				t.Errorf("ticked at %v, want %v", got, want)
			}
			if got := r.at(lod.L6); !same(got, c.asleep...) {
				t.Errorf("%v asleep, want %v", got, c.asleep)
			}
		})
	}
}

func TestSectors_AKeenEntityWakesTheSleepersRoundItBeforeItComes(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(3500)
	r.run()
	if r.held(3500) {
		t.Fatal("the space holds the one at 3500 before anything keen comes, want it asleep")
	}
	r.p.write.Spawn(r.keens.Entry(keenThing{x: 3000}))
	r.run()
	r.run()
	if !r.held(3500) || !same(r.at(lod.L4), 3000, 3500) {
		t.Errorf("the one beside the keen one held %v, at L4 %v; want it woken at L4",
			r.held(3500), r.at(lod.L4))
	}
}

func TestSectors_SwitchedOffTickEverySectorAtL2AndStillGradeTheObservers(t *testing.T) {
	r := sectorWorld(t, fourLevels)
	cam := &sloped{Camera: cameratest.NewFromSpace(4096, 512, 0), near: 1, fall: 0.15}
	r.watch.cams = []camera.Camera{cam}
	r.p.write.SetBanded(false)
	r.spawn(100, 2000)
	r.p.write.Spawn(r.keens.Entry(keenThing{x: 3500}))
	r.run()
	r.run()
	s := r.p.module.sectors
	if got := ticked(s); got != ([8]lod.Level{2, 2, 2, 2, 2, 2, 2, 2}) {
		t.Errorf("ticked at %v, want every sector at L2", got)
	}
	if got := r.at(lod.L2); !same(got, 100, 2000, 3500) {
		t.Errorf("%v at L2, want every entity", got)
	}
	if got, want := drawn(s, cam), [8]lod.Level{2, 2, 3, 4, 5, 5, 5, 5}; got != want ||
		s.lifts != 2 {
		t.Errorf("drawn at %v, %d lifted; want %v, 2", got, s.lifts, want)
	}
}

func TestSectors_KeepTheMarksAndKeenThroughASaveAndGoOnFromThem(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 1200, 3900)
	r.p.write.Spawn(r.keens.Entry(keenThing{x: 2000}))
	r.run()
	r.run()
	want := map[lod.Level][]float32{lod.L2: {100}, lod.L3: {1200}, lod.L4: {2000}, lod.L6: {3900}}
	if got := r.byLevel(); !sameLevels(got, want) {
		t.Fatalf("entities %v before the save, want %v", got, want)
	}
	far := map[lod.Level][]float32{0: {100, 3900}, 1: {1200}, 2: {2000}}
	if got := r.byFar(); !sameLevels(got, far) {
		t.Fatalf("entities %v by their pace before the save, want %v", got, far)
	}
	path := t.TempDir() + "/save.bin"
	r.ecs.Pause()
	if err := r.ecs.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded := newSectorRig(0, 2, twoLevels)
	loaded.setup(t, path)
	if got := loaded.byLevel(); !sameLevels(got, want) {
		t.Errorf("entities %v loaded, want %v", got, want)
	}
	if got := loaded.byFar(); !sameLevels(got, far) {
		t.Errorf("entities %v by their pace loaded, want %v", got, far)
	}
	if _, level, ok := loaded.keenOne(); !ok || level != lod.L4 {
		t.Errorf("the keen one loaded %v keeping %v, want it keeping L4", ok, level)
	}
	loaded.run()
	if got := loaded.byLevel(); !sameLevels(got, want) {
		t.Errorf("entities %v a tick after the load, want %v", got, want)
	}
	if got := loaded.byFar(); !sameLevels(got, far) {
		t.Errorf("entities %v by their pace a tick after the load, want %v", got, far)
	}
	if !loaded.held(2000) || loaded.held(3900) {
		t.Errorf("the space holds the keen one %v and the sleeper %v, want only the keen one",
			loaded.held(2000), loaded.held(3900))
	}
}

func TestSectors_KeepAnAbstractTravellerThroughASave(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100, 3900)
	far := r.idAt(3900)
	run(r.ecs, r.p.module, func(cb *goke.CmdBuf) {
		cb.AddOne(far, r.abstractID, sector.Abstract{})
	})
	if got := r.travelling(); !same(got, 3900) {
		t.Fatalf("%v travelling before the save, want 3900", got)
	}
	path := t.TempDir() + "/save.bin"
	r.ecs.Pause()
	if err := r.ecs.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded := newSectorRig(0, 2, twoLevels)
	loaded.setup(t, path)
	if got := loaded.travelling(); !same(got, 3900) {
		t.Errorf("%v travelling loaded, want 3900", got)
	}
	loaded.run()
	if got := loaded.travelling(); !same(got, 3900) {
		t.Errorf("%v travelling a tick after the load, want 3900", got)
	}
}

// the entity whose box sets off at x
func (r *sectorRig) idAt(x float32) uid.UID64 {
	for r.all.All(); r.all.Next(); {
		cur := r.all.Cursor()
		for j, id := range cur.IDs {
			if r.base.Slice(cur)[j].Pos.TopLeft.X == x {
				return id
			}
		}
	}
	panic("no entity there")
}

// the entities travelling the abstract graph, each by its x
func (r *sectorRig) travelling() []float32 {
	var found []float32
	for id := range living(r.abstract) {
		for r.all.All(); r.all.Next(); {
			cur := r.all.Cursor()
			for j, of := range cur.IDs {
				if of == id {
					found = append(found, r.base.Slice(cur)[j].Pos.TopLeft.X)
				}
			}
		}
	}
	return found
}

func TestRead_LevelAtIsTheLevelTicked(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.p.write.Spawn(r.keens.Entry(keenThing{x: 3500}))
	r.run()
	r.run()
	read := &r.p.read
	for _, c := range []struct {
		x    float32
		want lod.Level
	}{
		// the near observer's L2 over the far one's L3
		{100, lod.L2},
		{1200, lod.L3},
		{2000, lod.L6},
		// the keen one's sector and the one beside it
		{3500, lod.L4},
		{2600, lod.L4},
	} {
		if got := read.LevelAt(c.x, 256); got != c.want {
			t.Errorf("at x %v: %v, want %v", c.x, got, c.want)
		}
	}
	r.watch.cams = r.watch.cams[1:]
	r.run()
	if got := read.LevelAt(100, 256); got != lod.L3 {
		t.Errorf("at x 100 the far observer alone: %v, want L3", got)
	}
	r.p.write.SetBanded(false)
	for _, x := range []float32{100, 2000, 3500} {
		if got := read.LevelAt(x, 256); got != lod.L2 {
			t.Errorf("at x %v the levels switched off: %v, want L2", x, got)
		}
	}
	flat := NewPlugin(Config{
		Space:    SpaceCfg{Width: 4096, Height: 512},
		Entities: EntitiesCfg{MaxCount: 8, MinSize: 5, MaxSize: 20},
	})
	if got := flat.read.LevelAt(100, 256); got != lod.L2 {
		t.Errorf("in a world of no sectors: %v, want L2", got)
	}
}

func TestSectors_ObserveAndKeepAllocateNothingOnceWarm(t *testing.T) {
	r := sectorWorldOf(t, aabbworld.Torus, 2, twoLevels)
	r.spawn(100, 2000)
	r.p.write.Spawn(r.keens.Entry(keenThing{x: 3000}))
	r.run()
	r.run()
	s := r.p.module.sectors
	if allocs := testing.AllocsPerRun(100, func() {
		s.observe()
		s.keep()
	}); allocs != 0 {
		t.Errorf("%v allocations a pass, want none", allocs)
	}
}

func TestSectors_HoldEachObserversLevelsThoughTheObserversComeInAnotherOrder(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	r.spawn(100)
	r.observe(1700)
	r.run()
	cams := r.watch.cams
	r.watch.cams = []camera.Camera{cams[1], cams[0]}
	// 1288 off the first sector: the far observer holds it at L3, under half a sector past
	r.observe(1800)
	r.run()
	if got := r.at(lod.L3); !same(got, 100) {
		t.Errorf("%v at L3, want the one the far observer holds", got)
	}
}

func TestSectors_MarkHowManyLevelsCoarserThanTheFinestSeenEachEntityIs(t *testing.T) {
	for name, c := range map[string]struct {
		near, fall float32
		want       map[lod.Level][]float32
	}{
		"every sector at L4": {0.3, 0.3, map[lod.Level][]float32{0: {100, 1200, 2000}}},
		"L4, L5 past it":     {0.3, 0.15, map[lod.Level][]float32{0: {100}, 1: {1200, 2000}}},
		// L3 graded between
		"L2, L4 past it": {1, 0.3, map[lod.Level][]float32{0: {100}, 1: {1200}, 2: {2000}}},
	} {
		t.Run(name, func(t *testing.T) {
			r := sectorWorld(t, fourLevels)
			cam := &sloped{Camera: cameratest.NewFromSpace(4096, 512, 0), near: c.near,
				fall: c.fall}
			r.watch.cams = []camera.Camera{cam}
			r.spawn(100, 1200, 2000)
			r.run()
			if got := r.byFar(); !sameLevels(got, c.want) {
				t.Errorf("by their pace %v (by level %v), want %v", got, r.byLevel(), c.want)
			}
		})
	}
}

func TestSectors_PaceByTheFinestLevelAnyObserverSees(t *testing.T) {
	r := sectorWorld(t, twoLevels)
	near, far := r.watch.cams[0], r.watch.cams[1]
	r.spawn(100, 1200)
	for _, c := range []struct {
		cams []camera.Camera
		want map[lod.Level][]float32
	}{
		{[]camera.Camera{near, far}, map[lod.Level][]float32{0: {100}, 1: {1200}}},
		{[]camera.Camera{far, near}, map[lod.Level][]float32{0: {100}, 1: {1200}}},
		// the far observer alone sees L3 at the finest
		{[]camera.Camera{far}, map[lod.Level][]float32{0: {100, 1200}}},
	} {
		r.watch.cams = c.cams
		r.run()
		if got := r.byFar(); !sameLevels(got, c.want) {
			t.Errorf("seen by %d observers: by their pace %v, want %v", len(c.cams), got, c.want)
		}
	}
}

func TestSectors_PaceAWorldWithoutObserversInFullEverywhere(t *testing.T) {
	for _, unobserved := range []lod.Fallback{lod.UnobservedL4, lod.UnobservedL5} {
		ladder := twoLevels
		ladder.Unobserved = unobserved
		r := sectorWorld(t, ladder)
		r.watch.cams = nil
		r.spawn(100, 1200, 2300, 3500)
		r.p.write.Spawn(r.keens.Entry(keenThing{x: 2000}))
		r.run()
		r.run()
		want := map[lod.Level][]float32{0: {100, 1200, 2000, 2300, 3500}}
		if got := r.byFar(); !sameLevels(got, want) {
			t.Errorf("unobserved at %v: by their pace %v (by level %v), want %v",
				unobserved.Level(), got, r.byLevel(), want)
		}
	}
}

// a shot keeping its sector at L4 does not hasten the world the observers see at L5
func TestSectors_AKeenEntityLeavesThePaceAsTheObserversSeeIt(t *testing.T) {
	r := sectorWorld(t, fourLevels)
	cam := &sloped{Camera: cameratest.NewFromSpace(4096, 512, 0), near: 0.15, fall: 0.15}
	r.watch.cams = []camera.Camera{cam}
	r.spawn(100, 3500)
	r.p.write.Spawn(r.keens.Entry(keenThing{x: 2000}))
	r.run()
	r.run()
	levels := map[lod.Level][]float32{lod.L4: {2000}, lod.L5: {100, 3500}}
	if got := r.byLevel(); !sameLevels(got, levels) {
		t.Fatalf("by level %v, want %v", got, levels)
	}
	want := map[lod.Level][]float32{0: {100, 2000, 3500}}
	if got := r.byFar(); !sameLevels(got, want) {
		t.Errorf("by their pace %v, want %v", got, want)
	}
}

func TestPace_GoesEveryStepOnTheFinestSectorsSeenAndSlowerOnTheCoarser(t *testing.T) {
	r := sectorWorld(t, fourLevels)
	cam := &sloped{Camera: cameratest.NewFromSpace(4096, 512, 0), near: 0.3, fall: 0.15}
	r.watch.cams = []camera.Camera{cam}
	r.spawn(100, 2000)
	r.run()
	const d = 10 * time.Millisecond
	for range 4 {
		r.run()
		// L4 the finest seen, L5 a level coarser
		want := map[float32]time.Duration{100: d}
		if r.p.read.Clock().Steps()%2 == 1 {
			want[2000] = 2 * d
		}
		if got := r.going(3); !maps.Equal(got, want) {
			t.Errorf("step %d, paces %v: going %v, want %v", r.p.read.Clock().Steps(), paced[3],
				got, want)
		}
	}
}

// a ladder of every level, L0 from 8 shown pixels a world unit
var sixLevels = lod.Ladder{Scales: [lod.Levels - 1]float32{8, 4, 2, 1, 0.5, 0.25}}

// the rig's world of sixLevels seen by one camera from (100, 256) at 8 pixels a world unit short
// of x 1000 and 0.3 past it, its sectors graded L0, L0, L1, L2, L3, L4, L5, L5, with an entity
// in a sector at L0, L1, L2 and L5
func closeWorld(t *testing.T) *sectorRig {
	t.Helper()
	r := sectorWorld(t, sixLevels)
	cam := &sloped{Camera: cameratest.NewFromSpace(4096, 512, 0), near: 8, fall: 0.3}
	r.watch.cams = []camera.Camera{cam}
	r.spawn(100, 1100, 1600, 3100)
	r.run()
	return r
}

func TestSectors_PaceFromTheCloseLevelAnObserverSees(t *testing.T) {
	r := closeWorld(t)
	levels := map[lod.Level][]float32{lod.L0: {100}, lod.L1: {1100}, lod.L2: {1600},
		lod.L5: {3100}}
	if got := r.byLevel(); !sameLevels(got, levels) {
		t.Fatalf("by level %v, want %v", got, levels)
	}
	far := map[lod.Level][]float32{0: {100}, 1: {1100}, 2: {1600}, 5: {3100}}
	if got := r.byFar(); !sameLevels(got, far) {
		t.Errorf("by their pace %v, want %v", got, far)
	}
	const d = 10 * time.Millisecond
	for range 8 {
		r.run()
		steps := r.p.read.Clock().Steps()
		want := map[float32]time.Duration{100: d}
		if steps%2 == 1 {
			want[1100] = 2 * d
		}
		if steps%4 == 3 {
			want[1600] = 4 * d
		}
		if steps%8 == 7 {
			want[3100] = 8 * d
		}
		if got := r.going(3); !maps.Equal(got, want) {
			t.Errorf("step %d, paces %v: going %v, want %v", steps, paced[3], got, want)
		}
	}
}

func TestWithin_KeepsAQueryToTheSectorsOfOneScale(t *testing.T) {
	r := closeWorld(t)
	for k, want := range [][]float32{{100, 1100}, {1600}, {3100}} {
		if got := r.found(r.within[k]); !same(got, want...) {
			t.Errorf("within the scale %d: %v, want %v", k, got, want)
		}
	}
	got := map[float32]bool{}
	for r.detail.q.Paced(10 * time.Millisecond); r.detail.q.Next(); {
		for _, b := range r.detail.base.Slice(r.detail.q.Cursor()) {
			got[b.Pos.TopLeft.X] = true
		}
	}
	if !maps.Equal(got, map[float32]bool{100: true}) {
		t.Errorf("the close scale's detail at Exact went over %v, want the one at L0 alone", got)
	}
	plain := sectorWorldOf(t, 0, 0, lod.Ladder{})
	plain.spawn(100, 2000)
	for k, want := range [][]float32{nil, {100, 2000}, nil} {
		if got := plain.found(plain.within[k]); !same(got, want...) {
			t.Errorf("without sectors, within the scale %d: %v, want %v", k, got, want)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("within the scale of L1 passed, want only a scale's first level")
		}
	}()
	r.p.Within(lod.L1)
}

func TestSectors_KeepEveryLevelsMarkThroughASave(t *testing.T) {
	r := closeWorld(t)
	levels, far := r.byLevel(), r.byFar()
	if len(levels[lod.L0]) == 0 || len(levels[lod.L1]) == 0 || len(levels[lod.L5]) == 0 {
		t.Fatalf("by level %v before the save, want some at L0, L1 and L5", levels)
	}
	path := t.TempDir() + "/save.bin"
	r.ecs.Pause()
	if err := r.ecs.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded := newSectorRig(0, 2, sixLevels)
	loaded.setup(t, path)
	if got := loaded.byLevel(); !sameLevels(got, levels) {
		t.Errorf("entities %v loaded, want %v", got, levels)
	}
	if got := loaded.byFar(); !sameLevels(got, far) {
		t.Errorf("entities %v by their pace loaded, want %v", got, far)
	}
}
