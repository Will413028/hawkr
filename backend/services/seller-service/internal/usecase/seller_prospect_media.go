package usecase

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/urlsafety"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// 站外業者的品牌圖片角色。值域跟 Admin 送來的 `kind` 逐字對齊（logo | cover），
// 兩個角色各自獨立一組欄位——換 logo 不動 cover。
const (
	ProspectMediaKindLogo  = "logo"
	ProspectMediaKindCover = "cover"
)

// sourceURLMaxLen 與 Admin 的 publicHttpsUrl 同值。兩邊必須同步：來源網址要原樣
// 回得去（Admin 的 assertProspectMediaSaved 逐字比對送出值與回應值），後端這裡
// 截斷或正規化都會讓前端把一次成功的儲存判成失敗。
const sourceURLMaxLen = 2048

var (
	// ErrProspectMediaKindInvalid 是 kind 不在值域內。
	ErrProspectMediaKindInvalid = errors.New("品牌圖片類型只能是 logo 或 cover")
	// ErrProspectMediaSourceURLInvalid 是來源網址沒通過公開 HTTPS 檢查。
	ErrProspectMediaSourceURLInvalid = errors.New("來源網址必須是公開的 HTTPS 頁面")
)

// ProspectMediaInput 是一次換圖的輸入。刻意沒有「圖片網址」欄位：bytes 一律先經
// staged-token flow 進自家 R2，這裡只收轉正用的 token。研究來源的外部網址有 64%
// 是帶簽章與到期參數的 CDN 連結，存成顯示網址幾天後就是壞圖。
type ProspectMediaInput struct {
	ProspectID           int64
	Kind                 string
	StagedToken          string
	SourceURL            string
	ExpectedMediaVersion int64
	OperatorAccountID    *int64
}

// UpdateProspectMedia 換一張站外業者的品牌圖片。
//
// 順序與 SellerUseCase.UpdateLogo 相同且不可調換：Finalize → Link → 寫 DB → Release。
// Link 必須在寫 DB 之前且失敗即中止——沒有 asset_reference 的資產是 media GC 的
// 刪除目標，先寫 DB 再 link 失敗會留下「業務列指著一個第 8 天會被實體刪掉的 key」，
// 而且顯示時才 404。
func (uc *SellerProspectUseCase) UpdateProspectMedia(ctx context.Context, in ProspectMediaInput) (domain.SellerProspect, error) {
	refType, err := prospectMediaRefType(in.Kind)
	if err != nil {
		return domain.SellerProspect{}, err
	}
	if err := validateProspectSourceURL(in.SourceURL); err != nil {
		return domain.SellerProspect{}, err
	}
	if strings.TrimSpace(in.StagedToken) == "" {
		return domain.SellerProspect{}, fmt.Errorf("%w: staged token is empty", ErrProspectMediaKindInvalid)
	}

	assets, err := uc.mediaClient.FinalizeImages(ctx, []string{in.StagedToken}, refType)
	if err != nil {
		return domain.SellerProspect{}, fmt.Errorf("finalize prospect %s: %w", in.Kind, err)
	}
	if len(assets) == 0 {
		return domain.SellerProspect{}, fmt.Errorf("finalize prospect %s: no asset returned", in.Kind)
	}

	refID := fmt.Sprintf("%d", in.ProspectID)
	if linkErr := uc.mediaClient.LinkImages(ctx, refType, refID, []string{assets[0].AssetId}); linkErr != nil {
		return domain.SellerProspect{}, fmt.Errorf("link prospect %s image (asset_id=%s, prospect_id=%d): %w",
			in.Kind, assets[0].AssetId, in.ProspectID, linkErr)
	}

	// CAS 落空時剛轉正的資產會沒有業務列指著它，7 天後由 media GC 回收。這是刻意
	// 接受的結果，與 SellerUseCase.UpdateLogo 的版本衝突路徑一致：替代方案是先查
	// 版本再轉正，但那只是把競態窗口挪小，並不會消失，卻多一次往返。
	updated, err := uc.repo.UpdateMedia(ctx, domain.ProspectMediaUpdate{
		ProspectID:           in.ProspectID,
		Kind:                 in.Kind,
		ObjectKey:            assets[0].ObjectKey,
		SourceURL:            in.SourceURL,
		ExpectedMediaVersion: in.ExpectedMediaVersion,
		OperatorAccountID:    in.OperatorAccountID,
	})
	if err != nil {
		return domain.SellerProspect{}, err
	}

	// 釋放被這次換圖取代的舊資產。失敗只 warn：新圖已經寫進 DB 且有引用，
	// 這裡失敗的後果是舊資產多留著（成本），不是顯示壞掉（正確性）。
	if relErr := uc.mediaClient.ReleaseReferences(ctx, refType,
		[]string{refID}, []string{assets[0].AssetId}); relErr != nil {
		logger.Warn(ctx, "failed to release superseded prospect media references",
			zap.Int64("prospect_id", in.ProspectID), zap.String("kind", in.Kind), zap.Error(relErr))
	}

	return updated, nil
}

func prospectMediaRefType(kind string) (string, error) {
	switch kind {
	case ProspectMediaKindLogo:
		return media.RefTypeSellerProspectLogo, nil
	case ProspectMediaKindCover:
		return media.RefTypeSellerProspectCover, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrProspectMediaKindInvalid, kind)
	}
}

// validateProspectSourceURL 在 pkg/urlsafety 的公開 HTTPS 檢查之上，補齊 Admin
// 的 publicHttpsUrl 多擋的四項：fragment、任何字面 IP（不只私有位址）、`.local`
// 主機名、以及長度上限。兩邊的值域必須一致——後端比前端寬會讓一筆前端拒收的
// 網址從回填或其他呼叫端寫進來，之後 Admin 讀回時判定成「存檔結果不完整」。
func validateProspectSourceURL(raw string) error {
	if len(raw) > sourceURLMaxLen {
		return fmt.Errorf("%w: 超過 %d 字元", ErrProspectMediaSourceURLInvalid, sourceURLMaxLen)
	}
	if err := urlsafety.ValidatePublicEvidenceURL(raw); err != nil {
		return fmt.Errorf("%w: %w", ErrProspectMediaSourceURLInvalid, err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrProspectMediaSourceURLInvalid, err)
	}
	if u.Fragment != "" || strings.Contains(raw, "#") {
		return fmt.Errorf("%w: 不接受帶 fragment 的網址", ErrProspectMediaSourceURLInvalid)
	}
	host := u.Hostname()
	if net.ParseIP(host) != nil {
		return fmt.Errorf("%w: 不接受以 IP 表示的主機", ErrProspectMediaSourceURLInvalid)
	}
	if !strings.Contains(host, ".") || strings.HasSuffix(host, ".local") {
		return fmt.Errorf("%w: 主機名 %q 不是公開網域", ErrProspectMediaSourceURLInvalid, host)
	}
	return nil
}
