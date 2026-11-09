package world

import (
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// a stretch of a field previewed for the sheets: told its Unfold as though it unfolded, the parts
// its rules give there photographed as they would be spawned, none spawned, then kept by the
// sheets as a fold keeps them
type preview struct {
	id    uid.UID64
	at    place
	box   geom.AABB
	shots []shot
}

// the ids of previews, past every index an entity's id takes, so a Seek of one finds none
const (
	previewIDs  = uid.UID64(0xFFFF0000)
	previewMask = uid.UID64(0xFFFF)
)

// isPreview reports whether id is a preview's
func isPreview(id uid.UID64) bool { return id&^previewMask == previewIDs }

// has the walk say whether the sheets are photographed and of which atlas
func (s *wholes) sheeting() {
	r := s.w.p.renderer
	s.walk.sheets = r.sheet.albedo != nil && r.atlas != nil
	s.walk.atlas, _ = r.atlas.(*Atlas)
	if s.walk.sheets {
		r.sheet.bare = r.sheet.bare[:0]
	}
}

// keptBy reports whether the sheets keep the parts of the i-th field of a block whose looks are
// looks, nil for none
func (s *wholes) keptBy(looks []render.Appearance, i int) bool {
	if looks == nil {
		return true
	}
	return s.kept(&looks[i])
}

// kept reports whether the sheets keep the parts of a field of look, nil for none: its kind
// declares no look of its own at L4
func (s *wholes) kept(look *render.Appearance) bool {
	wa := s.walk.atlas
	return wa == nil || look == nil || !wa.declares(look.SpriteID, lod.Strategic)
}

// bare notes for the sheets the stretch of the field whole over p, coarser than L3 with none
// standing, where they keep none of its parts, and previews it this step where the sheets wait
// for it, perFrame a step at most
func (s *wholes) bare(whole uid.UID64, p sector.Part) {
	sheet := &s.w.p.renderer.sheet
	at := place{whole: whole, sector: p.Sector}
	if _, ok := sheet.folded[at]; ok {
		return
	}
	sheet.bare = append(sheet.bare, at)
	if sheet.pending[p.Sector] && len(s.previewing) < sheet.perFrame {
		s.previewing = append(s.previewing, piece{whole: whole, sector: p.Sector, box: p.Box})
	}
}

// has a preview of each previewing, of an id of the world's own its Unfold's subject
func (s *wholes) preview() {
	for k := range s.previewing {
		p := &s.previewing[k]
		s.previewed = previewIDs | (s.previewed+1)&previewMask
		p.stretch = s.previewed
		// a slot of a past step keeps its shots
		n := len(s.previews)
		s.previews = slices.Grow(s.previews, 1)[:n+1]
		v := &s.previews[n]
		v.id, v.at, v.box = p.stretch, place{whole: p.whole, sector: p.sector}, p.box
		v.shots = v.shots[:0]
	}
}

// previewOf is the preview id, nil for none
func (s *wholes) previewOf(id uid.UID64) *preview {
	for k := range s.previews {
		if s.previews[k].id == id {
			return &s.previews[k]
		}
	}
	return nil
}

// has the sheets keep the parts each preview photographed, none too, its sector photographed
// with them, and lets the previews go
func (s *wholes) foldPreviews() {
	sheet := &s.w.p.renderer.sheet
	for k := range s.previews {
		p := &s.previews[k]
		sheet.fold(p.at, p.shots)
		sheet.pending[p.at.sector] = false
	}
	s.previews = s.previews[:0]
}

// waiting has pending say, by sector, whether its photograph waits for a preview: a view draws it
// at L4 or coarser, its photograph lags, and a stretch of a field bare to the sheets lies there
func (s *sheets) waiting() {
	clear(s.pending)
	for _, at := range s.bare {
		i := at.sector
		if !s.wanted[i] || s.painted[i] == s.version[i] || s.pending[i] {
			continue
		}
		if _, ok := s.folded[at]; !ok {
			s.pending[i] = true
		}
	}
}

// shoot has the preview a part aimed at, given by whole, hold the part's shot as the sheets
// would photograph it spawned, laid off the middle of the preview's box and on the ground under
// its middle: one refused as a spawn is, a full world aside, is an error; a preview gone or
// another's holds none
func (s *spawnSystem) shoot(whole uid.UID64, cmd SpawnPart) error {
	p := s.w.wholes.previewOf(cmd.in)
	if p == nil || p.at.whole != whole {
		return nil
	}
	by, box, err := s.checked(cmd.Entry,
		placing{whole: whole, at: middle(p.box).Add(cmd.At), part: true})
	if err != nil {
		return err
	}
	r := s.w.p.renderer
	if r.atlas == nil {
		return nil
	}
	wa, _ := r.atlas.(*Atlas)
	var levels byLevel
	if wa != nil {
		levels = wa.levels
	}
	l3, ok := levels.at(by.kind.SpriteID, lod.L3)
	if !ok || !r.photographable(wa, l3) {
		return nil
	}
	row := cmd.Entry.Row()
	var z Z
	if by.kind.HasZ {
		z = by.kind.Z.Resolve(row)
		z.Altitude = s.w.groundAt(Position{AABB: box}.Center())
	}
	var lie *Lie
	var lying Lie
	if by.lies {
		if lying = by.lie.Resolve(row); lying.Length > 0 {
			lie = &lying
		}
	}
	n := len(p.shots)
	p.shots = slices.Grow(p.shots, 1)[:n+1]
	r.photographed(&p.shots[n], box, by.kind.Velocity.Resolve(row).Dir, lie, z, l3)
	return nil
}
