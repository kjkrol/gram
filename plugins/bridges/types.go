package bridges

import (
	"fmt"
	"image/color"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Spec is what a kind of bridge is: whom its deck carries (Allows, at Pace; 0 the ground's) and
// how heavy they may be (Load, kg; 0 any), how wide and thick its deck is, its railings and how
// wide a pier is.
type Spec struct {
	Allows    cell.Domain
	Load      float32
	Width     float32
	Thickness float32
	Pace      float64
	Rail      cell.Rail
	Pier      float32
	// Span is how far apart the piers of a bridge laid along a path stand, in world units; 0
	// stands none.
	Span float32
	// Model has a bridge laid along a path drawn as solids, its deck Girder deep, in the colours
	// of its Body and its Parapet; None leaves it to its deck's and piers' looks.
	Model   Model
	Girder  float32
	Body    color.RGBA
	Parapet color.RGBA
}

// Row is a bridge as a level's file writes it: from the head From to the head To, straight and
// level; or along Path, every cell from head to head as [x, y, dx, dy], its axis passing the
// cell (dx, dy) off the middle, turning and climbing as they lie, Lanes marked and Width wide.
type Row struct {
	From  cell.ID      `yaml:"from"`
	To    cell.ID      `yaml:"to"`
	Path  [][4]float32 `yaml:"path"`
	Lanes uint8        `yaml:"lanes"`
	Width float32      `yaml:"width"`
}

type deckRow struct {
	At     cell.ID
	Links  cell.Links
	Height float32
	Dir    geom.Vec
	// a winding bridge's: its own width, its axis off the cell's middle, its lanes
	Winds bool
	Width float32
	Off   geom.Vec
	Lanes uint8
}

type pierRow struct {
	At          geom.Vec
	Bottom, Top float32
}

// a bridge whole: its heads' middles, the way from the first to the second, its deck's level
type wholeRow struct {
	From, To geom.Vec
	Dir      geom.Vec
	Deck     float32
}

func (r wholeRow) lie(wide float32) world.Lie {
	return world.Lie{Length: f32.Hypot(r.To.X-r.From.X, r.To.Y-r.From.Y), Width: wide}
}

// box bounds the deck's rectangle wide wide, no side under least: a smaller handle is refused
func (r wholeRow) box(wide, least float32) plane.AABB {
	mid := geom.NewVec((r.From.X+r.To.X)/2, (r.From.Y+r.To.Y)/2)
	c := r.lie(wide).Corners(mid, r.Dir)
	lo, hi := c[0], c[0]
	for _, p := range c[1:] {
		lo = geom.NewVec(min(lo.X, p.X), min(lo.Y, p.Y))
		hi = geom.NewVec(max(hi.X, p.X), max(hi.Y, p.Y))
	}
	size := geom.NewVec(max(hi.X-lo.X, least), max(hi.Y-lo.Y, least))
	return plane.NewAABB(geom.NewVec(mid.X-size.X/2, mid.Y-size.Y/2), size.X, size.Y)
}

// Types is the register of the kinds of bridges, defined on the Stage's kinds.
type Types struct {
	p     *Plugin
	specs map[string]Spec
	names []string
}

// Define registers the kind of bridge named name, as spec says, with the kinds of its parts,
// "<name>.deck" and "<name>.pier", and of its handle, "<name>.whole".
func (t *Types) Define(name string, spec Spec) {
	if spec.Width <= 0 || spec.Allows == 0 {
		panic(fmt.Sprintf("bridges: bridge %q: its deck needs a Width and a domain it carries",
			name))
	}
	if _, ok := t.specs[name]; ok {
		panic(fmt.Sprintf("bridges: bridge %q is defined twice", name))
	}
	w, brd := t.p.worldPlugin, t.p.board
	span := brd.Read(0).Board().Grid.CellSpan()
	kind.Define[Row](w.Kinds(), name, kind.Spec{
		comp.Load(func(r Row) world.Position {
			m := brd.Read(0).Board().CellCenter(r.From)
			return world.Position{AABB: plane.NewAABB(geom.NewVec(m.X-span/2, m.Y-span/2), span,
				span)}
		}),
		comp.Const(world.Velocity{}),
	})
	shape := board.Shape{Size: span, Height: spec.Thickness}
	decks := board.NewEntities(brd, shape, func(r deckRow) cell.ID { return r.At })
	deck := []comp.Comp{
		comp.Load(func(r deckRow) cell.Floor {
			f := cell.Floor{
				Allows:    spec.Allows,
				Height:    r.Height,
				Thickness: spec.Thickness,
				Load:      spec.Load,
				Pace:      spec.Pace,
				Graded:    !r.Winds,
				Links:     r.Links,
				Width:     spec.Width,
				Off:       r.Off,
				Lanes:     r.Lanes,
			}
			if r.Width > 0 {
				f.Width = r.Width
			}
			return f
		}),
		comp.Const(spec.Rail),
		comp.Load(func(r deckRow) world.Velocity { return world.Velocity{Dir: r.Dir} }),
		// the handle casts the deck's shadow whole
		comp.Const(light.Shadow{Cast: light.NoShadow}),
	}
	pier := kind.Spec{}
	if spec.Model != None {
		deck = append(deck, comp.Const(world.Unseen{}))
		pier = append(pier, comp.Const(world.Unseen{}))
	}
	decks.Define(deckName(name), deck...)
	half := spec.Pier / 2
	kind.Define[pierRow](w.Kinds(), pierName(name), append(pier,
		comp.Load(func(r pierRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.At.X-half, r.At.Y-half),
				spec.Pier, spec.Pier)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(r pierRow) world.Z {
			return world.Z{Altitude: r.Bottom, Height: max(r.Top-r.Bottom, 0)}
		}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: f32.Inf(1)}),
		comp.Const(light.Shadow{Cast: light.Baked}),
		comp.Const(world.Fixed{}),
	))
	least := float32(w.Res.Config.Entities.MinSize)
	kind.Define[wholeRow](w.Kinds(), wholeName(name), kind.Spec{
		comp.Load(func(r wholeRow) world.Position {
			return world.Position{AABB: r.box(spec.Width, least)}
		}),
		comp.Load(func(r wholeRow) world.Velocity { return world.Velocity{Dir: r.Dir} }),
		// as a deck lies: from its Altitude down its Height
		comp.Load(func(r wholeRow) world.Z {
			return world.Z{Altitude: r.Deck, Height: spec.Thickness}
		}),
		comp.Load(func(r wholeRow) world.Lie { return r.lie(spec.Width) }),
		comp.Const(light.Shadow{Cast: light.Baked}),
		// its pieces cut sight, cell by cell
		comp.Const(vision.Transparency{Value: 1}),
		comp.Const(world.Fixed{}),
		comp.Const(world.Whole{}),
	})
	if t.specs == nil {
		t.specs = map[string]Spec{}
	}
	t.specs[name] = spec
	t.names = append(t.names, name)
}

// Names are the kinds of bridges defined, in the order defined.
func (t *Types) Names() []string { return slices.Clone(t.names) }

// Named is the kind of bridge defined as name; an unknown name panics.
func (t *Types) Named(name string) Type {
	spec, ok := t.specs[name]
	if !ok {
		panic(fmt.Sprintf("bridges: no bridge is defined as %q", name))
	}
	w := t.p.worldPlugin
	return Type{
		name:  name,
		spec:  spec,
		deck:  kind.Named[deckRow](w.Kinds(), deckName(name)),
		pier:  kind.Named[pierRow](w.Kinds(), pierName(name)),
		whole: kind.Named[wholeRow](w.Kinds(), wholeName(name)),
	}
}

// Type is a kind of bridge defined.
type Type struct {
	name  string
	spec  Spec
	deck  kind.Of[deckRow]
	pier  kind.Of[pierRow]
	whole kind.Of[wholeRow]
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

func deckName(name string) string  { return name + ".deck" }
func pierName(name string) string  { return name + ".pier" }
func wholeName(name string) string { return name + ".whole" }
