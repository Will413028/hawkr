package media

import (
	"slices"
	"uuid"
)

// RetiredAssets 列出退場的資產：寫入之前擁有者所有位置的資產減去寫入之後所有位置的資產（集合差，去重，順序依 before）。
// 同一個擁有者列的多個位置可以指著同一個資產，所以不能逐位置比對：重送沒變的 token 或只換掉其中一個位置時，仍被指著的資產不退場。
// 結果交給同 transaction 的 MarkDeleted（B1-D23）。
func RetiredAssets(before, after []uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, b := range before {
		if !slices.Contains(after, b) && !slices.Contains(out, b) {
			out = append(out, b)
		}
	}
	return out
}

// AssetIDs 收集各個可為空的位置指著的資產，略過沒有資產的位置，給 RetiredAssets 的 before 與 after。
func AssetIDs(positions ...*uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(positions))
	for _, p := range positions {
		if p != nil {
			out = append(out, *p)
		}
	}
	return out
}
