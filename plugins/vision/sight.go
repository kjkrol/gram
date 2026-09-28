package vision

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// MaxSeen caps how many entities one scan records, nearest first.
const MaxSeen = 8

// Sizing of the outline buffer: the first three decide MaxSamples.
const (
	// MaxSightRadius is the longest range the outline is sized for; a longer Radius still sees,
	// its outline is only coarser.
	MaxSightRadius = 300
	// MaxHalfAngleMilli is the widest half-angle the sizing assumes, in milliradians (pi/6).
	MaxHalfAngleMilli = 524
	// EdgeTolerance is how far a shadow edge may land from its true angle at full range.
	EdgeTolerance = 5

	// MaxSamples keeps a cone within the three limits above accurate to EdgeTolerance.
	arcMilli   = 2 * MaxHalfAngleMilli * MaxSightRadius
	gapMilli   = 1000 * EdgeTolerance
	MaxSamples = (arcMilli+gapMilli-1)/gapMilli + 1
)

// Sight is what an entity can take in — where it looks, how wide, how far — and
// what the last scan found there. Facing is its own, whichever way the entity moves.
type Sight struct {
	Facing    geom.Vec // unit vector
	HalfAngle float64  // radians either side of Facing
	Radius    float64  // world units
	// Blockers are the world.Layers whose entities cut or dim this sight; one on none of them is
	// looked over — a walker under a hawk — and still seen. Zero: every entity does. A flat world's;
	// a world with heights refuses it.
	Blockers world.Layers
	// Eye is how high above the entity's bottom (its Z.Altitude) it looks from, in a world with heights;
	// a flat world refuses it.
	Eye  float64
	Seen Sighted // nearest first
}

// Transparency is how see-through an entity is to a Sight: 0 cuts sight as an entity without it
// does, 1 is as if absent, between them a ray through it spends 1/Value of its reach per unit.
type Transparency struct{ Value float64 }

// Sighted is what one scan found, nearest first. Count says how many of the
// arrays are in use.
type Sighted struct {
	IDs   [MaxSeen]uid.UID64
	Dists [MaxSeen]float32
	Count uint8
}

// MaxShadowsPerSample caps the stretches of hidden ground kept per angle of an outline; the nearest
// are kept.
const MaxShadowsPerSample = 2

// Band is a stretch along one angle of a view, From to To away from the observer; zero is none.
type Band struct{ From, To float32 }

// SightOutline is the drawn shape of one entity's view: a reach per evenly spaced angle across the
// cone and, in a world with heights, the stretches of ground out of sight along each — the holes in a
// view that reaches its full Radius. Only an entity carrying it has its outline computed.
type SightOutline struct {
	Depths  [MaxSamples]float32
	Shadows [MaxSamples][MaxShadowsPerSample]Band
	Count   uint8
}
