package weathering

import (
	"fmt"
	"time"

	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

// Overhead is the moment a cell hears once a second of game time: the air over it (Air), its low
// clouds' cover (Low), how high its ground stands in their base there (Reach, 1 at it) and the
// time of day (Hour), about the cell's entity.
type Overhead struct {
	ID    uid.UID64
	Cell  cell.ID
	Air   air.Mass
	Low   float32
	Reach float32
	Hour  time.Duration
}

// Who is the cell's entity.
func (o Overhead) Who() uid.UID64 { return o.ID }

var _ host.About = Overhead{}

// Clouded is the word of a cell under low clouds covering at least at of its sky.
func Clouded(at float32) cond.Word[Overhead] {
	return cond.NewWord(fmt.Sprintf("clouded %v", at), func(o Overhead) bool { return o.Low >= at })
}

// Damp is the word of a cell under air at least at damp.
func Damp(at float32) cond.Word[Overhead] {
	return cond.NewWord(fmt.Sprintf("damp %v", at),
		func(o Overhead) bool { return o.Air.Damp >= at })
}

// Reaches is the word of a cell whose ground stands at least at of the way up to the low clouds'
// base over it: 1, into them.
func Reaches(at float32) cond.Word[Overhead] {
	return cond.NewWord(fmt.Sprintf("reaches %v", at),
		func(o Overhead) bool { return o.Reach >= at })
}

// Hours is the word of the time of day from up to to, round midnight when to comes first.
func Hours(from, to time.Duration) cond.Word[Overhead] {
	return cond.NewWord(fmt.Sprintf("from %v to %v", from, to), func(o Overhead) bool {
		if from <= to {
			return o.Hour >= from && o.Hour < to
		}
		return o.Hour >= from || o.Hour < to
	})
}

// Wring is a knob on every cell of the board: how much of the low clouds over it falls (Falls, 0
// to 1) and how much of the air's damp over it gathers into low clouds a second (Gathers), none
// but as an effect turns it.
type Wring struct{ Falls, Gathers float32 }

// Knob: an effect may turn it (effect.Knob).
func (Wring) Knob() {}

var _ effect.Knob = Wring{}

// Rules takes the rules of an Overhead.
func (w *Weathering) Rules() host.Host { return &w.overheads }
