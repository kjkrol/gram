// Package rule is how a game says what its entities do: a [Branch] says when, on whom and
// what — at a moment a plugin catches, as a rule a role obeys; over time, as a plan a kind
// carries — its steps putting effects on and giving commands. A change that holds is an effect
// (package rule/effect). The package is gram's core, beside entity and clock; the hosts a plugin
// runs the rules of its moments with are package plugin's.
//
//	roles := s.world.Roles()
//	roles.Define(MortalRole, rule.Branch[unit.Standing]{
//		Name: "fall in",
//		When: unit.Standing.Fallen,
//		On:   rule.Self,
//		Do:   rule.Apply(world.Despawn{}),
//	})
//	units.Define(ScoutKind, land, profile, rule.Plays(roles.Named(MortalRole)))
//
// A rule is a sentence: who, when, on whom, what. Who is the role the rule is given to; when is
// the moment, the type P a Branch is of, and the conditions in that moment's own words; on whom
// is [On]; what is the steps. Defining registers and hands nothing back: Named is the handle
// wherever a thing is built on, and a game keeps its names as constants.
//
// # Branches
//
// A [Branch] of P runs its Do When its conditions hold: a predicate of P (unit.Standing.Fallen,
// clock.At(dusk)), the entity's state ([Under]), the world's or a plugin's ([UnderCond.On]),
// [Playing] a role, [Chance], the other of a pair in the moment's word (vision.Seeing,
// collision.Met; [Paired] underneath), [Not] any of them, several at once ([All]), nil for
// always. On says whom the steps act on: [Self], [Other] (the other of the pair, the subject of
// the fact), [Here] and [Around] (the places round, on a moment that is plugin.Placed); a Branch
// inside another's Do acts on what the outer one does unless it says otherwise. Do is a [Step]:
// [OneOf] the first that does not fail, [Steps] one after another, a Branch asking more, [Put],
// [Take], [Hold] an effect, [Apply] a command (a [Command] without an On is a step too). A rule
// holds no Go code but its predicates and keeps no memory of its own: an effect's presence is
// its memory, Not(Under(e)) asks whether it has done a thing already.
//
// A moment is [plugin.About] one entity; one that is [plugin.Placed] stands on places of its
// own, a board's cells. Chance draws from the world's seed, the step's game time and the entity,
// so a load and a replay draw alike. Every Put and Take of a step lands together in the effects'
// pass. Apply gives a command for whom the branch acts on, the same a player gives; one that is
// [plugin.Aimed] is told the [plugin.Subject] of the moment. A Branch's String is its name and
// its moment, "fall in" of unit.Standing, with what narrowed it. A step its moment cannot run —
// Here where P is not Placed, a step that lasts, a message — panics by the rule's name as the
// rule is built; so does a Branch at the top of a rule or a plan without a Name or an On.
//
// # Who gets a rule
//
// Nobody hands a rule to a plugin. A rule is obeyed by a role, a role is played, and once a
// Stage's Init returns the engine gives every rule of every role played to the plugin in use that
// catches its moment. A rule no plugin in use catches is an error wrapping [plugin.ErrUnhosted],
// naming the rule and its role. A plugin runs the rules of its moments inside a pass it makes
// anyway, so a rule costs no query of its own: Playing and Paired at the top of a rule it reads
// off its own query.
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
// Obeys narrows each rule on top of its own When; on a moment that is Met, to the pairs whose own
// entity plays the role, and a pair rule whose other's role is its own still runs once a pair.
// An entity playing two roles that obey the same rule obeys it once for each. [Playing] holds
// while the entity plays a role; under On Here or Around it asks the place turned to:
// Branch[unit.Standing]{Name: "pull", On: rule.Around(1), Do: rule.Branch[any]{When:
// rule.Playing(lever), Do: rule.Apply(rule.Triggered{})}}.
//
// # Plans
//
// What an entity does over time is a plan, [Plan]: named, over a root the world runs in every
// step of its simulation — standing in the tactical pause, saved with the game. A kind carries
// it in its Spec, and the plan is defined in the Stage's world as the kind is registered. The
// root is OneOf of branches (the first that can, every tick, an earlier taking over from a
// later), Steps of them (one after another, remembering its place) or one branch; done, it
// begins again the next tick. A Branch at its top says its Name and its On.
//
//	guard := rule.Plan(GuardPlan, rule.OneOf(
//		rule.Branch[any]{
//			Name: "to the gate",
//			When: rule.Under(alarm).On(entity.World),
//			On:   rule.Self,
//			Do: rule.Steps(
//				rule.Apply(navigation.MoveTo{To: entity.Named(GateName)}),
//				rule.Until[navigation.Arrived](),
//			),
//		}.Timeout(20 * time.Second),
//		rule.Branch[navigation.Arrived]{
//			Name: "rest",
//			On:   rule.Self,
//			Do:   rule.Wait(2 * time.Second),
//		},
//		rule.Branch[any]{
//			Name: "patrol",
//			On:   rule.Self,
//			Do:   rule.Steps(rule.Apply(navigation.Onward{}), rule.Until[navigation.Arrived]()),
//		},
//		rule.Branch[any]{Name: "idle", On: rule.Self, Do: rule.Idle()},
//	))
//	units.Define(GuardKind, land, profile, guard)
//
// In a plan a Branch of a fact — a component a plugin writes for the entities with a plan,
// navigation.Arrived, Blocked — runs once as the fact comes to hold its When, its Do to the end
// though the fact goes, and not again until the fact comes afresh: a fact is a thing that
// happened. A Branch of any runs while its When holds. The steps that last are a plan's alone:
// [Until] waits for a fact coming afresh, [Wait] for a time, [Idle] for ever. [Branch.Timeout]
// has a branch fail once it has run for d, [Branch.Every] keeps it from beginning again within d.
// The state of a plan is the entity's mind, at most 128 steps; every fact is a component type.
//
// # Messages
//
// A command embedding [From], given with Apply on a branch whose On is Other, lands on that
// entity as a fact in place of going to a queue, the engine filling in who sent it; one without
// a plan takes none, and the step fails. A Branch on the command's type answers it, Apply on
// Other again, or passes it on to the subject of another fact. A message nobody takes up is
// dropped after [AskLife]; one passed on more than [MaxChain] times is not sent.
//
//	type pass struct{ rule.From }
//	type reply struct {
//		rule.From
//		Yes bool
//	}
//
//	rule.Branch[navigation.Blocked]{
//		Name: "ask to pass",
//		When: navigation.Blocked.WaitedLong,
//		On:   rule.Other,
//		Do: rule.Steps(
//			rule.Apply(pass{}),
//			rule.OneOf(
//				rule.Branch[any]{Do: rule.Until[reply]()}.Timeout(time.Second),
//				rule.Branch[any]{On: rule.Self, Do: rule.Apply(flee{})},
//			),
//		),
//	}
//	rule.Branch[reply]{
//		Name: "let through",
//		When: reply.yes,
//		On:   rule.Self,
//		Do:   rule.Apply(walk{}),
//	}
//
// # Commands
//
// A command about an effect is a [Command], written as a sentence: [Put] puts the effect on,
// [Take] takes it off, [Toggle] switches it; On says whom it is for, For how long a Put lasts,
// By who sets it off. Without an On, Put and Take are steps of a rule or a plan.
//
//	openWest := rule.Put(open).On(entity.Group("west trapdoors")).By(entity.Named("west lever"))
//	flipGate := rule.Toggle(ajar).On(entity.Group("gate"))
//	alarm    := rule.Put(alarmed).On(entity.World)
//	hasten   := rule.Put(haste).On(s.selection.Selected(hasty))
//	freeze   := rule.Put(frozen).On(s.selection.Pointed()).For(3 * time.Second)
//
// Whom is a [Target]: entity.Named, entity.Group, entity.World — the state of the whole game,
// which rules read with Under(e).On(entity.World) — or a plugin's own, a [Router]. The plugin the
// target belongs to carries the command out, so a key (control.Give), players.Issue and Apply
// all give it the same way. [Triggered] is the command by which an entity sets commands off:
// every Command whose By names it or its group is given, so a role says only that a plate stood
// on triggers, and the command says what that opens. The game defines its commands by name in
// its world (world.Commands).
//
//	roles.Define(PlateRole, rule.Branch[collision.Meeting]{
//		Name: "press", On: rule.Self, Do: rule.Apply(rule.Triggered{})})
//	plates.Define("plate", comp.Const(collision.Collider{}), rule.Plays(roles.Named(PlateRole)))
//	cmds.Define(OpenWestCmd, rule.Put(fx.Named(OpenEf)).
//		On(entity.Group("west trapdoors")).By(entity.Named("west lever")))
//	s.player.Bind(control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever",
//		cmds.Named(OpenWestCmd)))
//
// How an entity is drawn, decided every frame, is said in Go, by render.Rule.
package rule
