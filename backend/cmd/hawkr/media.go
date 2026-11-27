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

type mediaSettings struct {
	store media.StoreConfig
	keks  string
}

type mediaPort interface {
	Confirm(ctx context.Context, uploader media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error)
	MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error
	ReadSecure(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, string, error)
	CopyPublic(ctx context.Context, uploader media.Uploader, sourceID uuid.UUID) (media.PutPublicResult, error)
}

var (
	_ mediaPort = (*media.Module)(nil)
	_ mediaPort = (*media.Fake)(nil)
)

func readMedia(e *config.Env) mediaSettings {
	return mediaSettings{store: media.StoreConfigFromEnv(e), keks: e.String("MEDIA_KEKS")}
}

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
