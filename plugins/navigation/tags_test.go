package navigation

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/selection"
)

// selTags is the selection tags the navigation tests use, in the order selection.NewPlugin
// defines them.
var selTags = selection.Tags{Selectable: 0, Selected: 1}

// selectedMarks is the selection family with Selectable and Selected set.
var selectedMarks = tag.Tags[selection.Family](0).With(selTags.Selectable, selTags.Selected)

// selectableMarks is the selection family with Selectable alone.
var selectableMarks = tag.Tags[selection.Family](0).With(selTags.Selectable)
