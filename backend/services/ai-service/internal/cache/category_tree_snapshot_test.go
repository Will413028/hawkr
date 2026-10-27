package cache

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadTreeSnapshot(t *testing.T) {
	raw, err := os.ReadFile("testdata/category_tree_snapshot.json")
	require.NoError(t, err)

	snap, err := LoadTreeSnapshot(raw)
	require.NoError(t, err)
	require.NotEmpty(t, snap.Nodes)

	names := snap.NodeNames()
	require.True(t, names["甘藍"], "L2 名要在節點名集合裡")
	require.True(t, names["初秋"], "L3 名也要在——別名不得撞任何層級的節點名")
	require.True(t, names["花卉"], "快照必須含花卉，否則撞到花卉節點名抓不到")
	require.False(t, names["高麗菜"], "高麗菜不是節點名，所以可以當別名")

	l2 := snap.L2ByName()
	require.Equal(t, "117", l2["甘藍"].ID)
	_, isL2 := l2["初秋"]
	require.False(t, isL2, "L3 不得出現在 L2 索引裡")
}

func TestLoadTreeSnapshot_RejectsMalformed(t *testing.T) {
	_, err := LoadTreeSnapshot([]byte("not json"))
	require.Error(t, err)
}
