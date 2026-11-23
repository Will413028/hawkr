package model

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// stubMedia 是 Media 的替身：tokens 是確認會成功的 token（對應資產的可見性），failures 是確認回的錯誤，files 是機敏檔的內容。
type stubMedia struct {
	tokens   map[string]media.Visibility
	failures map[string]error
	files    map[uuid.UUID][]byte
	readErr  error
	confirms []string
}

func (s *stubMedia) Confirm(_ context.Context, _ media.Uploader, token string) (media.ConfirmedAsset, error) {
	s.confirms = append(s.confirms, token)
	if err := s.failures[token]; err != nil {
		return media.ConfirmedAsset{}, err
	}
	vis, ok := s.tokens[token]
	if !ok {
		return media.ConfirmedAsset{}, apperr.NotFound("沒有這個 token")
	}
	return media.ConfirmedAsset{ID: uuid.NewV7(), ObjectKey: "k/" + token, Visibility: vis}, nil
}

func (*stubMedia) MarkDeleted(context.Context, []uuid.UUID) error { return nil }

func (s *stubMedia) ReadSecure(_ context.Context, id uuid.UUID) (io.ReadCloser, string, error) {
	if s.readErr != nil {
		return nil, "", s.readErr
	}
	data, ok := s.files[id]
	if !ok {
		return nil, "", apperr.NotFound("沒有這份檔")
	}
	return io.NopCloser(bytes.NewReader(data)), http.DetectContentType(data), nil
}

func violationFields(t *testing.T, err error) []string {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("要 InvalidInput：%v", err)
	}
	var out []string
	for _, v := range ae.FieldViolations() {
		out = append(out, v.Field)
	}
	return out
}

// 不合格的 token（media 拒絕、檢查不過）一次全部回報，各指到自己的欄位；其他錯誤立刻停下、原樣回傳；全部合格時與輸入一一對應。
func TestConfirmAllReportsEveryRejectedItemAndStopsOnOtherErrors(t *testing.T) {
	m := &stubMedia{
		tokens:   map[string]media.Visibility{"a": media.VisibilitySecure, "b": media.VisibilityPublic, "c": media.VisibilitySecure},
		failures: map[string]error{"expired": apperr.FailedPrecondition("已過期"), "down": errors.New("media down")},
	}
	secureOnly := func(_ int, a media.ConfirmedAsset) string {
		if a.Visibility != media.VisibilitySecure {
			return "必須機敏"
		}
		return ""
	}
	items := func(tokens ...string) []ConfirmItem {
		out := make([]ConfirmItem, len(tokens))
		for i, token := range tokens {
			out[i] = ConfirmItem{Field: apperr.IndexedField("Docs", i), Token: token}
		}
		return out
	}
	ctx := context.Background()
	assets, err := ConfirmAll(ctx, m, media.Uploader{ID: 1}, items("a", "c"), secureOnly)
	if err != nil || len(assets) != 2 || assets[0].ObjectKey != "k/a" || assets[1].ObjectKey != "k/c" {
		t.Fatalf("全部合格：%v %v", assets, err)
	}
	_, err = ConfirmAll(ctx, m, media.Uploader{ID: 1}, items("expired", "a", "unknown", "b"), secureOnly)
	if got := violationFields(t, err); !slices.Equal(got, []string{"Docs[0]", "Docs[2]", "Docs[3]"}) {
		t.Fatalf("欄位 = %v", got)
	}
	m.confirms = nil
	_, err = ConfirmAll(ctx, m, media.Uploader{ID: 1}, items("a", "down", "c"), secureOnly)
	if err == nil || apperr.KindOf(err) == apperr.KindInvalidInput || !slices.Equal(m.confirms, []string{"a", "down"}) {
		t.Fatalf("內部錯誤要立刻停下：err=%v confirms=%v", err, m.confirms)
	}
}

// 機敏檔的型別是 media 上傳時記下的；media 說沒有是資料不一致（ErrFileMissing），不是機敏資產原樣回，其他錯誤原樣回。
func TestReadSecureReturnsTheRecordedTypeAndClassifiesFailures(t *testing.T) {
	id := uuid.NewV7()
	pdf := []byte("%PDF-1.4 x")
	m := &stubMedia{files: map[uuid.UUID][]byte{id: pdf}}
	got, err := ReadSecure(context.Background(), m, id)
	if err != nil {
		t.Fatal(err)
	}
	data, rerr := io.ReadAll(got.Body)
	_ = got.Body.Close()
	if rerr != nil || !bytes.Equal(data, pdf) || got.ContentType != "application/pdf" {
		t.Fatalf("data=%q type=%s err=%v", data, got.ContentType, rerr)
	}
	if _, err := ReadSecure(context.Background(), m, uuid.NewV7()); !errors.Is(err, ErrFileMissing) {
		t.Fatalf("media 說沒有：%v", err)
	}
	m.readErr = media.ErrNotSecure
	if _, err := ReadSecure(context.Background(), m, id); !errors.Is(err, media.ErrNotSecure) || errors.Is(err, ErrFileMissing) {
		t.Fatalf("不是機敏資產：%v", err)
	}
	m.readErr = errors.New("media down")
	if _, err := ReadSecure(context.Background(), m, id); err == nil || errors.Is(err, ErrFileMissing) {
		t.Fatalf("其他錯誤原樣：%v", err)
	}
}
