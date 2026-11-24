package charge

import (
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
)

// checkValues 取出 schema.sql 裡某個 CHECK 的 IN (...) 清單。
func checkValues(t *testing.T, constraint string) []string {
	t.Helper()
	return checkColumnValues(t, constraint, "status")
}

// checkColumnValues 取出 schema.sql 裡某個欄位的 CHECK IN (...) 清單。
func checkColumnValues(t *testing.T, constraint, column string) []string {
	t.Helper()
	raw, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`CONSTRAINT ` + constraint + ` CHECK \(` + column + ` IN \(([^)]*)\)\)`).FindSubmatch(raw)
	if m == nil {
		t.Fatalf("schema.sql 沒有 %s", constraint)
	}
	var out []string
	for _, v := range regexp.MustCompile(`'([A-Z_]+)'`).FindAllSubmatch(m[1], -1) {
		out = append(out, string(v[1]))
	}
	return out
}

func names[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// 狀態用 Go 型別加 CHECK：兩邊的清單必須逐項相同，新增或改名只改一邊會在這裡失敗。
func TestStatusTypesMatchTheSchemaChecks(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		constraint string
		values     []string
	}{
		{"payments_status_check", names(model.PaymentStatusValues())},
		{"transfers_status_check", names(model.TransferStatusValues())},
		{"refunds_status_check", names(model.RefundStatusValues())},
	} {
		if got := checkValues(t, c.constraint); !slices.Equal(got, c.values) {
			t.Errorf("%s：schema=%v，Go 型別=%v", c.constraint, got, c.values)
		}
	}
	if got, want := checkColumnValues(t, "refunds_reversal_status_check", "reversal_status"), names(model.ReversalStatusValues()); !slices.Equal(got, want) {
		t.Errorf("refunds_reversal_status_check：schema=%v，Go 型別=%v", got, want)
	}
}

// provider 名稱同時是 payments.provider 與 inbox 的 provider 名稱：必須符合兩邊共同的格式（payments_provider_check、inbox 的 providerPattern）。
// 與收款帳戶 provider 同名會讓 inbox 的 Register panic，由整合測試 TestAChargeProviderNamedLikeTheAccountProviderPanicsAtAssembly 守住（內部依賴圖不允許這裡 import account）。
func TestTheFakeProviderNameMatchesTheProviderColumnFormat(t *testing.T) {
	t.Parallel()
	if !regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`).MatchString(NewFake().Name()) {
		t.Fatalf("provider 名稱 %q 不符合格式", NewFake().Name())
	}
}

// constraintBody 取出 schema.sql 裡某個 CHECK 的整段內容（到該行結尾）。
func constraintBody(t *testing.T, constraint string) string {
	t.Helper()
	raw, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`CONSTRAINT ` + constraint + ` CHECK (.*)`).FindSubmatch(raw)
	if m == nil {
		t.Fatalf("schema.sql 沒有 %s", constraint)
	}
	return string(m[1])
}

// 外部 ID 不只在 SUCCEEDED 存得下來：provider 已回 ID 的狀態（退款 REFUNDING、建立之後才失敗的 FAILED、轉帳的 FAILED）都允許，入帳一定要有 ID，
// 還沒呼叫 provider 的狀態不能有。行為由整合測試 TestTheSchemaStoresTheExternalIDOfEveryStateThatHasOne 守，這裡守 schema 的宣告不被改回「ID 與 SUCCEEDED 一一對應」。
func TestExternalIDsAreAllowedInEveryStateTheProviderHasAlreadyAnswered(t *testing.T) {
	t.Parallel()
	for constraint, want := range map[string][]string{
		"refunds_external_state_check":   {"REFUNDING", "SUCCEEDED", "FAILED", "ANOMALY"},
		"transfers_external_state_check": {"SUCCEEDED", "FAILED"},
	} {
		if got := checkValuesIn(t, constraint); !slices.Equal(got, want) {
			t.Errorf("%s：schema=%v，want %v", constraint, got, want)
		}
	}
	for _, c := range []string{"refunds_succeeded_check", "transfers_succeeded_check"} {
		if body := constraintBody(t, c); regexp.MustCompile(`=\s*\(external_`).MatchString(body) {
			t.Errorf("%s 把外部 ID 綁死在 SUCCEEDED：%s", c, body)
		}
	}
}

// checkValuesIn 取出某個 CHECK 裡 status IN (...) 的清單。
func checkValuesIn(t *testing.T, constraint string) []string {
	t.Helper()
	body := constraintBody(t, constraint)
	m := regexp.MustCompile(`status IN \(([^)]*)\)`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("%s 沒有 status IN：%s", constraint, body)
	}
	var out []string
	for _, v := range regexp.MustCompile(`'([A-Z_]+)'`).FindAllStringSubmatch(m[1], -1) {
		out = append(out, v[1])
	}
	return out
}
