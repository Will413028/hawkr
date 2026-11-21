package reply

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 對照測試的「舊」指舊 ai-service 的 domain/（dialogue_templates.go、recorded.go、pick_next_question.go、slots.go）。
// 預期文字取自舊 testdata/dialogue_templates/*.json 的 populatedArgsBlocks，zh-TW 逐字相同。

const zh, en = uitext.ZhTW, uitext.EN

func b(k BlockKind, text string) Block { return Block{k, text} }

func TestRenderParity(t *testing.T) {
	disamb := planner.BuildItemDisambiguation([]planner.ItemCandidate{{CategoryID: 1, Label: "小白菜"}})
	packaging := planner.BuildFor(slots.Packaging, slots.Set{})
	cases := []struct {
		name string
		src  string
		plan planner.Plan
		args Args
		want []Block
	}{
		{"ack and ask required, no facts", "testdata/dialogue_templates/ack_and_ask_required.json (emptyArgsBlocks)",
			planner.Plan{Act: planner.AckAndAskRequired}, Args{},
			[]Block{b(Understood, "已收到你的訊息。")}},
		{"ack and ask required, with facts and missing", "testdata/dialogue_templates/ack_and_ask_required.json (populatedArgsBlocks)",
			planner.Plan{Act: planner.AckAndAskRequired, Salient: []slots.Recorded{{Key: slots.Item, Value: "寶島梨"}}},
			Args{Missing: []model.SlotKey{slots.DeliveryCity, slots.Quantity}},
			[]Block{b(Understood, "已記錄：品項：寶島梨"), b(NextStep, "還想請你補充：配送城市、數量")}},
		{"ack and ask optional uses the question prompt", "testdata/dialogue_templates/ack_and_ask_optional.json; dialogue_templates_test.go:55",
			planner.Plan{Act: planner.AckAndAskOptional, Question: packaging}, Args{},
			[]Block{b(Understood, "好的，主要資訊都到齊了。"), b(NextStep, "包裝上有什麼需求嗎？")}},
		{"correction applied", "testdata/dialogue_templates/correction_applied.json; dialogue_templates_test.go:64",
			planner.Plan{Act: planner.CorrectionApplied, Salient: []slots.Recorded{{Key: slots.Quantity, Value: "50"}}}, Args{},
			[]Block{b(Understood, "已經幫你更新：數量：50")}},
		{"input rejected", "testdata/dialogue_templates/input_rejected.json; dialogue_templates_test.go:73",
			planner.Plan{Act: planner.InputRejected}, Args{Rejected: []slots.Rejection{{Key: slots.DeliveryCity, Reason: slots.ReasonCityUnrecognized}}},
			[]Block{b(Warning, "有幾個內容我沒辦法直接採用：配送城市：不是可辨識的縣市名稱"), b(NextStep, "麻煩再確認一下。")}},
		{"extraction degraded", "testdata/dialogue_templates/extraction_degraded.json",
			planner.Plan{Act: planner.ExtractionDegraded}, Args{},
			[]Block{b(Warning, "需求解析暫時無法使用。"), b(Context, "已保留你目前的需求內容，可以繼續補充或稍後再試。")}},
		{"request basics ready", "testdata/dialogue_templates/request_basics_ready.json",
			planner.Plan{Act: planner.RequestBasicsReady}, Args{},
			[]Block{b(Understood, "好的，需求已經齊全了。"), b(NextStep, "我現在就去幫你找找看合適的候選。")}},
		{"price preview available", "testdata/dialogue_templates/price_preview_available.json",
			planner.Plan{Act: planner.PricePreviewAvailable}, Args{},
			[]Block{b(Context, "已經有市場參考價可以看了。"), b(NextStep, "要不要先確認一下希望詢價區間？")}},
		{"rfq reconfirm required", "testdata/dialogue_templates/rfq_reconfirm_required.json",
			planner.Plan{Act: planner.RfqReconfirmRequired}, Args{},
			[]Block{b(Warning, "需求內容有更新，之前確認的 RFQ 快照已經過期。"), b(NextStep, "需要重新確認一次。")}},
		{"publication approval required", "testdata/dialogue_templates/publication_approval_required.json",
			planner.Plan{Act: planner.PublicationApprovalRequired}, Args{},
			[]Block{b(Complete, "RFQ 快照已經確認好了。"), b(Rationale, "等你核准後才會正式送出給賣家。")}},
		{"item needs disambiguation uses the question prompt", "testdata/dialogue_templates/item_needs_disambiguation.json; dialogue_templates_test.go:137",
			planner.Plan{Act: planner.ItemNeedsDisambiguation, Question: disamb}, Args{},
			[]Block{b(NextStep, "找到幾個相符的品項，麻煩選一下你要的是哪一個：")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Render(zh, c.plan, c.args); !reflect.DeepEqual(got, c.want) {
				t.Errorf("(%s)\n got %v\nwant %v", c.src, got, c.want)
			}
		})
	}
}

