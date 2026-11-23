package quote

import (
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

var now = time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)

func num(v int64, exp int32) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(v), Exp: exp, Valid: true}
}

func ptr[T any](v T) *T { return &v }

func TestVersionStatus(t *testing.T) {
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	for name, c := range map[string]struct {
		version, latest int32
		withdrawn       *int32
		validUntil      time.Time
		want            string
	}{
		"目前版、有效":     {2, 2, nil, future, model.ResponseActive},
		"目前版、過期":     {2, 2, nil, past, model.ResponseExpired},
		"剛好到期那一刻還有效": {2, 2, nil, now, model.ResponseActive},
		"被取代":        {1, 2, nil, future, model.ResponseSuperseded},
		"目前版已撤回":     {2, 2, ptr(int32(2)), future, model.ResponseWithdrawn},
		"撤回的是別的版本":   {2, 2, ptr(int32(1)), future, model.ResponseActive},
		"已撤回優先於被取代":  {1, 2, ptr(int32(1)), future, model.ResponseWithdrawn},
		"被取代優先於過期":   {1, 2, nil, past, model.ResponseSuperseded},
		"撤回優先於過期":    {2, 2, ptr(int32(2)), past, model.ResponseWithdrawn},
	} {
		if got := VersionStatus(c.version, c.latest, c.withdrawn, c.validUntil, now); got != c.want {
			t.Errorf("%s：%s，要 %s", name, got, c.want)
		}
	}
}

func TestAcceptability(t *testing.T) {
	for name, c := range map[string]struct {
		status       string
		closed, live bool
		want         string
	}{
		"進行中":      {model.ResponseActive, false, false, model.AcceptabilityAcceptable},
		"賣家停止寫入":   {model.ResponseActive, true, false, model.AcceptabilityIneligible},
		"這張需求已有成交": {model.ResponseActive, false, true, model.AcceptabilityIneligible},
		"撤回":       {model.ResponseWithdrawn, true, true, model.AcceptabilityWithdrawn},
		"被取代":      {model.ResponseSuperseded, false, false, model.AcceptabilitySuperseded},
		"過期":       {model.ResponseExpired, false, false, model.AcceptabilityExpired},
	} {
		if got := Acceptability(c.status, c.closed, c.live); got != c.want {
			t.Errorf("%s：%s，要 %s", name, got, c.want)
		}
	}
}

func version(goods int64, offered pgtype.Numeric, offeredUnit string) store.ProcurementSellerResponseVersion {
	return store.ProcurementSellerResponseVersion{ID: uuid.NewV7(), OfferedQuantity: offered, OfferedQuantityUnit: offeredUnit, GoodsAmount: goods}
}

func TestOutsideRange(t *testing.T) {
	// 區間每 1 KG 100 到 200 元；報 3 KG 時貨款 300 到 600 在區間內，報 2.5 KG 時 250 到 500。
	for name, c := range map[string]struct {
		v    store.ProcurementSellerResponseVersion
		want bool
	}{
		"下界剛好":        {version(300, num(3000, -3), "KG"), false},
		"上界剛好":        {version(600, num(3000, -3), "KG"), false},
		"低於下界":        {version(299, num(3000, -3), "KG"), true},
		"高於上界":        {version(601, num(3000, -3), "KG"), true},
		"整數表示的數量":     {version(601, num(3, 0), "KG"), true},
		"數量不是整數倍、區間內": {version(250, num(2500, -3), "KG"), false},
		"數量不是整數倍、低於":  {version(249, num(2500, -3), "KG"), true},
		"數量不是整數倍、高於":  {version(501, num(2500, -3), "KG"), true},
		"小於一個基準、區間內":  {version(50, num(500, -3), "KG"), false},
		"小於一個基準、高於":   {version(101, num(500, -3), "KG"), true},
		"單位不同不比較":     {version(1, num(3000, -3), "G"), false},
		"數量是零不比較":     {version(1, num(0, -3), "KG"), false},
		"數量不是有限數不比較":  {version(1, pgtype.Numeric{NaN: true, Valid: true}, "KG"), false},
	} {
		if got := OutsideRange(100, 200, 1, "KG", c.v); got != c.want {
			t.Errorf("%s：%v，要 %v", name, got, c.want)
		}
	}
	// 克的計價基準是每 1000 個：每 1000 G 100 到 200 元，報 1500 G 時 150 到 300，報 2500 G 時 250 到 500。
	for name, c := range map[string]struct {
		v    store.ProcurementSellerResponseVersion
		want bool
	}{
		"1500 G 區間內":  {version(150, num(1500, 0), "GRAM"), false},
		"1500 G 低於":   {version(149, num(1500, 0), "GRAM"), true},
		"1500 G 高於":   {version(301, num(1500, 0), "GRAM"), true},
		"2500 G 上界剛好": {version(500, num(2500, 0), "GRAM"), false},
		"3000 G 低於":   {version(299, num(3000, 0), "GRAM"), true},
		"報價用公斤不比較":    {version(1, num(3, 0), "KG"), false},
		"1005 G 下界 100.5 進位成 101、貨款 100 低於": {version(100, num(1005, 0), "GRAM"), true},
		"1005 G 貨款 101 剛好":                  {version(101, num(1005, 0), "GRAM"), false},
	} {
		if got := OutsideRange(100, 200, 1000, "GRAM", c.v); got != c.want {
			t.Errorf("%s：%v，要 %v", name, got, c.want)
		}
	}
	// 乘積超過 int64 也不溢位。
	if !OutsideRange(1<<62, 1<<62, 1, "KG", version(1, num(8, 0), "KG")) {
		t.Error("下界乘數量超過 int64 時貨款一定低於它")
	}
}

