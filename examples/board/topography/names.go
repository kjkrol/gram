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

	BloodMoonEf    = "blood moon"
	WindwardRainEf = "windward rain"
	GatheringEf    = "gathering"

	MortalRole = "mortal"
	LunarRole  = "lunar"
	UplandRole = "upland"

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
