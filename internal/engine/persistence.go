package engine

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
)

type persistence struct {
	host *ecsHost
}

var _ game.Persistence = (*persistence)(nil)

func (p *persistence) List(basePath string) ([]string, error) { return listSaves(basePath) }

func (p *persistence) Save(basePath, label string, resources ...any) error {
	return save(p.host.ecs, basePath, label, p.host.persistGroups(resources...))
}

func (p *persistence) Load(basePath, label string, resources ...any) error {
	comps := p.host.providedComps()
	loaded, err := load(p.host.ecs, basePath, label, comps, p.host.persistGroups(resources...))
	if err != nil {
		return err
	}
	p.host.loaded = loaded
	p.host.runRestore()
	systems := p.host.postLoadSystems()
	p.host.addPendingSetup(func() []goke.System { return systems })
	return nil
}
