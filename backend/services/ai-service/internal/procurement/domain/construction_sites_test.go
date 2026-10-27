package domain

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var itemResolutionConstructionSites = map[string]string{
	"procurement/domain/item_resolution.go":        "建構子本體",
	"procurement/cropidentity/crop_gate.go":        "CategoryFromSelection：系統解析，分類來自買家已確認到的分類階段",
	"procurement/usecase/crop_selection_resume.go": "CategoryFromSelection：resume 後重新解析，來源同上",
	"procurement/usecase/answer.go":                "CategoryFromSelection：自填價逃生艙沿用已確認的分類",
}

func TestItemResolutionConstructionSitesAreRegistered(t *testing.T) {
	root := filepath.Join("..", "..")
	found := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if strings.HasSuffix(path, "_test.go") || strings.Contains(path, "/testkit/") || strings.Contains(path, "/gen/") {
			return nil
		}
		b, rErr := os.ReadFile(path)
		if rErr != nil {
			return rErr
		}
		src := string(b)
		for _, ctor := range []string{"NewSystemItemResolution(", "NewBuyerChoiceItemResolution(", "NewSelfPricedItemResolution("} {
			if strings.Contains(src, ctor) {
				rel := filepath.ToSlash(path)
				if i := strings.Index(rel, "procurement/"); i >= 0 {
					rel = rel[i:]
				}
				found[rel] = true
			}
		}
		return nil
	})
	require.NoError(t, err)

	var unregistered []string
	for site := range found {
		if _, ok := itemResolutionConstructionSites[site]; !ok {
			unregistered = append(unregistered, site)
		}
	}
	sort.Strings(unregistered)
	require.Empty(t, unregistered,
		"新的 ItemResolution 建構點必須登記在 itemResolutionConstructionSites 並寫明分類來源。"+
			"2026-09-18 兩次漏接（pricebookCard 與 drillDownGroup）都是因為沒有任何清單枚舉建構點，"+
			"改一個漏一個。登記時一併在 TestEveryConstructionSiteDeclaresCategoryOrigin 補案例。")

	var missing []string
	for site := range itemResolutionConstructionSites {
		if !found[site] {
			missing = append(missing, site)
		}
	}
	sort.Strings(missing)
	require.Empty(t, missing, "登記表有已經不存在的建構點，清單會腐化，請刪除：%v", missing)
}

func TestEveryConstructionSiteDeclaresCategoryOrigin(t *testing.T) {
	crop, group, commodity := "LA1", "moa-group:abc", "958000000000000001"
	sel := &CropSelection{Stage: CropSelectionItem, ParentCategoryID: "1242", ParentLabel: "甘藍"}

	cases := []struct {
		site string
		got  *ItemResolution
	}{
		{"NewSystemItemResolution／CROP（crop_gate、crop_selection_resume 走這條）",
			NewSystemItemResolution(&crop, nil, nil, nil, "甘藍-初秋", CategoryFromSelection(sel))},
		{"NewSystemItemResolution／GROUP",
			NewSystemItemResolution(nil, &group, nil, nil, "甘藍", CategoryFromSelection(sel))},
		{"NewSystemItemResolution／COMMODITY",
			NewSystemItemResolution(nil, nil, &commodity, nil, "毛豬（規格豬）", CategoryFromSelection(sel))},
		{"NewBuyerChoiceItemResolution／作物卡（pricebookCard 與 drillDownGroup 都灌進這條）",
			NewBuyerChoiceItemResolution(CropDisambiguationOption{
				Label: "甘藍-初秋", CropCode: &crop, EntityCategoryID: "1242"})},
		{"NewBuyerChoiceItemResolution／整個大類",
			NewBuyerChoiceItemResolution(CropDisambiguationOption{
				Label: "整個甘藍都可以", ResolvedLabel: "甘藍", GroupCode: &group, EntityCategoryID: "1242"})},
		{"NewSelfPricedItemResolution／自填價逃生艙",
			NewSelfPricedItemResolution(450, "甘藍", CategoryFromSelection(sel))},
	}

	for _, c := range cases {
		require.NotNil(t, c.got, c.site)
		require.Equal(t, "1242", c.got.CategoryID,
			"%s 沒有把分類帶出來。下游 ExternalLaneSearcher 靠這個欄位展開祖先鏈比對站外業者標籤；"+
				"空的話整套分類對齊在這條路徑上靜默失效（2026-09-18 實測：站外候選 10 筆 score 4 掉成 3 筆 score 2）", c.site)
	}
}
