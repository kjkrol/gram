package cameras

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/vec"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cameraSystem)(nil)

// cameraSystem carries out the view's commands on the cameras of this view, and every tick keeps
// each fastened camera with its entity: behind it — centred on it at its altitude and turning,
// eased, until the way it walks runs up the screen, in perspective looking down more steeply
// where the ground between them would hide it — or inside it, the eye going with it and looking
// the way it faces. Only Follow or LookOut given again, View for an eye inside, or the entity
// gone, lets it go.
type cameraSystem struct {
	orders Orders
	relief *relief.Relief // the ground between a fastened eye and its entity; nil, level
	// topAt is the top of the cell under a point as it is drawn — the ground and its kind's
	// Height — which an eye riding in an entity never goes under; nil, the ground
	topAt     func(geom.Vec) float64
	selected  tag.Tag[selection.Family]
	selecting bool // the selection was given: Follow has a unit to fasten to

	query    *goke.Query
	base     goke.Comp[world.Base]
	z        goke.OptComp[world.Z]
	eye      goke.OptComp[world.Eye]
	marks    goke.OptComp[tag.Tags[selection.Family]]
	owners   goke.OptComp[tag.Tags[owner.Family]]
	driven   goke.OptComp[steering.Driven]
	drivenID goke.CompID

	following []following
	released  []uid.UID64 // let go last tick, stopped by now: their Driven comes off
}

// following is one camera fastened to one entity — behind it, or inside it — and how its entity
// is driven this tick; behind it in perspective, how steeply the player wants to look down
// (wantPitch, read after tilts tilts) and how steeply the eye looks (appliedPitch).
type following struct {
	cam    *viewCamera
	target uid.UID64
	drive  steering.Driven
	inside bool
	// inside: the way the eye looks, the entity turning to face it, while aiming — till it does
	aim    geom.Vec
	aiming bool

	wantPitch, appliedPitch float32
	tilts                   uint32
}

// shoulder is how far below the middle of the screen, as a part of its height, a fastened camera
// holds its entity looking along the ground; less the steeper it looks down, none straight down.
const shoulder = 0.25

// followEase is how long a fastened camera takes to turn most of the way behind its entity: about
// two thirds of any turn in that time.
const followEase = 250 * time.Millisecond

// clearStep is by how much a fastened eye in perspective looks down more steeply at a time, until
// the ground no longer hides its entity.
const clearStep = 5 * math.Pi / 180

func (s *cameraSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base).Optional(&s.z).Optional(&s.eye).Optional(&s.marks).Optional(&s.owners).Optional(&s.driven).Build()
	s.drivenID = si.RegComp[steering.Driven]()
}

