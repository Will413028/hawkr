//go:build integration

package seller_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"strings"
	"sync"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type stagedMedia struct {
	mu    sync.Mutex
	files map[uuid.UUID][]byte
}

func newStagedMedia() *stagedMedia { return &stagedMedia{files: map[uuid.UUID][]byte{}} }

func assetFor(token string) uuid.UUID {
	sum := sha256.Sum256([]byte(token))
	return uuid.UUID(sum[:16])
}

func (*stagedMedia) Confirm(_ context.Context, _ media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error) {
	if strings.HasPrefix(token, "expired-") {
		return media.ConfirmedAsset{}, apperr.FailedPrecondition("圖片無法使用，請重新上傳")
	}
	id := assetFor(token)
	if strings.HasPrefix(token, "logo") || strings.HasPrefix(token, "cover") || strings.HasPrefix(token, "t") {
		if want != media.VisibilityPublic {
			return media.ConfirmedAsset{}, apperr.InvalidInput("需要機敏檔")
		}
		return media.ConfirmedAsset{ID: id, ObjectKey: "public/" + token, Visibility: media.VisibilityPublic}, nil
	}
	if want != media.VisibilitySecure {
		return media.ConfirmedAsset{}, apperr.InvalidInput("需要公開圖片")
	}
	return media.ConfirmedAsset{ID: id, ObjectKey: "secure/" + id.String() + "/", Visibility: media.VisibilitySecure}, nil
}

func (*stagedMedia) MarkDeleted(context.Context, []uuid.UUID) error { return nil }

func (m *stagedMedia) ReadSecure(_ context.Context, assetID uuid.UUID) (io.ReadCloser, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[assetID]
	if !ok {
		return nil, "", apperr.NotFound("找不到資產")
	}
	return io.NopCloser(bytes.NewReader(data)), http.DetectContentType(data), nil
}

func (m *stagedMedia) put(assetID uuid.UUID, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[assetID] = data
}
