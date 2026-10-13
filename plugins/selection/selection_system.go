package selection

import (
	cgoke "github.com/kjkrol/ceremony/goke"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*SelectionSystem)(nil)

// world units: a unit high up is drawn above its ground
const pickReach = 160

// SelectionSystem carries out the selection's commands: Select as the Selected tag on the
// Selectable entities the player owns, a command for the selected or the pointed at, Hovered on
// the entity each Hover points at.
type SelectionSystem struct {
	selects    *command.Queue[Select]
	effectCmds *command.Queue[effectCommand]
	effects    *cgoke.Effects
	whom       []uid.UID64
	hovers     *command.Queue[Hover]
	hovered    bool
	allows     *command.Queue[Allow]
	forbids    *command.Queue[Forbid]
	marksID    goke.CompID
	space      *aabbworld.Space
	tags       Tags

	marqueeQueue *command.Queue[Marquee]
	marquees     *marquees

	query  *goke.Query
	marks  goke.Comp[tag.Tags[Family]]
	owners goke.OptComp[tag.Tags[owner.Family]]
	roles  goke.OptComp[tag.Tags[cond.Roles]]

	lookup     *goke.Query
	lookupBase goke.Comp[world.Base]
	lookupZ    goke.OptComp[world.Z]

	look func() world.Look
}

// NewSelectionSystem builds a SelectionSystem draining selects over space, picking through each
// command's camera what look draws there.
func NewSelectionSystem(selects *command.Queue[Select], space *aabbworld.Space, tags Tags, look func() world.Look) *SelectionSystem {
	return &SelectionSystem{selects: selects, space: space, tags: tags, look: look}
}

func (s *SelectionSystem) Init(si *goke.SysInit) {
	s.marksID = si.RegComp[tag.Tags[Family]]()
	s.query = si.NewQueryBuilder(&s.marks).Optional(&s.owners).Optional(&s.roles).Build()
	s.lookup = si.NewQueryBuilder(&s.lookupBase).Optional(&s.lookupZ).Build()
}

func (s *SelectionSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	if s.marqueeQueue != nil {
		s.marqueeQueue.Drain(func(i command.Issued[Marquee]) {
			if i.Command.Camera != nil {
				s.marquees.show(i.Command.Camera, i.Command.Screen)
			}
		})
	}
	s.selects.Drain(func(i command.Issued[Select]) {
		cmd := i.Command
		if s.marquees != nil && cmd.Camera != nil {
			s.marquees.hide(cmd.Camera)
		}
		hit := make(map[uid.UID64]struct{}, len(cmd.IDs))
		switch {
		case cmd.IDs != nil:
			for _, id := range cmd.IDs {
				hit[id] = struct{}{}
			}
		case cmd.Screen == (geom.AABB{}) || cmd.Camera == nil:
			s.space.Query(cmd.Box, aabbworld.AnyCapability, func(id uid.UID64) { hit[id] = struct{}{} })
		default:
			s.space.Query(grow(cmd.Box, pickReach), aabbworld.AnyCapability, func(id uid.UID64) {
				if s.drawnIn(id, cmd.Screen, cmd.Camera) {
					hit[id] = struct{}{}
				}
			})
		}
		s.applySelection(hit, cmd.Additive, i.Issuer)
	})
	if s.effectCmds != nil {
		s.effectCmds.Drain(func(i command.Issued[effectCommand]) { s.carry(cb, i.Issuer, i.Command) })
	}
	if s.hovers != nil {
		s.hover()
	}
	if s.allows != nil {
		s.allows.Drain(func(i command.Issued[Allow]) { s.permit(cb, i.Issuer, i.Entity, i.ByEntity, true, i.Command.Selected) })
		s.forbids.Drain(func(i command.Issued[Forbid]) { s.permit(cb, i.Issuer, i.Entity, i.ByEntity, false, false) })
	}
}

func (s *SelectionSystem) permit(cb *goke.CmdBuf, by command.Issuer, id uid.UID64, byEntity, on, selected bool) {
	set := func(m *tag.Tags[Family]) {
		if on {
			*m = m.With(s.tags.Selectable)
			if selected {
				*m = m.With(s.tags.Selected)
			}
		} else {
			*m = m.Without(s.tags.Selectable).Without(s.tags.Selected)
		}
	}
	if !byEntity {
		s.whom = s.whom[:0]
		s.eachSelected(by, 0, func(id uid.UID64) { s.whom = append(s.whom, id) })
		for _, id := range s.whom {
			if s.query.Seek(id) {
				set(s.marks.At(s.query.Cursor()))
			}
		}
		return
	}
	if s.query.Seek(id) {
		set(s.marks.At(s.query.Cursor()))
	} else if on {
		var m tag.Tags[Family]
		set(&m)
		cb.AddOne(id, s.marksID, m)
	}
}

