// Package bullet fires shots and flies them: entities of the world a Shoot spawns at a unit's
// muzzle — a kind of the world like any other, drawn from its sprite — flown by the plugin every
// step of the simulation past the world's step cap and swept by collision, so what lies on the
// path is struck, until the flight ends: a Landing for the rules, the shot gone unless it Lands,
// resting then until a Burst. A weapon — which ammo, how often, who carries it — is the game's.
//
// # Body, Shots and Ammo
//
// [Body] is what a kind of shot is: its Size, Speed, Range, its Gravity if thrown and whether it
// Lands. [Shots] defines the kinds on the world's kinds — NewShots(w).Define(name, body, extra...)
// is an [Ammo], what a Shoot names — each a [Shot] row the plugin builds as it fires. A shot
// carries a collision.Collider without Physics (a sensor: detected, never pushed) and a
// collision.Sweep that ignores its shooter, its Body, its [Flight], its shooter's owners (a rule
// tells friendly fire by them) and, in a world with heights, a world.Z; extra may add tags and
// Layers. The game must use collision: without it nothing is struck.
//
// # Shoot
//
// [Shoot] is the command: by a player, from every Selected unit it owns (the Selected tag is
// selection's, given to [NewPlugin]); by an entity, through Order in a rule or a plan, from
// itself. The shot goes towards At when Targeted, towards the moment's Subject when the command
// is aimed (plugin.Aimed: a Sighting's nearest seen), else the way the shooter faces (its
// Velocity's Dir); from a muzzle just outside the shooter's box, at the height of its world.Eye,
// else the middle of its Z, else 0. A thrown shot (Gravity) gets the climb that brings it down
// where it goes to — At, no further than its Range — on the ground the plugin was given
// ([Plugin.WithGround]; 0 without). The shot is spawned through world.Spawn at the next step.
//
// # Flight
//
// Every step, before the world moves, the flight system flies each shot its speed along its way,
// no further than its range, writing its Sweep's From and its box, and a thrown one's Z; collision
// then pairs the whole stretch and keeps the nearest contact. Where the step reached the shot's
// Range, the ground (a thrown one), a closed edge or an open one, the Flight's Ending says so, and
// the shot lands at the next step — once collision has looked at that last step — unless a contact
// came first: then it lands just short of what it struck (Struck, with Other; Wall, with Cell).
// A landed shot loses its Sweep and Collider; one whose Body does not Land lies a step more, for
// the effects its landing cast, and is gone; one flown out by an open edge is marked
// world.Outside, the world's leaving rules told, else despawned by the world. A shot's Base.Vel
// stays zero: the plugin moves it. Spawning and landing are a step apart each, so a shot hits no
// sooner than the second step after the Shoot.
//
// # Landing, Resting and Blast
//
// [Landing] is told once, as a flight ends: where, whom it Struck (its Subject, and whom its
// ForOther acts on), which Wall, whether it came down (Grounded, its Range flown too) or Left.
// [Resting] is told every step for a landed shot that Lands; [Burst] is the command a resting
// shot gives itself (Order in a rule of its Resting, say under an effect a fuse's Then cast) and
// [Blast] what every entity within the Burst's Radius is to the pair rules, with its Distance
// from the shot; the shot is gone. They are rules of the roles the shots play. The Meeting of a
// shot and what it struck is collision's, for its rules — a shot striking is
// detected, never pushed, so what it struck feels no push either.
//
// # A weapon is the game's
//
// What fires how and when is written in rules and effects over this vocabulary: a reloading
// effect on the unit, a binding or a rule that Shoots Unless it; a fuse an effect on the
// grenade, Then bang, a rule of Resting under bang that Bursts; a wounded effect a Blast rule
// applies ForOther. See examples/bullet-demo.
//
// # Limits
//
// A wrapping world is refused (a sweep across the seam). Shots do not penetrate: the nearest
// contact ends the flight. A landed shot touches nothing: a mine that feels a tread is a rule
// of the board's cell.Now, not of the shot.
package bullet
