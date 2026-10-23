package selection

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
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

	r := NewRenderer(h.tags.Selected, h.world.Read(0).Look, nil)
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

func TestHighlight_OutlinesEveryPieceOfTheFootprintAsAMark(t *testing.T) {
	var f render.Frame
	f.Reset(cameratest.NewFromSpace(1000, 1000, 0))
	unit := render.Corners{{0, 0}, {10, 0}, {0, 10}, {10, 10}}
	DefaultHighlight.compose(&f, []render.Corners{unit, unit})
	if f.Len() != 8 {
		t.Errorf("%d pieces, want the four sides of each of two pieces", f.Len())
	}
	f.Each(func(at render.At, _ []render.Vertex) {
		if at != render.OnTop(outlineLevel) {
			t.Errorf("an outline side lies at %v, want a mark on top", at)
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
	r := NewRenderer(selected, w.Read(0).Look, func() *render.Styles { return &styles })
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
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	blue, red := 0, 0
	f.Each(func(_ render.At, v []render.Vertex) {
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

type drawsCtx struct {
	plugin.Installer
	drawn []render.Source
}

func (*drawsCtx) UseModule(goke.Module)      {}
func (*drawsCtx) Pool() *parallel.Pool       { return nil }
func (*drawsCtx) Hosts(...host.Host)         {}
func (*drawsCtx) ECS() *goke.ECS             { return goke.New() }
func (c *drawsCtx) Draws(s ...render.Source) { c.drawn = append(c.drawn, s...) }

func TestPlugin_DrawsTheHighlightsAndTheBox(t *testing.T) {
	p := NewPlugin(world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}))
	ctx := &drawsCtx{}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.drawn) != 1 || ctx.drawn[0] != render.Source(p.renderer) {
		t.Errorf("selection draws %v, want its renderer", ctx.drawn)
	}
}
