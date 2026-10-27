package cache

import (
	"encoding/json"
	"fmt"
)

// TreeSnapshotNode 是分類樹快照裡的一個節點。只留別名檢查會用到的三個欄位：
// 父子關係在檢查裡用不到，存下來只會讓快照更大也更容易過期。
type TreeSnapshotNode struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// TreeSnapshot 是 cmd/category-alias-gen 產生別名表時所依據的那一份分類樹。
// 它讓 PR gate 能純離線、決定性地驗證別名表，不必打外部服務——打外部服務的
// gate 會讓營運改一個分類名就把無關的 PR 弄紅。
//
// 含花卉：檢查「別名不得等於任何既有節點名」需要完整的節點名集合。
type TreeSnapshot struct {
	Nodes []TreeSnapshotNode `json:"nodes"`
}

func LoadTreeSnapshot(raw []byte) (TreeSnapshot, error) {
	var snap TreeSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return TreeSnapshot{}, fmt.Errorf("decoding category tree snapshot: %w", err)
	}
	if len(snap.Nodes) == 0 {
		return TreeSnapshot{}, fmt.Errorf("category tree snapshot has no nodes")
	}
	return snap, nil
}

// NodeNames 是所有層級的節點名集合，供「別名不得撞既有節點名」使用。
func (s TreeSnapshot) NodeNames() map[string]bool {
	out := make(map[string]bool, len(s.Nodes))
	for _, n := range s.Nodes {
		out[n.Name] = true
	}
	return out
}

// L2ByName 只索引 Level=2 的節點，供「別名的 target 必須是 L2」使用。
// 同名 L2 保留先出現的那個；別名指向同名 L2 本來就是歧義，由生成端的
// 「同一 alias 不得映到多個 L2」那道檢查負責擋下。
func (s TreeSnapshot) L2ByName() map[string]TreeSnapshotNode {
	out := make(map[string]TreeSnapshotNode)
	for _, n := range s.Nodes {
		if n.Level != 2 {
			continue
		}
		if _, seen := out[n.Name]; !seen {
			out[n.Name] = n
		}
	}
	return out
}
