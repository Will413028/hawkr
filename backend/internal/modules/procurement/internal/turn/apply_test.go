package turn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/reply"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// spread 是 n 個命中，依序落在 roots 個大類（輪流）。
func spread(n int, roots ...string) []resolver.Match {
	var out []resolver.Match
	for i := 0; i < n; i++ {
		root := roots[i%len(roots)]
		out = append(out, match(int64(100+i), fmt.Sprintf("品%d", i), root+" > 品"+fmt.Sprint(i)))
	}
	return out
}

func optionIDs(p InterruptPayload) []string {
	var ids []string
	for _, o := range p.Question.Options {
		ids = append(ids, o.OptionID)
	}
	return ids
}

func cardFromMatches(ms []resolver.Match) InterruptPayload {
	return buildPayload(buildCard("蘋果", ms), uitext.ZhTW)
}

func buildPayload(c *pendingCard, l uitext.Locale) InterruptPayload {
	return *interrupt(l, c).Next
}

func TestGroupCardOnlyWhenMoreThanFiveAcrossGroups(t *testing.T) {
	for name, tc := range map[string]struct {
		ms        []resolver.Match
		stage     string
		wantCands int
	}{
		"6 across 2 groups asks the group": {spread(6, "水果", "加工品"), StageGroup, 2},
		"7 across 7 groups caps at 5":      {spread(7, "a", "b", "c", "d", "e", "f", "g"), StageGroup, 5},
		"6 in one group stays item":        {spread(6, "水果"), StageItem, 5},
		"5 across 2 groups stays item":     {spread(5, "水果", "加工品"), StageItem, 5},
		"2 across 2 groups stays item":     {spread(2, "水果", "加工品"), StageItem, 2},
	} {
		t.Run(name, func(t *testing.T) {
			p := cardFromMatches(tc.ms)
			if p.Stage != tc.stage || len(p.Candidates) != tc.wantCands {
				t.Fatalf("stage=%s candidates=%d，要 %s/%d", p.Stage, len(p.Candidates), tc.stage, tc.wantCands)
			}
			if len(p.Question.Options) != tc.wantCands+1 || !p.Question.AllowFreeText {
				t.Fatalf("卡要有 %d 個選項加其他並開放自由輸入：%+v", tc.wantCands, p.Question)
			}
		})
	}
}

func TestGroupCardListsGroupsByBestMatchOrder(t *testing.T) {
	ms := []resolver.Match{match(1, "a", "乙 > a"), match(2, "b", "甲 > b"), match(3, "c", "乙 > c"), match(4, "d", "丙 > d"), match(5, "e", "甲 > e"), match(6, "f", "乙 > f")}
	p := cardFromMatches(ms)
	var got []string
	for _, c := range p.Candidates {
		got = append(got, c.Label)
		if c.Path != c.Label {
			t.Fatalf("大類卡的候選路徑就是大類：%+v", c)
		}
	}
	if want := []string{"乙", "甲", "丙"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("大類順序 %v，要 %v", got, want)
	}
	if p.Candidates[0].CategoryID != rootID("乙") {
		t.Fatalf("候選存的是大類 ID：%+v", p.Candidates[0])
	}
}

func TestGroupCardNeedsGroupInfo(t *testing.T) {
	ms := spread(6, "水果", "加工品")
	ms[3] = match(900, "無路徑", "")
	if p := cardFromMatches(ms); p.Stage != StageItem {
		t.Fatalf("有命中沒有大類資訊不分組：%s", p.Stage)
	}
}

func apply(t *testing.T, rs Resolver, p InterruptPayload, a Answer) Outcome {
	t.Helper()
	o, err := ApplyAnswer(context.Background(), rs, uitext.ZhTW, now, p, a)
	if err != nil {
		t.Fatalf("ApplyAnswer：%v", err)
	}
	return o
}

func groupPayload(t *testing.T) InterruptPayload {
	t.Helper()
	p := cardFromMatches(spread(6, "水果", "加工品"))
	if p.Stage != StageGroup {
		t.Fatal("前置：要是 GROUP 卡")
	}
	return p
}

func groupOpt(name string) string { return "category-" + fmt.Sprint(rootID(name)) }