// 舊 dialogue_templates_test.go:10：每個 act 在每個語系都有段落，段落有種類與文字。
func TestEveryActRendersInEveryLocale(t *testing.T) {
	q := planner.BuildFor(slots.Packaging, slots.Set{})
	for _, l := range []uitext.Locale{zh, en} {
		for _, act := range planner.All {
			plan := planner.Plan{Act: act, Question: q, Salient: []slots.Recorded{{Key: slots.Item, Value: "梨"}}}
			args := Args{Missing: []model.SlotKey{slots.Unit}, Rejected: []slots.Rejection{{Key: slots.Unit, Reason: slots.ReasonUnitNotAllowed}}}
			blocks := Render(l, plan, args)
			if len(blocks) == 0 {
				t.Errorf("%s/%s: no blocks", l, act)
			}
			for _, blk := range blocks {
				if blk.Kind == "" || strings.TrimSpace(blk.Text) == "" {
					t.Errorf("%s/%s: empty block %+v", l, act, blk)
				}
			}
		}
	}
}

func TestRenderEnglishIsNotChinese(t *testing.T) {
	for _, act := range planner.All {
		plan := planner.Plan{Act: act, Question: planner.BuildFor(slots.Packaging, slots.Set{}), Salient: []slots.Recorded{{Key: slots.Item, Value: "pear"}}}
		for _, blk := range Render(en, plan, Args{Missing: []model.SlotKey{slots.Unit}, Rejected: []slots.Rejection{{Key: slots.Unit, Reason: slots.ReasonUnitNotAllowed}}}) {
			for _, r := range blk.Text {
				if r >= 0x4e00 && r <= 0x9fff {
					t.Errorf("%s: en text contains Chinese: %q", act, blk.Text)
					break
				}
			}
		}
	}
}

func TestTextFallbackJoinsWithSpace(t *testing.T) { // dialogue_templates_test.go:125,133
	got := TextFallback([]Block{b(Understood, "已收到你的需求。"), b(NextStep, "還想請你補充：配送城市。")})
	if got != "已收到你的需求。 還想請你補充：配送城市。" {
		t.Errorf("got %q", got)
	}
	if TextFallback(nil) != "" {
		t.Error("empty blocks must give an empty string")
	}
}

// 舊 slots.go 的拒絕原因字串（SlotLabels 在 recorded.go:393；原因在 slots.go:131-221、normalize.go:368）。
func TestRejectionReasonsMatchOldTextInZhTW(t *testing.T) {
	want := map[slots.Reason]string{
		slots.ReasonEmpty:            "空值，未採用",
		slots.ReasonTruncated:        "超過 200 字，已截斷",
		slots.ReasonQuantityRange:    "數量需介於 1 與 1000000 之間",
		slots.ReasonBudgetRange:      "單價預算需介於 1 與 10000000 元之間",
		slots.ReasonUnitNotAllowed:   "單位不在允許清單內：KG/CATTY/GRAM",
		slots.ReasonCityUnrecognized: "不是可辨識的縣市名稱",
		slots.ReasonDateFormat:       "日期格式需為 YYYY-MM-DD",
		slots.ReasonDateMissingYear:  "缺少西元年份，請補上年份（例如 2026-08-20）",
		slots.ReasonDatePast:         "到貨日不可早於今天",
		slots.ReasonDateTooFar:       "到貨日不可超過 365 天後",
		slots.ReasonResolutionNoItem: "沒有品項可以解析",
	}
	for reason, text := range want {
		if got := RejectionReason(zh, slots.Rejection{Key: slots.Item, Reason: reason}); got != text {
			t.Errorf("%s = %q, want %q", reason, got, text)
		}
	}
	for _, r := range []slots.Reason{
		slots.ReasonEmpty, slots.ReasonTruncated, slots.ReasonQuantityRange, slots.ReasonBudgetRange, slots.ReasonUnitNotAllowed,
		slots.ReasonCityUnrecognized, slots.ReasonDateFormat, slots.ReasonDateMissingYear, slots.ReasonDatePast, slots.ReasonDateTooFar,
		slots.ReasonResolutionNoItem, slots.ReasonResolutionNoLabel, slots.ReasonResolutionUnknownKind,
	} {
		for _, l := range []uitext.Locale{zh, en} {
			if got := RejectionReason(l, slots.Rejection{Reason: r}); got == "" || got == string(r) {
				t.Errorf("%s/%s has no text (got %q)", l, r, got)
			}
		}
	}
}

