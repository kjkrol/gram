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
	MaxSightRadius = 600
	// MaxHalfAngleMilli is the widest half-angle the sizing assumes, in milliradians (pi/6).
	MaxHalfAngleMilli = 524
	// EdgeTolerance is how far a shadow edge may land from its true angle at full range.
	EdgeTolerance = 5

	// MaxSamples keeps a cone within the three limits above accurate to EdgeTolerance.
	arcMilli   = 2 * MaxHalfAngleMilli * MaxSightRadius
	gapMilli   = 1000 * EdgeTolerance
	MaxSamples = (arcMilli+gapMilli-1)/gapMilli + 1
)

// Sight is what an entity can take in — where it looks and how far — a knob vision only reads and
// an effect may Alter. Facing is its own, whichever way the entity moves, unless Ahead; how wide it
// sees and from how high is its world.Eye, which the scan needs beside it. What the scan found is
// the entity's Sighted.
type Sight struct {
	Facing geom.Vec // unit vector
	Radius float64  // world units
	// Blockers are the world.Layers whose entities cut or dim this sight; one on none of them is
	// looked over — a walker under a hawk — and still seen. Zero: every entity does. A flat world's;
	// a world with heights refuses it.
	Blockers world.Layers
	// Ahead has the sight look the way the entity moves, Facing only while it has no heading.
	Ahead bool
}

// Looking is where the sight looks for an entity heading dir: dir when Ahead and there is one,
// Facing otherwise.
func (s Sight) Looking(dir geom.Vec) geom.Vec {
	if s.Ahead && (dir.X != 0 || dir.Y != 0) {
		return dir
	}
	return s.Facing
}

// Transparency is how see-through an entity is to a Sight: 0 cuts sight as an entity without it
// does, 1 is as if absent, between them a ray through it spends 1/Value of its reach per unit.
type Transparency struct{ Value float64 }

// Sighted is what an observer's last scan found, nearest first, beside its Sight: vision writes
// it, and gives one to an observer that has none. Count says how many of the arrays are in use.
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
