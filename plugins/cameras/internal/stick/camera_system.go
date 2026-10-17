package stick

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/vec"
	"github.com/kjkrol/uid"
)

// Entities is what a Keeper reads and writes of the entities its cameras follow.
type Entities interface {
	// Of is the entity id as a camera follows it; false once it is gone.
	Of(id uid.UID64) (Target, bool)
	// Drive writes the eye's part of what drives the entity id, where it looks, whether it is
	// flown and how steeply it climbs; nothing for one driven by nothing.
	Drive(id uid.UID64, look geom.Vec, flown bool, climb float64)
	// Tilt tilts the eye of the entity id by tilt radians, down for a positive one.
	Tilt(id uid.UID64, tilt float64)
}

// Target is an entity as a camera follows it: X and Y its middle, Alt the height it stands at,
// Level its eye's, Dir the way it faces (of length 1, or none), Across how wide its eye sees in
// radians, Tilt how its eye is tilted (Eyed: it has one), Turning whether a hand turns it.
type Target struct {
	X, Y, Alt, Level float64
	Dir              geom.Vec
	Across           float32
	Tilt             float64
	Eyed, Turning    bool
}

// Keeper carries out the commands on the cameras its Control's Maker made and keeps every one
// fastened behind or inside its entity as the camera's fastening says (camera.Fastening).
type Keeper struct {
	orders      Orders
	free        func(x, y, z float32) bool // nil, everywhere
	perspective bool                       // Ride reaches Inside
	cams        *[]*viewCamera

	ents      Entities
	following []following
}

type following struct {
	cam    *viewCamera
	target uid.UID64
	inside bool
	// inside, while aiming: the way the eye looks, the entity turning to face it
	aim    geom.Vec
	aiming bool

	// behind, in perspective: how steeply the player wants to look down, and how steeply the
	// eye does
	wantPitch, appliedPitch float32
	tilts                   uint32
	// inside: the unit's own Eye.Tilt, given back on letting go
	tilt0  float64
	tilted bool
}

// a part of the screen's height a fastened camera holds its entity below the middle
const shoulder = 0.25

// about two thirds of any turn in this time
const followEase = 250 * time.Millisecond

// how much steeper a fastened eye looks at a time until the ground no longer hides its entity
const clearStep = 5 * math.Pi / 180

// Update carries out the commands given since and keeps the cameras over ents as d goes by.
func (s *Keeper) Update(d time.Duration, ents Entities) {
	s.ents = ents
	s.orders.Views(func(c camera.Camera) {
		if cam, ok := c.(*viewCamera); ok {
			if f := cam.Fastening(); f.How == camera.Inside { // out of the unit, over it still
				cam.Fasten(camera.Fastening{Entity: f.Entity, How: camera.Centred})
				return
			}
			cam.next()
		}
	})
	s.orders.Turns(func(c camera.Camera, angle float32) {
		// an eye inside turns with its unit
		if cam, ok := c.(*viewCamera); ok && !cam.insideUnit() {
			cam.Turn(angle)
		}
	})
	s.orders.Tilts(func(c camera.Camera, angle float32) {
		if cam, ok := c.(*viewCamera); ok {
			cam.Tilt(angle)
		}
	})
	s.orders.LookFroms(func(c camera.Camera, x, y, z float32) {
		if cam, ok := c.(*viewCamera); ok {
			s.comeOut(cam)
			cam.LookFrom(x, y, z)
		}
	})
	s.orders.LookAts(func(c camera.Camera, x, y, z float32) {
		if cam, ok := c.(*viewCamera); ok {
			s.comeOut(cam)
			cam.LookAt(x, y, z)
		}
	})
	s.orders.Rides(func(c camera.Camera) {
		if cam, ok := c.(*viewCamera); ok {
			s.ride(cam)
		}
	})
	s.orders.Looks(func(c camera.Camera, dx, dy float32) {
		for k := range s.following {
			if f := &s.following[k]; f.cam == c && f.inside {
				s.look(f, dx, dy)
			}
		}
	})
	s.reconcile()
	kept := s.following[:0]
	var gone []*viewCamera
	for _, f := range s.following {
		if s.keep(&f, d) {
			kept = append(kept, f)
		} else {
			gone = append(gone, f.cam)
		}
	}
	s.following = kept
	for _, cam := range gone {
		cam.leaveInside()
		cam.Fasten(camera.Fastening{})
	}
}

