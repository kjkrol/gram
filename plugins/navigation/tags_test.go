package navigation

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/selection"
)

// in the order selection.NewPlugin defines them
var selTags = selection.Tags{Selectable: 0, Selected: 1}

var selectedMarks = tag.Tags[selection.Family](0).With(selTags.Selectable, selTags.Selected)

var selectableMarks = tag.Tags[selection.Family](0).With(selTags.Selectable)
