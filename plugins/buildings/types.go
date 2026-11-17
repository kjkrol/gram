package buildings

import (
	"fmt"
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
)

// Roof is the shape of the roof over a part's walls.
type Roof uint8

const (
	// Flat is a level roof.
	Flat Roof = iota
	// Gable is two slopes meeting at a ridge along the part, a wall's gable at each end.
	Gable
	// Hip is four slopes, the ridge shorter than the part by its width.
	Hip
	// Pyramid is four slopes meeting at a point.
	Pyramid
)

// Part is a solid of a building: walls on a rectangle and a roof over them. Zero, it is the
// building's own: its rectangle, its walls as high as its storeys, a flat roof.
type Part struct {
	Roof Roof
	// Pitch is how high the roof rises, a share of half the part's width.
	Pitch float32
	// Size is the part's length and width, world units; zero, the building's own.
	Size geom.Vec
	// Off is where the part's middle lies off the building's, along it and across, shares of
	// half its length and half its width.
	Off geom.Vec
	// Height is how high the part's walls stand, world units; zero, the building's storeys and
	// Above over them.
	Height float32
	Above  float32
	// Walls and Tiles colour the part; zero, the Spec's.
	Walls color.RGBA
	Tiles color.RGBA
	// Blind leaves the part's walls without windows.
	Blind bool
	// Fine has the part drawn at the close and standard scales alone.
	Fine bool
}

// Spec is a kind of building: its parts, the first its body, and what a building of it is where
// the map says nothing.
type Spec struct {
	// Storey is how high a storey is, world units.
	Storey float32
	// Floors and Size are a building's storeys and its length and width, world units, where its
	// line of the map gives none.
	Floors int
	Size   geom.Vec
	Walls  color.RGBA
	Tiles  color.RGBA
	// Bay is how wide a window's bay is along a wall, world units; zero, no windows.
	Bay   float32
	Parts []Part
	// Roles are the roles a building of the kind plays beside BuildingRole.
	Roles []string
}

// Row is a building of a level's file: where its middle stands and its length and width, world
// units, how it is turned, degrees from the east towards the south, and its storeys.
type Row struct {
	Mid    [2]float32 `yaml:"mid"`
	Size   [2]float32 `yaml:"size"`
	Turn   float32    `yaml:"turn"`
	Floors int        `yaml:"floors"`
}

// Types is the register of the kinds of buildings, defined on the Stage's kinds.
type Types struct {
	p     *Plugin
	specs map[string]Spec
	names []string
}

// Define registers the kind of building named name, as spec says: a part of its settlement
// standing for good, drawn by the plugin, playing BuildingRole and spec's Roles.
func (t *Types) Define(name string, spec Spec) {
	if spec.Storey <= 0 {
		panic(fmt.Sprintf("buildings: %q needs a Storey", name))
	}
	if _, ok := t.specs[name]; ok {
		panic(fmt.Sprintf("buildings: %q is defined twice", name))
	}
	if len(spec.Parts) == 0 {
		spec.Parts = []Part{{}}
	}
	w := t.p.worldPlugin
	roles := w.Roles()
	plays := []*ceremony.Part{roles.Named(BuildingRole)}
	for _, r := range spec.Roles {
		plays = append(plays, roles.Named(r))
	}
	kind.Define[Row](w.Kinds(), name, w.Roster().Spec(board.StandingTemplate,
		// a part: SpawnPart lays its box off the middle of its settlement's sector
		comp.Load(func(r Row) world.Position {
			b := spec.of(r)
			hx, hy := b.reach()
			return world.Position{AABB: plane.NewAABB(geom.NewVec(-hx, -hy), 2*hx, 2*hy)}
		}),
		comp.Load(func(r Row) world.Velocity { return world.Velocity{Dir: spec.of(r).dir} }),
		comp.Load(func(r Row) world.Z { return world.Z{Height: spec.of(r).shade(spec)} }),
		comp.Load(func(r Row) world.Stand {
			b := spec.of(r)
			return world.Stand{Length: b.size.X, Width: b.size.Y}
		}),
		comp.Const(world.Unseen{}),
		ceremony.Plays(plays...)))
	if t.specs == nil {
		t.specs = map[string]Spec{}
	}
	t.specs[name] = spec
	t.names = append(t.names, name)
}

// Named is the kind of building defined as name; an unknown name panics.
func (t *Types) Named(name string) Spec {
	spec, ok := t.specs[name]
	if !ok {
		panic(fmt.Sprintf("buildings: no building is defined as %q", name))
	}
	return spec
}

// a building as it stands: its middle, its length and width, the way its length runs and how
// high its walls are
type standing struct {
	mid   geom.Vec
	size  geom.Vec
	dir   geom.Vec
	walls float32
}

func (s Spec) of(r Row) standing {
	b := standing{
		mid:  geom.NewVec(r.Mid[0], r.Mid[1]),
		size: geom.NewVec(r.Size[0], r.Size[1]),
	}
	if b.size.X <= 0 || b.size.Y <= 0 {
		b.size = s.Size
	}
	sin, cos := f32.Sincos(r.Turn * math.Pi / 180)
	b.dir = geom.NewVec(cos, sin)
	floors := r.Floors
	if floors <= 0 {
		floors = max(s.Floors, 1)
	}
	b.walls = float32(floors) * s.Storey
	return b
}

// reach is half the sides of the box round the building as it is turned
func (b standing) reach() (hx, hy float32) {
	ux, uy := f32.Abs(b.dir.X), f32.Abs(b.dir.Y)
	return (ux*b.size.X + uy*b.size.Y) / 2, (uy*b.size.X + ux*b.size.Y) / 2
}

// shade is how high the building's shadow is cast from: its body's walls and half its roof
func (b standing) shade(s Spec) float32 {
	body := s.Parts[0]
	return b.walls + body.Pitch*b.size.Y/4
}
