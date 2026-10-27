package cropidentity

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

func metaTree() *categoryTreeFake {
	active := codemeta.CommonStatusActive.Value
	return &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "1", Name: "蔬菜", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "10", Name: "葉菜類", ParentId: "1", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "101", Name: "甘藍", ParentId: "10", Status: active},
				{Id: "102", Name: "台灣高麗菜", ParentId: "10", Status: active},
			}},
			{Id: "11", Name: "果菜類", ParentId: "1", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "111", Name: "番茄", ParentId: "11", Status: active},
			}},
			{Id: "12", Name: "根莖類", ParentId: "1", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "121", Name: "蘿蔔", ParentId: "12", Status: active},
			}},
		}},
	}}}
}

func fingerprint(r DisambiguationResult) string {
	ids := optionIDs(r.Candidates)
	sort.Strings(ids)
	categoryID, _ := r.Selection.ConfirmedCategory()
	stage := ""
	if r.Selection != nil {
		stage = r.Selection.Stage
	}
	return fmt.Sprintf("ask=%v unavailable=%v stage=%s category=%s options=%v",
		r.NeedsDisambiguation, r.Unavailable, stage, categoryID, ids)
}

func TestMetamorphic_ParaphrasesResolveIdentically(t *testing.T) {
	groups := []struct {
		kind    string
		queries []string
	}{
		{"人工別名（cache.synonyms）", []string{"高麗菜", "甘藍"}},
		{"異體字", []string{"蕃茄", "番茄"}},
		{"臺／台正規化", []string{"臺灣高麗菜", "台灣高麗菜"}},
	}

	for _, g := range groups {
		t.Run(g.kind, func(t *testing.T) {
			var want string
			for i, q := range g.queries {
				got, err := v2UC(metaTree(), nil).Resolve(context.Background(), q)
				require.NoError(t, err)
				if i == 0 {
					want = fingerprint(got)
					continue
				}
				require.Equal(t, want, fingerprint(got),
					"「%s」與「%s」指的是同一件事，解析結果必須一致；不一致代表變體展開對其中一個失效，"+
						"而失效的樣子是「查無此品項」而不是報錯", g.queries[0], q)
			}
		})
	}
}

func TestMetamorphic_ConjunctionNeverLosesAnItem(t *testing.T) {
	categoriesOf := func(r DisambiguationResult) map[string]bool {
		out := map[string]bool{}
		if id, _ := r.Selection.ConfirmedCategory(); id != "" {
			out[id] = true
		}
		for _, c := range r.Candidates {
			out[c.CategoryID] = true
		}
		return out
	}

	a, err := v2UC(metaTree(), nil).Resolve(context.Background(), "甘藍")
	require.NoError(t, err)
	b, err := v2UC(metaTree(), nil).Resolve(context.Background(), "蘿蔔")
	require.NoError(t, err)
	both, err := v2UC(metaTree(), nil).Resolve(context.Background(), "甘藍和蘿蔔")
	require.NoError(t, err)

	union := categoriesOf(both)
	require.NotEmpty(t, union)
	for _, single := range []DisambiguationResult{a, b} {
		for id := range categoriesOf(single) {
			var covered bool
			for u := range union {
				covered = covered || u == id || parentOf(metaTree(), u, id)
			}
			require.True(t, covered,
				"單獨查得到分類 %s，合在一起問就不見了——買家問兩樣東西不該靜默只剩一樣", id)
		}
	}
}

// parentOf 回報 id 是不是 ancestor 底下的節點：合併查詢可能先出「大品項」卡，
// 單項的葉節點在那張卡的某個選項之下，同樣算沒有遺失。
func parentOf(tree *categoryTreeFake, ancestor, id string) bool {
	for _, root := range tree.response.GetCategories() {
		for _, l2 := range root.GetChildren() {
			if l2.GetId() != ancestor {
				continue
			}
			for _, l3 := range l2.GetChildren() {
				if l3.GetId() == id {
					return true
				}
			}
		}
	}
	return false
}
