package stick

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/vec"
	"github.com/kjkrol/uid"
)

// Entities is what a Keeper reads and writes of the entities its cameras follow.
type Entities interface {
	// Of is the entity id as a camera follows it; false once it is gone.
	Of(id uid.UID64) (Target, bool)
	// Drive writes the eye's part of what drives the entity id, where it looks, whether it is
	// flown and how steeply it climbs; nothing for one driven by nothing.
	Drive(id uid.UID64, look geom.Vec, flown bool, climb float32)
	// Tilt tilts the eye of the entity id by tilt radians, down for a positive one.
	Tilt(id uid.UID64, tilt float32)
}

// Target is an entity as a camera follows it: X and Y its middle, Alt the height it stands at,
// Level its eye's, Dir the way it faces (of length 1, or none), Moving whether it goes that way,
// Across how wide its eye sees in radians, Tilt how its eye is tilted (Eyed: it has one), Turning
// whether a hand turns it.
type Target struct {
	X, Y, Alt, Level float32
	Dir              geom.Vec
	Moving           bool
	Across           float32
	Tilt             float32
	Eyed, Turning    bool
}

// Arm is how a camera is held behind an entity, as the plugin's settings say: Distance cells
// long, tilted Pitch radians over the ground, aimed Aim of the entity's height over its feet;
// Tilts, whether the player's mouse tilts it.
type Arm struct {
	Pitch, Distance, Aim float32
	Tilts                bool
}

// Arms is where a Keeper reads the arm from and writes the tilt the mouse gives it: the
// plugin's settings, which keep it within their bounds.
type Arms interface {
	Arm() Arm
	Tilt(pitch float32)
}

// Keeper carries out the commands on the cameras its Control's Maker made and keeps every one
// fastened behind or inside its entity as the camera's fastening says (camera.Fastening).
type Keeper struct {
	orders      Orders
	arms        Arms
	free        func(x, y, z float32) bool // nil, everywhere
	perspective bool                       // Ride reaches Behind and Inside
	cams        *[]*viewCamera

	ents      Entities
	following []following
}

type following struct {
	cam    *viewCamera
	target uid.UID64
	how    camera.How
	// inside, while aiming: the way the eye looks, the entity turning to face it
	aim    geom.Vec
	aiming bool

	// behind: how far the arm reaches, shortened by what stands between
	reach float32
	// behind: how long since the mouse turned the camera round the entity, the easing back
	// behind it waiting orbitHold
	orbited time.Duration
	// inside: the unit's own Eye.Tilt, given back on letting go
	tilt0  float32
	tilted bool
}

func (f *following) inside() bool { return f.how == camera.Inside }

// kept reports whether a camera fastened how is the keeper's to place every tick
func kept(how camera.How) bool { return how == camera.Behind || how == camera.Inside }

// about two thirds of any turn in this time
const followEase = 250 * time.Millisecond

// behind an entity, how long after the mouse the camera is left where the mouse put it
const orbitHold = time.Second

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
	s.orders.Rides(func(c camera.Camera, entity uid.UID64) {
		if cam, ok := c.(*viewCamera); ok {
			s.ride(cam, entity)
		}
	})
	s.orders.Looks(func(c camera.Camera, dx, dy float32) {
		for k := range s.following {
			f := &s.following[k]
			switch {
			case f.cam != c:
			case f.inside():
				s.look(f, dx, dy)
			default:
				s.orbit(f, dx, dy)
			}
		}
	})
	s.reconcile()
	kept := s.following[:0]
	var gone []following
	for _, f := range s.following {
		if s.keep(&f, d) {
			kept = append(kept, f)
		} else {
			gone = append(gone, f)
		}
	}
	s.following = kept
	if s.cams != nil {
		for _, cam := range *s.cams {
			if cam.inPersp {
				cam.persp.settle(d)
			}
		}
	}
	for _, f := range gone {
		if f.inside() {
			f.cam.leaveInside()
		} else {
			s.release(f.cam, f.target)
		}
		f.cam.Fasten(camera.Fastening{})
	}
}