func TestApplyGroupChoice(t *testing.T) {
	all := spread(6, "水果", "加工品") // 水果：品0 品2 品4；加工品：品1 品3 品5
	t.Run("one match in the group confirms the leaf, never the group", func(t *testing.T) {
		rs := &fakeResolver{res: resolver.Resolution{Matches: append([]resolver.Match{}, all[:2]...)}}
		o := apply(t, rs, groupPayload(t), Answer{OptionID: groupOpt("水果")})
		if o.Kind != OutcomeResolved || o.Resolution.CategoryID != 100 || o.Resolution.Kind != slots.CategoryConfirmed || o.Resolution.Label != "品0" {
			t.Fatalf("要確認末層 100：%+v", o)
		}
	})
	t.Run("several matches in the group ask an item card with paths", func(t *testing.T) {
		rs := &fakeResolver{res: resolver.Resolution{Matches: all}}
		o := apply(t, rs, groupPayload(t), Answer{OptionID: groupOpt("水果")})
		if o.Kind != OutcomeInterrupt || o.Next.Stage != StageItem || len(o.Next.Candidates) != 3 {
			t.Fatalf("要是 3 個候選的 ITEM 卡：%+v", o)
		}
		for _, c := range o.Next.Candidates {
			if !strings.HasPrefix(c.Path, "水果 > ") {
				t.Fatalf("只列該大類底下的：%+v", c)
			}
		}
		if o.Next.Query != "蘋果" {
			t.Fatalf("Query 保留：%q", o.Next.Query)
		}
	})
	t.Run("matches that are not leaves are ignored", func(t *testing.T) {
		root := match(1, "水果", "水果")
		root.Leaf = false
		rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{root, all[0]}}}
		o := apply(t, rs, groupPayload(t), Answer{OptionID: groupOpt("水果")})
		if o.Kind != OutcomeResolved || o.Resolution.CategoryID != 100 {
			t.Fatalf("大類本身不是結果：%+v", o)
		}
	})
	t.Run("no match in the group lists the group leaves in taxonomy order, at most five", func(t *testing.T) {
		var leaves []resolver.Match
		for i := 0; i < 7; i++ {
			leaves = append(leaves, match(int64(500+i), fmt.Sprintf("葉%d", i), fmt.Sprintf("水果 > 葉%d", i)))
		}
		rs := &fakeResolver{res: resolver.Resolution{Matches: all[1:2]}, leaves: map[int64][]resolver.Match{rootID("水果"): leaves}}
		o := apply(t, rs, groupPayload(t), Answer{OptionID: groupOpt("水果")})
		if o.Kind != OutcomeInterrupt || o.Next.Stage != StageItem || len(o.Next.Candidates) != 5 || o.Next.Candidates[0].CategoryID != 500 || o.Next.Candidates[4].CategoryID != 504 {
			t.Fatalf("要列末層前 5：%+v", o)
		}
	})
	t.Run("a group without leaves falls to describe", func(t *testing.T) {
		rs := &fakeResolver{}
		o := apply(t, rs, groupPayload(t), Answer{OptionID: groupOpt("水果")})
		if o.Kind != OutcomeInterrupt || o.Next.Stage != StageDescribe {
			t.Fatalf("%+v", o)
		}
	})
	t.Run("resolver failure is an error, not an answer", func(t *testing.T) {
		_, err := ApplyAnswer(context.Background(), &fakeResolver{err: errors.New("down")}, uitext.ZhTW, now, groupPayload(t), Answer{OptionID: groupOpt("水果")})
		if err == nil {
			t.Fatal("要回錯誤")
		}
	})
}

func itemPayload(t *testing.T) InterruptPayload {
	t.Helper()
	p := cardFromMatches([]resolver.Match{match(1, "蘋果", "水果 > 蘋果"), match(2, "蘋果乾", "加工品 > 蘋果乾")})
	if p.Stage != StageItem {
		t.Fatal("前置：要是 ITEM 卡")
	}
	return p
}

func TestApplyItemChoice(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(1, "蘋果", "水果 > 蘋果"), match(2, "蘋果乾", "加工品 > 蘋果乾")}}}
	o := apply(t, rs, itemPayload(t), Answer{OptionID: "category-2"})
	if o.Kind != OutcomeResolved || o.Resolution.Kind != slots.CategoryConfirmed || o.Resolution.CategoryID != 2 || o.Resolution.Label != "蘋果乾" {
		t.Fatalf("%+v", o)
	}
}

