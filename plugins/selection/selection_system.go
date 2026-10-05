package selection

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*SelectionSystem)(nil)

// pickReach is how far round a Select's Box, in world units, the entities drawn into its Screen
// rectangle may stand: a unit high up is drawn above its ground.
const pickReach = 160

// SelectionSystem carries out Select commands as the Selected tag on Selectable entities — a bit
// flipped in place, seen the same tick — the player who gave one selecting and unselecting only
// what it owns (owner.Obeys). A Select with a Screen rectangle hits the entities drawn into it,
// where the world's Look draws them through the command's camera. After the Selects, a command for
// the selected or the one pointed at (rule.Casting) is carried out.
type SelectionSystem struct {
	selects  *control.Queue[Select]
	castings *control.Queue[casting] // when the plugin wires them
	effects  *effect.Effects         // the world's, which a command casts and takes off
	whom     []uid.UID64             // a command's targets, scratch
	allows   *control.Queue[Allow]
	forbids  *control.Queue[Forbid]
	marksID  goke.CompID
	space    *aabbworld.Space
	tags     Tags

	// The boxes being dragged, when the plugin wires them: Marquee shows one, Select hides it.
	marqueeQueue *control.Queue[Marquee]
	marquees     *marquees

	query  *goke.Query
	marks  goke.Comp[tag.Tags[Family]]
	owners goke.OptComp[tag.Tags[owner.Family]]
	roles  goke.OptComp[tag.Tags[rule.Roles]]

	lookup     *goke.Query
	lookupBase goke.Comp[world.Base]
	lookupZ    goke.OptComp[world.Z]

	look func() world.Look // how the world draws what may be picked
}

// NewSelectionSystem builds a SelectionSystem draining selects over space, picking through each
// command's camera what look draws there.
func NewSelectionSystem(selects *control.Queue[Select], space *aabbworld.Space, tags Tags, look func() world.Look) *SelectionSystem {
	return &SelectionSystem{selects: selects, space: space, tags: tags, look: look}
}

func (s *SelectionSystem) Init(si *goke.SysInit) {
	s.marksID = si.RegComp[tag.Tags[Family]]()
	s.query = si.NewQueryBuilder(&s.marks).Optional(&s.owners).Optional(&s.roles).Build()
	s.lookup = si.NewQueryBuilder(&s.lookupBase).Optional(&s.lookupZ).Build()
}

func (s *SelectionSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	if s.marqueeQueue != nil {
		s.marqueeQueue.Drain(func(i control.Issued[Marquee]) {
			if i.Command.Camera != nil {
				s.marquees.show(i.Command.Camera, i.Command.Screen)
			}
		})
	}
	s.selects.Drain(func(i control.Issued[Select]) {
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
		s.applySelection(hit, cmd.Additive, i.Player)
	})
	if s.castings != nil {
		s.castings.Drain(func(i control.Issued[casting]) { s.carry(cb, i.Player, i.Command) })
	}
	if s.allows != nil {
		s.allows.Drain(func(i control.Issued[Allow]) { s.permit(cb, i.Player, i.Entity, i.ByEntity, true, i.Command.Selected) })
		s.forbids.Drain(func(i control.Issued[Forbid]) { s.permit(cb, i.Player, i.Entity, i.ByEntity, false, false) })
	}
}

// permit makes the entity that asked selectable or not — a player's selected units, when a player
// asked: Selectable on, with Selected when asked, or off with Selected and Followed. An entity without the family gets it.
func (s *SelectionSystem) permit(cb *goke.CmdBuf, by control.PlayerID, id uid.UID64, byEntity, on, selected bool) {
	set := func(m *tag.Tags[Family]) {
		if on {
			*m = m.With(s.tags.Selectable)
			if selected {
				*m = m.With(s.tags.Selected)
			}
		} else {
			*m = m.Without(s.tags.Selectable).Without(s.tags.Selected).Without(s.tags.Followed)
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

// carry does what c says to whom it is for: the player's selected units, or the one pointed at.
func (s *SelectionSystem) carry(cb *goke.CmdBuf, by control.PlayerID, c casting) {
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
	on := c.cmd.Verb == rule.Casts
	if c.cmd.Verb == rule.Toggles {
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
			s.effects.CastFor(cb, id, e, c.cmd.Lasts)
		default:
			s.effects.Cast(cb, id, e)
		}
	}
}

// pointed is the entity drawn under the cursor c was given with, the nearest to the ground point
// under it; none for a command given without a cursor.
func (s *SelectionSystem) pointed(c casting) (uid.UID64, bool) {
	if !c.aimed || c.camera == nil {
		return 0, false
	}
	var best uid.UID64
	found, nearest := false, 0.0
	s.space.Query(grow(geom.NewAABBAt(c.at, 1, 1), pickReach), aabbworld.AnyCapability, func(id uid.UID64) {
		if !s.drawnIn(id, c.screen, c.camera) {
			return
		}
		centre := s.lookupBase.At(s.lookup.Cursor()).Pos.Center()
		dx, dy := centre.X-c.at.X, centre.Y-c.at.Y
		if d := dx*dx + dy*dy; !found || d < nearest {
			best, found, nearest = id, true, d
		}
	})
	return best, found
}

// eachSelected calls fn with every Selectable entity player by owns and has Selected — playing one
// of only's roles, unless only is empty.
func (s *SelectionSystem) eachSelected(by control.PlayerID, only tag.Tags[rule.Roles], fn func(uid.UID64)) {
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

// drawnIn reports whether id is drawn into the screen rectangle, as the world's Look draws it.
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

// grow is box widened by reach on every side.
func grow(box geom.AABB, reach float64) geom.AABB {
	return geom.AABB{
		TopLeft:     geom.NewVec(box.TopLeft.X-reach, box.TopLeft.Y-reach),
		BottomRight: geom.NewVec(box.BottomRight.X+reach, box.BottomRight.Y+reach),
	}
}

// applySelection tags every hit Selectable entity player by owns Selected and, unless additive,
// untags the rest of its own.
func (s *SelectionSystem) applySelection(hit map[uid.UID64]struct{}, additive bool, by control.PlayerID) {
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

// ownersAt is the owners of the i-th entity of a chunk whose owners are owners; none without them.
func ownersAt(owners []tag.Tags[owner.Family], i int) tag.Tags[owner.Family] {
	if owners == nil {
		return 0
	}
	return owners[i]
}
