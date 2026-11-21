package characterization

import (
	"context"
	"net/url"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// restDriver 以舊 gateway 的 REST API（統一 envelope）操作舊的 media-service：路徑與 wire 型別照舊 gateway 的
// handler（gateway/internal/handler/media.go），r.rest.Do 送出請求並以 ch.RESTKind 分類錯誤。Java 的業務錯誤
// 是 HTTP 200 加 envelope code（400 參數、404 不存在），RESTKind 兩者都看。media-service 刪除時一起刪除。
type restDriver struct {
	rest ch.REST
}

var _ Driver = (*restDriver)(nil)

const mediaBase = "/api/v1/app/media/upload_sessions"

func (d *restDriver) CreateUploadSession(ctx context.Context, c ch.Caller, in UploadInput) (UploadSession, error) {
	body := map[string]any{
		"filename": in.Filename, "contentType": in.ContentType, "fileSize": in.FileSize, "isSecure": in.IsSecure,
	}
	var out struct {
		SessionID string `json:"sessionId"`
		UploadURL string `json:"uploadUrl"`
		ExpiresIn string `json:"expiresIn"`
		IsSecure  bool   `json:"isSecure"`
	}
	if err := d.rest.Do(ctx, c, "POST", mediaBase, body, &out); err != nil {
		return UploadSession{}, err
	}
	return UploadSession{ID: out.SessionID, UploadURL: out.UploadURL, ExpiresIn: out.ExpiresIn, IsSecure: out.IsSecure}, nil
}

func (d *restDriver) CompleteUploadSession(ctx context.Context, c ch.Caller, sessionID string) (CompletedSession, error) {
	var out struct {
		StagedToken string `json:"stagedToken"`
		PreviewURL  string `json:"previewUrl"`
	}
	if err := d.rest.Do(ctx, c, "POST", mediaBase+"/"+url.PathEscape(sessionID)+"/complete", map[string]any{}, &out); err != nil {
		return CompletedSession{}, err
	}
	return CompletedSession{StagedToken: out.StagedToken, PreviewURL: out.PreviewURL}, nil
}