func (s *cameraSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	for _, id := range s.released {
		cb.RemoveCompOne(id, s.drivenID)
	}
	s.released = s.released[:0]
	s.orders.Views(func(c camera.Camera) {
		if cam, ok := c.(*viewCamera); ok {
			if f := s.fastened(cam); f != nil && f.inside { // out of the unit, back to the view it was in
				s.letGo(cam)
				return
			}
			cam.next()
		}
	})
	s.orders.Turns(func(c camera.Camera, angle float32) {
		if cam, ok := c.(*viewCamera); ok && !cam.insideUnit() { // an eye inside turns with its unit
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
	s.orders.LookOuts(func(c camera.Camera, by control.PlayerID) {
		if cam, ok := c.(*viewCamera); ok {
			s.lookOut(cb, cam, by)
		}
	})
	s.orders.Follows(func(c camera.Camera, by control.PlayerID) {
		cam, ok := c.(*viewCamera)
		if !ok {
			return
		}
		if f := s.fastened(cam); f != nil {
			inside := f.inside
			s.letGo(cam)
			if !inside {
				return
			}
		}
		if id, ok := s.theSelected(by); ok {
			s.fasten(cb, cam, id, false)
		}
	})
	for k := range s.following {
		s.following[k].drive = steering.Driven{}
	}
	s.orders.Drives(func(c camera.Camera, ahead, turn int8, sprint bool) {
		for k := range s.following {
			if f := &s.following[k]; f.cam == c {
				f.drive.Ahead = max(min(f.drive.Ahead+ahead, 1), -1)
				f.drive.Turn = max(min(f.drive.Turn+turn, 1), -1)
				f.drive.Sprint = f.drive.Sprint || sprint
			}
		}
	})
	s.orders.Looks(func(c camera.Camera, dx, dy float32) {
		for k := range s.following {
			if f := &s.following[k]; f.cam == c && f.inside {
				s.look(f, dx, dy)
			}
		}
	})
	kept := s.following[:0]
	var gone []*viewCamera // their entities gone: nothing to stop, the cameras on their own again
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
	}
}

// fastened is what cam is fastened to, nil for nothing.
func (s *cameraSystem) fastened(cam *viewCamera) *following {
	for i := range s.following {
		if s.following[i].cam == cam {
			return &s.following[i]
		}
	}
	return nil
}

// fasten fastens cam to id, behind it or inside it, driving it from now on.
func (s *cameraSystem) fasten(cb *goke.CmdBuf, cam *viewCamera, id uid.UID64, inside bool) {
	for i, r := range s.released { // fastened again before its Driven came off: it stays on
		if r == id {
			s.released = append(s.released[:i], s.released[i+1:]...)
			break
		}
	}
	cb.AddOne(id, s.drivenID, steering.Driven{})
	s.following = append(s.following, following{cam: cam, target: id, inside: inside})
	s.keep(&s.following[len(s.following)-1], 0)
}

// letGo unfastens cam and reports whether it was fastened; its entity, no hand on it, brakes to a
// stop and is no longer driven from the next tick, and an eye inside it comes out.
func (s *cameraSystem) letGo(cam *viewCamera) bool {
	for i, f := range s.following {
		if f.cam == cam {
			s.write(f.target, steering.Driven{})
			s.released = append(s.released, f.target)
			s.following = append(s.following[:i], s.following[i+1:]...)
			if f.inside {
				cam.leaveInside()
			}
			return true
		}
	}
	return false
}

// comeOut lets cam go when its eye is inside an entity: what puts the eye elsewhere does first.
func (s *cameraSystem) comeOut(cam *viewCamera) {
	if f := s.fastened(cam); f != nil && f.inside {
		s.letGo(cam)
	}
}

// write sets how id is driven, when it carries a Driven yet.
func (s *cameraSystem) write(id uid.UID64, in steering.Driven) {
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		drivens := s.driven.Slice(cur)
		if drivens == nil {
			continue
		}
		for i, got := range cur.IDs {
			if got == id {
				drivens[i] = in
				return
			}
		}
	}
}

// theSelected is the one Selected entity player by owns (owner.Obeys); false with none, or
// several.
func (s *cameraSystem) theSelected(by control.PlayerID) (uid.UID64, bool) {
	if !s.selecting {
		return 0, false
	}
	var one uid.UID64
	n := 0
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		marks, owners := s.marks.Slice(cur), s.owners.Slice(cur)
		if marks == nil {
			continue
		}
		for i, m := range marks {
			var owned tag.Tags[owner.Family]
			if owners != nil {
				owned = owners[i]
			}
			if m.Has(s.selected) && owner.Obeys(owned, by) {
				one, n = cur.IDs[i], n+1
			}
		}
	}
	return one, n == 1
}

// look has the eye riding in f's entity look round by a move of the mouse, dx and dy pixels: the head raised or lowered at once,
// the way it looks turned at once, the entity turning to face it from the next tick.
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

// facing is the way along the ground an eye at heading looks: the way behind puts up the screen.
func facing(heading float32) geom.Vec {
	s, c := math.Sincos(float64(heading) + math.Pi/4)
	return geom.NewVec(-s, -c)
}

