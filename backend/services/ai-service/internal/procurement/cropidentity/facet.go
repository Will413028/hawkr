package cropidentity

type categoryNode struct {
	id, label, parentID, parentLabel string
	level                            int
}

type facetGroup struct {
	id, label string
	leaves    []categoryNode
}

func (g facetGroup) weight() int {
	if len(g.leaves) == 0 {
		return 1
	}
	return len(g.leaves)
}

func facetGroups(nodes []categoryNode) []facetGroup {
	var groups []facetGroup
	index := map[string]int{}
	at := func(id, label string) int {
		i, ok := index[id]
		if !ok {
			i = len(groups)
			index[id] = i
			groups = append(groups, facetGroup{id: id, label: label})
		}
		return i
	}
	for _, n := range nodes {
		if n.level == 2 {
			at(n.id, n.label)
			continue
		}
		i := at(n.parentID, n.parentLabel)
		groups[i].leaves = append(groups[i].leaves, n)
	}
	return groups
}

func facetAsksAtGroup(groups []facetGroup) bool {
	if len(groups) < 2 {
		return false
	}
	total, largest := 0, 0
	for _, g := range groups {
		w := g.weight()
		total += w
		largest = max(largest, w)
	}
	return largest*5 < total*4
}

func dominantGroup(groups []facetGroup) facetGroup {
	best := groups[0]
	for _, g := range groups[1:] {
		if g.weight() > best.weight() {
			best = g
		}
	}
	return best
}
