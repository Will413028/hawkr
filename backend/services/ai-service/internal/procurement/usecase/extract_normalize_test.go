package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

// 2026-08-15 買家實測：說「高麗菜」，LLM（EXTRACTOR_MODEL=deepseek-flash）抽取結果
// 變成簡體「高丽菜」，跟 pricebook／product 目錄的繁體字比對失敗，候選搜尋 0 家。
// retryingExtractor 是三個 provider 共用的既定插入點，在這裡正規化，provider
// 換哪一個都受益，不用各自實作。
func TestRetryingExtractor_NormalizesSimplifiedChineseInTextFields(t *testing.T) {
	inner := &scriptedExtractor{results: []scriptedResult{{patch: domain.SlotPatch{
		Item: ptr("高丽菜"), Packaging: ptr("纸箱"),
	}}}}
	re := &retryingExtractor{inner: inner}

	result, err := re.Extract(context.Background(), "msg", domain.Slots{})

	require.NoError(t, err)
	require.Equal(t, "高麗菜", *result.Patch.Item)
	require.Equal(t, "紙箱", *result.Patch.Packaging)
}

// 已經是繁體、或欄位本來就是 nil（LLM 這輪沒填這個欄位）都不能被正規化步驟
// 意外動到——nil 一旦被誤轉成非 nil 空字串，會被 Slots.Apply 誤判成「買家清空
// 了這個欄位」而不是「這輪沒提到」。
func TestRetryingExtractor_LeavesNilAndTraditionalFieldsUnchanged(t *testing.T) {
	inner := &scriptedExtractor{results: []scriptedResult{{patch: domain.SlotPatch{
		Item: ptr("珍珠芭樂"), Packaging: nil,
	}}}}
	re := &retryingExtractor{inner: inner}

	result, err := re.Extract(context.Background(), "msg", domain.Slots{})

	require.NoError(t, err)
	require.Equal(t, "珍珠芭樂", *result.Patch.Item)
	require.Nil(t, result.Patch.Packaging)
}

// 抽取本身失敗時（例如 400 不可重試錯誤）不該去正規化一個沒意義的 patch——
// 這裡主要確認正規化步驟不會讓錯誤被吞掉或改變錯誤本身。
func TestRetryingExtractor_NormalizationDoesNotSwallowExtractError(t *testing.T) {
	wantErr := errors.New("400 invalid argument")
	inner := &scriptedExtractor{results: []scriptedResult{
		{err: wantErr},
	}}
	re := &retryingExtractor{inner: inner}

	_, err := re.Extract(context.Background(), "msg", domain.Slots{})

	require.ErrorIs(t, err, wantErr)
}

func TestRetryingExtractor_KeepsBuyerOwnCharactersVerbatim(t *testing.T) {
	inner := &scriptedExtractor{results: []scriptedResult{{patch: domain.SlotPatch{
		Item: ptr("虱目魚"),
	}}}}
	re := &retryingExtractor{inner: inner}

	result, err := re.Extract(context.Background(), "我要買虱目魚 50 公斤，送台南市", domain.Slots{})

	require.NoError(t, err)
	require.Equal(t, "虱目魚", *result.Patch.Item,
		"買家原文裡就有這幾個字，簡轉繁不得再動它——否則賣家收到的 RFQ 上是「蝨目魚」")
}

func TestRetryingExtractor_VerbatimPreservationTable(t *testing.T) {
	cases := []struct {
		name      string
		message   string
		extracted string
		want      string
	}{
		{"異體字且原文有 → 原字保留", "我要買虱目魚 50 公斤", "虱目魚", "虱目魚"},
		{"異體字的另一種寫法同理", "我要買蝨目魚 50 公斤", "蝨目魚", "蝨目魚"},
		{"模型吐簡體且原文沒有 → 轉繁體", "我要買高麗菜 50 公斤", "高丽菜", "高麗菜"},
		{"抽出的是原文子字串 → 原樣保留", "我想要一些有機小白菜", "有機小白菜", "有機小白菜"},
		{"模型回台而原文是臺 → 轉換後正好還原成原文的寫法", "我要買臺灣米", "台灣米", "臺灣米"},
		{"已是繁體又在原文裡 → 不動", "我要買珍珠芭樂", "珍珠芭樂", "珍珠芭樂"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inner := &scriptedExtractor{results: []scriptedResult{{patch: domain.SlotPatch{Item: ptr(c.extracted)}}}}
			re := &retryingExtractor{inner: inner}

			result, err := re.Extract(context.Background(), c.message, domain.Slots{})

			require.NoError(t, err)
			require.Equal(t, c.want, *result.Patch.Item,
				"買家原文裡已經存在的字串不得被簡轉繁改寫；只有模型自己生出來、原文沒有的才轉。"+
					"2026-09-18 dev 實測「虱目魚」被寫成「蝨目魚」，賣家收到的 RFQ 上就是錯字")
		})
	}
}