// keep holds f's camera with its entity over d: behind it, turned towards the way it walks and
// centred on it at its altitude; inside it, the eye at its centre as high as it stands, the
// entity steered by the eye's Pans and the view turned with it while it turns. False when the
// entity is gone.
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
			if f.inside && f.aiming {
				if f.drive.Turn != 0 { // the keys turn it: the look goes with it again
					f.aiming = false
				} else {
					f.drive.Face = f.aim
					if faces && math.Abs(math.Atan2(dir.X*f.aim.Y-dir.Y*f.aim.X, dir.X*f.aim.X+dir.Y*f.aim.Y)) < 1e-3 {
						f.aiming = false // it faces the way the eye looks: pinned to it again
					}
				}
			}
			if f.inside { // flown from inside: the way it is steered rises as the rider looks up
				f.drive.Flown, f.drive.Climb = true, -math.Sin(float64(f.cam.Pitch()))
			}
			if drivens := s.driven.Slice(cur); drivens != nil {
				drivens[i] = f.drive
			}
			if f.inside { // the eye in the entity, looking the way it faces or turns to face
				look, looks := dir, faces
				if f.aiming || f.drive.Face != (geom.Vec{}) {
					look, looks = f.aim, true
				}
				if looks {
					if by := ease(behind(float32(look.X), float32(look.Y))-f.cam.Heading(), 0); math.Abs(float64(by)) > 1e-5 {
						f.cam.Turn(by)
					}
				}
				f.cam.CenterOn(cx, cy, s.riding(cx, cy, level, f.cam.persp.cell)) // where its eye is
				return true
			}
			if faces {
				if by := ease(behind(float32(dir.X), float32(dir.Y))-f.cam.Heading(), d); math.Abs(float64(by)) > 1e-4 {
					f.cam.Turn(by)
				}
			}
			f.cam.CenterOn(cx, cy, alt)
			if f.cam.inPersp { // the eye in sight of the entity, before it looks over its shoulder
				s.clear(f, [3]float32{float32(cx), float32(cy), float32(alt)}, d)
			}
			// over its shoulder: the lower the eye, the lower the entity on the screen, the more of
			// what lies ahead above it
			_, h := f.cam.Viewport()
			f.cam.Pan(0, -h*shoulder*float32(math.Cos(float64(f.cam.Pitch()))))
			return true
		}
	}
	return false
}

// clear has a fastened camera in perspective, looking at its entity at t, look down steeply
// enough for the ground not to hide it: the eye — as high as it flies — comes in over the
// entity at once and goes back out, eased, as the way clears; the player's Tilt sets how far.
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

// clearPitch is the least pitch, from want up in clearStep steps to straight down, at which the
// eye of p, as high as it flies, sees t over the ground between them.
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

// lineClear reports whether the line from t to e runs over the ground, a quarter cell above it,
// sampled every half cell from a cell off t.
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

// lookOut puts cam's eye inside the one Selected entity player by owns and keeps it there — at its centre, as
// high as it stands, looking the way it faces, up the screen where it never moved — or lets it
// out, back to the view it was in, when it is inside one already; a camera fastened behind an
// entity is let go first.
func (s *cameraSystem) lookOut(cb *goke.CmdBuf, cam *viewCamera, by control.PlayerID) {
	if f := s.fastened(cam); f != nil {
		inside := f.inside
		s.letGo(cam)
		if inside {
			return
		}
	}
	id, ok := s.theSelected(by)
	if !ok {
		return
	}
	eye, across, dir, ok := s.eyeOf(id)
	if !ok {
		return
	}
	heading := cam.Heading()
	if dir.X != 0 || dir.Y != 0 {
		heading = behind(float32(dir.X), float32(dir.Y))
	}
	eye[2] = float32(s.riding(float64(eye[0]), float64(eye[1]), float64(eye[2]), cam.persp.cell))
	if !cam.enterInside(eye, heading, across) {
		return
	}
	s.fasten(cb, cam, id, true)
}

// eyeOf is where id looks from — its centre, as high as its world.Eye stands over its Z, its top
// without one — how wide it sees across (radians; 0 without an Eye: the camera's own field) and
// the way it faces, of length 1 or none; false when id is gone.
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

// riding is how high an eye riding in an entity at (x, y) stands: on the entity's top, but never
// under the top of the cell there as it is drawn — raised by its kind's Height — nor within a hair
// of it.
func (s *cameraSystem) riding(x, y, top float64, cell float32) float64 {
	if s.topAt == nil {
		return top
	}
	return max(top, s.topAt(geom.NewVec(x, y))+float64(cell)/256)
}

// behind is the heading from which the way (dx, dy) runs up the screen: the eye behind it.
func behind(dx, dy float32) float32 {
	return float32(math.Atan2(float64(-dx), float64(-dy))) - math.Pi/4
}

// ease is how much of the turn by, the shorter way round, a fastened camera makes over d; all of
// it for d zero, the first centring.
func ease(by float32, d time.Duration) float32 {
	return float32(math.Remainder(float64(by), 2*math.Pi)) * eased(d)
}

// eased is the part of any way a fastened camera goes over d: all of it for d zero.
func eased(d time.Duration) float32 {
	if d <= 0 {
		return 1
	}
	return float32(1 - math.Exp(-float64(d)/float64(followEase)))
}
