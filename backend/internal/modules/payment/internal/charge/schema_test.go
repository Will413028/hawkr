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
	raw, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`CONSTRAINT ` + constraint + ` CHECK \(status IN \(([^)]*)\)\)`).FindSubmatch(raw)
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
}

// provider 名稱同時是 payments.provider 與 inbox 的 provider 名稱：必須符合兩邊共同的格式（payments_provider_check、inbox 的 providerPattern）。
// 與收款帳戶 provider 同名會讓 inbox 的 Register panic，由整合測試 TestAChargeProviderNamedLikeTheAccountProviderPanicsAtAssembly 守住（內部依賴圖不允許這裡 import account）。
func TestTheFakeProviderNameMatchesTheProviderColumnFormat(t *testing.T) {
	t.Parallel()
	if !regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`).MatchString(NewFake().Name()) {
		t.Fatalf("provider 名稱 %q 不符合格式", NewFake().Name())
	}
}
