package main

import "github.com/kjkrol/gram/plugins/atmosphere/weathering"

func (a *arena) defineClimate() weathering.Config {
	snowy := map[string]string{}
	for _, name := range snowable() {
		snowy[name] = snowyCell(name)
	}
	return weathering.Config{
		Snowy: snowy,
		Ice:   IceCell,
		Sway:  []string{ForestCell, snowyCell(ForestCell)},
	}
}
