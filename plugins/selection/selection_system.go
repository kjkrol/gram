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
// where the world's Look draws them through the command's camera. After the Selects, an Apply puts
// its effect on what the player has selected.
type SelectionSystem struct {
	selects *control.Queue[Select]
	applies *control.Queue[Apply] // when the plugin wires them
	effects *effect.Effects       // the world's, which Apply casts
	space   *aabbworld.Space
	tags    Tags

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
	if s.applies != nil {
		s.applies.Drain(func(i control.Issued[Apply]) {
			if i.Command.Effect != (effect.Effect{}) {
				s.eachSelected(i.Player, i.Command.Only, func(id uid.UID64) { s.effects.Cast(cb, id, i.Command.Effect) })
			}
		})
	}
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
