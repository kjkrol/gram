package world

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// counting is a look that counts the sprites it is handed, and keeps them.
type counting struct {
	Look
	sprites []render.SpriteID
}

func (c *counting) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z Z, atlas render.AtlasSource, id render.SpriteID, light render.Light, sway float32) {
	c.sprites = append(c.sprites, id)
	c.Look.Sprite(f, cam, box, z, atlas, id, light, sway)
}

// flatAtlas is an AtlasSource with no sheet: enough for gathering quads without drawing.
type flatAtlas struct{}

func (flatAtlas) Atlas() *render.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

// drawThrough spawns one 10x10 entity of sprite 1 per position, heading east, lets pick say which
// of them the View holds (nil: the zero View, which sees everything), draws once through rules and
// returns the sprites drawn.
func drawThrough(t *testing.T, pick func(ids []uid.UID64, v *view.View), rules []render.Rule, at ...geom.Vec) []render.SpriteID {
	t.Helper()
	v := &view.View{}
	cam := icamera.NewFromSpace(1000, 1000, 0)
	var drawing render.Rules
	if err := drawing.Add(rules...); err != nil {
		t.Fatal(err)
	}
	look := &counting{Look: ilook.NewFlat(1000, 1000)}
	r := newRenderer(flatAtlas{}, func(camera.Camera) *view.View { return v }, &drawing, func() Look { return look })

	var base goke.Comp[Base]
	var appearance goke.Comp[Appearance]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &appearance)
		f.Create(len(at))
		var ids []uid.UID64
		i := 0
		for f.Next() {
			bases, looks := base.Slice(&f.Cursor), appearance.Slice(&f.Cursor)
			for j, id := range f.Cursor.IDs {
				bases[j] = Base{Pos: Position{AABB: plane.NewAABB(at[i], 10, 10)}, Vel: Velocity{Dir: geom.NewVec(1, 0)}}
				looks[j] = Appearance{SpriteID: 1}
				ids = append(ids, id)
				i++
			}
		}
		if pick != nil {
			pick(ids, v)
		}
		r.Init(si)
	}})

	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	return look.sprites
}

var quarters = []geom.Vec{geom.NewVec(100, 100), geom.NewVec(700, 100), geom.NewVec(100, 700), geom.NewVec(700, 700)}

func TestRenderer_Compose_DrawsOnlyWhatTheViewContains(t *testing.T) {
	firstOnly := func(ids []uid.UID64, v *view.View) {
		v.Culled = true
		v.In.Add(ids[0])
	}
	if drawn := drawThrough(t, firstOnly, nil, quarters...); len(drawn) != 1 {
		t.Errorf("a View holding one entity drew %d sprites, want 1", len(drawn))
	}
	if drawn := drawThrough(t, nil, nil, quarters...); len(drawn) != 4 {
		t.Errorf("the zero View drew %d sprites, want all 4", len(drawn))
	}
}

// The rules read what the renderer reads itself — the Base, the Appearance — and settle what is
// drawn: Facing turns the sprite, Over lays one on top, Show leaves out the rest.
func TestRenderer_Compose_DrawsAsTheRulesSay(t *testing.T) {
	const east, crown render.SpriteID = 5, 9
	rules := []render.Rule{
		Facing(func(v Velocity) render.SpriteID { return east }),
		render.Over[Appearance](Appearance{SpriteID: crown}),
		render.Show(func(b Base) bool { return b.Pos.AABB.TopLeft.X < 500 }),
	}
	drawn := drawThrough(t, nil, rules, quarters...)
	if len(drawn) != 4 {
		t.Fatalf("drew %v, want the two on the left, each turned east under a crown", drawn)
	}
	for i := 0; i < len(drawn); i += 2 {
		if drawn[i] != east || drawn[i+1] != crown {
			t.Errorf("drew %v, want %v under %v for each", drawn, east, crown)
		}
	}
}

// WithRenderer takes the effects' looks for its drawing rules: a look issues a slot past the
// kinds', a second renderer adds nothing twice, and an effect's first look after it is refused.
func TestWithRenderer_TakesTheEffectsLooks(t *testing.T) {
	p := testPlugin()
	p.Effects().Define("frozen", effect.Spec{})
	frozen := p.Effects().Named("frozen")
	p.Effects().Define("wet", effect.Spec{})
	wet := p.Effects().Named("wet")
	free := p.Kinds().NewSprite()
	if got := frozen.Look(0); got != free+1 {
		t.Errorf("frozen's look = %d, want the world's next slot %d", got, free+1)
	}
	p.WithRenderer(flatAtlas{})
	p.WithRenderer(flatAtlas{})
	defer func() {
		if recover() == nil {
			t.Error("the first look of wet after WithRenderer did not panic")
		}
	}()
	wet.Look(0)
}
