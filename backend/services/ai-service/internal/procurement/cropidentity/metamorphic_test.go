package cropidentity

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
)

func fingerprint(r DisambiguationResult) string {
	deref := func(p *string) string {
		if p == nil {
			return "-"
		}
		return *p
	}
	ids := optionIDs(r.Candidates)
	sort.Strings(ids)
	return fmt.Sprintf("ask=%v unavailable=%v crop=%s group=%s commodity=%s label=%q options=%v",
		r.NeedsDisambiguation, r.Unavailable, deref(r.CropCode), deref(r.GroupCode),
		deref(r.CommodityCode), r.Label, ids)
}

func TestMetamorphic_ParaphrasesResolveIdentically(t *testing.T) {
	groups := []struct {
		kind      string
		canonical string
		queries   []string
	}{
		{"人工別名（cache.synonyms）", "甘藍", []string{"高麗菜", "甘藍"}},
		{"異體字", "番茄", []string{"蕃茄", "番茄"}},
		{"臺／台正規化", "台灣高麗菜", []string{"臺灣高麗菜", "台灣高麗菜"}},
	}

	for _, g := range groups {
		t.Run(g.kind, func(t *testing.T) {
			var want string
			for i, q := range g.queries {
				pb := &perVariantPricebookFake{byVariant: map[string][]pbusecase.PriceableEntityMatch{
					g.canonical: {pm("CROP", "LA1", "甘藍-初秋", g.canonical, "ALIAS", 0, 1, []string{"1242"}, "G-CAB", true)},
				}}
				uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, nil)
				uc.pricebook = pb

				got, err := uc.Resolve(context.Background(), q)
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
	catalog := map[string][]pbusecase.PriceableEntityMatch{
		"甘藍": {pm("CROP", "LA1", "甘藍-初秋", "甘藍", "ALIAS", 0, 1, []string{"1242"}, "G-CAB", true)},
		"蘿蔔": {pm("CROP", "SD5", "蘿蔔-白蘿蔔", "蘿蔔", "ALIAS", 0, 1, []string{"601"}, "G-RAD", true)},
	}
	newUC := func() *DisambiguationUseCase {
		uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, nil)
		uc.pricebook = &perVariantPricebookFake{byVariant: catalog}
		return uc
	}

	codesOf := func(r DisambiguationResult) map[string]bool {
		out := map[string]bool{}
		if r.CropCode != nil {
			out[*r.CropCode] = true
		}
		for _, c := range r.Candidates {
			if c.CropCode != nil {
				out[*c.CropCode] = true
			}
		}
		return out
	}

	a, err := newUC().Resolve(context.Background(), "高麗菜")
	require.NoError(t, err)
	b, err := newUC().Resolve(context.Background(), "蘿蔔")
	require.NoError(t, err)
	both, err := newUC().Resolve(context.Background(), "高麗菜和蘿蔔")
	require.NoError(t, err)

	union := codesOf(both)
	for code := range codesOf(a) {
		require.True(t, union[code],
			"「高麗菜」單獨查得到 %s，合在一起問就不見了——買家問兩樣東西不該靜默只剩一樣", code)
	}
	for code := range codesOf(b) {
		require.True(t, union[code],
			"「蘿蔔」單獨查得到 %s，合在一起問就不見了", code)
	}
}

func TestMetamorphic_CategoryIsPathIndependent(t *testing.T) {
	const wantCategory = "1242"
	catalog := map[string][]pbusecase.PriceableEntityMatch{
		"甘藍-初秋": {pm("CROP", "LA1", "甘藍-初秋", "甘藍-初秋", "CANONICAL", 0, 1, []string{wantCategory}, "G-CAB", true)},
		"甘藍": {
			pm("CROP", "LA1", "甘藍-初秋", "甘藍", "ALIAS", 0, 1, []string{wantCategory}, "G-CAB", true),
			pm("CROP", "LA2", "甘藍-改良種", "甘藍", "ALIAS", 0, 2, []string{wantCategory}, "G-CAB", true),
		},
	}
	newUC := func() *DisambiguationUseCase {
		uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, nil)
		uc.pricebook = &perVariantPricebookFake{byVariant: catalog}
		return uc
	}

	direct := gateResolutionFor(t, newUC(), "甘藍-初秋")
	require.NotNil(t, direct)

	card, err := newUC().Resolve(context.Background(), "高麗菜")
	require.NoError(t, err)
	require.True(t, card.NeedsDisambiguation, "%+v", card)
	var picked *domain.ItemResolution
	for _, opt := range card.Candidates {
		if opt.CropCode != nil && *opt.CropCode == "LA1" {
			picked = domain.NewBuyerChoiceItemResolution(opt)
		}
	}
	require.NotNil(t, picked, "卡片上應該有 LA1 供買家挑：%+v", optionIDs(card.Candidates))

	require.Equal(t, direct.CategoryID, picked.CategoryID,
		"同一個實體 LA1，打精確品名直接定案與打別名從卡片挑，分類必須一樣。"+
			"2026-09-18／19 三次漏接都是某一條抵達路徑掉了分類，而另一條是對的，所以單看一條路永遠測不出來")
	require.Equal(t, wantCategory, direct.CategoryID)
}
