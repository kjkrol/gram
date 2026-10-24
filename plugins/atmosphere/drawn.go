package atmosphere

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/aloft"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/render"
)

// drawnSky is the atmosphere as the frame drawn reads it, as the last Publish copied it: the
// weather, its Dome the copy beside it, the sun, the heavens, the moon's phase, which workings go
// on and the Settings.
type drawnSky struct {
	air      air.Weather
	dome     air.Dome
	sun      light.Distant
	heavens  celestial.Heavens
	moon     float32
	running  Running
	settings Settings
}

// Publish copies the atmosphere for the frame drawn next, of the dome the patches changed since
// that buffer was drawn.
func (p *Plugin) Publish() {
	s := p.published.Back()
	p.fill(s)
	if d := s.air.Dome; d != nil {
		d.CopyInto(&s.dome)
		s.air.Dome = &s.dome
	}
}

// Swap has the atmosphere published last drawn.
func (p *Plugin) Swap() {
	p.published.Swap()
	p.swapped = true
}

func (p *Plugin) fill(s *drawnSky) {
	s.air, s.sun, s.heavens = p.read.Air(), p.read.Sun(), p.sky.Heavens()
	s.moon, s.running, s.settings = p.calendar.Now().Moon(), p.read.Running(), p.Res.Settings
}

// drawn is the atmosphere as the frame drawn reads it: as last published, the live one before any
// was
func (p *Plugin) drawn() *drawnSky {
	if p.swapped {
		return p.published.Front()
	}
	p.fill(&p.live)
	return &p.live
}

func (p *Plugin) drawnAir() air.Weather           { return p.drawn().air }
func (p *Plugin) drawnSun() light.Distant         { return p.drawn().sun }
func (p *Plugin) drawnHeavens() celestial.Heavens { return p.drawn().heavens }

func (p *Plugin) drawnShown() (stars, moon bool) {
	r := p.drawn().running
	return r.Stars, r.Moon
}

// lowClouds are the low clouds seen from above, drawn as the Settings drawn say
type lowClouds struct {
	*aloft.Renderer
	p *Plugin
}

func (c lowClouds) Compose(f *render.Frame, cam camera.Camera) {
	s := c.p.drawn().settings
	c.Show(s.Clouds != ShadowsAlone)
	c.Bodies(s.Clouds == LowBodies || s.Clouds == AllBodies)
	c.Coarse(s.coarse())
	c.BodiesTo(s.reach())
	c.Renderer.Compose(f, cam)
}
