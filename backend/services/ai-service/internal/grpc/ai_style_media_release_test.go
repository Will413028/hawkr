package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

// recorderMediaAPI implements mediaClient.API, records ReleaseReferences and can
// force FinalizeAndLink to fail so UpdateAiStyle returns before touching the DB.
type recorderMediaAPI struct {
	releasedRefType string
	releasedRefIDs  []string
	releaseCalled   bool
	finalizeCalled  bool
	finalizeErr     error
}

func (m *recorderMediaAPI) FinalizeAndLink(_ context.Context, _, _, _ string) (pkgmedia.AssetInfo, error) {
	m.finalizeCalled = true
	if m.finalizeErr != nil {
		return pkgmedia.AssetInfo{}, m.finalizeErr
	}
	return pkgmedia.AssetInfo{AssetId: "new-asset", ObjectKey: "public/ai/new-thumb.png"}, nil
}

func (m *recorderMediaAPI) ReleaseReferences(_ context.Context, refType string, refIDs []string, _ []string) error {
	m.releaseCalled = true
	m.releasedRefType, m.releasedRefIDs = refType, refIDs
	return nil
}

// #1489 / #2222：換 AI 風格縮圖時，舊引用只能在「新縮圖已 link + DB 已改指新 key」之後才釋放。
//
// 這裡驗的是失敗路徑——finalize/link 失敗時**絕對不可以**已經把舊引用釋放掉：
// ai_style.thumbnail_key 這時仍指著舊縮圖，舊引用一旦被刪，那張正在顯示的圖就會在 7 天後
// 被 media GC 連同 R2 物件實體刪除。舊版正是先 release 再 finalize，踩的就是這一步。
//
// （成功路徑的順序 —— link → DB Update → release(keep=[新資產]) —— 需要真實的
// *pgxpool.Pool 才跑得到 StyleUseCase.Update，故於同型別的 seller / social /
// content 測試中釘住。）
func TestUpdateAiStyle_FinalizeFailureMustNotReleaseOldThumbnail(t *testing.T) {
	mc := &recorderMediaAPI{finalizeErr: errors.New("finalize boom")}
	s := &AIGrpcServer{
		styleUseCase: usecase.NewStyleUseCase(nil), // never reached (finalize fails first)
		mediaClient:  mc,
	}

	token := "staged-thumb"
	_, err := s.UpdateAiStyle(context.Background(), &pb.UpdateAiStyleRequest{
		Id:                   "555",
		ThumbnailStagedToken: &token,
	})
	if err == nil {
		t.Fatal("expected Internal error when finalize fails, got nil")
	}
	if !mc.finalizeCalled {
		t.Fatal("FinalizeAndLink 未被呼叫，測試沒有走到被驗的路徑")
	}
	if mc.releaseCalled {
		t.Errorf("finalize 失敗仍釋放了舊引用（refType=%q refIDs=%v）—— ai_style.thumbnail_key "+
			"還指著舊縮圖，這會讓一張正在顯示的圖被 media GC 實體刪除",
			mc.releasedRefType, mc.releasedRefIDs)
	}
}
