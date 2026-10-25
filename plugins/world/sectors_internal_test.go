package world

import (
	"maps"
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

type points struct{ at []geom.Vec }

var _ sector.Anchors = (*points)(nil)

func (p *points) Anchors(at func(x, y float32)) {
	for _, v := range p.at {
		at(float32(v.X), float32(v.Y))
	}
}

type sectorRig struct {
	p      *Plugin
	things kind.Of[thing]
	ecs    *goke.ECS
	anchor *points
	all    *goke.Query
	near   *goke.Query
	second *goke.Query
	off    *goke.Query
	base   *goke.Comp[Base]
	// by policy
	paced [3]pacedQuery
}

type pacedQuery struct {
	q    *goke.Query
	base goke.Comp[Base]
}

// a world 4096x512 cut into sectors 512 a side: the first band 600 round the anchor, the second
// 1200
func sectorWorld(t *testing.T, bands ...sector.Band) *sectorRig {
	t.Helper()
	return sectorWorldOf(t, 0, bands...)
}

// sectorWorld with its edges
func sectorWorldOf(t *testing.T, edges aabbworld.Edges, bands ...sector.Band) *sectorRig {
	t.Helper()
	r := &sectorRig{anchor: &points{at: []geom.Vec{geom.NewVec(100, 256)}}}
	r.p = NewPlugin(Config{
		Space:    SpaceCfg{Width: 4096, Height: 512, Edges: edges},
		Entities: EntitiesCfg{MaxCount: 8, MinSize: 5, MaxSize: 20},
		Sectors:  SectorsCfg{Buckets: 2, Bands: bands},
	})
	r.p.write.Anchor(r.anchor)
	kind.Define[thing](r.p.Kinds(), "thing", kind.Spec{
		comp.Load(func(t thing) Position {
			return Position{AABB: plane.NewAABB(geom.NewVec(t.x, t.y), t.size, t.size)}
		}),
		comp.Const(Velocity{}),
	})
	r.things = kind.Named[thing](r.p.Kinds(), "thing")
	r.base = new(goke.Comp[Base])
	var b goke.Comp[Base]
	r.ecs = goke.New()
	r.ecs.Setup(append(r.p.module.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.all = si.NewQueryBuilder(r.base).Build()
		r.near = si.NewQueryBuilder(&b).Exclude(goke.Exclude[sector.Band2](),
			goke.Exclude[sector.Band3](), goke.Exclude[sector.Off]()).Build()
		r.second = si.NewQueryBuilder(&b).Include(goke.Include[sector.Band2]()).Build()
		r.off = si.NewQueryBuilder(&b).Include(goke.Include[sector.Off]()).Build()
		for k := range r.paced {
			pq := &r.paced[k]
			pq.q = si.NewQueryBuilder(&pq.base).With(r.p.Pace(sector.Policy(k))).Build()
		}
	}})...)
	r.p.module.RegSystems(r.ecs)
	return r
}

var twoBands = []sector.Band{
	{Reach: 600},
	{Reach: 1200, Pace: 4},
}

func (r *sectorRig) spawn(xs ...float32) {
	for _, x := range xs {
		r.p.write.Spawn(r.things.Entry(thing{x: x, y: 250, size: 10}))
	}
	r.run()
}

func (r *sectorRig) run() { run(r.ecs, r.p.module, func(*goke.CmdBuf) {}) }

// the entities by the band they carry, each by its x
func (r *sectorRig) bands() (near, second, off []float32) {
	xs := map[uid.UID64]float32{}
	for r.all.All(); r.all.Next(); {
		cur := r.all.Cursor()
		for j, id := range cur.IDs {
			xs[id] = r.base.Slice(cur)[j].Pos.TopLeft.X
		}
	}
	of := func(q *goke.Query) []float32 {
		var found []float32
		for id := range living(q) {
			found = append(found, xs[id])
		}
		return found
	}
	return of(r.near), of(r.second), of(r.off)
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

func TestSectors_MarkEachEntityByItsSectorsBand(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100, 900, 1200, 2000, 3500)
	r.run()
	near, second, off := r.bands()
	if !same(near, 100, 900) || !same(second, 1200) || !same(off, 2000, 3500) {
		t.Errorf("near %v, second %v, beyond %v; want 100 and 900, 1200, 2000 and 3500",
			near, second, off)
	}
}

