package roads

import "github.com/kjkrol/gram/plugins/roads/site"

// Router routes a road's legs one by one, as site's Planner does.
type Router = router

// PlanWith has p route the legs of its roads with routers route makes, one for each levelling
// switched to, and plan them with plan, in place of package site's.
func (p *Plugin) PlanWith(route func(levelling bool) Router, plan planning) {
	p.newRouter = func(_ site.Ground, _ site.Rules, _, levelling bool) router {
		return route(levelling)
	}
	p.plan = plan
}

// Ground is the board as p plans its roads over it now.
func (p *Plugin) Ground() site.Ground { return p.readied() }

// Says is what the window and the reason say now: the window's title, its estimate and the
// reason, each empty where nothing is said.
func (p *Plugin) Says() (ask, estimate, reason string) {
	ask, _ = text{p: p, say: askText}.Text(0, false)
	estimate, _ = text{p: p, say: estimateText}.Text(0, false)
	reason, _ = text{p: p, say: reasonText}.Text(0, false)
	return ask, estimate, reason
}
