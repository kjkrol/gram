package main

const (
	GrassCell = "grass"
	PitCell   = "pit"
	FenceCell = "fence"

	OpenEf         = "open"
	AjarEf         = "ajar"
	ReachingEf     = "reaching" // the hero means to pull, for a second
	PullingEf      = "pulling"  // the hero's hands on the lever, the swing long
	InReachEf      = "in reach" // a lever at hand
	StillEf        = "still"    // standing, not yet bored
	BoredEf        = "bored"    // standing long: the stages of boredom go round
	BlinkingEf     = "blinking"
	StampingEf     = "stamping"     // the stage of stamping now and then
	StampEf        = "stamp"        // one series: tup-tup-tup
	LookingEf      = "looking"      // turning to look about
	LookingLongEf  = "looking long" // a look held ten seconds
	BoredPlan      = "bored"
	BrakingEf      = "braking"       // slowing to a stop
	LeverPulledEf  = "lever pulled"  // the lever's lower position; none is the upper
	LeverPullingEf = "lever pulling" // the arm going down
	LeverPushingEf = "lever pushing" // the arm going up

	PlateRole  = "plate"
	LeverRole  = "lever"
	HandyRole  = "handy"
	MortalRole = "mortal"
	HeroRole   = "hero"

	SwitchWestCmd  = "switch west"
	OpenEastCmd    = "open east"
	FlipTheGateCmd = "flip the gate"
	PullCmd        = "pull"

	HeroKind     = "hero"
	TrapdoorKind = "trapdoor"
	GateKind     = "gate"
	PlateKind    = "plate"
	LeverKind    = "lever"

	HeroName = "hero"

	PlayerSide = "player"

	WireStage = "wire-demo"
	MainScene = "main"
)
