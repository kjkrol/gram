package world_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

// the forest's sectors, all drawn at L4 by a camera 1200 wide looking at (256, 256)
var seenAtL4 = []int{0, 1, 8, 9}

// has the forest start, or load, seen at L4 by its camera 1200 wide, its sheets photographed
func (f *forest) sheeted(t *testing.T, load bool) {
	t.Helper()
	f.cam.SetViewport(300, 300)
	f.zoom(0.25)
	f.start(t, load)
	f.w.Sheets()
	f.w.Write().NewView(f.cam)
}

// has the forest tick once and publish, and gives how many things it photographs by sector
func (f *forest) cycle() map[int]int {
	f.run(1)
	f.w.Published()
	return f.w.Photographed()
}

// the parts the forest's rules plant at density in each of sectors, by sector
func (f *forest) planting(density uint64, sectors ...int) map[int]int {
	want := map[int]int{}
	for _, s := range sectors {
		want[s] = plantedIn(f.id(), s, density)
	}
	return want
}

// whether the forest carries Unfolded
func (f *forest) isUnfolded() bool {
	for f.forests.All(); f.forests.Next(); {
		return f.marked.Present(f.forests.Cursor())
	}
	return false
}

// has the forest show no part spawned, no stretch and no Unfolded
func (f *forest) noneUnfolded(t *testing.T) {
	t.Helper()
	if trees, n := f.bySector(), f.w.Stretches(); len(trees) != 0 || n != 0 || f.isUnfolded() {
		t.Errorf("%d sectors of trees, %d stretches, Unfolded %v; want none spawned, none made, "+
			"no Unfolded", len(trees), n, f.isUnfolded())
	}
}

func TestPreview_PhotographsAFieldNeverUnfoldedWithThePartsItsRulesGiveSpawningNone(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256), forestLooks{})
	f.sheeted(t, false)
	for k := range 2 {
		if got := f.cycle(); len(got) != 0 {
			t.Fatalf("frame %d photographed %v things by sector before the preview, want none", k,
				got)
		}
	}
	if got := sectorsOf(f.unfolded); !slices.Equal(got, seenAtL4) {
		t.Fatalf("told the Unfold of sectors %v, want the previews of %v", got, seenAtL4)
	}
	got := f.cycle()
	want := f.planting(2, seenAtL4...)
	if shots := f.w.SheetShots(); !maps.Equal(shots, want) {
		t.Errorf("the sheets keep %v parts by sector, want those its rules plant, %v", shots, want)
	}
	if !maps.Equal(got, want) {
		t.Errorf("photographed %v things by sector, want the parts previewed, %v", got, want)
	}
	f.noneUnfolded(t)
	for range 30 {
		if got := f.cycle(); len(got) != 0 {
			t.Fatalf("photographed %v things by sector again, nothing changed", got)
		}
	}
	if n := len(f.unfolded); n != len(seenAtL4) {
		t.Errorf("told %d Unfolds, want the %d previews alone", n, len(seenAtL4))
	}
}

func TestPreview_NoneOfAFieldWithALookOfItsOwnAtL4(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256), forestLooks{declared: true})
	f.sheeted(t, false)
	for range 5 {
		f.cycle()
	}
	if got := f.w.SheetShots(); len(got) != 0 || len(f.unfolded) != 0 {
		t.Errorf("the sheets keep %v parts by sector, told %d Unfolds; want none of a forest "+
			"declaring its look at L4", got, len(f.unfolded))
	}
}

func TestPreview_GoesAsTheFieldUnfoldsAndComesBackFromItsPartsAsItFolds(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256), forestLooks{})
	f.sheeted(t, false)
	for range 3 {
		f.cycle()
	}
	previewed := f.w.SheetShots()
	f.zoom(0.5)
	f.cycle()
	f.cycle()
	if got := f.w.SheetShots(); len(got) != 0 {
		t.Fatalf("unfolded, the sheets keep %v parts by sector, want none", got)
	}
	f.plantedIn(t, 2, seenAtL4...)
	f.zoom(0.25)
	for range 2*int(foldAfter/frame) + 2 {
		if f.cycle(); len(f.folded) > 0 {
			break
		}
	}
	if got := sectorsOf(f.folded); !slices.Equal(got, seenAtL4) {
		t.Fatalf("folded sectors %v, want %v", got, seenAtL4)
	}
	if got := f.w.SheetShots(); !maps.Equal(got, previewed) {
		t.Errorf("folded, the sheets keep %v parts by sector, want those the preview had, %v",
			got, previewed)
	}
	for range 10 {
		f.cycle()
	}
	if n := len(f.unfolded); n != 2*len(seenAtL4) {
		t.Errorf("told %d Unfolds, want a preview and an unfolding of each sector, %d", n,
			2*len(seenAtL4))
	}
}

func TestPreview_ASaveKeepsNoneAndALoadPreviewsAnew(t *testing.T) {
	t.Chdir(t.TempDir())
	was := newForest(t, geom.NewVec(256, 256), forestLooks{})
	was.sheeted(t, false)
	was.cycle()
	// the previews told their Unfold, their parts not yet photographed
	was.cycle()
	if err := was.eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := newForest(t, geom.NewVec(256, 256), forestLooks{})
	now.sheeted(t, true)
	now.noneUnfolded(t)
	if got := now.w.SheetShots(); len(got) != 0 {
		t.Fatalf("loaded, the sheets keep %v parts by sector, want none", got)
	}
	for range 3 {
		now.cycle()
	}
	if got, want := now.w.SheetShots(), now.planting(2, seenAtL4...); !maps.Equal(got, want) {
		t.Errorf("loaded, the sheets keep %v parts by sector, want those previewed anew, %v", got,
			want)
	}
	now.noneUnfolded(t)
}

func TestPreview_LaysThePartsOnTheGround(t *testing.T) {
	const ground = 7
	f := newGroundedForest(t, geom.NewVec(256, 256), ground, forestLooks{})
	f.sheeted(t, false)
	for range 3 {
		f.cycle()
	}
	alts := f.w.SheetAltitudes()
	if len(alts) == 0 {
		t.Fatal("no part previewed")
	}
	for _, a := range alts {
		if a != ground {
			t.Fatalf("a part previewed at altitude %v, want the ground's, %v", a, ground)
		}
	}
}
