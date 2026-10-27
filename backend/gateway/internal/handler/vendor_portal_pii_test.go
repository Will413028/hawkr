package handler

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// public DTO 是 deny-by-default 白名單。欄位集合斷言擋的是「上游長胖時靜默
// 流出新欄位」——只擋已知 PII 欄名不夠。教訓見 API 設計規範 §3.1：
// 公開 DTO 不得帶店主的 email、已驗證手機、生日與街道地址。
func TestVendorPortalRFQDTOFieldSet(t *testing.T) {
	raw, err := json.Marshal(VendorPortalRFQDTO{})
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))

	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)

	// buyerNote 是買家自己打字、明知會給供應商看的補充說明——它進白名單是刻意的
	// 決定，不是漏擋。⛔ 買家私有預算 budgetPerUnit 仍然一律不得出現在這裡。
	//
	// publicationVersion 是第 13 欄，同樣是刻意加的：業者正式送出報價時要原樣帶回來
	// 做 CAS，把報價綁在「他當時看到的那一版 RFQ」上。它只說「你看到的是第幾版」，
	// 不洩漏任何買家內容。
	// ⛔ **sellerFacingPayloadHash 不得加進來**——那是我方的內部完整性摘要，交給站外
	// 的不受信任方只會多一個可被比對／重放的內部值，而它的驗證者本來就該是 server。
	require.Equal(t, []string{
		"buyerNote", "buyerRequestedRange", "deliveryCity", "deliveryDate",
		"item", "itemLabel", "packaging", "publicationVersion",
		"qualityKnowledgeState", "quantity",
		"referencePriceRange", "responseDeadline", "unit",
	}, got)
}

func TestVendorPortalPriceRangeDTOFieldSets(t *testing.T) {
	require.Equal(t, []string{
		"basisQuantity", "basisUnit", "lowerBoundAmount", "upperBoundAmount",
	}, jsonFieldNames(t, VendorPortalReferencePriceRangeDTO{}))

	require.Equal(t, []string{
		"basisQuantity", "basisUnit", "currency", "lowerBoundAmount",
		"priceComponent", "upperBoundAmount",
	}, jsonFieldNames(t, VendorPortalRequestedPriceRangeDTO{}))
}

func jsonFieldNames(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))

	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	return got
}

func TestVendorPortalRFQDTOHasNoPII(t *testing.T) {
	raw, err := json.Marshal(VendorPortalRFQDTO{
		ReferencePriceRange: &VendorPortalReferencePriceRangeDTO{},
		BuyerRequestedRange: &VendorPortalRequestedPriceRangeDTO{},
	})
	require.NoError(t, err)
	for _, banned := range []string{
		"email", "phone", "address", "budget", "midpoint",
		"buyerId", "accountId", "token", "secret",
	} {
		require.NotContains(t, string(raw), banned)
	}
}