func TestDecimal3AndMilli(t *testing.T) {
	for name, c := range map[string]struct {
		n    pgtype.Numeric
		want string
	}{
		"三位小數":    {num(3000, -3), "3.000"},
		"整數":      {num(3, 0), "3.000"},
		"一位小數":    {num(35, -1), "3.500"},
		"帶指數":     {num(3, 2), "300.000"},
		"小於一":     {num(5, -3), "0.005"},
		"尾端有多餘的零": {num(3500, -4), "0.350"},
	} {
		got, err := decimal3(c.n)
		if err != nil || got != c.want {
			t.Errorf("%s：%q %v，要 %q", name, got, err, c.want)
		}
	}
	for name, n := range map[string]pgtype.Numeric{
		"超過三位小數": num(1, -4), "NaN": {NaN: true, Valid: true}, "NULL": {}, "負數": num(-1, 0),
		"無限大": {InfinityModifier: pgtype.Infinity, Valid: true},
	} {
		if got, err := decimal3(n); err == nil {
			t.Errorf("%s 要回錯誤，得到 %q", name, got)
		}
	}
}

func TestPayloadHashIsTheRecomputedContentHash(t *testing.T) {
	v := store.ProcurementSellerResponseVersion{
		ID: uuid.NewV7(), SellerResponseID: uuid.NewV7(), Version: 1, ResponseType: "MANUAL_QUOTE", OfferedQualityGrade: "A", UnitBasisQuantity: num(1, 0), UnitBasisUnit: "KG",
		OfferedQuantity: num(3, 0), OfferedQuantityUnit: "KG", Currency: "TWD", GoodsAmount: 900, ShippingAmount: 100, TaxAmount: 50, TaxInclusionBasis: "TAX_EXCLUDED", TotalAmount: 1050,
		ValidUntil: now, SubmittedAt: now, ClientRequestID: uuid.NewV7(),
	}
	q, err := QuoteOf(v)
	if err != nil {
		t.Fatal(err)
	}
	a, err := PayloadHash(q)
	if err != nil || len(a) != 64 {
		t.Fatalf("%q %v", a, err)
	}
	// 不在原像裡的欄位（ID、版本號、送出時間、冪等鍵）改了，hash 不變；數量換成等值的表示也不變。
	v.ID, v.Version, v.SubmittedAt, v.ClientRequestID, v.OfferedQuantity = uuid.NewV7(), 9, now.Add(time.Hour), uuid.NewV7(), num(3000, -3)
	q2, _ := QuoteOf(v)
	if b, _ := PayloadHash(q2); a != b {
		t.Error("ID、版本號、送出時間與冪等鍵不該影響 hash")
	}
	v.GoodsAmount++
	v.TotalAmount++
	q3, _ := QuoteOf(v)
	if c, _ := PayloadHash(q3); a == c {
		t.Error("金額改了 hash 要變")
	}
}

func TestPageTokenRoundTripsAndRejectsGarbage(t *testing.T) {
	id := uuid.NewV7()
	at := time.Date(2026, 11, 23, 3, 0, 0, 123456000, time.UTC)
	got, ok := decodeToken(encodeToken(at, id))
	if !ok || got.ID != id || time.UnixMicro(got.CreatedAt).UTC() != at {
		t.Fatalf("%+v %v", got, ok)
	}
	for _, raw := range []string{"x", "!!!", "e30", "eyJpIjoiMDAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAwIiwicyI6MX0", encodeToken(time.Date(-5000, 1, 1, 0, 0, 0, 0, time.UTC), id)} {
		if _, ok := decodeToken(raw); ok {
			t.Errorf("%q 不是有效的 token", raw)
		}
	}
}
