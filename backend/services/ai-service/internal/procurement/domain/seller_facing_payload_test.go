package domain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ⚠️ 同 package（domain）已有 ptr[T]（slots_test.go）與 int64p（quote_test.go），
// 不要重新定義。crop_resolution_test.go 的 strp 在 domain_test 外部測試 package，
// 這裡用不到也看不到。

// 編譯只讀 canonical_payload_json，不 join procurement_requests／revision——
// 那條 join 不受 payload_hash 保護（doc 64 §1.2）。preimage 已於本輪之前擴大到
// 十一欄，需要的識別內容全部在裡面。
func TestCompileSellerFacingPayload_ReadsOnlyCanonicalPayload(t *testing.T) {
	canonical := []byte(`{
		"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN",
		"buyerRequestedRange":{"min":"80","max":"120"},
		"item":"高麗菜",
		"itemResolution":{"kind":"SELF_PRICED","label":"高麗菜"},
		"quantity":100,
		"unit":"公斤",
		"packaging":"紙箱",
		"deliveryCity":"台中市",
		"deliveryDate":"2026-09-20"
	}`)

	got, err := CompileSellerFacingPayload(canonical, ptr("2026-09-15"))
	require.NoError(t, err)
	require.Equal(t, "高麗菜", got.Item)
	require.Equal(t, "2026-09-15", *got.ResponseDeadline)
	require.Equal(t, "高麗菜", got.ItemLabel)
	require.Equal(t, 100, *got.Quantity)
	require.Equal(t, "公斤", *got.Unit)
	require.Equal(t, "紙箱", *got.Packaging)
	require.Equal(t, "台中市", *got.DeliveryCity)
	require.Equal(t, "2026-09-20", *got.DeliveryDate)
}

func TestCompileSellerFacingPayload_ExcludesBuyerBudget(t *testing.T) {
	canonical := []byte(`{
		"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN",
		"item":"高麗菜",
		"budgetPerUnit":"95",
		"buyerRequestedRange":{
			"currency":"TWD","priceComponent":"GOODS_ONLY",
			"basisQuantity":"1","basisUnit":"KG",
			"lowerBoundAmount":"80","upperBoundAmount":"120"
		}
	}`)

	got, err := CompileSellerFacingPayload(canonical, nil)
	require.NoError(t, err)

	b, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(b), "budgetPerUnit")
	require.NotContains(t, string(b), `"95"`, "買家私有預算的值不得以任何鍵名外流")
	require.Contains(t, string(b), "buyerRequestedRange")
}

// canonical payload 未來擴欄時，新欄位**不得**自動流進賣家視野：編譯是白名單，
// 不是 passthrough。這條擋住「把整包 canonical 直接 marshal 出去」這個改法。
func TestCompileSellerFacingPayload_IsWhitelistNotPassthrough(t *testing.T) {
	canonical := []byte(`{
		"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN",
		"item":"高麗菜",
		"budgetPerUnit":"95",
		"marketQuote":{"midpoint":100},
		"someFutureBuyerPrivateField":"leak-me"
	}`)

	got, err := CompileSellerFacingPayload(canonical, nil)
	require.NoError(t, err)

	b, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(b), "budgetPerUnit")
	require.NotContains(t, string(b), "leak-me")
	require.NotContains(t, string(b), "marketQuote")
	require.NotContains(t, string(b), "midpoint")
}

// 內容不同就要 hash 不同——這是 #2538 買家接受時 CAS 的基礎。
func TestSellerFacingPayloadHash_DiscriminatesContent(t *testing.T) {
	base := []byte(`{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","buyerRequestedRange":{"min":"80","max":"120"},"item":"高麗菜","quantity":100}`)
	other := []byte(`{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","buyerRequestedRange":{"min":"80","max":"120"},"item":"高麗菜","quantity":200}`)

	p1, err := CompileSellerFacingPayload(base, nil)
	require.NoError(t, err)
	p2, err := CompileSellerFacingPayload(other, nil)
	require.NoError(t, err)

	h1, err := p1.Hash()
	require.NoError(t, err)
	h2, err := p2.Hash()
	require.NoError(t, err)
	require.NotEqual(t, h1, h2, "數量不同必須產生不同 hash")
}

// responseDeadline 是 publication 的屬性、由參數帶入，但它進 seller-facing payload，
// 所以必須被 seller_facing_payload_hash 涵蓋——execute 時才 CAS 得到「賣家看到的
// 截止日」有沒有被換掉。
func TestSellerFacingPayloadHash_CoversResponseDeadline(t *testing.T) {
	canonical := []byte(`{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","item":"高麗菜"}`)

	p1, err := CompileSellerFacingPayload(canonical, ptr("2026-09-15"))
	require.NoError(t, err)
	p2, err := CompileSellerFacingPayload(canonical, ptr("2026-09-16"))
	require.NoError(t, err)
	p3, err := CompileSellerFacingPayload(canonical, nil)
	require.NoError(t, err)

	h1, err := p1.Hash()
	require.NoError(t, err)
	h2, err := p2.Hash()
	require.NoError(t, err)
	h3, err := p3.Hash()
	require.NoError(t, err)

	require.NotEqual(t, h1, h2, "截止日不同必須產生不同 hash")
	require.NotEqual(t, h1, h3, "有截止日與沒有截止日必須產生不同 hash")
}

// canonical payload 壞掉時要回錯誤，不得回一份空殼 payload——空殼會被算出一個
// 看起來合法的 hash，然後靜默通過 execute 的 CAS。
func TestCompileSellerFacingPayload_RejectsMalformedJSON(t *testing.T) {
	_, err := CompileSellerFacingPayload([]byte(`{"item":`), nil)
	require.Error(t, err)
}