func TestSectors_FollowTheAnchor(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100, 1200, 2000, 3500)
	r.run()
	r.anchor.at[0] = geom.NewVec(3500, 256)
	r.run()
	near, second, off := r.bands()
	if !same(near, 3500) || len(second) != 0 || !same(off, 100, 1200, 2000) {
		t.Errorf("near %v, second %v, beyond %v; want 3500, none, the rest", near, second, off)
	}
}

func TestSectors_MoveTheMarkOfAnEntityCrossingIntoAnotherBand(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100)
	r.run()
	r.moveTo(100, 1200)
	r.run()
	near, second, _ := r.bands()
	if len(near) != 0 || !same(second, 1200) {
		t.Errorf("near %v, second %v; want the one walked off into the second band", near, second)
	}
	r.moveTo(1200, 2000)
	r.run()
	if _, _, off := r.bands(); !same(off, 2000) {
		t.Errorf("beyond %v, want the one walked past the last band", off)
	}
}

func TestSectors_LeaveAWorldWithoutBandsUnmarked(t *testing.T) {
	r := sectorWorld(t)
	r.spawn(100, 2000, 3500)
	r.run()
	near, second, off := r.bands()
	if len(near) != 3 || len(second) != 0 || len(off) != 0 {
		t.Errorf("near %v, second %v, beyond %v; want every entity unmarked", near, second, off)
	}
}

func TestSectors_ReportTheSectorsAndEntitiesByBand(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100, 1200, 2000, 3500)
	r.run()
	r.run()
	got := map[string]string{}
	r.p.read.Sectors().Report(func(label, value string) { got[label] = value })
	if want := "first 2\nsecond 1\nbeyond 5"; got["Sectors"] != want {
		t.Errorf("sectors %q, want %q", got["Sectors"], want)
	}
	if want := "first 1\nsecond 1\nbeyond 2"; got["Entities by band"] != want {
		t.Errorf("entities %q, want %q", got["Entities by band"], want)
	}
}

func TestSectors_SwitchedOffLiveEverywhereAndOnAgainGoBackPastTheMargin(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100, 1200, 2000, 3500)
	r.run()
	r.p.module.commands.Put(command.Nobody, SwitchBands{On: false})
	r.run()
	r.run()
	near, second, off := r.bands()
	if r.p.read.Banded() || len(near) != 4 || len(second) != 0 || len(off) != 0 {
		t.Errorf("bands off (banded %v): near %v, second %v, beyond %v; want every entity near",
			r.p.read.Banded(), near, second, off)
	}
	got := map[string]string{}
	r.p.read.Sectors().Report(func(label, value string) { got[label] = value })
	if want := "first 8\nsecond 0\nbeyond 0\nbands off"; got["Sectors"] != want {
		t.Errorf("sectors %q, want %q", got["Sectors"], want)
	}
	r.p.module.commands.Put(command.Nobody, SwitchBands{On: true})
	r.run()
	r.run()
	// the sector of 2000 lies 1436 off, short of the second band's 1200 and its margin of 256
	near, second, off = r.bands()
	if !r.p.read.Banded() || !same(near, 100) || !same(second, 1200, 2000) || !same(off, 3500) {
		t.Errorf("bands on (banded %v): near %v, second %v, beyond %v; want 100, 1200 and 2000, "+
			"3500", r.p.read.Banded(), near, second, off)
	}
}

func TestSectors_SettingsOnlyInAWorldCutIntoSectorsAndNoKeys(t *testing.T) {
	bands := func(p *Plugin) (control.Switch, bool) {
		for _, s := range p.Settings() {
			if sw, ok := s.(control.Switch); ok && sw.Label == "Sectors' bands" {
				return sw, true
			}
		}
		return control.Switch{}, false
	}
	p := sectorWorld(t, twoBands...).p
	if sw, ok := bands(p); !ok || !sw.Get() || sw.Set(false) != (SwitchBands{On: false}) {
		t.Errorf("the bands' setting found %v, want it on and setting SwitchBands off", ok)
	}
	for _, k := range p.DefaultBindings() {
		if k.Type() == reflect.TypeFor[SwitchBands]() ||
			k.Type() == reflect.TypeFor[SwitchSectors]() {
			t.Errorf("%v is bound to %v, want the settings screen alone", k.Type(), k.Trigger)
		}
	}
	if _, ok := bands(sectorWorld(t).p); ok {
		t.Error("a world of no sectors has the bands' setting")
	}
}

