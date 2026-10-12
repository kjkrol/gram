// Package dialog is conversations written as data: nodes of lines and answers a Stage loads from
// YAML files, held by its entities and answered by a player through a window pinned to the
// speaker. What a conversation changes in the game is the game's own components, changed and
// asked by name.
//
// # Nodes
//
// A [Node] is who speaks, what is said and the answers offered, each a [Choice]: its words, when
// it is offered (If), what it changes (Then), where it leads (Next, or [End]) and the commands it
// gives (Do). [Plugin.Load] reads a file of them by name, [Plugin.Define] says one in code, both
// in the plugin's own section of a Stage, [Conversations]:
//
//	greet:
//	  speaker: Miller
//	  say: ["Hello, traveller!"]
//	  choices:
//	    - text: "Good to see you again!"
//	      if: friend
//	      next: glad
//	    - text: "Out of my way."
//	      then: [offend]
//	      next: end
//
// # Changes and conditions
//
// [Plugin.Changes] are what answers change, by name: [Alter] a function of the speaker's
// component and the listener, [AlterListener] of the listener's and the speaker.
// [Plugin.Conditions] are when answers are offered, by name: [When] a function of the speaker's
// component and the listener. An answer's if may name an effect the speaker is under instead,
// "!" denying it. A rule changes the game the same way with [Feel].
//
// # Conversations
//
// An entity's conversations begin where its [Script] says, or at the node a [Begin] names; a
// Begin is the entity's own, about whom it talks with. The conversation is the speaker's [Talk];
// [TalkingEf] is on the speaker while it talks and [TalkedEf] for a while after one talked to the
// end ([Plugin.DefineEffects]). A [Choose] answers, given by a button of [Plugin.Window], by a
// player the listener obeys.
//
// # Window
//
// [Plugin.Window] is one ui element for every speaker, pinned Under TalkingEf: its title the
// speaker, what is said, a button for each answer offered. [Plugin.SpeakerText],
// [Plugin.LineText] and [Plugin.ChoiceText] lay out a window of a game's own.
package dialog
