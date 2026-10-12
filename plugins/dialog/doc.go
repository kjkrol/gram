// Package dialog is conversations written as data: nodes of lines and answers a Stage loads from
// YAML files, held by its entities and answered by a player through a window pinned to the
// speaker. What a conversation changes in the game is the game's own: its components, changed and
// asked by name.
//
// # Nodes
//
// A [Node] is who speaks, what is said and the answers offered, each a [Choice]: its words, when
// it is offered (If), what it changes in the game (Then), where it leads (Next, or [End]) and the
// commands it gives (Do, the Stage's register). [Plugin.Load] reads a file of them by name,
// [Plugin.Define] says one in code; both in the plugin's own section of a Stage,
// [Conversations] — after the commands, before the kinds:
// stage.With(dialog.Conversations(s.defineConversations)) — after the changes and conditions their
// answers name. Node names are the Stage's, shared by every file; a file a character's, say.
//
//	greet:
//	  speaker: Miller
//	  say: ["Hello, traveller!"]
//	  choices:
//	    - text: "Good to see you again!"
//	      if: friend
//	      next: glad
//	    - text: "Hello to you too!"
//	      then: [warm]
//	      next: glad
//	    - text: "Out of my way."
//	      then: [offend]
//	      next: end
//
// # Changes and conditions
//
// The plugin keeps no model of what a conversation does: the game does, in components of its own
// on its kinds — what a host makes of whom it talked with, a quest begun, a sack of flour given —
// their fields of a fixed length as every component's. [Plugin.Changes] are what answers change,
// by name: [Alter] a function of the speaker's component and the listener, [AlterListener] of the
// listener's and the speaker. [Plugin.Conditions] are when answers are offered, by name: [When] a
// function of the speaker's component and the listener. An answer's then names changes, its if a
// condition or an effect the speaker is under, "!" denying it. A rule changes the game the same way
// with [Feel], an entity's own command about the one its moment names: struck, the host likes the
// striker less.
//
//	s.dialog.Changes().Define(WarmChange, dialog.Alter(func(r *Regard, listener uid.UID64) { r.Like(listener, 40) }))
//	s.dialog.Conditions().Define(FriendCond, dialog.When(func(r *Regard, listener uid.UID64) bool { return r.Liking(listener) >= 30 }))
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
// the speaker it is pinned to, by a player the listener obeys (players/owner): the answer's changes
// are made, its commands given, and the conversation goes on to its next node.
//
//	rule.Then[vision.Sighting]("talk", rule.Other(traveller),
//		rule.If(rule.Not(vision.Sighting.Nobody), rule.Unless(talking, rule.Unless(talked, rule.Order(dialog.Begin{}))))),
//
// # Window
//
// [Plugin.Window] is one ui element for every speaker: pinned Under TalkingEf, its title the
// speaker, what is said, a button for each answer offered. [Plugin.SpeakerText],
// [Plugin.LineText] and [Plugin.ChoiceText] lay out a window of a game's own. What a game shows of
// its own components — what a host makes of the selected unit — is ui.Of.
package dialog
