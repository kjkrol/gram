package world

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// the sectors of the stripes' second row the things across three stand over: A, B and C
const (
	sectorA = 4
	sectorB = 5
	sectorC = 6
)

// spanning is what stands over A, B and C, each of a look: a Whole's handle over a piece in each
// where whole, else one thing as long as the handle; the things that are drawn, by their order
type spanning struct {
	name   string
	things func(k standingKinds) []stood
	drawn  []int
}

var acrossThree = []spanning{
	{"a whole", func(k standingKinds) []stood {
		at := Z{Height: 10}
		return []stood{
			{box: plane.NewAABB(geom.NewVec(108, 364), 552, 40), z: at, sprite: k.dome,
				handle: true},
			{box: box(108, 364, 40), z: at, sprite: k.dome, of: 1},
			{box: box(364, 364, 40), z: at, sprite: k.dome, of: 1},
			{box: box(620, 364, 40), z: at, sprite: k.dome, of: 1},
		}
	}, []int{1, 2, 3}},
	{"a big thing", func(k standingKinds) []stood {
		return []stood{{box: plane.NewAABB(geom.NewVec(108, 364), 552, 40), z: Z{Height: 10},
			sprite: k.dome, fixed: true}}
	}, []int{0}},
}

func TestSheets_DrawWhatLivesInACellAliveAtItsLevelPhotographingNone(t *testing.T) {
	for _, c := range acrossThree {
		t.Run(c.name, func(t *testing.T) {
			k := newStandingKinds()
			s := newSeamRig(t, k, newStripes(lod.L3, lod.L4, lod.L4, lod.L4), c.things(k)...)
			f := s.publish()
			if got := s.recorded(f); !slices.Equal(got, c.drawn) {
				t.Fatalf("records of %v, want of %v alive", got, c.drawn)
			}
			for _, e := range f.all {
				if !e.ringed || e.tier == 0 || e.sheet != 0 {
					t.Errorf("the record of %d ringed %v in the tier slot %d, by sheet %d; want "+
						"ringed in a cell, by none", slices.Index(s.ids, e.id), e.ringed,
						e.tier-1, e.sheet)
				}
			}
			for _, i := range []int{sectorA, sectorB, sectorC} {
				if n := instancesOf(f, i); n > 0 {
					t.Errorf("the sector %d photographed of %d things, want of none", i, n)
				}
			}
			// at L4 a figure is a point: those of B and C drawn through the Look are at L3
			want := make([]render.SpriteID, len(c.drawn))
			for i := range want {
				want[i] = k.dome
			}
			if got := s.composedSprites(); !slices.Equal(got, want) {
				t.Errorf("drew %v through the Look, want %v, each at its cell's L3", got, want)
			}
		})
	}
}

func TestSheets_PhotographWhatLivesInACellWholeFromL4AndLetItGoAtASeam(t *testing.T) {
	for _, c := range acrossThree {
		t.Run(c.name, func(t *testing.T) {
			k := newStandingKinds()
			g := newStripes(lod.L4, lod.L4, lod.L4, lod.L4)
			s := newSeamRig(t, k, g, c.things(k)...)
			three := []int{sectorA, sectorB, sectorC}
			var versions [3]uint32
			for j, i := range three {
				versions[j] = s.r.sheet.version[i]
			}
			f := s.publish()
			for j, i := range three {
				if n, v := instancesOf(f, i), s.r.sheet.version[i]; n != 1 || v != versions[j]+1 ||
					f.sheet.held[i] == 0 {
					t.Errorf("the sector %d photographed of %d things, version %d, held %v; want "+
						"of one, %d, held", i, n, v, f.sheet.held[i], versions[j]+1)
				}
			}
			if got := s.recorded(f); len(got) != 0 {
				t.Errorf("records of %v, want none, the sheets holding them", got)
			}
			f.sheet.developed = true
			if f = s.publish(); len(f.sheet.photos) != 0 {
				t.Errorf("published again unchanged, %d photographs; want none",
					len(f.sheet.photos))
			}
			f.sheet.developed = true
			g.at[0] = lod.L3
			f = s.publish()
			if got := s.recorded(f); !slices.Equal(got, c.drawn) {
				t.Errorf("by a seam, records of %v, want of %v alive", got, c.drawn)
			}
			for j, i := range three[1:] {
				if n, v := instancesOf(f, i), s.r.sheet.version[i]; n != 0 ||
					v != versions[j+1]+2 {
					t.Errorf("by a seam, the sector %d photographed of %d things, version %d; "+
						"want of none, %d", i, n, v, versions[j+1]+2)
				}
			}
			if n, v := instancesOf(f, sectorA), s.r.sheet.version[sectorA]; n != -1 ||
				v != versions[0]+1 {
				t.Errorf("A at L3 photographed of %d things, version %d; want not, %d", n, v,
					versions[0]+1)
			}
		})
	}
}

func TestSheets_PhotographNoHandleNorRecordIt(t *testing.T) {
	k := newStandingKinds()
	g := newStripes(lod.L4, lod.L4, lod.L4, lod.L4)
	s := newSeamRig(t, k, g, acrossThree[0].things(k)...)
	f := s.publish()
	for _, i := range []int{sectorA, sectorB, sectorC} {
		if n := instancesOf(f, i); n != 1 {
			t.Errorf("the sector %d photographed of %d things, want of its piece alone", i, n)
		}
	}
	g.at[0] = lod.L3
	for _, e := range s.publish().all {
		if e.id == s.ids[0] {
			t.Error("the handle recorded, want its pieces alone")
		}
	}
}

