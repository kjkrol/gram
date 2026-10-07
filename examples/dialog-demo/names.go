package main

// The names this game defines its things by, each saying what it is: one place, so a name
// mistyped does not compile.
const (
	// kinds of cells
	MeadowCell = "meadow"

	// kinds of units
	TravellerKind = "traveller"
	HostKind      = "host"

	// effects
	GreetingEf = "greeting" // the host has said hello and waits for an answer
	TalkedEf   = "talked"   // the host has been answered: no hello again for a while
	PleasedEf  = "pleased"
	PuzzledEf  = "puzzled"
	OffendedEf = "offended"

	// roles
	TravellerRole = "traveller"
	HostRole      = "host"

	// commands: what the traveller's answers do to the host
	EndGreetingCmd = "end the greeting"
	TalkedCmd      = "talked"
	PleaseCmd      = "please the host"
	PuzzleCmd      = "puzzle the host"
	OffendCmd      = "offend the host"

	// scenes
	MainScene = "main"
)
