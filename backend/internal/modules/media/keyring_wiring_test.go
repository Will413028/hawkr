package media_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

type noJobs struct{}

func (noJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

func wiringDeps(ring *media.Keyring) media.Deps {
	return media.Deps{
		DB: &db.DB{}, Jobs: noJobs{}, Store: storage.NewFake(), NewID: uuid.NewV7, Now: time.Now, Keyring: ring,
	}
}

// 組裝點給的 Keyring 必須進到上傳服務：沒傳進去時四個機敏方法仍回 ErrNoKeyring。
// 空內容在辨認型別時就被拒（InvalidInput），不碰資料庫與儲存，所以不需要容器。
func TestNew_PassesTheKeyringToTheUploadService(t *testing.T) {
	ring, err := media.ParseKeyring("k1:AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=")
	if err != nil {
		t.Fatal(err)
	}
	with := media.New(wiringDeps(ring))
	_, err = with.UploadSecure(t.Context(), media.Uploader{ID: 1001}, strings.NewReader(""))
	if errors.Is(err, media.ErrNoKeyring) || apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("有 keyring：err = %v，預期 invalid_input（空內容）", err)
	}
	without := media.New(wiringDeps(nil))
	if _, err := without.UploadSecure(t.Context(), media.Uploader{ID: 1001}, strings.NewReader("")); !errors.Is(err, media.ErrNoKeyring) {
		t.Fatalf("沒有 keyring：err = %v，預期 ErrNoKeyring", err)
	}
}
