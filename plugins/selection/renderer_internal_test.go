package selection

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

func TestRenderer_Init_QueryMatchesOnlySelectedEntity(t *testing.T) {
	h := newHarness(t)
	selectedID := h.seed(50, 50, 10)
	unselectedID := h.seed(500, 500, 10)
	h.start()

	h.click(55, 55, false)
	if !h.isSelected(*selectedID) {
		t.Fatal("sanity check failed: expected the clicked entity to be selected")
	}
	if h.isSelected(*unselectedID) {
		t.Fatal("sanity check failed: expected the other entity to remain unselected")
	}

	r := NewRenderer(h.tags.Selected, h.world.Look)
	h.ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) { r.Init(si) }})

	r.query.All()
	found := map[uid.UID64]bool{}
	for r.query.Next() {
		cur := r.query.Cursor()
		marks := r.marks.Slice(cur)
		for i, id := range cur.IDs {
			if marks[i].Has(r.selected) {
				found[id] = true
			}
		}
	}
	if !found[*selectedID] {
		t.Error("expected Renderer's query to match the selected entity")
	}
	if found[*unselectedID] {
		t.Error("expected Renderer's query to NOT match the unselected entity")
	}
}

func TestDefaultHighlightStyle_OutlinesEveryPieceOfTheFootprintOnTheMarksTier(t *testing.T) {
	var f render.Frame
	f.Reset(icamera.NewFromSpace(1000, 1000, 0))
	unit := render.Corners{{0, 0}, {10, 0}, {0, 10}, {10, 10}}
	DefaultHighlightStyle().Compose(&f, []render.Corners{unit, unit})
	if f.Len() != 8 {
		t.Errorf("%d pieces, want the four sides of each of two pieces", f.Len())
	}
	f.Each(func(tier render.Tier, _ float32, _ []ebiten.Vertex) {
		if tier != render.Marks {
			t.Errorf("an outline side on tier %d, want Marks", tier)
		}
	})
}
