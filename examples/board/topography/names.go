package main

const (
	WaterCell  = "water"
	EarthCell  = "earth"
	SandCell   = "sand"
	RockCell   = "rock"
	ForestCell = "forest"
	BrookCell  = "brook"
	StreamCell = "stream"
	RiverCell  = "river"
	RoadCell   = "road"
	IceCell    = "ice"

	BloodMoonEf = "blood moon"

	MortalRole = "mortal"
	LunarRole  = "lunar"

	BleedCmd = "bleed"

	UnitKind    = "unit"
	RivalKind   = "rival"
	HawkKind    = "hawk"
	PlateauKind = "plateau"

	BridgeKind = "bridge"
	FordKind   = "ford"

	PlayerSide  = "player"
	RivalSide   = "rival"
	ReserveSide = "reserve"

	BoardTopographyStage = "board-topography"
	MainScene            = "main"
)

func snowyCell(kind string) string { return "snowy " + kind }