func TestApplyRejectsOptionsNotOnTheCard(t *testing.T) {
	for name, tc := range map[string]struct {
		p InterruptPayload
		a Answer
	}{
		"item id not in payload":         {itemPayload(t), Answer{OptionID: "category-999"}},
		"item id is a group option":      {itemPayload(t), Answer{OptionID: groupOpt("水果")}},
		"garbage id":                     {itemPayload(t), Answer{OptionID: "x"}},
		"describe stage has no options":  {*interrupt(uitext.ZhTW, describeCard("q")).Next, Answer{OptionID: planner.ItemOtherOptionID}},
		"group id not on the group card": {groupPayload(t), Answer{OptionID: "category-999"}},
	} {
		t.Run(name, func(t *testing.T) {
			if o := apply(t, &fakeResolver{}, tc.p, tc.a); o.Kind != OutcomeInvalid || o.Reason != InvalidOptionNotOnCard {
				t.Fatalf("%+v", o)
			}
		})
	}
}

func TestItemChoiceDoesNotTrustTheCardAlone(t *testing.T) {
	// 卡上有選項但 payload 的候選沒有它（payload 被改壞）：不確認。
	p := itemPayload(t)
	p.Candidates = nil
	if o := apply(t, &fakeResolver{}, p, Answer{OptionID: "category-1"}); o.Kind != OutcomeInvalid {
		t.Fatalf("%+v", o)
	}
}

func TestApplyOtherGoesToDescribeNeverConfirms(t *testing.T) {
	for _, p := range []InterruptPayload{itemPayload(t), groupPayload(t)} {
		o := apply(t, &fakeResolver{}, p, Answer{OptionID: planner.ItemOtherOptionID})
		if o.Kind != OutcomeInterrupt || o.Next.Stage != StageDescribe || o.Next.Query != "蘋果" || len(o.Next.Candidates) != 0 {
			t.Fatalf("%s：%+v", p.Stage, o)
		}
		q := o.Next.Question
		if len(q.Options) != 0 || !q.AllowFreeText || q.Prompt == "" {
			t.Fatalf("描述卡沒有選項、開放輸入：%+v", q)
		}
	}
}

func TestApplyFreeText(t *testing.T) {
	apple := match(7, "紅蘋果", "水果 > 紅蘋果")
	two := []resolver.Match{match(1, "梨", "水果 > 梨"), match(2, "梨", "花 > 梨")}
	rs := &fakeResolver{byTerm: map[string][]resolver.Match{"紅蘋果": {apple}, "梨": two, "神秘": nil, "海": spread(6, "水果", "加工品")}}
	describe := *interrupt(uitext.ZhTW, describeCard("原詞")).Next
	for name, tc := range map[string]struct {
		p     InterruptPayload
		text  string
		kind  OutcomeKind
		check func(Outcome) bool
	}{
		"item: one hit confirms":            {itemPayload(t), "紅蘋果", OutcomeResolved, func(o Outcome) bool { return o.Resolution.CategoryID == 7 }},
		"group: one hit confirms":           {groupPayload(t), " 紅蘋果 ", OutcomeResolved, func(o Outcome) bool { return o.Resolution.CategoryID == 7 }},
		"describe: one hit confirms":        {describe, "紅蘋果", OutcomeResolved, func(o Outcome) bool { return o.Resolution.Kind == slots.CategoryConfirmed }},
		"many hits ask an item card":        {itemPayload(t), "梨", OutcomeInterrupt, func(o Outcome) bool { return o.Next.Stage == StageItem && o.Next.Query == "梨" }},
		"many hits across groups ask group": {describe, "海", OutcomeInterrupt, func(o Outcome) bool { return o.Next.Stage == StageGroup && o.Next.Query == "海" }},
		"describe: no hit is buyer described": {describe, "  神秘 水果 ", OutcomeResolved, func(o Outcome) bool {
			return o.Resolution.Kind == slots.BuyerDescribed && o.Resolution.CategoryID == 0 && o.Resolution.Label == "神秘 水果"
		}},
		"item: no hit goes to describe": {itemPayload(t), "神秘", OutcomeInterrupt, func(o Outcome) bool {
			return o.Next.Stage == StageDescribe && o.Next.Query == "蘋果"
		}},
		"group: no hit goes to describe": {groupPayload(t), "神秘", OutcomeInterrupt, func(o Outcome) bool { return o.Next.Stage == StageDescribe }},
	} {
		t.Run(name, func(t *testing.T) {
			o := apply(t, rs, tc.p, Answer{FreeText: tc.text})
			if o.Kind != tc.kind || !tc.check(o) {
				t.Fatalf("%+v %+v", o, o.Next)
			}
		})
	}
}