func (s *Keeper) ride(cam *viewCamera) {
	f := cam.Fastening()
	switch f.How {
	case camera.Centred:
		f.How = camera.Behind
	case camera.Behind:
		f.How = camera.Centred
		if s.perspective {
			f.How = camera.Inside
		}
	case camera.Inside:
		f.How = camera.Centred
	default:
		return
	}
	cam.Fasten(f)
}

func (s *Keeper) reconcile() {
	if s.cams == nil {
		return
	}
	for _, cam := range *s.cams {
		f := cam.Fastening()
		want := f.How == camera.Behind || f.How == camera.Inside
		if e := s.fastened(cam); e != nil && (!want || e.target != f.Entity || e.inside != (f.How == camera.Inside)) {
			s.letGo(cam)
		}
		if want && s.fastened(cam) == nil {
			s.fasten(cam, f.Entity, f.How == camera.Inside)
		}
	}
}

func (s *Keeper) fastened(cam *viewCamera) *following {
	for i := range s.following {
		if s.following[i].cam == cam {
			return &s.following[i]
		}
	}
	return nil
}

func (s *Keeper) fasten(cam *viewCamera, id uid.UID64, inside bool) {
	if _, ok := s.ents.Of(id); !ok {
		cam.Fasten(camera.Fastening{})
		return
	}
	if inside && !s.enter(cam, id) {
		cam.Fasten(camera.Fastening{Entity: id, How: camera.Centred})
		return
	}
	s.following = append(s.following, following{cam: cam, target: id, inside: inside})
	s.keep(&s.following[len(s.following)-1], 0)
}

func (s *Keeper) letGo(cam *viewCamera) bool {
	for i, f := range s.following {
		if f.cam == cam {
			s.write(f.target, geom.Vec{}, false, 0)
			s.untilt(&s.following[i])
			s.following = append(s.following[:i], s.following[i+1:]...)
			if f.inside {
				cam.leaveInside()
			}
			return true
		}
	}
	return false
}

func (s *Keeper) comeOut(cam *viewCamera) {
	if f := s.fastened(cam); f != nil && f.inside {
		s.letGo(cam)
		cam.Fasten(camera.Fastening{})
	}
}

// the eye's part of what drives it alone; the hand's part is whoever drives it
func (s *Keeper) write(id uid.UID64, look geom.Vec, flown bool, climb float64) {
	s.ents.Drive(id, look, flown, climb)
}

// the eye looks up or down as it did before the rider took it
func (s *Keeper) untilt(f *following) {
	if !f.tilted {
		return
	}
	if _, ok := s.ents.Of(f.target); ok {
		s.ents.Tilt(f.target, f.tilt0)
	}
	f.tilted = false
}

func (s *Keeper) look(f *following, dx, dy float32) {
	if dy != 0 {
		f.cam.Tilt(dy * LookStep)
	}
	if dx == 0 {
		return
	}
	from := f.aim
	if !f.aiming {
		from = facing(f.cam.Heading())
	}
	sn, cs := math.Sincos(float64(dx * LookStep))
	f.aim, f.aiming = geom.NewVec(from.X*cs-from.Y*sn, from.X*sn+from.Y*cs), true
	f.cam.Turn(behind(float32(f.aim.X), float32(f.aim.Y)) - f.cam.Heading())
}

// the inverse of behind
func facing(heading float32) geom.Vec {
	s, c := math.Sincos(float64(heading) + math.Pi/4)
	return geom.NewVec(-s, -c)
}

func (s *Keeper) keep(f *following, d time.Duration) bool {
	t, ok := s.ents.Of(f.target)
	if !ok {
		return false
	}
	cx, cy, dir := t.X, t.Y, t.Dir
	faces := dir.X != 0 || dir.Y != 0
	if f.inside && f.aiming {
		// the keys turn it: the look goes with it again
		if t.Turning {
			f.aiming = false
		} else if faces && math.Abs(math.Atan2(dir.X*f.aim.Y-dir.Y*f.aim.X,
			dir.X*f.aim.X+dir.Y*f.aim.Y)) < 1e-3 {
			f.aiming = false // it faces the way the eye looks: pinned to it again
		}
	}
	if !f.inside {
		if faces {
			by := ease(behind(float32(dir.X), float32(dir.Y))-f.cam.Heading(), d)
			if math.Abs(float64(by)) > 1e-4 {
				f.cam.Turn(by)
			}
		}
		f.cam.CenterOn(cx, cy, t.Alt)
		if f.cam.inPersp { // before it looks over its shoulder
			s.clear(f, [3]float32{float32(cx), float32(cy), float32(t.Alt)}, d)
		}
		// over its shoulder: the lower the eye, the lower the entity on the screen
		_, h := f.cam.Viewport()
		f.cam.shift(0, -h*shoulder*float32(math.Cos(float64(f.cam.Pitch()))))
		return true
	}
	// flown from inside: the way it is steered rises as the rider looks up
	var look geom.Vec
	if f.aiming {
		look = f.aim
	}
	s.ents.Drive(f.target, look, true, -math.Sin(float64(f.cam.Pitch())))
	// the rider's head is the unit's eye: it looks up and down as the rider does
	if t.Eyed {
		if !f.tilted {
			f.tilt0, f.tilted = t.Tilt, true
		}
		s.ents.Tilt(f.target, -float64(f.cam.Pitch()))
	}
	way, looks := dir, faces
	if f.aiming {
		way, looks = f.aim, true
	}
	if looks {
		by := ease(behind(float32(way.X), float32(way.Y))-f.cam.Heading(), 0)
		if math.Abs(float64(by)) > 1e-5 {
			f.cam.Turn(by)
		}
	}
	f.cam.CenterOn(cx, cy, s.riding(cx, cy, t.Level, f.cam.persp.cell))
	return true
}