func TestSheets_NeverThinWhatLivesInACell(t *testing.T) {
	k := newStandingKinds()
	g := newStripes(lod.L4, lod.L4, lod.L4, lod.L4)
	g.ladder.Keep[lod.L4] = 1e-9
	at := Z{Height: 10}
	things := append(acrossThree[0].things(k), acrossThree[1].things(k)...)
	things[4].box = plane.NewAABB(geom.NewVec(108, 600), 552, 40)
	things = append(things, stood{box: box(364, 100, 20), z: at, sprite: k.dome, fixed: true})
	s := newSeamRig(t, k, g, things...)
	// no sector photographed, so every record waits for its sheet
	s.r.sheet.perFrame = 0
	if got := s.recorded(s.publish()); !slices.Equal(got, []int{1, 2, 3, 4}) {
		t.Errorf("records of %v, want of the pieces and the big thing, the small one thinned",
			got)
	}
}

func TestSheets_LeaveWhatIsSmallToTheRingWhereTiersLie(t *testing.T) {
	k := newStandingKinds()
	at := Z{Height: 10}
	// in B: one by the seam with A, one farther, and one farther still a piece of what is no
	// handle, so its own
	s := newSeamRig(t, k, newStripes(lod.L3, lod.L4, lod.L4, lod.L4),
		stood{box: box(270, 420, 20), z: at, sprite: k.dome, fixed: true},
		stood{box: box(440, 400, 20), z: at, sprite: k.dome, fixed: true},
		stood{box: box(440, 300, 20), z: at, sprite: k.dome, of: 2})
	f := s.publish()
	if n := instancesOf(f, sectorB); n != 2 {
		t.Errorf("B photographed of %d things, want of the two farther", n)
	}
	if len(f.all) != 1 || f.all[0].id != s.ids[0] || !f.all[0].ringed || f.all[0].tier != 0 {
		t.Errorf("records %v, want of the one by the seam, ringed by it", f.all)
	}
}

func TestSheets_PublishAndComposeWhatLivesInACellAllocatingNothingOnceWarm(t *testing.T) {
	k := newStandingKinds()
	things := append(acrossThree[0].things(k), acrossThree[1].things(k)...)
	for i := range things[4:] {
		things[4+i].box = plane.NewAABB(geom.NewVec(108, 600), 552, 40)
	}
	s := newSeamRig(t, k, newStripes(lod.L3, lod.L4, lod.L4, lod.L4), things...)
	views := map[camera.Camera]*view.View{s.cam: s.views[0]}
	s.r.views = func() map[camera.Camera]*view.View { return views }
	s.publish()
	var fr render.Frame
	again := func() {
		s.r.figures.Front().sheet.developed = true
		s.r.Publish()
		s.r.Swap()
		s.look.sprites = s.look.sprites[:0]
		fr.Reset(s.cam)
		s.r.Compose(&fr, s.cam)
	}
	again()
	again()
	if n := testing.AllocsPerRun(10, again); n != 0 {
		t.Errorf("a frame of what lives in a cell allocates %v times, want none", n)
	}
}

func TestSheets_ClimbTheTiersToTheFinestLevelUnderEachCell(t *testing.T) {
	for _, g := range []*sector.Grid{
		sector.NewGrid(500, 300, false, false, 100),
		sector.NewGrid(1024, 1000, false, false, 256),
		sector.NewGrid(700, 700, true, true, 100),
	} {
		s := sheetsOver(g, 1, 1, 1)
		for i := range s.seen {
			s.seen[i] = lod.Level((i*5 + i/3) % int(lod.Levels))
		}
		levels := s.climb(nil)
		last := g.Tiers() - 1
		if want := int(s.slots[last]) + g.TierLen(last); len(levels) != want {
			t.Fatalf("%d levels over %d sectors, want %d", len(levels), g.Len(), want)
		}
		for k := range g.Tiers() {
			for i := range g.TierLen(k) {
				want := lod.Omitted
				g.Under(k, i, func(j int) { want = min(want, s.seen[j]) })
				if got := levels[int(s.slots[k])+i]; got != want {
					t.Errorf("%d sectors: tier %d, cell %d at %v, want the finest under it, %v",
						g.Len(), k, i, got, want)
				}
			}
		}
	}
}

func TestSheets_LayTheTiersOfTheGridSmallAsTheWorldSays(t *testing.T) {
	cfg := Config{
		Space:    SpaceCfg{Width: 1024, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 32},
		Sectors:  SectorsCfg{Buckets: 2},
	}
	for _, small := range []float32{0, 50} {
		cfg.Sectors.Small = small
		p := NewPlugin(cfg)
		s, g := &p.renderer.sheet, p.module.sectors.grid
		if s.tiers == nil || len(s.slots) != g.Tiers() || s.small != p.Read(0).Small() {
			t.Errorf("small set to %v: %d tiers laid at %v small, want the grid's %d at %v",
				small, len(s.slots), s.small, g.Tiers(), p.Read(0).Small())
		}
	}
}