// round the entity the camera is fastened to; one fastened to nothing goes behind entity; both
// in the perspective alone
func (s *Keeper) ride(cam *viewCamera, entity uid.UID64) {
	f := cam.Fastening()
	switch f.How {
	case camera.Centred:
		if !s.perspective {
			return
		}
		f.How = camera.Behind
	case camera.Behind:
		f.How = camera.Inside
	case camera.Inside:
		f.How = camera.Centred
	default:
		if _, ok := s.ents.Of(entity); !ok || !s.perspective {
			return
		}
		f = camera.Fastening{Entity: entity, How: camera.Behind}
	}
	cam.Fasten(f)
}

func (s *Keeper) reconcile() {
	if s.cams == nil {
		return
	}
	for _, cam := range *s.cams {
		f := cam.Fastening()
		want := kept(f.How)
		if e := s.fastened(cam); e != nil && (!want || e.target != f.Entity || e.how != f.How) {
			s.letGo(cam)
		}
		if want && s.fastened(cam) == nil {
			s.fasten(cam, f.Entity, f.How)
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

func (s *Keeper) fasten(cam *viewCamera, id uid.UID64, how camera.How) {
	if _, ok := s.ents.Of(id); !ok {
		cam.Fasten(camera.Fastening{})
		return
	}
	if how == camera.Inside && !s.enter(cam, id) || how == camera.Behind && !cam.enterBehind() {
		cam.Fasten(camera.Fastening{Entity: id, How: camera.Centred})
		return
	}
	s.following = append(s.following, following{cam: cam, target: id, how: how,
		orbited: orbitHold})
	s.keep(&s.following[len(s.following)-1], 0)
}

// from behind id back into the view the camera came from, over id where it still is
func (s *Keeper) release(cam *viewCamera, id uid.UID64) {
	cam.leaveBehind()
	if t, ok := s.ents.Of(id); ok {
		cam.CenterOn(t.X, t.Y, t.Alt)
	}
}

func (s *Keeper) letGo(cam *viewCamera) bool {
	for i, f := range s.following {
		if f.cam == cam {
			s.write(f.target, geom.Vec{}, false, 0)
			s.untilt(&s.following[i])
			s.following = append(s.following[:i], s.following[i+1:]...)
			if f.inside() {
				cam.leaveInside()
			} else {
				s.release(cam, f.target)
			}
			return true
		}
	}
	return false
}

func (s *Keeper) comeOut(cam *viewCamera) {
	if f := s.fastened(cam); f != nil && f.inside() {
		s.letGo(cam)
		cam.Fasten(camera.Fastening{})
	}
}

// the eye's part of what drives it alone; the hand's part is whoever drives it
func (s *Keeper) write(id uid.UID64, look geom.Vec, flown bool, climb float32) {
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
	sn, cs := f32.Sincos(dx * LookStep)
	f.aim, f.aiming = geom.NewVec(from.X*cs-from.Y*sn, from.X*sn+from.Y*cs), true
	f.cam.Turn(behind(f.aim.X, f.aim.Y) - f.cam.Heading())
}

// the mouse behind an entity turns the camera round it, the same way round as the look inside
// turns the view, and tilts the arm where the settings let it; the entity goes its own way
func (s *Keeper) orbit(f *following, dx, dy float32) {
	if a := s.arms.Arm(); dy != 0 && a.Tilts {
		s.arms.Tilt(a.Pitch + dy*LookStep)
	}
	if dx != 0 {
		f.cam.Turn(-dx * LookStep)
	}
	f.orbited = 0
}

// the inverse of behind
func facing(heading float32) geom.Vec {
	s, c := f32.Sincos(heading + math.Pi/4)
	return geom.NewVec(-s, -c)
}

func (s *Keeper) keep(f *following, d time.Duration) bool {
	t, ok := s.ents.Of(f.target)
	if !ok {
		return false
	}
	cx, cy, dir := t.X, t.Y, t.Dir
	faces := dir.X != 0 || dir.Y != 0
	if f.inside() && f.aiming {
		// the keys turn it: the look goes with it again
		if t.Turning {
			f.aiming = false
		} else if faces && f32.Abs(f32.Atan2(dir.X*f.aim.Y-dir.Y*f.aim.X,
			dir.X*f.aim.X+dir.Y*f.aim.Y)) < 1e-3 {
			f.aiming = false // it faces the way the eye looks: pinned to it again
		}
	}
	if !f.inside() {
		f.orbited += d
		// the camera eases back behind the entity as it goes on, the mouse's view holding while
		// it stands and orbitHold after the mouse; the first centring takes all of it
		settles := d == 0 || t.Moving && f.orbited >= orbitHold
		if faces && settles {
			by := ease(behind(dir.X, dir.Y)-f.cam.Heading(), d)
			if f32.Abs(by) > 1e-4 {
				f.cam.Turn(by)
			}
		}
		a := s.arms.Arm()
		aim := [3]float32{cx, cy, t.Alt + a.Aim*max(t.Level-t.Alt, 0)}
		s.hold(f, aim, a, d)
		return true
	}
	// flown from inside: the way it is steered rises as the rider looks up
	var look geom.Vec
	if f.aiming {
		look = f.aim
	}
	s.ents.Drive(f.target, look, true, -f32.Sin(f.cam.Pitch()))
	// the rider's head is the unit's eye: it looks up and down as the rider does
	if t.Eyed {
		if !f.tilted {
			f.tilt0, f.tilted = t.Tilt, true
		}
		s.ents.Tilt(f.target, -f.cam.Pitch())
	}
	way, looks := dir, faces
	if f.aiming {
		way, looks = f.aim, true
	}
	if looks {
		by := ease(behind(way.X, way.Y)-f.cam.Heading(), 0)
		if f32.Abs(by) > 1e-5 {
			f.cam.Turn(by)
		}
	}
	f.cam.CenterOn(cx, cy, s.riding(cx, cy, t.Level, f.cam.persp.cell))
	return true
}

// the eye on the arm a behind the point at, tilted as the arm says: shortened at once to what
// stands between, eased back out as the way clears
func (s *Keeper) hold(f *following, at [3]float32, a Arm, d time.Duration) {
	p := f.cam.persp
	p.pitch = a.Pitch
	reach := s.reach(at, p.out(), a.Distance*p.cell, p.cell)
	if f.reach > 0 && reach > f.reach {
		reach = f.reach + (reach-f.reach)*eased(d)
	}
	f.reach = reach
	p.hold(at, reach)
}

// how far along way from t the arm reaches, up to length: a quarter of a cell short of the first
// thing solid
func (s *Keeper) reach(t, way [3]float32, length, cell float32) float32 {
	if s.free == nil {
		return length
	}
	step := cell / 4
	for along := step; ; along += step {
		at := vec.Add(t, vec.Scale(way, min(along, length)))
		if !s.free(at[0], at[1], at[2]) {
			return max(min(along, length)-step, step)
		}
		if along >= length {
			return length
		}
	}
}

func (s *Keeper) enter(cam *viewCamera, id uid.UID64) bool {
	eye, across, dir, ok := s.eyeOf(id)
	if !ok {
		return false
	}
	heading := cam.Heading()
	if dir.X != 0 || dir.Y != 0 {
		heading = behind(dir.X, dir.Y)
	}
	eye[2] = s.riding(eye[0], eye[1], eye[2], cam.persp.cell)
	return cam.enterInside(eye, heading, across)
}

// across is radians, 0 without an Eye; dir is of length 1 or none
func (s *Keeper) eyeOf(id uid.UID64) (eye [3]float32, across float32, dir geom.Vec, ok bool) {
	t, ok := s.ents.Of(id)
	if !ok {
		return eye, across, dir, false
	}
	return [3]float32{t.X, t.Y, t.Level}, t.Across, t.Dir, true
}

// never in what is solid there, nor within a hair of it
func (s *Keeper) riding(x, y, top, cell float32) float32 {
	if s.free == nil || s.free(x, y, top) {
		return top
	}
	return rise(s.free, x, y, top, cell) + cell/256
}

// the heading from which the way (dx, dy) runs up the screen
func behind(dx, dy float32) float32 {
	return f32.Atan2(-dx, -dy) - math.Pi/4
}

// the shorter way round; all of it for d zero, the first centring
func ease(by float32, d time.Duration) float32 {
	return f32.Remainder(by, 2*math.Pi) * eased(d)
}

func eased(d time.Duration) float32 {
	if d <= 0 {
		return 1
	}
	return 1 - f32.Exp(-float32(d)/float32(followEase))
}