// looks down steeply enough to see the entity at once, eases back out as the way clears
func (s *Keeper) clear(f *following, t [3]float32, d time.Duration) {
	p := f.cam.persp
	if f.appliedPitch == 0 || p.tilts != f.tilts {
		f.wantPitch, f.tilts = p.pitch, p.tilts
	}
	free := f.wantPitch
	if s.free != nil {
		free = s.clearPitch(p, t, f.wantPitch)
	}
	if f.appliedPitch > 0 && free < f.appliedPitch {
		free = f.appliedPitch + (free-f.appliedPitch)*eased(d)
	}
	if free != p.pitch {
		p.pitch = free
		p.CenterOn(float64(t[0]), float64(t[1]), float64(t[2]))
	}
	f.appliedPitch = p.pitch
}

func (s *Keeper) clearPitch(p *perspCamera, t [3]float32, want float32) float32 {
	sn, cs := math.Sincos(float64(p.heading) + math.Pi/4)
	for pitch := want; pitch < maxPitch; pitch += clearStep {
		r := float64(p.alt-t[2]) / math.Tan(float64(pitch))
		e := [3]float32{t[0] + float32(sn*r), t[1] + float32(cs*r), p.alt}
		if s.lineClear(t, e, p.cell) {
			return pitch
		}
	}
	return maxPitch
}

func (s *Keeper) lineClear(t, e [3]float32, cell float32) bool {
	d := vec.Sub(e, t)
	h := float32(math.Hypot(float64(d[0]), float64(d[1])))
	step := cell / 2
	for along := cell; along < h; along += step {
		at := vec.Add(t, vec.Scale(d, along/h))
		if !s.free(at[0], at[1], at[2]-cell/4) {
			return false
		}
	}
	return true
}

func (s *Keeper) enter(cam *viewCamera, id uid.UID64) bool {
	eye, across, dir, ok := s.eyeOf(id)
	if !ok {
		return false
	}
	heading := cam.Heading()
	if dir.X != 0 || dir.Y != 0 {
		heading = behind(float32(dir.X), float32(dir.Y))
	}
	eye[2] = float32(s.riding(float64(eye[0]), float64(eye[1]), float64(eye[2]), cam.persp.cell))
	return cam.enterInside(eye, heading, across)
}

// across is radians, 0 without an Eye; dir is of length 1 or none
func (s *Keeper) eyeOf(id uid.UID64) (eye [3]float32, across float32, dir geom.Vec, ok bool) {
	t, ok := s.ents.Of(id)
	if !ok {
		return eye, across, dir, false
	}
	return [3]float32{float32(t.X), float32(t.Y), float32(t.Level)}, t.Across, t.Dir, true
}

// never in what is solid there, nor within a hair of it
func (s *Keeper) riding(x, y, top float64, cell float32) float64 {
	if s.free == nil || s.free(float32(x), float32(y), float32(top)) {
		return top
	}
	return float64(rise(s.free, float32(x), float32(y), float32(top), cell)) + float64(cell)/256
}

// the heading from which the way (dx, dy) runs up the screen
func behind(dx, dy float32) float32 {
	return float32(math.Atan2(float64(-dx), float64(-dy))) - math.Pi/4
}

// the shorter way round; all of it for d zero, the first centring
func ease(by float32, d time.Duration) float32 {
	return float32(math.Remainder(float64(by), 2*math.Pi)) * eased(d)
}

func eased(d time.Duration) float32 {
	if d <= 0 {
		return 1
	}
	return float32(1 - math.Exp(-float64(d)/float64(followEase)))
}
