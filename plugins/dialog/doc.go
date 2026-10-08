// Package dialog is conversations written as data: nodes of lines and answers a Stage loads from
// YAML files, held by its entities, answered by a player through a window pinned to the speaker,
// and remembered — what each speaker makes of whom it talked with.
//
// # Nodes
//
// A [Node] is who speaks, what is said and the answers offered, each a [Choice]: its words, when
// it is offered (If), what the speaker makes of it (Mood), where it leads (Next, or [End]) and
// what it does in the game (Do, commands of the Stage's register). [Plugin.Load] reads a file of
// them by name, [Plugin.Define] says one in code; both where the Stage defines its commands. Node
// names are the Stage's, shared by every file; a file a character's, say.
//
//	greet:
//	  speaker: Miller
//	  say: ["Hello, traveller!"]
//	  choices:
//	    - text: "Hello to you too!"
//	      mood: 40
//	      next: glad
//	    - text: "Good to see you again!"
//	      if: friend
//	      next: glad
//	    - text: "Out of my way."
//	      mood: -60
//	      next: end
//
// An answer's If is friend, neutral or enemy — what the speaker makes of the listener, weighed by
// [Config] — talked, they talked to the end before, or the name of an effect the speaker is under;
// "!" before it when it does not hold.
//
// # Conversations
//
// An entity's conversations begin where its [Script] says ([Plugin.Script]: a kind's component, the
// same for all its entities or each one's own), or at the node a [Begin] names. A Begin is the
// entity's own, about whom it talks with: a rule's Order on a moment naming somebody — the host
// sees the traveller. The conversation is the speaker's [Talk]; the plugin's two effects are its
// handles ([Plugin.DefineEffects]): [TalkingEf] is on the speaker while it talks — taken off by
// anyone, a rule seeing the listener walk off, the conversation is over — and [TalkedEf] for a
// while after one talked to the end. A [Choose] answers, given by a button of [Plugin.Window] for
// the speaker it is pinned to, by a player the listener obeys (players/owner).
//
//	rule.Then[vision.Sighting]("talk", rule.Other(traveller),
//		rule.If(rule.Not(vision.Sighting.Nobody), rule.Unless(talking, rule.Unless(talked, rule.Order(dialog.Begin{}))))),
//
// # Memory
//
// Every speaker's [Memory] keeps what it makes of up to [MaxMemory] others, a mood from -100 to
// 100, and whether they talked to the end: answers move it, so does a [Mood] given by a rule.
// [Plugin.Stance] is it as a word — Friend, Neutral, Enemy — towards a player's chosen unit, for a
// label pinned to the unit the cursor points at.
//
// # Window
//
// [Plugin.Window] is one ui element for every speaker: pinned Under TalkingEf, its title the
// speaker, what is said, a button for each answer offered. [Plugin.SpeakerText],
// [Plugin.LineText] and [Plugin.ChoiceText] lay out a window of a game's own.
package dialog
