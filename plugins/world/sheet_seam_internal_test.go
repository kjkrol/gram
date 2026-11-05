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
	"github.com/kjkrol/gram/render/gputest"
)

// stripes is a world 1024 by 1000 cut into sectors 256 a side, four by four, each column seen at
// its level, ladder the world's
type stripes struct {
	*sector.Grid
	at     *[4]lod.Level
	ladder lod.Ladder
}

func newStripes(at ...lod.Level) stripes {
	c := stripes{Grid: sector.NewGrid(1024, 1000, false, false, 256), at: new([4]lod.Level)}
	copy(c.at[:], at)
	return c
}

func (c stripes) Level(_ camera.Camera, i int) lod.Level { return c.at[i%4] }
func (c stripes) Ladder() lod.Ladder                     { return c.ladder }

// newSeamRig is a sheet rig over stripes, the sectors a box covers its grid's
func newSeamRig(t *testing.T, k standingKinds, g stripes, things ...stood) *sheetRig {
	t.Helper()
	s := newSheetRig(t, k, g, things...)
	s.r.covers = g.Cover
	return s
}

// instancesOf is how many things f's photograph of sector i lays, -1 where f has none of it
func instancesOf(f *figures, i int) int {
	for _, p := range f.sheet.photos {
		if p.sector == i {
			return (p.to - p.from) / photoFloats
		}
	}
	return -1
}

// composedSprites is the sprites the rig's camera draws through the Look, points left out
func (s *sheetRig) composedSprites() []render.SpriteID {
	s.look.sprites = s.look.sprites[:0]
	var fr render.Frame
	fr.Reset(s.cam)
	s.r.Compose(&fr, s.cam)
	return s.look.sprites
}

func TestSheets_DrawWhatStandsByASeamAliveAtTheFinerLevel(t *testing.T) {
	k := newStandingKinds()
	g := newStripes(lod.L3, lod.L4, lod.L4, lod.L4)
	// in the sector from (256, 256): one across the seam, one whose box grown by its side reaches
	// over it, one farther and a unit by it; one across the seam from the finer side, lower down
	s := newSeamRig(t, k, g,
		stood{box: box(240, 300, 40), z: Z{Height: 10}, sprite: k.dome, fixed: true},
		stood{box: box(270, 420, 20), z: Z{Height: 10}, sprite: k.dome, fixed: true},
		stood{box: box(440, 400, 20), z: Z{Height: 10}, sprite: k.dome, fixed: true},
		stood{box: box(270, 470, 20), sprite: k.dome},
		stood{box: box(230, 700, 40), z: Z{Height: 10}, sprite: k.dome, fixed: true})
	f := s.publish()
	if n := instancesOf(f, 5); n != 1 || !f.sheet.held[5] {
		t.Fatalf("the sector by the seam photographed of %d things, held %v; want of the one "+
			"farther, held", n, f.sheet.held[5])
	}
	if n := instancesOf(f, 9); n > 0 || f.sheet.held[9] {
		t.Errorf("the sector the lower one crosses into photographed of %d, held %v; want of none",
			n, f.sheet.held[9])
	}
	if got := s.recorded(f); !slices.Equal(got, []int{0, 1, 3, 4}) {
		t.Errorf("records of %v, want of those by the seam and the unit", got)
	}
	for _, e := range f.all {
		ringed := e.id == s.ids[0] || e.id == s.ids[1]
		if e.ringed != ringed || e.sheet != 0 {
			t.Errorf("the record of %d ringed %v, by sheet %d; want ringed %v, by none",
				slices.Index(s.ids, e.id), e.ringed, e.sheet, ringed)
		}
	}
	// short of L4 a figure is drawn through the Look, at L4 a point
	want := []render.SpriteID{k.dome, k.dome, k.dome}
	if got := s.composedSprites(); !slices.Equal(got, want) {
		t.Errorf("drew %v through the Look, want those by the seam at L3 %v, the unit a point",
			got, want)
	}
	f.sheet.developed = true
	version := s.r.sheet.version[5]
	g.at[0] = lod.L4
	f = s.publish()
	if n := instancesOf(f, 5); n != 3 || s.r.sheet.version[5] != version+1 {
		t.Fatalf("the seam gone, the sector photographed of %d things, version %d; want of all "+
			"three, %d", n, s.r.sheet.version[5], version+1)
	}
	if got := s.recorded(f); !slices.Equal(got, []int{3}) {
		t.Errorf("the seam gone, records of %v, want of the unit alone", got)
	}
	f.sheet.developed = true
	if f = s.publish(); len(f.sheet.photos) != 0 || s.r.sheet.version[5] != version+1 {
		t.Errorf("published again, %d photographs, version %d; want none, %d",
			len(f.sheet.photos), s.r.sheet.version[5], version+1)
	}
	g.at[0] = lod.L3
	if f = s.publish(); instancesOf(f, 5) != 1 || s.r.sheet.version[5] != version+2 {
		t.Errorf("the seam back, the sector photographed of %d things, version %d; want of the "+
			"one farther, %d", instancesOf(f, 5), s.r.sheet.version[5], version+2)
	}
}

