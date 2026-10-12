package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	MeadowCell = "meadow"

	// kinds of units
	TravellerKind = "traveller"
	HostKind      = "host"

	// roles
	TravellerRole = "traveller"
	HostRole      = "host"

	// what the hosts' answers change in their Regard: then in dialogs/*.yaml
	WarmChange    = "warm"    // likes the traveller 40 more
	PleaseChange  = "please"  // 20 more
	ForgiveChange = "forgive" // 50 more
	IrkChange     = "irk"     // 20 less
	OffendChange  = "offend"  // 60 less

	// when the hosts' answers are offered, by their Regard: if in dialogs/*.yaml
	FriendCond = "friend"
	EnemyCond  = "enemy"

	// sides
	PlayerSide = "player"

	// the stage and its scenes
	DialogStage = "dialog-demo"
	MainScene   = "main"
)
