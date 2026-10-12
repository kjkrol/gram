// Package rule is how a game says what its entities do: a [Branch] says when and what — at a
// moment a plugin catches, as a rule a role obeys; over time, as a plan a kind carries — its
// steps casting effects and giving commands. A change that holds is an effect (package
// rule/effect). The package is gram's core, beside entity and clock; the hosts a plugin runs the
// rules of its moments with are package plugin's.
//
//	roles := s.world.Roles()
//	roles.Define(MortalRole, rule.Branch[unit.Standing]{
//		Name: "fall in",
//		When: unit.Standing.Fallen,
//		Do:   rule.Order(world.Despawn{}),
//	})
//	units.Define(ScoutKind, land, profile, rule.Plays(roles.Named(MortalRole)))
//
// A rule is a sentence: who, when, what. Who is the role the rule is given to; when is the
// moment, the type P a Branch is of, and the conditions in that moment's own words; what is the
// steps. Defining registers and hands nothing back: Named is the handle wherever a thing is built
// on, and a game keeps its names as constants.
//
// # Branches
//
// A [Branch] of P is done When its conditions hold: a predicate of P (unit.Standing.Fallen,
// clock.At(dusk)), the entity's state ([Under], [Free], [Playing], [Carrying], [Self]), the
// world's ([During], [While]), the other of a pair ([Other], better said in the moment's word:
// vision.Seeing, collision.Met), several at once ([All]), nothing for always. Its Do is a [Step]:
// [OneOf] the first that does not fail, [Steps] one after another, a Branch asking more, [Apply],
// [Keep], [Dispel] an effect, [Order] a command, [Trigger] the commands the entity sets off,
// [Chance], [Either], [Once], [ForOther] on the other of a pair, [Here] and [Around] on the places
// round. A rule holds no Go code but its predicates and keeps no memory of its own: an effect's
// presence is its memory.
//
// A moment is [plugin.About] one entity, whom the steps act for; one that is [plugin.Placed]
// stands on places of its own, a board's cells, which Here and Around turn a step on. Chance and
// Either draw from the world's seed, the step's game time and the entity, so a load and a replay
// draw alike. Every cast and Dispel of a step lands together in the effects' pass. Order gives a
// command for the entity, the same a player gives; one that is [plugin.Aimed] is told the
// [plugin.Subject] of the moment. A Branch's String is its name and its moment, "fall in" of
// unit.Standing, with what narrowed it. A step its moment cannot run — a Here where P is not
// Placed, a step that lasts, a Through — panics by the rule's name as the rule is built.
//
// # Who gets a rule
//
// Nobody hands a rule to a plugin. A rule is obeyed by a role, a role is played, and once a
// Stage's Init returns the engine gives every rule of every role played to the plugin in use that
// catches its moment. A rule no plugin in use catches is an error wrapping [plugin.ErrUnhosted],
// naming the rule and its role. A plugin runs the rules of its moments inside a pass it makes
// anyway, so a rule costs no query of its own; Self, Other and Carrying it reads off its own
// query, asked at the top of a rule alone.
//
// # Roles
//
// A [Part] is a role: a behaviour entities play — mortal, hasty, a trapdoor — not a group. A Stage
// defines its roles by name (world.Roles.Define, with the rules those playing it obey;
// [Part.Obeys] adds more). [Plays] is the component of an entity playing roles, for a kind's
// Spec; a cell plays the roles of its kind and its entry's own, a plugin those it is given. A
// Stage names 64 roles at most. A role's String is "the role mortal".
//
//	roles.Define(HastyRole)
//	units.Define(ScoutKind, land, profile,
//		rule.Plays(roles.Named(MortalRole), roles.Named(HastyRole)))
//
// Obeys narrows each rule on top of its own When: a rule of Self(hungry.Mark()) obeyed by mortal
// fires for a mortal entity under hungry; on a moment that is Met, for the pairs whose own entity
// plays the role. An entity playing two roles that obey the same rule obeys it once for each.
// [Playing] holds while the entity plays a role; inside Here or Around it asks the place turned
// to: Around(1, Branch[any]{When: Playing(lever), Do: Trigger()}).
//
// # Plans
//
// What an entity does over time is a plan, [Plan]: named, over a root step the world runs in
// every step of its simulation — standing in the tactical pause, saved with the game. A kind
// carries it in its Spec, and the plan is defined in the Stage's world as the kind is registered.
//
//	guard := rule.Plan(GuardPlan, rule.OneOf(
//		rule.Branch[any]{Name: "alarm", When: rule.During(alarm),
//			Do: rule.Order(navigation.MoveTo{Cell: gate}).Until[navigation.Arrived]()},
//		rule.Branch[navigation.Blocked]{Name: "blocked", When: navigation.Blocked.WaitedLong,
//			Do: rule.Order(navigation.StepAside{})},
//		rule.Branch[any]{Name: "patrol", Do: rule.Steps(
//			rule.Order(navigation.MoveTo{Cell: east}).Until[navigation.Arrived](),
//			rule.Order(navigation.MoveTo{Cell: west}).Until[navigation.Arrived](),
//		)},
//		rule.Idle(),
//	))
//	units.Define(GuardKind, land, profile, guard)
//
// In a plan a Branch of P is on a fact: a component a plugin writes for the entities with a plan
// (navigation.Blocked, Arrived), which the branch runs while the entity carries as its When says;
// Through, it goes on once begun until its Do is done, though the fact goes. OneOf is run every
// tick and an earlier branch, once it can, takes over from a later one; Steps remembers where it
// is. The steps that last are a plan's alone: [Until], [Wait], [Timeout], [Cooldown], [Invert],
// [Idle]. [Ordered.Until] waits for what came of an Order, [Ordered.Stay] keeps the branch, so a
// reactive branch gives its command once. The state of a plan is the entity's mind, at most 128
// steps; every fact is a component type.
//
// # Conversation
//
// [Ask] asks the subject of the fact it stands under for something — a type naming the ask — and
// waits for the answer: yes runs its agreed step, no or no answer in time its refused one. The ask
// reaches the other entity a tick later as the fact [Asked], which its own plan answers with
// [Agree] or [Refuse], or passes on with [Relay] to the subject of the fact it stands under. The
// answer comes back as the fact [Replied]. An ask nobody takes up is dropped after [AskLife]; a
// chain is at most [MaxChain] long and never loops, so every conversation ends.
//
// # Commands
//
// A command about an effect is a [Command], written as a sentence: [Cast] puts the effect on,
// [Lift] takes it off, [Toggle] switches it; On says whom it is for, For how long a Cast lasts,
// By who sets it off.
//
//	openWest := rule.Cast(open).On(entity.Group("west trapdoors")).By(entity.Named("west lever"))
//	flipGate := rule.Toggle(ajar).On(entity.Group("gate"))
//	alarm    := rule.Cast(alarmed).On(entity.World)
//	hasten   := rule.Cast(haste).On(s.selection.Selected(hasty))
//	freeze   := rule.Cast(frozen).On(s.selection.Pointed()).For(3 * time.Second)
//
// Whom is a [Target]: entity.Named, entity.Group, entity.World — the state of the whole game,
// which rules read with During — or a plugin's own, a [Router]. The plugin the target belongs to
// carries the command out, so a key (control.Give), players.Issue and Order all give it the same
// way. [Trigger] is the step by which an entity sets commands off: every Command whose By names
// it or its group is given, so a role says only that a plate stood on triggers, and the command
// says what that opens. The game defines its commands by name in its world (world.Commands).
//
//	roles.Define(PlateRole, rule.Branch[collision.Meeting]{Name: "press", Do: rule.Trigger()})
//	plates.Define("plate", comp.Const(collision.Collider{}), rule.Plays(roles.Named(PlateRole)))
//	cmds.Define(OpenWestCmd, rule.Cast(fx.Named(OpenEf)).
//		On(entity.Group("west trapdoors")).By(entity.Named("west lever")))
//	s.player.Bind(control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever",
//		cmds.Named(OpenWestCmd)))
//
// How an entity is drawn, decided every frame, is said in Go, by render.Rule.
package rule
