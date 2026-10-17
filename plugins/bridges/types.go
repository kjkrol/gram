package bridges

import (
	"fmt"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Spec is what a kind of bridge is: whom its deck carries (Allows, at Cost; 0 the ground's), how
// wide and thick its deck is, its railings and how wide a pier is.
type Spec struct {
	Allows    cell.Domain
	Width     float64
	Thickness float64
	Cost      float64
	Rail      cell.Rail
	Pier      float64
}

// Row is a bridge as a level's file writes it: from the head From to the head To.
type Row struct {
	From cell.ID `yaml:"from"`
	To   cell.ID `yaml:"to"`
}

type deckRow struct {
	At     cell.ID
	Links  cell.Links
	Height float64
	Dir    geom.Vec
}

type pierRow struct {
	At          geom.Vec
	Bottom, Top float64
}

// Types is the register of the kinds of bridges, defined on the Stage's kinds.
type Types struct {
	p     *Plugin
	specs map[string]Spec
	names []string
}

// Define registers the kind of bridge named name, as spec says, with the kinds of its parts,
// "<name>.deck" and "<name>.pier".
func (t *Types) Define(name string, spec Spec) {
	if spec.Width <= 0 || spec.Allows == 0 {
		panic(fmt.Sprintf("bridges: bridge %q: its deck needs a Width and a domain it carries",
			name))
	}
	if _, ok := t.specs[name]; ok {
		panic(fmt.Sprintf("bridges: bridge %q is defined twice", name))
	}
	w, brd := t.p.worldPlugin, t.p.board
	span := float64(brd.Board().Grid.CellSpan())
	kind.Define[Row](w.Kinds(), name, kind.Spec{
		comp.Load(func(r Row) world.Position {
			m := brd.Board().CellCenter(r.From)
			return world.Position{AABB: plane.NewAABB(geom.NewVec(m.X-span/2, m.Y-span/2), span,
				span)}
		}),
		comp.Const(world.Velocity{}),
	})
	shape := board.Shape{Size: span, Height: spec.Thickness}
	decks := board.NewEntities(brd, shape, func(r deckRow) cell.ID { return r.At })
	deck := []comp.Comp{
		comp.Load(func(r deckRow) cell.Floor {
			return cell.Floor{
				Allows:    spec.Allows,
				Height:    r.Height,
				Thickness: spec.Thickness,
				Cost:      spec.Cost,
				Graded:    true,
				Links:     r.Links,
				Width:     spec.Width,
			}
		}),
		comp.Const(spec.Rail),
		comp.Load(func(r deckRow) world.Velocity { return world.Velocity{Dir: r.Dir} }),
	}
	decks.Define(deckName(name), deck...)
	half := spec.Pier / 2
	kind.Define[pierRow](w.Kinds(), pierName(name), kind.Spec{
		comp.Load(func(r pierRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.At.X-half, r.At.Y-half),
				spec.Pier, spec.Pier)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(r pierRow) world.Z {
			return world.Z{Altitude: r.Bottom, Height: max(r.Top-r.Bottom, 0)}
		}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: math.Inf(1)}),
		comp.Const(world.Shadow{Cast: world.Baked}),
	})
	if t.specs == nil {
		t.specs = map[string]Spec{}
	}
	t.specs[name] = spec
	t.names = append(t.names, name)
}

// Named is the kind of bridge defined as name; an unknown name panics.
func (t *Types) Named(name string) Type {
	spec, ok := t.specs[name]
	if !ok {
		panic(fmt.Sprintf("bridges: no bridge is defined as %q", name))
	}
	w := t.p.worldPlugin
	return Type{
		name: name,
		spec: spec,
		deck: kind.Named[deckRow](w.Kinds(), deckName(name)),
		pier: kind.Named[pierRow](w.Kinds(), pierName(name)),
	}
}

// Type is a kind of bridge defined.
type Type struct {
	name string
	spec Spec
	deck kind.Of[deckRow]
	pier kind.Of[pierRow]
}

// Name is what the kind of bridge is called.
func (t Type) Name() string { return t.name }

// Spec is what the kind of bridge is.
func (t Type) Spec() Spec { return t.spec }

// Deck is the atlas slot its deck's pieces are drawn from, a sprite laid flat.
func (t Type) Deck() render.SpriteID { return t.deck.SpriteID() }

// Pier is the atlas slot its piers are drawn from, standing; a bridge over a map without relief
// stands on none.
func (t Type) Pier() render.SpriteID { return t.pier.SpriteID() }

func deckName(name string) string { return name + ".deck" }
func pierName(name string) string { return name + ".pier" }
