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
	SilhouetteEf   = "silhouette"

	MortalRole = "mortal"
	LunarRole  = "lunar"
	UplandRole = "upland"

	BleedCmd      = "bleed"
	SilhouetteCmd = "silhouette"

	UnitKind    = "unit"
	RivalKind   = "rival"
	HawkKind    = "hawk"
	PlateauKind = "plateau"

	BridgeKind  = "bridge"
	FordKind    = "ford"
	LanternKind = "lantern"

	PlayerSide  = "player"
	RivalSide   = "rival"
	ReserveSide = "reserve"

	BoardTopographyStage = "board-topography"
	MainScene            = "main"

	NaturalLook = "natural"
	GhibliLook  = "ghibli"
)

func snowyCell(kind string) string { return "snowy " + kind }
