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

// counting is a look that counts the sprites it is handed, and keeps them with their angles.
type counting struct {
	Look
	sprites []render.SpriteID
	angles  []float32
}

func (c *counting) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z Z, atlas render.AtlasSource, a render.Appearance, light render.Light) {
	c.sprites = append(c.sprites, a.SpriteID)
	c.angles = append(c.angles, a.Angle)
	c.Look.Sprite(f, cam, box, z, atlas, a, light)
}

// flatAtlas is an AtlasSource with no sheet: enough for gathering quads without drawing.
type flatAtlas struct{}

func (flatAtlas) Atlas() *render.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

// drawThrough spawns one 10x10 entity of sprite 1 per position, heading dir, lets pick say which
// of them the View holds (nil: the zero View, which sees everything), draws once through rules and
// returns the look that recorded what was drawn.
func drawThrough(t *testing.T, pick func(ids []uid.UID64, v *view.View), rules []render.Rule, dir geom.Vec, at ...geom.Vec) *counting {
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
				bases[j] = Base{Pos: Position{AABB: plane.NewAABB(at[i], 10, 10)}, Vel: Velocity{Dir: dir}}
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
	return look
}

var quarters = []geom.Vec{geom.NewVec(100, 100), geom.NewVec(700, 100), geom.NewVec(100, 700), geom.NewVec(700, 700)}

func TestRenderer_Compose_DrawsOnlyWhatTheViewContains(t *testing.T) {
	firstOnly := func(ids []uid.UID64, v *view.View) {
		v.Culled = true
		v.In.Add(ids[0])
	}
	if drawn := drawThrough(t, firstOnly, nil, geom.NewVec(1, 0), quarters...).sprites; len(drawn) != 1 {
		t.Errorf("a View holding one entity drew %d sprites, want 1", len(drawn))
	}
	if drawn := drawThrough(t, nil, nil, geom.NewVec(1, 0), quarters...).sprites; len(drawn) != 4 {
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
	drawn := drawThrough(t, nil, rules, geom.NewVec(1, 0), quarters...).sprites
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

// Turning writes the Appearance's Angle from the way the entity moves, in the engine's one
// convention: 0 east, against the clock with the screen's y growing down, in degrees.
func TestRenderer_Compose_TurningTurnsTheSpriteTheWayItMoves(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  geom.Vec
		want float32
	}{
		{"east", geom.NewVec(1, 0), 0},
		{"north, up the screen", geom.NewVec(0, -1), 90},
		{"west", geom.NewVec(-1, 0), -180}, // atan2's range: the same turn as 180
		{"still: the Appearance's own angle holds", geom.Vec{}, 0},
	} {
		look := drawThrough(t, nil, []render.Rule{Turning()}, tc.dir, quarters[0])
		if len(look.angles) != 1 || look.angles[0] != tc.want {
			t.Errorf("%s: drew angles %v, want [%v]", tc.name, look.angles, tc.want)
		}
	}
}
