package cropidentity

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

func v2CategoryTree() *categoryTreeFake {
	active := codemeta.CommonStatusActive.Value
	return &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "100", Name: "肉類", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "200", Name: "豬肉", ParentId: "100", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "2008", Name: "豬肉", ParentId: "200", Status: active},
				{Id: "2257", Name: "生鮮豬肉", ParentId: "200", Status: active},
			}},
			{Id: "210", Name: "雞肉", ParentId: "100", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "2011", Name: "雞肉", ParentId: "210", Status: active},
				{Id: "2258", Name: "生鮮雞肉", ParentId: "210", Status: active},
			}},
		}},
		{Id: "600", Name: "穀物", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "220", Name: "米", ParentId: "600", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "2022", Name: "米", ParentId: "220", Status: active},
			}},
		}},
	}}}
}

func newTaxonomyUC(tree cropCategoryReader) *DisambiguationUseCase {
	return NewDisambiguationUseCase(tree, nil)
}

func TestTaxonomyRetriever_ExactLeafIsStrongSiblingsWeak(t *testing.T) {
	got, err := taxonomyRetriever{uc: newTaxonomyUC(v2CategoryTree())}.Recall(context.Background(), understandQuery("豬肉"))

	require.NoError(t, err)
	require.NotEmpty(t, got)
	require.Equal(t, "CATEGORY|2008", got[0].Key)
	require.Equal(t, recall.KindCategory, got[0].Kind)
	require.Equal(t, recall.Strong, got[0].Quality)
	require.Equal(t, 3, got[0].CategoryLevel)
	require.Equal(t, []string{"2008"}, got[0].CategoryIDs)
	require.Equal(t, "200", got[0].ParentID, "L3 候選要帶 L2 父節點，ITEM 卡靠它")
	require.Equal(t, "豬肉", got[0].ParentLabel)
	require.Equal(t, 1, got[0].Rank)
	require.False(t, got[0].Pinned, "豬肉 不是經人工 synonym 改寫來的")
	keys := map[string]recall.Quality{}
	for _, c := range got {
		keys[c.Key] = c.Quality
	}
	require.Contains(t, keys, "CATEGORY|2257", "L2 豬肉 精確命中 ⇒ 子節點 生鮮豬肉 也進來")
	require.Equal(t, recall.Weak, keys["CATEGORY|2257"], "但只是弱訊號")
	require.NotContains(t, keys, "CATEGORY|2011")
}

func TestTaxonomyRetriever_ManualSynonymIsPinned(t *testing.T) {

	got, err := taxonomyRetriever{uc: newTaxonomyUC(v2CategoryTree())}.Recall(context.Background(), understandQuery("白肉雞"))

	require.NoError(t, err)
	require.NotEmpty(t, got)
	require.Equal(t, "CATEGORY|2011", got[0].Key)
	require.Equal(t, recall.Strong, got[0].Quality)
	require.True(t, got[0].Pinned)
	for _, c := range got[1:] {
		require.False(t, c.Pinned, "只有改寫後精確命中的那一個是 pinned：%s", c.Key)
	}
}

func TestTaxonomyRetriever_ContainmentIsWeak(t *testing.T) {
	got, err := taxonomyRetriever{uc: newTaxonomyUC(v2CategoryTree())}.Recall(context.Background(), understandQuery("生鮮豬"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "CATEGORY|2257", got[0].Key)
	require.Equal(t, recall.Weak, got[0].Quality)
}

func TestTaxonomyRetriever_NoMatchIsEmptyNotError(t *testing.T) {
	got, err := taxonomyRetriever{uc: newTaxonomyUC(v2CategoryTree())}.Recall(context.Background(), understandQuery("水泥"))
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestTaxonomyRetriever_TreeUnavailableIsErrorForCallerToDegrade(t *testing.T) {
	_, err := taxonomyRetriever{uc: newTaxonomyUC(&categoryTreeFake{err: errors.New("down")})}.Recall(context.Background(), understandQuery("豬肉"))
	require.Error(t, err)
}

func TestTaxonomyRetriever_NilTreeReaderIsEmpty(t *testing.T) {
	got, err := taxonomyRetriever{uc: newTaxonomyUC(nil)}.Recall(context.Background(), understandQuery("豬肉"))
	require.NoError(t, err)
	require.Empty(t, got)
}

func fruitTree() *categoryTreeFake {
	active := codemeta.CommonStatusActive.Value
	return &categoryTreeFake{response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "1", Name: "蔬菜", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "66", Name: "甜椒", ParentId: "1", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "665", Name: "水果彩椒", ParentId: "66", Status: active},
			}},
		}},
		{Id: "2", Name: "水果", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "133", Name: "香蕉", ParentId: "2", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "1331", Name: "香蕉", ParentId: "133", Status: active},
			}},
			{Id: "134", Name: "鳳梨", ParentId: "2", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "1341", Name: "鳳梨", ParentId: "134", Status: active},
			}},
		}},
	}}}
}

func TestTaxonomyRetriever_ExactL1EmitsItsGroupsAsStrongCandidates(t *testing.T) {
	got, err := taxonomyRetriever{uc: newTaxonomyUC(fruitTree())}.Recall(context.Background(), recall.Query{Text: "水果", Variants: []string{"水果"}})

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 2)
	require.Equal(t, recall.CandidateKey(recall.KindCategory, "133"), got[0].Key)
	require.Equal(t, recall.CandidateKey(recall.KindCategory, "134"), got[1].Key)
	for _, c := range got[:2] {
		require.Equal(t, 2, c.CategoryLevel, "L1 命中給的是未展開的 L2")
		require.Equal(t, recall.Strong, c.Quality, "買家打的就是這個大分類名")
		require.Equal(t, "2", c.ParentID)
		require.Equal(t, "水果", c.ParentLabel)
		require.Empty(t, c.CategoryIDs, "L2 不是可取價節點")
	}
}

func TestTaxonomyRetriever_L1HitOutranksSubstringLeaf(t *testing.T) {
	got, err := taxonomyRetriever{uc: newTaxonomyUC(fruitTree())}.Recall(context.Background(), recall.Query{Text: "水果", Variants: []string{"水果"}})

	require.NoError(t, err)
	var leaf *recall.Candidate
	for i := range got {
		if got[i].Code == "665" {
			leaf = &got[i]
		}
	}
	require.NotNil(t, leaf, "水果彩椒仍然被召回")
	require.Equal(t, recall.Weak, leaf.Quality, "它只是子串命中")
}
