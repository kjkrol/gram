package collision

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/uid"
)

// Meeting is one confirmed contact as seen from Self: who it met, the impulse exchanged
// (zero when only detected), the way Self left Other, the way Self faces and the way from its
// middle to Other's.
type Meeting struct {
	Self, Other uid.UID64
	Impact      float64
	Normal      geom.Vec
	Heading     geom.Vec
	Toward      geom.Vec
}

// Facing is the word of a meeting in which Self is turned towards Other, within the angle in
// radians of the way to it.
func Facing(within float64) cond.Word[Meeting] {
	return cond.NewWord("facing", func(m Meeting) bool {
		return m.Heading.X*m.Toward.X+m.Heading.Y*m.Toward.Y >= math.Cos(within)
	})
}

// Who is the entity that met the other.
func (m Meeting) Who() uid.UID64 { return m.Self }

// Whom tells each the other one.
func (m Meeting) Whom(each func(uid.UID64)) { each(m.Other) }

var (
	_ host.Met = Meeting{}
)
