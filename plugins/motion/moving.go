package motion

import (
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Moving is the moment of every entity in the velocity pass, every step: a rule of it scales
// Base.Vel.Value to slow or stop the entity. Course is the steering's, nil for an entity with
// none.
type Moving struct {
	ID     uid.UID64
	Base   *world.Base
	Course *steering.Course
}

// Who is the entity moving.
func (m Moving) Who() uid.UID64 { return m.ID }

// Still is the word of an entity that neither walks nor strafes this step.
func Still() cond.Word[Moving] {
	return cond.NewWord("still", func(m Moving) bool {
		return m.Base.Vel.Value == 0 && m.Base.Vel.Side == 0
	})
}

// Braking is the word of an entity slowing down: still going, its course asking for less speed
// than it has.
func Braking() cond.Word[Moving] {
	return cond.NewWord("braking", func(m Moving) bool {
		return m.Course != nil && m.Course.Speed != 0 &&
			f32.Abs(m.Course.WantSpeed) < f32.Abs(m.Course.Speed)
	})
}

var (
	_ host.About = Moving{}
)
