package world

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/fact"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/uid"
)

type moments struct {
	clock *clock.Clock
	host  host.StepRules[clock.Moment]
	tick  host.TickSource
	last  time.Duration
	begun bool
}

func (m *moments) system() goke.System {
	return goke.SystemFn{OnInit: func(*goke.SysInit) { m.host.Bind() }, OnUpdate: func(cb *goke.CmdBuf, d time.Duration) {
		now := m.clock.Read().Time() + d
		if !m.begun {
			m.last, m.begun = m.clock.Read().Time(), true
		}
		if !m.host.Empty() {
			m.host.Run(m.tick.Of(d), clock.Moment{Last: m.last, Now: now, Clock: m.clock.Read().Entity()})
		}
		m.last = now
	}}
}

// Unfold is the moment a stretch of an aggregate comes down finer than Strategic, once as it comes:
// the time its rules spawn the parts lying there (SpawnPart), a point aggregate's round At, its
// centre. It is told also as a preview, of a field's stretch a sheet draws holding none of its
// parts: the parts its rules give there are photographed into the sheet, none spawned.
type Unfold struct {
	ID uid.UID64
	At geom.Vec
	// Sector is the stretch's sector, a point aggregate's the one holding its centre.
	Sector int
	// Box is the aggregate within that sector: a field's part there, a point aggregate's own box.
	Box     geom.AABB
	stretch uid.UID64
}

// Who is the aggregate unfolding.
func (u Unfold) Who() uid.UID64 { return u.ID }

// Subject is the stretch unfolding, which a SpawnPart given under it is aimed at to lie there.
func (u Unfold) Subject() (uid.UID64, bool) { return u.stretch, true }

// Fold is the moment a stretch of an aggregate has lain at Strategic or coarser for the ladder's
// Fold seconds, once after its Unfold: the time its rules sum its Parts into it, each told a
// Folding.
type Fold struct {
	ID    uid.UID64
	Parts int
	// Sector and Box are the stretch's, as its Unfold told them.
	Sector int
	Box    geom.AABB
}

// Who is the aggregate folding.
func (f Fold) Who() uid.UID64 { return f.ID }

// Parts is the word of a stretch of an aggregate folding with n parts or more.
func Parts(n int) cond.Word[Fold] {
	return cond.NewWord("parts", func(f Fold) bool { return f.Parts >= n })
}

// Folding is the moment told each part of a stretch of an aggregate as it folds, the step of its
// Fold: the time the part's rules sum it into its Whole, the moment's subject, and despawn it.
type Folding struct {
	ID, Whole uid.UID64
	// Sector and Box are the stretch's, a field's part folding with the sector holding its centre.
	Sector int
	Box    geom.AABB
}

// Who is the part folded in.
func (f Folding) Who() uid.UID64 { return f.ID }

// Subject is the aggregate folding, whom the part's steps of Other and an Aimed command are of.
func (f Folding) Subject() (uid.UID64, bool) { return f.Whole, true }

// Met is the moment two aggregates travelling the abstract graph (sector.Abstract) come to
// overlap, as seen from A: once a pair till they have come apart.
type Met struct {
	A, B uid.UID64
}

// Who is the aggregate meeting the other.
func (m Met) Who() uid.UID64 { return m.A }

// Whom tells each the other one.
func (m Met) Whom(each func(uid.UID64)) { each(m.B) }

var (
	_ host.About   = Unfold{}
	_ fact.Subject = Unfold{}
	_ host.About   = Fold{}
	_ host.About   = Folding{}
	_ fact.Subject = Folding{}
	_ host.Met     = Met{}
)
