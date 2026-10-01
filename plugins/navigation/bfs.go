package navigation

import "github.com/kjkrol/gram/plugins/board/cell"

// breadthFirst returns the first cell match accepts, ring by ring from start through expand.
func breadthFirst(start cell.ID, neighbors func(cell.ID) []cell.ID,
	expand, match func(cell.ID) bool, maxVisited int) (cell.ID, bool) {
	queue := []cell.ID{start}
	visited := map[cell.ID]bool{start: true}
	for len(queue) > 0 && maxVisited > 0 {
		c := queue[0]
		queue = queue[1:]
		maxVisited--
		if match(c) {
			return c, true
		}
		for _, n := range neighbors(c) {
			if !visited[n] && expand(n) {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}
	return 0, false
}
