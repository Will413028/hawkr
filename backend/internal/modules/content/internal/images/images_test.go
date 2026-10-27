package images

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type rejectedMedia struct{ err error }

func (m rejectedMedia) Finalize(context.Context, media.Uploader, string, string) (media.Asset, error) {
	return media.Asset{}, m.err
}

func (rejectedMedia) Link(context.Context, media.Ref, []string) error   { return nil }
func (rejectedMedia) Unlink(context.Context, media.Ref, []string) error { return nil }

func TestRejectedImageKeepsContentErrorAheadOfMediaError(t *testing.T) {
	inner := apperr.InvalidInput("圖片無法使用，請重新上傳")
	downstream := fmt.Errorf("mediaservice：staged token expired：%w：%w", media.ErrStagedTokenRejected, inner)
	m := New(rejectedMedia{err: downstream})
	_, err := m.Resolve(t.Context(), 1, media.RefTypeBanner, Slot{
		Field: "PopupImageToken", Label: "彈窗圖", Token: "expired",
	})
	var got *apperr.Error
	if !errors.As(err, &got) {
		t.Fatalf("err = %v，預期 *apperr.Error", err)
	}
	if got == inner || got.Message != "彈窗圖無法使用，請重新上傳" || len(got.FieldViolations()) != 1 || got.FieldViolations()[0].Field != "PopupImageToken" {
		t.Fatalf("公開錯誤 = %+v，預期 content 的文字與 PopupImageToken 欄位", got)
	}
	if !strings.Contains(err.Error(), downstream.Error()) {
		t.Fatalf("錯誤文字 = %q，沒有下游細節", err)
	}
	if errors.Is(err, inner) {
		t.Fatal("下游錯誤不應在 unwrap chain")
	}
}

func TestRetiredIsASetDifferenceAcrossAllSlotsOfTheRef(t *testing.T) {
	old := []string{"a", "a"}
	replacedOne := Resolved{Key: "k", Asset: &media.Asset{ID: "b"}}
	kept := Resolved{Key: "k2"}
	// 兩個位置指著同一個資產 a，只換掉一個位置：a 仍被另一個位置指著，不退場。
	if got := Retired(old, After(old, replacedOne, kept)); len(got) != 0 {
		t.Fatalf("資產仍被別的位置指著，不該退場：%v", got)
	}
	// 兩個位置都換掉：a 只退場一次。
	if got := Retired(old, After(old, replacedOne, Resolved{Key: "k3", Asset: &media.Asset{ID: "c"}})); len(got) != 1 || got[0] != "a" {
		t.Fatalf("兩個位置都換掉：%v", got)
	}
	// 沿用與同一個資產不退場；刪除（沒有寫入之後的資產）全部退場並去重。
	if got := Retired([]string{"a", "b"}, After([]string{"a", "b"}, kept, Resolved{Key: "k4", Asset: &media.Asset{ID: "b"}})); len(got) != 0 {
		t.Fatalf("沿用與同一個資產：%v", got)
	}
	if got := Retired(old, nil); len(got) != 1 || got[0] != "a" {
		t.Fatalf("刪除：%v", got)
	}
}

// ReadSecure 實作 media.Media；這些測試不讀機敏資產。
func (rejectedMedia) ReadSecure(context.Context, string, media.Ref) (media.SecureAsset, error) {
	return media.SecureAsset{}, errors.New("not used")
}
