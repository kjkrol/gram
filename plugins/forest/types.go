package forest

import (
	"fmt"
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Spec is what a kind of tree is: how tall it stands and how wide its crown, in world units, how
// it looks and casts its shade, whether it stops what collides, and the game's roles it plays.
type Spec struct {
	Height float32
	// Crown is the side of the tree's box.
	Crown float32
	// Trunk is the share of its Height under its crown where its Side is drawn of its Colours.
	Trunk   float32
	Colours Colours
	// Top is its crown seen from above, at L3 and in the world's sheets; drawn of its Colours
	// where it is nil.
	Top render.SpriteDrawer
	// Side is its trunk and crown seen from the side, at L2; drawn of its Colours where it is nil.
	Side render.SpriteDrawer
	// Shadow is how it casts and takes shade, Baked taking all where it is zero.
	Shadow light.Shadow
	// Solid has it a post nothing walks through, which what collides meets.
	Solid bool
	// Body has it drawn at L2 as a body, its Side on the four faces and over the top of its box;
	// a billboard of its Side unless set.
	Body bool
	// Roles are the game's roles it plays besides TreeRole.
	Roles []string
}

// Colours are what a tree's looks are drawn in where its Spec gives no drawer of its own: a dark
// green crown on a brown trunk where they are zero.
type Colours struct {
	Crown  color.RGBA
	Leaves color.RGBA
	Trunk  color.RGBA
}

var defaultColours = Colours{
	Crown:  color.RGBA{R: 30, G: 95, B: 45, A: 255},
	Leaves: color.RGBA{R: 70, G: 135, B: 60, A: 255},
	Trunk:  color.RGBA{R: 95, G: 70, B: 45, A: 255},
}

// the side of a tree's sprites in the atlas
const spritePx = 48

type treeRow struct{}

// Types is the register of the kinds of trees, defined on the Stage's kinds.
type Types struct {
	p     *Plugin
	specs map[string]Spec
	names []string
}

// Define registers the kind of tree named name, as spec says: a part of its forest standing for
// good, playing TreeRole and spec's Roles.
func (t *Types) Define(name string, spec Spec) {
	if spec.Height <= 0 || spec.Crown <= 0 {
		panic(fmt.Sprintf("forest: tree %q needs a Height and a Crown", name))
	}
	if spec.Trunk < 0 || spec.Trunk >= 1 {
		panic(fmt.Sprintf("forest: tree %q: its Trunk is a share of its Height under 1", name))
	}
	if _, ok := t.specs[name]; ok {
		panic(fmt.Sprintf("forest: tree %q is defined twice", name))
	}
	if spec.Colours == (Colours{}) {
		spec.Colours = defaultColours
	}
	w := t.p.worldPlugin
	roles := w.Roles()
	plays := []*ceremony.Part{roles.Named(TreeRole)}
	for _, r := range spec.Roles {
		plays = append(plays, roles.Named(r))
	}
	half := spec.Crown / 2
	own := []comp.Comp{
		// a part: SpawnPart lays its box off the middle of its forest's sector
		comp.Const(world.Position{
			AABB: plane.NewAABB(geom.NewVec(-half, -half), spec.Crown, spec.Crown),
		}),
		comp.Const(world.Z{Height: spec.Height}),
		ceremony.Plays(plays...),
	}
	if spec.Shadow != (light.Shadow{}) {
		own = append(own, comp.Const(spec.Shadow))
	}
	if spec.Solid {
		own = append(own,
			comp.Const(collision.Collider{}),
			comp.Const(collision.Physics{Mass: f32.Inf(1)}))
	}
	kind.Define[treeRow](w.Kinds(), name, w.Roster().Spec(board.StandingTemplate, own...))
	if t.specs == nil {
		t.specs = map[string]Spec{}
	}
	t.specs[name] = spec
	t.names = append(t.names, name)
}

// Named is the kind of tree defined as name; an unknown name panics.
func (t *Types) Named(name string) Type {
	spec, ok := t.specs[name]
	if !ok {
		panic(fmt.Sprintf("forest: no tree is defined as %q", name))
	}
	return Type{name: name, spec: spec, kind: kind.Named[treeRow](t.p.worldPlugin.Kinds(), name)}
}

// Type is a kind of tree defined.
type Type struct {
	name string
	spec Spec
	kind kind.Of[treeRow]
}

// Name is what the kind of tree is called.
func (t Type) Name() string { return t.name }

// Spec is what the kind of tree is.
func (t Type) Spec() Spec { return t.spec }

// ID is the kind's in the world, its trees' world.Base.TypeID.
func (t Type) ID() kind.ID { return t.kind.ID() }

func (t Type) look(a *world.Atlas) {
	top, side := t.spec.Top, t.spec.Side
	if top == nil {
		top = crown(t.spec.Colours)
	}
	if side == nil {
		side = standing(t.spec.Colours, t.spec.Trunk)
	}
	s := a.Add(t.kind, spritePx, side)
	if t.spec.Body {
		s = s.Upright()
	}
	// nothing of its own from L4 out: the sheets photograph it in its L3 look only where its
	// kind declares none there
	s.At(lod.L3, top).Nothing(lod.Strategic)
}
