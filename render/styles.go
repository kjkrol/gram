package render

import "reflect"

// MaxSprites is how many sprites an atlas holds: the range of SpriteID.
const MaxSprites = 1 << 16

// Style is how a plugin draws about a kind of entity (an outline, a cone, a route): a value of
// the plugin's own type, declared on the kind's slot of the world's atlas by a figure's Look. A
// type is one by its method Style(), a mark never called, confirmed beside the type.
type Style interface{ Style() }

// Styles keeps the styles declared on an atlas, one of each type a sprite at most; StyleOf reads
// them by type.
type Styles struct {
	by map[reflect.Type][]Style // by sprite, as far as the last one styled
}

// Set declares st for the sprite id, in place of one of its type declared before.
func (s *Styles) Set(id SpriteID, st Style) {
	if s.by == nil {
		s.by = map[reflect.Type][]Style{}
	}
	t := reflect.TypeOf(st)
	table := s.by[t]
	if int(id) >= len(table) {
		table = append(table, make([]Style, int(id)+1-len(table))...)
	}
	table[id] = st
	s.by[t] = table
}

// StyleOf is the style of type S declared for the sprite id, false where none was; a nil Styles
// has none.
func StyleOf[S Style](s *Styles, id SpriteID) (S, bool) {
	var none S
	if s == nil || s.by == nil {
		return none, false
	}
	table := s.by[reflect.TypeOf(none)]
	if int(id) >= len(table) || table[id] == nil {
		return none, false
	}
	return table[id].(S), true
}
