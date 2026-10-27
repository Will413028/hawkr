package cropidentity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func leaf(id, parent string) categoryNode {
	return categoryNode{id: id, label: "葉" + id, parentID: parent, parentLabel: "組" + parent, level: 3}
}

func TestFacetGroups_KeepsFirstAppearanceOrderAndCountsUnexpandedAsOne(t *testing.T) {
	nodes := []categoryNode{leaf("a", "210"), leaf("b", "200"), {id: "220", label: "米", level: 2}, leaf("c", "200"), {id: "200", label: "豬肉", level: 2}}
	groups := facetGroups(nodes)
	require.Len(t, groups, 3)
	require.Equal(t, []string{"210", "200", "220"}, []string{groups[0].id, groups[1].id, groups[2].id})
	require.Equal(t, 1, groups[0].weight())
	require.Equal(t, 2, groups[1].weight(), "同組的未展開 L2 不加權，葉已經代表它")
	require.Equal(t, 1, groups[2].weight(), "沒有葉的未展開 L2 算 1")
	require.Empty(t, groups[2].leaves)
	require.Equal(t, "米", groups[2].label)
}

func TestFacetAsksAtGroup(t *testing.T) {
	g := func(id string, n int) facetGroup {
		fg := facetGroup{id: id}
		for i := 0; i < n; i++ {
			fg.leaves = append(fg.leaves, leaf(id+string(rune('a'+i)), id))
		}
		return fg
	}
	require.False(t, facetAsksAtGroup([]facetGroup{g("200", 5)}), "只有一組沒得問")
	require.True(t, facetAsksAtGroup([]facetGroup{g("200", 1), g("210", 1)}), "1:1 ＝ 50%")
	require.True(t, facetAsksAtGroup([]facetGroup{g("200", 3), g("210", 2)}), "3:2 ＝ 60%")
	require.True(t, facetAsksAtGroup([]facetGroup{g("200", 3), g("210", 1)}), "3:1 ＝ 75%")
	require.False(t, facetAsksAtGroup([]facetGroup{g("200", 4), g("210", 1)}), "4:1 ＝ 80% 不問，下鑽")
	require.False(t, facetAsksAtGroup([]facetGroup{g("200", 8), g("210", 1)}), "九個候選八個同組")
}

func TestDominantGroup_LargestElseFirst(t *testing.T) {
	groups := facetGroups([]categoryNode{leaf("a", "210"), leaf("b", "200"), leaf("c", "200")})
	require.Equal(t, "200", dominantGroup(groups).id)
	groups = facetGroups([]categoryNode{leaf("a", "210"), leaf("b", "200")})
	require.Equal(t, "210", dominantGroup(groups).id, "同權取先出現（候選已依品質→RRF 排好）")
}