func TestSectors_RefuseBandsOutOfOrder(t *testing.T) {
	for name, bands := range map[string][]sector.Band{
		"too many":      {{Reach: 1}, {Reach: 2}, {Reach: 3}, {Reach: 4}, {Reach: 5}, {Reach: 6}},
		"nearer":        {{Reach: 600}, {Reach: 500, Pace: 2}},
		"a paced first": {{Reach: 600, Pace: 2}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("the world took the bands")
				}
			}()
			sectorWorld(t, bands...)
		})
	}
}

// the entities going this step by policy, each by its x with the step it is given
func (r *sectorRig) going(policy sector.Policy) map[float32]time.Duration {
	pq := &r.paced[policy]
	got := map[float32]time.Duration{}
	for pq.q.Paced(10 * time.Millisecond); pq.q.Next(); {
		for _, b := range pq.base.Slice(pq.q.Cursor()) {
			got[b.Pos.TopLeft.X] = pq.q.Dt()
		}
	}
	return got
}

func TestPace_GoesInTheBandsAsItsPolicySays(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100, 1200, 2000)
	r.run()
	const d = 10 * time.Millisecond
	for range 4 {
		r.run()
		paced := map[float32]time.Duration{100: d}
		if r.p.read.Clock().Steps()%4 == 3 {
			paced[1200] = 4 * d
		}
		for policy, want := range map[sector.Policy]map[float32]time.Duration{
			sector.Everywhere: {100: d, 1200: d},
			sector.Paced:      paced,
			sector.Near:       {100: d},
		} {
			if got := r.going(policy); !maps.Equal(got, want) {
				t.Errorf("step %d, policy %d: going %v, want %v", r.p.read.Clock().Steps(), policy,
					got, want)
			}
		}
	}
}

func TestPace_LetsEveryBlockGoWithoutBands(t *testing.T) {
	r := sectorWorld(t)
	r.spawn(100, 2000)
	r.run()
	want := map[float32]time.Duration{100: 10 * time.Millisecond, 2000: 10 * time.Millisecond}
	if got := r.going(sector.Near); !maps.Equal(got, want) {
		t.Errorf("going %v, want every entity every step", got)
	}
}

func (r *sectorRig) held(x float32) bool {
	found := false
	box := geom.NewAABB(geom.NewVec(x, 250), geom.NewVec(x+10, 260))
	r.p.read.Space().Query(box, aabbworld.AnyCapability, func(uid.UID64) { found = true })
	return found
}

func TestSectors_LeaveWhatLiesBeyondOutOfTheSpaceAndBringItBack(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100, 2000)
	r.run()
	if !r.held(100) || r.held(2000) {
		t.Fatalf("the space holds the near entity %v and the one beyond %v; want only the near",
			r.held(100), r.held(2000))
	}
	r.anchor.at[0] = geom.NewVec(2000, 256)
	r.run()
	if !r.held(2000) {
		t.Error("the entity woken is not back in the space")
	}
}

// each pass reads the space first, so it answers from cells built before the crossing
func TestSectors_TheSpaceReadBeforeACrossingLetsGoOfThoseGoneBeyondAndTakesThemBack(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100, 300)
	r.run()
	if !r.held(100) || !r.held(300) {
		t.Fatalf("the space holds %v and %v near the anchor, want both", r.held(100), r.held(300))
	}
	r.anchor.at[0] = geom.NewVec(3500, 256)
	r.run()
	if r.held(100) || r.held(300) {
		t.Errorf("the space holds %v and %v gone beyond, want neither", r.held(100), r.held(300))
	}
	r.anchor.at[0] = geom.NewVec(100, 256)
	r.run()
	if !r.held(100) || !r.held(300) {
		t.Errorf("the space holds %v and %v woken, want both", r.held(100), r.held(300))
	}
}

func TestSectors_AfterALoadLayTheSleepersAnew(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100, 3500)
	r.run()
	s := r.p.module.sectors
	// a load brings other entities than those the sectors knew
	for i := range s.sleep {
		s.sleep[i] = nil
	}
	s.restart()
	r.anchor.at[0] = geom.NewVec(3500, 256)
	r.run()
	if near, _, _ := r.bands(); !same(near, 3500) {
		t.Errorf("near %v after the load, want the sleeper the anchor came to", near)
	}
}
