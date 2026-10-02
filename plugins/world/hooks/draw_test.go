package hooks_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/hooks"
	"github.com/kjkrol/gram/render"
)

// ghost marks an entity drawn as a ghost; mood is what an entity feels, read by With.
type (
	ghost struct{}
	mood  struct{ Angry bool }
)

// drawn is one entity of a drawing test: how it moves and what it carries.
type drawn struct {
	vel   world.Velocity
	ghost bool
	mood  *mood
}

// draw runs the rules over the entities, each starting from sprite 1, as the world's renderer does,
// and returns each one's layers in the order given.
func draw(t *testing.T, rules []plugin.Rule, entities ...drawn) [][]world.Appearance {
	t.Helper()
	h := &host.EachHost[world.Drawing]{}
	for _, r := range rules {
		if err := h.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	out := make([][]world.Appearance, len(entities))
	var base goke.Comp[world.Base]
	var ghosts goke.Comp[ghost]
	var moods goke.Comp[mood]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for k, e := range entities {
			comps := []goke.Addable{&base}
			if e.ghost {
				comps = append(comps, &ghosts)
			}
			if e.mood != nil {
				comps = append(comps, &moods)
			}
			f := si.NewFactory(comps...)
			f.Create(1)
			f.Next()
			// the box's left edge is the entity's place in the order given
			base.Slice(&f.Cursor)[0] = world.Base{Vel: e.vel, Pos: world.Position{AABB: plane.NewAABB(geom.NewVec(float64(k), 0), 1, 1)}}
			if e.mood != nil {
				moods.Slice(&f.Cursor)[0] = *e.mood
			}
			out[k] = []world.Appearance{{SpriteID: render.SpriteID(100 + k)}}
		}
		qb := si.NewQueryBuilder(&base)
		h.Bind(qb)
		q := qb.Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			bases := base.Slice(cur)
			h.Run(plugin.Tick{}, cur, func(i int) world.Drawing {
				k := int(bases[i].Pos.TopLeft.X)
				return world.Drawing{ID: cur.IDs[i], Base: &bases[i], Layers: &out[k]}
			})
		}
	}})
	return out
}

var (
	east  = world.Velocity{Dir: geom.NewVec(1, 0), Value: 1}
	north = world.Velocity{Dir: geom.NewVec(0, -1), Value: 1}
)

const eastward, other, spook, crown render.SpriteID = 7, 3, 9, 11

func facing() plugin.Rule {
	return hooks.Facing(func(v world.Velocity) render.SpriteID {
		if v.Dir.X > 0 {
			return eastward
		}
		return other
	})
}

func TestFacing_PicksTheSpriteFromTheEntitysHeading(t *testing.T) {
	got := draw(t, []plugin.Rule{facing()}, drawn{vel: east}, drawn{vel: north})
	if got[0][0].SpriteID != eastward || got[1][0].SpriteID != other {
		t.Errorf("sprites %v and %v, want %v heading east and %v heading north", got[0][0].SpriteID, got[1][0].SpriteID, eastward, other)
	}
}

// As draws one carrying the component as something else, after Facing has turned it; the others
// keep their own sprite.
func TestAs_DrawsACarrierAsAnotherSpriteAfterFacing(t *testing.T) {
	got := draw(t, []plugin.Rule{facing(), hooks.As[ghost](world.Appearance{SpriteID: spook})},
		drawn{vel: east, ghost: true}, drawn{vel: east})
	if len(got[0]) != 1 || got[0][0].SpriteID != spook {
		t.Errorf("the ghost drawn as %v, want %v alone", got[0], spook)
	}
	if got[1][0].SpriteID != eastward {
		t.Errorf("the other drawn as %v, want %v", got[1][0].SpriteID, eastward)
	}
}

func TestOverlay_LaysASpriteOnACarrierAlone(t *testing.T) {
	got := draw(t, []plugin.Rule{hooks.Overlay[ghost](world.Appearance{SpriteID: crown})}, drawn{ghost: true}, drawn{})
	if len(got[0]) != 2 || got[0][0].SpriteID != 100 || got[0][1].SpriteID != crown {
		t.Errorf("the carrier drawn as %v, want its own sprite under %v", got[0], crown)
	}
	if len(got[1]) != 1 {
		t.Errorf("the other drawn as %v, want its own sprite alone", got[1])
	}
}

// With reads the component every frame: the sprite follows its value.
func TestWith_ReworksTheSpriteByTheComponentsValue(t *testing.T) {
	angry := hooks.With(func(a world.Appearance, m mood) world.Appearance {
		if m.Angry {
			a.SpriteID += 50
		}
		return a
	})
	got := draw(t, []plugin.Rule{angry}, drawn{mood: &mood{Angry: true}}, drawn{mood: &mood{}}, drawn{})
	if got[0][0].SpriteID != 150 || got[1][0].SpriteID != 101 || got[2][0].SpriteID != 102 {
		t.Errorf("sprites %v, %v, %v; want the angry one reworked (150), the calm one and the one with no mood left alone (101, 102)",
			got[0][0].SpriteID, got[1][0].SpriteID, got[2][0].SpriteID)
	}
}
