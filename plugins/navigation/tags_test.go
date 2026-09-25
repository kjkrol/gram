package navigation

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/selection"
)

// selTags is the selection tags the navigation tests use, in the order selection.NewPlugin
// defines them.
var selTags = selection.Tags{Selectable: 0, Selected: 1, Followed: 2}

// selectedMarks is the selection family with Selectable and Selected set.
var selectedMarks = plugin.Tags[selection.Family](0).With(selTags.Selectable, selTags.Selected)

// selectableMarks is the selection family with Selectable alone.
var selectableMarks = plugin.Tags[selection.Family](0).With(selTags.Selectable)
