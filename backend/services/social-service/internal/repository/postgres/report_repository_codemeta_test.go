package postgres

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// reports.sql 的碼值改由呼叫端帶入之後，那三份清單就成了述詞本身——SQL 端再也讀不
// 出「哪些狀態可以裁決」「哪些動作算前科」。清單是純字串 slice：打錯一個字、漏掉一
// 個新增的碼值、或圖省事塞成整包 AllXxx，全都編譯得過、既有測試也全綠，只會在執行
// 期表現成「查詢永遠回 0 列」或「統計數字悄悄變大」——兩者都不會拋錯。
//
// 所以這裡把每份清單與 CodeMeta 的關係釘住，並且刻意用「數量」而非「內容」來鎖住
// 子集關係：新增一個 ReportResolveAction 時，這支測試會逼人回答「它算不算前科」，
// 而不是讓它靜默地被排除在外。

func assertAllRegistered[T any](t *testing.T, name string, values []string, lookup func(string) (T, bool)) {
	t.Helper()
	for _, v := range values {
		if _, ok := lookup(v); !ok {
			t.Errorf("%s 含有未登記於 CodeMeta 的值 %q——DB CHECK 會在寫入／比對時把它當成不存在的碼", name, v)
		}
	}
}

func TestResolvableAndReopenableStatusesPartitionCodeMeta(t *testing.T) {
	assertAllRegistered(t, "resolvableStatuses", resolvableStatuses, codemeta.ReportStatusFromValue)
	assertAllRegistered(t, "reopenableStatuses", reopenableStatuses, codemeta.ReportStatusFromValue)

	// 兩份清單必須是 ReportStatus 的一組**分割**：每個狀態不是「可被裁決的起點」就是
	// 「可被重開的終態」，不能兩者皆是、也不能兩者皆非。新增一個 ReportStatus 而沒
	// 決定它的歸屬時，AdminResolveReport 與 ReopenReport 會同時看不到它，該狀態的
	// 單子就此永久卡住而毫無訊號。
	union := append(append([]string(nil), resolvableStatuses...), reopenableStatuses...)
	sort.Strings(union)

	all := make([]string, 0, len(codemeta.AllReportStatuss))
	for _, v := range codemeta.AllReportStatuss {
		all = append(all, v.Value)
	}
	sort.Strings(all)

	if strings.Join(union, ",") != strings.Join(all, ",") {
		t.Errorf("resolvable ∪ reopenable 必須恰好等於 ReportStatus 全集：\n  聯集     = %v\n  CodeMeta = %v\n"+
			"新增狀態時要同時決定它屬於哪一邊（reports.sql 的兩條述詞各讀一份）", union, all)
	}

	seen := map[string]bool{}
	for _, v := range resolvableStatuses {
		seen[v] = true
	}
	for _, v := range reopenableStatuses {
		if seen[v] {
			t.Errorf("狀態 %q 同時出現在 resolvableStatuses 與 reopenableStatuses——"+
				"一個已在裁決佇列裡的單子又能被「重開」，兩條 UPDATE 會互相搶同一列", v)
		}
	}
}

// priorOffenceActions 是本次收斂最容易出錯的一處：它長得像值域，其實是業務述詞。
// 整包塞 AllReportResolveActions 讓 SQL 照樣跑、照樣回數字，只是被駁回的檢舉開始
// 被算成前科——管理員看到的累犯次數變高，而沒有任何東西會紅。
func TestPriorOffenceActionsExcludeDismissOnly(t *testing.T) {
	assertAllRegistered(t, "priorOffenceActions", priorOffenceActions, codemeta.ReportResolveActionFromValue)

	for _, v := range priorOffenceActions {
		if v == codemeta.ReportResolveActionDismiss.Value {
			t.Errorf("DISMISS 不得計入前科：檢舉不成立代表被檢舉人沒做錯事，" +
				"把它算進 90 天累犯次數會讓被誣告的人在管理員眼裡變成慣犯（reports.sql CountRecentActionsByOwner）")
		}
	}

	// 只排除 DISMISS 一個。新增第五個 ReportResolveAction 時這裡會紅，逼人明確回答
	// 「這個新動作算不算前科」——沉默地被排除在外是最糟的結果。
	if want := len(codemeta.AllReportResolveActions) - 1; len(priorOffenceActions) != want {
		t.Errorf("priorOffenceActions 有 %d 個值，預期 %d 個（ReportResolveAction 全集扣掉 DISMISS）。"+
			"若 CodeMeta 新增了裁決動作，請明確決定它算不算前科後再更新本測試",
			len(priorOffenceActions), want)
	}
}

