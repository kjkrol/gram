package world

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
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
}

// a world 4096x512 cut into sectors 512 a side: the first band 600 round the anchor, the second
// 1200
func sectorWorld(t *testing.T, bands ...sector.Band) *sectorRig {
	t.Helper()
	r := &sectorRig{anchor: &points{at: []geom.Vec{geom.NewVec(100, 256)}}}
	r.p = NewPlugin(Config{
		Space:    SpaceCfg{Width: 4096, Height: 512},
		Entities: EntitiesCfg{MaxCount: 8, MinSize: 5, MaxSize: 20},
		Sectors:  SectorsCfg{Buckets: 2, Bands: bands},
	})
	r.p.Anchor(r.anchor)
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
	}})...)
	r.p.module.RegSystems(r.ecs)
	return r
}

var twoBands = []sector.Band{
	{Reach: 600},
	{Reach: 1200, Pace: 4},
}

func (r *sectorRig) spawn(xs ...float64) {
	for _, x := range xs {
		r.p.Spawn(r.things.Entry(thing{x: x, y: 250, size: 10}))
	}
	r.run()
}

func (r *sectorRig) run() { run(r.ecs, r.p.module, func(*goke.CmdBuf) {}) }

// the entities by the band they carry, each by its x
func (r *sectorRig) bands() (near, second, off []float64) {
	xs := map[uid.UID64]float64{}
	for r.all.All(); r.all.Next(); {
		cur := r.all.Cursor()
		for j, id := range cur.IDs {
			xs[id] = r.base.Slice(cur)[j].Pos.TopLeft.X
		}
	}
	of := func(q *goke.Query) []float64 {
		var found []float64
		for id := range living(q) {
			found = append(found, xs[id])
		}
		return found
	}
	return of(r.near), of(r.second), of(r.off)
}

func (r *sectorRig) moveTo(x float64, to float64) {
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

func same(got []float64, want ...float64) bool {
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
	r.p.Sectors().Report(func(label, value string) { got[label] = value })
	if got["Sectors"] != "2 · 1, beyond 5" {
		t.Errorf("sectors %q, want 2 · 1, beyond 5", got["Sectors"])
	}
	if got["Entities by band"] != "1 · 1, beyond 2" {
		t.Errorf("entities %q, want 1 · 1, beyond 2", got["Entities by band"])
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
