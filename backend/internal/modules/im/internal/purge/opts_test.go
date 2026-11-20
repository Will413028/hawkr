package purge

import (
	"testing"

	"github.com/riverqueue/river/rivertype"
)

// 執行中的抹除 job 之外又排入的那一次要排在它後面，唯一性不能含 Running。
func TestEraseUniquenessExcludesRunning(t *testing.T) {
	t.Parallel()
	by := (ErasePartyArgs{}).InsertOpts().UniqueOpts.ByState
	if len(by) == 0 {
		t.Fatal("ByState 是空的：沒有限制狀態的唯一性會把 Running 也算進去")
	}
	for _, st := range by {
		if st == rivertype.JobStateRunning {
			t.Fatal("ByState 含 Running：執行中排入的 job 會被丟掉")
		}
	}
}

// 買家與店家的 job 以 (PartyKind, PartyID) 為唯一鍵：同一個數字是 buyer_id 也是 store_id 時是兩個不同的 job。
func TestEraseUniquenessKeysIncludeTheKind(t *testing.T) {
	t.Parallel()
	opts := (ErasePartyArgs{}).InsertOpts()
	if !opts.UniqueOpts.ByArgs {
		t.Fatal("唯一性要看參數")
	}
	if (ErasePartyArgs{PartyKind: "BUYER", PartyID: 7}) == (ErasePartyArgs{PartyKind: "STORE", PartyID: 7}) {
		t.Fatal("不同身分的參數相等")
	}
}
