package repository

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// 本檔守的是 #2446 契約凍結**收斂之後**的值域判定。前一波（expand）的相容層測試
// （expand_compat_test.go）隨相容層一起移除——它證明的是「新舊值都認」，而那正是收斂要終結的狀態。

// TestIsSucceededRunStatus 的負向 case 才是重點：正向只有一個值，寫成 `return true`
// 也會過。舊值那三個 case 釘住的是「相容層真的拆掉了」——把 legacy 分支偷偷加回來會踩紅。
func TestIsSucceededRunStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"契約凍結後唯一的成功終態", codemeta.ProcurementRunStatusSucceeded.Value, true},
		{"失敗終態不是成功", codemeta.ProcurementRunStatusFailed.Value, false},
		{"非終態不是成功", codemeta.ProcurementRunStatusRunning.Value, false},
		{"排隊中不是成功", codemeta.ProcurementRunStatusQueued.Value, false},
		{"改名前的 COMPLETED 不得再被接受", "COMPLETED", false},
		{"改名前的 PARTIAL 不得再被接受", "PARTIAL", false},
		{"未知值一律不算成功", "SOMETHING_ELSE", false},
		{"空字串不算成功", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isSucceededRunStatus(tt.status),
				"這個判斷決定 GetCandidateSetWithCandidates 要不要去讀候選。"+
					"漏認一個成功值 → 那些 run 對外變成「終態但零候選」；"+
					"多認一個非成功值 → 還在跑的 run 會提前吐出不完整的候選清單。兩種都沒有錯誤訊號。")
		})
	}
}

// Wave C 的兩道守衛（TestCandidateSetRowsAreConvertible／
// TestGeneratedQueriesDoNotReferenceErrorCode）隨 error_code 一起在本波移除：前者釘的是
// 「五支查詢欄位清單一致」，查詢改回 `SELECT *` 後這件事由 sqlc 結構性保證；後者釘的是
// 「生成的 SQL 不得提到即將被 DROP 的欄位」，欄位已 DROP 後這個形狀不再存在。
