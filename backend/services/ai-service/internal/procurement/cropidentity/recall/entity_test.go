package recall

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
)

type fakeEntityClient struct {
	matches     []pbusecase.PriceableEntityMatch
	err         error
	gotVariants []string
	gotLimit    int32
}

func (f *fakeEntityClient) SearchPriceableEntities(_ context.Context, variants []string, limit int32) ([]pbusecase.PriceableEntityMatch, error) {
	f.gotVariants, f.gotLimit = variants, limit
	return f.matches, f.err
}

func TestEntityRetriever_MapsMatchesAndQuality(t *testing.T) {
	fake := &fakeEntityClient{matches: []pbusecase.PriceableEntityMatch{
		{Kind: "COMMODITY", Code: "7", Name: "白肉雞（2.0 公斤以上）產地價", CategoryIDs: []string{"2011", "2258"}, MatchedForm: "白肉雞", FormKind: "SOURCE_TERM", Tier: 0, Rank: 1, HasForecast: true},
		{Kind: "CROP", Code: "J3", Name: "荔枝-糯米", MatchedForm: "糯米", FormKind: "VARIANT", Tier: 0, Rank: 2, HasForecast: true, GroupCode: "G-LYCHEE"},
		{Kind: "CROP_GROUP", Code: "G-CAB", Name: "甘藍", MatchedForm: "高麗菜", FormKind: "ALIAS", Tier: 0, Rank: 3, HasForecast: true, GroupCode: "G-CAB"},
		{Kind: "COMMODITY", Code: "8", Name: "白肉雞門市價（高屏）", MatchedForm: "白肉雞門市價（高屏）", FormKind: "CANONICAL", Tier: 1, Rank: 4, HasForecast: true},
	}}
	r := NewEntityRetriever(fake)

	got, err := r.Recall(context.Background(), Query{Text: "白肉雞", Variants: []string{"白肉雞", "白肉鸡"}})

	require.NoError(t, err)
	require.Equal(t, []string{"白肉雞", "白肉鸡"}, fake.gotVariants)
	require.Equal(t, DefaultEntityLimit, fake.gotLimit)
	require.Len(t, got, 4)
	require.Equal(t, "COMMODITY|7", got[0].Key)
	require.Equal(t, KindCommodity, got[0].Kind)
	require.Equal(t, Strong, got[0].Quality)
	require.Equal(t, []string{"2011", "2258"}, got[0].CategoryIDs)
	require.Equal(t, 1, got[0].Rank)
	require.Equal(t, "白肉雞", got[0].MatchedBy)
	require.True(t, got[0].HasForecast)
	require.Equal(t, Medium, got[1].Quality, "VARIANT 精確 = 中")
	require.Equal(t, "G-LYCHEE", got[1].GroupCode)
	require.Equal(t, Strong, got[2].Quality, "ALIAS 精確 = 強（人工對應）")
	require.Equal(t, Weak, got[3].Quality, "前綴 = 弱")
	require.Equal(t, "entity", r.Name())
}

func TestEntityRetriever_ErrorPropagates(t *testing.T) {
	fake := &fakeEntityClient{err: errors.New("pricebook down")}
	_, err := NewEntityRetriever(fake).Recall(context.Background(), Query{Text: "x", Variants: []string{"x"}})
	require.Error(t, err, "entity retriever 是主要來源，失敗必須往上拋")
}

func TestEntityRetriever_DefaultLimitIsPinned(t *testing.T) {
	require.Equal(t, int32(20), DefaultEntityLimit)
}

func TestQualityOf(t *testing.T) {
	require.Equal(t, Strong, QualityOf("CANONICAL", 0))
	require.Equal(t, Strong, QualityOf("SOURCE_TERM", 0))
	require.Equal(t, Strong, QualityOf("ALIAS", 0))
	require.Equal(t, Medium, QualityOf("VARIANT", 0))
	require.Equal(t, Weak, QualityOf("CANONICAL", 1))
	require.Equal(t, Weak, QualityOf("SOURCE_TERM", 2))
	require.Equal(t, Weak, QualityOf("UNKNOWN", 0))
}
