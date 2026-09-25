package selection

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
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

	r := NewRenderer(h.tags.Selected)
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

func TestDefaultHighlightStyle_OutlinesOnTheMarksTierWhateverTheDepth(t *testing.T) {
	for name, cam := range map[string]camera.Camera{
		"from above": icamera.NewFromSpace(1000, 1000, 0),
		"isometric":  icamera.NewFromSpaceWithConfig(1000, 1000, 0, camera.Config{Projection: camera.Isometric{Cell: 32}}),
	} {
		var f render.Frame
		f.Reset(cam)
		DefaultHighlightStyle().Compose(&f, cam, geom.NewAABBAt(geom.NewVec(100, 100), 20, 20), 5)
		if f.Len() != 4 {
			t.Errorf("%s: %d pieces, want the four sides of the outline", name, f.Len())
		}
		f.Each(func(tier render.Tier, _ float32, _ []ebiten.Vertex) {
			if tier != render.Marks {
				t.Errorf("%s: an outline side on tier %d, want Marks", name, tier)
			}
		})
	}
}
