// Package mediaservice 是遷移期的暫時轉接：以 Java media-service 的 gRPC 實作 media.Media。
//
// 遷移計畫 B1 把 media 改寫成 Go 之後刪除。只在組裝點（cmd/hawkr）接線，模組不得 import
// （internal/arch 檢查）。stub 由 buf 從 api/proto/media 生成（buf.gen.legacy.yaml），以 connect-go
// 的 gRPC 協定呼叫，單體不依賴 grpc-go 與舊的 go-pkg。
package mediaservice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/legacy/accounttype"
	mediapb "github.com/Will413028/hawkr/backend/internal/legacy/gen/media"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/media/mediaconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 呼叫的時間預算：轉正、建立引用與解除引用各自計時，沒有內層重試（重試由呼叫端的 River job 負責）。
const (
	finalizeTimeout = 10 * time.Second
	linkTimeout     = 5 * time.Second
	unlinkTimeout   = 5 * time.Second
	readTimeout     = 10 * time.Second
)

// Config 是連到 media-service 的設定。
type Config struct {
	// Addr 是 media-service 的 gRPC 位址，host:port（本機的 compose 是 media-service:9937）。
	Addr string
	// Secret 是服務間簽章的密鑰（INTERNAL_SERVICE_HMAC_SECRET，與 Java 端相同）。空字串表示不簽章，
	// 只有 media-service 的 INTERNAL_AUTH_MODE 不是 enforcing 時才能這樣用。
	Secret string
	// Now 是簽章的時間；nil 用 time.Now。
	Now func() time.Time
}

// Client 以 media-service 的 gRPC 實作 media.Media。
type Client struct {
	rpc mediaconnect.MediaServiceClient
}

var _ media.Media = (*Client)(nil)

// New 建立 client。media-service 是 grpc-java，走 HTTP/2 cleartext（h2c）。
func New(cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		return nil, errors.New("mediaservice：缺少 media-service 的位址")
	}
	rpc := mediaconnect.NewMediaServiceClient(
		grpcclient.HTTPClient(),
		"http://"+cfg.Addr,
		connect.WithGRPC(),
		connect.WithInterceptors(grpcclient.Signer(cfg.Secret, cfg.Now)),
	)
	return &Client{rpc: rpc}, nil
}

// Finalize 轉正 staged token，不建立引用：轉正只建立資產，引用由擁有者的同步 job 之後 Link。轉正了卻沒有引用的資產由
// GC 在建立滿 7 天後回收。
func (c *Client) Finalize(ctx context.Context, u media.Uploader, stagedToken, refType string) (media.Asset, error) {
	// 呼叫方的參數錯誤在本地擋下（程式錯誤，未分類）：遠端的 400 因此只可能是 token 的問題。
	if err := media.CheckFinalizeArgs(u, stagedToken, refType); err != nil {
		return media.Asset{}, fmt.Errorf("mediaservice：轉正（呼叫方的程式錯誤）：%w", err)
	}
	fctx, cancel := context.WithTimeout(ctx, finalizeTimeout)
	defer cancel()
	req := connect.NewRequest(&mediapb.FinalizeImagesRequest{StagedTokens: []string{stagedToken}, RefType: refType})
	setUploader(req.Header(), u)
	resp, err := c.rpc.FinalizeImages(fctx, req)
	if err != nil {
		return media.Asset{}, fmt.Errorf("mediaservice：轉正：%w", err)
	}
	if err := finalizeError(resp.Msg.GetCode(), resp.Msg.GetMessage()); err != nil {
		return media.Asset{}, err
	}
	if len(resp.Msg.GetAssets()) == 0 {
		return media.Asset{}, errors.New("mediaservice：轉正沒有回傳資產")
	}
	a := resp.Msg.GetAssets()[0]
	return media.Asset{ID: a.GetAssetId(), ObjectKey: a.GetObjectKey(), Secure: a.GetIsSecure()}, nil
}

// Link 登記 ref 對 assetIDs 的引用：單次呼叫加逾時，不在這裡重試。唯一的呼叫者是 River 的同步 job（refsync），job 本身有持久
// 的重試；內層再重試只會讓逾時之後才在遠端生效的請求變多。LinkImages 是冪等的（ON CONFLICT DO NOTHING，設計書 08 §4.6），
// 且不比對上傳者，所以由 worker 代為登記，呼叫只簽章、不帶帳號。
func (c *Client) Link(ctx context.Context, ref media.Ref, assetIDs []string) error {
	if len(assetIDs) == 0 {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, linkTimeout)
	defer cancel()
	resp, err := c.rpc.LinkImages(lctx, connect.NewRequest(&mediapb.LinkImagesRequest{RefType: ref.Type, RefId: ref.ID, AssetIds: assetIDs}))
	if err != nil {
		return fmt.Errorf("mediaservice：建立引用（assets %v、ref %s/%s）：%w", assetIDs, ref.Type, ref.ID, err)
	}
	return responseError("建立引用", resp.Msg.GetCode(), resp.Msg.GetMessage())
}

