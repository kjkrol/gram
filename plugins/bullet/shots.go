package bullet

import (
	"fmt"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Body is what a kind of shot is, a knob on every shot (an effect may Alter it): its Size (a square
// box), how fast it flies, how far, whether it is thrown (Gravity > 0: an arc that comes down where
// it was aimed, or at Range) and whether it Lands — lies where its flight ends, for a rule to fuse
// — or is spent, gone as it lands. A shot that is to Burst must Land.
type Body struct {
	Size, Speed, Range, Gravity float64
	Lands                       bool
}

// Shot is the row a kind of shot spawns from: the plugin builds it as it fires.
type Shot struct {
	From, Dir geom.Vec
	Altitude  float64 // the muzzle's: the shooter's Eye.Level, else the middle of its Z; 0 flat
	Climb     float64 // the vertical speed of a thrown one
	Range     float64 // this shot's, no further than its Body's
	Shooter   uid.UID64
	Owners    tag.Tags[owner.Family] // the shooter's, loaded on the shot: rules tell friendly fire
}

// Flight is a shot in the air, the plugin's own state beside its Body: where its centre is (At,
// the real one, its box there), which way and how far it goes, how far it has gone, the vertical
// speed of a thrown one, who shot it, and how its flight ends (Ending: noted as the last step is
// flown, settled a step later once collision has looked at that step — Struck, with Other, or
// Wall, with Cell, where it found a contact) and whether it lies Landed.
type Flight struct {
	At, Dir             geom.Vec
	Range, Flown, Climb float64
	Shooter             uid.UID64
	Ending              End
	Other               uid.UID64
	Cell                uint64
	Landed              bool
}

// Heading is which of n ways round the circle the shot flies, counted from east against the
// clock: the index of a directional twin, drawn with render.Arrow(float64(i)*360/n, …).
func (f Flight) Heading(n int) int {
	a := math.Atan2(-f.Dir.Y, f.Dir.X) // the screen's y grows down
	i := int(math.Round(a / (2 * math.Pi) * float64(n)))
	return ((i % n) + n) % n
}

// End is how a flight ends: not yet (Flying); its Range flown (Spent); a thrown shot come down
// (Grounded); stopped at a closed edge (Edge); flown out by an open one (Left); on an entity
// (Struck); on the solid ground (Wall).
type End uint8

const (
	Flying End = iota
	Spent
	Grounded
	Edge
	Left
	Struck
	Wall
)

// Shots defines kinds of shots on a world's kinds: NewShots(w).Define(name, body, extra...) is an
// Ammo, for Shoot. A shot is an entity of the world like any other, drawn from its kind's sprite:
// its box, a Collider (a sensor: only ever detected), a collision.Sweep that passes through its
// shooter, its Body and Flight, its shooter's owners, and in a world with heights a Z. extra may
// carry tags and Layers, never the owners' family nor what the kind gives itself.
type Shots struct {
	w      *world.Plugin
	bodies map[string]Body              // what each kind of shot flies as, by its name
	facing map[string][]render.SpriteID // the directional twins Facing declared, by the ammo's name
}

// NewShots defines kinds of shots on w's kinds.
func NewShots(w *world.Plugin) *Shots { return &Shots{w: w} }

// Define registers the kind of shot named name with body and extra; Named is the Ammo it is, for
// Shoot.
func (s *Shots) Define(name string, body Body, extra ...comp.Comp) {
	if body.Size <= 0 || body.Speed <= 0 || body.Range <= 0 {
		panic(fmt.Sprintf("bullet: ammo %q: Size, Speed and Range must be positive, got %+v", name, body))
	}
	half := body.Size / 2
	spec := kind.Spec{
		comp.Load(func(r Shot) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.From.X-half, r.From.Y-half), body.Size, body.Size)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(body),
		comp.Const(collision.Collider{}),
		comp.Load(func(r Shot) collision.Sweep { return collision.Sweep{From: r.From, Ignore: r.Shooter, Ignoring: true} }),
		comp.Load(func(r Shot) Flight {
			return Flight{At: r.From, Dir: r.Dir, Range: r.Range, Climb: r.Climb, Shooter: r.Shooter}
		}),
		comp.Load(func(r Shot) tag.Tags[owner.Family] { return r.Owners }),
	}
	if s.w.HasHeights() {
		spec = append(spec, comp.Load(func(r Shot) world.Z { return world.Z{Altitude: r.Altitude, Height: body.Size} }))
	}
	spec = append(spec, extra...)
	kind.Define[Shot](s.w.Kinds(), name, spec)
	if s.bodies == nil {
		s.bodies = map[string]Body{}
	}
	s.bodies[name] = body
}

// Named is the kind of shot defined as name, as Shoot names it; an unknown name panics.
func (s *Shots) Named(name string) Ammo {
	body, ok := s.bodies[name]
	if !ok {
		panic(fmt.Sprintf("bullet: no ammo is defined as %q", name))
	}
	return Ammo{kind: kind.Named[Shot](s.w.Kinds(), name), body: body, headings: s.facing[name]}
}

// Facing declares that the shots named name are drawn the way they fly, one of n twins round the
// circle: it issues the twins' slots and hands back the rule for world.Plugin.Draw — the Looks
// section's one step. Their drawers join the atlas on the ammo's Slot (render.Slot.Facing).
func (s *Shots) Facing(name string, n int) render.Rule {
	if n <= 0 {
		panic(fmt.Sprintf("bullet: ammo %q faces %d ways", name, n))
	}
	base := s.Named(name).SpriteID()
	twins := make([]render.SpriteID, n)
	for i := range twins {
		twins[i] = s.w.Kinds().NewSprite()
	}
	if s.facing == nil {
		s.facing = map[string][]render.SpriteID{}
	}
	s.facing[name] = twins
	return render.With(func(a render.Appearance, f Flight) render.Appearance {
		if a.SpriteID == base {
			a.SpriteID = twins[f.Heading(n)]
		}
		return a
	})
}

// Ammo is a kind of shot, as Shoot names it.
type Ammo struct {
	kind     kind.Of[Shot]
	body     Body
	headings []render.SpriteID
}

// ID is what every shot of this kind carries to say so.
func (a Ammo) ID() kind.ID { return a.kind.ID() }

// SpriteID is the atlas slot the shots of this kind are drawn from.
func (a Ammo) SpriteID() render.SpriteID { return a.kind.SpriteID() }

// Body is what this kind of shot is.
func (a Ammo) Body() Body { return a.body }

// Headings are the directional twins Facing declared for this ammo, east first, against the
// clock; nil without a Facing. An Ammo with them is a render.Faced: its Slot's Facing draws them.
func (a Ammo) Headings() []render.SpriteID { return a.headings }

// Entry is one shot of this kind, for the world to spawn.
func (a Ammo) Entry(shot Shot) kind.Entry { return a.kind.Entry(shot) }