// 品相狀態要給賣家看（docx V.7.5 的排除清單不含它）；wire 上的 key 是 gateway
// DTO 與 web 共用的契約，key 錯與值錯一樣致命。
func TestCompileSellerFacingPayload_CarriesQualityKnowledgeState(t *testing.T) {
	canonical := []byte(`{
		"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN",
		"buyerRequestedRange":{"min":"80","max":"120"},
		"item":"高麗菜"
	}`)

	got, err := CompileSellerFacingPayload(canonical, nil)
	require.NoError(t, err)
	require.Equal(t, "BUYER_QUALITY_UNKNOWN", got.QualityKnowledgeState)

	b, err := json.Marshal(got)
	require.NoError(t, err)
	require.Contains(t, string(b), `"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN"`)
}

// 進 preimage；順帶釘住相容性：舊 approval 沒有這個 key，hash 因此不同。既有 row
// 的 hash 是存下來的不重算，受影響的只有跨部署重送同一個 previewClientRequestId
// 的 preview 冪等比對（repository/publication_approval.go）。
func TestSellerFacingPayloadHash_CoversQualityKnowledgeState(t *testing.T) {
	withState := []byte(`{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","item":"高麗菜"}`)
	withoutState := []byte(`{"item":"高麗菜"}`)

	p1, err := CompileSellerFacingPayload(withState, nil)
	require.NoError(t, err)
	p2, err := CompileSellerFacingPayload(withoutState, nil)
	require.NoError(t, err)

	h1, err := p1.Hash()
	require.NoError(t, err)
	h2, err := p2.Hash()
	require.NoError(t, err)
	require.NotEqual(t, h1, h2, "賣家看得到的品相狀態必須被 preimage 涵蓋")
}

func TestSellerFacingPayload_OmitsBuyerNoteWhenAbsent(t *testing.T) {
	payload, err := CompileSellerFacingPayload([]byte(`{
		"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","item":"珍珠芭樂"
	}`), nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if payload.BuyerNote != nil {
		t.Fatalf("BuyerNote = %v, want nil", *payload.BuyerNote)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "buyerNote") {
		t.Fatalf("沒有補充說明時 buyerNote 這個鍵不得出現在序列化結果裡（%s）——"+
			"它一出現，所有既有 payload_hash 就全部改變，這一刀就從 additive 變成破壞性契約變更", raw)
	}
}

func TestSellerFacingPayloadHash_CoversBuyerNote(t *testing.T) {
	const canonical = `{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","item":"珍珠芭樂"}`

	without, err := CompileSellerFacingPayload([]byte(canonical), nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	with, err := CompileSellerFacingPayload(
		[]byte(`{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","item":"珍珠芭樂","buyerNote":"中大果，不要有壓傷"}`), nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if with.BuyerNote == nil || *with.BuyerNote != "中大果，不要有壓傷" {
		t.Fatalf("BuyerNote = %v，canonical payload 的補充說明必須原文帶到賣家視角", with.BuyerNote)
	}

	hashWithout, err := without.Hash()
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	hashWith, err := with.Hash()
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hashWithout == hashWith {
		t.Fatalf("補充說明改了而 payload hash 沒變 —— 賣家看到的內容不在 CAS 的保護範圍內")
	}
}

const canonicalWithRanges = `{
  "qualityKnowledgeState": "BUYER_QUALITY_UNKNOWN",
  "item": "甘藍-初秋",
  "buyerRequestedRange": {
    "currency": "TWD", "priceComponent": "GOODS_ONLY",
    "basisQuantity": "1", "basisUnit": "BOX",
    "lowerBoundAmount": "38000", "upperBoundAmount": "43000"
  }
}`

func TestCompileSellerFacingPayload_CarriesPriceRanges(t *testing.T) {
	payload, err := CompileSellerFacingPayload([]byte(canonicalWithRanges), nil)
	require.NoError(t, err)

	require.NotNil(t, payload.BuyerRequestedRange, "買家希望詢價區間必須帶到賣家視角")
	require.Equal(t, SellerFacingRequestedRange{
		Currency: "TWD", PriceComponent: "GOODS_ONLY",
		BasisQuantity: "1", BasisUnit: "BOX",
		LowerBoundAmount: "38000", UpperBoundAmount: "43000",
	}, *payload.BuyerRequestedRange)
}

func TestCompileSellerFacingPayload_OmitsPriceRangeKeysWhenAbsent(t *testing.T) {
	payload, err := CompileSellerFacingPayload(
		[]byte(`{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","item":"珍珠芭樂"}`), nil)
	require.NoError(t, err)
	require.Nil(t, payload.BuyerRequestedRange)

	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "buyerRequestedRange")
}

func TestSellerFacingPayloadHash_CoversPriceRanges(t *testing.T) {
	without, err := CompileSellerFacingPayload(
		[]byte(`{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","item":"甘藍-初秋"}`), nil)
	require.NoError(t, err)
	with, err := CompileSellerFacingPayload([]byte(canonicalWithRanges), nil)
	require.NoError(t, err)

	hashWithout, err := without.Hash()
	require.NoError(t, err)
	hashWith, err := with.Hash()
	require.NoError(t, err)
	require.NotEqual(t, hashWithout, hashWith,
		"價格區間改了而 payload hash 沒變——賣家看到的內容不在 CAS 的保護範圍內")
}