// Unlink 只移除 ref 對 assetIDs 的引用（UnlinkImages）：單次呼叫加逾時（理由同 Link）；沒有的引用 media-service 忽略，
// 所以 job 重試是安全的。
func (c *Client) Unlink(ctx context.Context, ref media.Ref, assetIDs []string) error {
	if len(assetIDs) == 0 {
		return nil
	}
	uctx, cancel := context.WithTimeout(ctx, unlinkTimeout)
	defer cancel()
	resp, err := c.rpc.UnlinkImages(uctx, connect.NewRequest(&mediapb.UnlinkImagesRequest{RefType: ref.Type, RefId: ref.ID, AssetIds: assetIDs}))
	if err != nil {
		return fmt.Errorf("mediaservice：解除引用（assets %v、ref %s/%s）：%w", assetIDs, ref.Type, ref.ID, err)
	}
	return responseError("解除引用", resp.Msg.GetCode(), resp.Msg.GetMessage())
}

// ReadSecure 讀機敏資產（ServerReadSecureAsset，只接受內部呼叫，由 Signer 簽章）：三元組（object_key、ref_type、ref_id）全等才
// 讀得到，media-service 對查無與越權回同樣的碼（403、404），這裡轉成 NotFound；不是機敏資產（400）是 media.ErrNotSecure，兩者分開。內容整份在記憶體（media 的 RPC 不串流）。
func (c *Client) ReadSecure(ctx context.Context, objectKey string, ref media.Ref) (media.SecureAsset, error) {
	rctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	resp, err := c.rpc.ServerReadSecureAsset(rctx, connect.NewRequest(&mediapb.ServerReadSecureAssetRequest{ObjectKey: objectKey, RefType: ref.Type, RefId: ref.ID}))
	if err != nil {
		return media.SecureAsset{}, fmt.Errorf("mediaservice：讀取機敏資產（ref %s/%s）：%w", ref.Type, ref.ID, err)
	}
	switch code := resp.Msg.GetCode(); code {
	case 0:
		return media.SecureAsset{Data: resp.Msg.GetFileData(), ContentType: resp.Msg.GetContentType(), Filename: resp.Msg.GetFilename()}, nil
	case 400:
		return media.SecureAsset{}, fmt.Errorf("mediaservice：讀取機敏資產（code=%d）：%s：%w", code, resp.Msg.GetMessage(), media.ErrNotSecure)
	case 403, 404:
		return media.SecureAsset{}, fmt.Errorf("mediaservice：讀取機敏資產（code=%d）：%s：%w", code, resp.Msg.GetMessage(), apperr.NotFound("找不到檔案"))
	default:
		return media.SecureAsset{}, fmt.Errorf("mediaservice：讀取機敏資產失敗（code=%d）：%s", code, resp.Msg.GetMessage())
	}
}

// setUploader 帶上上傳者的身分：media-service 轉正時確認 staged token 屬於 x-account-id。x-account-type 是舊契約
// 的帳號類型（買家 1、賣家 2、兩者 3、管理員 4），media-service 只拿它當閘門（買家、賣家、管理員都放行），不參與
// 歸屬的比對；帳號的角色與此無關，一律帶買家的值。
func setUploader(h http.Header, u media.Uploader) {
	h.Set("x-account-id", strconv.FormatInt(u.ID, 10))
	h.Set("x-account-type", legacyAccountType(u))
}

func legacyAccountType(u media.Uploader) string {
	if u.Admin {
		return strconv.Itoa(accounttype.Admin)
	}
	return strconv.Itoa(accounttype.Buyer)
}

// finalizeError 是轉正的 responseError：參數已在本地檢查過（ref_type 屬於封閉集合、token 非空白、上傳者存在），所以遠端的 400、404
// （帳號不符與查無資產不可分辨、會話不存在、狀態不對、已過期、HEAD 回查不符）都是 token 或它上傳的物件的問題，只回
// media.ErrStagedTokenRejected，遠端的碼與訊息只留在錯誤的文字（進 log），不解析訊息（API 設計規範「八、錯誤」）。
func finalizeError(code int32, msg string) error {
	err := responseError("轉正", code, msg)
	if code == 400 || code == 404 {
		return fmt.Errorf("%w：%w", media.ErrStagedTokenRejected, err)
	}
	return err
}

// responseError 把 media-service 放在回應裡的碼（CodeMeta 錯誤碼，0 是成功）轉成未分類的錯誤；訊息只留在錯誤的文字裡進 log，
// 不放進給使用者的文字（ADR 2026-09-25-api-error-model D5）。Link、Unlink 的呼叫者是 River job，只需要知道成敗並重試，沒有
// 使用者可以改輸入，所以不分類。
func responseError(op string, code int32, msg string) error {
	if code == 0 {
		return nil
	}
	return fmt.Errorf("mediaservice：%s失敗（code=%d）：%s", op, code, msg)
}