func (s *SelectionSystem) carry(cb *goke.CmdBuf, by command.Issuer, c effectCommand) {
	e := c.cmd.Effect
	if e == (effect.Effect{}) {
		return
	}
	s.whom = s.whom[:0]
	if c.pointed {
		if id, ok := s.pointed(c); ok {
			s.whom = append(s.whom, id)
		}
	} else {
		s.eachSelected(by, c.only, func(id uid.UID64) { s.whom = append(s.whom, id) })
	}
	on := c.cmd.Verb == ceremony.Puts
	if c.cmd.Verb == ceremony.Toggles {
		on = true
		for _, id := range s.whom {
			if s.effects.Has(id, e) {
				on = false
				break
			}
		}
	}
	for _, id := range s.whom {
		switch {
		case !on:
			s.effects.Dispel(id, e)
		case c.cmd.Lasts > 0:
			s.effects.CastFor(id, e, c.cmd.Lasts)
		default:
			s.effects.Cast(id, e)
		}
	}
}

func (s *SelectionSystem) pointed(c effectCommand) (uid.UID64, bool) {
	if !c.aimed || c.camera == nil {
		return 0, false
	}
	return s.pick(c.at, c.screen, c.camera)
}

func (s *SelectionSystem) hover() {
	if s.hovered {
		for s.query.All(); s.query.Next(); {
			marks := s.marks.Slice(s.query.Cursor())
			for i := range marks {
				marks[i] = marks[i].Without(s.tags.Hovered)
			}
		}
		s.hovered = false
	}
	s.hovers.Drain(func(i command.Issued[Hover]) {
		h := i.Command
		if h.Camera == nil {
			return
		}
		if id, ok := s.pick(h.At, h.Screen, h.Camera); ok && s.query.Seek(id) {
			m := s.marks.At(s.query.Cursor())
			*m = m.With(s.tags.Hovered)
			s.hovered = true
		}
	})
}

func (s *SelectionSystem) pick(at geom.Vec, screen geom.AABB, cam camera.Camera) (uid.UID64, bool) {
	var best uid.UID64
	found, nearest := false, 0.0
	s.space.Query(grow(geom.NewAABBAt(at, 1, 1), pickReach), aabbworld.AnyCapability, func(id uid.UID64) {
		if !s.drawnIn(id, screen, cam) {
			return
		}
		centre := s.lookupBase.At(s.lookup.Cursor()).Pos.Center()
		dx, dy := centre.X-at.X, centre.Y-at.Y
		if d := dx*dx + dy*dy; !found || d < nearest {
			best, found, nearest = id, true, d
		}
	})
	return best, found
}

func (s *SelectionSystem) eachSelected(by command.Issuer, only tag.Tags[cond.Roles], fn func(uid.UID64)) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		marks, owners, roles := s.marks.Slice(cursor), s.owners.Slice(cursor), s.roles.Slice(cursor)
		for i, id := range cursor.IDs {
			if marks[i].Has(s.tags.Selectable) && marks[i].Has(s.tags.Selected) && owner.Obeys(ownersAt(owners, i), by) &&
				(only == 0 || (roles != nil && roles[i]&only != 0)) {
				fn(id)
			}
		}
	}
}

func (s *SelectionSystem) drawnIn(id uid.UID64, screen geom.AABB, cam camera.Camera) bool {
	if !s.lookup.Seek(id) {
		return false
	}
	cur := s.lookup.Cursor()
	box := s.lookupBase.At(cur).Pos.AABB
	var stands world.Z
	if z := s.lookupZ.At(cur); z != nil {
		stands = *z
	}
	c := s.look().Drawn(cam, box.AABB, stands)
	minX, minY, maxX, maxY := c[0][0], c[0][1], c[0][0], c[0][1]
	for _, p := range c[1:] {
		minX, maxX = min(minX, p[0]), max(maxX, p[0])
		minY, maxY = min(minY, p[1]), max(maxY, p[1])
	}
	return float64(maxX) >= screen.TopLeft.X && float64(minX) <= screen.BottomRight.X &&
		float64(maxY) >= screen.TopLeft.Y && float64(minY) <= screen.BottomRight.Y
}

func grow(box geom.AABB, reach float64) geom.AABB {
	return geom.AABB{
		TopLeft:     geom.NewVec(box.TopLeft.X-reach, box.TopLeft.Y-reach),
		BottomRight: geom.NewVec(box.BottomRight.X+reach, box.BottomRight.Y+reach),
	}
}

func (s *SelectionSystem) applySelection(hit map[uid.UID64]struct{}, additive bool, by command.Issuer) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		marks, owners := s.marks.Slice(cursor), s.owners.Slice(cursor)
		for i, id := range cursor.IDs {
			if !marks[i].Has(s.tags.Selectable) || !owner.Obeys(ownersAt(owners, i), by) {
				continue
			}
			if _, ok := hit[id]; ok {
				marks[i] = marks[i].With(s.tags.Selected)
			} else if !additive {
				marks[i] = marks[i].Without(s.tags.Selected)
			}
		}
	}
}

func (s *SelectionSystem) chosen(by command.Issuer) (uid.UID64, bool) {
	var one uid.UID64
	n := 0
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		owners := s.owners.Slice(cur)
		for i, m := range s.marks.Slice(cur) {
			if m.Has(s.tags.Selected) && owner.Obeys(ownersAt(owners, i), by) {
				one, n = cur.IDs[i], n+1
			}
		}
	}
	return one, n == 1
}

func ownersAt(owners []tag.Tags[owner.Family], i int) tag.Tags[owner.Family] {
	if owners == nil {
		return 0
	}
	return owners[i]
}
