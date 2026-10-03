package bullet

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*shootSystem)(nil)

// shootSystem carries out the Shoots given: from the entity that gave one for itself, or from
// every Selected entity the player who gave it owns, it spawns a shot of the Ammo at the muzzle,
// aimed where the command says, else the way the shooter faces.
type shootSystem struct {
	w        *world.Plugin
	shoots   *control.Queue[Shoot]
	selected tag.Tag[selection.Family]
	groundOf func() ground.Heights

	query  *goke.Query
	base   goke.Comp[world.Base]
	marks  goke.OptComp[tag.Tags[selection.Family]]
	owners goke.OptComp[tag.Tags[owner.Family]]
	z      goke.OptComp[world.Z]
	eye    goke.OptComp[world.Eye]

	shooters []shooter // the shooters of one command, gathered before any is fired
}

// shooter is one entity a Shoot fires from, as the shootSystem read it.
type shooter struct {
	id     uid.UID64
	pos    world.Position
	facing geom.Vec
	z      *world.Z
	eye    *world.Eye
	owners tag.Tags[owner.Family]
}

func newShootSystem(w *world.Plugin, shoots *control.Queue[Shoot], selected tag.Tag[selection.Family], groundOf func() ground.Heights) *shootSystem {
	return &shootSystem{w: w, shoots: shoots, selected: selected, groundOf: groundOf}
}

func (s *shootSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base).Optional(&s.marks).Optional(&s.owners).Optional(&s.z).Optional(&s.eye).Build()
}

func (s *shootSystem) Update(*goke.CmdBuf, time.Duration) {
	if s.shoots.Empty() {
		return
	}
	s.shoots.Drain(func(i control.Issued[Shoot]) {
		s.shooters = s.shooters[:0]
		if i.ByEntity {
			if s.query.Seek(i.Entity) {
				s.shooters = append(s.shooters, s.at(i.Entity, s.query.Cursor()))
			}
		} else {
			s.selectedOf(i.Player)
		}
		for _, sh := range s.shooters {
			s.fire(i.Command, sh)
		}
	})
}

// at is the entity under the cursor, sought, as a shooter.
func (s *shootSystem) at(id uid.UID64, cur *goke.Cursor) shooter {
	b := s.base.At(cur)
	sh := shooter{id: id, pos: b.Pos, facing: b.Vel.Dir, z: s.z.At(cur), eye: s.eye.At(cur)}
	if o := s.owners.At(cur); o != nil {
		sh.owners = *o
	}
	return sh
}

// selectedOf gathers every Selected entity by owns as a shooter.
func (s *shootSystem) selectedOf(by control.PlayerID) {
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		bases, marks, owners := s.base.Slice(cur), s.marks.Slice(cur), s.owners.Slice(cur)
		zs, eyes := s.z.Slice(cur), s.eye.Slice(cur)
		if marks == nil {
			continue
		}
		for i, id := range cur.IDs {
			var owned tag.Tags[owner.Family]
			if owners != nil {
				owned = owners[i]
			}
			if !marks[i].Has(s.selected) || !owner.Obeys(owned, by) {
				continue
			}
			sh := shooter{id: id, pos: bases[i].Pos, facing: bases[i].Vel.Dir, owners: owned}
			if zs != nil {
				sh.z = &zs[i]
			}
			if eyes != nil {
				sh.eye = &eyes[i]
			}
			s.shooters = append(s.shooters, sh)
		}
	}
}

// fire spawns a shot of cmd's Ammo from sh: at the muzzle, just outside sh's box the way it goes
// — towards cmd.At when Targeted, towards the entity it was aimed at while that one is there, else
// the way sh faces; nowhere when no way is known. A thrown shot (Gravity) gets the climb that
// brings it down where it goes to, at its Range.
func (s *shootSystem) fire(cmd Shoot, sh shooter) {
	centre := sh.pos.Center()
	at, targeted := cmd.At, cmd.Targeted
	if !targeted && cmd.aimed && s.query.Seek(cmd.target) {
		at, targeted = s.base.At(s.query.Cursor()).Pos.Center(), true
	}
	dir := sh.facing
	if targeted {
		dir = at.Sub(centre)
	}
	l := math.Hypot(dir.X, dir.Y)
	if l < 1e-9 {
		return
	}
	dir = geom.NewVec(dir.X/l, dir.Y/l)
	body := cmd.Ammo.Body()
	off := (max(sh.pos.Size.X, sh.pos.Size.Y)+body.Size)/2/max(math.Abs(dir.X), math.Abs(dir.Y)) + 1
	from := geom.NewVec(centre.X+dir.X*off, centre.Y+dir.Y*off)

	shot := Shot{From: from, Dir: dir, Range: body.Range, Shooter: sh.id, Owners: sh.owners}
	if sh.z != nil {
		shot.Altitude = sh.z.Altitude + sh.z.Height/2
		if sh.eye != nil {
			shot.Altitude = sh.eye.Level(*sh.z)
		}
	}
	if body.Gravity > 0 {
		if targeted {
			shot.Range = min(math.Hypot(at.X-from.X, at.Y-from.Y), body.Range)
		}
		t := max(shot.Range, 1e-9) / body.Speed
		down := geom.NewVec(from.X+dir.X*shot.Range, from.Y+dir.Y*shot.Range)
		shot.Climb = body.Gravity*t/2 + (groundAt(s.groundOf, down)-shot.Altitude)/t
	}
	s.w.Spawn(cmd.Ammo.Entry(shot))
}

// groundAt is the height of the ground at p: 0 without one.
func groundAt(groundOf func() ground.Heights, p geom.Vec) float64 {
	if groundOf == nil {
		return 0
	}
	if g := groundOf(); g != nil {
		return g.At(p)
	}
	return 0
}
