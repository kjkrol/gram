package main

import "github.com/kjkrol/gram/plugins/board/cell"

// HighlandDomain is the plateau's units' own way of moving: they climb where the lowland
// walkers turn back (mobility.go).
const HighlandDomain cell.Domain = cell.Air << 1

const (
	WaterCell      = "water"
	EarthCell      = "earth"
	SandCell       = "sand"
	RockCell       = "rock"
	LaurisilvaCell = "laurisilva"
	WoodlandCell   = "woodland"
	PinewoodCell   = "pinewood"
	HeathCell      = "heath"
	FarmlandCell   = "farmland"
	ParkCell       = "park"
	BrookCell      = "brook"
	StreamCell     = "stream"
	RiverCell      = "river"
	RoadCell       = "road"
	BuiltCell      = "built"
	PavedCell      = "paved"
	AsphaltCell    = "asphalt"
	BuildingCell   = "building"
	IceCell        = "ice"

	BloodMoonEf    = "blood moon"
	WindwardRainEf = "windward rain"
	GatheringEf    = "gathering"
	SilhouetteEf   = "silhouette"

	MortalRole     = "mortal"
	LunarRole      = "lunar"
	UplandRole     = "upland"
	LaurisilvaRole = "laurisilva"
	WoodlandRole   = "woodland"
	PinewoodRole   = "pinewood"
	HeathRole      = "heath"
	FarmlandRole   = "farmland"
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
	BeamKind       = "beam"
	ArchKind       = "arch"
	ViaductKind    = "viaduct"
	PontoonKind    = "pontoon"
	TunnelKind     = "tunnel"
	FordKind       = "ford"
	LanternKind    = "lantern"
	GrowthKind     = "growth"
	TilKind        = "til"
	LaurelKind     = "laurel"
	VinhaticoKind  = "vinhatico"
	BarbusanoKind  = "barbusano"
	FaiaKind       = "faia"
	HeatherKind    = "heather"
	CedarKind      = "cedar"
	PineKind       = "pine"
	EucalyptusKind = "eucalyptus"
	AcaciaKind     = "acacia"
	ChestnutKind   = "chestnut"
	DragonKind     = "dragon tree"
	PalmKind       = "palm"
	BananaKind     = "banana"
	VineKind       = "vine"
	JacarandaKind  = "jacaranda"
	PlaneKind      = "plane"
	TipuanaKind    = "tipuana"
	CanaryPalmKind = "canary palm"
	FicusKind      = "ficus"
	SettlementKind = "settlement"
	CottageKind    = "cottage"
	LampKind       = "lamp"

	HouseKind     = "house"
	TownhouseKind = "townhouse"
	VillaKind     = "villa"
	BlockKind     = "block"
	TowerKind     = "tower"
	HotelKind     = "hotel"
	ChurchKind    = "church"
	ChapelKind    = "chapel"
	HallKind      = "hall"
	ShedKind      = "shed"

	PlayerSide  = "player"
	RivalSide   = "rival"
	ReserveSide = "reserve"

	BoardTopographyStage = "board-topography"
	MainScene            = "main"

	MadeiraMap = "madeira"
	FunchalMap = "funchal"

	NaturalLook = "natural"
	GhibliLook  = "ghibli"

	BarHUD     = "bar"
	WidgetsHUD = "widgets"

	// SheetsVersion is how the fields are planted: a change to the forests', the settlements' or
	// their parts' rules or looks changes it, or the sheets kept from an earlier run show the old.
	SheetsVersion = "madeira-13"
	// RoutesVersion is how the steps are priced: a change to the mobility, the kinds' paces or
	// the unit kinds' bodies changes it, or the hierarchy kept from an earlier run climbs the
	// old prices.
	RoutesVersion = "madeira-9"
)

func snowyCell(kind string) string { return "snowy " + kind }
