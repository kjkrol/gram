// Package owner is whose an entity is: a tag of the [Family] a player ([Of]), saved by its
// [Name], carried by every entity the player owns. It is a leaf, so the plugins that take commands
// read it without the players plugin.
//
// [Obeys] is the one rule they keep: an entity takes commands from its owners alone, and one
// nobody owns from the virtual player command.Nobody alone. [Allies] tells whether two entities
// are on one side.
package owner