func TestSheets_NeverThinWhatStandsAliveByASeam(t *testing.T) {
	k := newStandingKinds()
	g := newStripes(lod.L3, lod.L4, lod.L4, lod.L4)
	g.ladder.Keep[lod.L4] = 1e-9
	s := newSeamRig(t, k, g,
		stood{box: box(240, 300, 40), sprite: k.dome, fixed: true},
		stood{box: box(270, 420, 20), sprite: k.dome, fixed: true},
		stood{box: box(470, 400, 20), sprite: k.dome})
	if got := s.recorded(s.publish()); !slices.Equal(got, []int{0, 1}) {
		t.Errorf("records of %v, want of those by the seam, the unit at L4 thinned", got)
	}
}

func TestSheets_DrawWhatABoxCrossesIntoAFinerSectorTwoAwayAlive(t *testing.T) {
	k := newStandingKinds()
	g := newStripes(lod.L3, lod.L4, lod.L4, lod.L4)
	// its middle in the third column, its box over the second and a grown one over the first
	s := newSeamRig(t, k, g,
		stood{box: plane.NewAABB(geom.NewVec(400, 300), 260, 20), sprite: k.dome, fixed: true})
	f := s.publish()
	if len(f.all) != 1 || !f.all[0].ringed || instancesOf(f, 6) > 0 {
		t.Errorf("records %v, the third column photographed of %d; want the long one ringed, "+
			"photographed nowhere", f.all, instancesOf(f, 6))
	}
}

func TestSheets_LayADeckWholeAndLevelAcrossASeam(t *testing.T) {
	gputest.Need(t)
	k := newStandingKinds()
	g := newStripes(lod.L3, lod.L4, lod.L4, lod.L4)
	// a deck of pieces 24 long from 240, 30 up and 2 thick: three texels a piece
	deck := Z{Altitude: 30, Height: 2}
	var pieces []stood
	for x := float32(240); x < 480; x += 24 {
		pieces = append(pieces, stood{box: plane.NewAABB(geom.NewVec(x, 300), 24, 24), z: deck,
			sprite: k.flat, fixed: true})
	}
	s := newSeamRig(t, k, g, pieces...)
	f := s.publish()
	alive := map[int]bool{}
	for _, e := range f.all {
		alive[slices.Index(s.ids, e.id)] = true
		if e.z.Top() != deck.Top() {
			t.Errorf("a piece alive tops at %v, want %v", e.z.Top(), deck.Top())
		}
	}
	if len(alive) != 2 || !alive[0] || !alive[1] {
		t.Fatalf("pieces alive %v, want the one across the seam and the one by it", alive)
	}
	s.r.develop(nil)
	albedo, relief := texels(s.r.sheet.albedo), texels(s.r.sheet.relief)
	// the deck's texels from the sector's edge, x 32, on; it runs along the row 39
	for x := 32; x < 60; x++ {
		if in := x >= 36; (albedo(x, 39)[3] != 0) != in {
			t.Errorf("the albedo at x %d is %v, want the deck from x 36 on alone", x, albedo(x, 39))
		}
	}
	for _, x := range []int{37, 45, 58} {
		if h := heightOf(relief(x, 39)); f32Off(h, deck.Top()) > 0.005 {
			t.Errorf("the deck in the sheet at x %d is %v high, want %v as alive", x, h, deck.Top())
		}
	}
}

func TestSheets_PublishAndComposeWhatStandsByASeamAllocatingNothingOnceWarm(t *testing.T) {
	k := newStandingKinds()
	s := newSeamRig(t, k, newStripes(lod.L3, lod.L4, lod.L4, lod.L4),
		stood{box: box(240, 300, 40), sprite: k.dome, fixed: true},
		stood{box: box(270, 420, 20), sprite: k.dome, fixed: true},
		stood{box: box(440, 400, 20), sprite: k.dome, fixed: true})
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
		t.Errorf("a frame of what stands by a seam allocates %v times, want none", n)
	}
}
