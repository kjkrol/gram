package roads

import (
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/plugins/roads/site"
	"github.com/kjkrol/uid"
)

// Estimated is the moment of a road planned in a player's hand, every tick the plan stands, of
// the plugin's own entity: Estimate what the road takes, Price what the Tariff makes of it (0
// without one). A rule of it may Refuse the road.
type Estimated struct {
	Self     uid.UID64
	Estimate site.Estimate
	Price    float32
}

// Who is the plugin's entity.
func (e Estimated) Who() uid.UID64 { return e.Self }

// Levels is the word of a plan moving the ground's corners.
func Levels() cond.Word[Estimated] {
	return cond.NewWord("levels", func(e Estimated) bool { return e.Estimate.Levels() })
}

// Blocks is the word of a plan whose levelling waits for something standing on a cell to go.
func Blocks() cond.Word[Estimated] {
	return cond.NewWord("blocks", func(e Estimated) bool { return len(e.Estimate.Blocked) > 0 })
}

// Crosses is the word of a plan meeting a road standing.
func Crosses() cond.Word[Estimated] {
	return cond.NewWord("crosses", func(e Estimated) bool { return len(e.Estimate.Crossings) > 0 })
}

// Decked is the word of a plan running over a bridge's deck.
func Decked() cond.Word[Estimated] {
	return cond.NewWord("decked", func(e Estimated) bool { return len(e.Estimate.Decks) > 0 })
}

// Longer is the word of a plan of more than cells cells.
func Longer(cells int) cond.Word[Estimated] {
	return cond.NewWord("longer", func(e Estimated) bool { return len(e.Estimate.Cells) > cells })
}

// Moves is the word of a plan moving more earth than earth, cut and fill together.
func Moves(earth float32) cond.Word[Estimated] {
	return cond.NewWord("moves", func(e Estimated) bool {
		return e.Estimate.Cut+e.Estimate.Fill > earth
	})
}

// Built is the moment of a road built, of the plugin's own entity, once as it is laid: what was
// laid and what it cost.
type Built struct {
	Self     uid.UID64
	Estimate site.Estimate
	Price    float32
}

// Who is the plugin's entity.
func (b Built) Who() uid.UID64 { return b.Self }

// Levelled is the word of a road built that moved the ground's corners.
func Levelled() cond.Word[Built] {
	return cond.NewWord("levelled", func(b Built) bool { return b.Estimate.Levels() })
}

// Crossed is the word of a road built that met a road standing.
func Crossed() cond.Word[Built] {
	return cond.NewWord("crossed", func(b Built) bool { return len(b.Estimate.Crossings) > 0 })
}

var (
	_ host.About = Estimated{}
	_ host.About = Built{}
)
