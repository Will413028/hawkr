package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// cropRow 組一列「crop 型」的 catalog row，欄位齊全度跟 repository 實際回傳的
// 一致（MatchKind 恆為 "crop"，group 資訊一律帶著）。
func cropRow(cropCode, cropName, variant string) domain.CatalogSearchRow {
	return domain.CatalogSearchRow{
		MatchKind: "crop", GroupCode: "g-cabbage", GroupName: "甘藍",
		CropCode: cropCode, CropName: cropName, VariantName: variant,
	}
}

func TestListGroupCropOptions_PreservesRepositoryOrderAndFillsCropFields(t *testing.T) {
	// 排序是 repository（SQL ORDER BY 成交量）的職責，usecase 不得重排——這支
	// 斷言的就是「送進來什麼順序，送出去就是什麼順序」。
	repo := &fakeForecastRepo{
		groupCropOptionsFn: func(ctx context.Context, groupCode string, windowDays, limit int32) ([]domain.CatalogSearchRow, error) {
			return []domain.CatalogSearchRow{
				cropRow("LA1", "甘藍-初秋", "初秋"),
				cropRow("LA2", "甘藍-改良種", "改良種"),
			}, nil
		},
	}
	u := NewSearchCropCatalog(repo, &fakeSurfaceFormRepo{}, forecast.ModelArtifact{})

	got, err := u.ListGroupCropOptions(context.Background(), "g-cabbage", 10)

	if err != nil {
		t.Fatalf("ListGroupCropOptions: unexpected error %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].CropCode == nil || *got[0].CropCode != "LA1" || got[1].CropCode == nil || *got[1].CropCode != "LA2" {
		t.Fatalf("order/crop codes = %+v, want LA1 then LA2", got)
	}
	for i, m := range got {
		if m.MatchType != "crop" {
			t.Errorf("got[%d].MatchType = %q, want crop", i, m.MatchType)
		}
		if m.GroupCode != "g-cabbage" || m.GroupName != "甘藍" {
			t.Errorf("got[%d] group = %q/%q, want g-cabbage/甘藍", i, m.GroupCode, m.GroupName)
		}
		if m.MatchedAlias != nil {
			t.Errorf("got[%d].MatchedAlias = %q, want nil — 下鑽結果不是靠別名找到的", i, *m.MatchedAlias)
		}
	}
	if got[0].VariantName == nil || *got[0].VariantName != "初秋" {
		t.Errorf("got[0].VariantName = %v, want 初秋", got[0].VariantName)
	}
}

func TestListGroupCropOptions_HasForecastPerCropNotPerGroup(t *testing.T) {
	// 這支是本檔最有鑑別力的一條：HasForecast 必須逐個子品項各自判定。若實作
	// 誤用 groupHasForecast（大類只要有任一子品項有模型就全部 true），兩列都會
	// 是 true，這支轉紅。下游 ai-service 的 ResolveCrop 拿 HasForecast 當可否
	// 報價的篩選條件，錯了就會給買家一個選了也算不出價的選項。
	repo := &fakeForecastRepo{
		groupCropOptionsFn: func(ctx context.Context, groupCode string, windowDays, limit int32) ([]domain.CatalogSearchRow, error) {
			return []domain.CatalogSearchRow{
				cropRow("LA1", "甘藍-初秋", "初秋"),
				cropRow("LA9", "甘藍-進口", "進口"),
			}, nil
		},
	}
	artifact := forecast.ModelArtifact{Series: map[string]forecast.SeriesModel{
		"moa:109:LA1": {SeriesID: "moa:109:LA1", MarketID: "moa:109", CropID: "moa:LA1"},
	}}
	u := NewSearchCropCatalog(repo, &fakeSurfaceFormRepo{}, artifact)

	got, err := u.ListGroupCropOptions(context.Background(), "g-cabbage", 10)

	if err != nil {
		t.Fatalf("ListGroupCropOptions: unexpected error %v", err)
	}
	if !got[0].HasForecast {
		t.Error("LA1 有 series，HasForecast 應為 true")
	}
	if got[1].HasForecast {
		t.Error("LA9 沒有 series，HasForecast 應為 false——不能沿用大類層級的判定")
	}
	// 沒有模型的子品項仍要回傳（過濾是呼叫端的規則，見本方法 doc comment）。
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2——不可在這層過濾掉沒有模型的子品項", len(got))
	}
}

func TestListGroupCropOptions_NonPositiveLimitUsesServerDefault(t *testing.T) {
	var seenLimit, seenWindow int32
	repo := &fakeForecastRepo{
		groupCropOptionsFn: func(ctx context.Context, groupCode string, windowDays, limit int32) ([]domain.CatalogSearchRow, error) {
			seenLimit, seenWindow = limit, windowDays
			return nil, nil
		},
	}
	u := NewSearchCropCatalog(repo, &fakeSurfaceFormRepo{}, forecast.ModelArtifact{})

	if _, err := u.ListGroupCropOptions(context.Background(), "g-cabbage", 0); err != nil {
		t.Fatalf("ListGroupCropOptions: unexpected error %v", err)
	}
	if seenLimit != defaultListGroupCropOptionsLimit {
		t.Errorf("limit = %d, want server default %d", seenLimit, defaultListGroupCropOptionsLimit)
	}
	// 窗格由 usecase 決定、不由呼叫端傳入：wire 上沒有這個欄位，改窗格不該
	// 變成一次 proto 變更。
	if seenWindow != listGroupCropOptionsWindowDays {
		t.Errorf("windowDays = %d, want %d", seenWindow, listGroupCropOptionsWindowDays)
	}
}

func TestListGroupCropOptions_UnknownGroupReturnsEmptyNotError(t *testing.T) {
	repo := &fakeForecastRepo{
		groupCropOptionsFn: func(ctx context.Context, groupCode string, windowDays, limit int32) ([]domain.CatalogSearchRow, error) {
			return nil, nil
		},
	}
	u := NewSearchCropCatalog(repo, &fakeSurfaceFormRepo{}, forecast.ModelArtifact{})

	got, err := u.ListGroupCropOptions(context.Background(), "g-does-not-exist", 10)

	if err != nil {
		t.Fatalf("查無大類必須是業務結果不是錯誤，got err %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}

func TestListGroupCropOptions_RepositoryErrorPropagates(t *testing.T) {
	// 對照組：查無列回空是業務結果，DB 真的壞掉則必須往上拋——兩者不能都被
	// 吞成空清單，否則買家會看到一張「這個大類沒有子品項」的假卡片。
	sentinel := errors.New("boom")
	repo := &fakeForecastRepo{
		groupCropOptionsFn: func(ctx context.Context, groupCode string, windowDays, limit int32) ([]domain.CatalogSearchRow, error) {
			return nil, sentinel
		},
	}
	u := NewSearchCropCatalog(repo, &fakeSurfaceFormRepo{}, forecast.ModelArtifact{})

	if _, err := u.ListGroupCropOptions(context.Background(), "g-cabbage", 10); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
}
