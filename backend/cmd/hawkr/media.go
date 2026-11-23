package main

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"uuid"
)

// mediaSettings 是 media 模組的設定：物件儲存（MEDIA_STORAGE 與該實作的設定）與機敏資產的 KEK 金鑰環（MEDIA_KEKS）。
type mediaSettings struct {
	store media.StoreConfig
	keks  string
}

// mediaPort 是組裝點交給擁有者模組的 media 方法集合（各擁有者只依賴自己用到的子集）；正式啟動給 *media.Module，測試給 media.Fake。
type mediaPort interface {
	Confirm(ctx context.Context, uploader media.Uploader, token string) (media.ConfirmedAsset, error)
	MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error
	ReadSecure(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, string, error)
	CopyPublic(ctx context.Context, uploader media.Uploader, sourceID uuid.UUID) (media.PutPublicResult, error)
}

var (
	_ mediaPort = (*media.Module)(nil)
	_ mediaPort = (*media.Fake)(nil)
)

// readMedia 讀 media 的環境變數；缺漏記在 e，由呼叫端的 e.Err 一起回報。
func readMedia(e *config.Env) mediaSettings {
	return mediaSettings{store: media.StoreConfigFromEnv(e), keks: e.String("MEDIA_KEKS")}
}

// newMedia 建出 media 模組：儲存設定不完整或 KEK 格式錯誤時回錯誤（啟動失敗，不退回 fake）。
func newMedia(s mediaSettings, d *db.DB, jc jobs.Enqueuer, log *slog.Logger) (*media.Module, error) {
	store, err := media.OpenStore(s.store)
	if err != nil {
		return nil, err
	}
	keyring, err := media.ParseKeyring(s.keks)
	if err != nil {
		return nil, err
	}
	return media.New(media.Deps{DB: d, Jobs: jc, Store: store, NewID: uuid.NewV7, Now: time.Now, Logger: log, Keyring: keyring}), nil
}