func TestDescribeTextIsBoundedBySlotRules(t *testing.T) {
	describe := *interrupt(uitext.ZhTW, describeCard("原詞")).Next
	o := apply(t, &fakeResolver{}, describe, Answer{FreeText: strings.Repeat("果", slots.MaxTextLen+10)})
	if o.Kind != OutcomeResolved || o.Resolution.Kind != slots.BuyerDescribed || len([]rune(o.Resolution.Label)) != slots.MaxTextLen {
		t.Fatalf("文字依 slots 的長度規則截斷：%+v", o)
	}
}

func TestApplyInvalidAnswers(t *testing.T) {
	for name, tc := range map[string]struct {
		p      InterruptPayload
		a      Answer
		reason InvalidReason
	}{
		"blank text":        {itemPayload(t), Answer{FreeText: " \t　"}, InvalidBlankText},
		"neither":           {itemPayload(t), Answer{}, InvalidNeedExactlyOne},
		"both":              {itemPayload(t), Answer{OptionID: "category-1", FreeText: "x"}, InvalidNeedExactlyOne},
		"unknown stage":     {InterruptPayload{Stage: "WHAT", Query: "q"}, Answer{FreeText: "x"}, InvalidUnknownStage},
		"empty stage":       {InterruptPayload{}, Answer{OptionID: "category-1"}, InvalidUnknownStage},
		"describe + option": {*interrupt(uitext.ZhTW, describeCard("q")).Next, Answer{OptionID: "category-1"}, InvalidOptionNotOnCard},
	} {
		t.Run(name, func(t *testing.T) {
			rs := &fakeResolver{}
			o := apply(t, rs, tc.p, tc.a)
			if o.Kind != OutcomeInvalid || o.Reason != tc.reason || o.Resolution != nil || o.Next != nil {
				t.Fatalf("%+v", o)
			}
			if rs.calls != 0 {
				t.Fatal("無效答案不該去讀分類樹")
			}
		})
	}
}

func TestUnusableMatchesNeverConfirm(t *testing.T) {
	// 標籤空白的命中 Apply 不會收下：不能成為確認，也不出現在卡上。
	rs := &fakeResolver{byTerm: map[string][]resolver.Match{"梨": {match(1, "", "水果 > x")}}}
	o := apply(t, rs, itemPayload(t), Answer{FreeText: "梨"})
	if o.Kind != OutcomeInterrupt || o.Next.Stage != StageDescribe {
		t.Fatalf("%+v", o)
	}
}

func TestInterruptPayloadRoundTripsForNewStages(t *testing.T) {
	for _, p := range []InterruptPayload{groupPayload(t), *interrupt(uitext.EN, describeCard("apple")).Next} {
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var back InterruptPayload
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p, back) {
			t.Fatalf("往返不同：%s", b)
		}
	}
}

func TestNewCardsRenderInBothLocales(t *testing.T) {
	for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
		g := buildPayload(buildCard("蘋果", spread(6, "水果", "加工品")), l)
		d := buildPayload(describeCard("蘋果"), l)
		if g.Question.Prompt == "" || d.Question.Prompt == "" || g.Question.Prompt == d.Question.Prompt {
			t.Fatalf("%v：大類卡與描述卡各有自己的問句：%q / %q", l, g.Question.Prompt, d.Question.Prompt)
		}
		if ids := optionIDs(g); ids[len(ids)-1] != planner.ItemOtherOptionID {
			t.Fatalf("大類卡最後是其他：%v", ids)
		}
	}
	z := buildPayload(buildCard("蘋果", spread(6, "水果", "加工品")), uitext.ZhTW).Question.Prompt
	e := buildPayload(buildCard("蘋果", spread(6, "水果", "加工品")), uitext.EN).Question.Prompt
	if z == e {
		t.Fatal("zh-TW 與 en 的大類問句要不同")
	}
}

