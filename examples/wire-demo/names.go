package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	GrassCell   = "grass"
	BoardsCell  = "boards"
	PitCell     = "pit"
	PlateCell   = "plate"
	LeverCell   = "lever"
	FenceCell   = "fence"
	GateCell    = "gate"
	GatewayCell = "gateway"

	// effects
	OpenEf  = "open"
	AjarEf  = "ajar"
	HasteEf = "haste"
	PullEf  = "pull"

	// roles
	PlateRole  = "plate"
	LeverRole  = "lever"
	HastyRole  = "hasty"
	HandyRole  = "handy"
	MortalRole = "mortal"

	// commands
	OpenWestCmd    = "open west"
	OpenEastCmd    = "open east"
	FlipTheGateCmd = "flip the gate"
	HastenCmd      = "hasten"
	ReachCmd       = "reach"

	// kinds of units
	ScoutKind    = "scout"
	PorterKind   = "porter"
	WandererKind = "wanderer"
)
