// Package bullet fires shots and flies them: entities of the world a Shoot spawns at a unit's
// muzzle, flown every step past the world's step cap and swept by collision, so what lies on
// the path is struck. A weapon — which ammo, how often, who carries it — is the game's rules
// and effects.
//
// # Body, Shots and Ammo
//
// [Body] is what a kind of shot is: its Size, Speed, Range, its Gravity if thrown and whether
// it Lands. [Shots] (Plugin.Shots) defines the kinds: Define(name, body, extra...) registers an
// ammo, Named is the [Ammo] a Shoot names. A shot is a kind of the world like any other, drawn
// from its sprite, carrying a Collider without Physics (a sensor), a Sweep ignoring its shooter,
// its Body, its [Flight] and its shooter's owners. The game must use collision: without it
// nothing is struck.
//
// # Shoot
//
// [Shoot] is the command: by a player, from every Selected unit it owns; by an entity, through
// act.Self.Do in a rule or a plan, from itself. The shot goes towards At when Targeted, towards the
// moment's Subject when aimed, else the way the shooter faces. A thrown shot comes down where
// it goes to, on the ground [Plugin.WithGround] gives.
//
// # Landing, Resting and Blast
//
// [Landing] is told once as a flight ends: whom it Struck (its Subject), which Wall, whether it
// came down or Left. [Resting] is told every step for a landed shot that Lands; [Burst] is the
// command such a shot gives itself, and [Blast] what every entity within the Burst's Radius is
// to the pair rules, [Within] its word of the other. A [Flight] is a plan's fact, [Lying] its
// word of a shot come to rest: a fuse. The Meeting of a shot and what it struck is collision's.
//
// # Dressing a shot
//
// A shot is dressed on the world's atlas as any entity: a look Under an effect, and Turning for
// one drawn the way it flies, the sprite authored facing east.
//
// # Limits
//
// A wrapping world is refused. Shots do not penetrate: the nearest contact ends the flight. A
// landed shot touches nothing.
package bullet
