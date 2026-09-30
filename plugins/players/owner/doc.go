// Package owner is whose an entity is: a tag of the [Family] a player ([Of]), saved by its
// [Name], carried by every entity the player owns — several players' tags on one entity it shares
// among them. It is the players plugin's (plugins/players, which registers the family and hands
// every Player its tag), kept a leaf importing only plugin and control, so the plugins that take
// commands — selection, navigation, the topography's cameras — read it without the players
// plugin.
//
// [Obeys] is the one rule they keep: an entity takes commands from its owners alone; one nobody
// owns belongs to the virtual player control.Nobody — the game's code, a script, a test or an AI
// the game runs as nobody — and takes commands from it alone. A player never selects nor orders
// another's units, nor ownerless ones.
package owner
