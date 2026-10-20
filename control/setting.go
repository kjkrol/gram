package control

import "github.com/kjkrol/ceremony/command"

// Setting is one thing of a plugin's the players adjust on the settings screen, a control each:
// a [Switch], a [Choice] or a [Range]. Get is read every frame; Set builds the command carrying
// the value chosen, which the screen issues as a button issues its commands. Section is the
// section the setting is listed under, a path of levels ("Picture/Graphics"); empty, the
// plugin's own.
type Setting interface{ setting() }

// The sections of the settings screen the plugins share: the picture, how it is drawn in a
// tactical view and riding in first person and the layers of each, the ground's care, the areas
// shown, the graphics.
const (
	Picture               = "Picture"
	PictureTactical       = "Picture/Tactical"
	PictureTacticalLayers = "Picture/Tactical/Layers"
	PictureRiding         = "Picture/Riding"
	PictureRidingLayers   = "Picture/Riding/Layers"
	PictureCare           = "Picture/Care"
	PictureShadows        = "Picture/Shadows"
	PictureAreas          = "Picture/Areas"
	Graphics              = "Picture/Graphics"
)

// SectionOf is the section s is listed under; empty for the plugin's own.
func SectionOf(s Setting) string {
	switch s := s.(type) {
	case Switch:
		return s.Section
	case Choice:
		return s.Section
	case Range:
		return s.Section
	}
	return ""
}

// Switch is a setting that is on or off.
type Switch struct {
	Label   string
	Section string
	Get     func() bool
	Set     func(on bool) command.Command
}

// Choice is a setting with one of its Options chosen, Get and Set by its index.
type Choice struct {
	Label   string
	Section string
	Options []string
	Get     func() int
	Set     func(i int) command.Command
}

// Range is a setting of a number between Min and Max, moved a Step at a time; Shows writes the
// number for the player (nil: as it is).
type Range struct {
	Label   string
	Section string
	Min     float64
	Max     float64
	Step    float64
	Get     func() float64
	Set     func(v float64) command.Command
	Shows   func(v float64) string
}

func (Switch) setting() {}
func (Choice) setting() {}
func (Range) setting()  {}

var _ Setting = Switch{}
var _ Setting = Choice{}
var _ Setting = Range{}