func TestZeroMatchesAsksDescribeNotEmptyItemCard(t *testing.T) {
	for name, ms := range map[string][]resolver.Match{"none": nil, "only non-leaf": {nonLeaf(match(7, "水果", "水果"))}} {
		p := cardFromMatches(ms)
		if p.Stage != StageDescribe || len(p.Candidates) != 0 {
			t.Fatalf("%s：要 DESCRIBE：%+v", name, p)
		}
	}
	rs := &fakeResolver{}
	o := apply(t, rs, itemPayload(t), Answer{FreeText: "查無此物"})
	if o.Kind != OutcomeInterrupt || o.Next.Stage != StageDescribe {
		t.Fatalf("ITEM 打字查無結果進 DESCRIBE：%+v", o)
	}
}

func nonLeaf(m resolver.Match) resolver.Match { m.Leaf = false; return m }

func TestNonLeafMatchesNeverConfirmOrCount(t *testing.T) {
	// 6 個命中裡 2 個是非末層：只剩 4 個末層，不到 GROUP 門檻。
	ms := spread(4, "水果", "加工品")
	ms = append(ms, nonLeaf(match(900, "x", "水果")), nonLeaf(match(901, "y", "加工品")))
	if p := cardFromMatches(ms); p.Stage != StageItem || len(p.Candidates) != 4 {
		t.Fatalf("非末層不計數：%+v", p)
	}
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{nonLeaf(match(900, "x", "水果")), match(100, "品0", "水果 > 品0")}}}
	o := apply(t, rs, *interrupt(uitext.ZhTW, describeCard("蘋果")).Next, Answer{FreeText: "蘋果"})
	if o.Kind != OutcomeResolved || o.Resolution.CategoryID != 100 {
		t.Fatalf("非末層不能確認：%+v", o)
	}
}

func TestChildlessRootGroupPickDoesNotConfirmTheGroup(t *testing.T) {
	root := match(5000, "孤兒", "孤兒")
	root.RootID = 5000
	leaf := match(100, "品0", "孤兒 > 品0")
	leaf.RootID = 5000
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{root, leaf}}, leaves: map[int64][]resolver.Match{5000: nil}}
	p := InterruptPayload{Stage: StageGroup, Query: "蘋果", Candidates: []PayloadCandidate{{CategoryID: 5000, Label: "孤兒", Path: "孤兒"}}}
	p.Question = *questionWire(reply.Question(uitext.ZhTW, planner.BuildGroupDisambiguation([]planner.ItemCandidate{{CategoryID: 5000, Label: "孤兒", Path: "孤兒"}})))
	o := apply(t, rs, p, Answer{OptionID: "category-5000"})
	if o.Kind == OutcomeResolved && o.Resolution.CategoryID == 5000 {
		t.Fatalf("不能確認大類自己：%+v", o)
	}
	if o.Kind != OutcomeResolved || o.Resolution.CategoryID != 100 {
		t.Fatalf("要確認末層 100：%+v", o)
	}
}

func TestItemPickRechecksCurrentTaxonomy(t *testing.T) {
	// category-2 已不在現行分類樹：不確認，用現在的命中重新出卡。
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(1, "蘋果", "水果 > 蘋果"), match(3, "蘋果醋", "加工品 > 蘋果醋")}}}
	o := apply(t, rs, itemPayload(t), Answer{OptionID: "category-2"})
	if o.Kind != OutcomeInterrupt || o.Next.Stage != StageItem || len(o.Next.Candidates) != 2 {
		t.Fatalf("要重新出卡：%+v", o)
	}
	for _, c := range o.Next.Candidates {
		if c.CategoryID == 2 {
			t.Fatalf("不能再列已消失的分類：%+v", o.Next.Candidates)
		}
	}
}

func TestOptionWithWhitespaceOnlyTextIsAnOptionAnswer(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(1, "蘋果", "水果 > 蘋果"), match(2, "蘋果乾", "加工品 > 蘋果乾")}}}
	o := apply(t, rs, itemPayload(t), Answer{OptionID: "category-2", FreeText: "  \t"})
	if o.Kind != OutcomeResolved || o.Resolution.CategoryID != 2 {
		t.Fatalf("%+v", o)
	}
}
