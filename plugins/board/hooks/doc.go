// Package hooks holds ready-made hooks of where entities stand, for the board plugin's Hook:
// LogFalls writes a line for whoever stands where it may not. A game wanting something else writes
// its own rule of a unit.Standing or cell.Now (rule.On). A file using another plugin's hooks
// too imports this one as bhooks.
package hooks
