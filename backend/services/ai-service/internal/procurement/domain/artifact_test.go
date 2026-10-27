package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

func allArtifactTypes() []ArtifactType {
	return []ArtifactType{
		ArtifactRequestSnapshot, ArtifactClarification, ArtifactCandidateResults,
		ArtifactPricingPreview, ArtifactDealTemperature, ArtifactConfirmedRFQ,
		ArtifactRFQPublicationApproval, ArtifactRFQPublicationResult, ArtifactSellerQuoteResult,
		ArtifactErrorRecovery, ArtifactPurchasePlan, ArtifactCartApproval, ArtifactCartResult,
	}
}

func TestArtifactType_MatchesCodeMetaRegistry(t *testing.T) {
	// 逐一比對 CodeMeta 生成的 AllProcurementArtifactTypes，確保 domain 常數與
	// 業務類型碼.md 的 13 個值一一對應——新增/刪除 CodeMeta 值卻忘記更新這裡
	// 會在這裡先炸，而不是等到 golden fixture 才發現。
	require.Len(t, codemeta.AllProcurementArtifactTypes, 13, "doc 57 §4.5：Track A 10 型＋Track B 3 型＝13")
	require.Len(t, allArtifactTypes(), 13)

	registryValues := make([]string, 0, len(codemeta.AllProcurementArtifactTypes))
	for _, e := range codemeta.AllProcurementArtifactTypes {
		registryValues = append(registryValues, e.Value)
	}
	types := allArtifactTypes()
	domainValues := make([]string, 0, len(types))
	for _, t := range types {
		domainValues = append(domainValues, string(t))
	}
	require.ElementsMatch(t, registryValues, domainValues)
}

func TestArtifactType_TrackAAndTrackBAreDisjointAndComplete(t *testing.T) {
	// doc 57 §4.5：「兩個 Track 不得共用一個 generic schema」——先確保集合本身互斥
	// 且聯集覆蓋全部 13 型，回歸測試才能真正保護這條不變式。
	for _, at := range allArtifactTypes() {
		require.NotEqual(t, at.IsTrackA(), at.IsTrackB(), "ArtifactType %q 必須恰好屬於一個 Track", at)
	}

	wantTrackA := []ArtifactType{
		ArtifactRequestSnapshot, ArtifactClarification, ArtifactCandidateResults,
		ArtifactPricingPreview, ArtifactDealTemperature, ArtifactConfirmedRFQ,
		ArtifactRFQPublicationApproval, ArtifactRFQPublicationResult, ArtifactSellerQuoteResult,
		ArtifactErrorRecovery,
	}
	wantTrackB := []ArtifactType{ArtifactPurchasePlan, ArtifactCartApproval, ArtifactCartResult}

	for _, at := range wantTrackA {
		require.True(t, at.IsTrackA(), "%q 應屬 Track A", at)
	}
	for _, at := range wantTrackB {
		require.True(t, at.IsTrackB(), "%q 應屬 Track B", at)
	}
}

func TestArtifact_JSONRoundTrip_RequestSnapshot(t *testing.T) {
	item := "brand"
	data := RequestSnapshotData{
		RevisionID: 42,
		RevisionNo: 3,
		Slots:      Slots{Item: &item},
		SlotsHash:  "abc123",
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)

	asOf := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	artifact := Artifact{
		ArtifactID: 100,
		Type:       ArtifactRequestSnapshot,
		Version:    1,
		Checksum:   "sha256:deadbeef",
		Status:     ArtifactDraftStatus(),
		AsOf:       asOf,
		DomainRef:  "42",
		Data:       raw,
	}

	wire, err := json.Marshal(artifact)
	require.NoError(t, err)

	var decoded Artifact
	require.NoError(t, json.Unmarshal(wire, &decoded))
	require.Equal(t, artifact.ArtifactID, decoded.ArtifactID)
	require.Equal(t, artifact.Type, decoded.Type)
	require.True(t, decoded.AsOf.Equal(asOf))

	var decodedData RequestSnapshotData
	require.NoError(t, json.Unmarshal(decoded.Data, &decodedData))
	require.Equal(t, data.RevisionID, decodedData.RevisionID)
	require.Equal(t, *data.Slots.Item, *decodedData.Slots.Item)
}

func TestArtifact_IDIsWireString(t *testing.T) {
	// doc 57 §1.2-4：所有 ID 對外 JSON 一律 string，不得以數字序列化（JS 精度問題）。
	artifact := Artifact{ArtifactID: 9007199254740993, Type: ArtifactErrorRecovery, Status: ArtifactDraftStatus()}
	wire, err := json.Marshal(artifact)
	require.NoError(t, err)
	require.Contains(t, string(wire), `"artifactId":"9007199254740993"`)
}

func TestCandidateSummary_ExternalLaneOmitsSellerFields(t *testing.T) {
	// doc 57 §4.1：EXTERNAL lane 沒有平台賣家，四個賣家身分欄位必須整個省略而非空字串
	// （omitempty），不得偽造平台 seller identity。
	c := CandidateSummary{
		CandidateID: 1,
		Lane:        LaneExternal,
		DisplayName: "外部候選",
	}
	wire, err := json.Marshal(c)
	require.NoError(t, err)
	for _, key := range []string{"storeId", "sellerName", "storeCode", "logoKey"} {
		require.NotContainsf(t, string(wire), `"`+key+`"`, "EXTERNAL lane 不應帶 %s", key)
	}
}

func TestErrorRecoveryData_MirrorsProcurementErrorBodyShape(t *testing.T) {
	// doc 57 §4.7：錯誤 envelope 形狀沿用既有實作，不重新設計——本測試釘住欄位集合。
	data := ErrorRecoveryData{Reason: "STALE_REVISION", Message: "版本已過期", Retryable: true}
	wire, err := json.Marshal(data)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(wire, &decoded))
	require.ElementsMatch(t, []string{"reason", "message", "retryable"}, keysOf(decoded))
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestArtifactChecksum 守 doc 57 §4.5 的 checksum 欄位：同內容同值、不同內容不同值，
// 且格式與既有的 Slots.Hash 一致（sha256 hex）——兩處若各用一套雜湊，之後要對帳
// 「這版 artifact 跟那份 snapshot 是不是同一份事實」會多一層無謂的換算。
func TestArtifactChecksum(t *testing.T) {
	a := ArtifactChecksum([]byte(`{"a":1}`))
	b := ArtifactChecksum([]byte(`{"a":1}`))
	c := ArtifactChecksum([]byte(`{"a":2}`))

	if a != b {
		t.Errorf("同內容必須同 checksum: %q vs %q", a, b)
	}
	if a == c {
		t.Errorf("不同內容必須不同 checksum，都得到 %q", a)
	}
	if len(a) != 64 {
		t.Errorf("len = %d, want 64（sha256 hex，同 Slots.Hash）", len(a))
	}
}
