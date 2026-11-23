//go:build integration

package seller_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"strings"
	"sync"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// stagedMedia 是 media 的替身（seller 模組的 Media port）：Confirm 回以 token 命名的資產（資產 ID 由 token 決定，見 assetFor），
// 這些測試不經真的暫存上傳。token 開頭是 logo、cover、t 的是公開圖片，其餘是機敏檔（證件與佐證）；"expired-" 開頭的 token 模擬
// media 拒絕確認（無效、過期或不屬於這個帳號）。機敏檔的內容由 put 放進來，沒放的讀不到。
type stagedMedia struct {
	mu    sync.Mutex
	files map[uuid.UUID][]byte
}

func newStagedMedia() *stagedMedia { return &stagedMedia{files: map[uuid.UUID][]byte{}} }

// assetFor 是 token 確認之後的資產 ID：token 的 SHA-256 的前 16 位元組。
func assetFor(token string) uuid.UUID {
	sum := sha256.Sum256([]byte(token))
	return uuid.UUID(sum[:16])
}

func (*stagedMedia) Confirm(_ context.Context, _ media.Uploader, token string) (media.ConfirmedAsset, error) {
	if strings.HasPrefix(token, "expired-") {
		return media.ConfirmedAsset{}, apperr.FailedPrecondition("圖片無法使用，請重新上傳")
	}
	id := assetFor(token)
	if strings.HasPrefix(token, "logo") || strings.HasPrefix(token, "cover") || strings.HasPrefix(token, "t") {
		return media.ConfirmedAsset{ID: id, ObjectKey: "public/" + token, Visibility: media.VisibilityPublic}, nil
	}
	return media.ConfirmedAsset{ID: id, ObjectKey: "secure/" + id.String() + "/", Visibility: media.VisibilitySecure}, nil
}

func (*stagedMedia) MarkDeleted(context.Context, []uuid.UUID) error { return nil }

func (m *stagedMedia) ReadSecure(_ context.Context, assetID uuid.UUID) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[assetID]
	if !ok {
		return nil, apperr.NotFound("找不到資產")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// put 放一份機敏檔的內容。
func (m *stagedMedia) put(assetID uuid.UUID, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[assetID] = data
}
