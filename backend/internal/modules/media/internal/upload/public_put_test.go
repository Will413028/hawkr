package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	stdimage "image"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/image"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPreparePublic_ReencodesAValidImage(t *testing.T) {
	res, err := newSlots().preparePublic(t.Context(), bytes.NewReader(tinyPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	if res.Format.ContentType() != "image/png" || len(res.Data) == 0 {
		t.Fatalf("結果不對：%q，%d 位元組", res.Format.ContentType(), len(res.Data))
	}
}

func TestPreparePublic_RejectsNonImagesAsInvalidInput(t *testing.T) {
	cases := map[string][]byte{
		"pdf":       []byte("%PDF-1.7\nbody"),
		"empty":     nil,
		"text":      []byte("hello"),
		"fake-png":  append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, []byte("not really")...),
		"fake-jpeg": {0xFF, 0xD8, 0xFF, 0xE0, 1, 2, 3},
	}
	for name, data := range cases {
		if _, err := newSlots().preparePublic(t.Context(), bytes.NewReader(data)); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v，預期 invalid_input", name, err)
		}
	}
}

func TestPreparePublic_EnforcesTheSizeLimitOnActualBytes(t *testing.T) {
	head := tinyPNG(t)[:16]
	big := io.MultiReader(bytes.NewReader(head), io.LimitReader(zeroReader{}, model.MaxPublicUploadSize+1-int64(len(head))))
	_, err := newSlots().preparePublic(t.Context(), big)
	if apperr.KindOf(err) != apperr.KindInvalidInput || !strings.Contains(err.Error(), "大小超過上限") {
		t.Fatalf("err = %v，預期超過大小上限的 invalid_input", err)
	}
}

func TestPublicErr_ClassifiesInputProblemsAndPipelineFailures(t *testing.T) {
	for _, e := range []error{image.ErrUnrecognized, image.ErrMalformed, image.ErrTooLarge} {
		if got := publicErr(fmt.Errorf("w: %w", e)); apperr.KindOf(got) != apperr.KindInvalidInput {
			t.Errorf("%v：err = %v，預期 invalid_input", e, got)
		}
	}
	got := publicErr(fmt.Errorf("w: %w", image.ErrProcessing))
	if got == nil || apperr.KindOf(got) == apperr.KindInvalidInput || !errors.Is(got, image.ErrProcessing) {
		t.Fatalf("err = %v，管線自己的失敗不該說成輸入不合法", got)
	}
	if boom := errors.New("boom"); publicErr(boom) != boom {
		t.Fatal("其他錯誤應原樣回傳")
	}
}

func TestRejectionReason_SeparatesInputFromPipelineFailures(t *testing.T) {
	if r, ok := rejectionReason(fmt.Errorf("w: %w", image.ErrMalformed)); !ok || r != model.RejectUnrecognizedFormat {
		t.Fatalf("輸入的問題：%q %v", r, ok)
	}
	if r, ok := rejectionReason(fmt.Errorf("w: %w", image.ErrProcessing)); !ok || r != model.RejectProcessingFailed {
		t.Fatalf("管線的失敗：%q %v", r, ok)
	}
	if _, ok := rejectionReason(errors.New("boom")); ok {
		t.Fatal("一般錯誤是暫時失敗，不該被當成永久拒絕")
	}
}

func TestPreparePublic_WaitsForADecodeSlotAndReturnsTheCtxErrorWhenCancelled(t *testing.T) {
	s := newSlots()
	for range cap(s.publicSlots) {
		s.publicSlots <- struct{}{}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.preparePublic(ctx, bytes.NewReader(tinyPNG(t))); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，預期 ctx 的錯誤，不是立即失敗", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := s.preparePublic(t.Context(), bytes.NewReader(tinyPNG(t)))
		done <- err
	}()
	<-s.publicSlots
	if err := <-done; err != nil {
		t.Fatalf("放回名額後應該成功：%v", err)
	}
}

func TestPreparePublic_DecodesNoMoreThanTheSlotBound(t *testing.T) {
	s := newSlots()
	if cap(s.publicSlots) != model.PublicPutConcurrency {
		t.Fatalf("名額 = %d，預期 %d", cap(s.publicSlots), model.PublicPutConcurrency)
	}
	if _, err := s.preparePublic(t.Context(), bytes.NewReader(tinyPNG(t))); err != nil {
		t.Fatal(err)
	}
	if len(s.publicSlots) != 0 {
		t.Fatalf("解碼結束後名額沒放回：%d", len(s.publicSlots))
	}
}

func TestAcquire_FailFastWhenFullAndWaitsWhenAsked(t *testing.T) {
	slots := make(chan struct{}, 1)
	busy := errors.New("busy")
	release, err := acquire(t.Context(), slots, false, busy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquire(t.Context(), slots, false, busy); !errors.Is(err, busy) {
		t.Fatalf("fail-fast：err = %v，預期 busy", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := acquire(ctx, slots, true, busy); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait 且 ctx 取消：err = %v，預期 ctx 的錯誤", err)
	}
	got := make(chan error, 1)
	go func() {
		rel, err := acquire(t.Context(), slots, true, busy)
		if err == nil {
			rel()
		}
		got <- err
	}()
	release()
	if err := <-got; err != nil {
		t.Fatalf("wait：放回名額後應該取得：%v", err)
	}
}

func TestPreparePublic_ReadErrorIsNotAnInputRejection(t *testing.T) {
	_, err := newSlots().preparePublic(t.Context(), io.MultiReader(strings.NewReader("x"), errReader{}))
	if err == nil || apperr.KindOf(err) == apperr.KindInvalidInput {
		t.Fatalf("err = %v，讀取錯誤不該被當成輸入不合法", err)
	}
}

func newSlots() *Service {
	return &Service{publicSlots: make(chan struct{}, model.PublicPutConcurrency)}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }
