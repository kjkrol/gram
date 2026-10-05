package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	GrassCell   = "grass"
	RoadCell    = "road"
	WaterCell   = "water"
	WallCell    = "wall"
	LowWallCell = "low wall"

	// effects
	WoundedEf = "wounded"
	BangEf    = "bang"
	FuseEf    = "fuse"

	// roles
	MortalRole  = "mortal"
	RoundRole   = "round"
	GrenadeRole = "grenade"

	// kinds of units
	SoldierKind  = "soldier"
	WandererKind = "wanderer"
	RoundKind    = "round"
	GrenadeKind  = "grenade"
)
