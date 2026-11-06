package world_test

import (
	"image/color"
	"maps"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// forestLooks are the forest's trees and the forest red, the forest's own look declared at L4
// where declared
type forestLooks struct{ declared bool }

var _ world.Figure = forestLooks{}

func (forestLooks) Define(*world.Plugin) {}

func (l forestLooks) Look(a *world.Atlas, w *world.Plugin) {
	red := render.Solid(color.RGBA{R: 255, A: 255})
	a.Add(kind.Named[treeRow](w.Kinds(), treeKind), 8, red)
	s := a.Add(kind.Named[forestRow](w.Kinds(), forestKind), 8, red)
	if l.declared {
		s.At(lod.L4, render.Solid(color.RGBA{G: 255, A: 255}))
	}
}

func TestSheets_KeepAFieldsPartsFoldedWithNoLookAtL4TillItUnfoldsThere(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256), forestLooks{})
	f.start(t, false)
	f.w.Sheets()
	f.run(2)
	before := f.bySector()
	f.look(geom.NewVec(1300, 256))
	f.until(t, "fold", 2*int(foldAfter/frame))
	want := map[int]int{0: len(before[0]), 8: len(before[8])}
	if got := f.w.SheetShots(); !maps.Equal(got, want) || want[0]+want[8] == 0 {
		t.Fatalf("kept %v parts by sector, want those folded, %v", got, want)
	}
	f.run(2)
	if got := f.w.SheetShots(); !maps.Equal(got, want) {
		t.Errorf("kept %v parts by sector with the parts gone, want them kept, %v", got, want)
	}
	f.look(geom.NewVec(256, 256))
	f.until(t, "unfold", 10)
	f.run(1)
	if got := f.w.SheetShots(); len(got) != 0 {
		t.Errorf("kept %v parts by sector unfolded again, want none", got)
	}
}

func TestSheets_KeepNoPartsOfAFieldWithALookOfItsOwnAtL4(t *testing.T) {
	f := newForest(t, geom.NewVec(256, 256), forestLooks{declared: true})
	f.start(t, false)
	f.w.Sheets()
	f.run(2)
	f.look(geom.NewVec(1300, 256))
	f.until(t, "fold", 2*int(foldAfter/frame))
	if got := f.w.SheetShots(); len(got) != 0 {
		t.Errorf("kept %v parts by sector of a forest declaring its look at L4, want none", got)
	}
}
