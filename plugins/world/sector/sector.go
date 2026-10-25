package sector

// Band is a band of pace round the anchors: the sectors within Reach world units of the nearest
// anchor and not of a nearer band, their systems going a step every Pace of the world's (the
// first band's every one).
type Band struct {
	Reach float32
	Pace  int
}

// MaxBands is how many bands a world may have.
const MaxBands = 5

// Policy is where in the bands a system goes and how often, said by the plugin owning it;
// beyond the last band none goes.
type Policy uint8

const (
	// Everywhere goes every step in every band.
	Everywhere Policy = iota
	// Paced goes every step in the first band and a step every Pace of the world's in each
	// farther one, over as many steps at once.
	Paced
	// Near goes in the first band alone.
	Near
)

// The marks of what lies in the bands past the first and beyond the last: zero-size components a
// system's query includes or excludes. What lies in the first band carries none.
type (
	Band2 struct{}
	Band3 struct{}
	Band4 struct{}
	Band5 struct{}
	Off   struct{}
)

// Anchors are the points the world lives round, each handed to at: the nearer a sector, the
// fuller its band.
type Anchors interface {
	Anchors(at func(x, y float32))
}
