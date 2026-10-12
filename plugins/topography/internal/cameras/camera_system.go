package cameras

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/vec"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cameraSystem)(nil)

type cameraSystem struct {
	orders      Orders
	relief      *relief.Relief         // nil, level
	topAt       func(geom.Vec) float64 // nil, the ground
	perspective bool                   // Ride reaches Inside
	cams        *[]*viewCamera

	query  *goke.Query
	base   goke.Comp[world.Base]
	z      goke.OptComp[world.Z]
	eye    goke.OptComp[world.Eye]
	driven goke.OptComp[steering.Driven]

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
}

// a part of the screen's height a fastened camera holds its entity below the middle
const shoulder = 0.25

// about two thirds of any turn in this time
const followEase = 250 * time.Millisecond

// how much steeper a fastened eye looks at a time until the ground no longer hides its entity
const clearStep = 5 * math.Pi / 180

func (s *cameraSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base).Optional(&s.z).Optional(&s.eye).Optional(&s.driven).Build()
}

func (s *cameraSystem) Update(_ *goke.CmdBuf, d time.Duration) {
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

func (s *cameraSystem) ride(cam *viewCamera) {
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

func (s *cameraSystem) reconcile() {
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

func (s *cameraSystem) fastened(cam *viewCamera) *following {
	for i := range s.following {
		if s.following[i].cam == cam {
			return &s.following[i]
		}
	}
	return nil
}

func (s *cameraSystem) fasten(cam *viewCamera, id uid.UID64, inside bool) {
	if !s.query.Seek(id) {
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

func (s *cameraSystem) letGo(cam *viewCamera) bool {
	for i, f := range s.following {
		if f.cam == cam {
			s.write(f.target, geom.Vec{}, false, 0)
			s.following = append(s.following[:i], s.following[i+1:]...)
			if f.inside {
				cam.leaveInside()
			}
			return true
		}
	}
	return false
}

func (s *cameraSystem) comeOut(cam *viewCamera) {
	if f := s.fastened(cam); f != nil && f.inside {
		s.letGo(cam)
		cam.Fasten(camera.Fastening{})
	}
}

// the eye's part of a Driven alone; the hand's part is whoever drives it
func (s *cameraSystem) write(id uid.UID64, look geom.Vec, flown bool, climb float64) {
	if !s.query.Seek(id) {
		return
	}
	if d := s.driven.At(s.query.Cursor()); d != nil {
		d.Look, d.Flown, d.Climb = look, flown, climb
	}
}

func (s *cameraSystem) look(f *following, dx, dy float32) {
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

func (s *cameraSystem) keep(f *following, d time.Duration) bool {
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		for i, id := range cur.IDs {
			if id != f.target {
				continue
			}
			base := s.base.Slice(cur)[i]
			box := base.Pos.AABB
			cx, cy := (box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2
			var z world.Z
			if zs := s.z.Slice(cur); zs != nil {
				z = zs[i]
			}
			alt, level := z.Altitude, z.Top()
			if eyes := s.eye.Slice(cur); eyes != nil {
				level = eyes[i].Level(z)
			}
			dir := base.Vel.Dir
			faces := dir.X != 0 || dir.Y != 0
			var driven *steering.Driven
			if drivens := s.driven.Slice(cur); drivens != nil {
				driven = &drivens[i]
			}
			if f.inside && f.aiming {
				// the keys turn it: the look goes with it again
				if driven != nil && driven.Turn != 0 {
					f.aiming = false
				} else if faces && math.Abs(math.Atan2(dir.X*f.aim.Y-dir.Y*f.aim.X, dir.X*f.aim.X+dir.Y*f.aim.Y)) < 1e-3 {
					f.aiming = false // it faces the way the eye looks: pinned to it again
				}
			}
			// flown from inside: the way it is steered rises as the rider looks up
			if driven != nil && f.inside {
				driven.Look = geom.Vec{}
				if f.aiming {
					driven.Look = f.aim
				}
				driven.Flown, driven.Climb = true, -math.Sin(float64(f.cam.Pitch()))
			}
			if f.inside {
				look, looks := dir, faces
				if f.aiming {
					look, looks = f.aim, true
				}
				if looks {
					if by := ease(behind(float32(look.X), float32(look.Y))-f.cam.Heading(), 0); math.Abs(float64(by)) > 1e-5 {
						f.cam.Turn(by)
					}
				}
				f.cam.CenterOn(cx, cy, s.riding(cx, cy, level, f.cam.persp.cell))
				return true
			}
			if faces {
				if by := ease(behind(float32(dir.X), float32(dir.Y))-f.cam.Heading(), d); math.Abs(float64(by)) > 1e-4 {
					f.cam.Turn(by)
				}
			}
			f.cam.CenterOn(cx, cy, alt)
			if f.cam.inPersp { // before it looks over its shoulder
				s.clear(f, [3]float32{float32(cx), float32(cy), float32(alt)}, d)
			}
			// over its shoulder: the lower the eye, the lower the entity on the screen
			_, h := f.cam.Viewport()
			f.cam.shift(0, -h*shoulder*float32(math.Cos(float64(f.cam.Pitch()))))
			return true
		}
	}
	return false
}

// looks down steeply enough to see the entity at once, eases back out as the way clears
func (s *cameraSystem) clear(f *following, t [3]float32, d time.Duration) {
	p := f.cam.persp
	if f.appliedPitch == 0 || p.tilts != f.tilts {
		f.wantPitch, f.tilts = p.pitch, p.tilts
	}
	free := f.wantPitch
	if s.relief != nil {
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

func (s *cameraSystem) clearPitch(p *perspCamera, t [3]float32, want float32) float32 {
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

func (s *cameraSystem) lineClear(t, e [3]float32, cell float32) bool {
	d := vec.Sub(e, t)
	h := float32(math.Hypot(float64(d[0]), float64(d[1])))
	step := cell / 2
	for along := cell; along < h; along += step {
		at := vec.Add(t, vec.Scale(d, along/h))
		if float32(s.relief.GroundAt(geom.NewVec(float64(at[0]), float64(at[1]))))+cell/4 > at[2] {
			return false
		}
	}
	return true
}

func (s *cameraSystem) enter(cam *viewCamera, id uid.UID64) bool {
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
func (s *cameraSystem) eyeOf(id uid.UID64) (eye [3]float32, across float32, dir geom.Vec, ok bool) {
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		for i, got := range cur.IDs {
			if got != id {
				continue
			}
			base := s.base.Slice(cur)[i]
			box := base.Pos.AABB
			var z world.Z
			if zs := s.z.Slice(cur); zs != nil {
				z = zs[i]
			}
			level := z.Top()
			if eyes := s.eye.Slice(cur); eyes != nil {
				level, across = eyes[i].Level(z), float32(eyes[i].Angle)
			}
			eye = [3]float32{float32((box.TopLeft.X + box.BottomRight.X) / 2), float32((box.TopLeft.Y + box.BottomRight.Y) / 2), float32(level)}
			if n := math.Hypot(base.Vel.Dir.X, base.Vel.Dir.Y); n > 0 {
				dir = geom.NewVec(base.Vel.Dir.X/n, base.Vel.Dir.Y/n)
			}
			return eye, across, dir, true
		}
	}
	return eye, across, dir, false
}

// never under the top of the cell as it is drawn, nor within a hair of it
func (s *cameraSystem) riding(x, y, top float64, cell float32) float64 {
	if s.topAt == nil {
		return top
	}
	return max(top, s.topAt(geom.NewVec(x, y))+float64(cell)/256)
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
