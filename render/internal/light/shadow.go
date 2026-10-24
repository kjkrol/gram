package light

// Casting is how a thing casts its shadow on the ground.
type Casting uint8

const (
	// Patch casts a soft patch every frame, away from the light: what moves.
	Patch Casting = iota
	// Silhouette casts the thing's own drawn shape every frame, away from the light: a figure
	// worth the cost.
	Silhouette
	// Baked has the ground bake the shadow of the thing's solid with its own: what stands still.
	Baked
	// NoShadow casts none.
	NoShadow
)

// Shadowing is how a thing casts its shadow and which shade it takes; the zero one casts a Patch
// and takes all.
type Shadowing struct {
	Cast Casting
	Take Taking
}

// ShadowPolicy is how a thing casts its live shadow at distance world units from the eye (0
// through a camera without one), declared as it says: Patch, Silhouette or NoShadow.
type ShadowPolicy func(declared Casting, distance float32) Casting