// 上面兩支測試守住清單的內容，這一支守住**接線**：哪份清單被送進哪一條 query。
// 兩者是不同的錯誤——把 reopenableStatuses 誤傳給 CountRecentActionsByOwner，清單
// 本身仍然完全正確，SQL 也照跑，只是統計數字從此恆為 0。
//
// usecase 側的 fake repo 幫不上忙：它自己把回傳列的 Status 設成 APPEALED，所以
// 「Reopen 之後是不是 APPEALED」在那一層是同義反覆，測的是 fake 不是產品碼。真正
// 決定寫入什麼值的是本檔上方的常數與下面這幾個 params，攔截 DBTX 才看得到。
type capturingDBTX struct {
	lastArgs []interface{}
}

var errCaptureOnly = errors.New("capturing DBTX: no rows are ever returned")

func (c *capturingDBTX) Exec(_ context.Context, _ string, args ...interface{}) (pgconn.CommandTag, error) {
	c.lastArgs = args
	return pgconn.CommandTag{}, errCaptureOnly
}

func (c *capturingDBTX) Query(_ context.Context, _ string, args ...interface{}) (pgx.Rows, error) {
	c.lastArgs = args
	return nil, errCaptureOnly
}

func (c *capturingDBTX) QueryRow(_ context.Context, _ string, args ...interface{}) pgx.Row {
	c.lastArgs = args
	return errRow{}
}

type errRow struct{}

func (errRow) Scan(...any) error { return errCaptureOnly }

func newCapturingRepo() (*ReportRepository, *capturingDBTX) {
	db := &capturingDBTX{}
	return &ReportRepository{q: gen.New(db)}, db
}

// argStrings 取出第 i 個參數並斷言它是字串清單。
func argStrings(t *testing.T, args []interface{}, i int) []string {
	t.Helper()
	if i >= len(args) {
		t.Fatalf("query 只帶了 %d 個參數，取不到第 %d 個——generated code 的參數順序變了", len(args), i)
	}
	got, ok := args[i].([]string)
	if !ok {
		t.Fatalf("第 %d 個參數型別是 %T，預期 []string", i, args[i])
	}
	return got
}

func TestReopenSendsAppealedNotPending(t *testing.T) {
	repo, db := newCapturingRepo()
	_, _ = repo.Reopen(context.Background(), 1) // 回傳值必為錯（fake 不吐列），只看送出去什麼

	if len(db.lastArgs) < 3 {
		t.Fatalf("ReopenReport 預期 3 個參數（id / 目標狀態 / 可重開終態），實際 %d 個", len(db.lastArgs))
	}
	target, ok := db.lastArgs[1].(string)
	if !ok {
		t.Fatalf("目標狀態參數型別是 %T，預期 string", db.lastArgs[1])
	}
	if target != codemeta.ReportStatusAppealed.Value {
		t.Errorf("Reopen 把單子寫成 %q，必須是 %q。PENDING 代表「還沒有人看過」，"+
			"用它會讓已經處置過、正在被申訴的案件混進未讀佇列，管理員再也分不出兩者",
			target, codemeta.ReportStatusAppealed.Value)
	}

	want := append([]string(nil), reopenableStatuses...)
	got := append([]string(nil), argStrings(t, db.lastArgs, 2)...)
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ReopenReport 收到的候選狀態是 %v，預期 reopenableStatuses %v", got, want)
	}
}

func TestAdminResolveAndRecentActionsSendTheirOwnLists(t *testing.T) {
	repo, db := newCapturingRepo()
	_, _ = repo.AdminResolve(context.Background(), 1,
		codemeta.ReportStatusDismissed.Value, codemeta.ReportResolveActionDismiss.Value, 7)

	got := append([]string(nil), argStrings(t, db.lastArgs, 4)...)
	want := append([]string(nil), resolvableStatuses...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("AdminResolveReport 收到的起始狀態是 %v，預期 resolvableStatuses %v——"+
			"帶錯清單不會報錯，只會讓 UPDATE 恆命中 0 列、每一次裁決都回 ErrAlreadyResolved", got, want)
	}

	repo, db = newCapturingRepo()
	_, _ = repo.CountRecentActionsByOwner(context.Background(), 999)

	got = append([]string(nil), argStrings(t, db.lastArgs, 1)...)
	want = append([]string(nil), priorOffenceActions...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("CountRecentActionsByOwner 收到的動作清單是 %v，預期 priorOffenceActions %v——"+
			"帶錯清單不會報錯，累犯次數只會靜靜地變成錯的數字", got, want)
	}
}