func TestSlotLabelsMatchOldTable(t *testing.T) { // recorded.go:393
	want := map[model.SlotKey]string{
		slots.Item: "品項", slots.ItemResolution: "品項分類", slots.Quantity: "數量", slots.Unit: "單位", slots.Packaging: "包裝",
		slots.BudgetPerUnit: "單價預算", slots.DeliveryCity: "配送城市", slots.DeliveryDate: "到貨日期", slots.ExactMatchRequired: "是否需完全相符",
	}
	for k, w := range want {
		if got := SlotLabel(zh, k); got != w {
			t.Errorf("SlotLabel(%s) = %q, want %q", k, got, w)
		}
		if got := SlotLabel(en, k); got == "" || got == string(k) || got == w {
			t.Errorf("SlotLabel(en, %s) = %q", k, got)
		}
	}
}

func TestRecordedValueRendersBooleans(t *testing.T) { // recorded.go:496
	if got := RecordedValue(zh, slots.Recorded{Key: slots.ExactMatchRequired, Value: "true"}); got != "是" {
		t.Errorf("true = %q", got)
	}
	if got := RecordedValue(zh, slots.Recorded{Key: slots.ExactMatchRequired, Value: "false"}); got != "否" {
		t.Errorf("false = %q", got)
	}
	if got := RecordedValue(zh, slots.Recorded{Key: slots.Quantity, Value: "50"}); got != "50" {
		t.Errorf("quantity = %q", got)
	}
}

// 問題卡的文字（pick_next_question.go:62 SlotPrompts、:75 SlotOptions、:235-243 消歧選項）。
func TestQuestionTextParity(t *testing.T) {
	prompts := map[model.SlotKey]string{
		slots.Item: "這次想採購什麼品項？", slots.DeliveryCity: "要送到哪個城市呢？", slots.Packaging: "包裝上有什麼需求嗎？",
		slots.BudgetPerUnit: "單價預算大概抓多少？", slots.DeliveryDate: "希望什麼時候送達？",
	}
	for k, w := range prompts {
		if got := Question(zh, planner.BuildFor(k, slots.Set{})).Prompt; got != w {
			t.Errorf("prompt(%s) = %q, want %q", k, got, w)
		}
	}
	both := Question(zh, planner.BuildFor(slots.Quantity, slots.Set{}))
	if both.Prompt != "大概需要多少數量？" {
		t.Errorf("compound prompt = %q", both.Prompt)
	}
	var units []string
	for _, o := range both.UnitOptions {
		units = append(units, o.ID+"="+o.Label)
	}
	if want := []string{"unit-kg=公斤", "unit-catty=台斤", "unit-gram=公克"}; !reflect.DeepEqual(units, want) {
		t.Errorf("unit options = %v", units)
	}
	followUp := Question(zh, planner.BuildFor(slots.Quantity, slots.Set{Quantity: ptr(int32(5))}))
	if followUp.Prompt != "這批貨要用什麼單位計算？" {
		t.Errorf("unit follow-up prompt = %q", followUp.Prompt)
	}
	city := Question(zh, planner.BuildFor(slots.DeliveryCity, slots.Set{}))
	if len(city.Options) != 6 || city.Options[0].ID != "city-taipei" || city.Options[0].Label != "臺北市" {
		t.Errorf("city options = %+v", city.Options)
	}

	choices := Question(zh, planner.BuildItemDisambiguation([]planner.ItemCandidate{{CategoryID: 18, Label: "有機小白菜"}}))
	if choices.Prompt != "找到幾個相符的品項，麻煩選一下你要的是哪一個：" || choices.Options[0].Label != "有機小白菜" || choices.Options[1].Label != "其他（自己輸入）" {
		t.Errorf("choices = %+v", choices)
	}
	none := Question(zh, planner.BuildItemDisambiguation(nil))
	if none.Prompt != "找不到符合的品項，請描述你要的品項：" || none.Options[0].Label != "自行描述品項" {
		t.Errorf("none = %+v", none)
	}
	if Question(zh, nil) != nil {
		t.Error("nil question must stay nil")
	}
}

func ptr[T any](v T) *T { return &v }
