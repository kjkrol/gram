package selection

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/plugins/world"
	"image/color"
	"testing"

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

	r := NewRenderer(h.tags.Selected, h.world.Look, nil)
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

func TestHighlight_OutlinesEveryPieceOfTheFootprintOnTheMarksTier(t *testing.T) {
	var f render.Frame
	f.Reset(icamera.NewFromSpace(1000, 1000, 0))
	unit := render.Corners{{0, 0}, {10, 0}, {0, 10}, {10, 10}}
	DefaultHighlight.compose(&f, []render.Corners{unit, unit})
	if f.Len() != 8 {
		t.Errorf("%d pieces, want the four sides of each of two pieces", f.Len())
	}
	f.Each(func(tier render.Tier, _ float32, _ []render.Vertex) {
		if tier != render.Marks {
			t.Errorf("an outline side on tier %d, want Marks", tier)
		}
	})
}

func TestRenderer_OutlinesEachKindInItsHighlight(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 1, MaxSize: 10},
	})
	selected := tag.Tag[Family](1)
	var styles render.Styles
	styles.Set(7, Highlight{Line: color.RGBA{B: 255, A: 255}, Width: 4})
	r := NewRenderer(selected, w.Look, func() *render.Styles { return &styles })
	var base goke.Comp[world.Base]
	var marks goke.Comp[tag.Tags[Family]]
	var app goke.Comp[render.Appearance]
	ecs := goke.New()
	ecs.Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			f := si.NewFactory(&base, &marks, &app)
			f.Create(2)
			for f.Next() {
				for i := range f.Cursor.IDs {
					at := geom.NewVec(100+200*float64(i), 100)
					base.Slice(&f.Cursor)[i].Pos = world.Position{AABB: plane.NewAABB(at, 10, 10)}
					marks.Slice(&f.Cursor)[i] = tag.Tags[Family](0).With(selected)
					app.Slice(&f.Cursor)[i] = render.Appearance{SpriteID: render.SpriteID(6 + i)}
				}
			}
		}},
		goke.SystemFn{OnInit: r.Init},
	)
	cam := icamera.NewFromSpace(1000, 1000, 0)
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	blue, red := 0, 0
	f.Each(func(_ render.Tier, _ float32, v []render.Vertex) {
		switch {
		case v[0].ColorB == 1:
			blue++
		case v[0].ColorR > 0.8:
			red++
		}
	})
	if blue != 4 || red != 4 {
		t.Errorf("%d blue and %d red sides, want 4 each: the styled kind in its Highlight, the "+
			"other in DefaultHighlight", blue, red)
	}
}
