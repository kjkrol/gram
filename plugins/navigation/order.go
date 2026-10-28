package navigation

import "github.com/kjkrol/gram/plugins/navigation/internal/order"

// The order a unit carries, the commands it is given and the facts it is told, the systems' own
// (internal/order).
type (
	MoveOrder = order.MoveOrder
	Goal      = order.Goal
	Leg       = order.Leg
	Path      = order.Path
	Level     = order.Level
	Round     = order.Round
	MoveTo    = order.MoveTo
	LookAt    = order.LookAt
	Onward    = order.Onward
	StepAside = order.StepAside
	Detour    = order.Detour
	Pass      = order.Pass
	Hold      = order.Hold
	Settle    = order.Settle
	Stop      = order.Stop
	Trail     = order.Trail
	Touch     = order.Touch
	Blocked   = order.Blocked
	Arrived   = order.Arrived
	LastOrder = order.LastOrder
)

const (
	MaxWaypoints  = order.MaxWaypoints
	MaxPathLength = order.MaxPathLength
	EitherLevel   = order.EitherLevel
	OnGround      = order.OnGround
	OnDeck        = order.OnDeck
)

// NewRound makes a Round; the rest are the words of a Touch and of a Blocked.
var (
	NewRound         = order.NewRound
	PushedByAlly     = order.PushedByAlly
	ReachedTheGroup  = order.ReachedTheGroup
	InTheWay         = order.InTheWay
	ClearsTheWay     = order.ClearsTheWay
	Blocks           = order.Blocks
	GoalTaken        = order.GoalTaken
	StrangerOnMyGoal = order.StrangerOnMyGoal
	WaitsFirst       = order.WaitsFirst
	NoWayRound       = order.NoWayRound
	WaitedLong       = order.WaitedLong
	WalksMyWay       = order.WalksMyWay
	Groupmate        = order.Groupmate
	Stuck            = order.Stuck
	Cornered         = order.Cornered
	IdleAllyOnMyGoal = order.IdleAllyOnMyGoal
	IdleAlly         = order.IdleAlly
	StrangerOnGoal   = order.StrangerOnGoal
)
