package main

import "github.com/kjkrol/gram/plugins/board/cell"

// HighlandDomain is the plateau's units' own way of moving: they climb where the lowland
// walkers turn back (mobility.go).
const HighlandDomain cell.Domain = cell.Air << 1

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
	BuiltCell  = "built"
	IceCell    = "ice"

	BloodMoonEf    = "blood moon"
	WindwardRainEf = "windward rain"
	GatheringEf    = "gathering"
	SilhouetteEf   = "silhouette"

	MortalRole     = "mortal"
	LunarRole      = "lunar"
	UplandRole     = "upland"
	ForestRole     = "forest"
	SettlementRole = "settlement"
	HouseRole      = "house"
	LampRole       = "lamp"
	CaravanRole    = "caravan"
	CarrierRole    = "carrier"
	BandRole       = "band"
	MemberRole     = "member"

	CaravanPlan   = "caravan"
	TravellerPlan = "traveller"

	BleedCmd      = "bleed"
	SilhouetteCmd = "silhouette"

	UnitKind      = "unit"
	RivalKind     = "rival"
	HawkKind      = "hawk"
	PlateauKind   = "plateau"
	LookoutKind   = "lookout"
	CaravanKind   = "caravan"
	WagonKind     = "wagon"
	PorterKind    = "porter"
	TravellerKind = "traveller"
	BandKind      = "band"

	BridgeKind     = "bridge"
	FordKind       = "ford"
	LanternKind    = "lantern"
	ForestKind     = "forest"
	LaurelKind     = "laurel"
	PineKind       = "pine"
	HeatherKind    = "heather"
	SettlementKind = "settlement"
	HouseKind      = "house"
	LampKind       = "lamp"

	PlayerSide  = "player"
	RivalSide   = "rival"
	ReserveSide = "reserve"

	BoardTopographyStage = "board-topography"
	MainScene            = "main"

	NaturalLook = "natural"
	GhibliLook  = "ghibli"

	BarHUD     = "bar"
	WidgetsHUD = "widgets"

	// SheetsVersion is how the fields are planted: a change to the forests', the settlements' or
	// their parts' rules or looks changes it, or the sheets kept from an earlier run show the old.
	SheetsVersion = "madeira-3"
	// RoutesVersion is how the steps are priced: a change to the mobility, the kinds' paces or
	// the unit kinds' bodies changes it, or the hierarchy kept from an earlier run climbs the
	// old prices.
	RoutesVersion = "madeira-1"
)

func snowyCell(kind string) string { return "snowy " + kind }
