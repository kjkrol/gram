// Package roads lays roads by one set of rules, package site's, with a road in a player's hand:
// planned, priced, asked about and then built.
//
// [NewPlugin] takes the world, the board and the [Config]: what the game allows a road
// (site.Rules), the kind of road the tool lays and how wide, the Tariff pricing a plan, and
// whether the tool levels the ground and holds the game in the tactical pause. A road is a
// cell.Way laid cell after cell (board.BoardWrite.SetWay), a Graded kind lying as Transport
// Tycoon lays it, a track as the ground lies.
//
// # A road in a player's hand
//
// [Plugin.DefaultBindings] have Shift+R take a road in hand ([Tool]); while the player holds it,
// the cursor aims the road ([Aim]), a left click marks a point of its route ([Mark]: the first
// where it begins, every next one it runs to by the cheapest way, a point no way reaches not
// marked, the last again taken back), a left drag with Shift lays one piece in a cell toward a
// neighbour ([Piece], joining the roads ending at the cell), and a right click or Esc puts the
// road down. The plugin draws the road in hand over the ground as the levelling will leave it,
// green where it may lie and yellow where it may not ([Plugin.Reason] says why), the cells its
// levelling moves outlined and those something stands on marked in red. [Plugin.Window] asks
// "Build the road?" with what the road takes and what it costs: [Build] plans it anew (a plan
// the ground changed under is shown again and waits for a second Build, one levelling under a
// unit waits for the unit to go) and has it built at the next step of the simulation: every
// cell graded on the ground [Plugin.WithShaper] gives, then the ways laid, none on a bridge's
// span or a ford, which the road leads onto, a head's leading onto the span; a road along a road
// standing lays and costs nothing anew; [Cancel] builds nothing. Whether the tool levels the ground
// is the players' setting ([Levelling], the settings screen's); Config.MaxStep is the
// topography's Shaping.MaxStep, how far the ground round a graded cell follows; Config.InPause
// has taking the road in hand hold the game in the tactical pause, the players letting it go.
//
// # Rules of the game
//
// The price is the game's own work: Config.Tariff turns an [site.Estimate] into a number. The
// decision is its rules': every tick a plan stands in a hand the plugin's entity has the moment
// [Estimated], at once, in the tactical pause too, and a rule of a role the entity plays may
// answer [Refuse] with why; every road built has the moment [Built]. A game's own pass reads
// what was built from [Read.Built]. The roads confirmed and waiting for a step
// (Resources.Orders) and the receipts not handed out yet (Resources.Receipts) are saved with the
// game beside the Settings: a road confirmed in the pause is built, and paid for, after a load;
// a road in hand is not saved.
//
// # Vocabulary
//
// Moments, the plugin's entity's: [Estimated] (words [Levels], [Blocks], [Crosses], [Decked],
// [Longer], [Moves]) and [Built] (words [Levelled], [Crossed]). Commands, a player's with the
// road tool in hand: [Aim], [Mark], [Piece], [Build], [Cancel]; a rule's answer to Estimated:
// [Refuse]; the settings screen's: [Levelling].
package roads
