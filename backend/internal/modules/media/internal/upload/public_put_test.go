package upload

import (
	"bytes"
	stdimage "image"
	"image/png"
	"io"
	"strings"
	"testing"

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
	res, err := preparePublic(t.Context(), bytes.NewReader(tinyPNG(t)))
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
		if _, err := preparePublic(t.Context(), bytes.NewReader(data)); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v，預期 invalid_input", name, err)
		}
	}
}

func TestPreparePublic_EnforcesTheSizeLimitOnActualBytes(t *testing.T) {
	big := io.LimitReader(zeroReader{}, model.MaxPublicUploadSize+1)
	if _, err := preparePublic(t.Context(), big); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err = %v，預期 invalid_input", err)
	}
}

func TestPreparePublic_ReadErrorIsNotAnInputRejection(t *testing.T) {
	_, err := preparePublic(t.Context(), io.MultiReader(strings.NewReader("x"), errReader{}))
	if err == nil || apperr.KindOf(err) == apperr.KindInvalidInput {
		t.Fatalf("err = %v，讀取錯誤不該被當成輸入不合法", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }
