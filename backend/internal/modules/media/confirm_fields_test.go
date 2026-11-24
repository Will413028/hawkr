package media_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type confirmStub struct {
	failures map[string]error
	secure   map[string]bool
	calls    []string
}

func (s *confirmStub) Confirm(_ context.Context, _ media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error) {
	s.calls = append(s.calls, token)
	if err := s.failures[token]; err != nil {
		return media.ConfirmedAsset{}, err
	}
	v := media.VisibilityPublic
	if s.secure[token] {
		v = media.VisibilitySecure
	}
	if v != want {
		return media.ConfirmedAsset{}, apperr.InvalidInput("必須是公開圖片")
	}
	return media.ConfirmedAsset{ID: uuid.New(), ObjectKey: "public/" + token, Visibility: v}, nil
}

func violationFields(err error) []string {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		return nil
	}
	var out []string
	for _, v := range ae.FieldViolations() {
		out = append(out, v.Field)
	}
	return out
}

func TestConfirmFieldsReportsEveryRejectedTokenOnItsOwnField(t *testing.T) {
	s := &confirmStub{
		failures: map[string]error{"expired": apperr.FailedPrecondition("上傳已過期"), "unknown": apperr.NotFound("找不到")},
		secure:   map[string]bool{"doc": true},
	}
	items := []media.ConfirmItem{{Field: "Images[0]", Token: "ok"}, {Field: "Images[1]", Token: "expired"}, {Field: "Images[2]", Token: "doc"}, {Field: "Images[3]", Token: "unknown"}}
	_, err := media.ConfirmFields(t.Context(), s, media.Uploader{ID: 1}, items, media.VisibilityPublic)
	got := violationFields(err)
	if apperr.KindOf(err) != apperr.KindInvalidInput || len(got) != 3 || got[0] != "Images[1]" || got[1] != "Images[2]" || got[2] != "Images[3]" {
		t.Fatalf("err=%v fields=%v", err, got)
	}
}

func TestConfirmFieldsStopsOnErrorsTheUserCannotFix(t *testing.T) {
	down := apperr.Unavailable("media 暫時不可用")
	s := &confirmStub{failures: map[string]error{"down": down}}
	_, err := media.ConfirmFields(t.Context(), s, media.Uploader{ID: 1}, []media.ConfirmItem{{Field: "A", Token: "a"}, {Field: "B", Token: "down"}, {Field: "C", Token: "c"}}, media.VisibilityPublic)
	if !errors.Is(err, down) || len(s.calls) != 2 {
		t.Fatalf("err=%v calls=%v", err, s.calls)
	}
}

func TestConfirmFieldAttributesTheRejectionToTheField(t *testing.T) {
	s := &confirmStub{failures: map[string]error{"bad": apperr.NotFound("找不到")}}
	_, err := media.ConfirmField(t.Context(), s, media.Uploader{ID: 1}, "AvatarStagedToken", "bad", media.VisibilityPublic)
	if got := violationFields(err); len(got) != 1 || got[0] != "AvatarStagedToken" {
		t.Fatalf("err=%v fields=%v", err, got)
	}
	a, err := media.ConfirmField(t.Context(), s, media.Uploader{ID: 1}, "AvatarStagedToken", "good", media.VisibilityPublic)
	if err != nil || a.ObjectKey != "public/good" {
		t.Fatalf("a=%+v err=%v", a, err)
	}
}

func TestConfirmFieldsKeepsTheAlreadyConfirmedMarker(t *testing.T) {
	t.Parallel()
	elsewhere := errors.Join(apperr.FailedPrecondition("used"), media.ErrUploadAlreadyConfirmed)
	stub := &confirmStub{failures: map[string]error{"a": elsewhere, "b": apperr.InvalidInput("bad")}}
	_, err := media.ConfirmFields(context.Background(), stub, media.Uploader{ID: 1}, []media.ConfirmItem{{Field: "A", Token: "a"}, {Field: "B", Token: "b"}}, media.VisibilityPublic)
	if !errors.Is(err, media.ErrUploadAlreadyConfirmed) || apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err = %v，預期仍是欄位錯誤且帶已確認標記", err)
	}
	_, err = media.ConfirmFields(context.Background(), &confirmStub{failures: map[string]error{"b": apperr.InvalidInput("bad")}}, media.Uploader{ID: 1}, []media.ConfirmItem{{Field: "B", Token: "b"}}, media.VisibilityPublic)
	if errors.Is(err, media.ErrUploadAlreadyConfirmed) {
		t.Fatalf("無關的拒絕不該帶已確認標記：%v", err)
	}
}
