package underground

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

// Spec is what a kind of structure under the ground is: whom its floor carries (Allows, at Pace;
// 0 the ground's) and how heavy they may be (Load, kg; 0 any), how wide a tunnel of it runs and
// how much room lies over its floor, up to its roof.
type Spec struct {
	Allows cell.Domain
	Load   float32
	Width  float32
	Clear  float32
	Pace   float64
}

// Row is a tunnel as a level's file writes it: from the head From to the head To, turning at the
// cells Via.
type Row struct {
	From cell.ID   `yaml:"from"`
	To   cell.ID   `yaml:"to"`
	Via  []cell.ID `yaml:"via"`
	// Lanes are the lanes of the road through it: one marked with them is a road's, drawn
	// on through its mouths as the roads over the ground are.
	Lanes uint8 `yaml:"lanes"`
}

// RoomRow is a chamber as a level's file writes it: under the rectangle of cells from From to
// To, its floor Depth under the lowest ground over it.
type RoomRow struct {
	From  cell.ID `yaml:"from"`
	To    cell.ID `yaml:"to"`
	Depth float32 `yaml:"depth"`
}

// a piece of a structure under a cell: its floor's height over the cell's ground's level, a
// negative one, the storey it takes and, of a tunnel, the ways it runs on
type boreRow struct {
	At     cell.ID
	Links  cell.Links
	Height float32
	Storey uint8
	Wide   bool
	Lanes  uint8
}

// a structure whole: the box over its cells, from its corner At, and its lowest floor
type wholeRow struct {
	At, Size geom.Vec
	Floor    float32
}

// Types is the register of the kinds of structures under the ground, defined on the Stage's
// kinds.
type Types struct {
	p     *Plugin
	specs map[string]Spec
	names []string
	rooms map[string]bool
}

// Define registers the kind of tunnel named name, as spec says, with the kinds of its parts,
// "<name>.bore", and of its handle, "<name>.whole".
func (t *Types) Define(name string, spec Spec) {
	if spec.Width <= 0 {
		panic(fmt.Sprintf("underground: tunnel %q: it needs a Width", name))
	}
	t.define(name, spec)
	brd := t.p.board
	span := brd.Read(0).Board().Grid.CellSpan()
	kind.Define[Row](t.p.worldPlugin.Kinds(), name, kind.Spec{
		comp.Load(func(r Row) world.Position {
			m := brd.Read(0).Board().CellCenter(r.From)
			return world.Position{AABB: plane.NewAABB(geom.NewVec(m.X-span/2, m.Y-span/2), span,
				span)}
		}),
		comp.Const(world.Velocity{}),
	})
}

// Chamber registers the kind of chamber named name, as spec says, its Width left out, with the
// kinds of its parts, "<name>.bore", and of its handle, "<name>.whole".
func (t *Types) Chamber(name string, spec Spec) {
	spec.Width = 0
	t.define(name, spec)
	if t.rooms == nil {
		t.rooms = map[string]bool{}
	}
	t.rooms[name] = true
	brd := t.p.board
	span := brd.Read(0).Board().Grid.CellSpan()
	kind.Define[RoomRow](t.p.worldPlugin.Kinds(), name, kind.Spec{
		comp.Load(func(r RoomRow) world.Position {
			m := brd.Read(0).Board().CellCenter(r.From)
			return world.Position{AABB: plane.NewAABB(geom.NewVec(m.X-span/2, m.Y-span/2), span,
				span)}
		}),
		comp.Const(world.Velocity{}),
	})
}

func (t *Types) define(name string, spec Spec) {
	switch {
	case spec.Allows == 0 || spec.Clear <= 0:
		panic(fmt.Sprintf("underground: %q: it needs a domain its floor carries and the room "+
			"over it, Clear", name))
	case t.specs[name] != (Spec{}):
		panic(fmt.Sprintf("underground: %q is defined twice", name))
	}
	w, brd := t.p.worldPlugin, t.p.board
	span := brd.Read(0).Board().Grid.CellSpan()
	bores := board.NewEntities(brd, board.Shape{Size: span, Height: spec.Clear},
		func(r boreRow) cell.ID { return r.At })
	bores.Define(boreName(name),
		comp.Load(func(r boreRow) cell.Floor {
			f := cell.Floor{
				Allows: spec.Allows,
				Height: r.Height,
				Clear:  spec.Clear,
				Load:   spec.Load,
				Pace:   spec.Pace,
				Graded: true,
				Storey: r.Storey,
			}
			if !r.Wide {
				f.Links, f.Width, f.Lanes = r.Links, spec.Width, r.Lanes
			}
			return f
		}),
		comp.Const(light.Shadow{Cast: light.NoShadow}),
		// what lies under the ground cuts no sight over it
		comp.Const(vision.Transparency{Value: 1}),
		// the plugin draws it from inside
		comp.Const(world.Unseen{}),
	)
	kind.Define[wholeRow](w.Kinds(), wholeName(name), kind.Spec{
		comp.Load(func(r wholeRow) world.Position {
			return world.Position{AABB: plane.NewAABB(r.At, r.Size.X, r.Size.Y)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(r wholeRow) world.Z {
			return world.Z{Altitude: r.Floor, Height: spec.Clear}
		}),
		comp.Const(light.Shadow{Cast: light.NoShadow}),
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

// Named is the kind of structure defined as name; an unknown name panics.
func (t *Types) Named(name string) Type {
	spec, ok := t.specs[name]
	if !ok {
		panic(fmt.Sprintf("underground: no structure is defined as %q", name))
	}
	w := t.p.worldPlugin
	return Type{
		name:  name,
		spec:  spec,
		room:  t.rooms[name],
		bore:  kind.Named[boreRow](w.Kinds(), boreName(name)),
		whole: kind.Named[wholeRow](w.Kinds(), wholeName(name)),
	}
}

// Type is a kind of structure under the ground defined.
type Type struct {
	name  string
	spec  Spec
	room  bool
	bore  kind.Of[boreRow]
	whole kind.Of[wholeRow]
}

// Name is what the kind of structure is called.
func (t Type) Name() string { return t.name }

// Spec is what the kind of structure is.
func (t Type) Spec() Spec { return t.spec }

func boreName(name string) string  { return name + ".bore" }
func wholeName(name string) string { return name + ".whole" }
