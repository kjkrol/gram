package engine

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
)

// persistence is the only concrete implementation of game.Persistence,
// bound to one Stage's ecsHost.
type persistence struct {
	host *ecsHost
}

var _ game.Persistence = (*persistence)(nil)

// List returns every save found for basePath, "" (quicksave) first.
func (p *persistence) List(basePath string) ([]string, error) { return listSaves(basePath) }

// Save writes resources, every tracked Serializable and the ECS snapshot under basePath/label.
func (p *persistence) Save(basePath, label string, resources ...any) error {
	return save(p.host.ecs, basePath, label, p.host.persistGroups(resources...))
}

// Load restores a snapshot written by Save; a resource the save does not hold keeps its value.
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
