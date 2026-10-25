package order

import (
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

// MaxWaypoints bounds the goals queued behind a MoveOrder's Target.
const MaxWaypoints = 8

// MoveOrder commands an entity to path toward Target, then through each queued goal in turn; it
// is removed once the last is reached.
type MoveOrder struct {
	Target cell.ID
	// Level is which of Target's levels the order ends on.
	Level Level
	// Spot is where in Target the entity's centre stops, zero the centre; At is the point the order
	// was given for, round which a group stands.
	Spot, At  geom.Vec
	Waypoints [MaxWaypoints]Goal
	Queued    uint8
	Path      Path
	Leg       Leg
	// Ask is what the order asks of the routing for its Path, or what the routing answered.
	Ask    Ask
	Waited time.Duration
	// Retry is how long more the entity waits before it runs again, from the same cell and round
	// the same cells, a search for a route that found nothing; Fails is how many did in a row.
	Retry time.Duration
	Fails uint8
	// Bumped says the entity struck someone since the last tick; Cooldown is how long it then
	// keeps its new route before it would react to a bump again.
	Bumped   bool
	Cooldown time.Duration
	// Struck is the way off whoever the entity struck last, Hit who that was when HitUnit, Aside
	// the way it steps round them for AsideFor more; Met are the last Mets it struck standing,
	// Avoid the first Avoids cells its routes go round; Stalled is how long it has come no nearer
	// Toward than Closest, or had its step into Toward refused, Held once for too long, Stalls how
	// often it stalled since it last reached one.
	Struck, Aside geom.Vec
	Hit           uid.UID64
	HitUnit       bool
	AsideFor      time.Duration
	Met           [4]uid.UID64
	Mets          uint8
	Avoid         [4]cell.ID
	Avoids        uint8
	Stalled       time.Duration
	Toward        cell.ID
	Closest       float32
	Stalls        uint8
	Held          bool
	// Linger is how long the entity stands on its Target before it goes on to the next queued goal;
	// zero passes it. GivingWay marks an order to give way, whose Target is no goal.
	Linger    time.Duration
	GivingWay bool
	// Holding is how long more a Hold keeps the entity where it stands; WaitedOut says one ran
	// out, Cornered that a Detour found no way round, both till its next step.
	Holding   time.Duration
	WaitedOut bool
	Cornered  bool
	// Face is the world point the entity turns towards once it has arrived; zero turns it nowhere.
	Face geom.Vec
	// Group is the MoveTo that gave the order, one number to every unit it sent; zero, none.
	Group uint32
}

// Ask is a MoveOrder's request to the routing, which answers it the step it is made, or its
// answer.
type Ask uint8

const (
	AskNone    Ask = iota
	AskRoute       // a route to the Target
	AskNearest     // the nearest free cell round the Target, with a route there
	NoRoute        // the routing found no route to the Target
	NoPlace        // the routing found no free cell round the Target
)

// Goal is a goal queued behind a MoveOrder's Target: its Cell, where in it the entity stops (Spot,
// zero the centre), the point it was given for and the level it lies on.
type Goal struct {
	Cell     cell.ID
	Spot, At geom.Vec
	Level    Level
}

// Enqueue adds a goal after the last queued one; false when the queue is full.
func (m *MoveOrder) Enqueue(g Goal) bool {
	if int(m.Queued) >= MaxWaypoints {
		return false
	}
	m.Waypoints[m.Queued] = g
	m.Queued++
	return true
}

// Advance takes the first queued goal as m's Target; false with none queued.
func Advance(m *MoveOrder) bool {
	if m.Queued == 0 {
		return false
	}
	g := m.Waypoints[0]
	m.Target, m.Spot, m.At, m.Level = g.Cell, g.Spot, g.At, g.Level
	copy(m.Waypoints[:], m.Waypoints[1:m.Queued])
	m.Queued--
	m.Path = Path{}
	Found(m)
	return true
}

// the wait after the first search in a row that found nothing, the second, the third and on
var retryAfter = [...]time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}

// Failed counts a search for m's route that found nothing and sets the wait before the next.
func Failed(m *MoveOrder) {
	m.Fails = min(m.Fails+1, uint8(len(retryAfter)))
	m.Retry = retryAfter[m.Fails-1]
}

// Found clears m's waits after searches that found nothing.
func Found(m *MoveOrder) { m.Retry, m.Fails = 0, 0 }

// Meet notes id among those m's unit struck standing, the oldest let go first.
func Meet(m *MoveOrder, id uid.UID64) {
	for _, known := range m.Met[:m.Mets] {
		if known == id {
			return
		}
	}
	if int(m.Mets) < len(m.Met) {
		m.Met[m.Mets] = id
		m.Mets++
		return
	}
	copy(m.Met[:], m.Met[1:])
	m.Met[len(m.Met)-1] = id
}

// Learn notes c among the cells m's routes go round, the oldest let go first.
func Learn(m *MoveOrder, c cell.ID) {
	for _, known := range m.Avoid[:m.Avoids] {
		if known == c {
			return
		}
	}
	m.Retry = 0 // round other cells, another search
	if int(m.Avoids) < len(m.Avoid) {
		m.Avoid[m.Avoids] = c
		m.Avoids++
		return
	}
	copy(m.Avoid[:], m.Avoid[1:])
	m.Avoid[len(m.Avoid)-1] = c
}

// Leg is the single step an entity is travelling, its cells held in the occupancy until it
// reaches To's centre; a slantwise step along a deck (Decked) holds no corner.
type Leg struct {
	From, To         cell.ID
	C1, C2           cell.ID
	FromDeck, ToDeck bool
	Diagonal         bool
	Decked           bool
	Active           bool
}

// Spot is a cell held on one of its levels, its raised deck or its ground.
type Spot struct {
	C    cell.ID
	Deck bool
}

// Cells are the cells l steps through, the first n.
func Cells(l Leg) (cells [4]cell.ID, n int) {
	if l.Diagonal {
		return [4]cell.ID{l.From, l.To, l.C1, l.C2}, 4
	}
	return [4]cell.ID{l.From, l.To}, 2
}

// Covers reports whether l steps through c.
func Covers(l Leg, c cell.ID) bool {
	cells, n := Cells(l)
	return slices.Contains(cells[:n], c)
}

// Held is what l holds in the occupancy, the first n.
func Held(l Leg) (held [4]Spot, n int) {
	held = [4]Spot{{l.From, l.FromDeck}, {l.To, l.ToDeck}, {l.C1, false}, {l.C2, false}}
	if l.Diagonal && !l.Decked {
		return held, 4
	}
	return held, 2
}
